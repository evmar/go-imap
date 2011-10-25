package imap

import (
	"bytes"
	"testing"
	"reflect"
)

func serverResponse(text string) *reader {
	r := bytes.NewBufferString(text)
	return &reader{newParser(r)}
}

func TestStatus(t *testing.T) {
	r := serverResponse("* OK [PERMANENTFLAGS ()] Flags permitted.\r\n")
	tag, resp, err := r.readResponse()
	check(err)
	if tag != untagged {
		t.FailNow()
	}

	expected := &Response{
		status: OK,
		code: "PERMANENTFLAGS ()",
		text: "Flags permitted.",
		extra: nil,
	}
	if !reflect.DeepEqual(resp, expected) {
		t.Fatalf("DeepEqual(%#v, %#v)", resp, expected)
	}
}
