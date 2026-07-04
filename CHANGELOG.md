# Changelog

## [1.4.0](https://github.com/CURSED-ME/loopers-oss/compare/v1.3.0...v1.4.0) (2026-07-04)


### Features

* enhance loop detection resilience and synchronize documentation ([15fe1fe](https://github.com/CURSED-ME/loopers-oss/commit/15fe1feb227c244adcb034704582607ee171f8b5))

## [1.3.0](https://github.com/CURSED-ME/loopers-oss/compare/v1.2.0...v1.3.0) (2026-07-03)


### Features

* Implement local policy engine, agent identity, and framework adapters ([639bae1](https://github.com/CURSED-ME/loopers-oss/commit/639bae1ae42abea3c63e7858e0dc21d7cc318af5))
* **sdk:** support Phase 1 session headers ([ec81008](https://github.com/CURSED-ME/loopers-oss/commit/ec81008939fb5c3d9e78fe210d2e6c766bfc99b6))


### Bug Fixes

* go mod tidy for golang 1.26.4 pipeline ([2fa97e0](https://github.com/CURSED-ME/loopers-oss/commit/2fa97e0545eab6c1b658fbfebd4e263e606d89fc))
* gofmt and go mod tidy for pipeline ([024b3bb](https://github.com/CURSED-ME/loopers-oss/commit/024b3bbfb30be0ef0862c671f2e406987e5eea87))
* gofmt server.go ([34de90a](https://github.com/CURSED-ME/loopers-oss/commit/34de90af3cfaba980a8f406a8b245fc0c26f3992))
* **oss:** harden enforcement engine (phase 1 audit remediation) ([104a1ba](https://github.com/CURSED-ME/loopers-oss/commit/104a1ba9f6f717850e071bb36ee6830f3d199e67))
* resolve data race in Alerter between Close() and channel sends ([32462ec](https://github.com/CURSED-ME/loopers-oss/commit/32462ec9d467b578b4cd30e185d1d5fa3bf2bf98))
* **sdk:** add Phase 1 headers to all adapters, bump to v1.2.0, expand test coverage ([9a40389](https://github.com/CURSED-ME/loopers-oss/commit/9a40389146df62674d53705f500851601181d1cb))

## [1.2.0](https://github.com/CURSED-ME/loopers-oss/compare/v1.1.0...v1.2.0) (2026-07-03)


### Features

* Implement local policy engine, agent identity, and framework adapters ([639bae1](https://github.com/CURSED-ME/loopers-oss/commit/639bae1ae42abea3c63e7858e0dc21d7cc318af5))
* MCP Domination Phase 1 implementation ([74467a4](https://github.com/CURSED-ME/loopers-oss/commit/74467a481e26ed47a1f570c86cce068ab2782e59))
* **sdk:** support Phase 1 session headers ([ec81008](https://github.com/CURSED-ME/loopers-oss/commit/ec81008939fb5c3d9e78fe210d2e6c766bfc99b6))


### Bug Fixes

* go mod tidy for golang 1.26.4 pipeline ([2fa97e0](https://github.com/CURSED-ME/loopers-oss/commit/2fa97e0545eab6c1b658fbfebd4e263e606d89fc))
* gofmt and go mod tidy for pipeline ([024b3bb](https://github.com/CURSED-ME/loopers-oss/commit/024b3bbfb30be0ef0862c671f2e406987e5eea87))
* gofmt server.go ([34de90a](https://github.com/CURSED-ME/loopers-oss/commit/34de90af3cfaba980a8f406a8b245fc0c26f3992))
* **oss:** harden enforcement engine (phase 1 audit remediation) ([104a1ba](https://github.com/CURSED-ME/loopers-oss/commit/104a1ba9f6f717850e071bb36ee6830f3d199e67))
* resolve data race in Alerter between Close() and channel sends ([32462ec](https://github.com/CURSED-ME/loopers-oss/commit/32462ec9d467b578b4cd30e185d1d5fa3bf2bf98))
* **sdk:** add Phase 1 headers to all adapters, bump to v1.2.0, expand test coverage ([9a40389](https://github.com/CURSED-ME/loopers-oss/commit/9a40389146df62674d53705f500851601181d1cb))
* update cosign args to use --bundle ([a15a022](https://github.com/CURSED-ME/loopers-oss/commit/a15a022889a35499d53a3293aa8c88943830194b))

## [1.1.0](https://github.com/CURSED-ME/loopers-oss/compare/v1.0.0...v1.1.0) (2026-06-28)


### Features

* Deterministic loop detection engine ([f5680fd](https://github.com/CURSED-ME/loopers-oss/commit/f5680fdc399ceb51acadc302913ea8f7725b3b5e))
* Enterprise architecture hardening and stability updates ([9d55de1](https://github.com/CURSED-ME/loopers-oss/commit/9d55de15a51e4f6a5e67860beeaccb0acb8612d1))
* Generic providers, dynamic pricing, and Python SDK integrations ([9717c43](https://github.com/CURSED-ME/loopers-oss/commit/9717c43405d8b1cc05981695c1bc14d95e0910be))
* MCP Domination Phase 1 implementation ([74467a4](https://github.com/CURSED-ME/loopers-oss/commit/74467a481e26ed47a1f570c86cce068ab2782e59))
* **sdk:** support Phase 1 session headers ([ec81008](https://github.com/CURSED-ME/loopers-oss/commit/ec81008939fb5c3d9e78fe210d2e6c766bfc99b6))
* **security:** Phase 0 Hardening - OWASP v2 Security Events and OpenTelemetry Tracing ([2b3cf8a](https://github.com/CURSED-ME/loopers-oss/commit/2b3cf8aef402b2629b36beacc593059ec4b300fe))
* Structured Security Event Emission with OWASP Top 10 2025 integration ([3f74756](https://github.com/CURSED-ME/loopers-oss/commit/3f74756087d9b9249c1de3546420875f1f09b30f))


### Bug Fixes

* ensure alerter is initialized even without webhook URL ([7a42408](https://github.com/CURSED-ME/loopers-oss/commit/7a42408555fdc56d4c323c99283b22bee16ba752))
* go mod tidy for golang 1.26.4 pipeline ([2fa97e0](https://github.com/CURSED-ME/loopers-oss/commit/2fa97e0545eab6c1b658fbfebd4e263e606d89fc))
* gofmt and go mod tidy for pipeline ([024b3bb](https://github.com/CURSED-ME/loopers-oss/commit/024b3bbfb30be0ef0862c671f2e406987e5eea87))
* **oss:** harden enforcement engine (phase 1 audit remediation) ([104a1ba](https://github.com/CURSED-ME/loopers-oss/commit/104a1ba9f6f717850e071bb36ee6830f3d199e67))
* resolve CI formatting and demo E2E tests ([92623b5](https://github.com/CURSED-ME/loopers-oss/commit/92623b5c0e2005290ee1c1b8ba28d970336fa87e))
* **sdk:** add Phase 1 headers to all adapters, bump to v1.2.0, expand test coverage ([9a40389](https://github.com/CURSED-ME/loopers-oss/commit/9a40389146df62674d53705f500851601181d1cb))
* update cosign args to use --bundle ([a15a022](https://github.com/CURSED-ME/loopers-oss/commit/a15a022889a35499d53a3293aa8c88943830194b))
