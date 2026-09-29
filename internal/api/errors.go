package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

var ErrRateLimited = errors.New("rate limit exceeded")

type StatusError struct {
	Code   int
	Status string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("binance returned status %s", e.Status)
}

type RateLimitError struct {
	StatusCode int
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("binance rate limit exceeded, retry after %s", e.RetryAfter)
	}

	return "binance rate limit exceeded"
}

func (e *RateLimitError) Unwrap() error {
	return ErrRateLimited
}

func statusError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return &RateLimitError{
			StatusCode: resp.StatusCode,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
		}
	default:
		return &StatusError{Code: resp.StatusCode, Status: resp.Status}
	}
}

func parseRetryAfter(value string) time.Duration {
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds <= 0 {
			return 0
		}

		return time.Duration(seconds) * time.Second
	}

	if deadline, err := http.ParseTime(value); err == nil {
		if wait := time.Until(deadline); wait > 0 {
			return wait
		}
	}

	return 0
}
