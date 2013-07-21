package main

import (
	"io"
	"log"
	"os"
)

var logger *log.Logger

type LoggingReader struct {
	r   io.Reader
	max int
}

func newLoggingReader(r io.Reader, max int) *LoggingReader {
	return &LoggingReader{r, max}
}

func (r *LoggingReader) Read(p []byte) (int, error) {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.Ltime)
	}

	n, err := r.r.Read(p)
	if err != nil {
		return n, err
	}

	if r.max > 0 && n > r.max {
		logger.Printf("<- %q...", p[0:r.max])
	} else {
		logger.Printf("<- %q", p[0:n])
	}
	return n, err
}
