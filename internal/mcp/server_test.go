package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scaler-tech/toad/internal/state"
)

type mockDB struct {
	tokens map[string]*state.MCPToken
}

func (m *mockDB) ValidateMCPToken(token string) (*state.MCPToken, error) {
	return m.tokens[token], nil
}

func TestAuthMiddleware(t *testing.T) {
	db := &mockDB{tokens: map[string]*state.MCPToken{
		"toad_valid": {
			Token:       "toad_valid",
			SlackUserID: "U123",
			SlackUser:   "alice",
			Role:        "dev",
			CreatedAt:   time.Now(),
		},
	}}

	handler := authMiddleware(db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := tokenFromContext(r.Context())
		if tok == nil {
			t.Fatal("expected token in context")
		}
		if tok.SlackUserID != "U123" {
			t.Errorf("got user %q, want U123", tok.SlackUserID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name string
		auth string
		want int
	}{
		{"valid token", "Bearer toad_valid", http.StatusOK},
		{"missing header", "", http.StatusUnauthorized},
		{"invalid token", "Bearer toad_bad", http.StatusUnauthorized},
		{"wrong scheme", "Basic toad_valid", http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/mcp", nil)
			if tt.auth != "" {
				req.Header.Set("Authorization", tt.auth)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tt.want {
				t.Errorf("got %d, want %d", w.Code, tt.want)
			}
		})
	}
}
