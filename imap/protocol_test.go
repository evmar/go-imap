package imap

import (
	"bytes"
	"testing"
)

func serverResponse(text string) *reader {
	r := bytes.NewBufferString(text)
	return &reader{newParser(r)}
}

func TestStatus(t *testing.T) {
	r := serverResponse("* OK [PERMANENTFLAGS ()] Flags permitted.\r\n")
	_, _, err := r.readOne()
	check(err)
}
