package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"backendchallengegolang/internal/config"
)

type Client struct {
	baseURL string
	http    *http.Client
	maxBody int64
}

func NewClient(properties config.BinanceAPIProperties, httpClient *http.Client) *Client {
	return &Client{
		baseURL: strings.TrimRight(properties.BaseURL, "/"),
		http:    httpClient,
		maxBody: properties.MaxMemorySize.Bytes(),
	}
}

func (c *Client) GetExchangeInfo(ctx context.Context) (ExchangeInfo, error) {
	var info ExchangeInfo

	if err := c.get(ctx, "/api/v3/exchangeInfo", nil, &info); err != nil {
		return ExchangeInfo{}, err
	}

	return info, nil
}

func (c *Client) GetRateLimits(ctx context.Context) ([]RateLimit, error) {
	info, err := c.GetExchangeInfo(ctx)
	if err != nil {
		return nil, err
	}

	return info.RateLimits, nil
}

func (c *Client) GetOrderBook(ctx context.Context, symbol string, depth int) (OrderBook, error) {
	query := url.Values{}
	query.Set("symbol", symbol)
	query.Set("limit", strconv.Itoa(depth))

	var book OrderBook

	if err := c.get(ctx, "/api/v3/depth", query, &book); err != nil {
		return OrderBook{}, err
	}

	return book, nil
}

func (c *Client) CloseIdleConnections() {
	c.http.CloseIdleConnections()
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build %s request: %w", path, err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("call %s: %w", path, statusError(resp))
	}

	payload, err := c.readBody(resp)
	if err != nil {
		return fmt.Errorf("read %s response: %w", path, err)
	}

	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}

	return nil
}

func (c *Client) readBody(resp *http.Response) ([]byte, error) {
	if c.maxBody <= 0 {
		return io.ReadAll(resp.Body)
	}

	payload, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBody+1))
	if err != nil {
		return nil, err
	}

	if int64(len(payload)) > c.maxBody {
		return nil, fmt.Errorf("response exceeds %d bytes", c.maxBody)
	}

	return payload, nil
}
