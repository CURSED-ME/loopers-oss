import pytest
from loopers_client.adapters.crewai import get_loopers_crewai_llm
from loopers_client.adapters.autogen import get_loopers_autogen_config

def test_crewai_adapter():
    llm = get_loopers_crewai_llm(
        loopers_key="test_key",
        proxy_url="http://test:8080/v1",
        model="gpt-3.5-turbo",
        session_id="session_123"
    )
    
    # Under the hood it returns a ChatOpenAI
    assert llm.model_name == "gpt-3.5-turbo"
    assert llm.openai_api_base == "http://test:8080/v1"
    assert llm.openai_api_key.get_secret_value() == "test_key"
    assert "X-Loopers-Session-ID" in llm.default_headers
    assert llm.default_headers["X-Loopers-Session-ID"] == "session_123"

def test_autogen_adapter():
    config = get_loopers_autogen_config(
        loopers_key="test_key",
        proxy_url="http://test:8080/v1",
        model="gpt-4o",
        session_id="session_123"
    )
    
    assert "config_list" in config
    assert len(config["config_list"]) == 1
    
    entry = config["config_list"][0]
    assert entry["model"] == "gpt-4o"
    assert entry["api_key"] == "test_key"
    assert entry["base_url"] == "http://test:8080/v1"
    assert "default_headers" in entry
    assert entry["default_headers"]["X-Loopers-Session-ID"] == "session_123"
