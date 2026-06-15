import pytest

def test_langchain_adapter():
    try:
        from loopers_client.integrations.langchain import ChatLoopers, HAS_LANGCHAIN
        if not HAS_LANGCHAIN:
            pytest.skip("langchain-openai not installed")
    except ImportError:
        pytest.skip("langchain module missing")
        
    llm = ChatLoopers(
        loopers_url="http://localhost:8080",
        loopers_key="lp-test-key",
        provider_key="sk-openai-key",
        session_id="test-session",
        session_budget=5.0,
        max_steps=10
    )
    
    assert llm.openai_api_base == "http://localhost:8080/openai/v1"
    assert llm.openai_api_key.get_secret_value() == "lp-test-key"
    assert "X-Loopers-Provider-Key" in llm.default_headers
    assert llm.default_headers["X-Loopers-Provider-Key"] == "sk-openai-key"
    assert llm.default_headers["X-Loopers-Session-ID"] == "test-session"
    assert llm.default_headers["X-Loopers-Session-Budget"] == "5.0"
    assert llm.default_headers["X-Loopers-Session-Max-Steps"] == "10"

def test_llama_index_adapter():
    try:
        from loopers_client.integrations.llama_index import LoopersLLM, HAS_LLAMA_INDEX
        if not HAS_LLAMA_INDEX:
            pytest.skip("llama-index-llms-openai not installed")
    except ImportError:
        pytest.skip("llama_index module missing")
        
    llm = LoopersLLM(
        loopers_url="http://localhost:8080",
        loopers_key="lp-test-key",
        provider_key="sk-openai-key",
        session_id="test-session",
        session_budget=5.0,
        max_steps=10
    )
    
    assert llm.api_base == "http://localhost:8080/openai/v1"
    assert llm.api_key == "lp-test-key"

    # Verify default_headers are set correctly (the real transport-level headers)
    assert "X-Loopers-Provider-Key" in llm.default_headers
    assert llm.default_headers["X-Loopers-Provider-Key"] == "sk-openai-key"
    assert llm.default_headers["X-Loopers-Session-ID"] == "test-session"
    assert llm.default_headers["X-Loopers-Session-Budget"] == "5.0"
    assert llm.default_headers["X-Loopers-Session-Max-Steps"] == "10"
