# Slack Thinking Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace temporary emoji reactions with Slack's native `assistant.threads.setStatus` thinking indicator.

**Architecture:** Add `SetStatus`/`ClearStatus` methods to the Slack client wrapping `slack-go/slack`'s `SetAssistantThreadsStatusContext`. Replace all temporary reaction calls (`:eyes:`, `:thinking_face:`) in handlers with `SetStatus`, delete all `RemoveReaction("eyes")` calls, and add a 90-second refresh ticker for the tadpole coding phase. Keep persistent reactions (`:hatching_chick:`, `:white_check_mark:`, `:x:`, `:speech_balloon:`, `:warning:`) unchanged.

**Tech Stack:** Go, `github.com/slack-go/slack` v0.18.0 (`AssistantThreadsSetStatusParameters`, `SetAssistantThreadsStatusContext`)

---

### Task 1: Add SetStatus and ClearStatus to Slack client

**Files:**
- Modify: `internal/slack/responder.go:101-133` (after existing reaction methods)
- Test: `internal/slack/client_test.go`

- [ ] **Step 1: Write failing tests for SetStatus and ClearStatus**

Add to `internal/slack/client_test.go`:

```go
func TestSetStatus_NilAPI(t *testing.T) {
	// SetStatus should not panic with a nil api client
	c := &Client{}
	c.SetStatus("C123", "1234.5678", "thinking...")
}

func TestClearStatus_NilAPI(t *testing.T) {
	c := &Client{}
	c.ClearStatus("C123", "1234.5678")
}

func TestSetStatus_WithLoadingMessages(t *testing.T) {
	// SetStatus should not panic with a nil api client even with loading messages
	c := &Client{}
	c.SetStatus("C123", "1234.5678", "thinking...", "loading 1", "loading 2")
}
```

Note: `SetStatus` is best-effort with no return value, and the underlying `c.api` is a concrete `*slack.Client` (not an interface), so mock-based parameter verification is not practical without refactoring. The nil-guard tests verify the safety path; the actual API call is exercised by the Slack integration itself.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/slack/ -run TestSetStatus -v`
Expected: FAIL — `SetStatus` method not found

- [ ] **Step 3: Implement SetStatus and ClearStatus**

Add to `internal/slack/responder.go` after the `SwapReaction` method (line 133):

```go
// SetStatus shows a native Slack thinking indicator on a thread.
// The status auto-clears when the bot posts a reply to the thread, or after 2 minutes.
// Best-effort: errors are logged, not returned (purely cosmetic).
func (c *Client) SetStatus(channel, threadTS, status string, loadingMessages ...string) {
	if c.api == nil {
		return
	}
	err := c.api.SetAssistantThreadsStatusContext(context.Background(), slack.AssistantThreadsSetStatusParameters{
		ChannelID:       channel,
		ThreadTS:        threadTS,
		Status:          status,
		LoadingMessages: loadingMessages,
	})
	if err != nil {
		slog.Debug("failed to set thread status", "error", err, "status", status)
	}
}

// ClearStatus explicitly clears the thinking indicator on a thread.
// Use on error paths where no reply will be posted to auto-clear it.
func (c *Client) ClearStatus(channel, threadTS string) {
	c.SetStatus(channel, threadTS, "")
}
```

Add `"context"` to the imports in `responder.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/slack/ -run "TestSetStatus|TestClearStatus" -v`
Expected: PASS

- [ ] **Step 5: Run full test suite and format check**

Run: `go test ./internal/slack/... -v && gofmt -l internal/slack/`
Expected: All pass, no formatting issues

- [ ] **Step 6: Commit**

```
feat: add SetStatus and ClearStatus for Slack thinking indicator
```

---

### Task 2: Replace temporary reactions in handleTriggered

**Files:**
- Modify: `cmd/handlers.go:131` (React eyes), lines 171/186/209/276/317/335/360 (RemoveReaction eyes), line 229 (SwapReaction eyes→thinking_face), lines 203/354/393 (SwapReaction eyes→warning), line 415 (SwapReaction eyes→speech_balloon)

- [ ] **Step 1: Replace initial `React("eyes")` with `SetStatus`**

At line 131, replace:
```go
slackClient.React(msg.Channel, msg.Timestamp, "eyes")
```
with:
```go
slackClient.SetStatus(msg.Channel, threadTS, "Triaging message...",
	"Hopping to it...", "Reading the lily pad...", "Warming up...")
```

- [ ] **Step 2: Delete all `RemoveReaction("eyes")` calls**

Delete these lines (7 occurrences in `handleTriggered`):
- Line 171: `slackClient.RemoveReaction(msg.Channel, msg.Timestamp, "eyes")`
- Line 186: `slackClient.RemoveReaction(msg.Channel, msg.Timestamp, "eyes")`
- Line 209: `slackClient.RemoveReaction(msg.Channel, msg.Timestamp, "eyes")`
- Line 276: `slackClient.RemoveReaction(msg.Channel, msg.Timestamp, "eyes")`
- Line 317: `slackClient.RemoveReaction(msg.Channel, msg.Timestamp, "eyes")`
- Line 335: `slackClient.RemoveReaction(msg.Channel, msg.Timestamp, "eyes")`
- Line 360: `slackClient.RemoveReaction(msg.Channel, msg.Timestamp, "eyes")`

- [ ] **Step 3: Replace `SwapReaction("eyes", "thinking_face")` with SetStatus**

At line 229, replace:
```go
slackClient.SwapReaction(msg.Channel, msg.Timestamp, "eyes", "thinking_face")
```
with:
```go
slackClient.ClearStatus(msg.Channel, threadTS)
```

Note: The reply that follows will also clear the status, but calling ClearStatus explicitly ensures the thinking indicator doesn't persist alongside the "I'm not sure" message.

- [ ] **Step 4: Replace `SwapReaction("eyes", "warning")` with ClearStatus + React**

Three occurrences — replace each `SwapReaction(msg.Channel, msg.Timestamp, "eyes", "warning")` with:
```go
slackClient.ClearStatus(msg.Channel, threadTS)
slackClient.React(msg.Channel, msg.Timestamp, "warning")
```

At lines 203, 354, 393.

- [ ] **Step 5: Replace `SwapReaction("eyes", "speech_balloon")` with React only**

At line 415, replace:
```go
slackClient.SwapReaction(msg.Channel, msg.Timestamp, "eyes", "speech_balloon")
```
with:
```go
slackClient.React(msg.Channel, msg.Timestamp, "speech_balloon")
```

The status auto-clears when the ribbit reply is posted, so no explicit ClearStatus needed.

- [ ] **Step 6: Add SetStatus for investigation phase**

Before the `investigateTriggered` call at line 261, add:
```go
slackClient.SetStatus(msg.Channel, threadTS, "Investigating the codebase...",
	"Searching the swamp...", "Following the breadcrumbs...")
```

- [ ] **Step 7: Add SetStatus for ribbit phase**

Before the ribbit `Respond` call at line 390, add:
```go
slackClient.SetStatus(msg.Channel, threadTS, "Reading the codebase...",
	"Searching the swamp...", "Chasing down the answer...")
```

- [ ] **Step 8: Build and verify**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 9: Commit**

```
feat: replace temporary emoji reactions with SetStatus in handleTriggered
```

---

### Task 3: Replace temporary reactions in retry intent path

**Files:**
- Modify: `cmd/handlers.go:166-211` (retry intent block)

- [ ] **Step 1: Update retry intent error path**

The retry spawn failure at line 203 was already updated in Task 2 (SwapReaction → ClearStatus + React). Verify that line 209 RemoveReaction was also deleted.

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: Clean build

- [ ] **Step 3: Commit** (if any standalone changes — otherwise fold into Task 2)

---

### Task 4: Add SetStatus to interactive.go button flow

**Files:**
- Modify: `internal/slack/interactive.go:45-49`

- [ ] **Step 1: Add SetStatus call before processing indicator**

In `handleInteractive`, after line 43 (`slog.Info("fix button clicked"...)`), add a `SetStatus` call. The `handleInteractive` function has access to `c *Client`:

```go
c.SetStatus(channel, threadTS, "Spawning tadpole...",
	"Hatching...", "Preparing the lily pad...")
```

Add this before the `processingBlocks` line (line 46). The `SpawnedByBlocks` context block with `:hourglass_flowing_sand:` stays as-is since it's a message update (persistent), not a temporary status.

- [ ] **Step 2: Build and verify**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 3: Commit**

```
feat: add SetStatus thinking indicator on fix button click
```

---

### Task 5: Add SetStatus to tadpole runner with coding phase ticker

**Files:**
- Modify: `internal/tadpole/runner.go:97-101` (initial status), lines 125/147/190/202/258/271 (phase updates), lines 120/300-301 (completion)

- [ ] **Step 1: Add setStatus helper to Runner**

Add a helper method to Runner after `swapReact` (line 354):

```go
// setStatus shows a Slack thinking indicator for the current phase.
func (r *Runner) setStatus(task Task, status string, loadingMessages ...string) {
	if r.slack == nil || task.SlackChannel == "" || task.SlackThreadTS == "" {
		return
	}
	r.slack.SetStatus(task.SlackChannel, task.SlackThreadTS, status, loadingMessages...)
}

// clearStatus explicitly clears the Slack thinking indicator.
func (r *Runner) clearStatus(task Task) {
	if r.slack == nil || task.SlackChannel == "" || task.SlackThreadTS == "" {
		return
	}
	r.slack.ClearStatus(task.SlackChannel, task.SlackThreadTS)
}
```

- [ ] **Step 2: Add SetStatus calls at each phase**

Add `setStatus` calls alongside existing `updateStatus` calls:

At line 125 (worktree setup `updateStatus`), add after the `updateStatus`:
```go
r.setStatus(task, "Setting up worktree...")
```

At line 147 (coding agent), add after the `updateStatus`:
```go
r.setStatus(task, "Coding agent is working...")
```

At line 190 (validating), add after the `updateStatus`:
```go
r.setStatus(task, "Validating changes...")
```

At line 202-203 (retry), add after the `updateStatus`:
```go
r.setStatus(task, fmt.Sprintf("Retrying — attempt %d/%d...", attempt+1, maxRetries))
```

At line 258 (pushing fix), add after the `updateStatus`:
```go
r.setStatus(task, "Pushing fix...")
```

At line 271 (opening PR), add after the `updateStatus`:
```go
r.setStatus(task, fmt.Sprintf("Opening %s...", vcsProvider.PRNoun()))
```

- [ ] **Step 3: Add coding phase ticker for 2-minute timeout refresh**

Wrap the `r.agent.Run` call at line 174 with a refresh ticker:

```go
statusDone := make(chan struct{})
statusTicker := time.NewTicker(90 * time.Second)
go func() {
	defer statusTicker.Stop()
	for {
		select {
		case <-statusTicker.C:
			r.setStatus(task, "Coding agent is working...")
		case <-statusDone:
			return
		case <-ctx.Done():
			return
		}
	}
}()

agentOut, err := r.agent.Run(ctx, agent.RunOpts{
	// ... existing opts ...
})
close(statusDone)
```

- [ ] **Step 4: Add same ticker pattern for retry agent runs**

The retry `r.agent.Run` at line 206 can also exceed 2 minutes. Add the same ticker pattern:

```go
retryDone := make(chan struct{})
retryTicker := time.NewTicker(90 * time.Second)
go func() {
	defer retryTicker.Stop()
	for {
		select {
		case <-retryTicker.C:
			r.setStatus(task, fmt.Sprintf("Retrying — attempt %d/%d...", attempt+1, maxRetries))
		case <-retryDone:
			return
		case <-ctx.Done():
			return
		}
	}
}()

_, err := r.agent.Run(ctx, agent.RunOpts{
	// ... existing opts ...
})
close(retryDone)
```

- [ ] **Step 5: Clear status on completion**

At line 120 (fail function), add before `r.swapReact(task, "hatching_chick", "x")`:
```go
r.clearStatus(task)
```

At line 300-301 (success), add before `r.swapReact(task, "hatching_chick", "white_check_mark")`:
```go
r.clearStatus(task)
```

- [ ] **Step 6: Build and verify**

Run: `go build ./... && go vet ./...`
Expected: Clean build

- [ ] **Step 7: Run all tests**

Run: `go test ./...`
Expected: All pass

- [ ] **Step 8: Commit**

```
feat: add SetStatus thinking indicators to tadpole runner with refresh ticker
```

---

### Task 6: Update SETUP.md with new scope

**Files:**
- Modify: `SETUP.md:121-131`

- [ ] **Step 1: Add assistant:write scope**

After the `chat:write` line (line 126), add:
```
   - `assistant:write` — show thinking indicators
```

- [ ] **Step 2: Commit**

```
docs: add assistant:write scope to SETUP.md
```

**Manual step:** The Slack app manifest must also be updated in the Slack dashboard (api.slack.com) to add the `assistant:write` scope under Bot Token Scopes, then reinstall the app to the workspace.

---

### Task 7: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All pass

- [ ] **Step 2: Run format check**

Run: `gofmt -l .`
Expected: No output (all formatted)

- [ ] **Step 3: Run vet**

Run: `go vet ./...`
Expected: Clean

- [ ] **Step 4: Manual review of all changes**

Review the diff to verify:
- No `:eyes:` reactions remain in handlers
- No `RemoveReaction("eyes")` calls remain
- All `SwapReaction("eyes", ...)` replaced appropriately
- Persistent reactions (`:hatching_chick:`, `:white_check_mark:`, `:x:`, `:speech_balloon:`, `:warning:`) unchanged
- Ticker goroutines have clean shutdown via done channels
- `SetStatus` is best-effort (no error returns blocking flow)