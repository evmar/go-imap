package main

import (
	"io"
)

type mbox struct {
	io.Writer
}

func newMbox(w io.Writer) *mbox {
	return &mbox{w}
}

func (m *mbox) writeMessage(rfc822 []byte) {
	m.Write([]byte("From whatever\r\n"))
	m.Write(rfc822)
	m.Write([]byte("\r\n"))
}
