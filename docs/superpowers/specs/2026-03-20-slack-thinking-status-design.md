# Slack Thinking Status Integration

Replace temporary emoji reactions with Slack's `assistant.threads.setStatus` API for a polished thinking/loading indicator, while keeping persistent reactions for final states.

## Context

Toad currently uses emoji reactions as status indicators through the entire message lifecycle (`:eyes:` → `:speech_balloon:`, `:hatching_chick:` → `:white_check_mark:`/`:x:`). Slack's `assistant.threads.setStatus` API provides a native, styled loading indicator that auto-clears when a reply is posted. Linear's Slack app recently adopted this pattern.

Key constraint: `setStatus` auto-clears after 2 minutes of inactivity. Tadpole runs can take up to 10 minutes, so long-running phases need periodic refresh.

## Reaction Split

**Replace with `setStatus` (temporary indicators):**
- `:eyes:` — initial acknowledgment
- `:thinking_face:` — unclear request
- `:hourglass_flowing_sand:` — spawning indicator (in `interactive.go` `SpawnedByBlocks` context block)

**Keep as persistent reactions (final states):**
- `:hatching_chick:` — tadpole is running (persists through long runs that survive the 2-minute status timeout)
- `:white_check_mark:` — tadpole success
- `:x:` — tadpole failure
- `:speech_balloon:` — ribbit completed
- `:warning:` — error (with error message in thread)

## Scope Requirements

The `assistant.threads.setStatus` API requires the **`assistant:write`** scope. This means:
- Update the Slack app manifest to add `assistant:write`
- Update `SETUP.md` to list the new scope in the required bot token scopes

## Slack Client (`internal/slack/client.go`)

Two new methods:

- **`SetStatus(channel, threadTS, status string, loadingMessages ...string)`** — Calls `assistant.threads.setStatus` via `slack-go/slack`'s native `SetAssistantThreadsStatusContext` method (supported in v0.18.0+). The variadic `loadingMessages` param optionally passes rotating loading messages. Best-effort: log errors, don't return them.
- **`ClearStatus(channel, threadTS)`** — Sends empty status string. Used on error paths where no reply will auto-clear it.

Note: `React` returns errors but callers generally ignore them. `SetStatus` will not return errors — log-and-discard internally — since it is purely cosmetic and should never block the main flow.

## Handler Changes (`cmd/handlers.go`)

### `handleTriggered` flow

1. Replace `React("eyes")` with `SetStatus(channel, ts, "Triaging message...", "Hopping to it...", "Reading the lily pad...")` — playful for the short triage phase
2. **Delete all 7 `RemoveReaction("eyes")` call sites** scattered through `handleTriggered` — no longer needed since `setStatus` auto-clears
3. After triage routes to ribbit: `SetStatus(channel, ts, "Reading the codebase...", "Searching the swamp...", "Following the breadcrumbs...")`
4. After triage routes to tadpole: status handed off to the tadpole runner
5. Replace `SwapReaction("eyes", "thinking_face")` for unclear requests with `SetStatus(channel, ts, "Hmm, thinking about this...")`
6. On error paths: replace `SwapReaction("eyes", "warning")` with `ClearStatus` + `React("warning")` + error message
7. **Investigation phase** (`investigateTriggered` call): add `SetStatus(channel, ts, "Investigating the codebase...")` before the investigation Sonnet call

### Retry intent path

The "try again" flow in `handleTriggered` (when a user retries a failed tadpole in-thread) currently adds `:eyes:` → claim → resolve → spawn. Apply the same `SetStatus` replacement here: `SetStatus` instead of `React("eyes")`, and remove the corresponding `RemoveReaction("eyes")` calls.

### `handleTadpoleRequest` flow (button-triggered spawns)

- The `:hourglass_flowing_sand:` indicator lives in `SpawnedByBlocks` (`internal/slack/responder.go`) and is triggered from `handleInteractive` (`internal/slack/interactive.go`), not from `handleTadpoleRequest` in `handlers.go`
- Replace the hourglass context block text with a `SetStatus(channel, ts, "Spawning tadpole...")` call in `handleInteractive`
- Keep the `SpawnedByBlocks` block update for showing "Tadpole spawned by <username>" — that is a persistent message update, not a status indicator
- Tadpole runner takes over status from there

### `handlePassive` flow

Currently has no status indicator. Leave as-is — passive ribbits appear silently and adding a thinking indicator to an unsolicited reply would be confusing.

## Digest Engine (`internal/digest/digest.go`)

The digest engine auto-spawns tadpoles and uses `ReactFunc` to add `:hatching_chick:` reactions. Since `:hatching_chick:` is a persistent reaction (not being replaced), **no changes needed** for the digest spawn path.

If digest gains an investigation/analysis phase visible to users in the future, `setStatus` could be added then. Out of scope for now.

## Tadpole Runner (`internal/tadpole/runner.go`)

Phase-based `SetStatus` calls alongside existing `updateStatus` calls. Informative (not playful) since accuracy matters for long runs:

| Phase | Status text |
|-------|------------|
| Spawn | `"Setting up worktree..."` |
| Coding | `"Coding agent is working..."` |
| Validating | `"Validating changes..."` |
| Retry | `"Retrying — attempt 2/3..."` |
| Pushing | `"Pushing and opening PR..."` |

### Coding phase ticker

The `RunClaude` call can exceed 2 minutes. Before calling the agent, start a `time.Ticker` at 90-second intervals that re-sends the current status. Stop it when the agent returns:

```go
ticker := time.NewTicker(90 * time.Second)
defer ticker.Stop()
go func() {
    for range ticker.C {
        slackClient.SetStatus(channel, ts, "Coding agent is working...")
    }
}()
result, err := r.agent.Run(ctx, runOpts)
```

On completion: `ClearStatus` before adding persistent reaction (`:white_check_mark:` or `:x:`).

The `:hatching_chick:` reaction on the original message stays as the persistent "tadpole running" indicator — it survives the 2-minute status timeout and gives users a visual marker that a tadpole is active.

## Ribbit Flow (`internal/ribbit/ribbit.go`)

No changes needed. Status is set by the handler before calling `Respond()` and auto-clears when the reply is posted. On retry (empty result → retry with +5 max turns), the status stays active since no reply has been posted. On error, the handler calls `ClearStatus` before adding `:warning:` + error message.

## Loading Messages Style

- **Triage/ribbit phases (short):** Playful toad personality — "Hopping to it...", "Searching the swamp..."
- **Tadpole phases (long, informative):** Functional and accurate — "Coding agent is working...", "Validating changes..."

## Testing

- Unit tests for `SetStatus`/`ClearStatus` — verify correct API call with params
- Update existing handler tests — replace `React("eyes")` assertions with `SetStatus` assertions
- Ticker test for tadpole runner — verify refresh goroutine starts/stops cleanly, no leaks
- All mocked via existing Slack client interface — no integration tests needed

## Setup Changes

- Add `assistant:write` to required bot token scopes in Slack app manifest
- Update `SETUP.md` scope list to include `assistant:write`