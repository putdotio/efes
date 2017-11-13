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
	tempdir, err = ioutil.TempDir("", "storage-file-receiver-test")
	if err != nil {
		t.Fatal(err)
	}
	fr = newFileReceiver(tempdir, log.DefaultLogger)
}

func tearDown() {
	os.RemoveAll(tempdir)
}

func TestFileReceiver(t *testing.T) {
	setup(t)
	defer tearDown()

	testOffset(t, 404, false, 0)
	testCreate(t)
	testOffset(t, 200, true, 0)
	testSend(t, 0, false, "foo")
	testOffset(t, 200, true, 3)
	testSend(t, 3, false, "bar")
	testOffset(t, 200, true, 6)
	testDelete(t)
	testOffset(t, 404, false, 0)
}

func TestFileReceiverNoCreate(t *testing.T) {
	setup(t)
	defer tearDown()

	testSend(t, 0, false, "baz")
	testOffset(t, 200, true, 3)
	testDelete(t)
	testOffset(t, 404, false, 0)
}

func TestFileReceiverNoDelete(t *testing.T) {
	setup(t)
	defer tearDown()

	testCreate(t)
	testSend(t, 0, true, "baz")
	testOffset(t, 404, false, 0)
}

func TestFileReceiverZeroByte(t *testing.T) {
	setup(t)
	defer tearDown()

	testCreate(t)
	testSend(t, 0, false, "")
	testOffset(t, 200, true, 0)
	testDelete(t)
}

func TestFileReceiverSingleRequest(t *testing.T) {
	setup(t)
	defer tearDown()

	testSend(t, 0, true, "foo")
	testOffset(t, 404, false, 0)
}

func TestFileReceiverInvalidOffset(t *testing.T) {
	setup(t)
	defer tearDown()

	req, err := http.NewRequest("PATCH", testPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("storage-file-offset", "1")
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusPreconditionFailed {
		t.Logf("handler returned wrong status code: got %v want %v", status, http.StatusPreconditionFailed)
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
	req.Header.Set("storage-file-offset", "6")
	rr := httptest.NewRecorder()
	fr.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

func testOffset(t *testing.T, statusCode int, checkValue bool, value int64) {
	t.Helper()
	req, err := http.NewRequest("HEAD", testPath, nil)
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

func testSend(t *testing.T, offset int, sendLength bool, data string) {
	t.Helper()
	b := bytes.NewBufferString(data)
	req, err := http.NewRequest("PATCH", testPath, b)
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
