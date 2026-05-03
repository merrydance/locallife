package baofu

import "net/http"

type Client struct {
	config    Config
	transport *Transport
}

func NewClient(cfg Config, httpClient HTTPDoer) (*Client, error) {
	cfg = cfg.Normalized()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Client{config: cfg, transport: NewTransport(httpClient, cfg.Timeout)}, nil
}

func (c *Client) Config() Config {
	if c == nil {
		return Config{}
	}
	return c.config
}

var _ HTTPDoer = (*http.Client)(nil)
