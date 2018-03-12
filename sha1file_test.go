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

	testSeekAndRead(t, sf, 0, 9, content[:9])
	testSeekAndRead(t, sf, 2, 3, content[2:5])                // do not pass sf.calculated
	testSeekAndRead(t, sf, 2, 7, content[2:9])                // read exactly up to sf.calculated
	testSeekAndRead(t, sf, 2, 9, content[2:11])               // read beyond sf.calculated
	testSeekAndRead(t, sf, 11, len(content)-11, content[11:]) // read the rest (to the end)

	calculatedSha1 := hex.EncodeToString(sf.Sum(nil))
	if calculatedSha1 != expectedSha1 {
		t.Fatalf("invalid sha1: %v", calculatedSha1)
	}
}

func testSeekAndRead(t *testing.T, sf *Sha1File, seek int64, read int, expected string) {
	t.Helper()
	m, err := sf.Seek(seek, io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}
	if m != seek {
		t.Fatalf("invalid seek position: %v", m)
	}

	b := make([]byte, read)
	n, err := sf.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	if n != read {
		t.Fatalf("invalid number of bytes read: %v", n)
	}
	if string(b) != expected {
		t.Fatalf("invalid content: %s", string(b))
	}
}
