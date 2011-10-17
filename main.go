package main

import (
	"os"
	"bufio"
	"log"
)

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

func readExtra(imap *IMAP) {
	for {
		select {
		case msg := <-imap.unsolicited:
			log.Printf("*** unsolicited: %T %+v", msg, msg)
		default:
			return
		}
	}
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	user, pass := loadAuth("auth")

	imap := NewIMAP()
	imap.unsolicited = make(chan interface{}, 100)

	log.Printf("connecting")
	hello, err := imap.Connect("imap.gmail.com:993")
	check(err)
	log.Printf("server hello: %s", hello)

	log.Printf("logging in")
	resp, err := imap.Auth(user, pass)
	check(err)
	log.Printf("%s", resp)

	{
		lists, err := imap.List("", WildcardAny)
		check(err)
		for _, list := range lists {
			log.Printf("- %s", list)
		}
		readExtra(imap)
	}

	{
		resp, err := imap.Examine("lkml")
		check(err)
		log.Printf("%s", resp)
		log.Printf("%+v", resp)
		readExtra(imap)
	}

	{
		fetches, err := imap.Fetch("1:4", []string{"ALL"})
		check(err)
		log.Printf("%s", resp)
		for _, fetch := range fetches {
			log.Printf("%+v", fetch)
		}
		readExtra(imap)
	}

	log.Printf("done")
}
