package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"github.com/cenkalti/log"
)

const testPath = "/dir/file.txt"

var (
	tempdir string
	fr      *FileReceiver
)

func setup(t *testing.T) {
	var err error
	tempdir, err = ioutil.TempDir("", "efes-file-receiver-test")
	if err != nil {
		t.Fatal(err)
	}
	fr = newFileReceiver(tempdir, log.DefaultLogger, nil)
}

func tearDown() {
	os.RemoveAll(tempdir)
}

func TestFileReceiver(t *testing.T) {
	setup(t)
	defer tearDown()

	testOffset(t, 0)
	testCreate(t)
	testOffset(t, 0)
	testSend(t, 0, -1, "foo")
	testOffset(t, 3)
	testSend(t, 3, -1, "bar")
	testOffset(t, 6)
	testDelete(t)
	testOffset(t, 0)
}

func TestFileReceiverNoCreate(t *testing.T) {
	setup(t)
	defer tearDown()

	testSend(t, 0, -1, "baz")
	testOffset(t, 3)
	testDelete(t)
	testOffset(t, 0)
}

func TestFileReceiverNoDelete(t *testing.T) {
	setup(t)
	defer tearDown()

	testCreate(t)
	testSend(t, 0, 3, "baz")
	testOffset(t, 0)
}

func TestFileReceiverZeroByte(t *testing.T) {
	setup(t)
	defer tearDown()

	testCreate(t)
	testSend(t, 0, -1, "")
	testOffset(t, 0)
	testDelete(t)
}

func TestFileReceiverSingleRequest(t *testing.T) {
	setup(t)
	defer tearDown()

	testSend(t, 0, 3, "foo")
	testOffset(t, 0)
}

func TestFileReceiverInvalidOffset(t *testing.T) {
	setup(t)
	defer tearDown()

	req, err := http.NewRequest("PATCH", testPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("efes-file-offset", "1")
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusConflict {
		t.Logf("handler returned wrong status code: got %v want %v", status, http.StatusConflict)
		t.Fatal(rr.Body.String())
	}
}

func testCreate(t *testing.T) {
	t.Helper()
	req, err := http.NewRequest("POST", testPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func testDelete(t *testing.T) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, testPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func testOffset(t *testing.T, value int64) {
	t.Helper()
	req, err := http.NewRequest("HEAD", testPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	offset, err := strconv.ParseInt(rr.Header().Get("efes-file-offset"), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if offset != value {
		t.Fatalf("invalid offset: %d", offset)
	}
}

func testSend(t *testing.T, offset int, length int, data string) {
	t.Helper()
	b := bytes.NewBufferString(data)
	req, err := http.NewRequest("PATCH", testPath, b)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("efes-file-offset", strconv.Itoa(offset))
	if length >= 0 {
		req.Header.Set("efes-file-length", strconv.Itoa(length))
	}
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}
