> **Status: COMPLETED** — This feature has been implemented and is running in production.

# Toad Personality System — Design Spec

## Overview

A configurable personality system that controls Toad's behavior through 22 numeric traits (0.0–1.0). Traits are stored in a base personality file (YAML), adjusted over time through Slack feedback and outcome metrics, and visualized as a radar chart on the dashboard. Personality files are exportable and sharable between teams.

## Core Concepts

### Single Unified Personality

One personality per Toad instance (per Slack workspace). Not split by mode — the same traits affect ribbit, tadpole, digest, and investigation behavior. Each trait naturally manifests differently depending on context (e.g., "risk tolerance" means speculative answers in ribbit, bold refactors in tadpole).

### Traits Are Not Config Values

Traits are abstract behavioral tendencies on a 0.0–1.0 scale. A translation layer maps trait values to concrete prompt wording, config parameters, and behavioral thresholds. Example: `confidence_threshold: 0.80` doesn't mean `min_confidence: 0.80` — it means "quite cautious" which might translate to `min_confidence: 0.95` and conservative prompt language.

### Three-Layer Architecture

1. **Base personality** — YAML file, sharable, defines starting values for all 22 traits
2. **Learned adjustments** — accumulated deltas from feedback, stored separately for auditability
3. **Effective personality** — base + adjustments, clamped to 0.0–1.0, used at runtime

No decay — learned adjustments persist until explicitly reset or overwritten. Manual dashboard edits add to the learned layer (not the base).

### Export = Flatten

When exporting/sharing a personality, all layers are compounded into a single file. The recipient gets a new base with no learned adjustments. This means the internal layering is purely for auditability within an instance.

## Trait Catalog (22 traits, 5 categories)

### Investigation & Analysis

| Trait | Low (0.0) | High (1.0) | What It Changes |
|-------|-----------|------------|-----------------|
| **Thoroughness** | Surface-level scan | Deep multi-file trace | Max turns for investigation, prompt depth instructions, file search breadth |
| **Context Hunger** | Just the mentioned file | Full call chain + tests + history | How many related files Toad pulls in, whether it reads git blame, test coverage |
| **Confidence Threshold** | Acts on hunches | Needs strong evidence | Digest min_confidence, triage actionable threshold, spawn vs report decision |
| **Pattern Recognition** | Takes requests literally | Infers broader patterns | Whether Toad notices "this same bug exists in 3 other places" or just fixes the one |

### Action & Execution

| Trait | Low (0.0) | High (1.0) | What It Changes |
|-------|-----------|------------|-----------------|
| **Risk Tolerance** | Minimal safe fix | Bold refactors | max_files_changed, prompt framing, willingness to touch shared code |
| **Scope Appetite** | Only what's asked | "While I'm here" fixes | Whether Toad fixes adjacent issues, updates related tests, improves naming |
| **Test Affinity** | Skip unless asked | Always add/update tests | Prompt instructions about test writing, validation expectations |
| **Creativity** | Follow existing patterns | Novel approaches | Prompt wording about pattern conformity vs suggesting architectural improvements |
| **Retry Persistence** | Give up fast | Try many approaches | max_retries, whether to vary strategy between attempts |

### Quality & Standards

| Trait | Low (0.0) | High (1.0) | What It Changes |
|-------|-----------|------------|-----------------|
| **Strictness** | Ship with warnings | Zero tolerance | Validation pass/fail criteria, whether lint warnings block shipping |
| **Pattern Conformity** | Loose style match | Exact codebase style | Prompt emphasis on matching conventions, naming, error handling patterns |
| **Documentation Drive** | Code speaks for itself | Comment everything | Whether Toad adds comments, updates READMEs, writes docstrings |
| **Speed vs Polish** | Fast and good enough | Take time, get it right | Timeout durations, max_turns, iteration on PR description quality |

### Communication

| Trait | Low (0.0) | High (1.0) | What It Changes |
|-------|-----------|------------|-----------------|
| **Verbosity** | Terse one-liners | Detailed explanations | Response length constraints, PR description detail, Slack notification wordiness |
| **Explanation Depth** | Just the answer | Show reasoning + context | Whether Toad explains WHY, includes alternatives considered, links related code |
| **Notification Eagerness** | Only actionable results | Update on everything | Which events trigger Slack messages, progress updates vs final results only |
| **Defensiveness** | Just do what's asked | Push back on bad ideas | Whether Toad questions requests, suggests alternatives, flags issues with the approach |
| **Tone** | Purely technical | Friendly, casual | Persona instructions, humor/emojis, conversational vs report style |

### Autonomy & Initiative

| Trait | Low (0.0) | High (1.0) | What It Changes |
|-------|-----------|------------|-----------------|
| **Autonomy** | Wait for explicit asks | Proactively spawn work | Digest auto-spawn behavior, creating PRs from passive observations |
| **Collaboration** | Auto-merge, move fast | Always request review | Auto-merge setting, reviewer assignment, human sign-off gates |
| **Initiative** | Respond only | Suggest improvements | Whether Toad volunteers observations like "I noticed X could be improved" |
| **Scope Sensitivity** | Optimistic ("I can do that") | Conservative estimation | max_est_size filter, whether Toad takes on "large" tasks or flags for humans |

## Current Toad Personality (Default Base)

Derived from existing prompts, config defaults, and hardcoded behavior:

```yaml
# personality.yaml — "The Careful Craftsman"
version: 1
name: "default"
description: "Conservative, scope-disciplined, pattern-following. Toad's original personality."

traits:
  # Investigation & Analysis
  thoroughness: 0.70
  context_hunger: 0.50
  confidence_threshold: 0.80
  pattern_recognition: 0.30

  # Action & Execution
  risk_tolerance: 0.30
  scope_appetite: 0.20
  test_affinity: 0.40
  creativity: 0.20
  retry_persistence: 0.30

  # Quality & Standards
  strictness: 0.70
  pattern_conformity: 0.80
  documentation_drive: 0.20
  speed_vs_polish: 0.55

  # Communication
  verbosity: 0.35
  explanation_depth: 0.40
  notification_eagerness: 0.50
  defensiveness: 0.25
  tone: 0.60

  # Autonomy & Initiative
  autonomy: 0.30
  collaboration: 0.70
  initiative: 0.30
  scope_sensitivity: 0.75
```

Evidence for these values:
- **Thoroughness 0.70** — Investigation gets 25 max turns with resume. Ribbit gets 10. Prompt says "take as many turns as you need."
- **Confidence Threshold 0.80** — `min_confidence: 0.95` in config, digest prompt says "MOST batches should return []", "be extremely conservative."
- **Risk Tolerance 0.30** — `max_files_changed: 5`, prompt says "minimal, focused changes — only what's needed."
- **Scope Appetite 0.20** — "Do NOT touch unrelated code." "Do NOT add unnecessary comments to unchanged code."
- **Pattern Conformity 0.80** — "Read relevant files first to understand existing code style." "Follow existing code style."
- **Retry Persistence 0.30** — `max_retries: 1`. One attempt, no strategy variation.
- **Verbosity 0.35** — "Keep it short (3-5 lines for questions, up to 10 for bugs)." Under 2000 chars.
- **Tone 0.60** — "Be conversational." "Friendly code assistant." Uses emoji in Slack.
- **Autonomy 0.30** — Digest disabled by default. Auto-spawn off. Requires explicit @mention or reaction.
- **Collaboration 0.70** — Auto-merge opt-in (default false). Suggested reviewers auto-detected.

## Concurrency

The personality Manager follows the same pattern as `state.Manager`: a `sync.RWMutex` guards the in-memory cache. `Effective()` returns a value copy (not a pointer), so consumers hold a snapshot that won't race with writes. Write operations (feedback, manual adjust, import) acquire the write lock, update the in-memory cache, and write through to SQLite — identical to the write-through pattern in `state.Manager`.

## Slack Integration

### Reaction Routing

The existing `handleReaction` in `internal/slack/events.go` handles one emoji (the trigger emoji, default `frog`). The personality system adds a second category of reactions: personality feedback emojis on Toad's own messages.

**Routing logic:**

1. Is the reaction on a message authored by Toad? (Check `IsToadReply` via bot user ID match)
   - **Yes** → check if the emoji is a mapped personality emoji. If so, route to `personality.ProcessEmoji()`. If not a personality emoji, ignore (general reactions on Toad messages are not feedback unless they're 👍/👎, which route to the LLM-interpreted channel).
   - **No** → existing behavior: check if it's the trigger emoji, route to tadpole spawn flow.

2. Conflict resolution: if a personality emoji is also the trigger emoji (unlikely but possible with custom mappings), the trigger emoji takes precedence on non-Toad messages, and personality feedback takes precedence on Toad messages.

3. Thread replies to Toad messages that are not @mentions are candidates for LLM-interpreted feedback. A new helper `IsReplyToToad(channel, threadTS)` checks whether the thread parent was authored by Toad (distinct from the existing `IsToadReply` which checks authorship of a specific message). This enables routing reply text to the personality feedback system.

### Personality Emoji Configuration

The mapped emoji palette is defined in `personality.yaml` alongside the traits:

The following lives in `~/.toad/config.yaml` (not in `personality.yaml`):

```yaml
feedback_emojis:
  turtle: {traits: [thoroughness, speed_vs_polish], direction: [-0.03, 0.03]}
  rabbit: {traits: [thoroughness, context_hunger], direction: [0.03, 0.03]}
  mute: {traits: [verbosity, explanation_depth], direction: [-0.03, -0.03]}
  loudspeaker: {traits: [verbosity, explanation_depth], direction: [0.03, 0.03]}
  dart: {traits: [scope_appetite, risk_tolerance], direction: "reinforce"}  # see below
  ocean: {traits: [scope_appetite], direction: [-0.03]}
  test_tube: {traits: [test_affinity], direction: [0.03]}
  bulb: {traits: [creativity], direction: [0.03]}
```

Teams can customize this mapping. Emoji names use Slack's colon-free identifiers.

**Reinforcement mechanic:** when an emoji has `direction: "reinforce"`, it doesn't nudge the trait in either direction. Instead, it reduces the magnitude of the most recent negative adjustment on those traits by 50%. This acts as a "you got it right" signal that counteracts recent corrections. If there are no recent negative adjustments, reinforcement is a no-op (logged for audit but no trait change).

**Emoji config ownership:** feedback emoji mappings live in `~/.toad/config.yaml` (team config), NOT in `personality.yaml`. They are team workflow preferences, not part of the exportable personality. When importing a shared personality, your emoji mappings are unaffected.

## Feedback System

### Three Channels

#### 1. Mapped Emojis (Specific, Direct)

Predefined emoji → trait mappings for precise adjustments. Team learns a small vocabulary:

| Emoji | Meaning | Traits Affected | Direction |
|-------|---------|-----------------|-----------|
| 🐢 | Too slow/thorough | thoroughness, speed_vs_polish | decrease, increase speed |
| 🐇 | Too shallow | thoroughness, context_hunger | increase |
| 🔇 | Too verbose | verbosity, explanation_depth | decrease |
| 📢 | Too quiet/brief | verbosity, explanation_depth | increase |
| 🎯 | Scope was perfect | scope_appetite, risk_tolerance | reinforce current |
| 🌊 | Too much scope | scope_appetite | decrease |
| 🧪 | Needs more tests | test_affinity | increase |
| 💡 | Good creative solution | creativity | increase |

The emoji palette is configurable — teams can remap or add their own.

#### 2. LLM-Interpreted Feedback (Flexible, Inferred)

Thread replies and general reactions are parsed by a quick Haiku call to determine which traits to adjust:

- Input: the feedback text/reaction, Toad's output that was reacted to, current personality values
- Output: list of `{trait, delta, reasoning}`
- The Haiku reasoning is logged for auditability

General 👍/👎 provides a weak signal spread across traits that produced the output. Specific text ("dig deeper next time", "this was too aggressive") maps to specific traits.

**Error handling and rate limiting:**
- Fail-open: if the Haiku call fails (timeout, rate limit, error), log the failure and skip — no trait adjustment. Feedback is best-effort.
- Debounce: max one LLM-interpreted feedback call per Slack thread per 5 minutes. Additional feedback within the window is queued and batched into the next call.
- Timeout: 30 seconds (matches existing triage timeout).
- Cost control: only thread replies that are direct responses to Toad's messages trigger interpretation. General channel chatter is never sent to Haiku for interpretation.
- Debounce state: an in-memory `map[string]time.Time` (thread TS → last call time) on the personality Manager. Ephemeral — lost on restart, which is fine since debounce is a rate-limit mechanism, not state.

**Routing `ProcessText`:** the Slack `handleMessage` function checks `IsReplyToToad` (determined by matching the parent message's user ID against the bot user ID). If true and the message is not an @mention (which routes to triage), the message text is passed to `personality.ProcessText()`. This is a new code path in `handleMessage`, gated behind `personality.LearningEnabled()`.

#### 3. Outcome Metrics (Objective, Delayed)

| Signal | Strength | Traits Affected |
|--------|----------|-----------------|
| PR merged without changes | Strong positive | Reinforce: risk_tolerance, scope_appetite, test_affinity, strictness |
| PR closed without merge | Strong negative | Weaken: risk_tolerance, confidence_threshold, scope_sensitivity |
| PR merged after review rounds | Moderate | Increase: strictness, pattern_conformity, collaboration |
| Fast time-to-merge | Moderate positive | Reinforce current balance |
| Many review comments | Moderate | Increase: strictness, test_affinity, pattern_conformity |
| Ribbit thread continues (follow-ups) | Weak | Increase: thoroughness, explanation_depth |
| Digest opportunity dismissed | Moderate | Increase: confidence_threshold |
| Digest opportunity approved | Moderate positive | Reinforce: confidence_threshold, autonomy |

**PR Lifecycle Tracking:**

Outcome metrics require tracking PR state after creation. The existing `ShipCallback` in `tadpole/runner.go` fires after PR creation — extend this to register a deferred status check. A background goroutine polls PR status (via `gh pr view` or the VCS provider interface) at intervals (e.g., 1h, 4h, 24h) until the PR is merged, closed, or the check expires (7 days). When a terminal state is reached, the outcome signal is fed to `personality.ProcessOutcome()`.

This piggybacks on the existing `reviewer` polling loop which already watches tadpole PRs for review comments — extend it to also capture merge/close events and review round counts.

### Learning Mechanics

- Each feedback event nudges traits by a small delta (±0.01–0.05)
- Stronger signals (PR outcomes) get larger deltas than weak signals (general emoji)
- No decay — adjustments persist until explicitly changed
- Conservative adjustment: a single event never moves a trait more than 0.05
- All changes clamped to 0.0–1.0 range
- Multiple feedback events on the same output are processed independently
- **Dampening near extremes:** deltas are multiplied by `max(0.05, 1 - abs(current - 0.5) * 2)` as a trait approaches 0.0 or 1.0. This means a trait at 0.9 receives only 20% of the raw delta, and a trait at the absolute extreme (0.0 or 1.0) still receives 5% — preventing permanent lock-in. This avoids saturation from sustained positive/negative signals without requiring manual resets.
- **Learned adjustment cap:** the learned layer cannot push any trait more than ±0.35 from its base value. Beyond that, manual adjustment or a base file change is required. This prevents runaway drift while still allowing meaningful learning.

## Trait-to-Behavior Translation

The personality system exposes trait values through a single interface. Consumers (ribbit, tadpole, digest, triage) read effective trait values and translate them into prompt wording and config parameters.

### Translation Approach

Each trait defines a set of prompt fragments and config overrides at key breakpoints:

```
trait: thoroughness
  0.0-0.3: prompt += "Do a quick scan. Focus on the most obvious answer."
           max_turns = max(5, base_max_turns * 0.5)
  0.3-0.6: prompt += "Search the codebase to find the answer."
           max_turns = base_max_turns
  0.6-0.8: prompt += "Search thoroughly. Check related files and trace call chains."
           max_turns = base_max_turns * 1.2
  0.8-1.0: prompt += "Do an exhaustive investigation. Trace every call chain, check tests, read git history."
           max_turns = base_max_turns * 1.5
```

Prompt fragments are appended to the base prompt. Config overrides are applied as multipliers on the defaults. This keeps the personality system isolated — it doesn't rewrite prompts, it augments them.

### Integration Points

- **Ribbit** (`internal/ribbit/`) — reads: thoroughness, context_hunger, verbosity, explanation_depth, tone, defensiveness
- **Tadpole** (`internal/tadpole/`) — reads: risk_tolerance, scope_appetite, test_affinity, creativity, retry_persistence, strictness, pattern_conformity, documentation_drive, speed_vs_polish
- **Digest** (`internal/digest/`) — reads: confidence_threshold, autonomy, scope_sensitivity, pattern_recognition, notification_eagerness
- **Triage** (`internal/triage/`) — reads: confidence_threshold, scope_sensitivity
- **All modes** — reads: collaboration, initiative, tone

## Storage

### Personality File

Location: `~/.toad/personality.yaml` (base) alongside the existing config.

### Learned Adjustments

Stored in SQLite (`~/.toad/state.db`) in a `personality_adjustments` table:

```sql
CREATE TABLE personality_adjustments (
    id INTEGER PRIMARY KEY,
    trait TEXT NOT NULL,
    delta REAL NOT NULL,
    source TEXT NOT NULL,        -- "emoji", "llm_interpreted", "outcome", "manual"
    trigger_detail TEXT,         -- e.g. "🐢 on ribbit reply", "PR #42 merged", "dashboard edit"
    reasoning TEXT,              -- Haiku reasoning for LLM-interpreted, human note for manual
    before_value REAL NOT NULL,
    after_value REAL NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

The effective personality is computed at startup (base + sum of deltas per trait, clamped) and cached in memory. Each new adjustment updates the cache.

The `personality_adjustments` table is created in the existing `migrate()` function in `internal/state/db.go` using `CREATE TABLE IF NOT EXISTS`, following the established pattern for schema additions.

### Audit Log

Every trait modification is a row in `personality_adjustments`. This provides a full history for analysis:
- When did Toad get more cautious?
- Which feedback events caused the biggest shifts?
- Is a particular user's feedback dominating the personality?

Queryable through the dashboard and exportable for analysis.

## Dashboard

### Radar Chart Page (existing dashboard)

Add the radar chart to the existing dashboard (`cmd/web/dashboard.html`). Shows the effective personality as an interactive radar chart rendered with a JS charting library (Chart.js radar type). Trait values displayed on hover.

### Personality Management Page (new)

A dedicated `/personality` page on the dashboard with:

1. **Radar chart** — effective personality visualization
2. **Trait sliders** — manual adjustment of each trait, grouped by category. Changes go to the learned layer.
3. **Recent adjustments feed** — last N personality changes with source, reasoning, before/after
4. **Base file info** — name, description, when loaded, option to export current effective personality as a new base
5. **Import** — load a shared personality file as new base (resets learned adjustments)

## Package Design

New package: `internal/personality/`

```
internal/personality/
    personality.go      -- Personality type, Load(), EffectiveValues(), trait definitions
    feedback.go         -- ProcessFeedback(), emoji mapping, outcome signal handling
    interpreter.go      -- LLM-interpreted feedback (Haiku calls)
    translator.go       -- Trait-to-prompt/config translation
    store.go            -- SQLite persistence for adjustments
```

The package exposes a clean interface:

```go
// Traits holds all 22 personality trait values. A struct (not a map) for compile-time safety.
type Traits struct {
	// Investigation & Analysis
	Thoroughness         float64
	ContextHunger        float64
	ConfidenceThreshold  float64
	PatternRecognition   float64

	// Action & Execution
	RiskTolerance        float64
	ScopeAppetite        float64
	TestAffinity         float64
	Creativity           float64
	RetryPersistence     float64

	// Quality & Standards
	Strictness           float64
	PatternConformity    float64
	DocumentationDrive   float64
	SpeedVsPolish        float64

	// Communication
	Verbosity            float64
	ExplanationDepth     float64
	NotificationEagerness float64
	Defensiveness        float64
	Tone                 float64

	// Autonomy & Initiative
	Autonomy             float64
	Collaboration        float64
	Initiative           float64
	ScopeSensitivity     float64
}

// Mode represents a Toad operational mode for trait translation.
type Mode string

const (
	ModeRibbit  Mode = "ribbit"
	ModeTadpole Mode = "tadpole"
	ModeDigest  Mode = "digest"
	ModeTriage  Mode = "triage"
)

// Overrides contains config parameter adjustments derived from personality traits.
// Pointer fields are optional — nil means "use the default from config."
type Overrides struct {
	MaxTurns        *int
	MaxRetries      *int
	MaxFilesChanged *int
	TimeoutMinutes  *int
	MinConfidence   *float64
	MaxEstSize      *string
}

// OutcomeSignal represents an objective outcome event.
type OutcomeSignal struct {
	Type         string  // "pr_merged", "pr_closed", "pr_review_rounds", "ribbit_followup", "digest_dismissed", "digest_approved"
	PRURL        string  // for PR signals
	ReviewRounds int     // for pr_review_rounds
	Metadata     map[string]string
}

// Adjustment represents a single trait modification for the audit log.
type Adjustment struct {
	ID            int64
	Trait         string
	Delta         float64
	Source        string    // "emoji", "llm_interpreted", "outcome", "manual"
	TriggerDetail string
	Reasoning     string
	BeforeValue   float64
	AfterValue    float64
	CreatedAt     time.Time
}

type Manager struct { ... }

// Reading — Effective() returns a value copy, safe for concurrent use.
func (m *Manager) Effective() Traits
func (m *Manager) Base() Traits

// Feedback
func (m *Manager) ProcessEmoji(emoji, context string) error
func (m *Manager) ProcessText(text, context string) error
func (m *Manager) ProcessOutcome(signal OutcomeSignal) error
func (m *Manager) ManualAdjust(trait string, value float64, note string) error

// Export/Import
func (m *Manager) Export() ([]byte, error)     // flatten to YAML
func (m *Manager) Import(data []byte) error    // load as new base, reset adjustments

// Translation
func (m *Manager) PromptFragments(mode Mode) []string  // prompt additions for a mode
func (m *Manager) ConfigOverrides(mode Mode) Overrides  // config parameter adjustments

// Learning control — gated by config flag `personality.learning_enabled` (default: true when personality is configured)
func (m *Manager) LearningEnabled() bool

// Audit
func (m *Manager) RecentAdjustments(limit int) []Adjustment

// Construction — mirrors state.Manager pattern
func NewManager(base Traits) *Manager                    // in-memory only (tests)
func NewPersistentManager(db *state.DB, base Traits) (*Manager, error)  // hydrates from DB; no historySize — adjustments are an unbounded append-only audit log
```

Consumers use `PromptFragments()` and `ConfigOverrides()` — they never read raw trait values and interpret them ad-hoc. This keeps the translation logic centralized.

## Scope & Non-Goals

- **Multi-repo personality:** one personality per Toad instance, not per repo. Different repos do not get different trait values. This is a deliberate simplification — if needed later, a per-repo override layer could be added.
- **Personality versioning:** the `version: 1` field in the YAML enables future migration. When traits are added or renamed, a migration function maps v1 → v2 files (filling new traits with defaults, renaming keys). Removed traits are silently dropped on load.
- **Import behavior:** importing a shared personality resets learned adjustments. The dashboard shows a confirmation warning before import. The previous personality (base + adjustments) is auto-exported as a backup before overwrite.

## Testing

Follows the existing codebase pattern: `NewManager(base)` creates an in-memory-only manager for tests (no SQLite dependency). `NewPersistentManager(base, db)` hydrates from the database for production. All feedback, translation, and dampening logic is testable against the in-memory manager.
