package types

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCloudCostResponseUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantCode int
		wantSets int
	}{
		{
			name:     "empty response",
			input:    `{"code": 200, "data": {"sets": []}}`,
			wantCode: 200,
			wantSets: 0,
		},
		{
			name:     "single set",
			input:    `{"code": 200, "data": {"sets": [{"cloudCosts": {}}]}}`,
			wantCode: 200,
			wantSets: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp CloudCostResponse
			if err := json.Unmarshal([]byte(tt.input), &resp); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if resp.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", resp.Code, tt.wantCode)
			}
			if len(resp.Data.Sets) != tt.wantSets {
				t.Errorf("Sets count = %v, want %v", len(resp.Data.Sets), tt.wantSets)
			}
		})
	}
}

func TestCloudCostItemUnmarshal(t *testing.T) {
	input := `{
		"properties": {
			"providerID": "test-id",
			"provider": "AWS",
			"accountID": "123456789",
			"service": "AmazonEC2",
			"category": "Compute",
			"labels": {"owner": "team-alpha", "environment": "prod"}
		},
		"window": {
			"start": "2026-01-01T00:00:00Z",
			"end": "2026-01-02T00:00:00Z"
		},
		"listCost": {"cost": 100.50, "kubernetesPercent": 0.75},
		"netCost": {"cost": 80.40, "kubernetesPercent": 0.75},
		"amortizedNetCost": {"cost": 70.30, "kubernetesPercent": 0.75},
		"invoicedCost": {"cost": 80.40, "kubernetesPercent": 0.75},
		"amortizedCost": {"cost": 90.45, "kubernetesPercent": 0.75}
	}`

	var item CloudCostItem
	if err := json.Unmarshal([]byte(input), &item); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	// Verify properties
	if item.Properties.ProviderID != "test-id" {
		t.Errorf("ProviderID = %v, want test-id", item.Properties.ProviderID)
	}
	if item.Properties.Provider != "AWS" {
		t.Errorf("Provider = %v, want AWS", item.Properties.Provider)
	}
	if item.Properties.Service != "AmazonEC2" {
		t.Errorf("Service = %v, want AmazonEC2", item.Properties.Service)
	}
	if item.Properties.Category != "Compute" {
		t.Errorf("Category = %v, want Compute", item.Properties.Category)
	}

	// Verify labels
	if item.Properties.Labels["owner"] != "team-alpha" {
		t.Errorf("Labels[owner] = %v, want team-alpha", item.Properties.Labels["owner"])
	}
	if item.Properties.Labels["environment"] != "prod" {
		t.Errorf("Labels[environment] = %v, want prod", item.Properties.Labels["environment"])
	}

	// Verify costs
	if item.ListCost.Cost != 100.50 {
		t.Errorf("ListCost.Cost = %v, want 100.50", item.ListCost.Cost)
	}
	if item.NetCost.Cost != 80.40 {
		t.Errorf("NetCost.Cost = %v, want 80.40", item.NetCost.Cost)
	}
	if item.AmortizedNetCost.Cost != 70.30 {
		t.Errorf("AmortizedNetCost.Cost = %v, want 70.30", item.AmortizedNetCost.Cost)
	}
	if item.ListCost.KubernetesPercent != 0.75 {
		t.Errorf("KubernetesPercent = %v, want 0.75", item.ListCost.KubernetesPercent)
	}

	// Verify window
	if item.Window.Start != "2026-01-01T00:00:00Z" {
		t.Errorf("Window.Start = %v, want 2026-01-01T00:00:00Z", item.Window.Start)
	}
}

func TestCloudCostPropertiesWithoutLabels(t *testing.T) {
	input := `{
		"providerID": "test-id",
		"provider": "AWS",
		"accountID": "123456789",
		"service": "AmazonS3",
		"category": "Storage"
	}`

	var props CloudCostProperties
	if err := json.Unmarshal([]byte(input), &props); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if props.Labels != nil {
		t.Errorf("Labels should be nil when not present, got %v", props.Labels)
	}
	if props.Service != "AmazonS3" {
		t.Errorf("Service = %v, want AmazonS3", props.Service)
	}
}

func TestCloudCostResponseFromFixture(t *testing.T) {
	// Load fixture file
	data, err := os.ReadFile("testdata/cloudcost-response.json")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	var resp CloudCostResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}

	// Verify response structure
	if resp.Code != 200 {
		t.Errorf("Code = %v, want 200", resp.Code)
	}
	if len(resp.Data.Sets) != 1 {
		t.Fatalf("Sets count = %v, want 1", len(resp.Data.Sets))
	}
	if len(resp.Data.Sets[0].CloudCosts) != 3 {
		t.Errorf("CloudCosts count = %v, want 3", len(resp.Data.Sets[0].CloudCosts))
	}

	// Verify specific services are present
	services := make(map[string]bool)
	for _, item := range resp.Data.Sets[0].CloudCosts {
		services[item.Properties.Service] = true
	}

	expectedServices := []string{"AmazonEC2", "AmazonRDS", "AmazonElastiCache"}
	for _, svc := range expectedServices {
		if !services[svc] {
			t.Errorf("expected service %s not found", svc)
		}
	}

	// Verify label extraction
	for _, item := range resp.Data.Sets[0].CloudCosts {
		if item.Properties.Service == "AmazonEC2" {
			if item.Properties.Labels["owner"] != "team-alpha" {
				t.Errorf("EC2 owner = %v, want team-alpha", item.Properties.Labels["owner"])
			}
			if item.Properties.Labels["environment"] != "prod" {
				t.Errorf("EC2 environment = %v, want prod", item.Properties.Labels["environment"])
			}
			if item.ListCost.Cost != 1500.50 {
				t.Errorf("EC2 ListCost = %v, want 1500.50", item.ListCost.Cost)
			}
			if item.ListCost.KubernetesPercent != 0.85 {
				t.Errorf("EC2 KubernetesPercent = %v, want 0.85", item.ListCost.KubernetesPercent)
			}
		}
	}

	// Calculate total costs
	var totalListCost float64
	for _, item := range resp.Data.Sets[0].CloudCosts {
		totalListCost += item.ListCost.Cost
	}
	expectedTotal := 1500.50 + 500.00 + 200.00
	if totalListCost != expectedTotal {
		t.Errorf("Total ListCost = %v, want %v", totalListCost, expectedTotal)
	}
}
