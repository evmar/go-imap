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
	OK Status = iota
	NO
	BAD
)

func (s Status) String() string {
	return []string{
		"OK",
		"NO",
		"BAD",
	}[s]
}

const (
	WildcardAny          = "%"
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
	case TriTrue:
		return "true"
	case TriFalse:
		return "false"
	}
	return "unknown"
}

type Tag int

const Untagged = Tag(-1)

type Response struct {
	status Status
	text   string
}

type ResponseChan chan *Response

type IMAP struct {
	// Client thread.
	nextTag int

	responseData chan interface{}

	// Background thread.
	r        *textproto.Reader
	w        io.Writer
	protoLog *log.Logger

	lock    sync.Mutex
	pending map[Tag]chan *Response
}

func NewIMAP() *IMAP {
	return &IMAP{pending: make(map[Tag]chan *Response)}
}

func (imap *IMAP) Connect(hostport string) (string, os.Error) {
	conn, err := tls.Dial("tcp", hostport, nil)
	if err != nil {
		return "", err
	}

	imap.r = textproto.NewReader(bufio.NewReader(conn))
	imap.w = conn

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
		imap.protoLog.Printf("server<- %s...", toSend[0:min(len(command), 20)])
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

func (imap *IMAP) Fetch(sequence string, fields []string, ch ResponseChan) os.Error {
	var fieldsStr string
	if len(fields) == 1 {
		fieldsStr = fields[0]
	} else {
		fieldsStr = "\"" + strings.Join(fields, " ") + "\""
	}
	return imap.Send(fmt.Sprintf("FETCH %s %s", sequence, fieldsStr), ch)
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
			imap.responseData <- resp
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
		"OK":  OK,
		"NO":  NO,
		"BAD": BAD,
	}
	code, text := splitToken(text)

	status, known := codes[code]
	if !known {
		return BAD, "", fmt.Errorf("unexpected status %q", code)
	}
	return status, text, nil
}

type ResponseCapabilities struct {
	caps []string
}

type ResponseList struct {
	inferiors  TriBool
	selectable TriBool
	marked     TriBool
	children   TriBool
	delim      string
	mailbox    string
}

type ResponseFlags struct {
	flags []string
}

type ResponseExists struct {
	count int
}
type ResponseRecent struct {
	count int
}

type Address struct {
	name, source, address string
}

func (a *Address) FromSexp(s []Sexp) {
	if name := nilOrString(s[0]); name != nil {
		a.name = *name
	}
	if source := nilOrString(s[1]); source != nil {
		a.source = *source
	}
	mbox := nilOrString(s[2])
	host := nilOrString(s[3])
	if mbox != nil && host != nil {
		address := *mbox + "@" + *host
		a.address = address
	}
}
func AddressListFromSexp(s Sexp) []Address {
	if s == nil {
		return nil
	}

	saddrs := s.([]Sexp)
	addrs := make([]Address, len(saddrs))
	for i, s := range saddrs {
		addrs[i].FromSexp(s.([]Sexp))
	}
	return addrs
}

type ResponseFetchEnvelope struct {
	date, subject, inReplyTo, messageId *string
	from, sender, replyTo, to, cc, bcc  []Address
}

type ResponseFetch struct {
	msg          int
	flags        Sexp
	envelope     ResponseFetchEnvelope
	internalDate string
	size         int
}


func ParseResponse(origtext string) (resp interface{}, err os.Error) {
	defer func() {
		if e := recover(); e != nil {
			resp = nil
			err = e.(os.Error)
		}
	}()

	command, text := splitToken(origtext)
	switch command {
	case "CAPABILITY":
		caps := strings.Split(text, " ")
		return &ResponseCapabilities{caps}, nil
	case "LIST":
		// "(" [mbx-list-flags] ")" SP (DQUOTE QUOTED-CHAR DQUOTE / nil) SP mailbox
		p := newParser(text)
		flags, err := p.parseParenStringList()
		check(err)
		p.expect(" ")

		delim, err := p.parseQuoted()
		check(err)
		p.expect(" ")

		mailbox, err := p.parseQuoted()
		check(err)

		err = p.expectEOF()
		check(err)

		list := &ResponseList{delim: delim, mailbox: mailbox}
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
		flags, err := p.parseParenStringList()
		check(err)
		err = p.expectEOF()
		check(err)

		return &ResponseFlags{flags}, nil

	case "OK", "NO", "BAD":
		status, text, err := ParseStatus(origtext)
		check(err)
		return &Response{status, text}, nil
	}

	num, err := strconv.Atoi(command)
	if err == nil {
		command, text := splitToken(text)
		switch command {
		case "EXISTS":
			return &ResponseExists{num}, nil
		case "RECENT":
			return &ResponseRecent{num}, nil
		case "FETCH":
			p := newParser(text)
			sexp, err := p.parseSexp()
			check(err)
			if len(sexp)%2 != 0 {
				panic("fetch sexp must have even number of items")
			}
			fetch := &ResponseFetch{msg: num}
			for i := 0; i < len(sexp); i += 2 {
				key := sexp[i].(string)
				switch key {
				case "ENVELOPE":
					env := sexp[i+1].([]Sexp)
					log.Printf("env %+v", env)
					// This format is insane.
					if len(env) != 10 {
						return nil, fmt.Errorf("envelope needed 10 fields, had %d", len(env))
					}
					fetch.envelope.date = nilOrString(env[0])
					fetch.envelope.subject = nilOrString(env[1])
					fetch.envelope.from = AddressListFromSexp(env[2])
					fetch.envelope.sender = AddressListFromSexp(env[3])
					fetch.envelope.replyTo = AddressListFromSexp(env[4])
					fetch.envelope.to = AddressListFromSexp(env[5])
					fetch.envelope.cc = AddressListFromSexp(env[6])
					fetch.envelope.bcc = AddressListFromSexp(env[7])
					fetch.envelope.inReplyTo = nilOrString(env[8])
					fetch.envelope.messageId = nilOrString(env[9])
				case "FLAGS":
					fetch.flags = sexp[i+1]
				case "INTERNALDATE":
					fetch.internalDate = sexp[i+1].(string)
				case "RFC822.SIZE":
					fetch.size, err = strconv.Atoi(sexp[i+1].(string))
					if err != nil {
						return nil, err
					}
				default:
					panic(fmt.Sprintf("%#v", key))
				}
			}
			return fetch, nil
		}
	}

	return nil, fmt.Errorf("unhandled untagged response %s", text)
}
