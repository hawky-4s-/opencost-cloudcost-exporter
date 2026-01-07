# Metrics Reference

Complete reference for all metrics exposed by opencost-cloudcost-exporter.

## Cost Metrics

### `aws_cloud_cost_total`

AWS cloud cost in USD for the configured time window.

| Label               | Description               | Example                         |
|---------------------|---------------------------|---------------------------------|
| `provider_id`       | AWS resource ARN          | `arn:aws:ec2:eu-west-1:123:...` |
| `account_id`        | AWS Account ID            | `883112916672`                  |
| `service`           | AWS Service name          | `AmazonEC2`, `AmazonRDS`        |
| `category`          | Cost category             | `Compute`, `Storage`, `Network` |
| `cost_type`         | Type of cost calculation  | `amortized_net`                 |
| `region`            | AWS region                | `eu-west-1`                     |
| `availability_zone` | AWS availability zone     | `eu-west-1a`                    |
| `owner`             | Owner label from resource | `team-alpha`                    |
| `environment`       | Environment label         | `prod`, `staging`               |
| `cluster`           | Kubernetes cluster name   | `eks-main`                      |

### `aws_cloud_cost_kubernetes_percent`

Percentage of the cost attributed to Kubernetes workloads (0-1 scale).

> **Note**: This metric is disabled by default. Enable with `--emit-kube-percent-metrics=true` or set `emitKubePercentMetrics: true` in Helm values.

| Label         | Description              | Example           |
|---------------|--------------------------|-------------------|
| `provider_id` | AWS resource ARN         | `arn:aws:ec2:...` |
| `account_id`  | AWS Account ID           | `883112916672`    |
| `service`     | AWS Service name         | `AmazonEC2`       |
| `category`    | Cost category            | `Compute`         |
| `cost_type`   | Type of cost calculation | `amortized_net`   |
| `region`      | AWS region               | `eu-west-1`       |

### `currency_exchange_rate`

Currency exchange rate from base currency to target currency (fetched from Frankfurter API).

| Label    | Description     | Example |
|----------|-----------------|---------|
| `base`   | Base currency   | `USD`   |
| `target` | Target currency | `EUR`   |

## Cost Types

| Type            | Description                                    | Use Case                  |
|-----------------|------------------------------------------------|---------------------------|
| `list`          | On-demand list price                           | Compare to public pricing |
| `net`           | Actual spend with discounts                    | Current period costs      |
| `amortized_net` | **Recommended** - RI/SP costs spread over term | Dashboard default         |
| `invoiced`      | What appears on invoice                        | Billing reconciliation    |
| `amortized`     | RI/SP amortized without discounts              | Financial planning        |

## Self-Observability Metrics

### `cloudcost_exporter_info`

Build information about the exporter. Always has value `1`.

| Label     | Description      |
|-----------|------------------|
| `version` | Semantic version |
| `commit`  | Git commit SHA   |
| `date`    | Build timestamp  |

### `cloudcost_exporter_scrape_duration_seconds`

Histogram of time taken to fetch data from OpenCost API.

### `cloudcost_exporter_scrape_errors_total`

Counter of failed scrape attempts.

### `cloudcost_exporter_cache_hits_total`

Counter of requests served from cache.

### `cloudcost_exporter_cache_misses_total`

Counter of cache misses requiring API fetch.

### `cloudcost_exporter_cache_age_seconds`

Current age of cached data in seconds.

### `cloudcost_exporter_last_successful_scrape_timestamp`

Unix timestamp of the last successful OpenCost API fetch.

## Recording Rules

Pre-aggregated metrics deployed via Helm PrometheusRule:

### Single Dimension Aggregations

| Rule                                  | Expression                                                               |
|---------------------------------------|--------------------------------------------------------------------------|
| `aws_cloud_cost:by_owner:daily`       | `sum by (owner) (aws_cloud_cost_total{cost_type="amortized_net"})`       |
| `aws_cloud_cost:by_service:daily`     | `sum by (service) (aws_cloud_cost_total{cost_type="amortized_net"})`     |
| `aws_cloud_cost:by_region:daily`      | `sum by (region) (aws_cloud_cost_total{cost_type="amortized_net"})`      |
| `aws_cloud_cost:by_environment:daily` | `sum by (environment) (aws_cloud_cost_total{cost_type="amortized_net"})` |
| `aws_cloud_cost:by_category:daily`    | `sum by (category) (aws_cloud_cost_total{cost_type="amortized_net"})`    |
| `aws_cloud_cost:by_cluster:daily`     | `sum by (cluster) (aws_cloud_cost_total{cost_type="amortized_net"})`     |
| `aws_cloud_cost:by_account:daily`     | `sum by (account_id) (aws_cloud_cost_total{cost_type="amortized_net"})`  |
| `aws_cloud_cost:total:daily`          | `sum(aws_cloud_cost_total{cost_type="amortized_net"})`                   |

### Multi-Dimension Aggregations

| Rule                                      | Expression                                                                       |
|-------------------------------------------|----------------------------------------------------------------------------------|
| `aws_cloud_cost:by_owner_service:daily`   | `sum by (owner, service) (aws_cloud_cost_total{cost_type="amortized_net"})`      |
| `aws_cloud_cost:by_owner_region:daily`    | `sum by (owner, region) (aws_cloud_cost_total{cost_type="amortized_net"})`       |
| `aws_cloud_cost:by_service_region:daily`  | `sum by (service, region) (aws_cloud_cost_total{cost_type="amortized_net"})`     |
| `aws_cloud_cost:by_account_service:daily` | `sum by (account_id, service) (aws_cloud_cost_total{cost_type="amortized_net"})` |

### List Cost & Savings

| Rule                                   | Expression                                                  |
|----------------------------------------|-------------------------------------------------------------|
| `aws_cloud_cost_list:by_owner:daily`   | `sum by (owner) (aws_cloud_cost_total{cost_type="list"})`   |
| `aws_cloud_cost_list:by_service:daily` | `sum by (service) (aws_cloud_cost_total{cost_type="list"})` |
| `aws_cloud_cost_list:total:daily`      | `sum(aws_cloud_cost_total{cost_type="list"})`               |
| `aws_cloud_cost:savings:daily`         | `sum(list) - sum(amortized_net)`                            |
| `aws_cloud_cost:savings_percent:daily` | `(sum(list) - sum(amortized_net)) / sum(list) * 100`        |

## Example Queries

```promql
# Total daily cost
aws_cloud_cost:total:daily

# Cost by service (top 10)
topk(10, aws_cloud_cost:by_service:daily)

# Cost by owner
aws_cloud_cost:by_owner:daily

# Cost by region
aws_cloud_cost:by_region:daily

# Savings from reservations/discounts
aws_cloud_cost:savings:daily

# Savings percentage
aws_cloud_cost:savings_percent:daily

# Kubernetes-attributed costs only
sum(aws_cloud_cost_total{cost_type="amortized_net"} * aws_cloud_cost_kubernetes_percent)

# Currency conversion (USD to EUR)
aws_cloud_cost:total:daily * on() currency_exchange_rate{base="USD", target="EUR"}
```
