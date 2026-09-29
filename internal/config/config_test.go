package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeConfig(t *testing.T, name string, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	return path
}

const validYAML = `
binance:
  baseUrl: https://example.test
  socks5Proxy: 127.0.0.1:1080
  maxMemorySize: 20MB
  timeout:
    connection: 1s
    read: 2s
    write: 3s
    total: 4s
  orderBook:
    depth: 500
    infoEndpointTimeout: 6s
    dryRun: true
    concurrency: 3
    maxRetries: 1
    retryBackoff: 100ms
    maxRuntime: 30s
`

func TestLoad_YAML(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "config.yaml", validYAML)

	settings, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned %v", err)
	}

	if settings.Binance.BaseURL != "https://example.test" {
		t.Errorf("baseUrl = %q, want %q", settings.Binance.BaseURL, "https://example.test")
	}

	if settings.Binance.Socks5Proxy != "127.0.0.1:1080" {
		t.Errorf("socks5Proxy = %q, want %q", settings.Binance.Socks5Proxy, "127.0.0.1:1080")
	}

	if want := 20 * Megabyte; settings.Binance.MaxMemorySize != want {
		t.Errorf("maxMemorySize = %d, want %d", settings.Binance.MaxMemorySize.Bytes(), int64(want))
	}

	if want := 2 * time.Second; settings.Binance.Timeout.Read.Duration() != want {
		t.Errorf("timeout.read = %s, want %s", settings.Binance.Timeout.Read, want)
	}

	if settings.Binance.OrderBook.Depth != 500 {
		t.Errorf("depth = %d, want 500", settings.Binance.OrderBook.Depth)
	}

	if !settings.Binance.OrderBook.DryRun {
		t.Error("dryRun = false, want true")
	}

	if settings.Binance.OrderBook.Concurrency != 3 {
		t.Errorf("concurrency = %d, want 3", settings.Binance.OrderBook.Concurrency)
	}

	if want := 100 * time.Millisecond; settings.Binance.OrderBook.RetryBackoff.Duration() != want {
		t.Errorf("retryBackoff = %s, want %s", settings.Binance.OrderBook.RetryBackoff, want)
	}
}

func TestLoad_JSON(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "config.json", `{
		"binance": {
			"baseUrl": "https://json.test",
			"maxMemorySize": "1GB",
			"timeout": {"connection": "1s", "read": "1s", "write": "1s", "total": "1s"},
			"orderBook": {
				"depth": 10,
				"infoEndpointTimeout": "1s",
				"concurrency": 2,
				"maxRetries": 0,
				"retryBackoff": "1s",
				"maxRuntime": "1m"
			}
		}
	}`)

	settings, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned %v", err)
	}

	if settings.Binance.BaseURL != "https://json.test" {
		t.Errorf("baseUrl = %q, want %q", settings.Binance.BaseURL, "https://json.test")
	}

	if want := Gigabyte; settings.Binance.MaxMemorySize != want {
		t.Errorf("maxMemorySize = %d, want %d", settings.Binance.MaxMemorySize.Bytes(), int64(want))
	}
}

func TestLoad_AppliesDefaultsForMissingValues(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "config.yaml", `
binance:
  baseUrl: https://partial.test
  orderBook:
    concurrency: 4
`)

	settings, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned %v", err)
	}

	defaults := Default()

	if settings.Binance.Timeout.Total.Duration() != defaults.Binance.Timeout.Total.Duration() {
		t.Errorf(
			"timeout.total = %s, want default %s",
			settings.Binance.Timeout.Total,
			defaults.Binance.Timeout.Total,
		)
	}

	if settings.Binance.OrderBook.Depth != defaults.Binance.OrderBook.Depth {
		t.Errorf("depth = %d, want default %d", settings.Binance.OrderBook.Depth, defaults.Binance.OrderBook.Depth)
	}

	if settings.Binance.OrderBook.Concurrency != 4 {
		t.Errorf("concurrency = %d, want 4", settings.Binance.OrderBook.Concurrency)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()

	if _, err := Load(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
		t.Fatal("Load returned nil, want error")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, "config.yaml", "binance: [not a mapping]")

	if _, err := Load(path); err == nil {
		t.Fatal("Load returned nil, want error")
	}
}

func TestValidate_ReportsEveryProblem(t *testing.T) {
	t.Parallel()

	settings := Config{}

	err := settings.Validate()
	if err == nil {
		t.Fatal("Validate returned nil, want error")
	}

	for _, want := range []string{
		"binance.baseUrl is required",
		"binance.maxMemorySize must be positive",
		"binance.timeout.connection must be positive",
		"binance.orderBook.depth must be positive",
		"binance.orderBook.concurrency must be positive",
		"binance.orderBook.maxRuntime must be positive",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("Validate error %q does not mention %q", err, want)
		}
	}
}

func TestValidate_DefaultIsValid(t *testing.T) {
	t.Parallel()

	if err := Default().Validate(); err != nil {
		t.Fatalf("Default().Validate() returned %v", err)
	}
}
