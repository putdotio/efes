package main

import (
	"io"
	"sync/atomic"
)

// ReadCounter is counter for io.Reader.
type ReadCounter struct {
	r     io.Reader
	count int64
}

func newReadCounter(r io.Reader) *ReadCounter {
	return &ReadCounter{r: r}
}

func (c *ReadCounter) Read(buf []byte) (int, error) {
	n, err := c.r.Read(buf)
	atomic.AddInt64(&c.count, int64(n))
	return n, err
}

// Count function returns counted bytes.
func (c *ReadCounter) Count() int64 {
	return atomic.LoadInt64(&c.count)
}
