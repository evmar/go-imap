package imap

import (
	"errors"
	"strconv"
	"fmt"
)

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

// ResponseStatus contains the status response of an OK/BAD/FAIL
// message.  (Messages of the form "OK [CODE HERE] ..." are parsed
// as specific other response types, like ResponseUIDValidity.)
type ResponseStatus struct {
	status Status
	code   interface{}
	text   string
	extra  []interface{}
}

func (r *ResponseStatus) String() string {
	return fmt.Sprintf("%s [%s] %s", r.status, r.code, r.text)
}

// IMAPError is an error returned for IMAP-level errors, such
// as "unknown mailbox".
type IMAPError struct {
	Status Status
	Text   string
}

func (e *IMAPError) Error() string {
	return fmt.Sprintf("imap: %s %s", e.Status, e.Text)
}

const (
	WildcardAny          = "%"
	WildcardAnyRecursive = "*"
)

type tag int

const untagged = tag(-1)

type reader struct {
	*parser
}

// Read a full response (e.g. "* OK foobar\r\n").
func (r *reader) readResponse() (tag, interface{}, error) {
	tag, err := r.readTag()
	if err != nil {
		return untagged, nil, err
	}

	if tag == untagged {
		resp, err := r.readUntagged()
		if err != nil {
			return untagged, nil, err
		}
		return tag, resp, nil
	} else {
		resp, err := r.readStatus("")
		if err != nil {
			return untagged, nil, err
		}
		return tag, resp, nil
	}

	panic("not reached")
}

// Read the tag, the first part of the response.
// Expects either "*" or "a123".
func (r *reader) readTag() (tag, error) {
	str, err := r.readToken()
	if err != nil {
		return untagged, err
	}
	if len(str) == 0 {
		return untagged, errors.New("read empty tag")
	}

	switch str[0] {
	case '*':
		return untagged, nil
	case 'a':
		tagnum, err := strconv.Atoi(str[1:])
		if err != nil {
			return untagged, err
		}
		return tag(tagnum), nil
	}

	return untagged, fmt.Errorf("unexpected response %q", str)
}

// ResponsePermanentFlags contains the flags the client can change
// permanently.
type ResponsePermanentFlags struct {
	Flags []string
}

// ResponseUIDValidity contains the unique identifier validity value.
// See RFC 3501 section 2.3.1.1.
type ResponseUIDValidity struct {
	Value int
}

// ResponseUIDNext contains the next message uid.
// See RFC 3501 section 2.3.1.1.
type ResponseUIDNext struct {
	Value int
}

// Read a status response, one starting with OK/NO/BAD.
func (r *reader) readStatus(statusStr string) (resp *ResponseStatus, outErr error) {
	defer func() {
		if e := recover(); e != nil {
			if osErr, ok := e.(error); ok {
				outErr = osErr
				return
			}
			panic(e)
		}
	}()

	if len(statusStr) == 0 {
		var err error
		statusStr, err = r.readToken()
		check(err)
	}

	statusStrs := map[string]Status{
		"OK":  OK,
		"NO":  NO,
		"BAD": BAD,
	}

	status, known := statusStrs[statusStr]
	if !known {
		panic(fmt.Errorf("unexpected status %q", statusStr))
	}

	peek, err := r.ReadByte()
	check(err)
	var code interface{}
	if peek != '[' {
		r.UnreadByte()
	} else {
		codeStr, err := r.readToken()
		check(err)

		switch codeStr {
		case "PERMANENTFLAGS":
			/* "PERMANENTFLAGS" SP "(" [flag-perm *(SP flag-perm)] ")" */
			flags, err := r.readParenStringList()
			check(err)
			code = &ResponsePermanentFlags{flags}
			check(r.expect("]"))
		case "UIDVALIDITY":
			num, err := r.readNumber()
			check(err)
			code = &ResponseUIDValidity{num}
			check(r.expect("]"))
		case "UIDNEXT":
			num, err := r.readNumber()
			check(err)
			code = &ResponseUIDNext{num}
			check(r.expect("]"))
		default:
			text, err := r.ReadString(']')
			check(err)
			if len(text) > 1 {
				code = codeStr + " " + text[0:len(text)-1]
			} else {
				code = codeStr
			}
		}

		/*
		 resp-text-code  = "ALERT" /
		 "BADCHARSET" [SP "(" astring *(SP astring) ")" ] /
		 capability-data / "PARSE" /
		 "PERMANENTFLAGS" SP "("
		 [flag-perm *(SP flag-perm)] ")" /
		 "READ-ONLY" / "READ-WRITE" / "TRYCREATE" /
		 "UIDNEXT" SP nz-number / "UIDVALIDITY" SP nz-number /
		 "UNSEEN" SP nz-number /
		 atom [SP 1*<any TEXT-CHAR except "]">]
		*/

		check(r.expect(" "))
	}

	rest, err := r.readToEOL()
	check(err)

	return &ResponseStatus{status, code, rest, nil}, nil
}

// ResponseCapabilities contains the server capability list from a
// CAPABILITIY message.
type ResponseCapabilities struct {
	Capabilities []string
}

func (r *reader) readCAPABILITY() *ResponseCapabilities {
	caps := make([]string, 0)
	for {
		cap, err := r.readToken()
		check(err)
		if len(cap) == 0 {
			break
		}
		caps = append(caps, cap)
	}
	check(r.expectEOL())
	return &ResponseCapabilities{caps}
}

// ResponseList contains the list metadata from a LIST message.
type ResponseList struct {
	Inferiors,
	Selectable,
	Marked,
	Children *bool
	Delim string
	Name  string
}

func (r *reader) readLIST() *ResponseList {
	// "(" [mbx-list-flags] ")" SP (DQUOTE QUOTED-CHAR DQUOTE / nil) SP mailbox
	flags, err := r.readParenStringList()
	check(err)
	r.expect(" ")

	delim, err := r.readQuoted()
	check(err)
	r.expect(" ")

	name, err := r.readQuoted()
	check(err)

	check(r.expectEOL())

	list := &ResponseList{Delim: string(delim), Name: string(name)}
	for _, flag := range flags {
		switch flag {
		case "\\Noinferiors":
			b := false
			list.Inferiors = &b
		case "\\Noselect":
			b := false
			list.Selectable = &b
		case "\\Marked":
			b := true
			list.Marked = &b
		case "\\Unmarked":
			b := false
			list.Marked = &b
		case "\\HasChildren":
			b := true
			list.Children = &b
		case "\\HasNoChildren":
			b := false
			list.Children = &b
		default:
			panic(fmt.Sprintf("unknown list flag %q", flag))
		}
	}
	return list
}

// ResponseFlags contains the mailbox flags from a FLAGS message.
type ResponseFlags struct {
	Flags []string
}

func (r *reader) readFLAGS() *ResponseFlags {
	flags, err := r.readParenStringList()
	check(err)
	check(r.expectEOL())
	return &ResponseFlags{flags}
}

// ResponseFetchEnvelope contains the broken-down message metadata
// retrieved when fetching the ENVELOPE data of a message.
type ResponseFetchEnvelope struct {
	date, subject, inReplyTo, messageId *string
	from, sender, replyTo, to, cc, bcc  []Address
}

// ResponseFetch contains the message data from a FETCH message.
type ResponseFetch struct {
	Msg                  int
	Flags                sexp
	Envelope             ResponseFetchEnvelope
	InternalDate         string
	Size                 int
	Rfc822, Rfc822Header []byte
}

func (r *reader) readFETCH(num int) *ResponseFetch {
	s, err := r.readSexp()
	check(err)
	if len(s)%2 != 0 {
		panic("fetch sexp must have even number of items")
	}
	fetch := &ResponseFetch{Msg: num}
	for i := 0; i < len(s); i += 2 {
		key := s[i].(string)
		switch key {
		case "ENVELOPE":
			env := s[i+1].([]sexp)
			// This format is insane.
			if len(env) != 10 {
				panic(fmt.Sprintf("envelope needed 10 fields, had %d", len(env)))
			}
			fetch.Envelope.date = nilOrString(env[0])
			fetch.Envelope.subject = nilOrString(env[1])
			fetch.Envelope.from = addressListFromSexp(env[2])
			fetch.Envelope.sender = addressListFromSexp(env[3])
			fetch.Envelope.replyTo = addressListFromSexp(env[4])
			fetch.Envelope.to = addressListFromSexp(env[5])
			fetch.Envelope.cc = addressListFromSexp(env[6])
			fetch.Envelope.bcc = addressListFromSexp(env[7])
			fetch.Envelope.inReplyTo = nilOrString(env[8])
			fetch.Envelope.messageId = nilOrString(env[9])
		case "FLAGS":
			fetch.Flags = s[i+1]
		case "INTERNALDATE":
			fetch.InternalDate = s[i+1].(string)
		case "RFC822":
			fetch.Rfc822 = s[i+1].([]byte)
		case "RFC822.HEADER":
			fetch.Rfc822Header = s[i+1].([]byte)
		case "RFC822.SIZE":
			fetch.Size, err = strconv.Atoi(s[i+1].(string))
			check(err)
		default:
			panic(fmt.Sprintf("unhandled fetch key %#v", key))
		}
	}
	check(r.expectEOL())
	return fetch
}

// ResponseExists contains the message count of a mailbox.
type ResponseExists struct {
	Count int
}

// ResponseRecent contains the number of messages with the recent
// flag set.
type ResponseRecent struct {
	Count int
}

func (r *reader) readUntagged() (resp interface{}, outErr error) {
	defer func() {
		if e := recover(); e != nil {
			if osErr, ok := e.(error); ok {
				outErr = osErr
				return
			}
			panic(e)
		}
	}()

	command, err := r.readToken()
	check(err)

	switch command {
	case "CAPABILITY":
		return r.readCAPABILITY(), nil
	case "LIST":
		return r.readLIST(), nil
	case "FLAGS":
		return r.readFLAGS(), nil
	case "OK", "NO", "BAD":
		resp, err := r.readStatus(command)
		check(err)
		if resp.code == nil {
			return resp, nil
		}
		switch resp.code.(type) {
		case string:
			// XXX write a parser for this code type.
			return resp, nil
		default:
			return resp.code, nil
		}
	}

	num, err := strconv.Atoi(command)
	if err == nil {
		command, err := r.readToken()
		check(err)

		switch command {
		case "EXISTS":
			check(r.expectEOL())
			return &ResponseExists{num}, nil
		case "RECENT":
			check(r.expectEOL())
			return &ResponseRecent{num}, nil
		case "FETCH":
			return r.readFETCH(num), nil
		}
	}

	return nil, fmt.Errorf("unhandled untagged response %s", command)
}
