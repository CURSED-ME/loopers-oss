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
        image: ghcr.io/try-loopers/loopers:latest
        env:
          REDIS_ADDR: redis:6379
          SERVER_PORT: 8080
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
          OUTPUT=$(loopers keys create --name "ci-run-${{ github.run_id }}" --provider openai)
          KEY=$(echo "$OUTPUT" | grep -i "Raw Key" | awk '{print $3}')
          HASH=$(echo "$OUTPUT" | grep -i "Hash" | awk '{print $2}')
          echo "LOOPERS_KEY=$KEY" >> $GITHUB_ENV
          echo "KEY_HASH=$HASH" >> $GITHUB_ENV

      - name: Set CI budget
        run: |
          loopers budget set $KEY_HASH --daily 2.00

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
from openai import APIStatusError

@pytest.fixture(autouse=True)
def fail_on_budget_exceeded():
    yield
    # You can catch standard API status errors
    # and check if 'budget exceeded' is in the message
```

## Cost Reporting

Report the cost of each run directly to your job summary:

```yaml
- name: Report AI Costs
  if: always()
  run: |
    echo "### AI API Cost Report" >> $GITHUB_STEP_SUMMARY
    echo '```text' >> $GITHUB_STEP_SUMMARY
    loopers budget status $KEY_HASH >> $GITHUB_STEP_SUMMARY
    echo '```' >> $GITHUB_STEP_SUMMARY
```
