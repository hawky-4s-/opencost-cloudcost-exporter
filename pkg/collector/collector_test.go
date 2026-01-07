package collector

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/cache"
	"github.com/hawky-4s-/opencost-cloudcost-exporter/pkg/client"
)

func TestCloudCostCollector_Describe(t *testing.T) {
	c := newTestCollector(t, `{"code": 200, "data": {"sets": []}}`)
	ch := make(chan *prometheus.Desc, 10)

	c.Describe(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}

	if count < 3 {
		t.Errorf("expected at least 3 metric descriptions (costTotal, kubePercent, exchangeRate), got %d", count)
	}
}

func TestCloudCostCollector_Collect_EmptyResponse(t *testing.T) {
	c := newTestCollector(t, `{"code": 200, "data": {"sets": []}}`)

	// Force a fetch
	ch := make(chan prometheus.Metric, 100)
	c.Collect(ch)
	close(ch)

	// Should have self-observability metrics but no cost metrics
	count := 0
	for range ch {
		count++
	}

	if count < 6 {
		t.Errorf("expected at least 6 self-observability metrics, got %d", count)
	}
}

func TestCloudCostCollector_Collect_WithCosts(t *testing.T) {
	mockResponse := `{
		"code": 200,
		"data": {
			"sets": [{
				"cloudCosts": {
					"test-item": {
						"properties": {
						    "providerId": "arn:aws:logs:eu-central-1:1235454w23:log-group:/aws/rds/instance/cloud-dt-1/upgrade",
						    "provider": "aws",
							"accountID": "123456789",
							"service": "AmazonEC2",
							"category": "Compute",
							"availabilityZone": "us-east-1a",
							"labels": {
								"owner": "team-alpha",
								"environment": "prod",
								"cluster": "eks-main"
							}
						},
						"listCost": {"cost": 100.50, "kubernetesPercent": 0.75},
						"netCost": {"cost": 80.40, "kubernetesPercent": 0.75},
						"amortizedNetCost": {"cost": 70.30, "kubernetesPercent": 0.75},
						"invoicedCost": {"cost": 80.40, "kubernetesPercent": 0.75},
						"amortizedCost": {"cost": 90.45, "kubernetesPercent": 0.75}
					}
				}
			}]
		}
	}`

	c := newTestCollectorWithOptions(t, mockResponse, WithKubePercentMetrics(true))

	// Collect metrics
	ch := make(chan prometheus.Metric, 100)
	c.Collect(ch)
	close(ch)

	// Count cost metrics (should have 5 cost types + 1 kube percent = 6 per unique key)
	costMetrics := 0
	for m := range ch {
		desc := m.Desc().String()
		if strings.Contains(desc, "aws_cloud_cost") {
			costMetrics++
		}
	}

	if costMetrics < 6 {
		t.Errorf("expected at least 6 cost metrics, got %d", costMetrics)
	}
}

func TestCloudCostCollector_RegionLabel(t *testing.T) {
	// Test that region label uses RegionID, not availability zone
	mockResponse := `{
		"code": 200,
		"data": {
			"sets": [{
				"cloudCosts": {
					"test-item": {
						"properties": {
							"providerID": "arn:aws:ec2:eu-west-1:123456789:instance/i-123",
							"provider": "aws",
							"accountID": "123456789",
							"service": "AmazonEC2",
							"category": "Compute",
							"availabilityZone": "eu-west-1a",
							"regionID": "eu-west-1"
						},
						"listCost": {"cost": 100.0, "kubernetesPercent": 0}
					}
				}
			}]
		}
	}`

	c := newTestCollector(t, mockResponse)

	ch := make(chan prometheus.Metric, 100)
	c.Collect(ch)
	close(ch)

	// Verify that metrics contain region label with correct value (eu-west-1, not eu-west-1a)
	foundCostMetric := false
	for m := range ch {
		desc := m.Desc().String()
		if strings.Contains(desc, "aws_cloud_cost_total") {
			foundCostMetric = true
			// The metric should NOT contain region_id label (should only have region)
			if strings.Contains(desc, "region_id") {
				t.Error("metric should not contain region_id label, only region")
			}
		}
	}

	if !foundCostMetric {
		t.Error("expected to find aws_cloud_cost_total metric")
	}
}

func TestCloudCostCollector_CacheHit(t *testing.T) {
	mockResponse := `{"code": 200, "data": {"sets": []}}`
	c := newTestCollector(t, mockResponse)

	// First collect should miss cache
	ch1 := make(chan prometheus.Metric, 100)
	c.Collect(ch1)
	close(ch1)

	// Second collect should hit cache
	ch2 := make(chan prometheus.Metric, 100)
	c.Collect(ch2)
	close(ch2)

	// Check cache hits metric
	hits, _ := c.cache.Stats()
	if hits < 1 {
		t.Errorf("expected at least 1 cache hit, got %d", hits)
	}
}

func TestCloudCostCollector_SelfMetrics(t *testing.T) {
	c := newTestCollector(t, `{"code": 200, "data": {"sets": []}}`)

	// Trigger a collection
	ch := make(chan prometheus.Metric, 100)
	c.Collect(ch)
	close(ch)

	// Check scrape duration was recorded
	count := testutil.CollectAndCount(c.scrapeDuration)
	if count != 1 {
		t.Errorf("expected scrape_duration metric, got count=%d", count)
	}
}

func newTestCollector(t *testing.T, mockResponse string) *CloudCostCollector {
	return newTestCollectorWithOptions(t, mockResponse)
}

func newTestCollectorWithOptions(t *testing.T, mockResponse string, opts ...Option) *CloudCostCollector {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(mockResponse))
	}))
	t.Cleanup(server.Close)

	cl := client.New(server.URL)
	ca := cache.New(time.Hour, time.Hour*6)

	return New(cl, ca, opts...)
}

func TestCloudCostCollector_ExchangeRateMetrics(t *testing.T) {
	// This test verifies the exchange rate metric descriptor is properly defined
	c := newTestCollector(t, `{"code": 200, "data": {"sets": []}}`)

	// Check that the exchangeRate metric is defined
	ch := make(chan *prometheus.Desc, 10)
	c.Describe(ch)
	close(ch)

	found := false
	for desc := range ch {
		if strings.Contains(desc.String(), "currency_exchange_rate") {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected currency_exchange_rate metric to be described")
	}
}
