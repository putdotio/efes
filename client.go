package main

// Client is for reading and writing files on Efes.
type Client struct{}

// NewClient creates a new Client.
func NewClient(c *Config) (*Client, error) {
	return &Client{}, nil
}

func (c *Client) Write(path string) error {
	return nil
}

func (c *Client) Read(path string) error {
	return nil
}
