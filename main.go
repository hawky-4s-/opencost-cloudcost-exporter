// opencost-cloudcost-exporter is a Prometheus exporter for AWS cloud costs from OpenCost.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/cache"
	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/client"
	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/collector"
)

// Build information - injected via ldflags
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// CLI flags
	opencostURL := flag.String("opencost-url", getEnv("OPENCOST_URL", "http://opencost.opencost:9003"), "OpenCost service URL")
	port := flag.String("port", getEnv("PORT", "9100"), "Metrics server port")
	window := flag.String("window", getEnv("WINDOW", "2d"), "Time window for cost queries")
	aggregate := flag.String("aggregate", getEnv("AGGREGATE", "service,category"), "Aggregation dimensions")
	cacheTTL := flag.Duration("cache-ttl", parseDuration(getEnv("CACHE_TTL", "1h")), "Cache TTL")
	maxStale := flag.Duration("max-stale", parseDuration(getEnv("MAX_STALE", "6h")), "Maximum age for stale data")
	emitKubePercentMetrics := flag.Bool("emit-kube-percent-metrics", getEnv("EMIT_KUBE_PERCENT_METRICS", "false") == "true", "Emit kubernetes percent metric")
	currencySymbols := flag.String("currency-symbols", getEnv("CURRENCY_SYMBOLS", "CNY,EUR"), "Comma-separated target currency symbols for exchange rates")
	logLevel := flag.String("log-level", getEnv("LOG_LEVEL", "info"), "Log level (debug, info, warn, error)")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		println("opencost-cloudcost-exporter", version, commit, date)
		os.Exit(0)
	}

	// Configure structured JSON logging
	var level slog.Level
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
	slog.SetDefault(logger)

	slog.Info("starting opencost-cloudcost-exporter",
		"version", version,
		"commit", commit,
		"date", date,
		"opencost_url", *opencostURL,
		"port", *port,
		"window", *window,
		"cache_ttl", cacheTTL.String(),
		"max_stale", maxStale.String(),
	)

	// Register build info metric
	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "cloudcost_exporter",
		Name:      "info",
		Help:      "Build information about the opencost-cloudcost-exporter",
	}, []string{"version", "commit", "date"})
	buildInfo.WithLabelValues(version, commit, date).Set(1)
	prometheus.MustRegister(buildInfo)

	// Create components
	cl := client.New(*opencostURL,
		client.WithWindow(*window),
		client.WithAggregate(*aggregate),
		client.WithTimeout(30*time.Second),
	)
	ca := cache.New(*cacheTTL, *maxStale)
	// Parse currency symbols
	var symbols []string
	if *currencySymbols != "" {
		for _, s := range strings.Split(*currencySymbols, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				symbols = append(symbols, s)
			}
		}
	}

	coll := collector.New(cl, ca,
		collector.WithKubePercentMetrics(*emitKubePercentMetrics),
		collector.WithCurrencySymbols(symbols),
	)

	// Register collector
	prometheus.MustRegister(coll)

	// HTTP server
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", healthzHandler)
	mux.HandleFunc("/readyz", readyzHandler(cl, ca))

	server := &http.Server{
		Addr:         ":" + *port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		slog.Info("shutting down server")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(ctx)
	}()

	slog.Info("server listening", "addr", server.Addr)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

// healthzHandler returns 200 OK if the server is running.
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// readyzHandler returns 200 OK if OpenCost is reachable and cache is populated.
func readyzHandler(cl *client.Client, ca *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if cache is populated
		if !ca.IsPopulated() {
			// Try to ping OpenCost
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			if err := cl.Ping(ctx); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("not ready: " + err.Error()))
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	}
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Hour
	}
	return d
}
