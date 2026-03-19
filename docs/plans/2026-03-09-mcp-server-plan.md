> **Status: COMPLETED** — This feature has been implemented and is running in production.

# Toad MCP Server Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an MCP server to the toad daemon so users can ask codebase questions (ribbit) and devs can query logs via Claude Desktop/Code.

**Architecture:** Streamable HTTP MCP server embedded in the toad daemon, using the official Go SDK (`github.com/modelcontextprotocol/go-sdk`). Auth via bearer tokens generated through Slack. Two tools: `ask` (all users) and `logs` (devs only). Session context for multi-turn conversations.

**Tech Stack:** `github.com/modelcontextprotocol/go-sdk/mcp` for MCP protocol, existing `internal/ribbit` and `internal/triage` engines, SQLite for token storage.

**Design doc:** `docs/plans/2026-03-09-mcp-server-design.md`

---

### Task 1: Add MCP config struct

**Files:**
- Modify: `internal/config/config.go`

**Step 1: Add MCPConfig struct and wire into Config**

Add to `internal/config/config.go` after the `LogConfig` struct (~line 125):

```go
type MCPConfig struct {
	Enabled bool     `yaml:"enabled"`
	Port    int      `yaml:"port"`
	Devs    []string `yaml:"devs"` // Slack user IDs with dev role
}
```

Add to the `Config` struct (~line 13):

```go
MCP MCPConfig `yaml:"mcp"`
```

Add defaults in the `defaults()` function:

```go
MCP: MCPConfig{
	Enabled: false,
	Port:    8099,
},
```

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build

**Step 3: Commit**

`/release` — "Add MCPConfig to config"

---

### Task 2: Add mcp_tokens table to state DB

**Files:**
- Modify: `internal/state/db.go`
- Test: `internal/state/db_test.go` (if exists, else `internal/state/state_test.go`)

**Step 1: Write test for token CRUD**

Add a test file `internal/state/mcp_tokens_test.go`:

```go
package state

import (
	"testing"
	"time"
)

func TestMCPTokenCRUD(t *testing.T) {
	db, err := openTestDB()
	if err != nil {
		t.Fatal(err)
	}

	// Save a token
	tok := &MCPToken{
		Token:       "toad_abc123",
		SlackUserID: "U12345",
		SlackUser:   "alice",
		Role:        "dev",
		CreatedAt:   time.Now(),
	}
	if err := db.SaveMCPToken(tok); err != nil {
		t.Fatal(err)
	}

	// Validate token
	got, err := db.ValidateMCPToken("toad_abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got.SlackUserID != "U12345" {
		t.Errorf("got user %q, want U12345", got.SlackUserID)
	}
	if got.Role != "dev" {
		t.Errorf("got role %q, want dev", got.Role)
	}

	// Invalid token returns nil
	got, err = db.ValidateMCPToken("toad_invalid")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("expected nil for invalid token")
	}

	// Revoke token
	if err := db.RevokeMCPToken("U12345"); err != nil {
		t.Fatal(err)
	}
	got, err = db.ValidateMCPToken("toad_abc123")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Error("expected nil after revoke")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/state/ -run TestMCPTokenCRUD -v`
Expected: FAIL — MCPToken type and methods don't exist

**Step 3: Add MCPToken struct and DB schema**

Add to `internal/state/db.go` — the `MCPToken` struct near other structs:

```go
type MCPToken struct {
	Token       string
	SlackUserID string
	SlackUser   string
	Role        string // "dev" or "user"
	CreatedAt   time.Time
	LastUsedAt  time.Time
}
```

Add to `migrate()` function after the last `CREATE TABLE IF NOT EXISTS`:

```go
_, err = db.Exec(`CREATE TABLE IF NOT EXISTS mcp_tokens (
	token TEXT PRIMARY KEY,
	slack_user_id TEXT NOT NULL,
	slack_user TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'user',
	created_at DATETIME NOT NULL,
	last_used_at DATETIME
)`)
if err != nil {
	return fmt.Errorf("creating mcp_tokens table: %w", err)
}
```

**Step 4: Implement SaveMCPToken, ValidateMCPToken, RevokeMCPToken**

Add to `internal/state/db.go`:

```go
func (d *DB) SaveMCPToken(tok *MCPToken) error {
	_, err := d.db.Exec(
		`INSERT OR REPLACE INTO mcp_tokens (token, slack_user_id, slack_user, role, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		tok.Token, tok.SlackUserID, tok.SlackUser, tok.Role, tok.CreatedAt,
	)
	return err
}

func (d *DB) ValidateMCPToken(token string) (*MCPToken, error) {
	row := d.db.QueryRow(
		`SELECT token, slack_user_id, slack_user, role, created_at, last_used_at
		 FROM mcp_tokens WHERE token = ?`, token,
	)
	var tok MCPToken
	var lastUsed sql.NullTime
	err := row.Scan(&tok.Token, &tok.SlackUserID, &tok.SlackUser, &tok.Role, &tok.CreatedAt, &lastUsed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lastUsed.Valid {
		tok.LastUsedAt = lastUsed.Time
	}
	// Update last_used_at
	_, _ = d.db.Exec(`UPDATE mcp_tokens SET last_used_at = ? WHERE token = ?`, time.Now(), token)
	return &tok, nil
}

func (d *DB) RevokeMCPToken(slackUserID string) error {
	_, err := d.db.Exec(`DELETE FROM mcp_tokens WHERE slack_user_id = ?`, slackUserID)
	return err
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/state/ -run TestMCPTokenCRUD -v`
Expected: PASS

**Step 6: Commit**

`/release` — "Add mcp_tokens table with CRUD operations"

---

### Task 3: Add Go MCP SDK dependency

**Files:**
- Modify: `go.mod`, `go.sum`

**Step 1: Add the dependency**

Run: `go get github.com/modelcontextprotocol/go-sdk@latest`

**Step 2: Verify**

Run: `go build ./...`
Expected: Clean build

**Step 3: Commit**

`/release` — "Add official MCP Go SDK dependency"

---

### Task 4: MCP server skeleton with auth middleware

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/server_test.go`

**Step 1: Write test for auth middleware**

Create `internal/mcp/server_test.go`:

```go
package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scaler-tech/toad/internal/state"
)

// mockDB implements the token validation interface for tests
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
		w.WriteHeader(http.StatusOK)
	}))

	// Valid token
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer toad_valid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}

	// Missing token
	req = httptest.NewRequest("POST", "/mcp", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}

	// Invalid token
	req = httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer toad_invalid")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestAuthMiddleware -v`
Expected: FAIL — package doesn't exist

**Step 3: Implement server.go**

Create `internal/mcp/server.go`:

```go
package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scaler-tech/toad/internal/config"
	"github.com/scaler-tech/toad/internal/state"
)

type tokenKey struct{}

// TokenValidator abstracts token validation for testability.
type TokenValidator interface {
	ValidateMCPToken(token string) (*state.MCPToken, error)
}

// Server wraps the MCP server and its dependencies.
type Server struct {
	mcpServer *gomcp.Server
	cfg       config.MCPConfig
	db        TokenValidator
	httpSrv   *http.Server
}

// New creates a new MCP server. Tools must be registered before calling Start.
func New(cfg config.MCPConfig, db TokenValidator) *Server {
	mcpSrv := gomcp.NewServer(&gomcp.Implementation{
		Name:    "toad",
		Version: "1.0.0",
	}, nil)

	return &Server{
		mcpServer: mcpSrv,
		cfg:       cfg,
		db:        db,
	}
}

// Start begins listening on the configured port. Blocks until ctx is cancelled.
func (s *Server) Start(ctx context.Context) error {
	handler := gomcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *gomcp.Server {
			return s.mcpServer
		},
		nil,
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", authMiddleware(s.db, handler))

	s.httpSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.Port),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		s.httpSrv.Close()
	}()

	slog.Info("MCP server listening", "port", s.cfg.Port)
	err := s.httpSrv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

// MCPServer returns the underlying MCP server for tool registration.
func (s *Server) MCPServer() *gomcp.Server {
	return s.mcpServer
}

func authMiddleware(db TokenValidator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")

		tok, err := db.ValidateMCPToken(token)
		if err != nil {
			slog.Error("token validation error", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if tok == nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), tokenKey{}, tok)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tokenFromContext(ctx context.Context) *state.MCPToken {
	tok, _ := ctx.Value(tokenKey{}).(*state.MCPToken)
	return tok
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/ -run TestAuthMiddleware -v`
Expected: PASS

**Step 5: Commit**

`/release` — "Add MCP server skeleton with auth middleware"

---

### Task 5: Implement `logs` tool

**Files:**
- Create: `internal/mcp/tools.go`
- Create: `internal/mcp/tools_test.go`

**Step 1: Write test for logs tool**

Create `internal/mcp/tools_test.go`:

```go
package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadLogs(t *testing.T) {
	// Create a temp log file
	dir := t.TempDir()
	logFile := filepath.Join(dir, "toad.log")

	lines := []string{
		`time=2026-03-09T10:00:00Z level=INFO msg="server started" port=8099`,
		`time=2026-03-09T10:00:01Z level=DEBUG msg="processing message" channel=general`,
		`time=2026-03-09T10:00:02Z level=ERROR msg="triage failed" error="timeout"`,
		`time=2026-03-09T10:00:03Z level=INFO msg="ribbit complete" duration=2.5s`,
		`time=2026-03-09T10:00:04Z level=WARN msg="rate limited" user=U123`,
	}
	if err := os.WriteFile(logFile, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Read last 3 lines
	result, err := readLogs(logFile, 3, "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.Split(strings.TrimSpace(result), "\n")) != 3 {
		t.Errorf("expected 3 lines, got: %q", result)
	}

	// Filter by level
	result, err = readLogs(logFile, 100, "ERROR", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "triage failed") {
		t.Errorf("expected error line, got: %q", result)
	}
	if strings.Contains(result, "server started") {
		t.Error("should not contain INFO lines when filtering for ERROR")
	}

	// Search filter
	result, err = readLogs(logFile, 100, "", "ribbit", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "ribbit complete") {
		t.Errorf("expected ribbit line, got: %q", result)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestReadLogs -v`
Expected: FAIL — readLogs doesn't exist

**Step 3: Implement tools.go**

Create `internal/mcp/tools.go`:

```go
package mcp

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/scaler-tech/toad/internal/state"
)

type logsArgs struct {
	Lines  int    `json:"lines"  jsonschema:"Number of log lines to return (default 100)"`
	Level  string `json:"level"  jsonschema:"Filter by log level: DEBUG, INFO, WARN, ERROR"`
	Search string `json:"search" jsonschema:"Free-text search filter"`
	Since  string `json:"since"  jsonschema:"Time filter, e.g. 1h, 30m, or 2026-01-15T10:00"`
}

// RegisterLogsTool registers the logs tool on the MCP server.
func RegisterLogsTool(srv *gomcp.Server, logFile string, devIDs map[string]bool) {
	gomcp.AddTool(srv, &gomcp.Tool{
		Name:        "logs",
		Description: "Query toad daemon logs. Filter by level, time, or free-text search. Dev access only.",
	}, func(ctx context.Context, req *gomcp.CallToolRequest, args logsArgs) (*gomcp.CallToolResult, any, error) {
		tok := tokenFromContext(ctx)
		if tok == nil || tok.Role != "dev" {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "The logs tool requires dev access."}},
				IsError: true,
			}, nil, nil
		}

		lines := args.Lines
		if lines <= 0 {
			lines = 100
		}

		result, err := readLogs(logFile, lines, args.Level, args.Search, args.Since)
		if err != nil {
			slog.Error("logs tool error", "error", err)
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: fmt.Sprintf("Error reading logs: %v", err)}},
				IsError: true,
			}, nil, nil
		}

		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: result}},
		}, nil, nil
	})
}

func readLogs(logFile string, maxLines int, level, search, since string) (string, error) {
	f, err := os.Open(logFile)
	if err != nil {
		return "", fmt.Errorf("opening log file: %w", err)
	}
	defer f.Close()

	// Read all lines (log files are bounded by nature of daemon restarts)
	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("reading log file: %w", err)
	}

	// Parse since filter
	var sinceTime time.Time
	if since != "" {
		sinceTime, err = parseSince(since)
		if err != nil {
			return "", fmt.Errorf("invalid since value %q: %w", since, err)
		}
	}

	level = strings.ToUpper(level)

	// Filter from the end
	var matched []string
	for i := len(allLines) - 1; i >= 0 && len(matched) < maxLines; i-- {
		line := allLines[i]
		if line == "" {
			continue
		}
		if level != "" && !strings.Contains(line, "level="+level) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(search)) {
			continue
		}
		if !sinceTime.IsZero() {
			if t, ok := parseLogTime(line); ok && t.Before(sinceTime) {
				continue
			}
		}
		matched = append(matched, line)
	}

	// Reverse to chronological order
	for i, j := 0, len(matched)-1; i < j; i, j = i+1, j-1 {
		matched[i], matched[j] = matched[j], matched[i]
	}

	if len(matched) == 0 {
		return "No matching log lines found.", nil
	}
	return strings.Join(matched, "\n"), nil
}

func parseSince(s string) (time.Time, error) {
	// Try duration format: "1h", "30m", "2h30m"
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	// Try absolute time formats
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time format")
}

func parseLogTime(line string) (time.Time, bool) {
	// slog TextHandler format: time=2026-03-09T10:00:00Z
	idx := strings.Index(line, "time=")
	if idx < 0 {
		return time.Time{}, false
	}
	rest := line[idx+5:]
	end := strings.IndexByte(rest, ' ')
	if end < 0 {
		end = len(rest)
	}
	t, err := time.Parse(time.RFC3339, rest[:end])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/ -run TestReadLogs -v`
Expected: PASS

**Step 5: Commit**

`/release` — "Add logs tool with level, search, and time filtering"

---

### Task 6: Implement `ask` tool with session context

**Files:**
- Modify: `internal/mcp/tools.go`
- Modify: `internal/mcp/server.go`
- Create: `internal/mcp/session.go`
- Create: `internal/mcp/session_test.go`

**Step 1: Write test for session context store**

Create `internal/mcp/session_test.go`:

```go
package mcp

import (
	"testing"
)

func TestSessionStore(t *testing.T) {
	store := newSessionStore()

	// Empty session
	prior := store.GetContext("sess1")
	if prior != nil {
		t.Error("expected nil for new session")
	}

	// Add exchanges
	store.AddExchange("sess1", "What is toad?", "Toad is a daemon that monitors Slack.")
	store.AddExchange("sess1", "How does ribbit work?", "Ribbit answers questions using Claude.")

	prior = store.GetContext("sess1")
	if prior == nil {
		t.Fatal("expected context")
	}
	if len(prior.Exchanges) != 2 {
		t.Errorf("expected 2 exchanges, got %d", len(prior.Exchanges))
	}

	// Clear
	store.Clear("sess1")
	prior = store.GetContext("sess1")
	if prior != nil {
		t.Error("expected nil after clear")
	}

	// Separate sessions don't interfere
	store.AddExchange("sessA", "q1", "a1")
	store.AddExchange("sessB", "q2", "a2")
	if len(store.GetContext("sessA").Exchanges) != 1 {
		t.Error("sessions should be independent")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestSessionStore -v`
Expected: FAIL — types don't exist

**Step 3: Implement session.go**

Create `internal/mcp/session.go`:

```go
package mcp

import "sync"

// Exchange is a single Q&A pair in a session.
type Exchange struct {
	Question string
	Answer   string
}

// SessionContext holds conversation history for an MCP session.
type SessionContext struct {
	Exchanges []Exchange
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionContext
}

func newSessionStore() *sessionStore {
	return &sessionStore{sessions: make(map[string]*SessionContext)}
}

func (s *sessionStore) GetContext(sessionID string) *SessionContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx := s.sessions[sessionID]
	if ctx != nil && len(ctx.Exchanges) == 0 {
		return nil
	}
	return ctx
}

func (s *sessionStore) AddExchange(sessionID, question, answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, ok := s.sessions[sessionID]
	if !ok {
		ctx = &SessionContext{}
		s.sessions[sessionID] = ctx
	}
	ctx.Exchanges = append(ctx.Exchanges, Exchange{Question: question, Answer: answer})
}

func (s *sessionStore) Clear(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/ -run TestSessionStore -v`
Expected: PASS

**Step 5: Implement ask tool registration**

Add to `internal/mcp/tools.go`:

```go
type askArgs struct {
	Question     string `json:"question"     jsonschema:"The question to ask about the codebase"`
	Repo         string `json:"repo"         jsonschema:"Optional repo name to query (auto-detected if omitted)"`
	ClearContext bool   `json:"clear_context" jsonschema:"Reset conversation context for this session"`
}

// AskDeps holds the dependencies the ask tool needs.
type AskDeps struct {
	RibbitEngine  RibbitEngine
	TriageEngine  TriageEngine
	Resolver      RepoResolver
	Repos         []config.RepoConfig
	Sessions      *sessionStore
	RibbitSem     chan struct{}
}

// RibbitEngine abstracts ribbit.Engine for testability.
type RibbitEngine interface {
	Respond(ctx context.Context, msg string, tr *triage.Result, prior *ribbit.PriorContext, repoPath string, repoPaths map[string]string) (*ribbit.Response, error)
}

// TriageEngine abstracts triage.Engine for testability.
type TriageEngine interface {
	Classify(ctx context.Context, msg *islack.IncomingMessage, channelName string) (*triage.Result, error)
}

// RepoResolver abstracts config.Resolver for testability.
type RepoResolver interface {
	Resolve(triageHint string, files []string) *config.RepoConfig
}
```

The actual `RegisterAskTool` function:

```go
func RegisterAskTool(srv *gomcp.Server, deps *AskDeps) {
	gomcp.AddTool(srv, &gomcp.Tool{
		Name:        "ask",
		Description: "Ask toad a question about the codebase. Toad will search the code and answer using its knowledge of the project.",
	}, func(ctx context.Context, req *gomcp.CallToolRequest, args askArgs) (*gomcp.CallToolResult, any, error) {
		tok := tokenFromContext(ctx)
		if tok == nil {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "Authentication required."}},
				IsError: true,
			}, nil, nil
		}

		sessionID := tok.SlackUserID // use user ID as session key
		if args.ClearContext {
			deps.Sessions.Clear(sessionID)
		}

		slog.Info("MCP ask", "user", tok.SlackUser, "question", args.Question)

		// Acquire ribbit semaphore
		select {
		case deps.RibbitSem <- struct{}{}:
			defer func() { <-deps.RibbitSem }()
		case <-ctx.Done():
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "Request cancelled."}},
				IsError: true,
			}, nil, nil
		}

		// Build a synthetic IncomingMessage for triage
		msg := &islack.IncomingMessage{
			Text:      args.Question,
			IsMention: true,
		}

		// Triage
		tr, err := deps.TriageEngine.Classify(ctx, msg, "mcp")
		if err != nil {
			slog.Error("MCP triage failed", "error", err)
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "Sorry, I couldn't process that question."}},
				IsError: true,
			}, nil, nil
		}

		// Only allow questions through MCP
		if tr.Category != "question" && tr.Category != "" {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{
					Text: "I can only answer questions via MCP. For bugs and feature requests, please use Slack!",
				}},
			}, nil, nil
		}

		// Resolve repo
		repo := deps.Resolver.Resolve(args.Repo, tr.FilesHint)
		if repo == nil && args.Repo != "" {
			repo = deps.Resolver.Resolve(tr.Repo, tr.FilesHint)
		}
		if repo == nil {
			repo = config.PrimaryRepo(deps.Repos)
		}

		repoPath := repo.Path
		repoPaths := make(map[string]string)
		for _, r := range deps.Repos {
			repoPaths[r.Path] = r.Name
		}

		// Build prior context from session
		var prior *ribbit.PriorContext
		if sc := deps.Sessions.GetContext(sessionID); sc != nil {
			var summary []string
			for _, ex := range sc.Exchanges {
				summary = append(summary, "Q: "+ex.Question, "A: "+ex.Answer)
			}
			last := sc.Exchanges[len(sc.Exchanges)-1]
			prior = &ribbit.PriorContext{
				Summary:  strings.Join(summary, "\n"),
				Response: last.Answer,
			}
		}

		// Run ribbit
		resp, err := deps.RibbitEngine.Respond(ctx, args.Question, tr, prior, repoPath, repoPaths)
		if err != nil {
			slog.Error("MCP ribbit failed", "error", err)
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{&gomcp.TextContent{Text: "Sorry, I encountered an error answering that."}},
				IsError: true,
			}, nil, nil
		}

		// Store in session
		deps.Sessions.AddExchange(sessionID, args.Question, resp.Text)

		return &gomcp.CallToolResult{
			Content: []gomcp.Content{&gomcp.TextContent{Text: resp.Text}},
		}, nil, nil
	})
}
```

Note: The exact imports and interface types will need to match the actual ribbit/triage/config packages. Check `internal/ribbit/ribbit.go:84` for the `PriorContext` struct and `Respond` signature, and `internal/triage/triage.go:87` for the `Classify` signature.

**Step 6: Verify it compiles**

Run: `go build ./...`
Expected: Clean build (may need import adjustments)

**Step 7: Commit**

`/release` — "Add ask tool with triage, ribbit proxy, and session context"

---

### Task 7: Slack command handler for `/toad connect` and `/toad revoke`

**Files:**
- Modify: `internal/slack/client.go`
- Modify: `internal/slack/events.go`
- Create: `internal/slack/mcp_commands.go`

This task requires understanding how Slack slash commands or DM commands are currently handled. Check `internal/slack/events.go` for the message handler pattern.

**Step 1: Implement command handler**

Create `internal/slack/mcp_commands.go`:

```go
package slack

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/scaler-tech/toad/internal/config"
	"github.com/scaler-tech/toad/internal/state"
	"github.com/slack-go/slack"
)

// MCPCommandHandler handles MCP-related commands in DMs.
type MCPCommandHandler struct {
	db     *state.DB
	api    *slack.Client
	cfg    config.MCPConfig
	host   string // MCP server host for the config snippet
}

func NewMCPCommandHandler(db *state.DB, api *slack.Client, cfg config.MCPConfig) *MCPCommandHandler {
	host := fmt.Sprintf("localhost:%d", cfg.Port)
	return &MCPCommandHandler{db: db, api: api, cfg: cfg, host: host}
}

func (h *MCPCommandHandler) HandleConnect(userID, userName string) {
	// Generate token
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to generate MCP token", "error", err)
		h.dm(userID, "Sorry, I couldn't generate a token right now. Try again later.")
		return
	}
	token := "toad_" + hex.EncodeToString(b)

	role := "user"
	for _, devID := range h.cfg.Devs {
		if devID == userID {
			role = "dev"
			break
		}
	}

	tok := &state.MCPToken{
		Token:       token,
		SlackUserID: userID,
		SlackUser:   userName,
		Role:        role,
		CreatedAt:   time.Now(),
	}
	if err := h.db.SaveMCPToken(tok); err != nil {
		slog.Error("failed to save MCP token", "error", err)
		h.dm(userID, "Sorry, I couldn't save your token. Try again later.")
		return
	}

	snippet := fmt.Sprintf(`Here's your MCP token (role: *%s*). Add this to your Claude Desktop config:

`+"```"+`json
{
  "mcpServers": {
    "toad": {
      "url": "http://%s/mcp",
      "headers": {
        "Authorization": "Bearer %s"
      }
    }
  }
}
`+"```"+`

Keep this token secret! Use `+"`toad revoke`"+` to invalidate it.`, role, h.host, token)

	h.dm(userID, snippet)
	slog.Info("MCP token generated", "user", userName, "role", role)
}

func (h *MCPCommandHandler) HandleRevoke(userID, userName string) {
	if err := h.db.RevokeMCPToken(userID); err != nil {
		slog.Error("failed to revoke MCP token", "error", err)
		h.dm(userID, "Sorry, I couldn't revoke your token. Try again later.")
		return
	}
	h.dm(userID, "Your MCP token has been revoked. Use `toad connect` to generate a new one.")
	slog.Info("MCP token revoked", "user", userName)
}

func (h *MCPCommandHandler) dm(userID, text string) {
	ch, _, _, err := h.api.OpenConversation(&slack.OpenConversationParameters{Users: []string{userID}})
	if err != nil {
		slog.Error("failed to open DM", "error", err, "user", userID)
		return
	}
	_, _, err = h.api.PostMessage(ch.ID, slack.MsgOptionText(text, false))
	if err != nil {
		slog.Error("failed to send DM", "error", err, "user", userID)
	}
}
```

**Step 2: Wire command detection into message handling**

In the message handler (wherever DMs are processed), detect "toad connect" and "toad revoke" messages. Check `internal/slack/events.go` for the exact handler pattern — the command detection should check:
- Message is a DM (channel type = "im")
- Text contains "connect" or "revoke" (case-insensitive)
- Route to `MCPCommandHandler.HandleConnect` or `HandleRevoke`

This wiring depends on the exact event routing in `events.go`. The handler should be injected via the Client struct and called before normal triage/ribbit processing.

**Step 3: Verify it compiles**

Run: `go build ./...`
Expected: Clean build

**Step 4: Commit**

`/release` — "Add Slack DM commands for MCP token connect/revoke"

---

### Task 8: Wire MCP server into daemon startup

**Files:**
- Modify: `cmd/root.go`

**Step 1: Add MCP server initialization after ribbit/triage engine creation**

In `cmd/root.go`, after the ribbit engine and triage engine are created (~line 133), add:

```go
// MCP server
if cfg.MCP.Enabled {
	mcpSrv := mcp.New(cfg.MCP, stateDB)

	// Register tools
	mcp.RegisterLogsTool(mcpSrv.MCPServer(), cfg.Log.File, devSet(cfg.MCP.Devs))
	mcp.RegisterAskTool(mcpSrv.MCPServer(), &mcp.AskDeps{
		RibbitEngine: ribbitEngine,
		TriageEngine: triageEngine,
		Resolver:     resolver,
		Repos:        cfg.Repos,
		Sessions:     mcp.NewSessionStore(),
		RibbitSem:    ribbitSem,
	})

	// Start in background
	go func() {
		if err := mcpSrv.Start(ctx); err != nil {
			slog.Error("MCP server error", "error", err)
		}
	}()

	// Wire Slack MCP commands
	mcpCmds := islack.NewMCPCommandHandler(stateDB, slackClient.API(), cfg.MCP)
	slackClient.SetMCPHandler(mcpCmds)
}
```

Add helper:

```go
func devSet(ids []string) map[string]bool {
	m := make(map[string]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}
```

Note: The exact wiring depends on how `slackClient` exposes its API and whether `SetMCPHandler` needs to be added to the Client struct. Follow the existing patterns in `cmd/root.go` for hooking up callbacks.

**Step 2: Verify it compiles**

Run: `go build ./...`
Expected: Clean build

**Step 3: Manual smoke test**

Add to config:
```yaml
mcp:
  enabled: true
  port: 8099
  devs:
    - YOUR_SLACK_USER_ID
```

Run: `go run ./cmd/toad`
Expected: Logs show "MCP server listening port=8099"

**Step 4: Commit**

`/release` — "Wire MCP server into daemon startup"

---

### Task 9: End-to-end verification

**Step 1: Run full test suite**

Run: `go test ./...`
Expected: All tests pass

**Step 2: Run linting**

Run: `go vet ./...`
Expected: Clean

**Step 3: Check formatting**

Run: `gofmt -l .`
Expected: No output

**Step 4: Manual test with Claude Desktop**

1. Start toad with MCP enabled
2. DM toad "connect" in Slack
3. Receive token + config snippet
4. Add to Claude Desktop MCP config
5. Use `ask` tool in Claude Desktop: "What packages does toad have?"
6. Verify toad responds with codebase knowledge
7. Use `logs` tool: query recent logs
8. Verify logs are returned (or access denied for non-dev tokens)

**Step 5: Final commit**

`/release` — "Toad MCP server: ask + logs tools with Slack auth"
