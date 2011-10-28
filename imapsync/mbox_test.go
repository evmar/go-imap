package main

import (
	"bytes"
	"testing"
)

type fromEncodingTest struct {
	input, expected string
}

func (f *fromEncodingTest) Run(t *testing.T) {
	buf := bytes.NewBuffer(make([]byte, 0, 100))
	w := &fromEncodingWriter{buf}

	n, err := w.Write([]byte(f.input))
	if err != nil {
		t.Fatalf("%s", err)
	}
	if n < len(f.input) {
		t.Fatalf("%#v: expected write %d bytes, wrote %d", f, len(f.input), n)
	}
	if !bytes.Equal(buf.Bytes(), []byte(f.expected)) {
		t.Fatalf("expected %q, got %q", f.expected, buf.Bytes())
	}
}

func TestFromEncoding(t *testing.T) {
	tests := []fromEncodingTest{
		fromEncodingTest{
			input: "foo bar",
			expected: "foo bar",
		},
		fromEncodingTest{
			input: "foo\nbar",
			expected: "foo\nbar",
		},
		fromEncodingTest{
			input: "foo\nFrom bar\n",
			expected: "foo\n>From bar\n",
		},
		fromEncodingTest{
			input: "From bar\n",
			expected: ">From bar\n",
		},
		fromEncodingTest{
			input: ">From bar\n",
			expected: ">>From bar\n",
		},
		fromEncodingTest{
			input: "Foo\n> From bar\n> >From baz",
			expected: "Foo\n> From bar\n> >From baz",
		},
	}
	for _, test := range tests {
		test.Run(t)
	}
}
