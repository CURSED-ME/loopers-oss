import contextvars
from typing import Optional, Any

import httpx

# Context variable to hold response headers of the last request safely in multithreaded/async contexts
_last_headers_context = contextvars.ContextVar("last_headers", default={})

def _get_last_headers() -> dict:
    return _last_headers_context.get()

def _set_last_headers(headers: dict):
    _last_headers_context.set(headers)

def _response_hook(response: httpx.Response):
    headers = response.headers
    loopers_headers = {
        "request_cost": headers.get("X-Loopers-Request-Cost"),
        "estimated_cost": headers.get("X-Loopers-Request-Cost-Estimated"),
        "session_spend": headers.get("X-Loopers-Session-Spend"),
        "session_remaining": headers.get("X-Loopers-Session-Remaining"),
        "session_steps": headers.get("X-Loopers-Session-Steps"),
    }
    _set_last_headers(loopers_headers)

async def _async_response_hook(response: httpx.Response):
    _response_hook(response)

def _attach_loopers_attributes(res: Any):
    headers = _get_last_headers()
    if not headers:
        return
    
    # Safely attach Loopers metrics to the returned resource/completion/stream object
    for name, header_key in [
        ("loopers_cost", "request_cost"),
        ("loopers_cost_estimated", "estimated_cost"),
        ("loopers_session_spend", "session_spend"),
        ("loopers_session_remaining", "session_remaining"),
    ]:
        val = headers.get(header_key)
        try:
            setattr(res, name, float(val) if val else None)
        except Exception:
            pass

    steps_val = headers.get("session_steps")
    try:
        setattr(res, "loopers_session_steps", int(steps_val) if steps_val else None)
    except Exception:
        pass


# Try importing openai
try:
    import openai
    HAS_OPENAI = True
except ImportError:
    HAS_OPENAI = False


import time
import threading

class ZSPSyncMixin:
    def _init_zsp(self, loopers_url: str, loopers_key: str):
        self._loopers_url = loopers_url
        self._static_key = loopers_key
        self._ephemeral_key = None
        self._zsp_token = None
        self._token_expires_at = 0
        self._zsp_lock = threading.Lock()
        self._is_cloud = loopers_key.startswith("lc_")

    def _bootstrap_zsp(self):
        with self._zsp_lock:
            if time.time() < self._token_expires_at:
                return

            from cryptography.hazmat.primitives.asymmetric import ec
            import base64
            
            priv = ec.generate_private_key(ec.SECP256R1())
            pub_nums = priv.public_key().public_numbers()
            self._dpop_jwk_public = {
                "kty": "EC", "crv": "P-256",
                "x": base64.urlsafe_b64encode(pub_nums.x.to_bytes(32, "big")).rstrip(b"=").decode(),
                "y": base64.urlsafe_b64encode(pub_nums.y.to_bytes(32, "big")).rstrip(b"=").decode(),
            }

            resp = httpx.post(
                f"{self._loopers_url.rstrip('/')}/v1/auth/token",
                headers={"Authorization": f"Bearer {self._static_key}"},
                json={"dpop_jwk": self._dpop_jwk_public},
                timeout=10,
            )
            resp.raise_for_status()
            data = resp.json()
            self._token_expires_at = time.time() + data["expires_in"] - 300
            self._ephemeral_key = priv
            self._zsp_token = data["access_token"]

    def _inject_dpop_proof(self, request: httpx.Request):
        if not self._is_cloud:
            return

        if time.time() >= self._token_expires_at:
            self._bootstrap_zsp()
            
        request.headers["Authorization"] = f"Bearer {self._zsp_token}"
        
        import jwt as pyjwt
        import uuid
        proof = pyjwt.encode({
            "jti": str(uuid.uuid4()),
            "htm": request.method,
            "htu": f"{request.url.scheme}://{request.url.host}{request.url.path}",
            "iat": int(time.time()),
        }, self._ephemeral_key, algorithm="ES256", headers={"typ": "dpop+jwt", "jwk": self._dpop_jwk_public})
        
        request.headers["DPoP"] = proof

class ZSPAsyncMixin:
    def _init_zsp(self, loopers_url: str, loopers_key: str):
        self._loopers_url = loopers_url
        self._static_key = loopers_key
        self._ephemeral_key = None
        self._zsp_token = None
        self._token_expires_at = 0
        self._zsp_lock = None
        self._is_cloud = loopers_key.startswith("lc_")

    async def _bootstrap_zsp(self):
        import asyncio
        if self._zsp_lock is None:
            self._zsp_lock = asyncio.Lock()
            
        async with self._zsp_lock:
            if time.time() < self._token_expires_at:
                return

            from cryptography.hazmat.primitives.asymmetric import ec
            import base64
            
            priv = ec.generate_private_key(ec.SECP256R1())
            pub_nums = priv.public_key().public_numbers()
            self._dpop_jwk_public = {
                "kty": "EC", "crv": "P-256",
                "x": base64.urlsafe_b64encode(pub_nums.x.to_bytes(32, "big")).rstrip(b"=").decode(),
                "y": base64.urlsafe_b64encode(pub_nums.y.to_bytes(32, "big")).rstrip(b"=").decode(),
            }

            async with httpx.AsyncClient() as client:
                resp = await client.post(
                    f"{self._loopers_url.rstrip('/')}/v1/auth/token",
                    headers={"Authorization": f"Bearer {self._static_key}"},
                    json={"dpop_jwk": self._dpop_jwk_public},
                    timeout=10,
                )
            resp.raise_for_status()
            data = resp.json()
            self._token_expires_at = time.time() + data["expires_in"] - 300
            self._ephemeral_key = priv
            self._zsp_token = data["access_token"]

    async def _inject_dpop_proof(self, request: httpx.Request):
        if not self._is_cloud:
            return

        if time.time() >= self._token_expires_at:
            await self._bootstrap_zsp()
            
        request.headers["Authorization"] = f"Bearer {self._zsp_token}"
        
        import jwt as pyjwt
        import uuid
        proof = pyjwt.encode({
            "jti": str(uuid.uuid4()),
            "htm": request.method,
            "htu": f"{request.url.scheme}://{request.url.host}{request.url.path}",
            "iat": int(time.time()),
        }, self._ephemeral_key, algorithm="ES256", headers={"typ": "dpop+jwt", "jwk": self._dpop_jwk_public})
        
        request.headers["DPoP"] = proof

if HAS_OPENAI:
    class LoopersOpenAI(openai.OpenAI, ZSPSyncMixin):
        """
        A subclass of openai.OpenAI that automatically routes calls through
        the Loopers budget/rate-limit proxy and parses response metrics.
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
            **kwargs
        ):
            # Intercept event hooks to capture Loopers response headers
            self._init_zsp(loopers_url, loopers_key)
            event_hooks = kwargs.pop("event_hooks", {})
            if "request" not in event_hooks:
                event_hooks["request"] = []
            event_hooks["request"].append(self._inject_dpop_proof)
            if "response" not in event_hooks:
                event_hooks["response"] = []
            event_hooks["response"].append(_response_hook)

            if "http_client" not in kwargs:
                kwargs["http_client"] = httpx.Client(event_hooks=event_hooks)

            _provider_path = kwargs.pop("_provider_path", "openai/v1")
            base_url = f"{loopers_url.rstrip('/')}/{_provider_path}"

            default_headers = kwargs.pop("default_headers", {})
            default_headers["Authorization"] = f"Bearer {loopers_key}"
            if provider_key:
                default_headers["X-Loopers-Provider-Key"] = provider_key
            if session_id:
                default_headers["X-Loopers-Session-ID"] = session_id
            if session_budget is not None:
                default_headers["X-Loopers-Session-Budget"] = str(session_budget)
            if max_steps is not None:
                default_headers["X-Loopers-Session-Max-Steps"] = str(max_steps)
            if session_ttl is not None:
                default_headers["X-Loopers-Session-TTL"] = str(session_ttl)
            if max_tools is not None:
                default_headers["X-Loopers-Session-Max-Tools"] = str(max_tools)
            if max_servers is not None:
                default_headers["X-Loopers-Session-Max-Servers"] = str(max_servers)

            super().__init__(
                base_url=base_url,
                api_key=loopers_key,
                default_headers=default_headers,
                **kwargs
            )

        def request(self, *args, **kwargs):
            _set_last_headers({})
            res = super().request(*args, **kwargs)
            _attach_loopers_attributes(res)
            return res

    class LoopersAsyncOpenAI(openai.AsyncOpenAI, ZSPAsyncMixin):
        """
        A subclass of openai.AsyncOpenAI that automatically routes calls through
        the Loopers budget/rate-limit proxy and parses response metrics.
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
            **kwargs
        ):
            self._init_zsp(loopers_url, loopers_key)
            event_hooks = kwargs.pop("event_hooks", {})
            if "request" not in event_hooks:
                event_hooks["request"] = []
            event_hooks["request"].append(self._inject_dpop_proof)
            if "response" not in event_hooks:
                event_hooks["response"] = []
            event_hooks["response"].append(_async_response_hook)

            if "http_client" not in kwargs:
                kwargs["http_client"] = httpx.AsyncClient(event_hooks=event_hooks)

            _provider_path = kwargs.pop("_provider_path", "openai/v1")
            base_url = f"{loopers_url.rstrip('/')}/{_provider_path}"

            default_headers = kwargs.pop("default_headers", {})
            default_headers["Authorization"] = f"Bearer {loopers_key}"
            if provider_key:
                default_headers["X-Loopers-Provider-Key"] = provider_key
            if session_id:
                default_headers["X-Loopers-Session-ID"] = session_id
            if session_budget is not None:
                default_headers["X-Loopers-Session-Budget"] = str(session_budget)
            if max_steps is not None:
                default_headers["X-Loopers-Session-Max-Steps"] = str(max_steps)
            if session_ttl is not None:
                default_headers["X-Loopers-Session-TTL"] = str(session_ttl)
            if max_tools is not None:
                default_headers["X-Loopers-Session-Max-Tools"] = str(max_tools)
            if max_servers is not None:
                default_headers["X-Loopers-Session-Max-Servers"] = str(max_servers)

            super().__init__(
                base_url=base_url,
                api_key=loopers_key,
                default_headers=default_headers,
                **kwargs
            )

        async def request(self, *args, **kwargs):
            _set_last_headers({})
            res = await super().request(*args, **kwargs)
            _attach_loopers_attributes(res)
            return res

    class LoopersGroq(LoopersOpenAI):
        def __init__(self, *args, **kwargs):
            kwargs["_provider_path"] = "groq/v1"
            super().__init__(*args, **kwargs)

    class LoopersAsyncGroq(LoopersAsyncOpenAI):
        def __init__(self, *args, **kwargs):
            kwargs["_provider_path"] = "groq/v1"
            super().__init__(*args, **kwargs)

    class LoopersMistral(LoopersOpenAI):
        def __init__(self, *args, **kwargs):
            kwargs["_provider_path"] = "mistral/v1"
            super().__init__(*args, **kwargs)

    class LoopersAsyncMistral(LoopersAsyncOpenAI):
        def __init__(self, *args, **kwargs):
            kwargs["_provider_path"] = "mistral/v1"
            super().__init__(*args, **kwargs)

    class LoopersDeepSeek(LoopersOpenAI):
        def __init__(self, *args, **kwargs):
            kwargs["_provider_path"] = "deepseek/v1"
            super().__init__(*args, **kwargs)

    class LoopersAsyncDeepSeek(LoopersAsyncOpenAI):
        def __init__(self, *args, **kwargs):
            kwargs["_provider_path"] = "deepseek/v1"
            super().__init__(*args, **kwargs)

    class LoopersTogether(LoopersOpenAI):
        def __init__(self, *args, **kwargs):
            kwargs["_provider_path"] = "together/v1"
            super().__init__(*args, **kwargs)

    class LoopersAsyncTogether(LoopersAsyncOpenAI):
        def __init__(self, *args, **kwargs):
            kwargs["_provider_path"] = "together/v1"
            super().__init__(*args, **kwargs)

else:
    class LoopersOpenAI:
        def __init__(self, *args, **kwargs):
            raise ImportError(
                "The 'openai' package is required to use LoopersOpenAI. "
                "Install it via 'pip install openai'."
            )

    class LoopersAsyncOpenAI:
        def __init__(self, *args, **kwargs):
            raise ImportError(
                "The 'openai' package is required to use LoopersAsyncOpenAI. "
                "Install it via 'pip install openai'."
            )

    class LoopersGroq(LoopersOpenAI):
        pass

    class LoopersAsyncGroq(LoopersAsyncOpenAI):
        pass

    class LoopersMistral(LoopersOpenAI):
        pass

    class LoopersAsyncMistral(LoopersAsyncOpenAI):
        pass

    class LoopersDeepSeek(LoopersOpenAI):
        pass

    class LoopersAsyncDeepSeek(LoopersAsyncOpenAI):
        pass

    class LoopersTogether(LoopersOpenAI):
        pass

    class LoopersAsyncTogether(LoopersAsyncOpenAI):
        pass


# Try importing anthropic
try:
    import anthropic
    HAS_ANTHROPIC = True
except ImportError:
    HAS_ANTHROPIC = False

if HAS_ANTHROPIC:
    class LoopersAnthropic(anthropic.Anthropic):
        """
        A subclass of anthropic.Anthropic that automatically routes calls through
        the Loopers budget/rate-limit proxy and parses response metrics.
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
            **kwargs
        ):
            self._init_zsp(loopers_url, loopers_key)
            event_hooks = kwargs.pop("event_hooks", {})
            if "request" not in event_hooks:
                event_hooks["request"] = []
            event_hooks["request"].append(self._inject_dpop_proof)
            if "response" not in event_hooks:
                event_hooks["response"] = []
            event_hooks["response"].append(_response_hook)

            if "http_client" not in kwargs:
                kwargs["http_client"] = httpx.Client(event_hooks=event_hooks)

            base_url = f"{loopers_url.rstrip('/')}/anthropic"

            default_headers = kwargs.pop("default_headers", {})
            default_headers["Authorization"] = f"Bearer {loopers_key}"
            if provider_key:
                default_headers["X-Loopers-Provider-Key"] = provider_key
            if session_id:
                default_headers["X-Loopers-Session-ID"] = session_id
            if session_budget is not None:
                default_headers["X-Loopers-Session-Budget"] = str(session_budget)
            if max_steps is not None:
                default_headers["X-Loopers-Session-Max-Steps"] = str(max_steps)
            if session_ttl is not None:
                default_headers["X-Loopers-Session-TTL"] = str(session_ttl)
            if max_tools is not None:
                default_headers["X-Loopers-Session-Max-Tools"] = str(max_tools)
            if max_servers is not None:
                default_headers["X-Loopers-Session-Max-Servers"] = str(max_servers)

            super().__init__(
                base_url=base_url,
                auth_token=loopers_key,
                default_headers=default_headers,
                **kwargs
            )

        def request(self, *args, **kwargs):
            _set_last_headers({})
            res = super().request(*args, **kwargs)
            _attach_loopers_attributes(res)
            return res

    class LoopersAsyncAnthropic(anthropic.AsyncAnthropic):
        """
        A subclass of anthropic.AsyncAnthropic that automatically routes calls through
        the Loopers budget/rate-limit proxy and parses response metrics.
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
            **kwargs
        ):
            self._init_zsp(loopers_url, loopers_key)
            event_hooks = kwargs.pop("event_hooks", {})
            if "request" not in event_hooks:
                event_hooks["request"] = []
            event_hooks["request"].append(self._inject_dpop_proof)
            if "response" not in event_hooks:
                event_hooks["response"] = []
            event_hooks["response"].append(_async_response_hook)

            if "http_client" not in kwargs:
                kwargs["http_client"] = httpx.AsyncClient(event_hooks=event_hooks)

            base_url = f"{loopers_url.rstrip('/')}/anthropic"

            default_headers = kwargs.pop("default_headers", {})
            default_headers["Authorization"] = f"Bearer {loopers_key}"
            if provider_key:
                default_headers["X-Loopers-Provider-Key"] = provider_key
            if session_id:
                default_headers["X-Loopers-Session-ID"] = session_id
            if session_budget is not None:
                default_headers["X-Loopers-Session-Budget"] = str(session_budget)
            if max_steps is not None:
                default_headers["X-Loopers-Session-Max-Steps"] = str(max_steps)
            if session_ttl is not None:
                default_headers["X-Loopers-Session-TTL"] = str(session_ttl)
            if max_tools is not None:
                default_headers["X-Loopers-Session-Max-Tools"] = str(max_tools)
            if max_servers is not None:
                default_headers["X-Loopers-Session-Max-Servers"] = str(max_servers)

            super().__init__(
                base_url=base_url,
                auth_token=loopers_key,
                default_headers=default_headers,
                **kwargs
            )

        async def request(self, *args, **kwargs):
            _set_last_headers({})
            res = await super().request(*args, **kwargs)
            _attach_loopers_attributes(res)
            return res
else:
    class LoopersAnthropic:
        def __init__(self, *args, **kwargs):
            raise ImportError(
                "The 'anthropic' package is required to use LoopersAnthropic. "
                "Install it via 'pip install anthropic'."
            )

    class LoopersAsyncAnthropic:
        def __init__(self, *args, **kwargs):
            raise ImportError(
                "The 'anthropic' package is required to use LoopersAsyncAnthropic. "
                "Install it via 'pip install anthropic'."
            )
