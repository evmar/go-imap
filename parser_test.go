package main

import (
	"bytes"
	"reflect"
	"testing"
	"os"
	"fmt"
)

func testError(t *testing.T, err os.Error, format string, args ...interface{}) {
	if err != nil {
		t.Fatalf("%s: %s", fmt.Sprintf(format, args...), err)
	}
}

type parseTest struct {
	input string
	code func(p *Parser) (interface{}, os.Error)
	expected interface{}
}

func (test parseTest) Run(t *testing.T) {
	p := newParser(bytes.NewBufferString(test.input))
	ps, err := test.code(p)
	if err != nil {
		t.Fatalf("parsing %s: %s", test.input, err)
	}

	_, err = p.ReadByte()
	if err != nil {
		if err != os.EOF {
			t.Fatalf("parsing %s: %s", test.input, err)
		}
	} else {
		t.Fatalf("parsing %s: expected EOF", test.input)
	}

	if !reflect.DeepEqual(ps, test.expected) {
		t.Fatalf("DeepEqual(%#v, %#v)", ps, test.expected)
	}
}

func TestParseString(t *testing.T) {
	parseTest{
		input: "\"foo bar\"",
		code: func(p *Parser) (interface{}, os.Error) { return p.parseQuoted() },
		expected: "foo bar",
	}.Run(t)
}

func TestParseLiteral(t *testing.T) {
	tests := []parseTest{
		{
			input: "{5}\r\n01234",
			code: func(p *Parser) (interface{}, os.Error) { return p.parseLiteral() },
			expected: []byte("01234"),
		},

		{
			input: "({2}\r\nAB abc)",
			code: func(p *Parser) (interface{}, os.Error) { return p.parseSexp() },
			expected: []Sexp{[]byte("AB"), "abc"},
		},

	}

	for _, test := range tests {
		test.Run(t)
	}
}

func TestParseSimple(t *testing.T) {
	tests := []parseTest{
		{
			input: "(\\HasNoChildren \\Foo)",
			code: func(p *Parser) (interface{}, os.Error) {
				return p.parseParenStringList()
			},
			expected: []string{"\\HasNoChildren", "\\Foo"},
		},
	}

	for _, test := range tests {
		test.Run(t)
	}
}

func TestParseComplex(t *testing.T) {
	parseTest{
		input: `(ENVELOPE ("Fri, 14 Oct 2011 13:51:22 -0700" "Re: [PATCH 1/1] added code to export CAP_LAST_CAP in /proc/sys/kernel modeled after ngroups_max" (("Andrew Morton" NIL "akpm" "linux-foundation.org")) ((NIL NIL "linux-kernel-owner" "vger.kernel.org")) (("Andrew Morton" NIL "akpm" "linux-foundation.org")) (("Dan Ballard" NIL "dan" "mindstab.net")) (("Ingo Molnar" NIL "mingo" "elte.hu") ("Lennart Poettering" NIL "lennart" "poettering.net") ("Kay Sievers" NIL "kay.sievers" "vrfy.org") (NIL NIL "linux-kernel" "vger.kernel.org")) NIL "<1318460194-31983-1-git-send-email-dan@mindstab.net>" "<20111014135122.4bb95565.akpm@linux-foundation.org>") FLAGS () INTERNALDATE "14-Oct-2011 20:51:30 +0000" RFC822.SIZE 4623)`,
		code: func(p *Parser) (interface{}, os.Error) { return p.parseSexp() },

		expected: []Sexp{"ENVELOPE",
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
			"4623",
		},
	}.Run(t)
}
