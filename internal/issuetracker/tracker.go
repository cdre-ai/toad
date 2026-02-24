// Package issuetracker provides a generic interface for issue tracker integrations.
package issuetracker

import (
	"context"
	"strings"

	"github.com/hergen/toad/internal/config"
)

// IssueRef represents a reference to an issue in an external tracker.
type IssueRef struct {
	Provider string // "linear", "jira"
	ID       string // "PLF-3125"
	URL      string
	Title    string
}

// BranchPrefix returns a lowercased issue ID suitable for branch naming.
// e.g. "PLF-3125" → "plf-3125"
func (r *IssueRef) BranchPrefix() string {
	return strings.ToLower(r.ID)
}

// Tracker is the interface for issue tracker integrations.
type Tracker interface {
	// ExtractIssueRef extracts an issue reference from message text.
	// Returns nil if no issue reference is found.
	ExtractIssueRef(text string) *IssueRef

	// CreateIssue creates a new issue in the tracker.
	CreateIssue(ctx context.Context, opts CreateIssueOpts) (*IssueRef, error)

	// ShouldCreateIssues reports whether the tracker is configured to
	// auto-create issues for opportunities that lack an existing reference.
	ShouldCreateIssues() bool
}

// CreateIssueOpts holds parameters for creating a new issue.
type CreateIssueOpts struct {
	Title       string
	Description string
	Category    string // "bug" or "feature"
}

// NoopTracker is a no-op implementation that returns nil for everything.
type NoopTracker struct{}

func (NoopTracker) ExtractIssueRef(string) *IssueRef                                { return nil }
func (NoopTracker) CreateIssue(context.Context, CreateIssueOpts) (*IssueRef, error) { return nil, nil }
func (NoopTracker) ShouldCreateIssues() bool                                        { return false }

// NewTracker creates a Tracker from config. Returns NoopTracker when disabled.
func NewTracker(cfg config.IssueTrackerConfig) Tracker {
	if !cfg.Enabled {
		return NoopTracker{}
	}
	switch cfg.Provider {
	case "linear":
		return NewLinearTracker(cfg)
	default:
		return NoopTracker{}
	}
}
