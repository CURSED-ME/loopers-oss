import pytest
try:
    import langchain_openai
    HAS_CREWAI_DEPS = True
except ImportError:
    HAS_CREWAI_DEPS = False

from loopers_client.adapters.autogen import get_loopers_autogen_config
from loopers_client.policy_error import (
    LoopersPolicyDenied,
    parse_policy_denial,
    format_as_tool_output,
)


@pytest.mark.skipif(not HAS_CREWAI_DEPS, reason="langchain_openai not installed")
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


# ---- Policy Error Tests ----

class TestLoopersPolicyDenied:
    def test_str_with_tool_name(self):
        denial = LoopersPolicyDenied(tool_name="outbound_http", reason="secret accessed")
        assert "outbound_http" in str(denial)
        assert "secret accessed" in str(denial)

    def test_str_without_tool_name(self):
        denial = LoopersPolicyDenied(tool_name="", reason="environment restriction")
        assert "blocked by policy" in str(denial)
        assert "environment restriction" in str(denial)

    def test_fields_preserved(self):
        denial = LoopersPolicyDenied(
            tool_name="spawn_sub_agent",
            reason="dev env restriction",
            session_id="sess-123",
            mcp_server="main-server",
        )
        assert denial.tool_name == "spawn_sub_agent"
        assert denial.reason == "dev env restriction"
        assert denial.session_id == "sess-123"
        assert denial.mcp_server == "main-server"

    def test_is_exception(self):
        denial = LoopersPolicyDenied(tool_name="foo", reason="bar")
        assert isinstance(denial, Exception)
        with pytest.raises(LoopersPolicyDenied):
            raise denial


class TestFormatAsToolOutput:
    def test_with_tool_name(self):
        denial = LoopersPolicyDenied(tool_name="outbound_http", reason="secret accessed")
        result = format_as_tool_output(denial)
        assert result == "Error: tool [outbound_http] blocked. Reason: secret accessed"

    def test_without_tool_name(self):
        denial = LoopersPolicyDenied(tool_name="", reason="dev env restriction")
        result = format_as_tool_output(denial)
        assert result == "Error: request blocked by policy. Reason: dev env restriction"


class TestParsePolicyDenial:
    def test_returns_none_for_non_dict(self):
        assert parse_policy_denial(None) is None
        assert parse_policy_denial("string") is None
        assert parse_policy_denial(42) is None
        assert parse_policy_denial([]) is None

    def test_returns_none_for_unrelated_dict(self):
        assert parse_policy_denial({"message": "something else"}) is None
        assert parse_policy_denial({"error": {"type": "rate_limit"}}) is None

    def test_parses_http_403_format(self):
        """HTTP 403 structured error from LLM proxy (router.go)"""
        response_data = {
            "error": {
                "message": "Tool call [outbound_http] was denied by policy. Reason: secret accessed",
                "type": "policy_denied",
                "code": "policy_denied",
                "details": {
                    "tool_name": "outbound_http",
                    "mcp_server": "main-server",
                    "rule": "outbound HTTP blocked after secret access",
                },
                "support": "...",
            }
        }
        denial = parse_policy_denial(response_data, session_id="sess-abc")
        assert denial is not None
        assert denial.tool_name == "outbound_http"
        assert denial.reason == "outbound HTTP blocked after secret access"
        assert denial.session_id == "sess-abc"
        assert denial.mcp_server == "main-server"
        assert denial.raw_response is response_data

    def test_parses_mcp_jsonrpc_format(self):
        """JSON-RPC 2.0 error from MCP handler (mcp/handler.go) — HTTP 200 body"""
        response_data = {
            "jsonrpc": "2.0",
            "id": 1,
            "error": {
                "code": -32001,
                "message": "Error: tool [outbound_http] blocked. Reason: secret accessed",
                "data": {
                    "tool_name": "outbound_http",
                    "rule": "outbound HTTP blocked after secret access",
                },
            }
        }
        denial = parse_policy_denial(response_data, session_id="sess-xyz")
        assert denial is not None
        assert denial.tool_name == "outbound_http"
        assert denial.reason == "outbound HTTP blocked after secret access"
        assert denial.session_id == "sess-xyz"

    def test_parses_mcp_jsonrpc_extracts_tool_from_message(self):
        """Falls back to parsing tool name from message when data.tool_name is missing."""
        response_data = {
            "jsonrpc": "2.0",
            "id": 2,
            "error": {
                "code": -32001,
                "message": "Error: tool [spawn_sub_agent] blocked. Reason: dev restriction",
                "data": {},
            }
        }
        denial = parse_policy_denial(response_data)
        assert denial is not None
        assert denial.tool_name == "spawn_sub_agent"

    def test_override_tool_name(self):
        """Explicit tool_name parameter takes precedence over payload."""
        response_data = {
            "error": {
                "type": "policy_denied",
                "code": "policy_denied",
                "message": "Denied",
                "details": {"tool_name": "from_payload"},
            }
        }
        denial = parse_policy_denial(response_data, tool_name="from_param")
        assert denial is not None
        assert denial.tool_name == "from_param"

    def test_format_as_tool_output_integration(self):
        """End-to-end: parse a denial and format it for LLM context injection."""
        response_data = {
            "error": {
                "type": "policy_denied",
                "code": "policy_denied",
                "message": "Tool call [outbound_http] was denied",
                "details": {
                    "tool_name": "outbound_http",
                    "rule": "outbound HTTP blocked after secret access",
                },
            }
        }
        denial = parse_policy_denial(response_data)
        output = format_as_tool_output(denial)
        assert output == "Error: tool [outbound_http] blocked. Reason: outbound HTTP blocked after secret access"
