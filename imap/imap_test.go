package imap

import (
	"bytes"
	"testing"
)

func serverResponse(text string) *IMAP {
	r := bytes.NewBufferString(text)
	return &IMAP{r:newParser(r)}
}

func TestStatus(t *testing.T) {
	imap := serverResponse("* OK [PERMANENTFLAGS ()] Flags permitted.\r\n")
	_, _, err := imap.readOne()
	check(err)
}
