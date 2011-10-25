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
	if tag != rt.expectedTag {
		t.Fatalf("expected %v, got %v", rt.expectedTag, tag)
	}
	if !reflect.DeepEqual(resp, rt.expectedResponse) {
		t.Fatalf("DeepEqual(%#v, %#v)", resp, rt.expectedResponse)
	}
}


func TestProtocol(t *testing.T) {
	tests := []readerTest{
		readerTest{
			"* OK [PERMANENTFLAGS ()] Flags permitted.\r\n",
			untagged,
			&Response{
				status: OK,
				code: &ResponsePermanentFlags{[]string{}},
				text: "Flags permitted.",
			},
		},
		readerTest{
			"* OK [UIDVALIDITY 2] UIDs valid.\r\n",
			untagged,
			&Response{
				status: OK,
				code: &ResponseUIDValidity{2},
				text: "UIDs valid.",
			},
		},
		readerTest{
			"* OK [UIDNEXT 31677] Predicted next UID.\r\n",
			untagged,
			&Response{
				status: OK,
				code: "UIDNEXT 31677",
				text: "Predicted next UID.",
			},
		},
		readerTest{
			"a2 OK [READ-ONLY] INBOX selected. (Success)\r\n",
			tag(2),
			&Response{
				status: OK,
				code:"READ-ONLY",
				text:"INBOX selected. (Success)",
			},
		},
	}

	for _, test := range tests {
		test.Run(t)
	}
}
