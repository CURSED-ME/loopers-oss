# Episode 1: Loopers vs. LiteLLM (Final Benchmark Results)

This document contains the official benchmark results for Episode 1 of the LLM Gateway Benchmarks, comparing **Loopers (Go)** against **LiteLLM (Python/FastAPI)**.

## The Objective
Companies deploying LLM gateways do so to control costs, unify APIs, and track usage. Our goal is to evaluate the raw infrastructure performance and budget correctness of these gateways under concurrent load. 

> [!NOTE]
> **Disclaimer on Scope:** This benchmark is strictly about infrastructure performance (throughput, latency, footprint, and budget enforcement). It does not evaluate feature breadth. LiteLLM supports over 100+ providers and has an extensive feature set including SSO and RBAC. Loopers is a lightweight proxy supporting exactly 10 providers with a narrow focus on core proxy performance and atomic budget tracking. Both are evaluated using their official, recommended production configurations.

---

## 1. Budget Leak Test (The Primary Attack Vector)
**Goal:** Prove whether a proxy enforces a hard budget accurately under concurrent load.

**The Setup:**
- We gave each proxy a strict **$0.01** budget limit and sent **1,000 concurrent requests** simultaneously.
- At $0.0000315 per request, the $0.01 budget mathematically allows exactly **~317 requests**.

**The Results:**
| Metric | Loopers | LiteLLM |
|---|---|---|
| Budget limit | $0.01 | $0.01 |
| Requests allowed | **317** (Exact limit) | **1000** (Complete Leak) |
| Actual spend allowed | **$0.009985** | ~$0.0315 |
| Overspend | **$0.00** (0% leakage) | **$0.0215** (215% leakage) |

> [!WARNING]
> **Architectural Takeaway:**
> If a gateway cannot enforce a budget limit under moderate concurrency, it risks significant financial exposure. Loopers leverages a distributed lease system backed by atomic Redis Lua scripts, blocking requests exactly at the limit without data loss. LiteLLM utilizes a Python background worker to track spend asynchronously. Under high concurrency spikes, database updates lag behind incoming traffic, resulting in a Time-of-Check to Time-of-Use (TOCTOU) race condition where the budget limit is bypassed entirely before the tracker catches up.

---

## 2. Peak Throughput Test (Max RPS)
**Goal:** Find the maximum requests-per-second each proxy can sustain before dropping or erroring.

**The Setup:**
- Budgets uncapped, upstream mock responding instantly.
- Ramped from 10 to 2,000 concurrent Virtual Users (VUs) over 5 minutes.

**The Results:**
| Metric | Loopers | LiteLLM |
|---|---|---|
| Total HTTP Requests | 1,388,886 | 58,300 |
| Average RPS | **~4,623 req/s** | ~176.7 req/s |
| HTTP 200 OK Responses | 1,341,675 (96.6%) | 56,623 (97.1%) |
| Failed Requests | 47,211 (3.4%) | 1,677 (2.9%) |

> [!TIP]
> **Architectural Takeaway:**
> Loopers' Go/Gin multi-threaded architecture allows it to scale linearly with CPU cores. The Go scheduler multiplexes requests across available threads, absorbing the heavy traffic efficiently. LiteLLM, built on Python and `asyncio`, relies on an event loop that is fundamentally limited to a single thread per worker by the Python Global Interpreter Lock (GIL). This architectural trade-off gives LiteLLM a massive ecosystem advantage but imposes a strict ceiling on raw throughput.

---

## 3. Proxy Overhead Latency
**Goal:** Measure the latency the proxy itself adds to every request, isolated from upstream time.

**The Setup:**
- 500 VUs sustaining load for 60 seconds against a mock upstream with a fixed 50ms delay.
- *Proxy overhead = Measured total latency - 50ms upstream delay.*

**The Results:**
| Metric | Loopers Overhead | LiteLLM Overhead |
|---|---|---|
| P50 Latency | 52.63 ms | 1,445.41 ms |
| P90 Latency | 140.89 ms | 2,344.56 ms |
| P99 Latency | **240.98 ms** | **46,812.60 ms** |

> [!NOTE]
> **Architectural Takeaway:**
> LLM calls are notoriously slow; a proxy should ideally remain invisible. We measure P99 latency overhead because the 99th percentile represents real pain for users. Under a saturated load of 500 concurrent VUs, Loopers maintained stability (0.95% error rate) but experienced queueing that pushed its P99 overhead to 240.98ms. While Loopers did not hit the ideal <2ms target under this extreme stress test, LiteLLM buckled significantly, dropping 22.53% of its requests due to socket hangs and generating a P99 overhead of 46.8 seconds as the Python event loop became completely overwhelmed.

---

## 4. Resource Footprint
**Goal:** Measure the deployment cost of each proxy in terms of container size and memory.

**The Setup:**
- Evaluated Docker image size.
- Measured RSS Memory via `docker stats` after 60 seconds of idle, and again under 500 VU sustained load.
- Containers restricted to uniform `2g` limits to ensure a fair baseline.

**The Results:**
| Metric | Loopers | LiteLLM |
|---|---|---|
| Image Size (Disk) | 102 MB | 5.6 GB |
| Required Containers | 2 (Proxy + Redis) | 3 (Proxy + Redis + PostgreSQL) |
| **Total Idle Memory** | **41.58 MB** | **957.83 MB** |
| **Total Load Memory** | **75.74 MB** | **1.14 GB** |

> [!TIP]
> **Architectural Takeaway:**
> For infrastructure engineers, resource consumption at scale dictates deployment flexibility. LiteLLM requires PostgreSQL for production-grade budget tracking, and its three-container stack sits at nearly a gigabyte of RAM even when idle. Loopers, a statically linked Go binary, requires only two containers and idles at 41 MB, representing a 93% reduction in memory requirements. This tiny footprint enables Loopers to be deployed flexibly, such as a sidecar proxy, without contending for node resources.

---

## Reproducibility
We believe benchmarks must be fully open and reproducible. We have open-sourced the entire benchmark harness in a separate repository. You can run this exact test suite on your own machine:

1. Ensure Docker and `k6` are installed.
2. Clone the benchmark repository: `git clone https://github.com/CURSED-ME/llm-gateway-benchmark.git`
3. Navigate to the Episode 1 test environment and start the stack: `cd llm-gateway-benchmark/ep01-litellm && docker-compose up -d`
4. Seed the keys: `./seed.sh --leak`
5. Run the load test: `k6 run -e PROXY=loopers -e VUS=1000 -e DURATION=10s ../shared/harness/budget_leak_test.js`
