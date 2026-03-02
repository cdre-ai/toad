# VCS Providers

Toad uses a `Provider` interface for version control platform operations — creating PRs, checking CI status, reading review comments, and merging. Each repo can use a different provider via per-repo `vcs:` config overrides.

## Current Providers

| Provider | Platform key | CLI tool | Status |
|----------|-------------|----------|--------|
| GitHub | `github` | `gh` | Implemented |
| GitLab | `gitlab` | `glab` | Implemented (including self-hosted) |

## Possible Additions

| Provider | Platform key | CLI tool | Notes |
|----------|-------------|----------|-------|
| Bitbucket | `bitbucket` | `bb` or REST API | Atlassian's platform, common in enterprise |
| Gitea / Forgejo | `gitea` | `tea` | Self-hosted Git, GitHub-compatible API subset |
| Azure DevOps | `azuredevops` | `az repos` | Microsoft's platform |

## The Provider Interface

```go
type Provider interface {
    Check() error
    CreatePR(ctx context.Context, opts CreatePROpts) (string, error)
    EnableAutoMerge(ctx context.Context, repoPath, branch string) error
    GetPRState(ctx context.Context, prNumber int, repoPath string) (string, error)
    GetMergeability(ctx context.Context, prNumber int, repoPath string) (string, error)
    GetCIStatus(ctx context.Context, prNumber int, repoPath string) (*CIStatus, error)
    GetCIFailureLogs(ctx context.Context, failedRunIDs []string, repoPath string) string
    GetPRComments(ctx context.Context, prNumber int, repoPath string) ([]PRComment, error)
    AddCommentReaction(ctx context.Context, prNumber, commentID int, source, reaction, repoPath string) error
    ListBotPRs(ctx context.Context, branch, repoPath string) ([]int, error)
    MergePR(ctx context.Context, prNumber int, repoPath string) error
    ExtractPRNumber(prURL string) (int, error)
    ExtractRunID(detailsURL string) string
    PRNoun() string
}
```

This is the largest provider interface in Toad (14 methods) because the PR review watcher needs deep VCS integration. Here's what each group does:

**Core PR lifecycle:** `CreatePR`, `EnableAutoMerge`, `MergePR`, `PRNoun`

**PR state inspection:** `GetPRState`, `GetMergeability` — used by the reviewer to detect merged/closed PRs and merge conflicts

**CI integration:** `GetCIStatus`, `GetCIFailureLogs`, `ExtractRunID` — used by the reviewer to detect CI failures and spawn fix tadpoles with failure logs

**Review comments:** `GetPRComments`, `AddCommentReaction` — used by the reviewer to read human feedback and react when fixes are applied

**Bot PR management:** `ListBotPRs` — used to find and auto-merge bot-authored PRs (e.g., Renovate) targeting toad branches

**URL parsing:** `ExtractPRNumber` — used to extract PR numbers from URLs returned by `CreatePR`

## Architecture: Resolver Pattern

Unlike the agent and issue tracker (one instance per daemon), VCS uses a `Resolver` because each repo can have a different provider:

```go
type Resolver func(repoPath string) Provider
```

`NewResolver` pre-builds providers from per-repo configs, deduplicates identical configs to share instances, and Check()-s each unique provider once at startup.

Config example with per-repo overrides:
```yaml
vcs:
  platform: "github"          # global default

repos:
  - name: "main-app"
    path: "/code/main"        # uses global github

  - name: "internal-tool"
    path: "/code/internal"
    vcs:
      platform: "gitlab"      # override for this repo
      host: "gitlab.company.com"
```

## Adding a New Provider

### 1. Create the provider file

Create `internal/vcs/<name>.go` implementing all 14 `Provider` methods. Use `github.go` or `gitlab.go` as reference — they follow the same pattern of shelling out to a CLI tool and parsing JSON output.

```go
type BitbucketProvider struct {
    Host string
}

func (b *BitbucketProvider) Check() error {
    // Verify CLI tool is installed
}

func (b *BitbucketProvider) CreatePR(ctx context.Context, opts CreatePROpts) (string, error) {
    // Shell out to CLI or REST API
}

// ... implement remaining 12 methods
```

### 2. Register in the factory

In `provider.go`, add a case to `NewProvider`:

```go
case "bitbucket":
    return &BitbucketProvider{Host: cfg.Host}, nil
```

### 3. Add to config validation

In `internal/config/config.go`, add to both `validPlatforms` maps in `Validate()` and `ValidateRepos()`:

```go
validPlatforms := map[string]bool{"github": true, "gitlab": true, "bitbucket": true}
```

### 4. Write tests

See `github_test.go` / `gitlab_test.go` for the pattern. Key test areas:
- PR creation with labels
- CI status parsing
- Comment parsing with user types
- URL extraction (PR numbers, run IDs)
- `PRNoun()` return value

## Implementation Notes

- Both existing providers shell out to their respective CLI tools (`gh`, `glab`) rather than using REST APIs directly. This keeps auth simple — users configure the CLI tool once, and Toad inherits the credentials.
- `GetCIFailureLogs` truncates output to a reasonable size (included in retry prompts for fix tadpoles). Keep log output practical — a few hundred lines max.
- `PRNoun()` returns `"PR"` for GitHub and `"MR"` for GitLab. Used in user-facing Slack messages.
- `repoPath` is passed to most methods so the CLI tool runs in the correct repo context (`cmd.Dir`).
- The `Source` field on `PRComment` (`"review"` or `"issue"`) is GitHub-specific — it determines which API endpoint to use for reactions. New providers may not need this distinction.
