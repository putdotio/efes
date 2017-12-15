package main

import (
	"encoding/hex"
	"testing"
)

func TestCRC32PartialDigest(t *testing.T) {
	b := []byte("hello world")
	var d = NewCRC32IEEE().(*crc32digest)
	d.Write(b)
	hex1 := hex.EncodeToString(d.Sum(nil))

	b, err := d.MarshalText()
	if err != nil {
		t.Fatal(err)
	}

	var d2 = NewCRC32IEEE().(*crc32digest)
	err = d2.UnmarshalText(b)
	if err != nil {
		t.Fatal(err)
	}

	hex2 := hex.EncodeToString(d2.Sum(nil))
	if hex2 != hex1 {
		t.FailNow()
	}
}
