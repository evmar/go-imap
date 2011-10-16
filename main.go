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
		case update := <-imap.responseData:
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
	imap.responseData = make(chan interface{}, 100)
	imap.protoLog = log.New(os.Stderr, "proto ", log.Ltime)

	log.Printf("connecting")
	_, err := imap.Connect("imap.gmail.com:993")
	check(err)

	ch := make(chan *Response, 1)

	log.Printf("logging in")
	err = imap.Auth(user, pass, ch)
	check(err)
	state.Await(imap, ch)

/*
	err = imap.List("", WildcardAny, ch)
	check(err)
	log.Printf("%v", <-ch)
*/

	err = imap.Examine("lkml", ch)
	check(err)
	state.list = &List{name:"lkml"}
	state.Await(imap, ch)

	log.Printf("%v", state.list)

	err = imap.Fetch("1:4", []string{"RFC822.HEADER"}, ch)
	check(err)
	state.Await(imap, ch)

	log.Printf("done")
}
