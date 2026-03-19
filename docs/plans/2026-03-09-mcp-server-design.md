> **Status: COMPLETED** — This feature has been implemented and is running in production.

# Toad MCP Server Design

## Problem

Toad runs on a server, but two audiences need direct access to its capabilities:

1. **Devs** working on toad need observability into the running daemon (logs, state) from within their Claude session.
2. **Non-devs** use Claude Desktop/Code but lack local repo access. They want toad's ribbit Q&A (with its personality, system prompt, and repo knowledge) embedded in their Claude workflow.

## Architecture

A Streamable HTTP MCP server embedded in the toad daemon process. Shares existing ribbit engine, triage engine, state manager, and concurrency controls.

- **Transport:** Streamable HTTP (MCP spec 2025-03-26) on a configurable port (default 8099)
- **Auth:** Bearer tokens generated via Slack, stored in SQLite
- **Two roles:** `dev` (all tools) and `user` (ask only)

## Tools

### `ask`

Proxies questions through toad's ribbit engine.

- **Input:** `{ "question": string, "repo": string (optional), "clear_context": bool (optional) }`
- **Output:** Toad's ribbit response (text)
- **Available to:** All authenticated users

Flow:
1. Receive question + authenticated user context
2. Triage with Haiku (same as Slack) — reject non-questions with a polite message
3. Resolve repo (explicit param, triage hint, or primary fallback)
4. Run ribbit: Claude CLI with `--allowedTools Read,Glob,Grep`
5. Return text response

Session context: each SSE connection gets a session ID. Prior Q&A pairs are stored in memory and passed to ribbit as conversation context (same as Slack thread memory). Context clears when the connection drops, or when `clear_context: true` is passed.

Respects the existing ribbit concurrency semaphore — MCP requests share the pool with Slack.

No tadpole spawning, regardless of triage result.

### `logs`

Filtered access to toad's log file.

- **Input:** `{ "lines": number (default 100), "level": string (optional), "search": string (optional), "since": string (optional, e.g. "1h", "30m", "2024-01-15T10:00") }`
- **Output:** Matching log lines as text
- **Available to:** Dev role only

Reads `~/.toad/toad.log` from the end (tail behavior). Parses slog text format for level/time filtering. Free-text search matches against full log lines.

## Authentication

### Token generation

1. User DMs toad in Slack or uses `/toad connect`
2. Toad generates a token: `toad_` + 32 random hex chars
3. Stores in SQLite: `mcp_tokens` table (token, slack_user_id, slack_username, role, created_at, last_used_at)
4. Role determined by checking slack_user_id against `mcp.devs` config list
5. Toad DMs user the token + ready-to-paste MCP config snippet:

```json
{
  "mcpServers": {
    "toad": {
      "url": "https://your-toad-host:8099/mcp",
      "headers": {
        "Authorization": "Bearer toad_abc123..."
      }
    }
  }
}
```

### Token lifecycle

- Tokens don't expire by default (non-tech users shouldn't deal with rotation)
- `/toad revoke` in Slack to revoke your own token
- `last_used_at` updated on each request for audit
- Optional `mcp.token_ttl` config for future expiry support

## Config

```yaml
mcp:
  enabled: true
  port: 8099
  devs:
    - U12345  # Slack user IDs with dev role
    - U67890
```

## Package Structure

- `internal/mcp/server.go` — Streamable HTTP transport, auth middleware, session management
- `internal/mcp/tools.go` — tool definitions and handlers (ask, logs)
- `internal/mcp/tokens.go` — token generation, validation, DB operations

## Integration Points

- Starts as part of the main toad daemon in `cmd/root.go`
- Shares ribbit engine and triage engine (no duplication)
- Token table added to SQLite schema in `internal/state/`
- Slack command handler for `/toad connect` and `/toad revoke` added to `internal/slack/`
- MCP requests logged with Slack user ID for audit trail
