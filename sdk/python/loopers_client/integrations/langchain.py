from typing import Optional

try:
    from langchain_openai import ChatOpenAI
    HAS_LANGCHAIN = True
except ImportError:
    HAS_LANGCHAIN = False

if HAS_LANGCHAIN:
    class ChatLoopers(ChatOpenAI):
        """
        A subclass of ChatOpenAI that automatically routes calls through
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
                base_url=f"{loopers_url.rstrip('/')}/openai/v1",
                api_key=loopers_key,
                default_headers=headers,
                **kwargs,
            )
else:
    class ChatLoopers:
        def __init__(self, *args, **kwargs):
            raise ImportError("Please install langchain-openai to use ChatLoopers. Run `pip install loopers-client[langchain]`")
