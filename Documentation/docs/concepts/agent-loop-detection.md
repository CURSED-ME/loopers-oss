---
id: agent-loop-detection
title: Agent Loop Detection
sidebar_label: Agent Loop Detection
description: Deep dive into how Loopers protects against autonomous agent loops.
---

# Agent Loop Detection

Autonomous agents (like those built with LangChain, LlamaIndex, or raw LLM API calls) can sometimes get stuck in loops. This happens when an agent repeatedly fails a task, gets stuck in an execution cycle, or encounters an unexpected error it cannot resolve, repeatedly burning tokens and budget.

Loopers provides a sophisticated, multi-layered loop detection engine to catch these runaway agents before they drain your budget. 

There are three configurable detection layers: **Fingerprint**, **Velocity**, and **Stall**.

---

## 1. Fingerprint Detection (Fuzzy SimHash)

The primary defense against agent loops is the **Fingerprint Detector**. It looks for highly repetitive requests within a sliding time window.

### Polymorphic Loops & SimHash
Early iterations of loop detection relied on exact cryptographic hashing (like SHA-256 or FNV-1a). However, modern agents often *mutate* their prompts slightly when retrying (e.g., adding "Attempt 2" to the prompt, or changing a few words). Exact hashing fails to catch these "polymorphic" loops.

To solve this, Loopers normalizes the JSON payload (stripping volatile fields like temperature or seed) and generates a **64-bit Locality Sensitive Hash (SimHash)** using 3-byte trigrams. 

When a new request arrives, Loopers compares its SimHash against recent requests in a Redis sliding window. If the **Hamming distance** (the number of differing bits) is less than or equal to `max_distance`, it is considered a match.

### Configuration
You can configure this in your `loopers.yaml` under `loop_detection.fingerprint`:

```yaml
loop_detection:
  enabled: true
  fingerprint:
    threshold: 3            # Block after 3 similar requests...
    window_seconds: 60      # ...within a 60-second window.
    max_distance: 3         # Max Hamming distance (0 = exact match only, 3 = fuzzy match)
```

> **Tip:** A `max_distance` of 3 allows for minor prompt mutations. If you experience false positives (e.g., in a chat application with very short, similar messages), try lowering `max_distance` to 1 or 0.

---

## 2. Velocity Detection

Sometimes an agent isn't sending the *same* prompt, but it is making requests at a clearly anomalous, high-frequency rate that a human never would. The **Velocity Detector** enforces maximum throughput rules per session.

### Configuration
```yaml
loop_detection:
  enabled: true
  velocity:
    max_rps: 5.0              # Block if the session exceeds 5 requests per second
    max_endpoint_repeats: 20  # Block if the same endpoint is hit 20 times...
    repeat_window_seconds: 10 # ...within 10 seconds.
```

If these limits are breached, the session is instantly blocked.

---

## 3. Stall Detection

The most advanced detection layer is the **Stall Detector**. It monitors the *evolution* of the agent's prompts over time.

An agent might not be hitting rate limits (Velocity) or sending identical prompts (Fingerprint), but it might still be "stuck" — making very tiny semantic changes to its prompts over many sequential turns without actually making progress on the task.

The Stall detector requires a minimum Hamming distance between sequential requests. If the agent fails to produce a prompt that is sufficiently "diverse" from the previous one for a certain number of turns, it triggers a stall.

### Configuration
```yaml
loop_detection:
  enabled: true
  stall:
    min_hamming_distance: 5     # The agent's prompt hash must change by at least 5 bits
    low_diversity_threshold: 10 # Trigger after 10 sequential low-diversity turns
    action: "warn"              # Action: "warn" (log only) or "block"
```

> **Warning:** Stall detection requires globally unique session IDs. If multiple users share the same session ID, their interleaved requests will pollute the stall state history.

---

## MCP Governance Circuit Breaker

Loopers also extends loop detection to Model Context Protocol (MCP) tool calls. If an agent repeatedly calls the exact same tool with the exact same arguments in a rapid loop, the **MCP Circuit Breaker** will trip and block the session.

See the [Configuration Reference](../reference/config.md) for MCP circuit breaker settings.
