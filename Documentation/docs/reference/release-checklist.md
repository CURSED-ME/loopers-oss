---
id: release-checklist
title: Release Checklist
sidebar_label: Release Checklist
description: Pre release validation checklist for Loopers maintainers.
---

# Release Checklist

This list is for Loopers maintainers when preparing a new release.

## Pre Release

### Code Quality
- [ ] All integration tests pass successfully on the main branch
- [ ] go test command runs and passes locally without errors
- [ ] golangci lint checks pass with no warnings
- [ ] go mod tidy has been run to clean up unused dependencies
- [ ] All temporary development comments are resolved

### Correctness Checks
- [ ] Test budget limits under a heavy load of requests
- [ ] Confirm there is zero budget leakage or overspending
- [ ] Test fail closed behavior by stopping the Redis server
- [ ] Test mid stream cutoffs for all streaming AI services
- [ ] Test agent loop detection with repeating prompts

### Provider Checks
- [ ] Test OpenAI calls (normal and streaming)
- [ ] Test Anthropic calls (normal and streaming)
- [ ] Test Google Gemini calls (normal and streaming)
- [ ] Test AWS Bedrock calls
- [ ] Test Azure OpenAI calls
- [ ] Test Mistral AI calls
- [ ] Test Groq calls
- [ ] Test Cohere calls
- [ ] Test DeepSeek calls
- [ ] Test Together calls

### Documentation
- [ ] Update changelog file
- [ ] Update version info in readme file
- [ ] Write documentation for new features in the docs folder
- [ ] List any breaking changes clearly

## Release Steps

### 1. Tag the release version

Run the tag command to set the version:

```bash
git tag -a v1.x.x -m "Release v1.x.x"
git push origin v1.x.x
```

### 2. Check build artifacts

The build script will start automatically. Make sure:
- [ ] The release is created on GitHub
- [ ] The computer binaries are available
- [ ] The Docker image is uploaded to ghcr.io

### 3. Update Helm charts

```bash
cd helm
helm package .
helm repo index . --url https://charts.tryloopers.com
```

### 4. Post Release Tasks

- [ ] Publish the release notes on GitHub
- [ ] Announce the new release to the community
- [ ] Update main branch to the next development version
