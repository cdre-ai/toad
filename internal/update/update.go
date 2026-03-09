// Package update checks for new toad releases on GitHub.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Info holds the result of a version check.
type Info struct {
	Current    string
	Latest     string
	Available  bool   // true if Latest > Current
	ReleaseURL string // link to the release page
}

// githubRelease is the subset of the GitHub API release response we need.
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

const releaseURL = "https://api.github.com/repos/scaler-tech/toad/releases/latest"

// Cached ETag and last response to avoid rate limits.
// GitHub 304 responses don't count against the rate limit.
var (
	cacheMu   sync.Mutex
	cacheETag string
	cacheLast *githubRelease
)

// Check fetches the latest release from GitHub and compares to current.
// Returns nil Info (no error) if currentVersion is "dev" (local build).
func Check(currentVersion string) (*Info, error) {
	if currentVersion == "dev" || currentVersion == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	release, err := fetchLatestRelease(ctx)
	if err != nil {
		return nil, err
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(currentVersion, "v")

	return &Info{
		Current:    current,
		Latest:     latest,
		Available:  isNewer(latest, current),
		ReleaseURL: release.HTMLURL,
	}, nil
}

func fetchLatestRelease(ctx context.Context) (githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return githubRelease{}, fmt.Errorf("failed to create request: %w", err)
	}

	// Use conditional request to avoid rate limits.
	// GitHub 304 responses don't count against the rate limit.
	cacheMu.Lock()
	etag := cacheETag
	cached := cacheLast
	cacheMu.Unlock()
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return githubRelease{}, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	// 304 Not Modified — return cached response (doesn't count against rate limit)
	if resp.StatusCode == http.StatusNotModified && cached != nil {
		return *cached, nil
	}

	if resp.StatusCode != http.StatusOK {
		return githubRelease{}, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return githubRelease{}, fmt.Errorf("failed to parse release info: %w", err)
	}

	// Cache the ETag and response for next request
	if newETag := resp.Header.Get("ETag"); newETag != "" {
		cacheMu.Lock()
		cacheETag = newETag
		cacheLast = &release
		cacheMu.Unlock()
	}

	return release, nil
}

// isNewer returns true if version a is newer than version b.
// Handles semver with pre-release tags: 1.0.0 > 1.0.0-beta.4 > 1.0.0-beta.3.
func isNewer(a, b string) bool {
	aParts, aPre := parseSemver(a)
	bParts, bPre := parseSemver(b)

	for i := 0; i < 3; i++ {
		if aParts[i] > bParts[i] {
			return true
		}
		if aParts[i] < bParts[i] {
			return false
		}
	}

	// Same major.minor.patch — compare pre-release.
	// No pre-release (stable) beats any pre-release.
	if aPre == "" && bPre != "" {
		return true
	}
	if aPre != "" && bPre == "" {
		return false
	}
	// Both have pre-release: compare numerically (beta.4 > beta.3)
	return preReleaseNum(aPre) > preReleaseNum(bPre)
}

func preReleaseNum(pre string) int {
	if idx := strings.LastIndex(pre, "."); idx >= 0 {
		if n, err := strconv.Atoi(pre[idx+1:]); err == nil {
			return n
		}
	}
	return 0
}

func parseSemver(v string) ([3]int, string) {
	var parts [3]int
	pre := ""
	segments := strings.SplitN(v, ".", 3)
	for i := 0; i < len(segments) && i < 3; i++ {
		seg := segments[i]
		if dashIdx := strings.Index(seg, "-"); dashIdx >= 0 {
			pre = seg[dashIdx+1:]
			if i < len(segments)-1 {
				pre = pre + "." + strings.Join(segments[i+1:], ".")
			}
			seg = seg[:dashIdx]
			n, err := strconv.Atoi(seg)
			if err == nil {
				parts[i] = n
			}
			break
		}
		n, err := strconv.Atoi(seg)
		if err == nil {
			parts[i] = n
		}
	}
	return parts, pre
}
