package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// type digest struct {
//         h   [5]uint32
//         x   [chunk]byte
//         nx  int
//         len uint64
// }

var (
	digestsize       = 100
	errInvalidDigest = errors.New("invalid digest size")
)

func (d *digest) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, d.h[0])
	binary.Write(&b, binary.BigEndian, d.h[1])
	binary.Write(&b, binary.BigEndian, d.h[2])
	binary.Write(&b, binary.BigEndian, d.h[3])
	binary.Write(&b, binary.BigEndian, d.h[4])
	b.Write(d.x[:])
	binary.Write(&b, binary.BigEndian, int64(d.nx))
	binary.Write(&b, binary.BigEndian, d.len)
	if b.Len() != digestsize {
		return nil, errInvalidDigest
	}
	s := hex.EncodeToString(b.Bytes())
	return []byte(`"` + s + `"`), nil
}

func (d *digest) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	b, err = hex.DecodeString(s)
	if err != nil {
		return err
	}
	if len(b) != digestsize {
		return errInvalidDigest
	}
	r := bytes.NewReader(b)
	binary.Read(r, binary.BigEndian, &d.h[0])
	binary.Read(r, binary.BigEndian, &d.h[1])
	binary.Read(r, binary.BigEndian, &d.h[2])
	binary.Read(r, binary.BigEndian, &d.h[3])
	binary.Read(r, binary.BigEndian, &d.h[4])
	r.Read(d.x[:])
	var nx int64
	binary.Read(r, binary.BigEndian, &nx)
	d.nx = int(nx)
	binary.Read(r, binary.BigEndian, &d.len)
	return nil
}
