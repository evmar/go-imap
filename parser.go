package main

import (
	"log"
)

func init() {
	log.SetFlags(log.Ltime | log.Lshortfile)
}


type Parser struct {
	input string
	cur int
}

func NewParser(input string) *Parser {
	return &Parser{input, 0}
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

func (p *Parser) parseParenList() []string {
	if !p.expect("(") {
		return nil
	}

	atoms := make([]string, 0, 4)
	for {
		atom := p.parseAtom()
		if len(atom) == 0 {
			break
		}

		atoms = append(atoms, atom)

		if p.expect(")") {
			break
		}
		if !p.expect(" ") {
			return nil
		}
	}
	return atoms
}

func (p *Parser) parseString() (parsedString string, ok bool) {
	if !p.expect("\"") {
		return
	}
	i := p.cur
	for ; i < len(p.input); i++ {
		if p.input[i] == '"' {
			break
		}
		if p.input[i] == '\\' {
			i++
			if p.input[i] != '"' && p.input[i] != '\\' {
				return
			}
		}
	}
	str := p.input[p.cur:i]
	p.cur = i
	if !p.expect("\"") {
		return
	}

	return str, true
}
