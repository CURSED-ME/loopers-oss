---
id: monitoring-grafana
title: Monitoring and Grafana
sidebar_label: Monitoring and Grafana
description: Set up Prometheus metrics and the pre-built Grafana dashboard for Loopers.
---

# Monitoring and Grafana

Loopers provides built-in Prometheus metrics to monitor your AI application spending and requests. A pre-built Grafana dashboard is included in our repository to give you instant visibility.

## Metrics Endpoint

By default, metrics are served at:
`http://localhost:9090/metrics`

You can test this endpoint using:

```bash
curl http://localhost:9090/metrics | grep loopers_
```

## Available Metrics

Here are the real metrics tracked by Loopers:

| Metric | Type | Labels | Description |
|---|---|---|---|
| loopers_requests_total | Counter | provider, model, status | Total requests processed by the proxy |
| loopers_budget_blocks_total | Counter | provider, window | Requests blocked by budget window limits |
| loopers_loop_blocks_total | Counter | provider | Requests blocked by agent loop detection |
| loopers_loop_warns_total | Counter | provider | Warnings triggered by loop detection |
| loopers_shadow_blocked_total | Counter | provider, window | Requests that would be blocked in dry run mode |
| loopers_spend_usd_total | Counter | provider, key_hash | Total spending tracked in USD |
| loopers_request_duration_seconds | Histogram | provider | End to end request duration in seconds |
| loopers_tokens_total | Counter | provider, direction | Total input or output tokens processed |

## Prometheus Configuration

Add Loopers as a target in your `prometheus.yml` configuration:

```yaml
scrape_configs:
  - job_name: 'loopers'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 15s
```

## Grafana Dashboard

The dashboard file is located in the `./grafana` directory in the repository. To import it:

1. Open Grafana and click on **Dashboards** then **Import**.
2. Upload the `./grafana/loopers-dashboard.json` file.
3. Select your Prometheus data source.
4. Click **Import**.

### Dashboard Panels

The dashboard displays:
* **Request Throughput**: Requests per second over time.
* **Block Rate**: Percentage of requests blocked by budgets.
* **Budget Window Hits**: Which window is closest to its limit.
* **Latency**: Request duration metrics.
* **Cost by Provider**: Cumulative spending per AI provider.
* **Loops Detected**: stuck agent loops blocked over time.

## Docker Compose Monitoring

You can run Loopers, Prometheus, and Grafana together:

```yaml
# docker-compose.monitoring.yml
version: '3.8'
services:
  loopers:
    image: ghcr.io/cursed-me/loopers:latest
    ports:
      - 8080:8080
      - 9090:9090

  prometheus:
    image: prom/prometheus:latest
    ports:
      - 9091:9090
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml

  grafana:
    image: grafana/grafana:latest
    ports:
      - 3000:3000
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - ./grafana:/etc/grafana/provisioning/dashboards
```

Start the services:
```bash
docker-compose -f docker-compose.monitoring.yml up -d
```
Grafana is now available at http://localhost:3000 using username admin and password admin.

## Alerting Rules

You can set up alerts in Prometheus using these rules:

```yaml
# loopers-alerts.yml
groups:
  - name: loopers
    rules:
      - alert: HighBlockRate
        expr: rate(loopers_budget_blocks_total[5m]) / rate(loopers_requests_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning
        annotations:
          summary: "High budget block rate (above 10%)"

      - alert: LoopDetectionSpike
        expr: rate(loopers_loop_blocks_total[5m]) > 1
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Agent loops detected (possible runaway agent)"
```
