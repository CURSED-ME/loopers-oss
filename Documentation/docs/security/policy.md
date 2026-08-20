---
id: policy
title: Security Policy & Key Trust Model
sidebar_label: Vulnerability Policy
description: Loopers zero-storage credential model, key rotation, and vulnerability reporting process.
---

# Security Policy

## Pass-Through Key Trust Model

Loopers is architected with a **zero-storage, zero-persistence** security model to avoid the risk of stored credential leaks:

* **In-Flight Keys Only:** Provider API keys (`X-Loopers-Provider-Key`) are transmitted in HTTP request headers. Loopers extracts them into temporary memory, injects them into the upstream request payload, and redacts them immediately. The keys are never written to disk or stored in Redis.
* **Network Isolation:** Because API keys are transmitted on every request, **you must run Loopers within the same secure VPC/Network as your client application**, or secure the proxy endpoints exclusively using **HTTPS (TLS)**.
* **Log Redaction:** Loopers automatically scans and redacts all standard provider API key formats (`sk-`, `sk-ant-`, `AIza`) from its logs using regex hooks. Always verify your log pipeline to ensure it does not bypass the default output interceptors.

---

## Supported Versions

Currently supported versions of Loopers:

| Version | Status |
|---|---|
| 1.x.x (latest) | Supported |
| 0.x.x | Not Supported |

---

## Reporting a Security Issue

:::caution Do not open a public GitHub issue for security issues
If you find a security problem, please do not post it publicly. That could put other users at risk before a fix is ready.
:::

**You can report security issues by:**
1. Using private vulnerability reporting on the GitHub repository page (preferred).
2. Sending an email to `varadkhroiya17@gmail.com`.

### What to include in your report
* The version of Loopers you are using
* A description of the security issue
* Steps to reproduce the issue
* What kind of impact the issue has

### Response Time
* **Initial Acknowledgement:** Within 24 hours.
* **Assessment & Fix Plan:** Within 7 business days.

---

## Security Best Practices

### Running in Production
* Run Loopers with a dedicated Redis database that is not shared with other programs.
* Protect your Redis connection with TLS and authentication password.
* Place Loopers behind an internal load balancer / reverse proxy with TLS termination.
* Rotate your Loopers keys regularly.

### Key Rotation
To rotate an agent key:
1. Create a new key:
   ```bash
   loopers keys create --name mykey-v2 --provider openai
   ```
2. Update your agent or service environment with the new key.
3. Revoke the old key:
   ```bash
   loopers keys revoke <OLD_KEY_HASH>
   ```
