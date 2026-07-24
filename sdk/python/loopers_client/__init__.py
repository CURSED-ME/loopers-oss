from .client import (
    LoopersOpenAI,
    LoopersAsyncOpenAI,
    LoopersAnthropic,
    LoopersAsyncAnthropic,
    LoopersGroq,
    LoopersAsyncGroq,
    LoopersMistral,
    LoopersAsyncMistral,
    LoopersDeepSeek,
    LoopersAsyncDeepSeek,
    LoopersTogether,
    LoopersAsyncTogether,
)
from .policy_error import (
    LoopersPolicyDenied,
    parse_policy_denial,
    format_as_tool_output,
)

__all__ = [
    "LoopersOpenAI",
    "LoopersAsyncOpenAI",
    "LoopersAnthropic",
    "LoopersAsyncAnthropic",
    "LoopersGroq",
    "LoopersAsyncGroq",
    "LoopersMistral",
    "LoopersAsyncMistral",
    "LoopersDeepSeek",
    "LoopersAsyncDeepSeek",
    "LoopersTogether",
    "LoopersAsyncTogether",
    # Policy denial handling — agent self-correction
    "LoopersPolicyDenied",
    "parse_policy_denial",
    "format_as_tool_output",
]

try:
    from .adapters.crewai import get_loopers_crewai_llm
    __all__.append("get_loopers_crewai_llm")
except ImportError:
    pass

try:
    from .adapters.autogen import get_loopers_autogen_config
    __all__.append("get_loopers_autogen_config")
except ImportError:
    pass
