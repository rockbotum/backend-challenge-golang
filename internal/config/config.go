package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Binance BinanceAPIProperties `json:"binance" yaml:"binance"`
}

type BinanceAPIProperties struct {
	BaseURL       string          `json:"baseUrl" yaml:"baseUrl"`
	Socks5Proxy   string          `json:"socks5Proxy" yaml:"socks5Proxy"`
	MaxMemorySize DataSize        `json:"maxMemorySize" yaml:"maxMemorySize"`
	Timeout       Timeouts        `json:"timeout" yaml:"timeout"`
	OrderBook     OrderBookConfig `json:"orderBook" yaml:"orderBook"`
}

type Timeouts struct {
	Connection Duration `json:"connection" yaml:"connection"`
	Read       Duration `json:"read" yaml:"read"`
	Write      Duration `json:"write" yaml:"write"`
	Total      Duration `json:"total" yaml:"total"`
}

type OrderBookConfig struct {
	Depth               int      `json:"depth" yaml:"depth"`
	InfoEndpointTimeout Duration `json:"infoEndpointTimeout" yaml:"infoEndpointTimeout"`
	DryRun              bool     `json:"dryRun" yaml:"dryRun"`
	Concurrency         int      `json:"concurrency" yaml:"concurrency"`
	MaxRetries          int      `json:"maxRetries" yaml:"maxRetries"`
	RetryBackoff        Duration `json:"retryBackoff" yaml:"retryBackoff"`
	MaxRuntime          Duration `json:"maxRuntime" yaml:"maxRuntime"`
}

func Default() Config {
	return Config{
		Binance: BinanceAPIProperties{
			BaseURL:       "https://api.binance.com",
			MaxMemorySize: Megabyte * 32,
			Timeout: Timeouts{
				Connection: Duration(5 * time.Second),
				Read:       Duration(10 * time.Second),
				Write:      Duration(10 * time.Second),
				Total:      Duration(30 * time.Second),
			},
			OrderBook: OrderBookConfig{
				Depth:               100,
				InfoEndpointTimeout: Duration(15 * time.Second),
				Concurrency:         8,
				MaxRetries:          3,
				RetryBackoff:        Duration(500 * time.Millisecond),
				MaxRuntime:          Duration(5 * time.Minute),
			},
		},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	config := Default()

	if strings.EqualFold(filepath.Ext(path), ".json") {
		err = json.Unmarshal(data, &config)
	} else {
		err = yaml.Unmarshal(data, &config)
	}

	if err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	if err := config.Validate(); err != nil {
		return Config{}, fmt.Errorf("invalid config %q: %w", path, err)
	}

	return config, nil
}

func (c Config) Validate() error {
	var problems []error

	if strings.TrimSpace(c.Binance.BaseURL) == "" {
		problems = append(problems, errors.New("binance.baseUrl is required"))
	}

	if c.Binance.MaxMemorySize <= 0 {
		problems = append(problems, errors.New("binance.maxMemorySize must be positive"))
	}

	if c.Binance.Timeout.Connection <= 0 {
		problems = append(problems, errors.New("binance.timeout.connection must be positive"))
	}

	if c.Binance.Timeout.Read <= 0 {
		problems = append(problems, errors.New("binance.timeout.read must be positive"))
	}

	if c.Binance.Timeout.Write <= 0 {
		problems = append(problems, errors.New("binance.timeout.write must be positive"))
	}

	if c.Binance.Timeout.Total <= 0 {
		problems = append(problems, errors.New("binance.timeout.total must be positive"))
	}

	problems = append(problems, c.Binance.OrderBook.validate()...)

	return errors.Join(problems...)
}

func (o OrderBookConfig) validate() []error {
	var problems []error

	if o.Depth <= 0 {
		problems = append(problems, errors.New("binance.orderBook.depth must be positive"))
	}

	if o.InfoEndpointTimeout <= 0 {
		problems = append(problems, errors.New("binance.orderBook.infoEndpointTimeout must be positive"))
	}

	if o.Concurrency <= 0 {
		problems = append(problems, errors.New("binance.orderBook.concurrency must be positive"))
	}

	if o.MaxRetries < 0 {
		problems = append(problems, errors.New("binance.orderBook.maxRetries must not be negative"))
	}

	if o.RetryBackoff <= 0 {
		problems = append(problems, errors.New("binance.orderBook.retryBackoff must be positive"))
	}

	if o.MaxRuntime <= 0 {
		problems = append(problems, errors.New("binance.orderBook.maxRuntime must be positive"))
	}

	return problems
}
