# Graph Report - loopers-oss  (2026-05-24)

## Corpus Check
- 47 files · ~33,604 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 283 nodes · 351 edges · 20 communities detected
- Extraction: 79% EXTRACTED · 21% INFERRED · 0% AMBIGUOUS · INFERRED: 74 edges (avg confidence: 0.8)
- Token cost: 0 input · 0 output

## Community Hubs (Navigation)
- [[_COMMUNITY_Community 0|Community 0]]
- [[_COMMUNITY_Community 1|Community 1]]
- [[_COMMUNITY_Community 2|Community 2]]
- [[_COMMUNITY_Community 3|Community 3]]
- [[_COMMUNITY_Community 4|Community 4]]
- [[_COMMUNITY_Community 5|Community 5]]
- [[_COMMUNITY_Community 6|Community 6]]
- [[_COMMUNITY_Community 7|Community 7]]
- [[_COMMUNITY_Community 8|Community 8]]
- [[_COMMUNITY_Community 9|Community 9]]
- [[_COMMUNITY_Community 10|Community 10]]
- [[_COMMUNITY_Community 11|Community 11]]
- [[_COMMUNITY_Community 12|Community 12]]
- [[_COMMUNITY_Community 13|Community 13]]
- [[_COMMUNITY_Community 14|Community 14]]
- [[_COMMUNITY_Community 15|Community 15]]
- [[_COMMUNITY_Community 16|Community 16]]
- [[_COMMUNITY_Community 17|Community 17]]
- [[_COMMUNITY_Community 18|Community 18]]
- [[_COMMUNITY_Community 20|Community 20]]

## God Nodes (most connected - your core abstractions)
1. `NewServer()` - 15 edges
2. `Client` - 10 edges
3. `AnthropicProvider` - 10 edges
4. `AzureProvider` - 10 edges
5. `BedrockProvider` - 10 edges
6. `GeminiProvider` - 10 edges
7. `MistralProvider` - 10 edges
8. `OpenAIProvider` - 10 edges
9. `NewClient()` - 8 edges
10. `NewBedrockProvider()` - 8 edges

## Surprising Connections (you probably didn't know these)
- `getRedisClient()` --calls--> `NewClient()`  [INFERRED]
  cmd\loopers\commands.go → internal\budget\redis.go
- `TestBudgetRaceCondition()` --calls--> `NewClient()`  [INFERRED]
  internal\budget\engine_test.go → internal\budget\redis.go
- `TestBudgetWindowExpansion()` --calls--> `NewClient()`  [INFERRED]
  internal\budget\engine_test.go → internal\budget\redis.go
- `TestSessionEnforcement()` --calls--> `NewClient()`  [INFERRED]
  internal\budget\engine_test.go → internal\budget\redis.go
- `TestHealthEndpoint()` --calls--> `NewClient()`  [INFERRED]
  internal\server\server_test.go → internal\budget\redis.go

## Communities

### Community 0 - "Community 0"
Cohesion: 0.1
Nodes (10): BudgetExceededError, Client, getWindowConfigs(), TestBudgetRaceCondition(), TestBudgetWindowExpansion(), TestSessionEnforcement(), NewClient(), WindowInfo (+2 more)

### Community 1 - "Community 1"
Cohesion: 0.11
Nodes (14): NewAlerter(), TestAlerterDelivery(), TestAlerterThresholdCheck(), GenerateRawKey(), HashKey(), LoadStore(), Registry, NewRegistry() (+6 more)

### Community 2 - "Community 2"
Cohesion: 0.15
Nodes (13): _async_response_hook(), _attach_loopers_attributes(), _get_last_headers(), LoopersAnthropic, LoopersAsyncAnthropic, LoopersAsyncOpenAI, LoopersOpenAI, A subclass of openai.AsyncOpenAI that automatically routes calls through (+5 more)

### Community 3 - "Community 3"
Cohesion: 0.11
Nodes (13): BudgetExceededDetails, BudgetExceededResponse, ErrorPayload, NewBudgetExceededResponse(), NewSessionBudgetExceededResponse(), NewSessionStepsExceededResponse(), SessionBudgetExceededResponse, SessionStepsDetails (+5 more)

### Community 4 - "Community 4"
Cohesion: 0.13
Nodes (8): BedrockProvider, NewBedrockProvider(), TestBedrockCountInputTokensFallback(), TestBedrockFormatBudgetExceededFrame(), TestBedrockParseNonStreamResponse(), TestBedrockParseRequest(), TestBedrockRewritePath(), TestBedrockStreamChunkParsing()

### Community 5 - "Community 5"
Cohesion: 0.2
Nodes (10): NewAnthropicProvider(), NewOpenAIProvider(), mockReadCloser, SSEStreamReader, NewSSEStreamReader(), readEventStreamFrame(), TestAnthropicEventParsing(), TestDONEHandling() (+2 more)

### Community 6 - "Community 6"
Cohesion: 0.16
Nodes (6): Alerter, AlertEvent, AlertingConfig, BudgetExceededAlert, ThresholdAlert, ThresholdConfig

### Community 7 - "Community 7"
Cohesion: 0.15
Nodes (4): AnthropicProvider, formatAnthropicBudgetExceededSSE(), parseAnthropicFrame(), parseAnthropicStreamChunk()

### Community 8 - "Community 8"
Cohesion: 0.14
Nodes (3): AzureProvider, NewAzureProvider(), TestAzureProvider()

### Community 9 - "Community 9"
Cohesion: 0.15
Nodes (3): GeminiProvider, formatGeminiBudgetExceededSSE(), parseGeminiStreamChunk()

### Community 10 - "Community 10"
Cohesion: 0.14
Nodes (3): MistralProvider, NewMistralProvider(), TestMistralProvider()

### Community 11 - "Community 11"
Cohesion: 0.15
Nodes (3): OpenAIProvider, formatOpenAIBudgetExceededSSE(), parseOpenAIStreamChunk()

### Community 12 - "Community 12"
Cohesion: 0.2
Nodes (8): BodyBuffer(), KeyExtractor(), MaxBytesReader(), Recovery(), RequestID(), RequestLogger(), Server, serverContextKey

### Community 13 - "Community 13"
Cohesion: 0.2
Nodes (4): InitLogger(), NewRedactWriter(), RedactWriter, initConfig()

### Community 14 - "Community 14"
Cohesion: 0.22
Nodes (8): AnthropicCountResponse, AnthropicMessageDelta, AnthropicMessageStart, AnthropicUsage, ChatCompletionRequest, Message, OpenAIStreamChunk, OpenAIUsage

### Community 15 - "Community 15"
Cohesion: 0.43
Nodes (7): NewGeminiProvider(), TestGeminiCountInputTokensFallback(), TestGeminiInjectAuth(), TestGeminiParseNonStreamResponse(), TestGeminiParseRequest(), TestGeminiParseStreamChunk(), TestGeminiRewritePath()

### Community 16 - "Community 16"
Cohesion: 0.33
Nodes (4): Config, ModelPrice, ProviderConfig, Store

### Community 17 - "Community 17"
Cohesion: 0.47
Nodes (3): createLoopersFetch(), LoopersAnthropic, LoopersOpenAI

### Community 18 - "Community 18"
Cohesion: 0.4
Nodes (3): ModelPrice, PricingConfig, ProviderConfig

### Community 20 - "Community 20"
Cohesion: 1.0
Nodes (1): Provider

## Knowledge Gaps
- **33 isolated node(s):** `ModelPrice`, `ProviderConfig`, `PricingConfig`, `ThresholdConfig`, `AlertingConfig` (+28 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **Thin community `Community 20`** (2 nodes): `provider.go`, `Provider`
  Too small to be a meaningful cluster - may be noise or needs more connections extracted.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `NewServer()` connect `Community 1` to `Community 4`, `Community 5`, `Community 8`, `Community 10`, `Community 12`, `Community 15`?**
  _High betweenness centrality (0.472) - this node is a cross-community bridge._
- **Why does `NewClient()` connect `Community 0` to `Community 1`?**
  _High betweenness centrality (0.122) - this node is a cross-community bridge._
- **Why does `NewOpenAIProvider()` connect `Community 5` to `Community 8`, `Community 1`, `Community 10`, `Community 11`?**
  _High betweenness centrality (0.109) - this node is a cross-community bridge._
- **Are the 13 inferred relationships involving `NewServer()` (e.g. with `TestAlerterDelivery()` and `TestAlerterThresholdCheck()`) actually correct?**
  _`NewServer()` has 13 INFERRED edges - model-reasoned connections that need verification._
- **What connects `ModelPrice`, `ProviderConfig`, `PricingConfig` to the rest of the system?**
  _33 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Community 0` be split into smaller, more focused modules?**
  _Cohesion score 0.1 - nodes in this community are weakly interconnected._
- **Should `Community 1` be split into smaller, more focused modules?**
  _Cohesion score 0.11 - nodes in this community are weakly interconnected._