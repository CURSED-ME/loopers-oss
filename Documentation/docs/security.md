---
id: security
title: Security Policy
sidebar_label: Security
description: Loopers security model, guarantees, and vulnerability reporting process.
---

# Security Policy

## Security Model

Loopers is designed with security as our top priority.

### Zero Storage of Keys

Your real AI provider API keys (like OpenAI or Anthropic keys) are handled very safely:
* They are sent with your request in a header named X-Loopers-Provider-Key.
* They are kept only in the temporary memory of the computer while sending the request.
* They are forwarded to the AI provider and then immediately erased from memory.
* They are never written to disk or saved in any database.
* They are never written to log files.

This means that if someone manages to hack into your Loopers server or database, they will not find any of your real AI keys because those keys are never saved anywhere.

### Fail Closed Guarantee

If the Redis database or the Loopers program stops working:
* All new requests are blocked immediately with a service unavailable error.
* No request can go through without being checked.
* Loopers will start working normally again as soon as the database is back up.

This is a fail closed system. It acts like a locked gate that shuts automatically to protect your budget if something goes wrong.

### Loopers Key Security

The keys you create for Loopers (starting with lp) are also secure:
* They are generated using random codes.
* Only a secure hash of the key is saved in Redis. The actual key is shown to you once when you create it and is never saved anywhere.
* You can deactivate any key instantly using the command line.

## Supported Versions

Currently supported versions of Loopers:

| Version | Status |
|---|---|
| 1.x.x (latest) | Supported |
| 0.x.x | Not Supported |

## Reporting a Security Issue

:::caution Do not open a public GitHub issue for security issues
If you find a security problem, please do not post it publicly. That could put other users at risk before a fix is ready.
:::

**You can report security issues by:**

1. Using private vulnerability reporting on the GitHub repository page (this is the best way).
2. Sending an email to security@tryloopers.com.

### What to include in your report

* The version of Loopers you are using
* A description of the security issue
* Steps to reproduce the issue
* What kind of impact the issue has

### Response Time

We aim to acknowledge your report within 48 hours and complete a full assessment within 5 business days.

## Security Best Practices

### Running in Production

* Run Loopers with a dedicated Redis database that is not shared with other programs.
* Protect your Redis connection with a password.
* Place Loopers behind an internal load balancer so it is not directly accessible from the public internet.
* Rotate your Loopers keys regularly.

### Key Rotation

To rotate a key, follow these steps:

1. Create a new key:
   ```bash
   loopers keys create --name mykey-v2 --provider openai
   ```
2. Update your program to use the new key.
3. Deactivate the old key:
   ```bash
   loopers keys revoke OLD_KEY_HASH
   ```
