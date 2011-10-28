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

type UI struct {
	statusChan chan string
	netmon     *netmonReader
}

func (ui *UI) status(format string, args ...interface{}) {
	if ui.statusChan != nil {
		ui.statusChan <- fmt.Sprintf(format, args...)
	} else {
		fmt.Printf(format, args...)
		fmt.Printf("\n")
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

func (ui *UI) connect(useNetmon bool) *imap.IMAP {
	user, pass := loadAuth("auth")

	ui.status("connecting...")
	conn, err := tls.Dial("tcp", "imap.gmail.com:993", nil)
	check(err)

	var r io.Reader = conn
	if *dumpProtocol {
		r = newLoggingReader(r, 300)
	}
	if useNetmon {
		ui.netmon = newNetmonReader(r)
		r = ui.netmon
	}
	im := imap.New(r, conn)
	im.Unsolicited = make(chan interface{}, 100)

	hello, err := im.Start()
	check(err)
	ui.status("server hello: %s", hello)

	ui.status("logging in...")
	resp, caps, err := im.Auth(user, pass)
	check(err)
	ui.status("%s", resp)
	ui.status("server capabilities: %s", caps)

	return im
}

func (ui *UI) fetch(im *imap.IMAP, mailbox string) {
	ui.status("opening %s...", mailbox)
	examine, err := im.Examine(mailbox)
	check(err)
	ui.status("mailbox status: %+v", examine)
	readExtra(im)

	f, err := os.Create(mailbox + ".mbox")
	check(err)
	mbox := newMbox(f)

	query := fmt.Sprintf("1:%d", examine.Exists)
	ui.status("fetching messages %s", query)

	ch, err := im.FetchAsync(query, []string{"RFC822"})
	check(err)

	i := 1
L:
	for {
		r := <-ch
		switch r := r.(type) {
		case *imap.ResponseFetch:
			mbox.writeMessage(r.Rfc822)
			ui.status("got message %d/%d", i, examine.Exists)
			i++
		case *imap.ResponseStatus:
			ui.status("complete %v\n", r)
			break L
		}
	}
	readExtra(im)
}

func (ui *UI) runFetch(im *imap.IMAP, mailbox string) {
	ui.statusChan = make(chan string)
	go func() {
		defer func() {
			if e := recover(); e != nil {
				log.Printf("paniced %s", e)
			}
		}()
		ui.fetch(im, mailbox)
		close(ui.statusChan)
	}()

	ticker := time.NewTicker(1000 * 1000 * 1000)
	status := ""
	for ui.statusChan != nil {
		overprint := true
		select {
		case s, stillOpen := <-ui.statusChan:
			if s != status {
				overprint = false
				status = s
			}
			if !stillOpen {
				ui.statusChan = nil
				ticker.Stop()
			}
		case <-ticker.C:
			ui.netmon.Tick()
		}
		if overprint {
			fmt.Printf("\r\x1B[K")
		} else {
			fmt.Printf("\n")
		}
		if status != "" {
			fmt.Printf("[%.1fk/s] %s", ui.netmon.Bandwidth() / 1000.0, status)
		}
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

	ui := new(UI)

	switch mode {
	case "list":
		im := ui.connect(false)
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
		im := ui.connect(true)
		ui.runFetch(im, args[0])
	default:
		usage()
	}
}
