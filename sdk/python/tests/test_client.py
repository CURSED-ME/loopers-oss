import pytest
import respx
import httpx
from loopers_client import LoopersOpenAI, LoopersAnthropic

@respx.mock
def test_openai_headers_and_url():
    # Arrange
    respx.post("http://localhost:8080/openai/v1/chat/completions").mock(
        return_value=httpx.Response(200, json={"id": "chatcmpl-123", "choices": [], "object": "chat.completion"})
    )
    
    client = LoopersOpenAI(
        loopers_url="http://localhost:8080",
        loopers_key="lp-123",
        provider_key="sk-openai",
        session_id="sess-1",
        session_budget=5.0,
        max_steps=10
    )
    
    # Act
    client.chat.completions.create(
        model="gpt-4",
        messages=[{"role": "user", "content": "hi"}]
    )
    
    # Assert
    assert len(respx.calls) == 1
    request = respx.calls[0].request
    assert str(request.url) == "http://localhost:8080/openai/v1/chat/completions"
    assert request.headers.get("Authorization") == "Bearer lp-123"
    assert request.headers.get("X-Loopers-Provider-Key") == "sk-openai"
    assert request.headers.get("X-Loopers-Session-ID") == "sess-1"
    assert request.headers.get("X-Loopers-Session-Budget") == "5.0"
    assert request.headers.get("X-Loopers-Session-Max-Steps") == "10"

@respx.mock
def test_openai_metrics_parsing():
    # Arrange
    respx.post("http://localhost:8080/openai/v1/chat/completions").mock(
        return_value=httpx.Response(
            200, 
            json={"id": "chatcmpl-123", "choices": [], "object": "chat.completion"},
            headers={
                "X-Loopers-Request-Cost": "0.01",
                "X-Loopers-Session-Spend": "0.05",
            }
        )
    )
    
    client = LoopersOpenAI(
        loopers_url="http://localhost:8080",
        loopers_key="lp-123"
    )
    
    # Act
    response = client.chat.completions.create(
        model="gpt-4",
        messages=[{"role": "user", "content": "hi"}]
    )
    
    # Assert
    assert hasattr(response, "loopers_cost")
    assert response.loopers_cost == 0.01
    assert hasattr(response, "loopers_session_spend")
    assert response.loopers_session_spend == 0.05

@respx.mock
def test_anthropic_headers_and_url():
    # Arrange
    respx.post("http://localhost:8080/anthropic/v1/messages").mock(
        return_value=httpx.Response(200, json={"id": "msg_123", "type": "message", "role": "assistant", "model": "claude-3-opus", "content": []})
    )
    
    client = LoopersAnthropic(
        loopers_url="http://localhost:8080",
        loopers_key="lp-123",
        provider_key="sk-ant",
    )
    
    # Act
    client.messages.create(
        model="claude-3-opus-20240229",
        messages=[{"role": "user", "content": "hi"}],
        max_tokens=10
    )
    
    # Assert
    assert len(respx.calls) == 1
    request = respx.calls[0].request
    assert str(request.url) == "http://localhost:8080/anthropic/v1/messages"
    assert request.headers.get("Authorization") == "Bearer lp-123"
    assert request.headers.get("X-Loopers-Provider-Key") == "sk-ant"

@respx.mock
def test_groq_headers_and_url():
    # Arrange
    respx.post("http://localhost:8080/groq/v1/chat/completions").mock(
        return_value=httpx.Response(200, json={"id": "chatcmpl-123", "choices": [], "object": "chat.completion"})
    )
    
    from loopers_client import LoopersGroq
    client = LoopersGroq(
        loopers_url="http://localhost:8080",
        loopers_key="lp-123",
        provider_key="gsk_123",
    )
    
    # Act
    client.chat.completions.create(
        model="llama3-8b",
        messages=[{"role": "user", "content": "hi"}]
    )
    
    # Assert
    assert len(respx.calls) == 1
    request = respx.calls[0].request
    assert str(request.url) == "http://localhost:8080/groq/v1/chat/completions"
    assert request.headers.get("Authorization") == "Bearer lp-123"
    assert request.headers.get("X-Loopers-Provider-Key") == "gsk_123"
