package main

import (
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

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	user, pass := loadAuth("auth")

	im := imap.NewIMAP()
	im.Unsolicited = make(chan interface{}, 100)

	log.Printf("connecting")
	hello, err := im.Connect("imap.gmail.com:993")
	check(err)
	log.Printf("server hello: %s", hello)

	log.Printf("logging in")
	resp, err := im.Auth(user, pass)
	check(err)
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

	mbox, err := os.Create("mbox")
	check(err)

	{
		fetches, err := im.Fetch("1:4", []string{"RFC822"})
		check(err)
		for _, fetch := range fetches {
			mbox.Write([]byte("From whatever\r\n"))
			mbox.Write(fetch.Rfc822)
			mbox.Write([]byte("\r\n"))
		}
		readExtra(im)
	}

	log.Printf("done")
}
