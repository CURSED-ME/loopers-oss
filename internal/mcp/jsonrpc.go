package mcp

import "encoding/json"

// JSONRPCRequest represents a standard JSON-RPC 2.0 request.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// ToolCallParams represents the parameters for a "tools/call" MCP method.
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ParseJSONRPC parses the raw body into a JSONRPCRequest.
func ParseJSONRPC(body []byte) (*JSONRPCRequest, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

// ParseToolCallParams parses the params of a tools/call request.
func ParseToolCallParams(params json.RawMessage) (*ToolCallParams, error) {
	var toolCall ToolCallParams
	if err := json.Unmarshal(params, &toolCall); err != nil {
		return nil, err
	}
	return &toolCall, nil
}
