---
id: kubernetes-helm
title: Kubernetes and Helm
sidebar_label: Kubernetes and Helm
description: Deploy Loopers on Kubernetes using the official Helm chart.
---

# Kubernetes and Helm Deployment

Loopers includes an official Helm chart for deploying to Kubernetes clusters in production environments.

## Prerequisites

* Kubernetes version 1.24 or higher
* Helm version 3.10 or higher
* A running Redis instance (or use the built in Redis subchart)

## Quick Installation

```bash
# Add the Loopers repository
helm repo add loopers https://charts.tryloopers.com
helm repo update

# Install with default settings
helm install loopers loopers/loopers \
  --namespace loopers \
  --create-namespace \
  --set redis.enabled=true
```

## Custom values.yaml

```yaml
# values.yaml
replicaCount: 3

image:
  repository: ghcr.io/cursed-me/loopers
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 8080

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: loopers.yourdomain.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: loopers-tls
      hosts:
        - loopers.yourdomain.com

resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 100m
    memory: 64Mi

redis:
  enabled: true          # Use bundled Redis subchart
  auth:
    enabled: false
  master:
    persistence:
      enabled: true
      size: 1Gi

loopers:
  config:
    server:
      port: 8080
      log_level: info
    proxy:
      fail_closed: true
    budget:
      loop_threshold: 3
      session_ttl: 24h

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

metrics:
  enabled: true
  serviceMonitor:
    enabled: true        # For Prometheus Operator
```

## Apply Custom Values

```bash
helm upgrade --install loopers loopers/loopers \
  -f values.yaml \
  --namespace loopers
```

## Verifying the Deployment

```bash
# Check if pods are running
kubectl get pods -n loopers

# Check the service
kubectl get svc -n loopers

# Test health endpoint using port forwarding
kubectl port-forward svc/loopers 8080:8080 -n loopers
curl http://localhost:8080/health
```

## Using an External Redis

If you have a production Redis cluster already running:

```yaml
# values.yaml
redis:
  enabled: false    # Disable bundled Redis

loopers:
  config:
    redis:
      url: redis://your-redis-cluster:6379
```

Alternatively, you can use a Kubernetes Secret to store the Redis URL:

```bash
kubectl create secret generic loopers-redis \
  --from-literal=url="redis://:password@your-redis:6379" \
  -n loopers
```

```yaml
# values.yaml
loopers:
  redisSecretName: loopers-redis
```
