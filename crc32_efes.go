package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
)

// type digest struct {
//         crc uint32
//         tab *Table
// }

const (
	crc32DigestSize = 4
)

func (d *crc32digest) MarshalText() ([]byte, error) { // nolint: unparam
	b := bytes.NewBuffer(make([]byte, 0, crc32DigestSize))
	binary.Write(b, binary.BigEndian, d.crc) // nolint: errcheck
	ret := make([]byte, hex.EncodedLen(crc32DigestSize))
	hex.Encode(ret, b.Bytes())
	return ret, nil
}

func (d *crc32digest) UnmarshalText(text []byte) error {
	if len(text) != hex.EncodedLen(crc32DigestSize) {
		return errInvalidDigest
	}
	b := make([]byte, crc32DigestSize)
	_, err := hex.Decode(b, text)
	if err != nil {
		return errInvalidDigest
	}
	r := bytes.NewReader(b)
	binary.Read(r, binary.BigEndian, &d.crc) // nolint: errcheck
	ieeeOnce.Do(ieeeInit)
	d.tab = IEEETable
	return nil
}
