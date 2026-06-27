from typing import Optional

try:
    from llama_index.llms.openai import OpenAI
    HAS_LLAMA_INDEX = True
except ImportError:
    HAS_LLAMA_INDEX = False

if HAS_LLAMA_INDEX:
    class LoopersLLM(OpenAI):
        """
        A subclass of LlamaIndex's OpenAI LLM that automatically routes calls through
        the Loopers enforcement engine and parses session governance headers.
        """
        def __init__(
            self,
            loopers_url: str,
            loopers_key: str,
            provider_key: Optional[str] = None,
            session_id: Optional[str] = None,
            session_budget: Optional[float] = None,
            max_steps: Optional[int] = None,
            session_ttl: Optional[int] = None,
            max_tools: Optional[int] = None,
            max_servers: Optional[int] = None,
            **kwargs,
        ):
            headers = {"Authorization": f"Bearer {loopers_key}"}
            if provider_key:
                headers["X-Loopers-Provider-Key"] = provider_key
            if session_id:
                headers["X-Loopers-Session-ID"] = session_id
            if session_budget is not None:
                headers["X-Loopers-Session-Budget"] = str(session_budget)
            if max_steps is not None:
                headers["X-Loopers-Session-Max-Steps"] = str(max_steps)
            if session_ttl is not None:
                headers["X-Loopers-Session-TTL"] = str(session_ttl)
            if max_tools is not None:
                headers["X-Loopers-Session-Max-Tools"] = str(max_tools)
            if max_servers is not None:
                headers["X-Loopers-Session-Max-Servers"] = str(max_servers)

            super().__init__(
                api_base=f"{loopers_url.rstrip('/')}/openai/v1",
                api_key=loopers_key,
                default_headers=headers,
                **kwargs,
            )
else:
    class LoopersLLM:
        def __init__(self, *args, **kwargs):
            raise ImportError("Please install llama-index-llms-openai to use LoopersLLM. Run `pip install loopers-client[llama_index]`")
