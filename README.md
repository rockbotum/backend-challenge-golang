### Name: <Egor Vybornov>
### Email: <kallionsoft@gmail.com>
### CV: <@rockbotum>

## Description

Port of the original Kotlin/Spring WebFlux application to Go.

The calculator keeps the original algorithm and produces the same metrics, but the two problems
called out in the challenge are addressed:

- **Bounded concurrency.** Order books are fetched by a fixed pool of workers reading from a single
  `jobs` channel (`binance.orderBook.concurrency`), instead of one request after another.
  `jobs` is closed by the producer on completion or cancellation, `results` is closed only after
  every worker returns, and all stages select on `ctx.Done()`, so no goroutine outlives `Calculate`.
- **Request weight.** `ratelimit.Limiter` converts the `REQUEST_WEIGHT` windows published by
  `/api/v3/exchangeInfo` into token buckets and grants the weight of `/api/v3/depth` before every
  call (5 for `limit` up to 100, 25 up to 500, 50 up to 1000, 250 above). Every configured window
  must be satisfied. On HTTP 429 the buckets are drained and acquisitions are blocked for
  `Retry-After`, or for an exponentially growing `retryBackoff` when the exchange omits that
  header, so retries are never immediate. Retries are bounded by `binance.orderBook.maxRetries`
  and apply to 429 and 5xx only; a 4xx fails that symbol immediately and the run continues.
- **Streaming aggregation.** An order book is folded into `Metrics` as soon as it arrives and then
  dropped, so memory is bounded by the worker pool rather than by the symbol count. Volumes are
  accumulated with `decimal.Decimal` and never pass through `float64`. Averages are computed once,
  after all workers finish, over the books that actually contributed a level, and are reported as
  unavailable when no book contributed one.
- **Cancellation and timeouts.** A single `*http.Client` with an explicit `http.Transport` is
  shared by all workers, every request uses `http.NewRequestWithContext`, and `SIGINT`/`SIGTERM`
  or `binance.orderBook.maxRuntime` stops the run promptly. Response bodies are capped by
  `binance.maxMemorySize` through `io.LimitReader`.

Structure and technology changes:

- Kotlin/Spring WebFlux/Maven replaced by a plain Go module; the `src/main`/`src/test` layout is
  not copied.
- Configuration moved from `application.yml` binding to `internal/config` with `gopkg.in/yaml.v3`;
  durations (`250ms`, `1m30s`) and data sizes (`512KB`, `10MB`, `1GB`) are first class types.
  Missing properties fall back to defaults and the whole configuration is validated on load.
- `WebClient` replaced by a reusable `*http.Client` plus a typed wrapper in `internal/api`.
  Optional SOCKS5 routing is available through `binance.socks5Proxy` (`golang.org/x/net/proxy`),
  HTTP(S) proxies are read from `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`.
- `Mono`/`Flux` replaced by `(T, error)` returns, channels and a worker pool.
- `BinanceApiClient#getRateLimits` became `api.Client.GetRateLimits`, `OrderBookMetricsCalculator`
  became `metrics.Calculator`.
- `binance.orderBook.dryRun` limits a run to the first 10 tradable symbols for local testing.

Tests use `httptest` only, the real exchange is never called: model decoding including malformed
price levels and invalid decimals, HTTP 200/429/4xx/5xx, response size limit, context cancellation
and timeouts, dry-run symbol cap, concurrency limit, retries and give-up, streaming aggregation,
empty bids/asks, and absence of goroutine leaks.

Verified locally with `go build ./...`, `go vet ./...` and `go test ./...`. A dry run against the
live exchange loads 3713 symbols, processes 10 and prints the averages.

## Motivation

The original calculator retrieves order books one by one, which does not fit the app timeout, and
keeps every order book in memory to compute the averages. Both were explicitly listed as the
problem to solve. The port keeps the observable behaviour and the configuration surface of the
original application while addressing the timeout through bounded parallel fetching under the
published rate limits, and the memory consumption through streaming aggregation. Go was chosen
because the challenge focuses on concurrency primitives and cancellation, where the Go toolchain
maps directly onto the original reactive design without a framework.

## Feedback

- The limiter uses token buckets with even refill rather than the sliding window Binance counts, so
  under heavy parallelism the real weight can briefly exceed the published limit. This is why the
  limiter backs off aggressively after 429 instead of retrying tightly. Rate limits are counted per
  IP, so another client on the same address still affects the run.
- `binance.maxMemorySize` bounds a single response and therefore has to stay above the size of a
  full `/api/v3/exchangeInfo` payload, roughly 18MB at the time of writing.
- Averages are per order book, not per price level, and are reported as unavailable when no book
  contributed a level on that side.
- Only public endpoints are used, snapshots are taken once rather than streamed over websocket, and
  the metrics are aggregates of a single snapshot per symbol rather than a time series.
