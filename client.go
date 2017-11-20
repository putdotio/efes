package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/cenkalti/log"
)

// Client is for reading and writing files on Efes.
type Client struct {
	config     *Config
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
		config:     c,
		trackerURL: u,
		log:        logger,
	}, nil
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
