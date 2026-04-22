package main

import "C"
import (
	"context"
	"go-m-explosion/internal"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	ctx := context.Background()
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	failedTransactions := prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "failed_transactions_total",
			Help: "Total number of transactions rejected by the thread limiter",
		},
	)

	prometheus.MustRegister(failedTransactions)

	// Start Prometheus metrics server in a separate goroutine
	// Access metrics at http://localhost:8080/metrics
	go func() {
		http.Handle(
			"/metrics",
			promhttp.Handler(),
		)
		http.ListenAndServe(
			":8080",
			nil,
		)
	}()

	// STEP 1: Strictly limit logical processors (P) to 2
	// This should theoretically limit the number of threads, but Go's sysmon has other plans.
	runtime.GOMAXPROCS(2)

	// Wait 30 seconds to establish a "baseline" in Grafana (idle state)
	time.Sleep(40 * time.Second)

	// STEP 2: Launch 100 "greedy" goroutines
	runner := internal.NewThreadExplosionRunner(log, failedTransactions.Inc)
	runner.Run(ctx, internal.DoSlowCall)

	// Keep the process alive for another 45 seconds
	// This allows Prometheus to scrape the "post-explosion" state and cooldown.
	time.Sleep(45 * time.Second)
}
