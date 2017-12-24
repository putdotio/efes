package main

import (
	"errors"
	"io"
)

var errNoSeek = errors.New("Seek on io.Reader is not supported")

type readNoSeeker struct {
	r      io.Reader
	offset int64
}

func NewReadNoSeeker(r io.Reader) io.ReadSeeker {
	return &readNoSeeker{r: r}
}

func (f *readNoSeeker) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	f.offset += int64(n)
	return n, err
}

func (f *readNoSeeker) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekStart && offset == f.offset {
		return offset, nil
	}
	return 0, errNoSeek
}
