package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/martine/go-imap/imap"
	"io"
	"log"
	"os"
	"time"
)

var dumpProtocol *bool = flag.Bool("dumpprotocol", false, "dump imap stream")

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type progressMessage struct {
	cur, total int
	text       string
}

type UI struct {
	statusChan chan interface{}
	netmon     *netmonReader
}

func (ui *UI) log(format string, args ...interface{}) {
	ui.statusChan <- fmt.Sprintf(format, args...)
}

func (ui *UI) progress(cur, total int, format string, args ...interface{}) {
	message := &progressMessage{cur, total, fmt.Sprintf(format, args...)}
	ui.statusChan <- message
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

	ui.log("connecting...")
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
	ui.log("server hello: %s", hello)

	ui.log("logging in...")
	resp, caps, err := im.Auth(user, pass)
	check(err)
	ui.log("%s", resp)
	ui.log("server capabilities: %s", caps)

	return im
}

func (ui *UI) fetch(im *imap.IMAP, mailbox string) {
	ui.log("opening %s...", mailbox)
	examine, err := im.Examine(mailbox)
	check(err)
	ui.log("mailbox status: %+v", examine)
	readExtra(im)

	f, err := os.Create(mailbox + ".mbox")
	check(err)
	mbox := newMbox(f)

	query := fmt.Sprintf("1:%d", examine.Exists)
	ui.log("requesting messages %s", query)

	ch, err := im.FetchAsync(query, []string{"RFC822"})
	check(err)

	envelopeDate := time.Now().Local().Format(time.ANSIC)

	i := 0
	total := examine.Exists
	ui.progress(i, total, "fetching messages", i, total)
L:
	for {
		r := <-ch
		switch r := r.(type) {
		case *imap.ResponseFetch:
			mbox.writeMessage("imapsync@none", envelopeDate, r.Rfc822)
			i++
			ui.progress(i, total, "fetching messages")
		case *imap.ResponseStatus:
			ui.log("complete %v\n", r)
			break L
		}
	}
	readExtra(im)
}

func (ui *UI) runFetch(mailbox string) {
	ui.statusChan = make(chan interface{})
	go func() {
		defer func() {
			if e := recover(); e != nil {
				ui.statusChan <- e
			}
		}()
		im := ui.connect(true)
		ui.fetch(im, mailbox)
		close(ui.statusChan)
	}()

	ticker := time.NewTicker(1000 * 1000 * 1000)
	overprint := false
	status := ""
	overprintLast := false
	for ui.statusChan != nil {
		select {
		case s, stillOpen := <-ui.statusChan:
			switch s := s.(type) {
			case string:
				status = s
				overprint = false
			case *progressMessage:
				status = fmt.Sprintf("%s [%d/%d]", s.text, s.cur, s.total)
				overprint = true
			default:
				if s != nil {
					status = s.(error).Error()
					ui.statusChan = nil
					ticker.Stop()
				}
			}
			if !stillOpen {
				ui.statusChan = nil
				ticker.Stop()
			}
		case <-ticker.C:
			if ui.netmon != nil {
				ui.netmon.Tick()
			}
		}

		if overprintLast {
			fmt.Printf("\r\x1B[K")
		} else {
			fmt.Printf("\n")
		}
		overprintLast = overprint
		fmt.Printf("%s", status)
		if overprint && ui.netmon != nil {
			fmt.Printf(" [%.1fk/s]", ui.netmon.Bandwidth()/1000.0)
		}
	}
	fmt.Printf("\n")
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
		ui.runFetch(args[0])
	default:
		usage()
	}
}
