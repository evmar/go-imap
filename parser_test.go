package main

import (
	"testing"
)

func TestParse(t *testing.T) {
	input := "(\\HasNoChildren) \"/\" \"INBOX\""
	p := NewParser(input)
	ps := p.parseParenList()
	if len(ps) != 1 || ps[0] != "\\HasNoChildren" {
		t.Fatalf("parenlist")
	}

	if !p.expect(" ") {
		t.FailNow()
	}

	d, ok := p.parseString()
	if !ok || d != "/" {
		t.FailNow()
	}

	if !p.expect(" ") {
		t.FailNow()
	}

	box, ok := p.parseString()
	if !ok || box != "INBOX" {
		t.FailNow()
	}
}
