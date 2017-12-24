package main

import (
	"errors"
	"io"
)

// Sha1File calculates SHA-1 of the file as it is read.
type Sha1File struct {
	rs         io.ReadSeeker
	position   int64
	calculated int64
	digest     *sha1digest
}

func NewSha1File(rs io.ReadSeeker) *Sha1File {
	return &Sha1File{
		rs:     rs,
		digest: NewSha1(),
	}
}

func (f *Sha1File) Read(p []byte) (int, error) {
	if f.position > f.calculated {
		return 0, errors.New("missing data for sha1")
	}
	prev := f.position
	n, err := f.rs.Read(p)
	f.position += int64(n)
	c := p[f.calculated-prev : n]
	f.digest.Write(c) // nolint: errcheck
	f.calculated += int64(len(c))
	return n, err
}

func (f *Sha1File) Seek(offset int64, whence int) (int64, error) {
	newPosition, err := f.rs.Seek(offset, whence)
	if err != nil {
		return newPosition, err
	}
	if f.position < newPosition {
		return newPosition, errors.New("seeking forward is not supported")
	}
	f.position = newPosition
	return newPosition, nil
}

func (f *Sha1File) Sum(b []byte) []byte {
	return f.digest.Sum(b)
}
