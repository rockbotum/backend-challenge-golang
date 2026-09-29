package config

import "time"

type BinanceAPIProperties struct {
	BaseURL       string    `json:"baseUrl" yaml:"baseUrl"`
	MaxMemorySize DataSize  `json:"maxMemorySize" yaml:"maxMemorySize"`
	Timeout       Timeouts  `json:"timeout" yaml:"timeout"`
	OrderBook     OrderBook `json:"orderBook" yaml:"orderBook"`
}

type Timeouts struct {
	Connection time.Duration `json:"connection" yaml:"connection"`
	Read       time.Duration `json:"read" yaml:"read"`
	Write      time.Duration `json:"write" yaml:"write"`
}

type OrderBook struct {
	Depth               int           `json:"depth" yaml:"depth"`
	InfoEndpointTimeout time.Duration `json:"infoEndpointTimeout" yaml:"infoEndpointTimeout"`
	DryRun              bool          `json:"dryRun" yaml:"dryRun"`
}
