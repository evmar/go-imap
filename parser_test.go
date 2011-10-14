package main

import (
	"testing"
)

func TestParse(t *testing.T) {
	input := "(\\HasNoChildren) \"/\" \"INBOX\""
	p := newParser(input)
	ps, err := p.parseParenList()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0] != "\\HasNoChildren" {
		t.Fatalf("parenlist")
	}

	if !p.expect(" ") {
		t.FailNow()
	}

	d, err := p.parseString()
	if err != nil {
		t.Fatal(err)
	}
	if d != "/" {
		t.FailNow()
	}

	if !p.expect(" ") {
		t.FailNow()
	}

	box, err := p.parseString()
	if err != nil {
		t.Fatal(err)
	}
	if box != "INBOX" {
		t.FailNow()
	}
}
