package main

import (
	"fmt"
	"log"
	"os"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}


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

func (p *Parser) parseAtom() string {
/*
ATOM-CHAR       = <any CHAR except atom-specials>

atom-specials   = "(" / ")" / "{" / SP / CTL / list-wildcards /
                  quoted-specials / resp-specials
*/
	i := p.cur
L:
	for ; i < len(p.input); i++ {
		switch p.input[i] {
		case '(', ')', '{', ' ': break L
		// XXX handle others.
		}
	}
	atom := p.input[p.cur:i]
	p.cur = i
	return atom
}

func (p *Parser) parseParenList() ([]string, os.Error) {
	if !p.expect("(") {
		return nil, p.error("expected '('")
	}

	atoms := make([]string, 0, 4)
	for {
		atom := p.parseAtom()
		if len(atom) == 0 {
			break
		}

		atoms = append(atoms, atom)

		if !p.expect(" ") {
			break
		}
	}

	if !p.expect(")") {
		return nil, p.error("expected ')'")
	}

	return atoms, nil
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
