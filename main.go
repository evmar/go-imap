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

type List struct {
	name string
	flags []string
	exists int
}

type State struct {
	list *List
}

func (s *State) ProcessUpdate(update interface{}) {
	switch update := update.(type) {
	case *ResponseExists:
		s.list.exists = update.count
	case *ResponseFlags:
		s.list.flags = update.flags
	case *ResponseRecent:
		// ignore
	case *ResponseFetch:
		log.Printf("fetched message content %+v", update)
	default:
		log.Printf("unhandled update type %T", update)
	}
}

func (s *State) Await(imap *IMAP, ch chan *Response) *Response {
	for {
		select {
		case update := <-imap.unsolicited:
			s.ProcessUpdate(update)
		case response := <-ch:
			return response
		}
	}
	return nil
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	user, pass := loadAuth("auth")

	state := State{}
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

	if false {
		resp, lists, err := imap.List("", WildcardAny)
		check(err)
		log.Printf("%s", resp)
		for _, list := range lists {
			log.Printf("- %s", list)
		}
	}

	{
		resp, err := imap.Examine("lkml")
		check(err)
		log.Printf("%s", resp)
		log.Printf("%#v", resp)
		log.Printf("%+v", resp.extra)
	}

	ch := make(chan *Response, 1)

	err = imap.Fetch("1:4", []string{"ALL"}, ch)
	check(err)
	state.Await(imap, ch)

	log.Printf("done")
}
