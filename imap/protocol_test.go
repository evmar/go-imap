package imap

import (
	"bytes"
	"testing"
	"reflect"
)

type readerTest struct {
	input string
	expectedTag tag
	expectedResponse interface{}
}

func (rt readerTest) Run(t *testing.T) {
	r := &reader{newParser(bytes.NewBufferString(rt.input))}
	tag, resp, err := r.readResponse()
	check(err)
	if tag != untagged {
		t.FailNow()
	}
	if !reflect.DeepEqual(resp, rt.expectedResponse) {
		t.Fatalf("DeepEqual(%#v, %#v)", resp, rt.expectedResponse)
	}
}


func TestStatus(t *testing.T) {
	tests := []readerTest{
		readerTest{
			"* OK [PERMANENTFLAGS ()] Flags permitted.\r\n",
			untagged,
			&Response{
				status: OK,
				code: &ResponsePermanentFlags{[]string{}},
				text: "Flags permitted.",
				extra: nil,
			},
		},
		readerTest{
			"* OK [UIDVALIDITY 2] UIDs valid.\r\n",
			untagged,
			&Response{
				status: OK,
				code: &ResponseUIDValidity{2},
				text: "UIDs valid.",
				extra: nil,
			},
		},
		readerTest{
			"* OK [UIDNEXT 31677] Predicted next UID.\r\n",
			untagged,
			&Response{
				status: OK,
				code: "UIDNEXT 31677",
				text: "Predicted next UID.",
				extra: nil,
			},
		},
	}
	for _, test := range tests {
		test.Run(t)
	}
}
