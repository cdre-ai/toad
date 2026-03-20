# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Git Policy

**NEVER create git commits directly.** All commits and releases must go through the `/release` skill/command. Do not run `git commit`, `git add`, or `git push` outside of that flow.

## Build & Test

```bash
go build ./...              # Build all packages
go test ./...               # Run all tests
go test ./internal/state/   # Run tests for a single package
go test ./... -run TestFoo  # Run a specific test
go vet ./...                # Lint
gofmt -l .                  # Check formatting (must be clean before committing)
```

**Before committing:** Always run `gofmt -l .` and fix any output with `gofmt -w <file>`. CI enforces `gofmt` formatting and will fail on unformatted code.

No external test infrastructure needed — tests use in-memory SQLite (`:memory:`) and same-package access to unexported functions.

## Architecture

Toad is a Go daemon that monitors Slack channels, triages messages with Claude Haiku, and either answers questions (ribbit) or spawns autonomous coding agents (tadpoles) that create PRs.

**Message flow:** Slack event -> triage (Haiku, ~1s) -> route by category:
- `bug`/`feature` -> auto-spawn tadpole (worktree -> Claude -> validate -> PR)
- `question` -> ribbit reply (Claude + read-only tools)
- Passive: high-confidence bugs get a ribbit with :frog: CTA

**Tadpole lifecycle:** `CreateWorktree` -> `RunClaude` (CLI subprocess) -> `Validate` (test+lint+file count) -> retry loop -> `ship` (push + `gh pr create`) -> `RemoveWorktree`

**Multi-repo routing:** Config supports multiple repos via `repos:` list. At startup, `BuildProfiles` auto-detects each repo's stack/module from manifest files. Triage and digest prompts include repo profiles so Haiku can suggest a `"repo"` name. The `Resolver` verifies with file-existence stat checks (`resolver.go`), falling back to triage hint, then `primary` repo.

**Service-aware validation:** When `repos[].services` is configured, `resolveChecks()` in `validate.go` matches changed files to services by path prefix and runs each service's lint/test commands from its subdirectory. Unmatched files fall back to root-level commands. This ensures tadpole PRs pass per-service CI (e.g. PHP services use `make stan && make cs`, Python services use `make lint`).

**Key patterns:**
- **Write-through state**: `state.Manager` caches runs in-memory maps, writes through to SQLite on every mutation. `NewManager()` is in-memory only (tests), `NewPersistentManager(db)` hydrates from DB.
- **Claim/Unclaim**: Atomic thread reservation prevents duplicate tadpoles from TOCTOU races. `Claim` reserves with empty placeholder, `Track` fills in the run ID, `Unclaim` on error removes placeholder only.
- **Concurrency**: Separate semaphores for ribbits (`MaxConcurrent*3`) and tadpoles (`MaxConcurrent`). Each runs in its own goroutine.
- **Channel access**: Bot auto-joins all public channels on startup. If `channels` config is empty, no filtering — events from all joined channels are processed.

**Packages:**
- `cmd/` — Cobra commands: `toad` (daemon), `toad run` (CLI one-shot), `toad init` (setup), `toad status`. Daemon logic split into `root.go` (bootstrap), `handlers.go` (message routing), `investigation.go` (prompts/parsing), `helpers.go` (utilities)
- `internal/slack/` — Socket Mode client, event routing, dedup, reply tracking
- `internal/triage/` — Haiku classification (actionable, category, size, keywords, files)
- `internal/ribbit/` — Sonnet with read-only tools, thread memory context, retry on empty result
- `internal/tadpole/` — Worktree, Claude runner, validation, pre-flight diff check, shipping, pool
- `internal/state/` — In-memory + SQLite state, crash recovery
- `internal/reviewer/` — Poll GitHub for PR review comments, spawn fix tadpoles
- `internal/digest/` — Toad King: batch messages, Haiku analysis, auto-spawn with guardrails. Split into `digest.go` (engine), `analyze.go` (LLM analysis), `chunking.go` (batching), `guardrails.go` (filtering)
- `internal/config/` — YAML config loading with cascading defaults, multi-repo profiles and resolver
- `internal/personality/` — 22-trait adaptive behavior system with outcome-based learning, dampening, and drift caps (±0.30)
- `internal/agent/` — Agent CLI abstraction (Claude Code subprocess), provider interface for swappable backends
- `internal/vcs/` — VCS provider abstraction (GitHub via `gh`, GitLab via `gitlab`), PR operations, CI status, suggested reviewers
- `internal/issuetracker/` — Linear integration: issue extraction, detail+comment fetching, assignee gating, crossposting
- `internal/mcp/` — Model Context Protocol server: `ask`, `logs`, `watches`, `query` tools with token auth
- `internal/tui/` — Shared huh theme for init wizard
- `internal/update/` — Auto-update mechanism via Homebrew
- `internal/log/` — Structured logging setup (slog with optional file output)
- `internal/preflight/` — Pre-run validation checks
- `internal/toadpath/` — Home directory resolution (`~/.toad` or `$TOAD_HOME`)

## Important Details

- Claude is invoked as a CLI subprocess (`claude --print --output-format json`), not via API
- Tadpoles use `--permission-mode acceptEdits --allowedTools Read,Write,Edit,Glob,Grep,Bash,Agent`, ribbit uses `--allowedTools Read,Glob,Grep`
- SQLite uses `modernc.org/sqlite` (pure Go, no CGo) with WAL mode; `dbRetry` wrapper retries on SQLITE_BUSY
- Config loads: defaults -> `~/.toad/config.yaml` -> `.toad.yaml` -> env vars
- All Slack tokens come from env vars (`TOAD_SLACK_APP_TOKEN`, `TOAD_SLACK_BOT_TOKEN`) or `.toad.yaml`
- Slack API calls have a 30-second HTTP timeout to prevent hung goroutines
- State DB at `~/.toad/state.db`, worktrees at `~/.toad/worktrees/`
- On startup, `RecoverOnStartup` marks stale runs as failed and cleans orphaned worktrees
- Personality traits have a learned cap of ±0.30 from baseline; balanced positive/negative signals on PR merge/close
- Tadpoles check diff vs main after validation but before shipping to catch already-fixed issues early
- Ribbit retries once on empty result (with +5 max_turns if first attempt hit max turns)
- Digest confidence floor is 0.85 in comment mode (dry-run + comment investigation); personality-driven otherwise
- CTA "Let Toad fix this" buttons appear on bug/feature ribbits, investigation findings, and failure messages
- Linear ticket comments (up to 20) are fetched alongside issue details for investigation context
- GitHub Actions: `tag.yml` (manual version tagging with auto-bump) triggers `release.yml` (GoReleaser + Docker)
