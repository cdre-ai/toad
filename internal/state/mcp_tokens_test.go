package state

import (
	"testing"
	"time"
)

func TestMCPToken_SaveAndValidate(t *testing.T) {
	db := openTestDB(t)

	tok := &MCPToken{
		Token:       "tok-abc123",
		SlackUserID: "U12345",
		SlackUser:   "alice",
		Role:        "dev",
		CreatedAt:   time.Now().Truncate(time.Second),
	}
	if err := db.SaveMCPToken(tok); err != nil {
		t.Fatalf("save token: %v", err)
	}

	got, err := db.ValidateMCPToken("tok-abc123")
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if got == nil {
		t.Fatal("expected to find token")
	}
	if got.SlackUserID != "U12345" {
		t.Errorf("got SlackUserID %q, want %q", got.SlackUserID, "U12345")
	}
	if got.SlackUser != "alice" {
		t.Errorf("got SlackUser %q, want %q", got.SlackUser, "alice")
	}
	if got.Role != "dev" {
		t.Errorf("got Role %q, want %q", got.Role, "dev")
	}
	if got.LastUsedAt.IsZero() {
		t.Error("expected LastUsedAt to be set after validation")
	}
}

func TestMCPToken_ValidateInvalid(t *testing.T) {
	db := openTestDB(t)

	got, err := db.ValidateMCPToken("nonexistent-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for invalid token, got %+v", got)
	}
}

func TestMCPToken_Revoke(t *testing.T) {
	db := openTestDB(t)

	tok := &MCPToken{
		Token:       "tok-revoke",
		SlackUserID: "U99999",
		SlackUser:   "bob",
		Role:        "user",
		CreatedAt:   time.Now(),
	}
	if err := db.SaveMCPToken(tok); err != nil {
		t.Fatalf("save token: %v", err)
	}

	// Verify it exists
	got, err := db.ValidateMCPToken("tok-revoke")
	if err != nil {
		t.Fatalf("validate before revoke: %v", err)
	}
	if got == nil {
		t.Fatal("expected token to exist before revoke")
	}

	// Revoke
	if err := db.RevokeMCPToken("U99999"); err != nil {
		t.Fatalf("revoke token: %v", err)
	}

	// Verify it's gone
	got, err = db.ValidateMCPToken("tok-revoke")
	if err != nil {
		t.Fatalf("validate after revoke: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil after revoke, got %+v", got)
	}
}

func TestMCPToken_SaveReplace(t *testing.T) {
	db := openTestDB(t)

	tok := &MCPToken{
		Token:       "tok-replace",
		SlackUserID: "U111",
		SlackUser:   "charlie",
		Role:        "user",
		CreatedAt:   time.Now(),
	}
	if err := db.SaveMCPToken(tok); err != nil {
		t.Fatalf("save token: %v", err)
	}

	// Save again with updated role
	tok.Role = "dev"
	if err := db.SaveMCPToken(tok); err != nil {
		t.Fatalf("save token replace: %v", err)
	}

	got, err := db.ValidateMCPToken("tok-replace")
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if got.Role != "dev" {
		t.Errorf("got Role %q after replace, want %q", got.Role, "dev")
	}
}
