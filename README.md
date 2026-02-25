# 🐸 toad

An AI-powered coding agent that lives in your Slack workspace. Drop a bug report or feature request in any channel, and toad spawns an autonomous agent that writes the code, runs your tests, and opens a PR — all in minutes.

## 🐸 The pond — what is toad?

Toad turns Slack into an engineering intake queue. Instead of bugs and feature requests piling up, toad listens, understands, and acts. It's built on [Claude Code](https://docs.anthropic.com/en/docs/claude-code) and designed for teams that want AI handling the small stuff so humans can focus on the big stuff.

Everything in toad is named after the lifecycle of a frog. Here's the glossary:

| Term | What it means |
|------|---------------|
| 🐸 **Toad** | The daemon itself — sits in your Slack pond, watching for messages |
| 🥚 **Triage** | Every message gets classified by Haiku in ~1 second: is it a bug? feature? question? how big? |
| 🐸 **Ribbit** | A codebase-aware reply to a question — Sonnet reads your code with read-only tools and answers in-thread |
| 🐣 **Tadpole** | An autonomous coding agent — creates a git worktree, invokes Claude Code, validates with your tests, and opens a PR |
| 👑 **Toad King** | The digest engine — passively watches all messages, batch-analyzes them, and auto-spawns tadpoles for obvious one-shot fixes |
| 🔁 **PR Watch** | After a tadpole ships a PR, toad watches for review comments and auto-spawns fix tadpoles (up to 3 rounds) |

## 🐣 How it works

```
Slack message → Triage (Haiku, ~1s) → Route by category:
  🐣 bug/feature  → spawn tadpole → worktree → Claude Code → validate → PR
  🐸 question     → ribbit reply (Sonnet + read-only codebase tools)
  👀 passive bug  → ribbit with 🐸 CTA to spawn tadpole on demand
```

**Tadpoles** run the full lifecycle autonomously: create a git worktree, invoke Claude Code to make changes, validate with your test/lint commands, retry on failure, then push and open a PR. The PR is the review gate — toad ships fast and lets humans approve.

**Ribbits** are for when you just need an answer. Mention toad with a question and it reads your codebase using Sonnet with read-only tools (Glob, Grep, Read), then replies in-thread with context-aware answers. Thread memory means follow-ups stay coherent.

**The Toad King** (optional) goes a step further — it passively collects every non-bot message in your channels, batch-analyzes them with Haiku on an interval, and auto-spawns tadpoles for high-confidence one-shot fixes. Think of it as a vigilant frog that catches bugs before anyone files them.

## 📋 Requirements

- Go 1.25+
- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) (`claude`)
- [GitHub CLI](https://cli.github.com) (`gh`), authenticated
- A Slack app with Socket Mode enabled

## 🚀 Install

### macOS and Linux

Install with [Homebrew](https://brew.sh/) (recommended):

```bash
brew tap cdre-ai/tap https://github.com/cdre-ai/homebrew-tap
brew install --cask toad
```

> **macOS security note:** If macOS blocks the app with "cannot be opened because the developer cannot be verified", the cask's post-install hook should handle this automatically. If not:
> ```bash
> xattr -d com.apple.quarantine $(which toad)
> ```

### Windows

Install with [Scoop](https://scoop.sh/):

```bash
scoop bucket add cdre-ai https://github.com/cdre-ai/homebrew-tap
scoop install toad
```

### Binary releases

Download pre-built binaries for Windows, macOS, or Linux from the [latest release](https://github.com/cdre-ai/toad/releases/latest).

### Go install

```bash
go install github.com/cdre-ai/toad@latest
```

### Build from source

```bash
git clone https://github.com/cdre-ai/toad.git
cd toad
make build
```

## 🔧 Quick start

### 1. Run the setup wizard

```bash
toad init
```

This walks you through creating a Slack app, entering tokens, configuring your repo, and optional features. It saves everything to `.toad.yaml`.

### 2. Start the daemon

```bash
toad
```

Toad connects to Slack via Socket Mode, auto-joins public channels, and starts listening.

### 3. Try it out

Mention `@toad` in any channel with a question or bug report and watch it work.

> For detailed Slack app setup, configuration options, and advanced features, see the **[Setup Guide](SETUP.md)**.

## 🐸 CLI commands

| Command | Description |
|---------|-------------|
| `toad` | Start the daemon |
| `toad init` | Interactive setup wizard |
| `toad run "task"` | Spawn a tadpole from the CLI (no Slack needed) |
| `toad status` | Open live monitoring dashboard in browser |
| `toad version` | Print version info |
| `toad update` | Self-update to latest version |

## 🏛️ Architecture

```
cmd/
  root.go          Cobra command: toad (daemon), message routing
  run.go           toad run (CLI one-shot)
  init.go          toad init (setup wizard)
  status.go        toad status (web dashboard)
  version.go       toad version (build info via ldflags)
  update.go        toad update (self-update via Homebrew)

internal/
  slack/           Socket Mode client, event routing, dedup
  triage/          Haiku classification (category, size, keywords, files)
  ribbit/          Sonnet responses with read-only codebase tools
  tadpole/         Worktree, Claude runner, validation, shipping, pool
  state/           In-memory + SQLite state, crash recovery
  reviewer/        PR review comment watcher, fix tadpole spawning
  digest/          Toad King: batch analysis, auto-spawn with guardrails
  issuetracker/    Generic issue tracker interface (Linear integration)
  update/          Version checking and self-update
  config/          YAML config with cascading defaults
  tui/             Shared theme for init wizard
  log/             Structured logging setup
```

### 💾 State & recovery

State is persisted to SQLite (`~/.toad/state.db`) with WAL mode. On startup, toad marks stale runs as failed and cleans orphaned worktrees. The dashboard reads directly from SQLite, so it works even when the daemon is stopped.

### 🏊 Concurrency

Separate semaphores keep Q&A responsive while tadpoles run:
- **Ribbit pool**: `max_concurrent * 3` (fast, seconds)
- **Tadpole pool**: `max_concurrent` (slow, minutes)

## 🛠️ Development

```bash
make build                  # Build binary
make test                   # Run tests with race detector
make lint                   # Run golangci-lint
make vet                    # Run go vet
make fmt                    # Format code
make clean                  # Remove binary and dist/
```

To test a single package:

```bash
go test ./internal/state/
```

## 📄 License

[Elastic License 2.0 (ELv2)](LICENSE) — free to use, modify, and distribute. You may not offer toad as a hosted/managed service.

---

*Built with [Claude Code](https://docs.anthropic.com/en/docs/claude-code). Toad eats bugs. 🐸*
