package imap

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"errors"
	"strconv"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}

func recoverError(err *error) {
	if e := recover(); e != nil {
		if osErr, ok := e.(error); ok {
			*err = osErr
			return
		}
		panic(e)
	}
}

type sexp interface{}
// One of:
//   string
//   []sexp
//   nil
func nilOrString(s sexp) *string {
	if s == nil {
		return nil
	}
	str := s.(string)
	return &str
}

type parser struct {
	*bufio.Reader
}

func newParser(r io.Reader) *parser {
	return &parser{bufio.NewReader(r)}
}

func (p *parser) expect(text string) error {
	buf := make([]byte, len(text))

	_, err := io.ReadFull(p, buf)
	if err != nil {
		return err
	}

	if !bytes.Equal(buf, []byte(text)) {
		return fmt.Errorf("expected %q, got %q", text, buf)
	}

	return nil
}

func (p *parser) expectEOL() error {
	return p.expect("\r\n")
}

func (p *parser) readToken() (token string, outErr error) {
	defer recoverError(&outErr)

	buf := bytes.NewBuffer(make([]byte, 0, 16))
	for {
		c, err := p.ReadByte()
		check(err)
		switch c {
		case ' ':
			return buf.String(), nil
		case ']', '\r':
			check(p.UnreadByte())
			return buf.String(), nil
		}
		buf.WriteByte(c)
	}

	panic("not reached")
}

func (p *parser) readNumber() (num int, outErr error) {
	defer recoverError(&outErr)

	num = 0
	for {
		c, err := p.ReadByte()
		check(err)
		if c >= '0' && c <= '9' {
			num = num * 10 + int(c - '0')
		} else {
			check(p.UnreadByte())
			return num, nil
		}
	}

	panic("not reached")
}

func (p *parser) readAtom() (outStr string, outErr error) {
	/*
		ATOM-CHAR       = <any CHAR except atom-specials>

		atom-specials   = "(" / ")" / "{" / SP / CTL / list-wildcards /
		                  quoted-specials / resp-specials
	*/
	defer recoverError(&outErr)
	atom := bytes.NewBuffer(make([]byte, 0, 16))

	for {
		c, err := p.ReadByte()
		check(err)

		switch c {
		case '(', ')', '{', ' ',
			// XXX: CTL
			'%', '*', // list-wildcards
			'"': // quoted-specials
			// XXX: note that I dropped '\' from the quoted-specials,
			// because it conflicts with parsing flags.  Who knows.
			// XXX: resp-specials
			err = p.UnreadByte()
			check(err)
			return atom.String(), nil
		}

		atom.WriteByte(c)
	}

	panic("not reached")
}

func (p *parser) readQuoted() (outStr string, outErr error) {
	defer recoverError(&outErr)

	err := p.expect("\"")
	check(err)

	quoted := bytes.NewBuffer(make([]byte, 0, 16))

	for {
		c, err := p.ReadByte()
		check(err)
		switch c {
		case '\\':
			c, err = p.ReadByte()
			check(err)
			if c != '"' && c != '\\' {
				return "", fmt.Errorf("backslash-escaped %c", c)
			}
		case '"':
			return quoted.String(), nil
		}
		quoted.WriteByte(c)
	}

	panic("not reached")
}

func (p *parser) readLiteral() (literal []byte, outErr error) {
	/*
		literal         = "{" number "}" CRLF *CHAR8
	*/
	defer recoverError(&outErr)

	check(p.expect("{"))

	lengthBytes, err := p.ReadSlice('}')
	check(err)

	length, err := strconv.Atoi(string(lengthBytes[0 : len(lengthBytes)-1]))
	check(err)

	err = p.expect("\r\n")
	check(err)

	literal = make([]byte, length)
	_, err = io.ReadFull(p, literal)
	check(err)

	return
}

func (p *parser) readBracketed() (text string, outErr error) {
	defer recoverError(&outErr)

	check(p.expect("["))
	text, err := p.ReadString(']')
	check(err)
	text = text[0:len(text)-1]

	return text, nil
}

func (p *parser) readSexp() (s []sexp, outErr error) {
	defer recoverError(&outErr)

	err := p.expect("(")
	check(err)

	sexps := make([]sexp, 0, 4)
	for {
		c, err := p.ReadByte()
		check(err)

		var exp sexp
		switch c {
		case ')':
			return sexps, nil
		case '(':
			p.UnreadByte()
			exp, err = p.readSexp()
		case '"':
			p.UnreadByte()
			exp, err = p.readQuoted()
		case '{':
			p.UnreadByte()
			exp, err = p.readLiteral()
		default:
			// TODO: may need to distinguish atom from string in practice.
			p.UnreadByte()
			exp, err = p.readAtom()
			if exp == "NIL" {
				exp = nil
			}
		}
		check(err)

		sexps = append(sexps, exp)

		c, err = p.ReadByte()
		check(err)
		if c != ' ' {
			err = p.UnreadByte()
			check(err)
		}
	}

	panic("not reached")
}

func (p *parser) readParenStringList() ([]string, error) {
	sexp, err := p.readSexp()
	if err != nil {
		return nil, err
	}
	strs := make([]string, len(sexp))
	for i, s := range sexp {
		str, ok := s.(string)
		if !ok {
			return nil, fmt.Errorf("list element %d is %T, not string", i, s)
		}
		strs[i] = str
	}
	return strs, nil
}

func (p *parser) readToEOL() (string, error) {
	line, prefix, err := p.ReadLine()
	if err != nil {
		return "", err
	}
	if prefix {
		return "", errors.New("got line prefix, buffer too small")
	}
	return string(line), nil
}
