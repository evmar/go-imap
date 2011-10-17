package imap

import (
	"io"
	"log"
	"os"
)

var logger *log.Logger

type LoggingReader struct {
	r io.Reader
}

func (r *LoggingReader) Read(p []byte) (int, os.Error) {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.Ltime)
	}

	n, err := r.r.Read(p)
	if err != nil {
		return n, err
	}
	logger.Printf("<- %q", p[0:n])
	return n, err
}
