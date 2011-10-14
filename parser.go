package main

import (
	"fmt"
	"log"
	"os"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}

type Sexp interface{}
// One of:
//   string
//   []Sexp
//   nil

type Parser struct {
	input string
	cur int
}

func newParser(input string) *Parser {
	return &Parser{input, 0}
}

func (p *Parser) error(text string) os.Error {
	return fmt.Errorf("parse error in %q near offset %d: %s", p.input, p.cur, text)
}

func (p *Parser) expect(text string) bool {
	if p.cur == len(p.input) {
		return false
	}
	if p.input[p.cur:p.cur+len(text)] != text {
		return false
	}
	p.cur += len(text)
	return true
}

func (p *Parser) expectEOF() os.Error {
	if p.cur != len(p.input) {
		return p.error("expected end of input")
	}
	return nil
}

func (p *Parser) parseAtom() (string, os.Error) {
/*
ATOM-CHAR       = <any CHAR except atom-specials>

atom-specials   = "(" / ")" / "{" / SP / CTL / list-wildcards /
                  quoted-specials / resp-specials
*/
	i := p.cur
L:
	for ; i < len(p.input); i++ {
		switch p.input[i] {
		case '(', ')', '{', ' ',
			// XXX: CTL
			'%', '*',  // list-wildcards
			'"':  // quoted-specials
			// XXX: note that I dropped '\' from the quoted-specials,
			// because it conflicts with parsing flags.  Who knows.
			// XXX: resp-specials
			break L
		}
	}

	if i == p.cur {
		return "", p.error("expected atom character")
	}

	atom := p.input[p.cur:i]
	p.cur = i
	return atom, nil
}

func (p *Parser) parseSexp() ([]Sexp, os.Error) {
	if !p.expect("(") {
		return nil, p.error("expected '('")
	}

	sexps := make([]Sexp, 0, 4)
L:
	for {
		if p.cur == len(p.input) {
			break
		}

		var exp Sexp
		var err os.Error
		switch p.input[p.cur] {
		case '(':
			exp, err = p.parseSexp()
		case '"':
			exp, err = p.parseString()
		case ')':
			break L
		default:
			// TODO: may need to distinguish atom from string in practice.
			exp, err = p.parseAtom()
			if exp == "NIL" {
				exp = nil
			}
		}
		if err != nil {
			return nil, err
		}
		sexps = append(sexps, exp)

		if !p.expect(" ") {
			break
		}
	}

	if !p.expect(")") {
		return nil, p.error("expected ')'")
	}

	return sexps, nil
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

func (p *Parser) parseString() (string, os.Error) {
	if !p.expect("\"") {
		return "", p.error("expected '\"'")
	}
	i := p.cur
	for ; i < len(p.input); i++ {
		if p.input[i] == '"' {
			break
		}
		if p.input[i] == '\\' {
			i++
			if p.input[i] != '"' && p.input[i] != '\\' {
				return "", p.error("expected special after backslash")
			}
		}
	}
	str := p.input[p.cur:i]
	p.cur = i
	if !p.expect("\"") {
		return "", p.error("expected '\"'")
	}

	return str, nil
}
