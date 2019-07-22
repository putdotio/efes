package main

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

func (c *Client) Read(key, path string) error {
	remotePath, err := c.getPath(key)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Get(remotePath.Path)
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
		f, err2 := os.Create(path)
		if err2 != nil {
			return err2
		}
		w = f
		cl = f
	}
	if c.config.Client.ShowProgress {
		var size int64 = -1
		if path != "-" {
			size = c.getContentLength(resp)
		}
		p := newWriteProgress(w, size)
		defer p.Close()
		w = p
	}
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		err2 := cl.Close()
		if err2 != nil {
			c.log.Errorln("Cannot close file:", err)
		}
		return err
	}
	return cl.Close()
}

func (c *Client) getPath(key string) (*GetPath, error) {
	form := url.Values{}
	form.Add("key", key)
	var response GetPath
	_, err := c.request(http.MethodGet, "get-path", form, &response)
	return &response, err
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
