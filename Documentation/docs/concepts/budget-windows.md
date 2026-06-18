---
id: budget-windows
title: Budget Windows
sidebar_label: Budget Windows
description: How Loopers enforces spending limits across 5 granular time windows.
---

# Budget Windows

Loopers allows you to set spending limits across five time windows. Each window works independently. The first limit you reach will block your requests.

## Available Windows

| Window | What it does | Example Use Case |
|---|---|---|
| minute | Limits spending per minute | Stops sudden spamming |
| hourly | Limits spending per hour | Limits budget for testing |
| daily | Limits spending per day | Daily budget allowance |
| weekly | Limits spending per week | Weekly budget limits |
| monthly | Limits spending per month | Monthly budget caps |

## Setting Budget Limits

You can set budgets using the command line:

```bash
loopers budget set KEY_HASH --minute 0.50 --hourly 2.00 --daily 10.00 --weekly 50.00 --monthly 150.00
```

All windows are optional. You can set limits on only one window, or combine any number of them.

## How Limits are Checked

Before forwarding a request, Loopers runs a script in Redis:

1. It reads the limits you configured for that key.
2. It checks how much money has already been spent in each window.
3. If spending in any window goes over the limit, it blocks the request.
4. If there is enough budget, it reserves the cost of the request.
5. After the AI responds, it updates the actual cost and refunds any unused amount.

This script runs in a single step, which means it is completely safe from concurrency errors.

## Checking Current Spend

You can check how much money has been spent in each window:

```bash
loopers budget status KEY_HASH
```

Example output:

```
KEY: lp-hash-abc123
  minute   spent=$0.12   limit=$0.50   (24%)
  hourly   spent=$0.89   limit=$2.00   (44%)
  daily    spent=$3.42   limit=$10.00  (34%)
  weekly   spent=$12.10  limit=$50.00  (24%)
  monthly  spent=$28.40  limit=$150.00 (19%)
```

## When do Windows Reset?

Windows reset automatically based on the clock:
* Minute resets at the start of every minute.
* Hourly resets at the start of every hour.
* Daily resets at midnight UTC.
* Weekly resets every Monday at midnight UTC.
* Monthly resets on the first day of every month at midnight UTC.

No background tasks are required to clean up or reset these budgets. Redis deletes old records automatically.

:::tip Hint
Use a small limit for the minute window, such as ten cents, to catch runaway loops before they consume a large portion of your daily budget.
:::
