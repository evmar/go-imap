package main

import (
	"bytes"
	"io"
	"os"
)

type fromEncodingWriter struct {
	w io.Writer
}

func (w *fromEncodingWriter) Write(buf []byte) (int, os.Error) {
	total := 0
	for len(buf) > 0 {
		// Insert a quote for the current line, if needed.
		ofs := 0
		for ; ofs < len(buf) && buf[ofs] == '>'; ofs++ {
			// iterate
		}
		magicFrom := []byte("From ")
		if ofs + len(magicFrom) <= len(buf) &&
			bytes.Equal(buf[ofs:ofs+len(magicFrom)], magicFrom) {
			_, err := w.w.Write([]byte(">"))
			if err != nil {
				return total, err
			}
		}

		// Find end of line.
		end := bytes.IndexByte(buf, '\n')
		if end < 0 {
			end = len(buf)
		} else {
			end++
		}

		// Write current line out and advance buffer.
		n, err := w.w.Write(buf[0:end])
		total += n
		if err != nil {
			return total, err
		}
		buf = buf[end:]
	}
	return total, nil
}

type mbox struct {
	io.Writer
}

func newMbox(w io.Writer) *mbox {
	return &mbox{w}
}

func (m *mbox) writeMessage(rfc822 []byte) os.Error {
	_, err := m.Write([]byte("From whatever\r\n"))
	if err != nil {
		return err
	}

	w := fromEncodingWriter{m}
	_, err = w.Write(rfc822)
	if err != nil {
		return err
	}

	_, err = m.Write([]byte("\r\n"))
	if err != nil {
		return err
	}

	return nil
}
