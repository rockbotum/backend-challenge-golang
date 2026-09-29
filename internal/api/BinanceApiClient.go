package api

import (
	"net/http"
	"time"
)

type BinanceClient struct {
	baseURL string
	client  *http.Client
	config  BinanceConfig
}

type BinanceConfig struct {
	Depth   int
	DryRun  bool
	Timeout time.Duration
}

func NewBinanceClient(baseURL string, httpClient *http.Client, config BinanceConfig) *BinanceClient {
	return &BinanceClient{
		baseURL: baseURL,
		client:  httpClient,
		config:  config,
	}
}

func (b *BinanceClient) GetSymbolOrderBookMono(symbol Symbol) *chan any {
	resultCh := make(chan any, 1)
	go func() {
		b.client.Get("/depth?symbol=$%w&limit=$%d", symbol.symbol, BinanceApi)
	}()
}
