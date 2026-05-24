# Budget Windows

Loopers allows you to specify budget limits across 5 distinct time windows, helping you manage both short-term burst traffic and long-term costs.

## Supported Windows

1. **Minute**: Used to enforce short-term burst caps (e.g. max $0.50/minute) to protect against runaway loops.
2. **Hourly**: Captures hourly spikes.
3. **Daily**: Enforces standard daily limits (e.g. $10.00/day).
4. **Weekly**: Limits cumulative cost per ISO week.
5. **Monthly**: Enforces long-term monthly budgets.

## Configuration

Budgets are configured via the Loopers CLI using the `loopers budget set` command:

```bash
loopers budget set <key-hash> \
  --minute 0.50 \
  --hourly 2.00 \
  --daily 10.00 \
  --weekly 50.00 \
  --monthly 200.00
```

## Atomic Multi-Window Checkout & Rollback

When a request is received, Loopers estimates its maximum cost (based on input size + model default max output tokens). It then atomically checks and reserves this estimated cost across **all configured windows** in Redis.

If any single window check fails:
- The request is immediately blocked (returns a `429 Too Many Requests` error).
- All successful reservations made for other windows during that request are **atomically rolled back** to ensure correct state.
