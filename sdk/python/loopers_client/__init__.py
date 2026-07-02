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
