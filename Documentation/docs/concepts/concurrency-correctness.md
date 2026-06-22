---
id: concurrency-correctness
title: Concurrency and Correctness
sidebar_label: Concurrency and Correctness
description: How Loopers guarantees atomic budget enforcement under extreme concurrent load.
---

# Concurrency and Correctness

One of the most important goals of Loopers is to make sure your budget checking is 100 percent correct, even when thousands of requests arrive at the same time. This page explains how we do that.

## The Problem: Double Spending

Imagine two people share a bank account with 10 dollars in it. They both check the balance at the exact same second:

1. Person A checks the account and sees 10 dollars. They decide to spend 8 dollars.
2. Person B checks the account at the same second and also sees 10 dollars. They decide to spend 5 dollars.
3. Both purchases go through because they checked the balance before the other purchase was recorded.
4. Now they have spent 13 dollars in total, which is 3 dollars more than they had.

This is a concurrency race condition. In computer programming, it is called a Time of Check to Time of Use (TOCTOU) race. If two requests ask Loopers for permission at the same millisecond, a simple system might let both go through and bypass your limit.

## The Solution: Local Leases and Background Sync

To provide ultra-low latency while preventing massive double spending, Loopers uses a **local lease/budget cache** mechanism. 

### How the check works

1. Loopers reserves a larger chunk of budget (default $1.00 USD) from Redis as a local "lease".
2. When a request arrives, Loopers performs fast-path atomic deductions locally in memory from this lease (`RemainingNano`).
3. Loopers reconciles the spent totals back to Redis via background heartbeats every 5 seconds.
4. An asynchronous background worker runs every 2 seconds to check for threshold limits and block keys globally if the total budget is exceeded.

Because of this asynchronous design, the system is not strictly synchronous on every request. While this allows us to achieve incredibly fast proxy speeds, it means there can be up to $1.00 USD of budget leakage per key before the global block propagates. 

## Refunding Unused Budget

For streaming responses (where the AI sends back text word by word), Loopers overestimates the cost of the request at the start. Once the request is complete, Loopers compares the actual cost to the reserved amount and adjusts the local lease:

1. Before the call: Deduct the estimated cost from the local lease.
2. During the call: Count the actual tokens in the streaming response.
3. After the call: Calculate the real cost and add any unused estimated budget back to the local lease.

The background heartbeat eventually syncs the accurate spent amount to Redis.
