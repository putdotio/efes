package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cenkalti/log"
)

var (
	errOffset   = errors.New("offset mismatch")
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
		w.Header().Set("storage-file-offset", strconv.FormatInt(offset, 10))
	case http.MethodPatch:
		offset, err := strconv.ParseInt(r.Header.Get("storage-file-offset"), 10, 64)
		if err != nil {
			http.Error(w, "invalid header: storage-file-offset", http.StatusBadRequest)
			return
		}
		var length int64 = -1
		lengthHeader := r.Header.Get("storage-file-length")
		if lengthHeader != "" {
			length, err = strconv.ParseInt(lengthHeader, 10, 64)
			if err != nil {
				http.Error(w, "invalid header: storage-file-length", http.StatusBadRequest)
				return
			}
		}
		err = saveFile(path, offset, length, r.Body)
		if err == errOffset || err == errNotExist {
			http.Error(w, "offset mismatch", http.StatusPreconditionFailed)
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
		err = os.MkdirAll(filepath.Dir(path), 0777)
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
		if err != nil {
			return err
		}
		if offset != fileOffset {
			return errOffset
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, r)
	newOffset := offset + n
	if err == nil && newOffset == length {
		// If we know the length of the file, we can delete the ".offset"
		// file without the need of a seperate DELETE from the client.
		err = deleteOffset(path)
	} else {
		saveOffset(path, newOffset)
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