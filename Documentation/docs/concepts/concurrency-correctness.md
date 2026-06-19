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

## The Solution: Redis Lua Scripts

To prevent this problem, Loopers uses special scripts that run inside the Redis database. Redis guarantees that these scripts run in a single, unbreakable step. This is called an atomic operation. No other database commands can run while the script is executing.

### How the check works

A simplified version of the Redis script does the following:

1. Reads the current spending total from the database.
2. Reads the limit you set.
3. Calculates the estimated cost of the new request.
4. Checks if the current spend plus the estimated cost goes over the limit.
5. If it does, it returns a block signal immediately.
6. If it does not, it adds the estimated cost to the spending total and returns an allowed signal.

Because the check and the reservation happen in one single step, 1,000 requests arriving at the same time are queued up and processed one by one. This means your budget limits will never be bypassed.

## Multi Window Atomicity

Loopers checks all five budget windows (minute, hourly, daily, weekly, monthly) in one single Redis transaction. Either all windows pass and the cost is reserved, or the request is blocked. There is no in between state.

## Refunding Unused Budget

For streaming responses (where the AI sends back text word by word), Loopers overestimates the cost of the request at the start. Once the request is complete, Loopers compares the actual cost to the reserved amount and refunds any unused budget back to your account:

1. Before the call: Reserve the estimated cost.
2. During the call: Count the actual tokens in the streaming response.
3. After the call: Calculate the real cost and adjust the budget total in Redis (refunding any difference).

This ensures your database always has the exact, correct cost recorded.
