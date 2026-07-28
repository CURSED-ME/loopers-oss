# Changelog

## [2.0.4](https://github.com/CURSED-ME/loopers-oss/compare/sdk-python-v2.0.3...sdk-python-v2.0.4) (2026-07-28)


### Bug Fixes

* **sdk:** add missing mixin inheritance and export for client adapters ([be1c101](https://github.com/CURSED-ME/loopers-oss/commit/be1c1016011a732029a36cc124ab65b264050423))
* **sdk:** add missing mixin inheritance and export for client adapters ([17afa19](https://github.com/CURSED-ME/loopers-oss/commit/17afa196f7f59e1d9dbd74bc6693ed88f21eb66d))

## [2.0.3](https://github.com/CURSED-ME/loopers-oss/compare/sdk-python-v2.0.2...sdk-python-v2.0.3) (2026-07-28)


### Dependencies

* upgrade grpc to v1.82.1 and update client SDKs ([ec3d855](https://github.com/CURSED-ME/loopers-oss/commit/ec3d8554ef32768f90745adc6b985a8657fe861c))

## [2.0.2](https://github.com/CURSED-ME/loopers-oss/compare/sdk-python-v2.0.1...sdk-python-v2.0.2) (2026-07-24)


### Bug Fixes

* **python-sdk:** lightweight dev dependencies in pyproject.toml to fi… ([b675079](https://github.com/CURSED-ME/loopers-oss/commit/b675079b1d34867b7ee4244653edfd7dd7cdf12c))
* **python-sdk:** lightweight dev dependencies in pyproject.toml to fix PyPI CI build timeout ([26dcbfb](https://github.com/CURSED-ME/loopers-oss/commit/26dcbfb870483705677afad7f20845f42771e715))

## [2.0.1](https://github.com/CURSED-ME/loopers-oss/compare/sdk-python-v2.0.0...sdk-python-v2.0.1) (2026-07-24)


### Bug Fixes

* **python-sdk:** fix fallback classes, header metric attributes, and … ([dc4bf61](https://github.com/CURSED-ME/loopers-oss/commit/dc4bf6194b675a0cdbbb340f274ab7b95140ad0b))
* **python-sdk:** fix fallback classes, header metric attributes, and lazy imports for PyPI CI build ([b77741e](https://github.com/CURSED-ME/loopers-oss/commit/b77741ea0a57668814628e0ada7bd634b8de2a8e))

## [2.0.0](https://github.com/CURSED-ME/loopers-oss/compare/sdk-python-v1.4.2...sdk-python-v2.0.0) (2026-07-24)


### ⚠ BREAKING CHANGES

* **policy:** none — all changes are purely additive.

### Features

* **policy:** stateful taint tracking + agent-friendly error formats ([4d4a00b](https://github.com/CURSED-ME/loopers-oss/commit/4d4a00b5bd11b6af8b7121ed82a3e04cc9ba78ce))
* **policy:** stateful taint tracking + agent-friendly error formats ([61f07e8](https://github.com/CURSED-ME/loopers-oss/commit/61f07e85d43fc4b3b75ba856be8c984104786897))

## [1.4.2](https://github.com/CURSED-ME/loopers-oss/compare/sdk-python-v1.4.1...sdk-python-v1.4.2) (2026-07-08)


### Bug Fixes

* explicit tomli requirement to fix python sdk ci build ([c173d2f](https://github.com/CURSED-ME/loopers-oss/commit/c173d2fa452e4b02394f8663982fa670ec852c83))
* explicit tomli requirement to fix python sdk ci build ([a764be4](https://github.com/CURSED-ME/loopers-oss/commit/a764be42f84efc164ab99f37d0145f5aeb043c14))

## [1.4.1](https://github.com/CURSED-ME/loopers-oss/compare/sdk-python-v1.4.0...sdk-python-v1.4.1) (2026-07-07)


### Bug Fixes

* Replace test shell script with Makefile for Scorecard ([fb471ce](https://github.com/CURSED-ME/loopers-oss/commit/fb471ceabf6a9d6e1d6e2e6736777d8a02070a5b))
* Resolve GitHub Scorecard Pinned-Dependencies vulnerabilities ([0648364](https://github.com/CURSED-ME/loopers-oss/commit/064836476ebb6e268a2d0f52b901a557f780172d))

## [1.4.0](https://github.com/CURSED-ME/loopers-oss/compare/sdk-python-v1.3.0...sdk-python-v1.4.0) (2026-07-03)


### Features

* Implement local policy engine, agent identity, and framework adapters ([639bae1](https://github.com/CURSED-ME/loopers-oss/commit/639bae1ae42abea3c63e7858e0dc21d7cc318af5))

## [1.3.0](https://github.com/CURSED-ME/loopers-oss/compare/sdk-python-v1.2.0...sdk-python-v1.3.0) (2026-06-28)


### Features

* add 14 providers, bump SDKs to 1.0.0, prepare for launch ([abd1d6d](https://github.com/CURSED-ME/loopers-oss/commit/abd1d6d06623bd4ba475247cc8f4d83d744e67f4))
* Generic providers, dynamic pricing, and Python SDK integrations ([9717c43](https://github.com/CURSED-ME/loopers-oss/commit/9717c43405d8b1cc05981695c1bc14d95e0910be))
* **sdk:** support Phase 1 session headers ([ec81008](https://github.com/CURSED-ME/loopers-oss/commit/ec81008939fb5c3d9e78fe210d2e6c766bfc99b6))


### Bug Fixes

* bump sdk versions and disable homebrew publishing for CI ([9e8ceae](https://github.com/CURSED-ME/loopers-oss/commit/9e8ceaef264fcde3dad46af132481f9db91cebb0))
* **ci:** disable homebrew tap until repo is created, bump versions to 0.4.5 ([8523ff4](https://github.com/CURSED-ME/loopers-oss/commit/8523ff498a82e8310a8cdc6f856e39b399ef7d5a))
* **ci:** fix goreleaser sbom config and bump versions to 0.4.3 ([56563ec](https://github.com/CURSED-ME/loopers-oss/commit/56563ece8d04927227308905b7105e396326880f))
* **ci:** fix invalid action commit hashes for goreleaser, bump version to 0.4.2 ([d6a40ef](https://github.com/CURSED-ME/loopers-oss/commit/d6a40efa1f51529046d3513368b9bea6e7d4ae54))
* **ci:** point syft to latest v0 to support --enrich flag and bump to v0.4.7 ([29a7f7f](https://github.com/CURSED-ME/loopers-oss/commit/29a7f7f7c2ae1ba293004db61a9743a2218b55d5))
* **ci:** use latest syft for goreleaser sboms, bump versions to 0.4.4 ([af0790a](https://github.com/CURSED-ME/loopers-oss/commit/af0790a6245cd9e9bf309b5747dfb89e6fe8c3f3))
* **sdk:** add Phase 1 headers to all adapters, bump to v1.2.0, expand test coverage ([9a40389](https://github.com/CURSED-ME/loopers-oss/commit/9a40389146df62674d53705f500851601181d1cb))
* **sdk:** fix anthropic sdk version in TS package, bump versions to 0.4.1 ([df374e6](https://github.com/CURSED-ME/loopers-oss/commit/df374e6d4e95aa09d77f24f94dd10dd41444c3b2))
* **security:** address OpenSSF scorecard issues and bump to v0.4.6 ([4d31477](https://github.com/CURSED-ME/loopers-oss/commit/4d3147704db87b8c11e988ddd17b06f18a658c9d))
