// Package types defines the data structures for the OpenCost cloudCost API response.
package types

// CloudCostResponse represents the response from the /cloudCost endpoint.
type CloudCostResponse struct {
	Code int           `json:"code"`
	Data CloudCostData `json:"data"`
}

// CloudCostData contains the cost data sets.
type CloudCostData struct {
	Sets []CloudCostSet `json:"sets"`
}

// CloudCostSet represents a set of cloud costs for a time window.
type CloudCostSet struct {
	CloudCosts map[string]CloudCostItem `json:"cloudCosts"`
}

// CloudCostItem represents a single cloud cost entry.
type CloudCostItem struct {
	Properties       CloudCostProperties `json:"properties"`
	Window           Window              `json:"window"`
	ListCost         CostValue           `json:"listCost"`
	NetCost          CostValue           `json:"netCost"`
	AmortizedNetCost CostValue           `json:"amortizedNetCost"`
	InvoicedCost     CostValue           `json:"invoicedCost"`
	AmortizedCost    CostValue           `json:"amortizedCost"`
}

// CloudCostProperties contains metadata about the cloud cost.
type CloudCostProperties struct {
	ProviderID        string            `json:"providerID"`
	Provider          string            `json:"provider"`
	AccountID         string            `json:"accountID"`
	AccountName       string            `json:"accountName"`
	InvoiceEntityID   string            `json:"invoiceEntityID"`
	InvoiceEntityName string            `json:"invoiceEntityName"`
	AvailabilityZone  string            `json:"availabilityZone"`
	RegionID          string            `json:"regionID"`
	Service           string            `json:"service"`
	Category          string            `json:"category"`
	Labels            map[string]string `json:"labels,omitempty"`
}

// Window represents the time window for the cost data.
type Window struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// CostValue represents a cost amount with Kubernetes attribution.
type CostValue struct {
	Cost              float64 `json:"cost"`
	KubernetesPercent float64 `json:"kubernetesPercent"`
}

// ExchangeRateResponse represents the response from the Frankfurter API.
type ExchangeRateResponse struct {
	Amount float64            `json:"amount"`
	Base   string             `json:"base"`
	Date   string             `json:"date"`
	Rates  map[string]float64 `json:"rates"`
}
