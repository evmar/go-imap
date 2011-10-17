package main

import (
	"bytes"
	"bufio"
	"fmt"
	"log"
	"os"
	"io"
	"strconv"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}

func recoverError(err *os.Error) {
	if e := recover(); e != nil {
		if osErr, ok := e.(os.Error); ok {
			*err = osErr
			return
		}
		panic(e)
	}
}

type Sexp interface{}
// One of:
//   string
//   []Sexp
//   nil
func nilOrString(s Sexp) *string {
	if s == nil {
		return nil
	}
	str := s.(string)
	return &str
}

type Parser struct {
	*bufio.Reader
}

func newParser(r io.Reader) *Parser {
	return &Parser{bufio.NewReader(r)}
}
func newParserString(s string) *Parser {
	return newParser(bytes.NewBufferString(s))
}

func (p *Parser) expect(text string) os.Error {
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

func (p *Parser) expectEOF() os.Error {
	_, err := p.ReadByte()
	if err != nil {
		if err == os.EOF {
			return nil
		}
		return err
	}
	return os.NewError("expected EOF")
}

func (p *Parser) parseAtom() (outStr string, outErr os.Error) {
/*
ATOM-CHAR       = <any CHAR except atom-specials>

atom-specials   = "(" / ")" / "{" / SP / CTL / list-wildcards /
                  quoted-specials / resp-specials
*/
	defer recoverError(&outErr)
	atom := bytes.NewBuffer(make([]byte, 0, 16))

L:
	for {
		c, err := p.ReadByte()
		check(err)

		switch c {
		case '(', ')', '{', ' ',
			// XXX: CTL
			'%', '*',  // list-wildcards
			'"':  // quoted-specials
			// XXX: note that I dropped '\' from the quoted-specials,
			// because it conflicts with parsing flags.  Who knows.
			// XXX: resp-specials
			err = p.UnreadByte()
			check(err)
			break L
		}

		atom.WriteByte(c)
	}

	return atom.String(), nil
}

func (p *Parser) parseLiteral() (literal []byte, outErr os.Error) {
/*
literal         = "{" number "}" CRLF *CHAR8
*/
	defer recoverError(&outErr)

	check(p.expect("{"))

	lengthBytes, err := p.ReadSlice('}')
	check(err)

	length, err := strconv.Atoi(string(lengthBytes[0:len(lengthBytes)-1]))
	check(err)

	err = p.expect("\r\n")
	check(err)

	literal = make([]byte, length)
	_, err = io.ReadFull(p, literal)
	check(err)

	return
}

func (p *Parser) parseSexp() (sexp []Sexp, outErr os.Error) {
	defer recoverError(&outErr)

	err := p.expect("(")
	check(err)

	sexps := make([]Sexp, 0, 4)
	for {
		c, err := p.ReadByte()
		check(err)

		var exp Sexp
		switch c {
		case ')':
			return sexps, nil
		case '(':
			p.UnreadByte()
			exp, err = p.parseSexp()
		case '"':
			p.UnreadByte()
			exp, err = p.parseQuoted()
		case '{':
			p.UnreadByte()
			exp, err = p.parseLiteral()
		default:
			// TODO: may need to distinguish atom from string in practice.
			p.UnreadByte()
			exp, err = p.parseAtom()
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

func (p *Parser) parseParenStringList() ([]string, os.Error) {
	sexp, err := p.parseSexp()
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

func (p *Parser) parseQuoted() (outStr string, outErr os.Error) {
	defer recoverError(&outErr)

	err := p.expect("\"")
	check(err)

	quoted := bytes.NewBuffer(make([]byte, 0, 16))

	for {
		c, err := p.ReadByte()
		check(err)
		switch (c) {
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
