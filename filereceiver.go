package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cenkalti/log"
	"github.com/getsentry/raven-go"
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
	raven.CaptureError(err, nil, &raven.Message{Message: message})
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
		fi, err := ReadFileInfo(path)
		if err != nil {
			f.internalServerError("cannot get offset", err, r, w)
			return
		}
		w.Header().Set("efes-file-offset", strconv.FormatInt(fi.Offset, 10))
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
		newOffset, digest, err := saveFile(path, offset, length, r.Body, f.log)
		if oerr, ok := err.(*OffsetMismatchError); ok {
			// Cannot use http.Error() because we want to set "efes-file-offset" header.
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("efes-file-offset", strconv.FormatInt(oerr.Required, 10))
			w.WriteHeader(http.StatusConflict)
			_, _ = fmt.Fprint(w, oerr.Error()) // nolint: gas
			return
		}
		if err != nil {
			f.internalServerError("cannot save file", err, r, w)
			return
		}
		if digest != nil {
			w.Header().Set("efes-file-sha1", hex.EncodeToString(digest.Sha1.Sum(nil)))
			w.Header().Set("efes-file-crc32", hex.EncodeToString(digest.CRC32.Sum(nil)))
		}
		w.Header().Set("efes-file-offset", strconv.FormatInt(newOffset, 10))
	case http.MethodDelete:
		fi, err := ReadFileInfo(path)
		if os.IsNotExist(err) {
			http.Error(w, "offset file does not exist", http.StatusNotFound)
			return
		}
		if err != nil {
			f.internalServerError("cannot read offset file", err, r, w)
			return
		}
		w.Header().Set("efes-file-sha1", hex.EncodeToString(fi.Digest.Sha1.Sum(nil)))
		w.Header().Set("efes-file-crc32", hex.EncodeToString(fi.Digest.CRC32.Sum(nil)))
		err = DeleteFileInfo(path)
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
	return SaveFileInfo(path, newFileInfo())
}

func saveFile(path string, offset int64, length int64, r io.Reader, log log.Logger) (int64, *Digest, error) {
	var fi *FileInfo
	var err error
	if offset == 0 {
		// File can be saved without a prior POST for creating offset file.
		err = createFile(path)
		if err != nil {
			return 0, nil, err
		}
		fi = newFileInfo()
	} else {
		fi, err = ReadFileInfo(path)
		if err != nil {
			return 0, nil, err
		}
		if offset != fi.Offset {
			return 0, nil, &OffsetMismatchError{offset, fi.Offset}
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		return 0, nil, err
	}
	_, err = f.Seek(offset, io.SeekStart)
	if err != nil {
		logCloseFile(log, f)
		return 0, nil, err
	}
	w := io.MultiWriter(f, fi.Digest.CRC32, fi.Digest.Sha1)
	n, _ := io.Copy(w, r)
	err = f.Sync()
	if err != nil {
		return 0, nil, err
	}
	err = f.Close()
	if err != nil {
		return 0, nil, err
	}
	fi.Offset = offset + n
	if fi.Offset == length {
		// If we know the length of the file, we can delete the ".offset"
		// file without the need of a separate DELETE from the client.
		err = DeleteFileInfo(path)
		return fi.Offset, &fi.Digest, err
	}
	return fi.Offset, nil, SaveFileInfo(path, fi)
}

// OffsetMismatchError is returned when the offset specified in request does not match the actual offset on the disk.
type OffsetMismatchError struct {
	Given, Required int64
}

func (e *OffsetMismatchError) Error() string {
	return fmt.Sprintf("given offset (%d) does not match required offset (%d)", e.Given, e.Required)
}
