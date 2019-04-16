package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

// type digest struct {
//         h   [5]uint32
//         x   [chunk]byte
//         nx  int
//         len uint64
// }

const (
	sha1digestSize = 100
	MaxUint        = ^uint(0)
	MaxInt         = int(MaxUint >> 1)
)

var errInvalidDigest = errors.New("invalid digest")

func (d *sha1digest) MarshalText() ([]byte, error) { // nolint: unparam
	b := bytes.NewBuffer(make([]byte, 0, sha1digestSize))
	binary.Write(b, binary.BigEndian, d.h[0]) // nolint: errcheck
	binary.Write(b, binary.BigEndian, d.h[1]) // nolint: errcheck
	binary.Write(b, binary.BigEndian, d.h[2]) // nolint: errcheck
	binary.Write(b, binary.BigEndian, d.h[3]) // nolint: errcheck
	binary.Write(b, binary.BigEndian, d.h[4]) // nolint: errcheck
	b.Write(d.x[:])
	binary.Write(b, binary.BigEndian, int64(d.nx)) // nolint: errcheck
	binary.Write(b, binary.BigEndian, d.len)       // nolint: errcheck
	ret := make([]byte, hex.EncodedLen(sha1digestSize))
	hex.Encode(ret, b.Bytes())
	return ret, nil
}

func (d *sha1digest) UnmarshalText(text []byte) error {
	if len(text) != hex.EncodedLen(sha1digestSize) {
		return errInvalidDigest
	}
	b := make([]byte, sha1digestSize)
	_, err := hex.Decode(b, text)
	if err != nil {
		return errInvalidDigest
	}
	r := bytes.NewReader(b)
	binary.Read(r, binary.BigEndian, &d.h[0]) // nolint: errcheck
	binary.Read(r, binary.BigEndian, &d.h[1]) // nolint: errcheck
	binary.Read(r, binary.BigEndian, &d.h[2]) // nolint: errcheck
	binary.Read(r, binary.BigEndian, &d.h[3]) // nolint: errcheck
	binary.Read(r, binary.BigEndian, &d.h[4]) // nolint: errcheck
	r.Read(d.x[:])                            // nolint: errcheck
	var nx int64
	binary.Read(r, binary.BigEndian, &nx) // nolint: errcheck
	if nx > int64(MaxInt) {
		return errInvalidDigest
	}
	d.nx = int(nx)
	binary.Read(r, binary.BigEndian, &d.len) // nolint: errcheck
	return nil
}
