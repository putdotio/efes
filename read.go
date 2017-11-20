package main

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

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
	var w io.Writer
	var cl io.Closer
	if path == "-" {
		w = os.Stdout
		cl = os.Stdout
	} else {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
		cl = f
		if c.config.Client.ShowProgress {
			size := c.getContentLength(resp)
			p := newWriteProgress(f, size)
			defer p.Close()
			w = p
		}
	}
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}
	return cl.Close()
}

func (c *Client) getPaths(key string) ([]string, error) {
	form := url.Values{}
	form.Add("key", key)
	var response GetPaths
	err := c.request(http.MethodGet, "get-paths", form, &response)
	return response.Paths, err
}

func (c *Client) getContentLength(resp *http.Response) int64 {
	contentLengthHeader := resp.Header.Get("Content-Length")
	if contentLengthHeader == "" {
		c.log.Warning("server sent no conent-length header")
		return -1
	}
	contentLength, err := strconv.ParseInt(contentLengthHeader, 10, 64)
	if err != nil {
		c.log.Errorln("cannot parse content-length header:", err.Error())
		return -1
	}
	return contentLength
}
