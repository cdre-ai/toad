---
allowed-tools: Bash(git add:*), Bash(git status:*), Bash(git commit:*), Bash(git tag:*), Bash(git push:*), Bash(gofmt:*), Bash(go build:*), Bash(go test:*), Bash(go vet:*), Bash(golangci-lint:*)
description: Commit, tag, and push a release
argument-hint: "[commit message]"
---

## Context

- Current git status: !`git status`
- Current git diff (staged and unstaged changes): !`git diff HEAD`
- Current branch: !`git branch --show-current`
- Recent commits: !`git log --oneline -5`
- Latest tag: !`git tag --sort=-v:refname | head -1`
- All recent tags: !`git tag --sort=-v:refname | head -5`

## Your task

Create a release: commit all changes, tag with the next version, and push.

### Steps

1. **Pre-flight checks**: Run `gofmt -l .`, `go build ./...`, `go test ./...`, `go vet ./...`, and `golangci-lint run ./...`. If any fail, stop and report the issue. Do NOT skip these checks.

2. **Determine next version**: Look at the latest tag above. Increment the patch version (e.g. `v0.1.32` -> `v0.1.33`). The tag format is always `vMAJOR.MINOR.PATCH`.

3. **Stage all changes**: Use `git add` to stage the modified files shown in the status above. Prefer adding specific files by name rather than `git add .` or `git add -A`.

4. **Create a commit**: Write a concise commit message summarizing the changes. If the user provided a commit message argument, use that instead. Do NOT include any `Co-Authored-By` lines or mention Claude/AI in the commit message. Use a HEREDOC for the message.

5. **Create the tag**: Tag the new commit with the next version (e.g. `git tag v0.1.33`).

6. **Push**: Push the commit and tag together: `git push && git push --tags`.

7. **Report**: Output the new version tag and a brief summary of what was released.

### Rules

- NEVER include `Co-Authored-By` lines in the commit message
- NEVER mention Claude, AI, or any assistant in the commit message
- ALWAYS run pre-flight checks before committing
- ALWAYS use the next patch version unless the user explicitly requests a different version bump
- You have the capability to call multiple tools in a single response. Use parallel calls where possible (e.g. the four pre-flight checks can run in parallel).
