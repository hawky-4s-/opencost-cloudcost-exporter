// Package collector provides a Prometheus collector for AWS cloud costs.
package collector

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/cache"
	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/client"
	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/types"
)

const namespace = "aws_cloud"

// CloudCostCollector collects AWS cloud cost metrics from OpenCost.
type CloudCostCollector struct {
	client *client.Client
	cache  *cache.Cache

	// Config options
	emitKubePercentMetrics bool
	currencySymbols        []string

	// Cost metrics
	costTotal    *prometheus.Desc
	kubePercent  *prometheus.Desc
	exchangeRate *prometheus.Desc

	// Self-observability metrics
	scrapeDuration       prometheus.Histogram
	scrapeErrors         prometheus.Counter
	cacheHits            prometheus.Counter
	cacheMisses          prometheus.Counter
	cacheAge             prometheus.Gauge
	lastSuccessfulScrape prometheus.Gauge

	mu         sync.Mutex
	refreshing bool // prevents concurrent refresh goroutines
}

// Option is a functional option for configuring the CloudCostCollector.
type Option func(*CloudCostCollector)

// WithKubePercentMetrics enables or disables the kubernetes percent metric.
func WithKubePercentMetrics(enabled bool) Option {
	return func(c *CloudCostCollector) {
		c.emitKubePercentMetrics = enabled
	}
}

// WithCurrencySymbols sets the target currency symbols for exchange rates.
func WithCurrencySymbols(symbols []string) Option {
	return func(c *CloudCostCollector) {
		c.currencySymbols = symbols
	}
}

// New creates a new CloudCostCollector.
func New(c *client.Client, ca *cache.Cache, opts ...Option) *CloudCostCollector {
	collector := &CloudCostCollector{
		client:                 c,
		cache:                  ca,
		emitKubePercentMetrics: false,           // disabled by default
		currencySymbols:        []string{"CNY", "EUR"}, // default symbols
		costTotal: prometheus.NewDesc(
			namespace+"_cost_total",
			"AWS cloud cost in USD",
			[]string{"provider_id", "account_id", "service", "category", "cost_type", "region", "availability_zone", "owner", "environment", "cluster"},
			nil,
		),
		kubePercent: prometheus.NewDesc(
			namespace+"_cost_kubernetes_percent",
			"Percentage of cost attributed to Kubernetes",
			[]string{"provider_id", "account_id", "service", "category", "cost_type", "region"},
			nil,
		),
		exchangeRate: prometheus.NewDesc(
			"currency_exchange_rate",
			"Currency exchange rate from base to target currency",
			[]string{"base", "target"},
			nil,
		),
		scrapeDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "cloudcost_exporter",
			Name:      "scrape_duration_seconds",
			Help:      "Time to fetch cloud costs from OpenCost",
			Buckets:   prometheus.DefBuckets,
		}),
		scrapeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cloudcost_exporter",
			Name:      "scrape_errors_total",
			Help:      "Total number of scrape errors",
		}),
		cacheHits: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cloudcost_exporter",
			Name:      "cache_hits_total",
			Help:      "Total number of cache hits",
		}),
		cacheMisses: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "cloudcost_exporter",
			Name:      "cache_misses_total",
			Help:      "Total number of cache misses",
		}),
		cacheAge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "cloudcost_exporter",
			Name:      "cache_age_seconds",
			Help:      "Age of cached data in seconds",
		}),
		lastSuccessfulScrape: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "cloudcost_exporter",
			Name:      "last_successful_scrape_timestamp",
			Help:      "Unix timestamp of last successful scrape",
		}),
	}

	for _, opt := range opts {
		opt(collector)
	}

	return collector
}

// Describe implements prometheus.Collector.
func (c *CloudCostCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.costTotal
	if c.emitKubePercentMetrics {
		ch <- c.kubePercent
	}
	ch <- c.exchangeRate
	c.scrapeDuration.Describe(ch)
	c.scrapeErrors.Describe(ch)
	c.cacheHits.Describe(ch)
	c.cacheMisses.Describe(ch)
	c.cacheAge.Describe(ch)
	c.lastSuccessfulScrape.Describe(ch)
}

// Collect implements prometheus.Collector.
func (c *CloudCostCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Try cache first
	data, isStale, ok := c.cache.Get()
	if ok {
		c.cacheHits.Inc()
		if isStale && !c.refreshing {
			// Try to refresh in background, but use stale data
			c.refreshing = true
			go func() {
				c.refreshCache()
				c.mu.Lock()
				c.refreshing = false
				c.mu.Unlock()
			}()
		}
	} else {
		c.cacheMisses.Inc()
		data = c.fetchAndCache()
	}

	// Update cache age metric
	c.cacheAge.Set(c.cache.Age().Seconds())

	// Emit self-observability metrics
	c.scrapeDuration.Collect(ch)
	c.scrapeErrors.Collect(ch)
	c.cacheHits.Collect(ch)
	c.cacheMisses.Collect(ch)
	c.cacheAge.Collect(ch)
	c.lastSuccessfulScrape.Collect(ch)

	if data == nil {
		return
	}

	// Emit cost metrics
	c.emitCostMetrics(ch, data)

	// Emit exchange rate metrics
	c.emitExchangeRates(ch)
}

func (c *CloudCostCollector) fetchAndCache() *types.CloudCostResponse {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	data, err := c.client.FetchCloudCosts(ctx)
	c.scrapeDuration.Observe(time.Since(start).Seconds())

	if err != nil {
		c.scrapeErrors.Inc()
		slog.Error("failed to fetch cloud costs", "error", err)
		return nil
	}

	c.cache.Set(data)
	c.lastSuccessfulScrape.SetToCurrentTime()
	return data
}

func (c *CloudCostCollector) refreshCache() {
	c.fetchAndCache()
}

func (c *CloudCostCollector) emitCostMetrics(ch chan<- prometheus.Metric, data *types.CloudCostResponse) {
	// Aggregate costs by service/category/labels
	type costKey struct {
		providerID       string
		accountID        string
		service          string
		category         string
		region           string
		availabilityZone string
		owner            string
		environment      string
		cluster          string
	}

	aggregated := make(map[costKey]*aggregatedCost)

	slog.Debug("processing cloud cost data",
		"num_sets", len(data.Data.Sets),
	)

	for setIdx, set := range data.Data.Sets {
		slog.Debug("processing cloud cost set",
			"set_index", setIdx,
			"num_items", len(set.CloudCosts),
		)

		for _, item := range set.CloudCosts {
			// Extract labels
			owner := item.Properties.Labels["owner"]
			environment := item.Properties.Labels["environment"]
			cluster := item.Properties.Labels["cluster"]
			region := item.Properties.RegionID
			availabilityZone := item.Properties.AvailabilityZone

			slog.Debug("raw cloud cost item",
				"item", item,
			)

			slog.Debug("processing cloud cost item",
				"account_id", item.Properties.AccountID,
				"service", item.Properties.Service,
				"category", item.Properties.Category,
				"all_labels", item.Properties.Labels,
				"region", region,
				"availability_zone", availabilityZone,
				"owner", owner,
				"environment", environment,
				"cluster", cluster,
				"list_cost", item.ListCost.Cost,
				"net_cost", item.NetCost.Cost,
				"amortized_net_cost", item.AmortizedNetCost.Cost,
				"invoiced_cost", item.InvoicedCost.Cost,
				"amortized_cost", item.AmortizedCost.Cost,
				"kube_percent", item.ListCost.KubernetesPercent,
			)

			key := costKey{
				providerID:       item.Properties.ProviderID,
				accountID:        item.Properties.AccountID,
				service:          item.Properties.Service,
				category:         item.Properties.Category,
				region:           region,
				availabilityZone: availabilityZone,
				owner:            owner,
				environment:      environment,
				cluster:          cluster,
			}

			if aggregated[key] == nil {
				aggregated[key] = &aggregatedCost{}
			}

			aggregated[key].listCost += item.ListCost.Cost
			aggregated[key].netCost += item.NetCost.Cost
			aggregated[key].amortizedNetCost += item.AmortizedNetCost.Cost
			aggregated[key].invoicedCost += item.InvoicedCost.Cost
			aggregated[key].amortizedCost += item.AmortizedCost.Cost
			aggregated[key].kubePercent = item.ListCost.KubernetesPercent
		}
	}

	slog.Debug("aggregation complete",
		"num_unique_keys", len(aggregated),
	)

	// Emit metrics for each aggregated cost
	for key, cost := range aggregated {
	labels := []string{key.providerID, key.accountID, key.service, key.category, key.region, key.availabilityZone, key.owner, key.environment, key.cluster}

		// Emit each cost type
		c.emitCost(ch, labels, "list", cost.listCost)
		c.emitCost(ch, labels, "net", cost.netCost)
		c.emitCost(ch, labels, "amortized_net", cost.amortizedNetCost)
		c.emitCost(ch, labels, "invoiced", cost.invoicedCost)
		c.emitCost(ch, labels, "amortized", cost.amortizedCost)

		// Emit kubernetes percent (only for amortized_net, to avoid duplication)
		if c.emitKubePercentMetrics {
			ch <- prometheus.MustNewConstMetric(
				c.kubePercent,
				prometheus.GaugeValue,
				cost.kubePercent,
				key.providerID, key.accountID, key.service, key.category, "amortized_net", key.region,
			)
		}
	}
}

func (c *CloudCostCollector) emitCost(ch chan<- prometheus.Metric, labels []string, costType string, value float64) {
	// Labels order: provider_id, account_id, service, category, region, availability_zone, owner, environment, cluster
	// Metric expects: provider_id, account_id, service, category, cost_type, region, availability_zone, owner, environment, cluster
	// We need to insert cost_type after category (index 4)
	fullLabels := make([]string, 0, len(labels)+1)
	fullLabels = append(fullLabels, labels[:4]...) // provider_id, account_id, service, category
	fullLabels = append(fullLabels, costType)       // cost_type
	fullLabels = append(fullLabels, labels[4:]...) // region, owner, environment, cluster
	ch <- prometheus.MustNewConstMetric(
		c.costTotal,
		prometheus.GaugeValue,
		value,
		fullLabels...,
	)
}

type aggregatedCost struct {
	listCost         float64
	netCost          float64
	amortizedNetCost float64
	invoicedCost     float64
	amortizedCost    float64
	kubePercent      float64
}

func (c *CloudCostCollector) emitExchangeRates(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Fetch exchange rates for configured currency symbols
	if len(c.currencySymbols) == 0 {
		return
	}
	rates, err := c.client.FetchExchangeRates(ctx, "USD", c.currencySymbols)
	if err != nil {
		slog.Error("failed to fetch exchange rates", "error", err)
		return
	}

	// Emit a metric for each currency rate
	for currency, rate := range rates.Rates {
		ch <- prometheus.MustNewConstMetric(
			c.exchangeRate,
			prometheus.GaugeValue,
			rate,
			rates.Base, currency,
		)
	}
}
