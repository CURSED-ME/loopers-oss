# Framework Adapters

Loopers provides native Python adapters for popular Agentic Frameworks to seamlessly route their LLM and Tool calls through the Loopers proxy. This enables budget tracking, circuit breaking, and policy enforcement for frameworks that do not natively support proxy configurations.

## CrewAI

CrewAI agents are powered by LangChain under the hood. Our CrewAI adapter returns a `ChatOpenAI` instance pre-configured to communicate with the Loopers proxy and inject the correct headers.

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
    session_id="research_task_001" # Optional: For session tracking
)

# Assign to your CrewAI agent
researcher = Agent(
    role="Senior Researcher",
    goal="Discover the latest AI trends",
    backstory="You are an expert AI researcher...",
    llm=loopers_llm,
    verbose=True
)

# ... Proceed with CrewAI as normal ...
```

## AutoGen

AutoGen requires an `llm_config` dictionary for its agents. Our adapter generates this configuration payload pointing to your proxy.

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
    session_id="coding_task_001" # Optional: For session tracking
)

# Assign to your AutoGen assistant
assistant = AssistantAgent(
    name="assistant",
    llm_config=llm_config
)

# ... Proceed with AutoGen as normal ...
```
