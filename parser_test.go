package main

import (
	"reflect"
	"testing"
)

func TestParseBasic(t *testing.T) {
	p := newParser("\"foo bar\"")
	s, err := p.parseQuoted()
	if err != nil {
		t.FailNow()
	}
	if s != "foo bar" {
		t.FailNow()
	}
}

func TestParseLiteral(t *testing.T) {
	input := "({10}\r\n0123456789 abc)"
	p := newParser(input)
	ps, err := p.parseSexp()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ps, []Sexp{"0123456789", "abc"}) {
		t.FailNow()
	}
}

func TestParseSimple(t *testing.T) {
	input := "(\\HasNoChildren) \"/\" \"INBOX\""
	p := newParser(input)
	ps, err := p.parseParenStringList()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0] != "\\HasNoChildren" {
		t.Fatalf("parenlist")
	}

	if !p.expect(" ") {
		t.FailNow()
	}

	d, err := p.parseQuoted()
	if err != nil {
		t.Fatal(err)
	}
	if d != "/" {
		t.FailNow()
	}

	if !p.expect(" ") {
		t.FailNow()
	}

	box, err := p.parseQuoted()
	if err != nil {
		t.Fatal(err)
	}
	if box != "INBOX" {
		t.FailNow()
	}

	err = p.expectEOF()
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseComplex(t *testing.T) {
	input := "(ENVELOPE (\"Fri, 14 Oct 2011 13:51:22 -0700\" \"Re: [PATCH 1/1] added code to export CAP_LAST_CAP in /proc/sys/kernel modeled after ngroups_max\" ((\"Andrew Morton\" NIL \"akpm\" \"linux-foundation.org\")) ((NIL NIL \"linux-kernel-owner\" \"vger.kernel.org\")) ((\"Andrew Morton\" NIL \"akpm\" \"linux-foundation.org\")) ((\"Dan Ballard\" NIL \"dan\" \"mindstab.net\")) ((\"Ingo Molnar\" NIL \"mingo\" \"elte.hu\") (\"Lennart Poettering\" NIL \"lennart\" \"poettering.net\") (\"Kay Sievers\" NIL \"kay.sievers\" \"vrfy.org\") (NIL NIL \"linux-kernel\" \"vger.kernel.org\")) NIL \"<1318460194-31983-1-git-send-email-dan@mindstab.net>\" \"<20111014135122.4bb95565.akpm@linux-foundation.org>\") FLAGS () INTERNALDATE \"14-Oct-2011 20:51:30 +0000\" RFC822.SIZE 4623)"
	p := newParser(input)
	d, err := p.parseSexp()
	if err != nil {
		t.Fatal(err)
	}

	// t.Logf("%#v", d)
	expected := []Sexp{"ENVELOPE",
		[]Sexp{"Fri, 14 Oct 2011 13:51:22 -0700",
			"Re: [PATCH 1/1] added code to export CAP_LAST_CAP in /proc/sys/kernel modeled after ngroups_max",
			[]Sexp{[]Sexp{"Andrew Morton", nil, "akpm", "linux-foundation.org"}},
			[]Sexp{[]Sexp{nil, nil, "linux-kernel-owner", "vger.kernel.org"}},
			[]Sexp{[]Sexp{"Andrew Morton", nil, "akpm", "linux-foundation.org"}},
			[]Sexp{[]Sexp{"Dan Ballard", nil, "dan", "mindstab.net"}},
			[]Sexp{[]Sexp{"Ingo Molnar", nil, "mingo", "elte.hu"}, []Sexp{"Lennart Poettering", nil, "lennart", "poettering.net"}, []Sexp{"Kay Sievers", nil, "kay.sievers", "vrfy.org"}, []Sexp{nil, nil, "linux-kernel", "vger.kernel.org"}},
			nil,
			"<1318460194-31983-1-git-send-email-dan@mindstab.net>",
			"<20111014135122.4bb95565.akpm@linux-foundation.org>"},
		"FLAGS",
		[]Sexp{},
		"INTERNALDATE",
		"14-Oct-2011 20:51:30 +0000",
		"RFC822.SIZE",
		"4623"}
	if !reflect.DeepEqual(d, expected) {
		t.FailNow()
	}

	err = p.expectEOF()
	if err != nil {
		t.Fatal(err)
	}
}
