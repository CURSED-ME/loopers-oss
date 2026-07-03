---
id: framework-adapters
title: Framework Adapters (CrewAI & AutoGen)
sidebar_label: Framework Adapters
description: Native Python SDK adapters for CrewAI and Microsoft AutoGen to route agent traffic through Loopers.
---

# Framework Adapters

Loopers provides native Python adapters for popular agentic frameworks to seamlessly route their LLM and tool calls through the Loopers proxy. This enables budget tracking, circuit breaking, and policy enforcement for frameworks that do not natively support proxy configurations.

---

## CrewAI

CrewAI agents are powered by LangChain under the hood. The Loopers CrewAI adapter returns a `ChatOpenAI` instance pre-configured to communicate with the Loopers proxy and inject the correct headers.

### Installation

```bash
pip install "loopers-client[crewai]"
```

### Usage

```python
from crewai import Agent, Task, Crew
from loopers_client.adapters.crewai import get_loopers_crewai_llm

# Initialize the Loopers-configured LLM
loopers_llm = get_loopers_crewai_llm(
    loopers_key="<YOUR_LOOPERS_KEY>",
    proxy_url="http://localhost:8080/v1",
    model="gpt-4o",
    session_id="research_task_001"  # Optional: For session tracking
)

# Assign to your CrewAI agent
researcher = Agent(
    role="Senior Researcher",
    goal="Discover the latest AI trends",
    backstory="You are an expert AI researcher...",
    llm=loopers_llm,
    verbose=True
)

# Proceed with CrewAI as normal
task = Task(
    description="Research the latest AI trends",
    agent=researcher,
    expected_output="A summary of AI trends"
)

crew = Crew(agents=[researcher], tasks=[task])
result = crew.kickoff()
```

All LLM calls made by the CrewAI agent will now be automatically routed through Loopers with full budget enforcement, loop detection, and policy evaluation.

---

## AutoGen

Microsoft AutoGen requires an `llm_config` dictionary for its agents. The Loopers adapter generates this configuration payload pointing to your proxy.

### Installation

```bash
pip install "loopers-client[autogen]"
```

### Usage

```python
from autogen import AssistantAgent, UserProxyAgent
from loopers_client.adapters.autogen import get_loopers_autogen_config

# Generate the Loopers configuration
llm_config = get_loopers_autogen_config(
    loopers_key="<YOUR_LOOPERS_KEY>",
    proxy_url="http://localhost:8080/v1",
    model="gpt-4o",
    session_id="coding_task_001"  # Optional: For session tracking
)

# Assign to your AutoGen assistant
assistant = AssistantAgent(
    name="assistant",
    llm_config=llm_config
)

user_proxy = UserProxyAgent(
    name="user_proxy",
    code_execution_config={"work_dir": "coding"},
    human_input_mode="NEVER"
)

# Proceed with AutoGen as normal
user_proxy.initiate_chat(assistant, message="Write a Python script to sort a list")
```

---

## How It Works

Both adapters work by:

1. **Routing**: Pointing the LLM client's `base_url` to your Loopers proxy (`http://localhost:8080/v1`) instead of directly to OpenAI.
2. **Authentication**: Using your Loopers proxy key (`lp-xxx`) as the API key, which Loopers validates and maps to budget limits.
3. **Session Tracking**: Optionally injecting the `X-Loopers-Session-ID` header for session-scoped budget enforcement and loop detection.

This means all the Loopers features — budget enforcement, mid-stream cutoffs, circuit breaking, policy evaluation, and security events — are automatically applied to every call your agent makes, with zero additional code.

---

## Supported Frameworks

| Framework | Adapter | Status |
|---|---|---|
| **LangChain** | `ChatLoopers` (built-in) | Stable |
| **LlamaIndex** | `LoopersLLM` (built-in) | Stable |
| **CrewAI** | `get_loopers_crewai_llm` | Stable |
| **AutoGen** | `get_loopers_autogen_config` | Stable |

For LangChain and LlamaIndex adapters, see the [Python SDK documentation](/docs/sdks/python).
