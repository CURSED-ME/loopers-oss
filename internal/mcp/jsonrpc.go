package mcp

import (
	"encoding/json"
	"fmt"
)

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

// ParseJSONRPC parses the raw body into a JSONRPCRequest and validates it.
func ParseJSONRPC(body []byte) (*JSONRPCRequest, error) {
	var req JSONRPCRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	if req.JSONRPC != "2.0" {
		return nil, fmt.Errorf("invalid jsonrpc version, expected '2.0'")
	}

	if req.Method == "" {
		return nil, fmt.Errorf("method cannot be empty")
	}
	if len(req.Method) > 256 {
		return nil, fmt.Errorf("method name too long (max 256 characters)")
	}

	if len(req.ID) > 0 && string(req.ID) != "null" {
		// Must be string or number
		if req.ID[0] != '"' && (req.ID[0] < '0' || req.ID[0] > '9') && req.ID[0] != '-' {
			return nil, fmt.Errorf("invalid id type, must be string, number, or null")
		}
	}

	if len(req.Params) > 0 && string(req.Params) != "null" {
		if req.Params[0] != '{' && req.Params[0] != '[' {
			return nil, fmt.Errorf("params must be an object or array")
		}
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
