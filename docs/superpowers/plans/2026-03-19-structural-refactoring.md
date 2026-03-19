# Structural Refactoring Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the two largest files (cmd/root.go and internal/digest/digest.go) into focused modules and archive completed plan docs.

**Architecture:** Pure code moves — extract functions into new files within the same package. No logic changes, no signature changes, no new packages. Every intermediate step must compile and pass tests.

**Tech Stack:** Go (same package, file-level splitting only)

---

## File Structure

### cmd/ package (currently root.go = 1,733 lines)
| File | Responsibility | Key functions |
|------|---------------|---------------|
| `root.go` | Daemon bootstrap, init, goroutine wiring | `Execute`, `runDaemon`, `daemonCounters` |
| `handlers.go` | Slack message routing and response handlers | `handleMessage`, `handleTriggered`, `handlePassive`, `handleTadpoleRequest` |
| `investigation.go` | Investigation prompts, execution, parsing | `investigateOpportunity`, `investigateTriggered`, `parseInvestigateResult`, `formatTicketContext`, `buildTaskDescription`, prompt constants |
| `helpers.go` | Small utility functions, repo sync, VCS resolver | `enrichWithIssueDetails`, `isRetryIntent`, `hasFailedTadpole`, `truncate`, `stripCodeFences`, `findMatchingBrace`, `syncRepos`, `syncAll`, `buildVCSResolver` |

### internal/digest/ package (currently digest.go = 1,198 lines)
| File | Responsibility | Key functions |
|------|---------------|---------------|
| `digest.go` | Engine struct, lifecycle, opportunity processing | Types, `New`, `Run`, `Collect`, `flush`, `processOpportunities`, `ResumeInvestigations`, `Stats`, `scopeKey`, `recordActedIssue`, `isActedIssue`, `trySpawn` |
| `analyze.go` | LLM analysis, prompt, parsing | `digestPrompt`, `analyze`, `analyzeWithRetry`, `parseOpportunities`, `stripDigestCodeFences`, `findMatchingBracket` |
| `chunking.go` | Message batching and dedup | `buildChunks`, `dedupChannel` |
| `guardrails.go` | Confidence/category/size filtering | `passesGuardrails` |

---

## Task 1: Split cmd/root.go

### Task 1a: Extract helpers.go

**Files:**
- Create: `cmd/helpers.go`
- Modify: `cmd/root.go` (remove extracted functions)

- [ ] **Step 1: Create cmd/helpers.go with extracted functions**

Create `cmd/helpers.go` with `package cmd` header and the following functions cut from root.go:
- `enrichWithIssueDetails` (root.go:1615-1662)
- `isRetryIntent` (root.go:1569-1587)
- `hasFailedTadpole` (root.go:1589-1597)
- `truncate` (root.go:1599-1613)
- `stripCodeFences` (root.go:1550-1567)
- `findMatchingBrace` (root.go:1514-1548)
- `syncRepos` (root.go:1665-1681)
- `syncAll` (root.go:1683-1713)
- `buildVCSResolver` (root.go:1715-1733)

Include only the imports needed by these functions.

- [ ] **Step 2: Remove extracted functions from root.go**

Delete the same function blocks from root.go. Remove any imports that are now unused.

- [ ] **Step 3: Verify build and tests**

Run: `go build ./cmd/ && go test ./cmd/`
Expected: BUILD SUCCESS, ALL TESTS PASS

- [ ] **Step 4: Verify no formatting issues**

Run: `gofmt -l cmd/helpers.go cmd/root.go`
Expected: No output

### Task 1b: Extract investigation.go

**Files:**
- Create: `cmd/investigation.go`
- Modify: `cmd/root.go` (remove extracted functions and constants)

- [ ] **Step 1: Create cmd/investigation.go with extracted functions**

Create `cmd/investigation.go` with `package cmd` header and the following cut from root.go:
- `buildTaskDescription` (root.go:1151-1182)
- `investigatePrompt` constant (root.go:1184-1224)
- `formatTicketContext` (root.go:1226-1263)
- `investigateOpportunity` (root.go:1265-1327)
- `triggeredInvestigatePrompt` constant (root.go:1329-1370)
- `investigateTriggered` (root.go:1372-1429)
- `parseInvestigateResult` (root.go:1431-1512)
- `reasonMaxTurns` constant (root.go:1501)
- `resumeVerdictPrompt` constant (root.go:1503-1512)

Include only the imports needed by these functions.

- [ ] **Step 2: Remove extracted functions from root.go**

Delete the same function and constant blocks from root.go. Remove unused imports.

- [ ] **Step 3: Verify build and tests**

Run: `go build ./cmd/ && go test ./cmd/`
Expected: BUILD SUCCESS, ALL TESTS PASS

- [ ] **Step 4: Verify no formatting issues**

Run: `gofmt -l cmd/investigation.go cmd/root.go`
Expected: No output

### Task 1c: Extract handlers.go

**Files:**
- Create: `cmd/handlers.go`
- Modify: `cmd/root.go` (remove extracted functions)

- [ ] **Step 1: Create cmd/handlers.go with extracted functions**

Create `cmd/handlers.go` with `package cmd` header and the following cut from root.go:
- `handleMessage` (root.go:593-674)
- `handleTriggered` (root.go:675-989)
- `handlePassive` (root.go:990-1033)
- `handleTadpoleRequest` (root.go:1034-1149)

Include only the imports needed by these functions.

- [ ] **Step 2: Remove extracted functions from root.go**

Delete the same function blocks from root.go. Remove unused imports.

- [ ] **Step 3: Verify build and tests**

Run: `go build ./cmd/ && go test ./cmd/`
Expected: BUILD SUCCESS, ALL TESTS PASS

- [ ] **Step 4: Verify no formatting issues**

Run: `gofmt -l cmd/handlers.go cmd/root.go`
Expected: No output

- [ ] **Step 5: Commit all Task 1 changes**

Run:
```bash
gofmt -w cmd/root.go cmd/handlers.go cmd/investigation.go cmd/helpers.go
go vet ./cmd/
go test ./cmd/
git add cmd/root.go cmd/handlers.go cmd/investigation.go cmd/helpers.go
git commit -m "Split cmd/root.go into handlers, investigation, and helpers modules"
```

---

## Task 2: Split internal/digest/digest.go

### Task 2a: Extract guardrails.go

**Files:**
- Create: `internal/digest/guardrails.go`
- Modify: `internal/digest/digest.go` (remove extracted functions)

- [ ] **Step 1: Create guardrails.go with extracted functions**

Create `internal/digest/guardrails.go` with `package digest` header and:
- `passesGuardrails` (digest.go:1133-1181)
- `trySpawn` (digest.go:1183-end)

Include only the imports needed.

- [ ] **Step 2: Remove extracted functions from digest.go**

Delete the same function blocks. Remove unused imports.

- [ ] **Step 3: Verify build and tests**

Run: `go build ./internal/digest/ && go test ./internal/digest/`
Expected: BUILD SUCCESS, ALL TESTS PASS

### Task 2b: Extract chunking.go

**Files:**
- Create: `internal/digest/chunking.go`
- Modify: `internal/digest/digest.go` (remove extracted functions)

- [ ] **Step 1: Create chunking.go with extracted functions**

Create `internal/digest/chunking.go` with `package digest` header and:
- `dedupChannel` (digest.go:1033-1062)
- `buildChunks` (digest.go:1064-1131)

Include only the imports needed.

- [ ] **Step 2: Remove extracted functions from digest.go**

Delete the same function blocks. Remove unused imports.

- [ ] **Step 3: Verify build and tests**

Run: `go build ./internal/digest/ && go test ./internal/digest/`
Expected: BUILD SUCCESS, ALL TESTS PASS

### Task 2c: Extract analyze.go

**Files:**
- Create: `internal/digest/analyze.go`
- Modify: `internal/digest/digest.go` (remove extracted functions and constant)

- [ ] **Step 1: Create analyze.go with extracted functions**

Create `internal/digest/analyze.go` with `package digest` header and:
- `digestPrompt` constant (digest.go:813-857)
- `analyzeWithRetry` (digest.go:859-887)
- `analyze` (digest.go:889-936)
- `parseOpportunities` (digest.go:938-979)
- `stripDigestCodeFences` (digest.go:981-996)
- `findMatchingBracket` (digest.go:998-1031)

Include only the imports needed.

- [ ] **Step 2: Remove extracted functions from digest.go**

Delete the same function, constant, and helper blocks. Remove unused imports.

- [ ] **Step 3: Verify build and tests**

Run: `go build ./internal/digest/ && go test ./internal/digest/`
Expected: BUILD SUCCESS, ALL TESTS PASS

- [ ] **Step 4: Commit all Task 2 changes**

Run:
```bash
gofmt -w internal/digest/digest.go internal/digest/analyze.go internal/digest/chunking.go internal/digest/guardrails.go
go vet ./internal/digest/
go test ./internal/digest/
git add internal/digest/
git commit -m "Split internal/digest/digest.go into analyze, chunking, and guardrails modules"
```

---

## Task 3: Archive completed plan docs

**Files:**
- Modify: `docs/plans/2026-03-09-mcp-server-design.md`
- Modify: `docs/plans/2026-03-09-mcp-server-plan.md`
- Modify: `docs/plans/2026-03-10-button-cta-design.md`
- Modify: `docs/plans/2026-03-10-button-cta-plan.md`
- Modify: `docs/plans/2026-03-10-investigation-outreach-design.md`
- Modify: `docs/plans/2026-03-10-investigation-outreach-plan.md`
- Modify: `docs/plans/2026-03-10-personality-system-design.md`
- Modify: `docs/plans/2026-03-10-personality-system-plan.md`

- [ ] **Step 1: Add completion headers to all plan docs**

Prepend to each of the 8 files above:
```markdown
> **Status: COMPLETED** — This feature has been implemented and is running in production.

```

- [ ] **Step 2: Commit**

Run:
```bash
git add docs/plans/
git commit -m "Mark completed plan docs as COMPLETED"
```

---

## Final Verification

- [ ] **Full build and test suite**

Run:
```bash
go build ./...
go test ./...
go vet ./...
gofmt -l .
golangci-lint run ./...
```
Expected: ALL PASS, no formatting issues, no lint errors.

- [ ] **Line count verification**

Run:
```bash
wc -l cmd/root.go cmd/handlers.go cmd/investigation.go cmd/helpers.go
wc -l internal/digest/digest.go internal/digest/analyze.go internal/digest/chunking.go internal/digest/guardrails.go
```
Expected: root.go < 600 lines, digest.go < 600 lines. No file over 600 lines.
