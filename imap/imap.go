package imap

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

type IMAP struct {
	// Client thread.
	nextTag int

	Unsolicited chan interface{}

	// Background thread.
	r *reader
	w io.Writer

	pendingLock sync.Mutex
	pendingTag  tag
	pendingChan chan interface{}
}

func New(r io.Reader, w io.Writer) *IMAP {
	return &IMAP{
		r:       &reader{newParser(r)},
		w:       w,
	}
}

func (imap *IMAP) Start() (string, error) {
	tag, r, err := imap.r.readResponse()
	if err != nil {
		return "", err
	}
	if tag != untagged {
		return "", fmt.Errorf("expected untagged server hello. got %q", tag)
	}
	resp := r.(*ResponseStatus)
	if resp.status != OK {
		return "", &IMAPError{resp.status, resp.text}
	}

	go func() {
		err := imap.readLoop()
		// XXX what to do with an error on the read thread?
		panic(err)
	}()

	return resp.text, nil
}

func (imap *IMAP) Send(ch chan interface{}, format string, args ...interface{}) error {
	tag := tag(imap.nextTag)
	imap.nextTag++

	toSend := []byte(fmt.Sprintf("a%d %s\r\n", int(tag), fmt.Sprintf(format, args...)))

	if ch != nil {
		imap.pendingLock.Lock()
		imap.pendingTag = tag
		imap.pendingChan = ch
		imap.pendingLock.Unlock()
	}

	_, err := imap.w.Write(toSend)
	return err
}

func (imap *IMAP) SendSync(format string, args ...interface{}) (*ResponseStatus, error) {
	ch := make(chan interface{}, 1)
	err := imap.Send(ch, format, args...)
	if err != nil {
		return nil, err
	}

	var response *ResponseStatus
	extra := make([]interface{}, 0)
L:
	for {
		r := <-ch
		switch r := r.(type) {
		case *ResponseStatus:
			response = r
			break L
		default:
			extra = append(extra, r)
		}
	}

	if len(extra) > 0 {
		response.extra = extra
	}
	// XXX callers discard unsolicited responses if this is not OK
	if response.status != OK {
		return response, &IMAPError{response.status, response.text}
	}
	return response, nil
}

func (imap *IMAP) Auth(user string, pass string) (string, []string, error) {
	resp, err := imap.SendSync("LOGIN %s %s", user, pass)
	if err != nil {
		return "", nil, err
	}

	var caps []string
	for _, extra := range resp.extra {
		switch extra := extra.(type) {
		case *ResponseCapabilities:
			caps = extra.Capabilities
		default:
			imap.Unsolicited <- extra
		}
	}
	return resp.text, caps, nil
}

func quote(in string) string {
	if strings.IndexAny(in, "\r\n") >= 0 {
		panic("invalid characters in string to quote")
	}
	return "\"" + in + "\""
}

func (imap *IMAP) List(reference string, name string) ([]*ResponseList, error) {
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

// ResponseExamine contains the response to examining a mailbox.
type ResponseExamine struct {
	Flags          []string
	Exists         int
	Recent         int
	PermanentFlags []string
	UIDValidity    int
	UIDNext        int
}

func (imap *IMAP) Examine(mailbox string) (*ResponseExamine, error) {
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
			r.Flags = extra.Flags
		case (*ResponseExists):
			r.Exists = extra.Count
		case (*ResponseRecent):
			r.Recent = extra.Count
		// XXX unseen
		case (*ResponsePermanentFlags):
			r.PermanentFlags = extra.Flags
		case (*ResponseUIDNext):
			value := extra.Value
			r.UIDNext = value
		case (*ResponseUIDValidity):
			value := extra.Value
			r.UIDValidity = value
		default:
			imap.Unsolicited <- extra
		}
	}
	return r, nil
}

func formatFetch(sequence string, fields []string) string {
	var fieldsStr string
	if len(fields) == 1 {
		fieldsStr = fields[0]
	} else {
		fieldsStr = "(" + strings.Join(fields, " ") + ")"
	}
	return fmt.Sprintf("FETCH %s %s", sequence, fieldsStr)
}

func (imap *IMAP) Fetch(sequence string, fields []string) ([]*ResponseFetch, error) {
	resp, err := imap.SendSync("%s", formatFetch(sequence, fields))
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

func (imap *IMAP) FetchAsync(sequence string, fields []string) (chan interface{}, error) {
	ch := make(chan interface{})
	err := imap.Send(ch, formatFetch(sequence, fields))
	if err != nil {
		return nil, err
	}

	// Stream all responses to this message into outChan, and everything
	// else into unsolicited.
	outChan := make(chan interface{})
	go func() {
		for {
			r := <-ch
			switch r := r.(type) {
			case *ResponseFetch:
				outChan <- r
			case *ResponseStatus:
				outChan <- r
				return
			default:
				imap.Unsolicited <- r
			}
		}
	}()
	return outChan, nil
}

// Repeatedly reads messages off the connection and dispatches them.
func (imap *IMAP) readLoop() error {
	var msgChan chan interface{}
	for {
		tag, r, err := imap.r.readResponse()
		check(err)

		if msgChan == nil {
			imap.pendingLock.Lock()
			msgChan = imap.pendingChan
			imap.pendingLock.Unlock()
		}

		if tag == untagged {
			if msgChan != nil {
				msgChan <- r
			} else {
				imap.Unsolicited <- r
			}
		} else {
			resp := r.(*ResponseStatus)

			imap.pendingLock.Lock()
			if imap.pendingTag != tag {
				return fmt.Errorf("expected response tag %s, got %s", imap.pendingTag, tag)
			}
			imap.pendingChan = nil
			imap.pendingLock.Unlock()

			msgChan <- resp
			msgChan = nil
		}
	}
	panic("not reached")
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
