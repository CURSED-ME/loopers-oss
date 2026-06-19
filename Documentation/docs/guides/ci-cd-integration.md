---
id: ci-cd-integration
title: CI/CD Integration
sidebar_label: CI/CD Integration
description: How to integrate Loopers budget enforcement into GitHub Actions and other CI pipelines.
---

# CI/CD Integration

Integrating Loopers into your test pipelines ensures that automated AI tests cannot run up huge API costs.

## GitHub Actions

### Setup as a Service Container

You can run Loopers as a background service alongside your tests:

```yaml
# .github/workflows/ai-tests.yml
name: AI Integration Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    services:
      redis:
        image: redis:7-alpine
        ports:
          - 6379:6379
      loopers:
        image: ghcr.io/cursed-me/loopers:latest
        env:
          LOOPERS_REDIS_URL: redis://redis:6379
          LOOPERS_SERVER_PORT: 8080
        ports:
          - 8080:8080

    steps:
      - uses: actions/checkout@v4

      - name: Wait for Loopers to be healthy
        run: |
          for i in $(seq 1 10); do
            curl -sf http://localhost:8080/health && break
            sleep 2
          done

      - name: Create CI budget key
        run: |
          KEY=$(curl -sf -X POST http://localhost:8080/api/keys \
            -H "Content-Type: application/json" \
            -d '{"name": "ci-run-${{ github.run_id }}", "provider": "openai"}' \
            | jq -r '.raw_key')
          echo "LOOPERS_KEY=$KEY" >> $GITHUB_ENV

      - name: Set CI budget
        run: |
          HASH=$(curl -sf http://localhost:8080/api/keys \
            | jq -r '.[] | select(.name == "ci-run-${{ github.run_id }}") | .hash')
          curl -sf -X POST http://localhost:8080/api/budget \
            -H "Content-Type: application/json" \
            -d "{\"hash\": \"$HASH\", \"daily\": 2.00}"

      - name: Run AI tests
        env:
          OPENAI_BASE_URL: http://localhost:8080/openai/v1
          OPENAI_API_KEY: ${{ env.LOOPERS_KEY }}
          LOOPERS_PROVIDER_KEY: ${{ secrets.OPENAI_API_KEY }}
        run: python -m pytest tests/ai/ -v
```

## Budget Isolation per Pull Request

You can give each pull request its own separate budget key to prevent multiple runs from sharing the same limits:

```bash
# In your test setup script
LOOPERS_KEY=$(loopers keys create \
  --name "pr-$PR_NUMBER-$RUN_ID" \
  --provider openai \
  | grep "Raw Key" | awk '{print $3}')

loopers budget set $KEY_HASH --daily 1.00
```

## Fail Fast on Budget Exhaustion

Configure your test runner to fail immediately when a budget limit is reached:

```python
# conftest.py
import pytest
from loopers_client import BudgetExceededError

@pytest.fixture(autouse=True)
def fail_on_budget_exceeded():
    yield
    # Check remaining budget after each test
    # Fail if budget is close to empty
```

## Cost Reporting

Report the cost of each run directly to your job summary:

```yaml
- name: Report AI Costs
  if: always()
  run: |
    STATUS=$(curl -sf "http://localhost:8080/api/budget/status/$KEY_HASH")
    SPENT=$(echo $STATUS | jq -r '.daily.spent')
    echo "### AI API Cost Report" >> $GITHUB_STEP_SUMMARY
    echo "| Window | Spent | Limit |" >> $GITHUB_STEP_SUMMARY
    echo "|---|---|---|" >> $GITHUB_STEP_SUMMARY
    echo "| Daily | \$$SPENT | \$2.00 |" >> $GITHUB_STEP_SUMMARY
```
