package main

import (
	"crypto/tls"
	"io"
	"os"
	"bufio"
	"imap"
	"log"
)

func check(err os.Error) {
	if err != nil {
		panic(err)
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

var verbose bool = false

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	user, pass := loadAuth("auth")

	conn, err := tls.Dial("tcp", "imap.gmail.com:993", nil)
	check(err)

	var r io.Reader = conn
	if verbose {
		r = newLoggingReader(r, 300)
	}
	im := imap.New(r, conn)
	im.Unsolicited = make(chan interface{}, 100)

	log.Printf("connecting")
	hello, err := im.Start()
	check(err)
	log.Printf("server hello: %s", hello)

	log.Printf("logging in")
	resp, caps, err := im.Auth(user, pass)
	check(err)
	log.Printf("capabilities: %s", caps)
	log.Printf("%s", resp)

	{
		mailboxes, err := im.List("", imap.WildcardAny)
		check(err)
		log.Printf("Available mailboxes:")
		for _, mailbox := range mailboxes {
			log.Printf("- %s", mailbox.Name)
		}
		readExtra(im)
	}

	{
		resp, err := im.Examine("INBOX")
		check(err)
		log.Printf("%s", resp)
		log.Printf("%+v", resp)
		readExtra(im)
	}

	f, err := os.Create("mbox")
	check(err)
	mbox := newMbox(f)

	{
		fetches, err := im.Fetch("1:4", []string{"RFC822"})
		check(err)
		for _, fetch := range fetches {
			mbox.writeMessage(fetch.Rfc822)
		}
		readExtra(im)
	}

	log.Printf("done")
}
