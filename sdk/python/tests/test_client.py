import pytest
import respx
import httpx
from loopers_client import LoopersOpenAI, LoopersAnthropic


@respx.mock
def test_openai_headers_and_url():
    """Verify LoopersOpenAI routes to the correct URL and sends all governance headers."""
    respx.post("http://localhost:8080/openai/v1/chat/completions").mock(
        return_value=httpx.Response(200, json={"id": "chatcmpl-123", "choices": [], "object": "chat.completion"})
    )

    client = LoopersOpenAI(
        loopers_url="http://localhost:8080",
        loopers_key="lp-123",
        provider_key="sk-openai",
        session_id="sess-1",
        session_budget=5.0,
        max_steps=10,
        session_ttl=3600,
        max_tools=5,
        max_servers=2,
    )

    client.chat.completions.create(
        model="gpt-4",
        messages=[{"role": "user", "content": "hi"}]
    )

    assert len(respx.calls) == 1
    request = respx.calls[0].request
    assert str(request.url) == "http://localhost:8080/openai/v1/chat/completions"
    assert request.headers.get("Authorization") == "Bearer lp-123"
    assert request.headers.get("X-Loopers-Provider-Key") == "sk-openai"
    assert request.headers.get("X-Loopers-Session-ID") == "sess-1"
    assert request.headers.get("X-Loopers-Session-Budget") == "5.0"
    assert request.headers.get("X-Loopers-Session-Max-Steps") == "10"
    assert request.headers.get("X-Loopers-Session-TTL") == "3600"
    assert request.headers.get("X-Loopers-Session-Max-Tools") == "5"
    assert request.headers.get("X-Loopers-Session-Max-Servers") == "2"


@respx.mock
def test_openai_optional_headers_omitted_when_not_set():
    """Verify optional headers are NOT sent when arguments are not provided."""
    respx.post("http://localhost:8080/openai/v1/chat/completions").mock(
        return_value=httpx.Response(200, json={"id": "chatcmpl-123", "choices": [], "object": "chat.completion"})
    )

    client = LoopersOpenAI(
        loopers_url="http://localhost:8080",
        loopers_key="lp-123",
    )

    client.chat.completions.create(
        model="gpt-4",
        messages=[{"role": "user", "content": "hi"}]
    )

    request = respx.calls[0].request
    assert request.headers.get("X-Loopers-Session-TTL") is None
    assert request.headers.get("X-Loopers-Session-Max-Tools") is None
    assert request.headers.get("X-Loopers-Session-Max-Servers") is None
    assert request.headers.get("X-Loopers-Session-Budget") is None
    assert request.headers.get("X-Loopers-Session-Max-Steps") is None


@respx.mock
def test_openai_metrics_parsing():
    """Verify Loopers response metrics are attached to the returned object."""
    respx.post("http://localhost:8080/openai/v1/chat/completions").mock(
        return_value=httpx.Response(
            200,
            json={"id": "chatcmpl-123", "choices": [], "object": "chat.completion"},
            headers={
                "X-Loopers-Request-Cost": "0.01",
                "X-Loopers-Session-Spend": "0.05",
                "X-Loopers-Session-Steps": "3",
                "X-Loopers-Session-Remaining": "4.95",
            }
        )
    )

    client = LoopersOpenAI(
        loopers_url="http://localhost:8080",
        loopers_key="lp-123"
    )

    response = client.chat.completions.create(
        model="gpt-4",
        messages=[{"role": "user", "content": "hi"}]
    )

    assert hasattr(response, "loopers_cost")
    assert response.loopers_cost == 0.01
    assert hasattr(response, "loopers_session_spend")
    assert response.loopers_session_spend == 0.05
    assert hasattr(response, "loopers_session_steps")
    assert response.loopers_session_steps == 3
    assert hasattr(response, "loopers_session_remaining")
    assert response.loopers_session_remaining == 4.95


@respx.mock
def test_anthropic_headers_and_url():
    """Verify LoopersAnthropic routes to the correct URL and sends all governance headers."""
    respx.post("http://localhost:8080/anthropic/v1/messages").mock(
        return_value=httpx.Response(200, json={"id": "msg_123", "type": "message", "role": "assistant", "model": "claude-3-opus", "content": []})
    )

    client = LoopersAnthropic(
        loopers_url="http://localhost:8080",
        loopers_key="lp-123",
        provider_key="sk-ant",
        session_id="sess-ant-1",
        session_budget=10.0,
        max_steps=5,
        session_ttl=1800,
        max_tools=3,
        max_servers=1,
    )

    client.messages.create(
        model="claude-3-opus-20240229",
        messages=[{"role": "user", "content": "hi"}],
        max_tokens=10
    )

    assert len(respx.calls) == 1
    request = respx.calls[0].request
    assert str(request.url) == "http://localhost:8080/anthropic/v1/messages"
    assert request.headers.get("Authorization") == "Bearer lp-123"
    assert request.headers.get("X-Loopers-Provider-Key") == "sk-ant"
    assert request.headers.get("X-Loopers-Session-ID") == "sess-ant-1"
    assert request.headers.get("X-Loopers-Session-Budget") == "10.0"
    assert request.headers.get("X-Loopers-Session-Max-Steps") == "5"
    assert request.headers.get("X-Loopers-Session-TTL") == "1800"
    assert request.headers.get("X-Loopers-Session-Max-Tools") == "3"
    assert request.headers.get("X-Loopers-Session-Max-Servers") == "1"


@respx.mock
def test_groq_headers_and_url():
    """Verify LoopersGroq routes to the groq/v1 path."""
    respx.post("http://localhost:8080/groq/v1/chat/completions").mock(
        return_value=httpx.Response(200, json={"id": "chatcmpl-123", "choices": [], "object": "chat.completion"})
    )

    from loopers_client import LoopersGroq
    client = LoopersGroq(
        loopers_url="http://localhost:8080",
        loopers_key="lp-123",
        provider_key="gsk_123",
        session_ttl=600,
        max_tools=10,
        max_servers=3,
    )

    client.chat.completions.create(
        model="llama3-8b",
        messages=[{"role": "user", "content": "hi"}]
    )

    assert len(respx.calls) == 1
    request = respx.calls[0].request
    assert str(request.url) == "http://localhost:8080/groq/v1/chat/completions"
    assert request.headers.get("Authorization") == "Bearer lp-123"
    assert request.headers.get("X-Loopers-Provider-Key") == "gsk_123"
    assert request.headers.get("X-Loopers-Session-TTL") == "600"
    assert request.headers.get("X-Loopers-Session-Max-Tools") == "10"
    assert request.headers.get("X-Loopers-Session-Max-Servers") == "3"
