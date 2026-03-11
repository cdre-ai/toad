# Personality System Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a 22-trait personality system that controls Toad's behavior, tunable via Slack feedback and a dashboard, with shareable personality files.

**Architecture:** New `internal/personality/` package with Manager (mirrors `state.Manager` pattern: `sync.RWMutex`, write-through to SQLite). Base traits loaded from `~/.toad/personality.yaml`, learned adjustments stored in `personality_adjustments` DB table. Translation layer maps trait values to prompt fragments and config overrides consumed by ribbit, tadpole, digest, and triage.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), Chart.js (dashboard radar chart), existing Slack Socket Mode client.

**Spec:** `docs/plans/2026-03-10-personality-system-design.md`

**Deferred to follow-up plan:**
- LLM-interpreted feedback (`interpreter.go`) — the Haiku-based text feedback parsing, debounce, and rate limiting. `ProcessText` is stubbed as a no-op logger in this plan. The full implementation requires its own design for prompt construction, cost budgeting, and response parsing.
- Configurable emoji mappings (`feedback_emojis` in config) — emoji-to-trait mappings are hardcoded in `defaultEmojiMappings()` for now. Making them user-configurable via `~/.toad/config.yaml` is deferred.

---

## Chunk 1: Core Types, Loading & Storage

### Task 1: Personality Types and Default Traits

**Files:**
- Create: `internal/personality/personality.go`
- Test: `internal/personality/personality_test.go`

- [ ] **Step 1: Write failing test for Traits struct and DefaultTraits()**

```go
// internal/personality/personality_test.go
package personality

import "testing"

func TestDefaultTraits(t *testing.T) {
	d := DefaultTraits()
	if d.Thoroughness != 0.70 {
		t.Errorf("Thoroughness = %v, want 0.70", d.Thoroughness)
	}
	if d.ConfidenceThreshold != 0.80 {
		t.Errorf("ConfidenceThreshold = %v, want 0.80", d.ConfidenceThreshold)
	}
	if d.ScopeAppetite != 0.20 {
		t.Errorf("ScopeAppetite = %v, want 0.20", d.ScopeAppetite)
	}
	if d.PatternConformity != 0.80 {
		t.Errorf("PatternConformity = %v, want 0.80", d.PatternConformity)
	}
}

func TestTraitsClamp(t *testing.T) {
	tr := Traits{Thoroughness: 1.5, RiskTolerance: -0.3}
	clamped := tr.Clamp()
	if clamped.Thoroughness != 1.0 {
		t.Errorf("Thoroughness = %v, want 1.0", clamped.Thoroughness)
	}
	if clamped.RiskTolerance != 0.0 {
		t.Errorf("RiskTolerance = %v, want 0.0", clamped.RiskTolerance)
	}
}

func TestTraitsAdd(t *testing.T) {
	base := DefaultTraits()
	delta := Traits{Thoroughness: 0.1, RiskTolerance: -0.1}
	result := base.Add(delta)
	if result.Thoroughness != 0.80 {
		t.Errorf("Thoroughness = %v, want 0.80", result.Thoroughness)
	}
	if result.RiskTolerance != 0.20 {
		t.Errorf("RiskTolerance = %v, want 0.20", result.RiskTolerance)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/personality/ -run TestDefault -v`
Expected: FAIL — package does not exist

- [ ] **Step 3: Implement Traits struct, DefaultTraits(), Clamp(), Add()**

```go
// internal/personality/personality.go
package personality

// Traits holds all 22 personality trait values (0.0–1.0).
type Traits struct {
	// Investigation & Analysis
	Thoroughness        float64 `yaml:"thoroughness" json:"thoroughness"`
	ContextHunger       float64 `yaml:"context_hunger" json:"context_hunger"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold" json:"confidence_threshold"`
	PatternRecognition  float64 `yaml:"pattern_recognition" json:"pattern_recognition"`

	// Action & Execution
	RiskTolerance    float64 `yaml:"risk_tolerance" json:"risk_tolerance"`
	ScopeAppetite    float64 `yaml:"scope_appetite" json:"scope_appetite"`
	TestAffinity     float64 `yaml:"test_affinity" json:"test_affinity"`
	Creativity       float64 `yaml:"creativity" json:"creativity"`
	RetryPersistence float64 `yaml:"retry_persistence" json:"retry_persistence"`

	// Quality & Standards
	Strictness         float64 `yaml:"strictness" json:"strictness"`
	PatternConformity  float64 `yaml:"pattern_conformity" json:"pattern_conformity"`
	DocumentationDrive float64 `yaml:"documentation_drive" json:"documentation_drive"`
	SpeedVsPolish      float64 `yaml:"speed_vs_polish" json:"speed_vs_polish"`

	// Communication
	Verbosity             float64 `yaml:"verbosity" json:"verbosity"`
	ExplanationDepth      float64 `yaml:"explanation_depth" json:"explanation_depth"`
	NotificationEagerness float64 `yaml:"notification_eagerness" json:"notification_eagerness"`
	Defensiveness         float64 `yaml:"defensiveness" json:"defensiveness"`
	Tone                  float64 `yaml:"tone" json:"tone"`

	// Autonomy & Initiative
	Autonomy         float64 `yaml:"autonomy" json:"autonomy"`
	Collaboration    float64 `yaml:"collaboration" json:"collaboration"`
	Initiative       float64 `yaml:"initiative" json:"initiative"`
	ScopeSensitivity float64 `yaml:"scope_sensitivity" json:"scope_sensitivity"`
}

// DefaultTraits returns the "Careful Craftsman" base personality
// derived from Toad's current hardcoded behavior.
func DefaultTraits() Traits {
	return Traits{
		Thoroughness:          0.70,
		ContextHunger:         0.50,
		ConfidenceThreshold:   0.80,
		PatternRecognition:    0.30,
		RiskTolerance:         0.30,
		ScopeAppetite:         0.20,
		TestAffinity:          0.40,
		Creativity:            0.20,
		RetryPersistence:      0.30,
		Strictness:            0.70,
		PatternConformity:     0.80,
		DocumentationDrive:    0.20,
		SpeedVsPolish:         0.55,
		Verbosity:             0.35,
		ExplanationDepth:      0.40,
		NotificationEagerness: 0.50,
		Defensiveness:         0.25,
		Tone:                  0.60,
		Autonomy:              0.30,
		Collaboration:         0.70,
		Initiative:            0.30,
		ScopeSensitivity:      0.75,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// Clamp returns a copy with all values clamped to [0.0, 1.0].
func (t Traits) Clamp() Traits {
	return Traits{
		Thoroughness:          clamp01(t.Thoroughness),
		ContextHunger:         clamp01(t.ContextHunger),
		ConfidenceThreshold:   clamp01(t.ConfidenceThreshold),
		PatternRecognition:    clamp01(t.PatternRecognition),
		RiskTolerance:         clamp01(t.RiskTolerance),
		ScopeAppetite:         clamp01(t.ScopeAppetite),
		TestAffinity:          clamp01(t.TestAffinity),
		Creativity:            clamp01(t.Creativity),
		RetryPersistence:      clamp01(t.RetryPersistence),
		Strictness:            clamp01(t.Strictness),
		PatternConformity:     clamp01(t.PatternConformity),
		DocumentationDrive:    clamp01(t.DocumentationDrive),
		SpeedVsPolish:         clamp01(t.SpeedVsPolish),
		Verbosity:             clamp01(t.Verbosity),
		ExplanationDepth:      clamp01(t.ExplanationDepth),
		NotificationEagerness: clamp01(t.NotificationEagerness),
		Defensiveness:         clamp01(t.Defensiveness),
		Tone:                  clamp01(t.Tone),
		Autonomy:              clamp01(t.Autonomy),
		Collaboration:         clamp01(t.Collaboration),
		Initiative:            clamp01(t.Initiative),
		ScopeSensitivity:      clamp01(t.ScopeSensitivity),
	}
}

// Add returns a new Traits with each field summed.
func (t Traits) Add(other Traits) Traits {
	return Traits{
		Thoroughness:          t.Thoroughness + other.Thoroughness,
		ContextHunger:         t.ContextHunger + other.ContextHunger,
		ConfidenceThreshold:   t.ConfidenceThreshold + other.ConfidenceThreshold,
		PatternRecognition:    t.PatternRecognition + other.PatternRecognition,
		RiskTolerance:         t.RiskTolerance + other.RiskTolerance,
		ScopeAppetite:         t.ScopeAppetite + other.ScopeAppetite,
		TestAffinity:          t.TestAffinity + other.TestAffinity,
		Creativity:            t.Creativity + other.Creativity,
		RetryPersistence:      t.RetryPersistence + other.RetryPersistence,
		Strictness:            t.Strictness + other.Strictness,
		PatternConformity:     t.PatternConformity + other.PatternConformity,
		DocumentationDrive:    t.DocumentationDrive + other.DocumentationDrive,
		SpeedVsPolish:         t.SpeedVsPolish + other.SpeedVsPolish,
		Verbosity:             t.Verbosity + other.Verbosity,
		ExplanationDepth:      t.ExplanationDepth + other.ExplanationDepth,
		NotificationEagerness: t.NotificationEagerness + other.NotificationEagerness,
		Defensiveness:         t.Defensiveness + other.Defensiveness,
		Tone:                  t.Tone + other.Tone,
		Autonomy:              t.Autonomy + other.Autonomy,
		Collaboration:         t.Collaboration + other.Collaboration,
		Initiative:            t.Initiative + other.Initiative,
		ScopeSensitivity:      t.ScopeSensitivity + other.ScopeSensitivity,
	}
}

// TraitNames returns the list of all trait names in canonical order.
func TraitNames() []string {
	return []string{
		"thoroughness", "context_hunger", "confidence_threshold", "pattern_recognition",
		"risk_tolerance", "scope_appetite", "test_affinity", "creativity", "retry_persistence",
		"strictness", "pattern_conformity", "documentation_drive", "speed_vs_polish",
		"verbosity", "explanation_depth", "notification_eagerness", "defensiveness", "tone",
		"autonomy", "collaboration", "initiative", "scope_sensitivity",
	}
}

// Get returns a trait value by name. Returns 0 and false for unknown names.
func (t Traits) Get(name string) (float64, bool) {
	switch name {
	case "thoroughness":
		return t.Thoroughness, true
	case "context_hunger":
		return t.ContextHunger, true
	case "confidence_threshold":
		return t.ConfidenceThreshold, true
	case "pattern_recognition":
		return t.PatternRecognition, true
	case "risk_tolerance":
		return t.RiskTolerance, true
	case "scope_appetite":
		return t.ScopeAppetite, true
	case "test_affinity":
		return t.TestAffinity, true
	case "creativity":
		return t.Creativity, true
	case "retry_persistence":
		return t.RetryPersistence, true
	case "strictness":
		return t.Strictness, true
	case "pattern_conformity":
		return t.PatternConformity, true
	case "documentation_drive":
		return t.DocumentationDrive, true
	case "speed_vs_polish":
		return t.SpeedVsPolish, true
	case "verbosity":
		return t.Verbosity, true
	case "explanation_depth":
		return t.ExplanationDepth, true
	case "notification_eagerness":
		return t.NotificationEagerness, true
	case "defensiveness":
		return t.Defensiveness, true
	case "tone":
		return t.Tone, true
	case "autonomy":
		return t.Autonomy, true
	case "collaboration":
		return t.Collaboration, true
	case "initiative":
		return t.Initiative, true
	case "scope_sensitivity":
		return t.ScopeSensitivity, true
	default:
		return 0, false
	}
}

// Set sets a trait by name. Returns false for unknown names.
func (t *Traits) Set(name string, value float64) bool {
	switch name {
	case "thoroughness":
		t.Thoroughness = value
	case "context_hunger":
		t.ContextHunger = value
	case "confidence_threshold":
		t.ConfidenceThreshold = value
	case "pattern_recognition":
		t.PatternRecognition = value
	case "risk_tolerance":
		t.RiskTolerance = value
	case "scope_appetite":
		t.ScopeAppetite = value
	case "test_affinity":
		t.TestAffinity = value
	case "creativity":
		t.Creativity = value
	case "retry_persistence":
		t.RetryPersistence = value
	case "strictness":
		t.Strictness = value
	case "pattern_conformity":
		t.PatternConformity = value
	case "documentation_drive":
		t.DocumentationDrive = value
	case "speed_vs_polish":
		t.SpeedVsPolish = value
	case "verbosity":
		t.Verbosity = value
	case "explanation_depth":
		t.ExplanationDepth = value
	case "notification_eagerness":
		t.NotificationEagerness = value
	case "defensiveness":
		t.Defensiveness = value
	case "tone":
		t.Tone = value
	case "autonomy":
		t.Autonomy = value
	case "collaboration":
		t.Collaboration = value
	case "initiative":
		t.Initiative = value
	case "scope_sensitivity":
		t.ScopeSensitivity = value
	default:
		return false
	}
	return true
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/personality/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(personality): add Traits type with defaults, clamp, and add operations
```

### Task 2: YAML Personality File Loading

**Files:**
- Modify: `internal/personality/personality.go`
- Test: `internal/personality/personality_test.go`

- [ ] **Step 1: Write failing test for LoadFile and Export**

```go
func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "personality.yaml")
	content := []byte(`version: 1
name: "test"
description: "Test personality"
traits:
  thoroughness: 0.90
  context_hunger: 0.50
  confidence_threshold: 0.80
  pattern_recognition: 0.30
  risk_tolerance: 0.60
  scope_appetite: 0.20
  test_affinity: 0.40
  creativity: 0.20
  retry_persistence: 0.30
  strictness: 0.70
  pattern_conformity: 0.80
  documentation_drive: 0.20
  speed_vs_polish: 0.55
  verbosity: 0.35
  explanation_depth: 0.40
  notification_eagerness: 0.50
  defensiveness: 0.25
  tone: 0.60
  autonomy: 0.30
  collaboration: 0.70
  initiative: 0.30
  scope_sensitivity: 0.75
`)
	os.WriteFile(path, content, 0o644)

	pf, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if pf.Name != "test" {
		t.Errorf("Name = %q, want %q", pf.Name, "test")
	}
	if pf.Traits.Thoroughness != 0.90 {
		t.Errorf("Thoroughness = %v, want 0.90", pf.Traits.Thoroughness)
	}
}

func TestLoadFileMissing(t *testing.T) {
	pf, err := LoadFile("/nonexistent/path.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if pf.Name != "default" {
		t.Errorf("Name = %q, want %q", pf.Name, "default")
	}
	if pf.Traits != DefaultTraits() {
		t.Error("missing file should return default traits")
	}
}

func TestExport(t *testing.T) {
	pf := &PersonalityFile{
		Version:     1,
		Name:        "exported",
		Description: "Test export",
		Traits:      DefaultTraits(),
	}
	data, err := pf.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "name: exported") {
		t.Errorf("export should contain name, got:\n%s", data)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/personality/ -run TestLoadFile -v`
Expected: FAIL — `LoadFile` undefined

- [ ] **Step 3: Implement PersonalityFile, LoadFile, Marshal**

```go
// Add to personality.go

// PersonalityFile is the on-disk YAML format for a personality.
type PersonalityFile struct {
	Version     int    `yaml:"version"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Traits      Traits `yaml:"traits"`
}

// LoadFile reads a personality YAML file. Returns defaults if the file does not exist.
func LoadFile(path string) (*PersonalityFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &PersonalityFile{
				Version:     1,
				Name:        "default",
				Description: "Conservative, scope-disciplined, pattern-following. Toad's original personality.",
				Traits:      DefaultTraits(),
			}, nil
		}
		return nil, fmt.Errorf("reading personality file: %w", err)
	}
	var pf PersonalityFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return nil, fmt.Errorf("parsing personality file: %w", err)
	}
	return &pf, nil
}

// Marshal serializes the personality file to YAML.
func (pf *PersonalityFile) Marshal() ([]byte, error) {
	return yaml.Marshal(pf)
}
```

Add imports: `"fmt"`, `"os"`, `"gopkg.in/yaml.v3"`. The manager.go file also needs `"log/slog"` and `"math"`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/personality/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(personality): add YAML personality file loading and export
```

### Task 3: DB Schema and ExecContext/QueryContext Exposure

**Files:**
- Modify: `internal/state/db.go`

- [ ] **Step 1: Expose ExecContext and QueryContext on state.DB**

The personality store needs generic SQL access. Add thin wrappers to `internal/state/db.go`:

```go
// ExecContext exposes the underlying DB ExecContext for other packages.
func (d *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return d.db.ExecContext(ctx, query, args...)
}

// QueryContext exposes the underlying DB QueryContext for other packages.
func (d *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return d.db.QueryContext(ctx, query, args...)
}
```

- [ ] **Step 2: Add personality_adjustments CREATE TABLE to migrate()**

In `internal/state/db.go`, add after the `github_slack_mappings` CREATE TABLE block (after line 169):

```go
_, err = db.Exec(`CREATE TABLE IF NOT EXISTS personality_adjustments (
	id INTEGER PRIMARY KEY,
	trait TEXT NOT NULL,
	delta REAL NOT NULL,
	source TEXT NOT NULL,
	trigger_detail TEXT,
	reasoning TEXT,
	before_value REAL NOT NULL,
	after_value REAL NOT NULL,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
)`)
if err != nil {
	return fmt.Errorf("creating personality_adjustments table: %w", err)
}
```

- [ ] **Step 3: Run existing state tests to verify no regression**

Run: `go test ./internal/state/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```
feat(personality): add personality_adjustments table and expose ExecContext/QueryContext
```

### Task 4: Personality Store (SQLite persistence)

**Files:**
- Create: `internal/personality/store.go`
- Test: `internal/personality/store_test.go`

- [ ] **Step 1: Write failing test for store operations**

```go
// internal/personality/store_test.go
package personality

import (
	"testing"
	"time"

	"github.com/scaler-tech/toad/internal/state"
)

func testDB(t *testing.T) *state.DB {
	t.Helper()
	db, err := state.OpenDBAt(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestStoreInsertAndSum(t *testing.T) {
	db := testDB(t)
	s := NewStore(db)

	err := s.Insert(Adjustment{
		Trait:       "thoroughness",
		Delta:       0.05,
		Source:      "emoji",
		BeforeValue: 0.70,
		AfterValue:  0.75,
	})
	if err != nil {
		t.Fatal(err)
	}

	err = s.Insert(Adjustment{
		Trait:       "thoroughness",
		Delta:       -0.03,
		Source:      "outcome",
		BeforeValue: 0.75,
		AfterValue:  0.72,
	})
	if err != nil {
		t.Fatal(err)
	}

	deltas, err := s.SumDeltas()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := deltas["thoroughness"]
	if !ok || got != 0.02 {
		t.Errorf("thoroughness delta = %v, want 0.02", got)
	}
}

func TestStoreRecent(t *testing.T) {
	db := testDB(t)
	s := NewStore(db)

	s.Insert(Adjustment{Trait: "tone", Delta: 0.01, Source: "emoji", BeforeValue: 0.6, AfterValue: 0.61})
	s.Insert(Adjustment{Trait: "verbosity", Delta: -0.02, Source: "manual", BeforeValue: 0.35, AfterValue: 0.33})

	recent, err := s.Recent(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 2 {
		t.Fatalf("got %d adjustments, want 2", len(recent))
	}
	// Most recent first
	if recent[0].Trait != "verbosity" {
		t.Errorf("first adjustment trait = %q, want %q", recent[0].Trait, "verbosity")
	}
}

func TestStoreClearAll(t *testing.T) {
	db := testDB(t)
	s := NewStore(db)

	s.Insert(Adjustment{Trait: "tone", Delta: 0.01, Source: "emoji", BeforeValue: 0.6, AfterValue: 0.61})

	if err := s.ClearAll(); err != nil {
		t.Fatal(err)
	}

	deltas, err := s.SumDeltas()
	if err != nil {
		t.Fatal(err)
	}
	if len(deltas) != 0 {
		t.Errorf("expected empty deltas after clear, got %v", deltas)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/personality/ -run TestStore -v`
Expected: FAIL — `NewStore` undefined

- [ ] **Step 3: Implement Store**

```go
// internal/personality/store.go
package personality

import (
	"context"
	"time"

	"github.com/scaler-tech/toad/internal/state"
)

const storeTimeout = 10 * time.Second

// Adjustment represents a single trait modification.
type Adjustment struct {
	ID            int64
	Trait         string
	Delta         float64
	Source        string // "emoji", "llm_interpreted", "outcome", "manual"
	TriggerDetail string
	Reasoning     string
	BeforeValue   float64
	AfterValue    float64
	CreatedAt     time.Time
}

// Store handles SQLite persistence for personality adjustments.
type Store struct {
	db *state.DB
}

// NewStore creates a personality adjustment store.
func NewStore(db *state.DB) *Store {
	return &Store{db: db}
}

// Insert records a personality adjustment.
func (s *Store) Insert(adj Adjustment) error {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO personality_adjustments (trait, delta, source, trigger_detail, reasoning, before_value, after_value, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		adj.Trait, adj.Delta, adj.Source, adj.TriggerDetail, adj.Reasoning,
		adj.BeforeValue, adj.AfterValue, time.Now(),
	)
	return err
}

// SumDeltas returns the accumulated delta for each trait.
func (s *Store) SumDeltas() (map[string]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	rows, err := s.db.QueryContext(ctx,
		"SELECT trait, SUM(delta) FROM personality_adjustments GROUP BY trait")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deltas := make(map[string]float64)
	for rows.Next() {
		var trait string
		var sum float64
		if err := rows.Scan(&trait, &sum); err != nil {
			return nil, err
		}
		deltas[trait] = sum
	}
	return deltas, rows.Err()
}

// Recent returns the N most recent adjustments, newest first.
func (s *Store) Recent(limit int) ([]Adjustment, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, trait, delta, source, COALESCE(trigger_detail,''), COALESCE(reasoning,''), before_value, after_value, created_at FROM personality_adjustments ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var adjs []Adjustment
	for rows.Next() {
		var a Adjustment
		if err := rows.Scan(&a.ID, &a.Trait, &a.Delta, &a.Source, &a.TriggerDetail, &a.Reasoning, &a.BeforeValue, &a.AfterValue, &a.CreatedAt); err != nil {
			return nil, err
		}
		adjs = append(adjs, a)
	}
	return adjs, rows.Err()
}

// ClearAll removes all personality adjustments (used on import/reset).
func (s *Store) ClearAll() error {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	_, err := s.db.ExecContext(ctx, "DELETE FROM personality_adjustments")
	return err
}
```

Note: `state.DB.ExecContext` and `QueryContext` were exposed in Task 3.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/personality/ -run TestStore -v`
Expected: PASS

- [ ] **Step 6: Run all state tests to verify no regression**

Run: `go test ./internal/state/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```
feat(personality): add SQLite store for personality adjustments
```

### Task 5: Manager — In-Memory and Persistent

**Files:**
- Create: `internal/personality/manager.go`
- Test: `internal/personality/manager_test.go`

- [ ] **Step 1: Write failing test for Manager**

```go
// internal/personality/manager_test.go
package personality

import "testing"

func TestNewManager(t *testing.T) {
	m := NewManager(DefaultTraits())
	eff := m.Effective()
	if eff.Thoroughness != 0.70 {
		t.Errorf("Thoroughness = %v, want 0.70", eff.Thoroughness)
	}
}

func TestManagerAdjust(t *testing.T) {
	m := NewManager(DefaultTraits())

	err := m.applyAdjustment("thoroughness", 0.05, "emoji", "🐇 on ribbit", "")
	if err != nil {
		t.Fatal(err)
	}

	eff := m.Effective()
	if eff.Thoroughness != 0.75 {
		t.Errorf("Thoroughness = %v, want 0.75", eff.Thoroughness)
	}

	base := m.Base()
	if base.Thoroughness != 0.70 {
		t.Errorf("Base Thoroughness = %v, want 0.70 (should not change)", base.Thoroughness)
	}
}

func TestManagerDampening(t *testing.T) {
	base := DefaultTraits()
	base.Thoroughness = 0.95 // near extreme
	m := NewManager(base)

	// Delta should be dampened: raw 0.05 * max(0.05, 1 - |0.95-0.5|*2) = 0.05 * 0.1 = 0.005
	err := m.applyAdjustment("thoroughness", 0.05, "emoji", "test", "")
	if err != nil {
		t.Fatal(err)
	}

	eff := m.Effective()
	// 0.95 + dampened delta (should be much less than 0.05)
	if eff.Thoroughness > 0.96 {
		t.Errorf("Thoroughness = %v, expected dampening near extreme", eff.Thoroughness)
	}
}

func TestManagerLearnedCap(t *testing.T) {
	m := NewManager(DefaultTraits())

	// Apply many adjustments to hit the ±0.35 cap
	for i := 0; i < 100; i++ {
		m.applyAdjustment("risk_tolerance", 0.05, "outcome", "test", "")
	}

	eff := m.Effective()
	base := m.Base()
	diff := eff.RiskTolerance - base.RiskTolerance
	if diff > 0.36 {
		t.Errorf("learned adjustment = %v, should not exceed 0.35 cap", diff)
	}
}

func TestManagerManualAdjust(t *testing.T) {
	m := NewManager(DefaultTraits())

	err := m.ManualAdjust("verbosity", 0.80, "user prefers verbose")
	if err != nil {
		t.Fatal(err)
	}

	eff := m.Effective()
	if eff.Verbosity != 0.80 {
		t.Errorf("Verbosity = %v, want 0.80", eff.Verbosity)
	}
}

func TestManagerPersistent(t *testing.T) {
	db := testDB(t)
	base := DefaultTraits()

	// Create manager, apply adjustment
	m, err := NewPersistentManager(db, base)
	if err != nil {
		t.Fatal(err)
	}
	m.applyAdjustment("tone", 0.05, "emoji", "test", "")

	// Create new manager from same DB — should hydrate
	m2, err := NewPersistentManager(db, base)
	if err != nil {
		t.Fatal(err)
	}
	eff := m2.Effective()
	if eff.Tone != 0.65 {
		t.Errorf("Tone = %v, want 0.65 (hydrated from DB)", eff.Tone)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/personality/ -run TestManager -v`
Expected: FAIL — `NewManager` undefined

- [ ] **Step 3: Implement Manager**

```go
// internal/personality/manager.go
package personality

import (
	"fmt"
	"log/slog"
	"math"
	"sync"

	"github.com/scaler-tech/toad/internal/state"
)

const learnedCap = 0.35 // max ±0.35 from base

// Manager manages the personality state: base traits + learned adjustments.
type Manager struct {
	mu       sync.RWMutex
	base     Traits
	learned  Traits // accumulated deltas
	store    *Store // nil for in-memory only
	learning bool   // whether feedback processing is enabled
}

// NewManager creates an in-memory-only manager (for tests).
func NewManager(base Traits) *Manager {
	return &Manager{base: base, learning: true}
}

// NewPersistentManager creates a manager backed by SQLite, hydrating learned state.
func NewPersistentManager(db *state.DB, base Traits) (*Manager, error) {
	s := NewStore(db)
	deltas, err := s.SumDeltas()
	if err != nil {
		return nil, fmt.Errorf("hydrating personality: %w", err)
	}

	var learned Traits
	for trait, delta := range deltas {
		learned.Set(trait, delta)
	}

	return &Manager{base: base, learned: learned, store: s, learning: true}, nil
}

// Effective returns the current effective traits (base + learned, clamped).
// Returns a value copy, safe for concurrent use.
func (m *Manager) Effective() Traits {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.base.Add(m.learned).Clamp()
}

// Base returns the base traits (value copy).
func (m *Manager) Base() Traits {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.base
}

// LearningEnabled returns whether feedback processing is enabled.
func (m *Manager) LearningEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.learning
}

// SetLearning enables or disables feedback processing.
func (m *Manager) SetLearning(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.learning = enabled
}

// ManualAdjust sets a trait to an absolute value by computing the required delta.
// Intentionally bypasses the learnedCap — manual dashboard edits are the escape
// hatch for exceeding the ±0.35 automatic learning cap (per spec).
func (m *Manager) ManualAdjust(trait string, targetValue float64, note string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	baseVal, ok := m.base.Get(trait)
	if !ok {
		return fmt.Errorf("unknown trait: %s", trait)
	}

	currentLearned, _ := m.learned.Get(trait)
	requiredDelta := targetValue - baseVal
	adjustDelta := requiredDelta - currentLearned

	m.learned.Set(trait, requiredDelta)

	if m.store != nil {
		return m.store.Insert(Adjustment{
			Trait:       trait,
			Delta:       adjustDelta,
			Source:      "manual",
			Reasoning:   note,
			BeforeValue: baseVal + currentLearned,
			AfterValue:  targetValue,
		})
	}
	return nil
}

// applyAdjustment applies a dampened, capped delta to a trait.
func (m *Manager) applyAdjustment(trait string, rawDelta float64, source, triggerDetail, reasoning string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	baseVal, ok := m.base.Get(trait)
	if !ok {
		return fmt.Errorf("unknown trait: %s", trait)
	}

	currentLearned, _ := m.learned.Get(trait)
	currentEffective := clamp01(baseVal + currentLearned)

	// Dampening near extremes: max(0.05, 1 - |current - 0.5| * 2)
	dampen := math.Max(0.05, 1.0-math.Abs(currentEffective-0.5)*2.0)
	delta := rawDelta * dampen

	// Apply learned cap: ±0.35 from base
	newLearned := currentLearned + delta
	if newLearned > learnedCap {
		newLearned = learnedCap
	}
	if newLearned < -learnedCap {
		newLearned = -learnedCap
	}
	actualDelta := newLearned - currentLearned

	if actualDelta == 0 {
		return nil
	}

	m.learned.Set(trait, newLearned)
	newEffective := clamp01(baseVal + newLearned)

	if m.store != nil {
		return m.store.Insert(Adjustment{
			Trait:         trait,
			Delta:         actualDelta,
			Source:        source,
			TriggerDetail: triggerDetail,
			Reasoning:     reasoning,
			BeforeValue:   currentEffective,
			AfterValue:    newEffective,
		})
	}
	return nil
}

// ProcessText is a stub for LLM-interpreted feedback (deferred to follow-up plan).
// Currently logs the feedback and returns nil.
func (m *Manager) ProcessText(text, context string) error {
	slog.Debug("personality text feedback received (not yet interpreted)", "text", text, "context", context)
	return nil
}

// RecentAdjustments returns the N most recent personality changes.
func (m *Manager) RecentAdjustments(limit int) ([]Adjustment, error) {
	if m.store == nil {
		return nil, nil
	}
	return m.store.Recent(limit)
}

// Export flattens (base + learned) into a PersonalityFile.
func (m *Manager) Export(name, description string) (*PersonalityFile, error) {
	eff := m.Effective()
	return &PersonalityFile{
		Version:     1,
		Name:        name,
		Description: description,
		Traits:      eff,
	}, nil
}

// Import loads a personality file as the new base and resets learned adjustments.
func (m *Manager) Import(pf *PersonalityFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.base = pf.Traits
	m.learned = Traits{} // reset

	if m.store != nil {
		return m.store.ClearAll()
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/personality/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(personality): add Manager with dampening, learned cap, and persistence
```

---

## Chunk 2: Trait Translation

### Task 6: Mode Types and Overrides

**Files:**
- Create: `internal/personality/translator.go`
- Test: `internal/personality/translator_test.go`

- [ ] **Step 1: Write failing test for PromptFragments and ConfigOverrides**

```go
// internal/personality/translator_test.go
package personality

import (
	"strings"
	"testing"
)

func TestPromptFragmentsRibbit(t *testing.T) {
	m := NewManager(DefaultTraits())
	frags := m.PromptFragments(ModeRibbit)
	if len(frags) == 0 {
		t.Error("expected prompt fragments for ribbit mode")
	}
	// Default thoroughness is 0.70 — should get "thorough" language
	found := false
	for _, f := range frags {
		if len(f) > 0 {
			found = true
		}
	}
	if !found {
		t.Error("expected non-empty prompt fragments")
	}
}

func TestConfigOverridesDigest(t *testing.T) {
	m := NewManager(DefaultTraits())
	ov := m.ConfigOverrides(ModeDigest)
	// Default confidence threshold is 0.80 — should produce a min_confidence override
	if ov.MinConfidence == nil {
		t.Error("expected MinConfidence override for digest mode")
	}
}

func TestPromptFragmentsTadpole(t *testing.T) {
	traits := DefaultTraits()
	traits.TestAffinity = 0.90 // high
	m := NewManager(traits)
	frags := m.PromptFragments(ModeTadpole)
	hasTestInstruction := false
	for _, f := range frags {
		if contains(f, "test") {
			hasTestInstruction = true
		}
	}
	if !hasTestInstruction {
		t.Error("high test_affinity should produce test-related prompt fragment")
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && strings.Contains(strings.ToLower(s), substr)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/personality/ -run TestPrompt -v`
Expected: FAIL

- [ ] **Step 3: Implement translator**

```go
// internal/personality/translator.go
package personality

import (
	"math"
)

// Mode represents a Toad operational mode.
type Mode string

const (
	ModeRibbit  Mode = "ribbit"
	ModeTadpole Mode = "tadpole"
	ModeDigest  Mode = "digest"
	ModeTriage  Mode = "triage"
)

// Overrides contains config parameter adjustments derived from personality traits.
type Overrides struct {
	MaxTurns        *int
	MaxRetries      *int
	MaxFilesChanged *int
	TimeoutMinutes  *int
	MinConfidence   *float64
	MaxEstSize      *string
}

func intPtr(v int) *int         { return &v }
func float64Ptr(v float64) *float64 { return &v }
func stringPtr(v string) *string    { return &v }

// PromptFragments returns personality-driven prompt additions for a mode.
func (m *Manager) PromptFragments(mode Mode) []string {
	t := m.Effective()
	var frags []string

	switch mode {
	case ModeRibbit:
		frags = append(frags, thoroughnessFragment(t.Thoroughness))
		frags = append(frags, contextHungerFragment(t.ContextHunger))
		frags = append(frags, verbosityFragment(t.Verbosity))
		frags = append(frags, explanationDepthFragment(t.ExplanationDepth))
		frags = append(frags, toneFragment(t.Tone))
		frags = append(frags, defensivenessFragment(t.Defensiveness))

	case ModeTadpole:
		frags = append(frags, riskToleranceFragment(t.RiskTolerance))
		frags = append(frags, scopeAppetiteFragment(t.ScopeAppetite))
		frags = append(frags, testAffinityFragment(t.TestAffinity))
		frags = append(frags, creativityFragment(t.Creativity))
		frags = append(frags, patternConformityFragment(t.PatternConformity))
		frags = append(frags, documentationDriveFragment(t.DocumentationDrive))
		frags = append(frags, strictnessFragment(t.Strictness))

	case ModeDigest:
		frags = append(frags, patternRecognitionFragment(t.PatternRecognition))
		frags = append(frags, initiativeFragment(t.Initiative))

	case ModeTriage:
		// Triage is fast classification — minimal personality influence on prompt
	}

	// Filter out empty fragments
	var result []string
	for _, f := range frags {
		if f != "" {
			result = append(result, f)
		}
	}
	return result
}

// ConfigOverrides returns personality-driven config parameter adjustments.
func (m *Manager) ConfigOverrides(mode Mode) Overrides {
	t := m.Effective()
	var ov Overrides

	switch mode {
	case ModeRibbit:
		ov.MaxTurns = intPtr(scaleInt(t.Thoroughness, 5, 15))
		ov.TimeoutMinutes = intPtr(scaleInt(t.SpeedVsPolish, 5, 15))

	case ModeTadpole:
		ov.MaxTurns = intPtr(scaleInt(t.SpeedVsPolish, 15, 45))
		ov.MaxRetries = intPtr(scaleInt(t.RetryPersistence, 0, 3))
		ov.MaxFilesChanged = intPtr(scaleInt(t.RiskTolerance, 3, 10))
		ov.TimeoutMinutes = intPtr(scaleInt(t.SpeedVsPolish, 5, 15))

	case ModeDigest:
		ov.MinConfidence = float64Ptr(scaleFloat(t.ConfidenceThreshold, 0.80, 0.99))
		ov.MaxEstSize = maxEstSizeFromTrait(t.ScopeSensitivity)

	case ModeTriage:
		ov.MinConfidence = float64Ptr(scaleFloat(t.ConfidenceThreshold, 0.80, 0.99))
	}

	return ov
}

// scaleInt linearly maps a trait [0,1] to an int range [lo,hi].
func scaleInt(trait float64, lo, hi int) int {
	return lo + int(math.Round(trait*float64(hi-lo)))
}

// scaleFloat linearly maps a trait [0,1] to a float range [lo,hi].
func scaleFloat(trait float64, lo, hi float64) float64 {
	return lo + trait*(hi-lo)
}

func maxEstSizeFromTrait(scopeSensitivity float64) *string {
	if scopeSensitivity > 0.7 {
		return stringPtr("small")
	}
	if scopeSensitivity > 0.4 {
		return stringPtr("medium")
	}
	return stringPtr("large")
}

// --- Prompt fragment functions ---
// Each returns a prompt instruction string based on trait value.

func thoroughnessFragment(v float64) string {
	if v < 0.3 {
		return "Do a quick scan. Focus on the most obvious answer."
	}
	if v < 0.6 {
		return "Search the codebase to find the answer."
	}
	if v < 0.8 {
		return "Search thoroughly. Check related files and trace call chains."
	}
	return "Do an exhaustive investigation. Trace every call chain, check tests, read git history."
}

func contextHungerFragment(v float64) string {
	if v < 0.3 {
		return "Focus on the specific file or function mentioned."
	}
	if v < 0.7 {
		return ""
	}
	return "Pull in surrounding context: callers, tests, and related files."
}

func verbosityFragment(v float64) string {
	if v < 0.3 {
		return "Be extremely concise — 1-3 lines max."
	}
	if v < 0.5 {
		return "Keep it short (3-5 lines for questions, up to 10 for bugs)."
	}
	if v < 0.7 {
		return "Give a clear, detailed answer. Up to 15 lines is fine."
	}
	return "Provide a thorough explanation with full context and examples."
}

func explanationDepthFragment(v float64) string {
	if v < 0.3 {
		return "Just give the answer — no need to explain why."
	}
	if v < 0.6 {
		return ""
	}
	return "Explain your reasoning — include why, not just what."
}

func toneFragment(v float64) string {
	if v < 0.3 {
		return "Be purely technical and professional."
	}
	if v < 0.7 {
		return "Be conversational but focused."
	}
	return "Be friendly and approachable. A bit of personality is welcome."
}

func defensivenessFragment(v float64) string {
	if v < 0.4 {
		return ""
	}
	if v < 0.7 {
		return "If the request seems off, briefly mention potential issues."
	}
	return "If you see problems with the approach, push back and suggest alternatives."
}

func riskToleranceFragment(v float64) string {
	if v < 0.3 {
		return "Make the absolute minimum change needed. Touch as few files as possible."
	}
	if v < 0.6 {
		return "Make focused changes. Small refactors are okay if they serve the fix."
	}
	return "Don't shy away from broader changes if they're the right solution."
}

func scopeAppetiteFragment(v float64) string {
	if v < 0.3 {
		return "Fix ONLY the specific issue. Do NOT touch any unrelated code."
	}
	if v < 0.6 {
		return "Focus on the task but fix obvious adjacent issues if trivial."
	}
	return "Improve code you encounter — fix naming, clean up patterns, update related tests."
}

func testAffinityFragment(v float64) string {
	if v < 0.3 {
		return ""
	}
	if v < 0.6 {
		return "Add tests if you're changing behavior that isn't covered."
	}
	return "Always write or update tests alongside your changes. Ensure new behavior is tested."
}

func creativityFragment(v float64) string {
	if v < 0.3 {
		return "Follow existing patterns exactly. Do not introduce new approaches."
	}
	if v < 0.7 {
		return ""
	}
	return "If you see a better approach than existing patterns, suggest and implement it."
}

func patternConformityFragment(v float64) string {
	if v < 0.4 {
		return ""
	}
	if v < 0.7 {
		return "Follow existing code style and naming conventions."
	}
	return "Match existing code style exactly — naming, error handling, structure, patterns."
}

func documentationDriveFragment(v float64) string {
	if v < 0.3 {
		return "Do NOT add comments or documentation unless absolutely necessary."
	}
	if v < 0.6 {
		return ""
	}
	return "Add clear comments for non-obvious logic. Update relevant docs if needed."
}

func strictnessFragment(v float64) string {
	if v < 0.3 {
		return ""
	}
	if v < 0.7 {
		return "All tests and linting must pass."
	}
	return "Zero tolerance — all tests pass, all lint clean, no warnings."
}

func patternRecognitionFragment(v float64) string {
	if v < 0.5 {
		return ""
	}
	return "Look for patterns — if this bug exists in one place, check if similar code has the same issue."
}

func initiativeFragment(v float64) string {
	if v < 0.5 {
		return ""
	}
	return "If you notice improvements beyond the immediate task, mention them in your response."
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/personality/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(personality): add trait-to-prompt/config translation layer
```

---

## Chunk 3: Feedback Processing

### Task 7: Emoji Feedback Processing

**Files:**
- Create: `internal/personality/feedback.go`
- Test: `internal/personality/feedback_test.go`

- [ ] **Step 1: Write failing test**

```go
// internal/personality/feedback_test.go
package personality

import "testing"

func TestProcessEmoji(t *testing.T) {
	m := NewManager(DefaultTraits())

	// 🐇 = too shallow → increase thoroughness, context_hunger
	err := m.ProcessEmoji("rabbit", "ribbit reply about auth")
	if err != nil {
		t.Fatal(err)
	}

	eff := m.Effective()
	if eff.Thoroughness <= 0.70 {
		t.Errorf("Thoroughness = %v, should have increased from 0.70", eff.Thoroughness)
	}
	if eff.ContextHunger <= 0.50 {
		t.Errorf("ContextHunger = %v, should have increased from 0.50", eff.ContextHunger)
	}
}

func TestProcessEmojiUnknown(t *testing.T) {
	m := NewManager(DefaultTraits())
	err := m.ProcessEmoji("sparkles", "some context")
	if err == nil {
		t.Error("expected error for unknown emoji")
	}
}

func TestProcessOutcomePRMerged(t *testing.T) {
	m := NewManager(DefaultTraits())

	err := m.ProcessOutcome(OutcomeSignal{
		Type:  "pr_merged",
		PRURL: "https://github.com/org/repo/pull/42",
	})
	if err != nil {
		t.Fatal(err)
	}

	// PR merged reinforces: risk_tolerance, scope_appetite, test_affinity, strictness
	// Values should not decrease
	eff := m.Effective()
	if eff.RiskTolerance < 0.30 {
		t.Errorf("RiskTolerance = %v, should not decrease after merge", eff.RiskTolerance)
	}
}

func TestProcessOutcomePRClosed(t *testing.T) {
	m := NewManager(DefaultTraits())

	err := m.ProcessOutcome(OutcomeSignal{
		Type:  "pr_closed",
		PRURL: "https://github.com/org/repo/pull/43",
	})
	if err != nil {
		t.Fatal(err)
	}

	eff := m.Effective()
	if eff.RiskTolerance >= 0.30 {
		t.Errorf("RiskTolerance = %v, should decrease after close", eff.RiskTolerance)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/personality/ -run TestProcess -v`
Expected: FAIL

- [ ] **Step 3: Implement feedback processing**

```go
// internal/personality/feedback.go
package personality

import "fmt"

// OutcomeSignal represents an objective outcome event.
type OutcomeSignal struct {
	Type         string // "pr_merged", "pr_closed", "pr_review_rounds", "ribbit_followup", "digest_dismissed", "digest_approved"
	PRURL        string
	ReviewRounds int
	Metadata     map[string]string
}

// emojiMapping defines which traits an emoji affects and by how much.
type emojiMapping struct {
	traits     []string
	directions []float64
}

// defaultEmojiMappings returns the standard emoji → trait mappings.
func defaultEmojiMappings() map[string]emojiMapping {
	return map[string]emojiMapping{
		"turtle":      {traits: []string{"thoroughness", "speed_vs_polish"}, directions: []float64{-0.03, 0.03}},
		"rabbit":      {traits: []string{"thoroughness", "context_hunger"}, directions: []float64{0.03, 0.03}},
		"mute":        {traits: []string{"verbosity", "explanation_depth"}, directions: []float64{-0.03, -0.03}},
		"loudspeaker": {traits: []string{"verbosity", "explanation_depth"}, directions: []float64{0.03, 0.03}},
		"ocean":       {traits: []string{"scope_appetite"}, directions: []float64{-0.03}},
		"test_tube":   {traits: []string{"test_affinity"}, directions: []float64{0.03}},
		"bulb":        {traits: []string{"creativity"}, directions: []float64{0.03}},
		// dart = reinforce, handled separately
	}
}

// ProcessEmoji applies a mapped emoji's trait adjustments.
func (m *Manager) ProcessEmoji(emoji, context string) error {
	if !m.LearningEnabled() {
		return nil
	}

	// Handle reinforcement emoji separately
	if emoji == "dart" {
		return m.processReinforce([]string{"scope_appetite", "risk_tolerance"}, context)
	}

	mappings := defaultEmojiMappings()
	mapping, ok := mappings[emoji]
	if !ok {
		return fmt.Errorf("unknown personality emoji: %s", emoji)
	}

	for i, trait := range mapping.traits {
		detail := fmt.Sprintf(":%s: on %s", emoji, truncate(context, 50))
		if err := m.applyAdjustment(trait, mapping.directions[i], "emoji", detail, ""); err != nil {
			return err
		}
	}
	return nil
}

// processReinforce reduces the magnitude of the most recent negative adjustment on the given traits.
func (m *Manager) processReinforce(traits []string, context string) error {
	if m.store == nil {
		return nil // in-memory mode — no history to look up
	}

	recent, err := m.store.Recent(50)
	if err != nil {
		return err
	}

	for _, trait := range traits {
		for _, adj := range recent {
			if adj.Trait == trait && adj.Delta < 0 {
				// Reduce magnitude by 50%
				correction := -adj.Delta * 0.5
				detail := fmt.Sprintf(":dart: reinforced %s (context: %s)", trait, truncate(context, 40))
				if err := m.applyAdjustment(trait, correction, "emoji", detail, "reinforcement"); err != nil {
					return err
				}
				break // only the most recent negative adjustment per trait
			}
		}
	}
	return nil
}

// ProcessOutcome applies trait adjustments based on objective outcomes.
func (m *Manager) ProcessOutcome(signal OutcomeSignal) error {
	if !m.LearningEnabled() {
		return nil
	}

	detail := fmt.Sprintf("%s: %s", signal.Type, signal.PRURL)

	// Helper to apply and log errors (multiple adjustments per signal,
	// we don't want one failure to block the rest).
	apply := func(trait string, delta float64, reasoning string) {
		if err := m.applyAdjustment(trait, delta, "outcome", detail, reasoning); err != nil {
			slog.Error("personality outcome adjustment failed",
				"trait", trait, "delta", delta, "error", err)
		}
	}

	switch signal.Type {
	case "pr_merged":
		// Strong positive: reinforce current traits
		apply("risk_tolerance", 0.02, "PR merged cleanly")
		apply("scope_appetite", 0.01, "PR merged cleanly")
		apply("test_affinity", 0.01, "PR merged cleanly")
		apply("strictness", 0.01, "PR merged cleanly")

	case "pr_closed":
		// Strong negative
		apply("risk_tolerance", -0.03, "PR closed without merge")
		apply("confidence_threshold", 0.02, "PR closed without merge")
		apply("scope_sensitivity", 0.02, "PR closed without merge")

	case "pr_review_rounds":
		// More review rounds = need more strictness
		if signal.ReviewRounds > 1 {
			reason := fmt.Sprintf("PR needed %d review rounds", signal.ReviewRounds)
			apply("strictness", 0.02, reason)
			apply("pattern_conformity", 0.01, reason)
			apply("collaboration", 0.01, reason)
		}

	case "ribbit_followup":
		// User followed up — answer wasn't sufficient
		apply("thoroughness", 0.01, "user followed up")
		apply("explanation_depth", 0.01, "user followed up")

	case "digest_dismissed":
		apply("confidence_threshold", 0.02, "digest opportunity dismissed")

	case "digest_approved":
		apply("confidence_threshold", -0.01, "digest opportunity approved")
		apply("autonomy", 0.01, "digest opportunity approved")
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/personality/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(personality): add emoji and outcome feedback processing
```

---

## Chunk 4: Config Integration

### Task 8: Add Personality Config Section

**Files:**
- Modify: `internal/config/config.go`
- Test: existing config tests should still pass

- [ ] **Step 1: Add PersonalityConfig to Config struct**

In `internal/config/config.go`, add to the `Config` struct:

```go
Personality  PersonalityConfig  `yaml:"personality"`
```

Add the type:

```go
type PersonalityConfig struct {
	Enabled         bool   `yaml:"enabled"`          // default: false (opt-in)
	LearningEnabled bool   `yaml:"learning_enabled"` // default: true
	FilePath        string `yaml:"file_path"`        // default: ~/.toad/personality.yaml
}
// Note: Custom emoji mappings (FeedbackEmojis) are deferred to a follow-up.
// The default emoji palette is hardcoded in internal/personality/feedback.go for now.
```

- [ ] **Step 2: Add defaults in `defaults()` function**

```go
Personality: PersonalityConfig{
	Enabled:         false,
	LearningEnabled: true,
},
```

- [ ] **Step 3: Run existing tests**

Run: `go test ./internal/config/ -v && go test ./... -count=1 2>&1 | tail -20`
Expected: PASS

- [ ] **Step 4: Commit**

```
feat(config): add personality configuration section
```

### Task 9: Wire Personality Manager into Daemon Startup

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Initialize personality Manager in the daemon startup**

In the daemon startup code in `cmd/root.go`, after config loading and DB opening, add:

```go
// Initialize personality manager
var personalityMgr *personality.Manager
if cfg.Personality.Enabled {
	pfPath := cfg.Personality.FilePath
	if pfPath == "" {
		pfPath = filepath.Join(home, "personality.yaml")
	}
	pf, err := personality.LoadFile(pfPath)
	if err != nil {
		return fmt.Errorf("loading personality: %w", err)
	}
	personalityMgr, err = personality.NewPersistentManager(stateDB, pf.Traits)
	if err != nil {
		return fmt.Errorf("initializing personality: %w", err)
	}
	personalityMgr.SetLearning(cfg.Personality.LearningEnabled)
	slog.Info("personality loaded", "name", pf.Name, "learning", cfg.Personality.LearningEnabled)
} else {
	// Even when disabled, provide a manager with defaults so consumers can call it without nil checks
	personalityMgr = personality.NewManager(personality.DefaultTraits())
	personalityMgr.SetLearning(false)
}
```

- [ ] **Step 2: Pass personality Manager to ribbit and tadpole constructors**

This requires adding a `personality *personality.Manager` field to `ribbit.Engine` and passing it through. The actual prompt augmentation will be wired in the next chunk (integration tasks).

For now, just thread the manager through without using it yet:

Add to `ribbit.New()`:
```go
func New(agentProvider agent.Provider, cfg *config.Config, pm *personality.Manager) *Engine {
```

Add to `tadpole.NewRunner()`:
```go
func NewRunner(cfg *config.Config, agentProvider agent.Provider, slack *islack.Client, sm *state.Manager, vcsResolver vcs.Resolver, pm *personality.Manager) *Runner {
```

Update all call sites in `cmd/root.go`. Also update any test files that call these constructors — pass `nil` for the personality manager in existing tests. Check:
- `internal/ribbit/` tests
- `internal/reviewer/` tests (if they call `tadpole.NewRunner`)
- Any other test that constructs ribbit.Engine or tadpole.Runner

- [ ] **Step 3: Build to verify compilation**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 4: Run tests**

Run: `go test ./... -count=1 2>&1 | tail -20`
Expected: PASS

- [ ] **Step 5: Commit**

```
feat(personality): wire Manager into daemon startup and pass to ribbit/tadpole
```

---

## Chunk 5: Prompt & Config Integration

### Task 10: Integrate Personality into Ribbit Prompts

**Files:**
- Modify: `internal/ribbit/ribbit.go`
- Test: `internal/ribbit/ribbit_test.go` (if exists, else manual verification)

- [ ] **Step 1: Add personality fragments to ribbit prompt construction**

In `ribbit.go`, after building the base prompt, append personality fragments:

```go
// In Respond(), after building the prompt:
if e.personality != nil {
	frags := e.personality.PromptFragments(personality.ModeRibbit)
	if len(frags) > 0 {
		prompt += "\n\n## Personality instructions\n\n" + strings.Join(frags, "\n")
	}
	ov := e.personality.ConfigOverrides(personality.ModeRibbit)
	if ov.MaxTurns != nil {
		maxTurns = *ov.MaxTurns
	}
}
```

Where `maxTurns` replaces the hardcoded `10` in the `agent.RunOpts`.

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```
feat(ribbit): integrate personality traits into prompt and config
```

### Task 11: Integrate Personality into Tadpole Prompts

**Files:**
- Modify: `internal/tadpole/runner.go`

- [ ] **Step 1: Add personality to buildTadpolePrompt**

Modify `buildTadpolePrompt` to accept a `*personality.Manager` parameter and append fragments:

```go
func buildTadpolePrompt(task Task, maxFiles int, repoPaths map[string]string, pm *personality.Manager) string {
	// ... existing code ...

	if pm != nil {
		frags := pm.PromptFragments(personality.ModeTadpole)
		if len(frags) > 0 {
			sb.WriteString("\n## Personality instructions\n\n")
			for _, f := range frags {
				sb.WriteString("- " + f + "\n")
			}
		}
	}

	return sb.String()
}
```

- [ ] **Step 2: Apply config overrides to RunOpts in Execute()**

```go
// In Execute(), before calling r.agent.Run():
if r.personality != nil {
	ov := r.personality.ConfigOverrides(personality.ModeTadpole)
	if ov.MaxTurns != nil {
		maxTurns = *ov.MaxTurns  // override cfg.Limits.MaxTurns
	}
	if ov.MaxRetries != nil {
		maxRetries = *ov.MaxRetries
	}
	if ov.MaxFilesChanged != nil {
		valCfg.MaxFilesChanged = *ov.MaxFilesChanged
	}
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```
feat(tadpole): integrate personality traits into prompt and config
```

### Task 12: Integrate Personality into Digest

**Files:**
- Modify: `internal/digest/digest.go`
- Modify: `cmd/root.go` (investigation prompt)

- [ ] **Step 1: Apply confidence override to digest guardrails**

In `digest.go`, where `minConfidence` is checked, apply personality override:

```go
// In processOpportunities or wherever minConfidence is used:
minConf := e.cfg.Digest.MinConfidence
if e.personality != nil {
	ov := e.personality.ConfigOverrides(personality.ModeDigest)
	if ov.MinConfidence != nil {
		minConf = *ov.MinConfidence
	}
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```
feat(digest): integrate personality confidence threshold override
```

---

## Chunk 6: Slack Feedback Routing

### Task 13: Add IsReplyToToad Helper

**Files:**
- Modify: `internal/slack/client.go`

- [ ] **Step 1: Implement IsReplyToToad**

```go
// IsReplyToToad checks if a message is a reply in a thread where the parent message
// was authored by Toad. Distinct from IsToadReply which checks a specific message.
func (c *Client) IsReplyToToad(channel, threadTS string) bool {
	if threadTS == "" {
		return false
	}
	return c.IsToadReply(channel, threadTS)
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```
feat(slack): add IsReplyToToad helper for personality feedback routing
```

### Task 14: Route Personality Emojis in handleReaction

**Files:**
- Modify: `internal/slack/events.go`

- [ ] **Step 1: Add personality emoji routing before trigger emoji check**

In `handleReaction`, modify the flow to check personality emojis first when the reaction is on a Toad message:

```go
func handleReaction(ctx context.Context, c *Client, ev *slackevents.ReactionAddedEvent) {
	slog.Debug("reaction event", "emoji", ev.Reaction, "user", ev.User, "channel", ev.Item.Channel)

	// Personality feedback: non-trigger reactions on Toad's own messages.
	// Check this BEFORE the trigger emoji guard so personality emojis aren't discarded.
	// The trigger emoji (:frog:) on toad messages still falls through to the existing
	// tadpole-request flow below.
	if ev.Reaction != c.triggers.Emoji && c.personalityHandler != nil {
		if c.inChannel(ev.Item.Channel) && c.IsToadReply(ev.Item.Channel, ev.Item.Timestamp) {
			c.personalityHandler(ctx, ev.Reaction, ev.Item.Channel, ev.Item.Timestamp)
			return
		}
	}

	// --- existing flow unchanged from here ---
	if ev.Reaction != c.triggers.Emoji {
		slog.Debug("skipping: non-trigger emoji", "emoji", ev.Reaction, "trigger", c.triggers.Emoji)
		return
	}
	if !c.inChannel(ev.Item.Channel) {
		slog.Debug("skipping: unmonitored channel", "channel", ev.Item.Channel)
		return
	}
	if c.markSeen(ev.Item.Channel, ev.Item.Timestamp) {
		slog.Debug("skipping: duplicate reaction", "ts", ev.Item.Timestamp)
		return
	}
	// ... rest of existing handleReaction (fetch message, dispatch) unchanged ...
}
```

Note: Personality reactions do NOT call `markSeen` because they use a different dedup key space (a user reacting :+1: and then :dart: on the same message are two separate personality signals, and the trigger-emoji flow should still work on that message independently).

Add `personalityHandler` field to Client struct:

```go
type PersonalityReactionHandler func(ctx context.Context, emoji, channel, ts string)
```

Add setter:
```go
func (c *Client) OnPersonalityReaction(handler PersonalityReactionHandler) {
	c.personalityHandler = handler
}
```

- [ ] **Step 2: Wire handler in cmd/root.go**

```go
if cfg.Personality.Enabled && personalityMgr.LearningEnabled() {
	slackClient.OnPersonalityReaction(func(ctx context.Context, emoji, channel, ts string) {
		if err := personalityMgr.ProcessEmoji(emoji, fmt.Sprintf("channel:%s ts:%s", channel, ts)); err != nil {
			slog.Debug("personality emoji not mapped", "emoji", emoji)
		}
	})
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```
feat(slack): route personality emoji reactions on toad messages
```

### Task 15: Route Thread Replies for LLM-Interpreted Feedback

**Files:**
- Modify: `internal/slack/events.go` (handleMessage)
- Modify: `cmd/root.go` (wire handler)

- [ ] **Step 1: Add personality text handler routing in handleMessage**

In `handleMessage`, add this block **before** the `msg := &IncomingMessage{...}` construction (around line 120), **after** the `markSeen` dedup check. At this point, @mentions have already been filtered out (line 108-111 returns early for mentions), so no additional mention guard is needed:

```go
// Personality feedback: thread replies to Toad messages (non-mention, non-bot).
// @mentions are already filtered above, so any thread reply to a toad message
// reaching this point is potential personality feedback.
if ev.ThreadTimeStamp != "" && !isBot && c.personalityTextHandler != nil {
	if c.IsToadReply(ev.Channel, ev.ThreadTimeStamp) {
		c.personalityTextHandler(ctx, fullText, ev.Channel, ev.ThreadTimeStamp)
		// Don't return — the message should still be dispatched to the regular handler
		// for triage (it might be a question AND feedback).
	}
}
```

Note: We use `IsToadReply` directly (checking if the thread parent was authored by Toad) rather than the `IsReplyToToad` wrapper, which is functionally identical. If you added `IsReplyToToad` in Task 13, either one works.

Add field and setter:
```go
type PersonalityTextHandler func(ctx context.Context, text, channel, threadTS string)

func (c *Client) OnPersonalityText(handler PersonalityTextHandler) {
	c.personalityTextHandler = handler
}
```

Note: The LLM interpreter (Haiku call) is deferred to a future task. For now, `ProcessText` can be a stub that logs the feedback. The full Haiku interpretation can be built later.

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```
feat(slack): route thread replies to personality text feedback handler
```

---

## Chunk 7: PR Outcome Tracking

### Task 16: Extend PR Watcher for Personality Outcome Signals

**Files:**
- Modify: `internal/reviewer/reviewer.go`

- [ ] **Step 1: Add personality callback to Watcher**

Add a `personalityOutcome` callback field to the `Watcher` struct:

```go
type OutcomeCallback func(signal personality.OutcomeSignal)
```

Add setter and call it when PRs reach terminal states. In the existing `poll()` method, where `ClosePRWatch` is called with `finalState`, add:

```go
if w.personalityOutcome != nil {
	sigType := "pr_closed"
	if strings.EqualFold(finalState, "MERGED") {
		sigType = "pr_merged"
	}
	w.personalityOutcome(personality.OutcomeSignal{
		Type:         sigType,
		PRURL:        watch.PRURL,
		ReviewRounds: watch.FixCount,
	})
}
```

- [ ] **Step 2: Wire in cmd/root.go**

```go
if cfg.Personality.Enabled {
	prWatcher.OnPersonalityOutcome(func(signal personality.OutcomeSignal) {
		if err := personalityMgr.ProcessOutcome(signal); err != nil {
			slog.Warn("personality outcome processing failed", "error", err)
		}
	})
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```
feat(reviewer): emit personality outcome signals on PR merge/close
```

---

## Chunk 8: Dashboard

### Task 17: Add Personality API Endpoints

**Files:**
- Modify: `cmd/root.go` (or wherever HTTP handlers are registered)

- [ ] **Step 1: Add /api/personality endpoint**

```go
http.HandleFunc("/api/personality", func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		eff := personalityMgr.Effective()
		base := personalityMgr.Base()
		recent, _ := personalityMgr.RecentAdjustments(20)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"effective":   eff,
			"base":        base,
			"adjustments": recent,
			"learning":    personalityMgr.LearningEnabled(),
		})
	case "POST":
		// Manual trait adjustment
		var req struct {
			Trait string  `json:"trait"`
			Value float64 `json:"value"`
			Note  string  `json:"note"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if err := personalityMgr.ManualAdjust(req.Trait, req.Value, req.Note); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.WriteHeader(200)
	}
})

http.HandleFunc("/api/personality/export", func(w http.ResponseWriter, r *http.Request) {
	pf, _ := personalityMgr.Export("exported", "Exported from dashboard")
	data, _ := pf.Marshal()
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", "attachment; filename=personality.yaml")
	w.Write(data)
})
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```
feat(dashboard): add personality API endpoints
```

### Task 18: Add Radar Chart to Dashboard HTML

**Files:**
- Modify: `cmd/web/dashboard.html`

- [ ] **Step 1: Add Chart.js CDN and personality section**

Add to the HTML `<head>`:
```html
<script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
```

Add a personality section to the dashboard body with a radar chart canvas and trait sliders. The section fetches from `/api/personality` on load and renders using Chart.js radar chart type.

This is a larger HTML/JS task. The key elements:
- A `<canvas id="personalityChart">` for the radar chart
- A slider group for each of the 5 categories
- A recent adjustments feed
- JS that fetches `/api/personality` and renders

- [ ] **Step 2: Test manually by running `toad` and opening the dashboard**

Run: `go build ./cmd/toad && ./toad` (or however the daemon starts)
Open: `http://localhost:<dashboard_port>`
Verify: radar chart renders, sliders work

- [ ] **Step 3: Commit**

```
feat(dashboard): add personality radar chart and trait management UI
```

---

## Chunk 9: Final Integration Tests

### Task 19: End-to-End Personality Test

**Files:**
- Create: `internal/personality/integration_test.go`

- [ ] **Step 1: Write integration test covering full lifecycle**

```go
func TestFullLifecycle(t *testing.T) {
	db := testDB(t)
	base := DefaultTraits()

	// 1. Create persistent manager
	m, err := NewPersistentManager(db, base)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Apply emoji feedback
	m.ProcessEmoji("rabbit", "shallow ribbit")
	m.ProcessEmoji("turtle", "too slow investigation")

	// 3. Apply outcome
	m.ProcessOutcome(OutcomeSignal{Type: "pr_merged", PRURL: "test"})

	// 4. Check effective values changed
	eff := m.Effective()
	if eff == base {
		t.Error("effective should differ from base after feedback")
	}

	// 5. Export
	pf, err := m.Export("test-export", "integration test")
	if err != nil {
		t.Fatal(err)
	}
	if pf.Traits == base {
		t.Error("exported traits should reflect learned adjustments")
	}

	// 6. Import resets
	m.Import(&PersonalityFile{Version: 1, Name: "fresh", Traits: DefaultTraits()})
	eff2 := m.Effective()
	if eff2 != DefaultTraits() {
		t.Error("import should reset to fresh base")
	}

	// 7. Translation produces output
	frags := m.PromptFragments(ModeRibbit)
	if len(frags) == 0 {
		t.Error("should produce prompt fragments")
	}
	ov := m.ConfigOverrides(ModeTadpole)
	if ov.MaxRetries == nil {
		t.Error("should produce config overrides")
	}
}
```

- [ ] **Step 2: Run test**

Run: `go test ./internal/personality/ -run TestFullLifecycle -v`
Expected: PASS

- [ ] **Step 3: Run full test suite**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 4: Run gofmt**

Run: `gofmt -l .`
Expected: no output (all files formatted)

- [ ] **Step 5: Run go vet**

Run: `go vet ./...`
Expected: no issues

- [ ] **Step 6: Commit**

```
feat(personality): add integration test covering full lifecycle
```
