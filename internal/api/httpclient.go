package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"backendchallengegolang/internal/config"
	"golang.org/x/net/proxy"
)

const DefaultMaxIdleConns = 128

func NewHTTPClient(timeouts config.Timeouts, socks5Proxy string) (*http.Client, error) {
	dialContext, err := dialContext(timeouts.Connection.Duration(), socks5Proxy)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(
			ctx context.Context,
			network string,
			address string,
		) (net.Conn, error) {
			if dialContext != nil {
				return dialContext(ctx, network, address)
			}

			return (&net.Dialer{Timeout: timeouts.Connection.Duration()}).DialContext(ctx, network, address)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          DefaultMaxIdleConns,
		MaxIdleConnsPerHost:   DefaultMaxIdleConns,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   timeouts.Connection.Duration(),
		ResponseHeaderTimeout: timeouts.Read.Duration(),
		ExpectContinueTimeout: timeouts.Write.Duration(),
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeouts.Total.Duration(),
	}, nil
}

type dialFunc func(ctx context.Context, network, address string) (net.Conn, error)

func dialContext(timeout time.Duration, socks5Proxy string) (dialFunc, error) {
	if socks5Proxy == "" {
		return nil, nil
	}

	address, err := normalizeProxyAddress(socks5Proxy)
	if err != nil {
		return nil, err
	}

	dialer, err := proxy.SOCKS5("tcp", address, nil, &net.Dialer{Timeout: timeout})
	if err != nil {
		return nil, fmt.Errorf("configure socks5 proxy %q: %w", socks5Proxy, err)
	}

	contextDialer, ok := dialer.(proxy.ContextDialer)
	if !ok {
		return nil, fmt.Errorf("socks5 proxy %q does not support context", socks5Proxy)
	}

	return contextDialer.DialContext, nil
}

func normalizeProxyAddress(value string) (string, error) {
	address := value

	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" {
		address = parsed.Host
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("invalid socks5 proxy %q: %w", value, err)
	}

	if host == "" || port == "" {
		return "", fmt.Errorf("invalid socks5 proxy %q: want host:port", value)
	}

	return net.JoinHostPort(host, port), nil
}
