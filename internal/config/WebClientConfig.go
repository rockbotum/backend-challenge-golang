package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/proxy"
)

type WebClient struct {
	baseURL string
	http    *http.Client
}

func NewClient(
	baseURL string,
	httpClient *http.Client,
) *WebClient {
	return &WebClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    httpClient,
	}
}

func (c *WebClient) Get(
	ctx context.Context,
	path string,
	query url.Values,
	result any,
) error {
	endpoint := c.baseURL + path

	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		endpoint,
		nil,
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"unexpected Binance status: %d",
			resp.StatusCode,
		)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func NewHTTPClientWithSOCKS5(
	host string,
	port string,
	properties BinanceAPIProperties,
) (*http.Client, error) {
	dialer, err := proxy.SOCKS5(
		"tcp",
		net.JoinHostPort(host, port),
		nil,
		&net.Dialer{
			Timeout: properties.Timeout.Connection,
		},
	)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		DialContext: func(
			ctx context.Context,
			network string,
			address string,
		) (net.Conn, error) {
			return dialer.Dial(network, address)
		},
		ResponseHeaderTimeout: properties.Timeout.Read,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   properties.Timeout.Read,
	}, nil
}
