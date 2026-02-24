package reviewer

import (
	"strings"
	"testing"
)

func TestExtractPRNumber_Valid(t *testing.T) {
	n, err := ExtractPRNumber("https://github.com/owner/repo/pull/42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 42 {
		t.Errorf("expected 42, got %d", n)
	}
}

func TestExtractPRNumber_TrailingSlash(t *testing.T) {
	n, err := ExtractPRNumber("https://github.com/owner/repo/pull/123/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 123 {
		t.Errorf("expected 123, got %d", n)
	}
}

func TestExtractPRNumber_LargeNumber(t *testing.T) {
	n, err := ExtractPRNumber("https://github.com/scaler-tech/scaler-mono/pull/9224")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 9224 {
		t.Errorf("expected 9224, got %d", n)
	}
}

func TestExtractPRNumber_NotAPRURL(t *testing.T) {
	_, err := ExtractPRNumber("https://github.com/owner/repo/issues/42")
	if err == nil {
		t.Error("expected error for non-PR URL")
	}
}

func TestExtractPRNumber_TooShort(t *testing.T) {
	_, err := ExtractPRNumber("https://github.com")
	if err == nil {
		t.Error("expected error for short URL")
	}
}

func TestExtractPRNumber_InvalidNumber(t *testing.T) {
	_, err := ExtractPRNumber("https://github.com/owner/repo/pull/abc")
	if err == nil {
		t.Error("expected error for non-numeric PR number")
	}
}

func TestExtractPRNumber_EmptyString(t *testing.T) {
	_, err := ExtractPRNumber("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestExtractRunID(t *testing.T) {
	tests := []struct {
		url    string
		expect string
	}{
		{"https://github.com/owner/repo/actions/runs/12345/job/67890", "12345"},
		{"https://github.com/owner/repo/actions/runs/99999", "99999"},
		{"https://github.com/owner/repo/actions/runs/12345/", "12345"},
		{"https://github.com/owner/repo/pull/42", ""},
		{"https://example.com/not-github", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractRunID(tt.url)
		if got != tt.expect {
			t.Errorf("extractRunID(%q) = %q, want %q", tt.url, got, tt.expect)
		}
	}
}

func TestExtractResultText(t *testing.T) {
	// Wrapped in --output-format json
	wrapped := `{"result":"{\"actionable\":true,\"summary\":\"fix typo\",\"reason\":\"code change requested\"}"}`
	got := extractResultText([]byte(wrapped))
	if !strings.Contains(got, "actionable") {
		t.Errorf("expected JSON content, got: %s", got)
	}

	// Plain text fallback
	plain := `{"actionable":false,"summary":"","reason":"just an approval"}`
	got = extractResultText([]byte(plain))
	if !strings.Contains(got, "approval") {
		t.Errorf("expected plain JSON, got: %s", got)
	}
}
