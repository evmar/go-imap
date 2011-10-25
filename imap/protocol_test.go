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
			"* OK Gimap ready for requests from 12.34 u6if.369\r\n",
			untagged,
			&ResponseStatus{
				status: OK,
				text: "Gimap ready for requests from 12.34 u6if.369",
			},
		},
		readerTest{
			"* OK [PERMANENTFLAGS ()] Flags permitted.\r\n",
			untagged,
			&ResponsePermanentFlags{[]string{}},
		},
		readerTest{
			"* OK [UIDVALIDITY 2] UIDs valid.\r\n",
			untagged,
			&ResponseUIDValidity{2},
		},
		readerTest{
			"* OK [UIDNEXT 31677] Predicted next UID.\r\n",
			untagged,
			&ResponseUIDNext{31677},
		},
		readerTest{
			"a2 OK [READ-ONLY] INBOX selected. (Success)\r\n",
			tag(2),
			&ResponseStatus{
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
