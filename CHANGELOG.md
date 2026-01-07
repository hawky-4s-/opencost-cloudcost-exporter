# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of opencost-cloudcost-exporter
- Prometheus metrics for AWS cloud costs from OpenCost
- Cost metrics: `aws_cloud_cost_total`, `aws_cloud_cost_kubernetes_percent`
- Support for 5 cost types: list, net, amortized_net, invoiced, amortized
- Labels: account_id, service, category, cost_type, region, owner, environment, cluster
- In-memory cache with TTL and stale data support
- Self-observability metrics (scrape duration, errors, cache stats)
- Build info metric (`cloudcost_exporter_info`)
- Health endpoints: `/healthz`, `/readyz`
- Structured JSON logging
- Helm chart with ServiceMonitor and PrometheusRule
- Recording rules for dashboard performance
- Alerting rules for spend thresholds and cost spikes
- Multi-stage Dockerfile with distroless base
- GitHub Actions CI/CD workflows
