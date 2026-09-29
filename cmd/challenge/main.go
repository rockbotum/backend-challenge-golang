package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backendchallengegolang/internal/api"
	"backendchallengegolang/internal/config"
	"backendchallengegolang/internal/metrics"
	"backendchallengegolang/internal/ratelimit"
)

const defaultConfigPath = "configs/config.yaml"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "challenge: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	configPath := flag.String("config", defaultConfigPath, "path to the YAML or JSON configuration file")
	verbose := flag.Bool("verbose", false, "enable debug logging")
	flag.Parse()

	settings, err := config.Load(*configPath)
	if err != nil {
		return err
	}

	logger := newLogger(*verbose)

	if err := calculate(logger, settings); err != nil {
		return err
	}

	return nil
}

func newLogger(verbose bool) *slog.Logger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

func calculate(logger *slog.Logger, settings config.Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient, err := api.NewHTTPClient(
		settings.Binance.Timeout,
		settings.Binance.Socks5Proxy,
	)
	if err != nil {
		return err
	}

	client := api.NewClient(settings.Binance, httpClient)
	defer client.CloseIdleConnections()

	infoCtx, cancelInfo := context.WithTimeout(ctx, settings.Binance.OrderBook.InfoEndpointTimeout.Duration())
	defer cancelInfo()

	exchangeInfo, err := client.GetExchangeInfo(infoCtx)
	if err != nil {
		return fmt.Errorf("load exchange info: %w", err)
	}

	limits := rateLimits(exchangeInfo.RateLimits)
	logger.Info(
		"exchange info loaded",
		"symbols", len(exchangeInfo.Symbols),
		"rateLimits", len(limits),
	)

	calculator := metrics.NewCalculator(
		client,
		ratelimit.New(limits, time.Now),
		metrics.Options{
			Depth:      settings.Binance.OrderBook.Depth,
			Workers:    settings.Binance.OrderBook.Concurrency,
			DryRun:     settings.Binance.OrderBook.DryRun,
			MaxRetries: settings.Binance.OrderBook.MaxRetries,
			Backoff:    settings.Binance.OrderBook.RetryBackoff.Duration(),
			Weight:     ratelimit.DepthWeight(settings.Binance.OrderBook.Depth),
		},
		logger,
	)

	runCtx, cancelRun := context.WithTimeout(ctx, settings.Binance.OrderBook.MaxRuntime.Duration())
	defer cancelRun()

	result, err := calculator.Calculate(runCtx, exchangeInfo.Symbols)
	if err != nil {
		return err
	}

	report(logger, result)

	return nil
}

func rateLimits(limits []api.RateLimit) []ratelimit.Limit {
	converted := make([]ratelimit.Limit, 0, len(limits))

	for _, limit := range limits {
		if limit.RateLimitType != "REQUEST_WEIGHT" {
			continue
		}

		window := limit.Window()
		if window <= 0 {
			continue
		}

		converted = append(converted, ratelimit.Limit{
			Weight: limit.Limit,
			Window: window,
		})
	}

	return converted
}

func report(logger *slog.Logger, result metrics.Result) {
	aggregates := result.Metrics

	logger.Info(
		"order books aggregated",
		"symbols", aggregates.Symbols,
		"failed", aggregates.Failed,
		"askVolume", aggregates.AskVolume.String(),
		"bidVolume", aggregates.BidVolume.String(),
		"askBooks", aggregates.AskCount,
		"bidBooks", aggregates.BidCount,
	)

	averageAsk, err := aggregates.AverageAskVolume()
	if err != nil {
		logger.Warn("average ask volume is unavailable", "reason", err)
	} else {
		logger.Info("average ask volume", "value", averageAsk.String())
	}

	averageBid, err := aggregates.AverageBidVolume()
	if err != nil {
		logger.Warn("average bid volume is unavailable", "reason", err)
	} else {
		logger.Info("average bid volume", "value", averageBid.String())
	}

	for _, symbolErr := range result.Errors {
		logger.Error("symbol failed", "reason", symbolErr)
	}
}
