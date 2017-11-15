package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cenkalti/log"
)

var (
	errNotExist = errors.New("not valid upload")
)

// FileReceiver implements http.Handler for receiving files from clients in chunks.
type FileReceiver struct {
	dir string
	log log.Logger
}

func newFileReceiver(dir string, logger log.Logger) *FileReceiver {
	return &FileReceiver{
		dir: dir,
		log: logger,
	}
}

func (f *FileReceiver) internalServerError(message string, err error, r *http.Request, w http.ResponseWriter) {
	message = message + ": " + err.Error()
	f.log.Error(message + "; " + r.URL.Path)
	http.Error(w, message, http.StatusInternalServerError)
}

func (f *FileReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := filepath.Join(f.dir, r.URL.Path)
	switch r.Method {
	case http.MethodPost:
		err := createFile(path)
		if err != nil {
			f.internalServerError("cannot create file", err, r, w)
			return
		}
	case http.MethodHead:
		offset, err := getOffset(path)
		if err == errNotExist {
			http.Error(w, "offset file does not exist", http.StatusNotFound)
			return
		}
		if err != nil {
			f.internalServerError("cannot get offset", err, r, w)
			return
		}
		w.Header().Set("efes-file-offset", strconv.FormatInt(offset, 10))
	case http.MethodPatch:
		offset, err := strconv.ParseInt(r.Header.Get("efes-file-offset"), 10, 64)
		if err != nil {
			http.Error(w, "invalid header: efes-file-offset", http.StatusBadRequest)
			return
		}
		var length int64 = -1
		lengthHeader := r.Header.Get("efes-file-length")
		if lengthHeader != "" {
			length, err = strconv.ParseInt(lengthHeader, 10, 64)
			if err != nil {
				http.Error(w, "invalid header: efes-file-length", http.StatusBadRequest)
				return
			}
		}
		err = saveFile(path, offset, length, r.Body)
		if oerr, ok := err.(*OffsetMismatchError); ok {
			// Cannot use http.Error() because we want to set "efes-file-offset" header.
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("efes-file-offset", strconv.FormatInt(oerr.Required, 10))
			w.WriteHeader(http.StatusConflict)
			fmt.Fprint(w, oerr.Error()) // nolint: gas
			return
		}
		if err != nil {
			f.internalServerError("cannot save file", err, r, w)
			return
		}
	case http.MethodDelete:
		err := deleteOffset(path)
		if err == errNotExist {
			http.Error(w, "offset file does not exist", http.StatusNotFound)
			return
		}
		if err != nil {
			f.internalServerError("cannot delete offset file", err, r, w)
			return
		}
	default:
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
}

func createFile(path string) error {
	f, err := os.Create(path)
	if os.IsNotExist(err) {
		err = os.MkdirAll(filepath.Dir(path), 0700)
		if err != nil {
			return err
		}
		f, err = os.Create(path)
	}
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	return saveOffset(path, 0)
}

func getOffset(path string) (int64, error) {
	b, err := ioutil.ReadFile(path + ".offset")
	if os.IsNotExist(err) {
		return 0, errNotExist
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(string(b), 10, 64)
}

func saveOffset(path string, offset int64) error {
	return ioutil.WriteFile(path+".offset", []byte(strconv.FormatInt(offset, 10)), 0666)
}

func saveFile(path string, offset int64, length int64, r io.Reader) error {
	if offset == 0 {
		// File can be saved without a prior POST for creating offset file.
		err := createFile(path)
		if err != nil {
			return err
		}
	} else {
		fileOffset, err := getOffset(path)
		if err == errNotExist {
			return &OffsetMismatchError{offset, 0}
		}
		if err != nil {
			return err
		}
		if offset != fileOffset {
			return &OffsetMismatchError{offset, fileOffset}
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		f.Close() // nolint: errcheck, gas
		return err
	}
	n, err := io.Copy(f, r)
	if err != nil {
		f.Close() // nolint: errcheck, gas
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	newOffset := offset + n
	if newOffset == length {
		// If we know the length of the file, we can delete the ".offset"
		// file without the need of a seperate DELETE from the client.
		err = deleteOffset(path)
	} else {
		err = saveOffset(path, newOffset)
	}
	return err
}

func deleteOffset(path string) error {
	err := os.Remove(path + ".offset")
	if os.IsNotExist(err) {
		return errNotExist
	}
	return err
}

// OffsetMismatchError is returned when the offset specified in request does not match the actual offset on the disk.
type OffsetMismatchError struct {
	Given, Required int64
}

func (e *OffsetMismatchError) Error() string {
	return fmt.Sprintf("given offset (%d) does not match required offset (%d)", e.Given, e.Required)
}
