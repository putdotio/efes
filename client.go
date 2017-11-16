package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/cenkalti/log"
)

// Client is for reading and writing files on Efes.
type Client struct {
	config     ClientConfig
	log        log.Logger
	trackerURL *url.URL
	httpClient http.Client
}

// NewClient creates a new Client.
func NewClient(c *Config) (*Client, error) {
	u, err := url.Parse(c.Client.TrackerURL)
	if err != nil {
		return nil, err
	}
	logger := log.NewLogger("client")
	if c.Debug {
		logger.SetLevel(log.DEBUG)
	}
	return &Client{
		config:     c.Client,
		trackerURL: u,
		log:        logger,
	}, nil
}

func (c *Client) Read(key, path string) error {
	paths, err := c.getPaths(key)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return errors.New("no path returned from tracker")
	}
	resp, err := http.Get(paths[0])
	if err != nil {
		return err
	}
	err = checkResponseError(resp)
	if err != nil {
		return err
	}
	var f *os.File
	if path == "-" {
		f = os.Stdout
	} else {
		f, err = os.Create(path)
		if err != nil {
			return err
		}
	}
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func (c *Client) getPaths(key string) ([]string, error) {
	form := url.Values{}
	form.Add("key", key)
	var response GetPaths
	err := c.request(http.MethodGet, "get-paths", form, &response)
	return response.Paths, err
}

func (c *Client) request(method, urlPath string, params url.Values, response interface{}) error {
	var reqBody io.Reader
	if method == http.MethodPost {
		reqBody = strings.NewReader(params.Encode())
	}
	newURL := *c.trackerURL
	newURL.Path = path.Join(c.trackerURL.Path, urlPath)
	if method == http.MethodGet {
		newURL.RawQuery = params.Encode()
	}
	req, err := http.NewRequest(method, newURL.String(), reqBody)
	if err != nil {
		return err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	c.log.Debugln("request method:", req.Method, "path:", req.URL.Path, "params:", params)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	err = checkResponseError(resp)
	if err != nil {
		return err
	}
	if response == nil {
		return nil
	}
	err = json.NewDecoder(resp.Body).Decode(response)
	if err != nil {
		return err
	}
	c.log.Debugf("%s got response: %#v", req.URL.Path, response)
	return nil
}

func (c *Client) Write(key, path string) error {
	if path == "-" {
		return c.writeReader(key, path, os.Stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close() // nolint: errcheck
	return c.writeFile(key, path, f)
}

func (c *Client) writeReader(key, path string, r io.Reader) error {
	path, fid, devid, err := c.createOpen(-1)
	if err != nil {
		return err
	}
	n, err := c.sendReader(path, r)
	if err != nil {
		return err
	}
	// Because we don't know the length of the stream,
	// we need to DELETE the ".offset" file on the server.
	err = c.deleteOffset(path)
	if err != nil {
		return err
	}
	return c.createClose(key, n, fid, devid)
}

func (c *Client) writeFile(key, path string, f *os.File) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()
	path, fid, devid, err := c.createOpen(size)
	if err != nil {
		return err
	}
	n, err := c.sendFile(path, f, size)
	if err != nil {
		return err
	}
	return c.createClose(key, n, fid, devid)
}

func (c *Client) sendReader(path string, r io.Reader) (int64, error) {
	var offset int64
	for {
		n, err := c.send(path, r, offset, -1)
		offset += n
		if cerr, ok := err.(*ClientError); ok {
			return offset, cerr
		}
		if err != nil {
			c.log.Errorln("error while sending the stream:", err)
			continue
		}
		return offset, nil
	}
}

func (c *Client) sendFile(path string, f *os.File, size int64) (int64, error) {
	var offset int64
	for {
		n, err := c.send(path, f, offset, size)
		offset += n
		if cerr, ok := err.(*ClientError); ok {
			// Get the offset from server and try again.
			if cerr.Code == http.StatusConflict {
				actualOffsetString := cerr.Header.Get("efes-file-offset")
				if actualOffsetString != "" {
					var actualOffset int64
					actualOffset, err = strconv.ParseInt(actualOffsetString, 10, 64)
					if err != nil {
						c.log.Errorln("got invalid offset from server:", actualOffsetString)
						return offset, err
					}
					_, err = f.Seek(actualOffset, os.SEEK_SET)
					if err != nil {
						return offset, err
					}
					offset = actualOffset
					c.log.Infoln("offset is reset from server:", offset)
					continue
				}
			}
			return offset, cerr
		}
		if err != nil {
			c.log.Errorln("error while sending the file:", err)
			continue
		}
		return offset, nil
	}
}

// send a patch request until and error occurs or stream is finished
func (c *Client) send(path string, r io.Reader, offset, size int64) (int64, error) {
	c.log.Debugln("client chunk size:", c.config.ChunkSize)
	totalCounter := newReadCounter(r)
	currentOffset := offset
	for i := 0; ; i++ {
		c.log.Debugf("sending chunk #%d from offset=%d", i, currentOffset)
		chunkReader := io.LimitReader(totalCounter, int64(c.config.ChunkSize))
		requestOffset := currentOffset
		err := c.patch(path, chunkReader, requestOffset, size)
		if err != nil {
			return totalCounter.Count(), err
		}
		currentOffset = offset + totalCounter.Count()
		c.log.Debugln("sent", currentOffset-requestOffset, "bytes")
		switch currentOffset {
		case size:
			// EOF is reached. Do not make a new PATCH request with empty body.
			return totalCounter.Count(), nil
		case requestOffset:
			// No bytes sent in last request.
			return totalCounter.Count(), nil
		}
	}
}

// send a single patch request to file receiver
func (c *Client) patch(path string, body io.Reader, offset, size int64) error {
	req, err := http.NewRequest(http.MethodPatch, path, body)
	if err != nil {
		return err
	}
	req.Header.Add("efes-file-offset", strconv.FormatInt(offset, 10))
	if size > -1 {
		req.Header.Add("efes-file-length", strconv.FormatInt(size, 10))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	return checkResponseError(resp)
}

func (c *Client) createOpen(size int64) (path string, fid, devid int64, err error) {
	form := url.Values{}
	if size > -1 {
		form.Add("size", strconv.FormatInt(size, 10))
	}
	var response CreateOpen
	err = c.request(http.MethodPost, "create-open", form, &response)
	return response.Path, response.Fid, response.Devid, err
}

func (c *Client) createClose(key string, size, fid, devid int64) error {
	form := url.Values{}
	form.Add("key", key)
	form.Add("size", strconv.FormatInt(size, 10))
	form.Add("fid", strconv.FormatInt(fid, 10))
	form.Add("devid", strconv.FormatInt(devid, 10))
	return c.request(http.MethodPost, "create-close", form, nil)
}

func (c *Client) deleteOffset(path string) error {
	req, err := http.NewRequest(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	return checkResponseError(resp)
}

// Delete the key on Efes.
func (c *Client) Delete(key string) error {
	form := url.Values{}
	form.Add("key", key)
	return c.request(http.MethodPost, "delete", form, nil)
}

// Exist checks the existing of a key on Efes.
func (c *Client) Exist(key string) (bool, error) {
	paths, err := c.getPaths(key)
	if err != nil {
		return false, err
	}
	return len(paths) > 0, nil
}
