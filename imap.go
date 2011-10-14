package main

import (
	"bufio"
	"crypto/tls"
	"os"
	"log"
	"strings"
	"fmt"
	"net/textproto"
	"io"
	"strconv"
	"sync"
)

func check(err os.Error) {
	if err != nil {
		panic(err)
	}
}

type Status int
const (
	OK = Status(iota)
	NO
	BAD
)
func (s Status) String() string {
	return []string{
		"OK",
		"NO",
		"BAD",
	}[s];
}

const (
	WildcardAny = "%"
	WildcardAnyRecursive = "*"
)

type TriBool int
const (
	TriUnknown = TriBool(iota)
	TriTrue
	TriFalse
)
func (t TriBool) String() string {
	switch t {
	case TriTrue: return "true"
	case TriFalse: return "false"
	}
	return "unknown"
}

type Tag int
const Untagged = Tag(-1)

type Response struct {
	status Status
	text string
}

type ResponseChan chan *Response

type IMAP struct {
	// Client thread.
	nextTag int

	// Background thread.
	r *textproto.Reader
	w io.Writer
	protoLog *log.Logger

	lock sync.Mutex
	pending map[Tag]chan *Response
}

func NewIMAP() *IMAP {
	return &IMAP{pending:make(map[Tag]chan *Response)}
}

func (imap *IMAP) Connect(hostport string) (string, os.Error) {
	log.Printf("dial")
	conn, err := tls.Dial("tcp", hostport, nil)
	if err != nil {
		return "", err
	}

	imap.r = textproto.NewReader(bufio.NewReader(conn))
	imap.w = conn

	log.Printf("readline")
	tag, text, err := imap.ReadLine()
	if err != nil {
		return "", err
	}
	if tag != Untagged {
		return "", fmt.Errorf("expected untagged server hello. got %q", text)
	}

	status, text, err := ParseStatus(text)
	if status != OK {
		return "", fmt.Errorf("server hello %v %q", status, text)
	}

	imap.StartLoops()

	return text, nil
}

func splitToken(text string) (string, string) {
	space := strings.Index(text, " ")
	if space < 0 {
		return text, ""
	}
	return text[:space], text[space+1:]
}

func (imap *IMAP) ReadLine() (Tag, string, os.Error) {
	line, err := imap.r.ReadLine()
	if err != nil {
		return Untagged, "", err
	}
	if imap.protoLog != nil {
		imap.protoLog.Printf("<-server %s", line)
	}

	switch line[0] {
	case '*':
		return Untagged, line[2:], nil
	case 'a':
		tagstr, text := splitToken(line)
		tagnum, err := strconv.Atoi(tagstr[1:])
		if err != nil {
			return Untagged, "", err
		}
		return Tag(tagnum), text, nil
	}

	return Untagged, "", fmt.Errorf("unexpected response %q", line)
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func (imap *IMAP) Send(command string, ch chan *Response) os.Error {
	tag := Tag(imap.nextTag)
	imap.nextTag++

	toSend := []byte(fmt.Sprintf("a%d %s\r\n", int(tag), command))
	if imap.protoLog != nil {
		imap.protoLog.Printf("server<- %s...", toSend[0:min(len(command),20)])
	}

	if ch != nil {
		imap.lock.Lock()
		imap.pending[tag] = ch
		imap.lock.Unlock()
	}

	_, err := imap.w.Write(toSend)
	return err
}

func (imap *IMAP) Auth(user string, pass string, ch ResponseChan) os.Error {
	return imap.Send(fmt.Sprintf("LOGIN %s %s", user, pass), ch)
}

func quote(in string) string {
	if strings.IndexAny(in, "\r\n") >= 0 {
		panic("invalid characters in string to quote")
	}
	return "\"" + in + "\""
}

func (imap *IMAP) List(reference string, name string, ch ResponseChan) os.Error {
	return imap.Send(fmt.Sprintf("LIST %s %s", quote(reference), quote(name)), ch)
}

func (imap *IMAP) Examine(mailbox string, ch ResponseChan) os.Error {
	return imap.Send(fmt.Sprintf("EXAMINE %s", quote(mailbox)), ch)
}

func (imap *IMAP) StartLoops() {
	go func() {
		err := imap.ReadLoop()
		panic(err)
	}()
}

func (imap *IMAP) ReadLoop() os.Error {
	for {
		tag, text, err := imap.ReadLine()
		if err != nil {
			return err
		}
		text = text

		if tag == Untagged {
			resp, err := ParseResponse(text)
			if err != nil {
				return err
			}
			log.Printf("%v", resp)
		} else {
			status, text, err := ParseStatus(text)
			if err != nil {
				return err
			}

			imap.lock.Lock()
			ch := imap.pending[tag]
			imap.pending[tag] = nil, false
			imap.lock.Unlock()

			if ch != nil {
				ch <- &Response{status, text}
			}
		}
	}
	return nil
}

func ParseStatus(text string) (Status, string, os.Error) {
	// TODO: response code
	codes := map[string]Status{
		"OK": OK,
		"NO": NO,
		"BAD": BAD,
	}
	code, text := splitToken(text)

	status, known := codes[code]
	if !known {
		return BAD, "", fmt.Errorf("unexpected status %q", code)
	}
	return status, text, nil
}

type Capabilities struct {
	caps []string
}

type List struct {
	inferiors TriBool
	selectable TriBool
	marked TriBool
	children TriBool
	delim string
	mailbox string
}

type Flags struct {
	flags []string
}

type Exists struct {
	count int
}
type Recent struct {
	count int
}

func ParseResponse(origtext string) (interface{}, os.Error) {
	command, text := splitToken(origtext)
	switch command {
	case "CAPABILITY":
		caps := strings.Split(text, " ")
		return &Capabilities{caps}, nil
	case "LIST":
		// "(" [mbx-list-flags] ")" SP (DQUOTE QUOTED-CHAR DQUOTE / nil) SP mailbox
		p := newParser(text)
		flags, err := p.parseParenList()
		if err != nil {
			return nil, err
		}
		p.expect(" ")

		delim, err := p.parseString()
		if err != nil {
			return nil, err
		}
		p.expect(" ")

		mailbox, err := p.parseString()
		if err != nil {
			return nil, err
		}

		err = p.expectEOF()
		if err != nil {
			return nil, err
		}

		list := &List{delim:delim, mailbox:mailbox}
		for _, flag := range flags {
			switch flag {
			case "\\Noinferiors":
				list.inferiors = TriFalse
			case "\\Noselect":
				list.selectable = TriFalse
			case "\\Marked":
				list.marked = TriTrue
			case "\\Unmarked":
				list.marked = TriFalse
			case "\\HasChildren":
				list.children = TriTrue
			case "\\HasNoChildren":
				list.children = TriFalse
			default:
				return nil, fmt.Errorf("unknown list flag %q", flag)
			}
		}
		return list, nil

	case "FLAGS":
		p := newParser(text)
		flags, err := p.parseParenList()
		if err != nil {
			return nil, err
		}
		err = p.expectEOF()
		if err != nil {
			return nil, err
		}

		return &Flags{flags}, nil

	case "OK", "NO", "BAD":
		status, text, err := ParseStatus(origtext)
		if err != nil {
			return nil, err
		}
		return &Response{status, text}, nil
	}

	num, err := strconv.Atoi(command)
	if err == nil {
		command, _ := splitToken(text)
		switch command {
		case "EXISTS":
			return &Exists{num}, nil
		case "RECENT":
			return &Recent{num}, nil
		}
	}

	return nil, fmt.Errorf("unhandled untagged response %s", text)
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

func main() {
	user, pass := loadAuth("auth")

	imap := NewIMAP()
	imap.protoLog = log.New(os.Stderr, "proto ", log.Ltime)

	text, err := imap.Connect("imap.gmail.com:993")
	check(err)
	log.Printf("connected %q", text)

	ch := make(chan *Response, 1)

	err = imap.Auth(user, pass, ch)
	check(err)
	log.Printf("%v", <-ch)

/*
	err = imap.List("", WildcardAny, ch)
	check(err)
	log.Printf("%v", <-ch)
*/

	err = imap.Examine("lkml", ch)
	check(err)
	log.Printf("%v", <-ch)

	log.Printf("done")
}
