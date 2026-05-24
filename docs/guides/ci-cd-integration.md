# CI/CD Integration Guide

Learn how to integrate Loopers budget verification into your GitHub Actions CI/CD pipeline using the official `loopers/budget-check` action.

## Scenario
If you have an automated workflow that runs regression tests against LLMs or spins up autonomous agents during testing, you can use Loopers to verify that there is enough budget remaining before starting the expensive runner, failing early if the remaining budget is too low.

## GitHub Action Usage

Add the `loopers/budget-check` step before your test execution steps:

```yaml
steps:
  - name: Check Out Repository
    uses: actions/checkout@v3

  - name: Verify AI Budget
    uses: loopers/budget-check@v1
    with:
      loopers-url: "https://loopers.my-org.com"
      loopers-key: ${{ secrets.LOOPERS_KEY }}
      key-hash: ${{ secrets.KEY_HASH }}
      min-remaining: 10.00 # Fail the build if remaining budget is less than $10.00 USD
```

## Inputs

- `loopers-url`: The accessible URL of your deployed Loopers proxy server.
- `loopers-key`: The raw Loopers key used for authentication.
- `key-hash`: The hash of the key whose budget status is to be checked.
- `min-remaining`: The minimum amount of remaining budget (USD) required. If the budget across any configured window is below this value, the action exits with code 1, failing the build.
