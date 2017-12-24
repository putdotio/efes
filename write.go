package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/cenkalti/backoff"
)

func (c *Client) Write(key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer logCloseFile(c.log, f)
	return c.writeFile(key, f)
}

func (c *Client) writeFile(key string, f *os.File) error {
	size, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	return c.writeReadSeeker(key, f, size)
}

func (c *Client) WriteReader(key string, r io.Reader) error {
	return c.writeReadSeeker(key, NewReadNoSeeker(r), -1)
}

func (c *Client) writeReadSeeker(key string, rs io.ReadSeeker, size int64) error {
	path, fid, err := c.createOpen(size)
	if err != nil {
		return err
	}
	err = c.sendFile(path, rs, size)
	if err != nil {
		return err
	}
	return c.createClose(key, fid)
}

func (c *Client) sendFile(path string, rs io.ReadSeeker, size int64) error {
	sf := NewSha1File(rs)
	var r io.Reader = sf
	if c.config.Client.ShowProgress {
		p := newReadProgress(r, size)
		defer p.Close()
		r = p
	}
	var remoteSha1 []byte
	bo := backoff.NewExponentialBackOff()
	first := true
	op := func() error {
		var offset int64
		var err error
		if first {
			first = false
		} else {
			offset, err = c.getOffset(path)
			if err != nil {
				c.log.Errorf("cannot get offset for path [%s]: %s", path, err.Error())
				return err
			}
			_, err = rs.Seek(offset, io.SeekStart)
			if err != nil {
				c.log.Errorf("cannot seek file: %s", err.Error())
				return err
			}
		}
		hashes, err := c.send(path, r, offset, size, bo)
		if err != nil {
			return err
		}
		remoteSha1, err = hex.DecodeString(hashes["sha1"])
		return err
	}
	err := backoff.Retry(op, bo)
	if err != nil {
		return err
	}
	localSha1 := sf.Sum(nil)
	if !bytes.Equal(remoteSha1, localSha1) {
		return fmt.Errorf("local sha1 (%s) does not match remote sha1 (%s)", hex.EncodeToString(localSha1), hex.EncodeToString(remoteSha1))
	}
	return nil
}

// send a patch request until and error occurs or stream is finished
func (c *Client) send(path string, r io.Reader, offset, size int64, bo backoff.BackOff) (map[string]string, error) {
	c.log.Debugln("client chunk size:", c.config.Client.ChunkSize)
	rc := newReadCounter(r)
	currentOffset := offset
	for i := 0; ; i++ {
		c.log.Debugf("sending chunk #%d from offset=%d", i, currentOffset)
		chunkReader := io.LimitReader(rc, int64(c.config.Client.ChunkSize))
		requestOffset := currentOffset
		resp, err := c.patch(path, chunkReader, requestOffset, size)
		if err != nil {
			return nil, err
		}
		bo.Reset()
		currentOffset = offset + rc.Count()
		c.log.Debugln("sent", currentOffset-requestOffset, "bytes")
		switch currentOffset {
		case size:
			// EOF is reached. Server has deleted the offset file.
			return c.hashes(resp), nil
		case requestOffset:
			// No bytes sent in last request. The file is read to the end.
			return c.deleteOffset(path)
		}
	}
}

func (c *Client) hashes(resp *http.Response) map[string]string {
	return map[string]string{
		"sha1":  resp.Header.Get("efes-file-sha1"),
		"crc32": resp.Header.Get("efes-file-crc32"),
	}
}

// send a single patch request to file receiver
func (c *Client) patch(path string, body io.Reader, offset, size int64) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("efes-file-offset", strconv.FormatInt(offset, 10))
	if size > -1 {
		req.Header.Add("efes-file-length", strconv.FormatInt(size, 10))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, checkResponseError(resp)
}

func (c *Client) getOffset(path string) (int64, error) {
	req, err := http.NewRequest(http.MethodHead, path, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(resp.Header.Get("efes-file-offset"), 10, 64)
}

func (c *Client) deleteOffset(path string) (map[string]string, error) {
	req, err := http.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return c.hashes(resp), checkResponseError(resp)
}

func (c *Client) createOpen(size int64) (path string, fid int64, err error) {
	form := url.Values{}
	if size > -1 {
		form.Add("size", strconv.FormatInt(size, 10))
	}
	var response CreateOpen
	err = c.request(http.MethodPost, "create-open", form, &response)
	return response.Path, response.Fid, err
}

func (c *Client) createClose(key string, fid int64) error {
	form := url.Values{}
	form.Add("key", key)
	form.Add("fid", strconv.FormatInt(fid, 10))
	return c.request(http.MethodPost, "create-close", form, nil)
}
