package main

import (
	"io"
	"sync"
)

// XXX status, like bytes read.

type netmonReader struct {
	r io.Reader

	lock     sync.Mutex
	bucket   int
	estimate float32
}

func newNetmonReader(r io.Reader) *netmonReader {
	return &netmonReader{r: r}
}

func (n *netmonReader) Tick() int {
	n.lock.Lock()
	var alpha float32 = 0.9
	bucket := n.bucket
	n.estimate = (alpha * float32(n.bucket)) + ((1 - alpha) * n.estimate)
	n.bucket = 0
	n.lock.Unlock()
	return bucket
}

func (n *netmonReader) Bandwidth() float32 {
	n.lock.Lock()
	val := n.estimate
	n.lock.Unlock()
	return val
}

func (n *netmonReader) Read(buf []byte) (int, error) {
	nb, err := n.r.Read(buf)
	n.lock.Lock()
	n.bucket += nb
	n.lock.Unlock()
	return nb, err
}
