package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
)

const path = "/dir/file.txt"

func TestFileReceiver(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "storage-file-receiver-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempdir)

	fr := newFileReceiver(tempdir)

	testOffset(t, fr, 404, false, 0)
	testCreate(t, fr)
	testOffset(t, fr, 200, true, 0)
	testSend(t, fr, 0, false, "foo")
	testOffset(t, fr, 200, true, 3)
	testSend(t, fr, 3, false, "bar")
	testOffset(t, fr, 200, true, 6)
	testDelete(t, fr)
	testOffset(t, fr, 404, false, 0)

	// patch without post
	testSend(t, fr, 0, false, "baz")
	testOffset(t, fr, 200, true, 3)
	testDelete(t, fr)
	testOffset(t, fr, 404, false, 0)

	// send file length
	testOffset(t, fr, 404, false, 0)
	testSend(t, fr, 0, true, "baz")
	testOffset(t, fr, 404, false, 0)

	// TODO test 0 byte files
}

func testCreate(t *testing.T, fr *FileReceiver) {
	req, err := http.NewRequest("POST", path, nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func testDelete(t *testing.T, fr *FileReceiver) {
	req, err := http.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("storage-file-offset", "6")
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func testOffset(t *testing.T, fr *FileReceiver, statusCode int, checkValue bool, value int64) {
	req, err := http.NewRequest("HEAD", path, nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != statusCode {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, statusCode)
	}
	if checkValue {
		offset, err := strconv.ParseInt(rr.Header().Get("storage-file-offset"), 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		if offset != value {
			t.Fatalf("invalid offset: %d", offset)
		}
	}
}

func testSend(t *testing.T, fr *FileReceiver, offset int, sendLength bool, data string) {
	b := bytes.NewBufferString(data)
	req, err := http.NewRequest("PATCH", path, b)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("storage-file-offset", strconv.Itoa(offset))
	if sendLength {
		req.Header.Set("storage-file-length", strconv.Itoa(len(data)))
	}
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}
