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
		resp, lists, err := imap.List("", WildcardAny)
		check(err)
		log.Printf("%s", resp)
		for _, list := range lists {
			log.Printf("- %s", list)
		}
		if len(resp.extra) > 0 {
			log.Printf("extra %+v", resp.extra)
		}
	}

	{
		resp, err := imap.Examine("lkml")
		check(err)
		log.Printf("%s", resp)
		log.Printf("%#v", resp)
		if len(resp.extra) > 0 {
			log.Printf("extra %+v", resp.extra)
		}
	}

	{
		resp, fetches, err := imap.Fetch("1:4", []string{"ALL"})
		check(err)
		log.Printf("%s", resp)
		log.Printf("%+v", fetches)
		if len(resp.extra) > 0 {
			log.Printf("extra %+v", resp.extra)
		}
	}

	log.Printf("done")
}
