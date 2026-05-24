# GA Release Checklist

Use this checklist to verify and execute release gates for any minor or major version of Loopers.

---

## 1. Pre-Release Verification Gates

- [ ] **Formatting and Linting:**
  - Run formatting tools and ensure zero lint issues:
    ```bash
    gofmt -w .
    golangci-lint run
    ```
- [ ] **Test Suite Execution:**
  - Execute the complete test suite with the race detector enabled (requires Redis):
    ```bash
    go test -v -race -count=1 ./...
    ```
- [ ] **Key Leakage Scan:**
  - Audit tests and codebase to ensure no developer credentials or provider API keys are committed:
    ```bash
    go test -v ./... 2>&1 | grep -E '(sk-[A-Za-z0-9]{32,}|sk-ant-api03-[A-Za-z0-9]{93,}|AIza[0-9A-Za-z]{35})'
    ```
- [ ] **Vulnerability Audit:**
  - Verify zero known vulnerabilities in Go dependencies:
    ```bash
    go install golang.org/x/vuln/cmd/govulncheck@latest
    govulncheck ./...
    ```

---

## 2. Model & Pricing Alignment

- [ ] **Validate Pricing Schema:**
  - Run the automated pricing configuration validator to ensure the `pricing.yaml` database has correct schema rules and fallback configurations:
    ```bash
    go run cmd/pricing-validator/main.go pricing.yaml
    ```
- [ ] **Update Model Costs:**
  - Cross-reference `pricing.yaml` rates against the latest pricing pages of OpenAI, Anthropic, Gemini, Bedrock, Azure, and Mistral.

---

## 3. SDK Synchronization

- [ ] **Python SDK:**
  - Verify tests in `sdk/python/` pass and the version matches the target release.
- [ ] **TypeScript SDK:**
  - Verify type definitions compile (`npm run build`) in `sdk/ts/` and matches target release.

---

## 4. Release Execution

- [ ] **Tag the Version:**
  - Standard semantic version tags should be pushed:
    ```bash
    git tag -a v1.0.0 -m "Release v1.0.0 GA"
    git push origin v1.0.0
    ```
- [ ] **Verify CI/CD Pipelines:**
  - Ensure the `.github/workflows/release.yml` completes successfully:
    - Binaries compiled for Linux, macOS, and Windows.
    - Software Bill of Materials (SBOM) successfully attached.
    - Checksums calculated and published.
    - Docker images built and signed via Cosign.
    - Homebrew Formulas updated automatically in `loopers-oss/homebrew-tap`.
