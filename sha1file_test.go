package main

import (
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func TestSha1File(t *testing.T) {
	content := "the quick brown fox jumps over the lazy dog\n"
	expectedSha1 := "5d2781d78fa5a97b7bafa849fe933dfc9dc93eba"

	f, err := ioutil.TempFile("", "test-sha1-file-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	_, err = f.WriteString(content)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}

	sf := NewSha1File(f)
	b := make([]byte, len(content))

	n, err := sf.Read(b[:9])
	if err != nil {
		t.Fatal(err)
	}
	if n != 9 {
		t.Fatalf("invalid number of bytes read: %v", n)
	}
	if sf.position != 9 {
		t.Fatalf("invalid file position: %v", sf.position)
	}
	m, err := sf.Seek(3, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	if m != 3 {
		t.Fatalf("invalid seek position: %v", m)
	}
	n, err = sf.Read(b[3:])
	if err != nil {
		t.Fatal(err)
	}
	if n != len(content)-3 {
		t.Fatalf("invalid number of bytes read: %v", n)
	}

	if string(b) != content {
		t.Fatalf("invalid read: %v", string(b))
	}
	calculatedSha1 := hex.EncodeToString(sf.Sum(nil))
	if calculatedSha1 != expectedSha1 {
		t.Fatalf("invalid sha1: %v", calculatedSha1)
	}
}
