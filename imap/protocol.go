package imap

import (
	"os"
	"strconv"
	"fmt"
)

type reader struct {
	*parser
}

func (r *reader) readResponse() (tag, interface{}, os.Error) {
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

func (r *reader) readTag() (tag, os.Error) {
	str, err := r.readToken()
	if err != nil {
		return untagged, err
	}
	if len(str) == 0 {
		return untagged, os.NewError("read empty tag")
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

func (r *reader) readStatus(statusStr string) (*Response, os.Error) {
	if len(statusStr) == 0 {
		var err os.Error
		statusStr, err = r.readToken()
		if err != nil {
			return nil, err
		}
	}

	statusStrs := map[string]Status{
		"OK":  OK,
		"NO":  NO,
		"BAD": BAD,
	}

	status, known := statusStrs[statusStr]
	if !known {
		return nil, fmt.Errorf("unexpected status %q", statusStr)
	}

	peek, err := r.Peek(1)
	if err != nil {
		return nil, err
	}
	var code string
	if peek[0] == '[' {
		code, err = r.readBracketed()
		if err != nil {
			return nil, err
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

		err = r.expect(" ")
		if err != nil {
			return nil, err
		}
	}

	rest, err := r.readToEOL()
	if err != nil {
		return nil, err
	}

	return &Response{status, code, rest, nil}, nil
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

func (r *reader) readFLAGS() *ResponseFlags {
	flags, err := r.readParenStringList()
	check(err)
	check(r.expectEOL())
	return &ResponseFlags{flags}
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

func (r *reader) readUntagged() (resp interface{}, outErr os.Error) {
	defer func() {
		if e := recover(); e != nil {
			if osErr, ok := e.(os.Error); ok {
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
		return resp, nil
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
