package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scaler-tech/toad/internal/state"
)

// TestContextPropagation verifies that context values set by HTTP middleware
// are visible inside MCP tool handlers via the Go SDK's Streamable HTTP transport.
func TestContextPropagation(t *testing.T) {
	// Create an MCP server with a tool that reads from context.
	srv := gomcp.NewServer(&gomcp.Implementation{Name: "test", Version: "0.1"}, nil)

	type pingArgs struct {
		Msg string `json:"msg"`
	}

	var gotToken *state.MCPToken

	gomcp.AddTool(srv, &gomcp.Tool{
		Name:        "ping",
		Description: "test tool",
	}, func(ctx context.Context, req *gomcp.CallToolRequest, args pingArgs) (*gomcp.CallToolResult, any, error) {
		gotToken = tokenFromContext(ctx)
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: "pong"}},
		}, nil, nil
	})

	// Wrap the MCP handler with our auth middleware.
	db := &mockDB{tokens: map[string]*state.MCPToken{
		"toad_test123": {
			Token:       "toad_test123",
			SlackUserID: "UTEST",
			SlackUser:   "testuser",
			Role:        "dev",
			CreatedAt:   time.Now(),
		},
	}}

	mcpHandler := gomcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *gomcp.Server { return srv },
		nil,
	)
	handler := authMiddleware(db, mcpHandler)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Step 1: Initialize the MCP session.
	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0.1"}}}`
	req, _ := http.NewRequest("POST", ts.URL, strings.NewReader(initBody))
	req.Header.Set("Authorization", "Bearer toad_test123")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("initialize request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("initialize: got status %d, body: %s", resp.StatusCode, body)
	}

	// Extract session ID from response header.
	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		t.Fatal("no Mcp-Session-Id header in initialize response")
	}

	// Step 2: Send initialized notification.
	notifBody := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	req, _ = http.NewRequest("POST", ts.URL, strings.NewReader(notifBody))
	req.Header.Set("Authorization", "Bearer toad_test123")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Session-Id", sessionID)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("initialized notification failed: %v", err)
	}
	resp.Body.Close()

	// Step 3: Call the tool.
	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping","arguments":{"msg":"hello"}}}`
	req, _ = http.NewRequest("POST", ts.URL, strings.NewReader(callBody))
	req.Header.Set("Authorization", "Bearer toad_test123")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Session-Id", sessionID)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("tool call failed: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("tool call: got status %d, body: %s", resp.StatusCode, body)
	}

	// Parse the response — could be SSE or JSON.
	responseStr := string(body)
	if strings.Contains(responseStr, "event:") {
		// SSE format — extract the data line.
		for _, line := range strings.Split(responseStr, "\n") {
			if strings.HasPrefix(line, "data: ") {
				responseStr = strings.TrimPrefix(line, "data: ")
				break
			}
		}
	}

	var result struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(responseStr), &result); err != nil {
		t.Fatalf("failed to parse tool response: %v (body: %s)", err, body)
	}

	// The critical assertion: did the tool handler see the token from context?
	if gotToken == nil {
		t.Fatal("tokenFromContext returned nil inside tool handler — context values NOT propagated")
	}
	if gotToken.SlackUserID != "UTEST" {
		t.Errorf("got SlackUserID %q, want UTEST", gotToken.SlackUserID)
	}
	if gotToken.Role != "dev" {
		t.Errorf("got Role %q, want dev", gotToken.Role)
	}
}
