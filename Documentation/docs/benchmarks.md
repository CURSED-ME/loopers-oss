---
id: benchmarks
title: Performance Benchmarks
sidebar_label: Benchmarks
description: Loopers versus LiteLLM throughput, latency, budget leakage, and memory benchmarks.
---

# Performance Benchmarks

Loopers is built to handle huge spikes in traffic without failing to enforce your budget. Here are the results from our latest tests comparing Loopers with Python alternatives like LiteLLM.

:::note How We Tested
We sent 1,000 requests at the exact same time to both systems using the same computer hardware. LiteLLM was tested using its standard settings. You can view the raw data in the [project repository](https://github.com/CURSED-ME/llm-gateway-benchmark/blob/master/ep01-litellm/results/budget_leak_2026-06-19.md).
:::

## Test Results

| Metric | Loopers (Go) | LiteLLM | Result |
|---|---|---|---|
| **Budget Leakage** | **Capped (within lease)** | 0.17% ($0.000017) | Loopers limits leakage using local leases |
| **Requests Per Second** | **4,623** | around 176 | Loopers is 25 times faster |
| **Latency Overhead** | **241 milliseconds** | 46,813 milliseconds | Loopers has 190 times lower delay |
| **Memory Usage** | **41 MB** | 958 MB | Loopers is 23 times lighter |

## Why is Loopers So Much Faster?

### Language and Runtime
Loopers is written in Go. Go is a fast, compiled language that is very good at handling many tasks at the same time. LiteLLM is written in Python. Python is slower because it has to translate code as it runs and can only do one thing at a time.

### System Design
Loopers uses a highly optimized proxy program built into the Go standard library. This is combined with Redis database scripts that run all budget checks in a single step, making the system incredibly fast.

### Budget Checking Model
LiteLLM checks the budget after the AI call is finished. This means the AI can spend your money before LiteLLM even realizes the budget is gone. Loopers checks and reserves the budget before the call is sent. While Loopers uses local budget leases to achieve maximum throughput (allowing a potential, controlled leakage of up to $1.00 per key), it guarantees that runaway spend is capped, unlike post-call auditing tools.

## Feature Comparison

| Feature | Loopers | Bifrost | AgentBudget | LiteLLM |
|---|---|---|---|---|
| **Type** | Proxy Server | Proxy Server | Code Library | Proxy Server |
| **Check Before Call** | Yes (Atomic) | Yes | Yes | No |
| **Zero Storage Keys** | Yes | Yes | Yes | No |
| **Stops stuck loops** | Yes | No | Yes | No |
| **Fail Closed Design** | Yes | Varies | Not Applicable | No |
