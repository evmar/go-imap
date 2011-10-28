package main

import (
	"crypto/tls"
	"io"
	"os"
	"bufio"
	"imap"
	"log"
	"flag"
	"fmt"
	"time"
)

var dumpProtocol *bool = flag.Bool("dumpprotocol", false, "dump imap stream")

func check(err os.Error) {
	if err != nil {
		panic(err)
	}
}

var statusChan chan string
var netmon *netmonReader

func status(format string, args ...interface{}) {
	if statusChan != nil {
		statusChan <- fmt.Sprintf(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

func loadAuth(path string) (string, string) {
	f, err := os.Open(path)
	check(err)
	r := bufio.NewReader(f)

	user, isPrefix, err := r.ReadLine()
	check(err)
	if isPrefix {
		panic("prefix")
	}

	pass, isPrefix, err := r.ReadLine()
	check(err)
	if isPrefix {
		panic("prefix")
	}

	return string(user), string(pass)
}

func readExtra(im *imap.IMAP) {
	for {
		select {
		case msg := <-im.Unsolicited:
			log.Printf("*** unsolicited: %T %+v", msg, msg)
		default:
			return
		}
	}
}

func connect(useNetmon bool) *imap.IMAP {
	user, pass := loadAuth("auth")

	conn, err := tls.Dial("tcp", "imap.gmail.com:993", nil)
	check(err)

	var r io.Reader = conn
	if *dumpProtocol {
		r = newLoggingReader(r, 300)
	}
	if useNetmon {
		netmon = newNetmonReader(r)
		r = netmon
	}
	im := imap.New(r, conn)
	im.Unsolicited = make(chan interface{}, 100)

	status("connecting")
	hello, err := im.Start()
	check(err)
	status("server hello: %s", hello)

	status("logging in")
	resp, caps, err := im.Auth(user, pass)
	check(err)
	status("capabilities: %s", caps)
	status("%s", resp)

	return im
}

func fetch(im *imap.IMAP, mailbox string) {
	examine, err := im.Examine(mailbox)
	check(err)
	status("%+v", examine)
	readExtra(im)

	f, err := os.Create(mailbox + ".mbox")
	check(err)
	mbox := newMbox(f)

	query := fmt.Sprintf("1:%d", examine.Exists)
	status("fetching %s", query)

	ch, err := im.FetchAsync(query, []string{"RFC822"})
	check(err)

	i := 0
L:
	for {
		r := <-ch
		switch r := r.(type) {
		case *imap.ResponseFetch:
			mbox.writeMessage(r.Rfc822)
			status("got message %d/%d", i, examine.Exists)
			i++
		case *imap.ResponseStatus:
			status("complete %v\n", r)
			break L
		}
	}
	readExtra(im)
}

func runFetch(im *imap.IMAP, mailbox string) {
	statusChan := make(chan string)
	go func() {
		fetch(im, mailbox)
		close(statusChan)
	}()

	ticker := time.NewTicker(1000 * 1000 * 1000)
	status := ""
	for statusChan != nil {
		select {
		case s, closed := <-statusChan:
			status = s
			if closed {
				statusChan = nil
			}
			ticker.Stop()
		case <-ticker.C:
			netmon.Tick()
		}
		log.Printf("%.1fk/s %s\n", netmon.Bandwidth() / 1000.0, status)
	}
}

func usage() {
	fmt.Printf("usage: %s command\n", os.Args[0])
	fmt.Printf("commands are:\n")
	fmt.Printf("  list   list mailboxes\n")
	fmt.Printf("  fetch  download mailbox\n")
	os.Exit(0)
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		usage()
	}
	mode := args[0]
	args = args[1:]

	switch mode {
	case "list":
		im := connect(false)
		mailboxes, err := im.List("", imap.WildcardAny)
		check(err)
		fmt.Printf("Available mailboxes:\n")
		for _, mailbox := range mailboxes {
			fmt.Printf("  %s\n", mailbox.Name)
		}
		readExtra(im)
	case "fetch":
		if len(args) < 1 {
			fmt.Printf("must specify mailbox to fetch\n")
			os.Exit(1)
		}
		im := connect(true)
		runFetch(im, args[0])
	default:
		usage()
	}
}
