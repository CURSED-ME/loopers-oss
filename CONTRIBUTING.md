# Contributing to Loopers

We welcome community contributions! Please review these guidelines before submitting a Pull Request.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct (detailed in `CODE_OF_CONDUCT.md`). We expect all participants to maintain a respectful, inclusive, and professional environment.

## Development Setup

1. **Go Toolchain:** Loopers requires Go 1.26.1+.
2. **Local Redis:** A running Redis instance is required to execute the budget engine test suite.
3. **Linting:** Ensure you format your code before opening a PR:
   ```bash
   gofmt -w .
   ```

## Pull Request Guidelines

1. **Keep it Correct:** Loopers prioritizes correctness and security over completeness. Every change should adhere strictly to the budget enforcement guarantees.
2. **Add Tests:** Any new feature or bug fix must include corresponding unit or integration tests (e.g. adding mock SSE streams to verify streaming parse regressions).
3. **Run CI checks locally:**
   ```bash
   go test -v -race ./...
   ```

## Adding a New Provider

If you want to add support for a new AI provider, please follow our detailed [Provider Contribution Guide](file:///c:/Users/varad/loopers-oss/docs/contributing/adding-a-provider.md).

It walks you through implementing the `Provider` interface, registering the provider, updating the pricing configurations, and validating changes.

