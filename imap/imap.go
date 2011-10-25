// Package imap implements an IMAP (RFC 3501) client.
package imap

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

func check(err os.Error) {
	if err != nil {
		panic(err)
	}
}

// Status represents server status codes which are returned by
// commands.
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

type Response struct {
	status Status
	code   string
	text   string
	extra  []interface{}
}

func (r *Response) String() string {
	return fmt.Sprintf("%s [%s] %s", r.status, r.code, r.text)
}

type IMAPError struct {
	status Status
	text   string
}

func (e *IMAPError) String() string {
	return fmt.Sprintf("%s %s", e.status, e.text)
}

const (
	WildcardAny          = "%"
	WildcardAnyRecursive = "*"
)

type tag int

const untagged = tag(-1)

type IMAP struct {
	// Client thread.
	nextTag int

	Unsolicited chan interface{}

	// Background thread.
	r *reader
	w io.Writer

	lock    sync.Mutex
	pending map[tag]chan *Response
}

func New(r io.Reader, w io.Writer) *IMAP {
	return &IMAP{
		r: &reader{newParser(r)},
		w: w,
		pending: make(map[tag]chan *Response),
	}
}

func (imap *IMAP) Start() (string, os.Error) {
	tag, r, err := imap.r.readResponse()
	if err != nil {
		return "", err
	}
	if tag != untagged {
		return "", fmt.Errorf("expected untagged server hello. got %q", tag)
	}
	resp := r.(*Response)
	if resp.status != OK {
		return "", &IMAPError{resp.status, resp.text}
	}

	imap.StartLoops()

	return resp.text, nil
}

func (imap *IMAP) Send(ch chan *Response, format string, args ...interface{}) os.Error {
	tag := tag(imap.nextTag)
	imap.nextTag++

	toSend := []byte(fmt.Sprintf("a%d %s\r\n", int(tag), fmt.Sprintf(format, args...)))

	if ch != nil {
		imap.lock.Lock()
		imap.pending[tag] = ch
		imap.lock.Unlock()
	}

	_, err := imap.w.Write(toSend)
	return err
}

func (imap *IMAP) SendSync(format string, args ...interface{}) (*Response, os.Error) {
	ch := make(chan *Response, 1)
	err := imap.Send(ch, format, args...)
	if err != nil {
		return nil, err
	}
	response := <-ch
	if response.status != OK {
		return nil, &IMAPError{response.status, response.text}
	}
	return response, nil
}

func (imap *IMAP) Auth(user string, pass string) (string, os.Error) {
	resp, err := imap.SendSync("LOGIN %s %s", user, pass)
	if err != nil {
		return "", err
	}
	for _, extra := range resp.extra {
		imap.Unsolicited <- extra
	}
	return resp.text, nil
}

func quote(in string) string {
	if strings.IndexAny(in, "\r\n") >= 0 {
		panic("invalid characters in string to quote")
	}
	return "\"" + in + "\""
}

func (imap *IMAP) List(reference string, name string) ([]*ResponseList, os.Error) {
	/* Responses:  untagged responses: LIST */
	response, err := imap.SendSync("LIST %s %s", quote(reference), quote(name))
	if err != nil {
		return nil, err
	}

	lists := make([]*ResponseList, 0)
	for _, extra := range response.extra {
		if list, ok := extra.(*ResponseList); ok {
			lists = append(lists, list)
		} else {
			imap.Unsolicited <- extra
		}
	}

	return lists, nil
}

type ResponseExamine struct {
	flags  []string
	exists int
	recent int
}

func (imap *IMAP) Examine(mailbox string) (*ResponseExamine, os.Error) {
	/*
	 Responses:  REQUIRED untagged responses: FLAGS, EXISTS, RECENT
	 REQUIRED OK untagged responses:  UNSEEN,  PERMANENTFLAGS,
	 UIDNEXT, UIDVALIDITY
	*/
	resp, err := imap.SendSync("EXAMINE %s", quote(mailbox))
	if err != nil {
		return nil, err
	}

	r := &ResponseExamine{}

	for _, extra := range resp.extra {
		switch extra := extra.(type) {
		case (*ResponseFlags):
			r.flags = extra.flags
		case (*ResponseExists):
			r.exists = extra.count
		case (*ResponseRecent):
			r.recent = extra.count
		//case (*Response):
		/*
		 // XXX parse tags
		*/
		default:
			imap.Unsolicited <- extra
		}
	}
	return r, nil
}

func (imap *IMAP) Fetch(sequence string, fields []string) ([]*ResponseFetch, os.Error) {
	var fieldsStr string
	if len(fields) == 1 {
		fieldsStr = fields[0]
	} else {
		fieldsStr = "(" + strings.Join(fields, " ") + ")"
	}
	resp, err := imap.SendSync("FETCH %s %s", sequence, fieldsStr)
	if err != nil {
		return nil, err
	}

	lists := make([]*ResponseFetch, 0)
	for _, extra := range resp.extra {
		if list, ok := extra.(*ResponseFetch); ok {
			lists = append(lists, list)
		} else {
			imap.Unsolicited <- extra
		}
	}
	return lists, nil
}

func (imap *IMAP) StartLoops() {
	go func() {
		err := imap.ReadLoop()
		panic(err)
	}()
}

func (imap *IMAP) ReadLoop() os.Error {
	var unsolicited []interface{}
	for {
		tag, r, err := imap.r.readResponse()
		check(err)
		if tag == untagged {
			if unsolicited == nil {
				imap.lock.Lock()
				hasPending := len(imap.pending) > 0
				imap.lock.Unlock()

				if hasPending {
					unsolicited = make([]interface{}, 0, 1)
				}
			}

			if unsolicited != nil {
				unsolicited = append(unsolicited, r)
			} else {
				imap.Unsolicited <- r
			}
		} else {
			resp := r.(*Response)
			resp.extra = unsolicited

			imap.lock.Lock()
			ch := imap.pending[tag]
			imap.pending[tag] = nil, false
			imap.lock.Unlock()

			ch <- resp
			unsolicited = nil
		}
	}
	panic("not reached")
}

type ResponseCapabilities struct {
	caps []string
}

type ResponseList struct {
	Inferiors,
	Selectable,
	Marked,
	Children *bool
	Delim string
	Name  string
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

func (a *Address) fromSexp(s []sexp) {
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
func addressListFromSexp(s sexp) []Address {
	if s == nil {
		return nil
	}

	saddrs := s.([]sexp)
	addrs := make([]Address, len(saddrs))
	for i, s := range saddrs {
		addrs[i].fromSexp(s.([]sexp))
	}
	return addrs
}

type ResponseFetchEnvelope struct {
	date, subject, inReplyTo, messageId *string
	from, sender, replyTo, to, cc, bcc  []Address
}

type ResponseFetch struct {
	Msg                  int
	Flags                sexp
	Envelope             ResponseFetchEnvelope
	InternalDate         string
	Size                 int
	Rfc822, Rfc822Header []byte
}

