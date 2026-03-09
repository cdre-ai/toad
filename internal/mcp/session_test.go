package mcp

import "testing"

func TestEmptySessionReturnsNil(t *testing.T) {
	s := NewSessionStore()
	if got := s.GetContext("user1"); got != nil {
		t.Fatalf("expected nil for unknown session, got %+v", got)
	}
}

func TestAddAndGetExchanges(t *testing.T) {
	s := NewSessionStore()
	s.AddExchange("user1", "what is toad?", "a daemon")
	s.AddExchange("user1", "how does it work?", "slack events")

	ctx := s.GetContext("user1")
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	if len(ctx.Exchanges) != 2 {
		t.Fatalf("expected 2 exchanges, got %d", len(ctx.Exchanges))
	}
	if ctx.Exchanges[0].Question != "what is toad?" {
		t.Errorf("unexpected first question: %s", ctx.Exchanges[0].Question)
	}
	if ctx.Exchanges[1].Answer != "slack events" {
		t.Errorf("unexpected second answer: %s", ctx.Exchanges[1].Answer)
	}
}

func TestClearSession(t *testing.T) {
	s := NewSessionStore()
	s.AddExchange("user1", "q", "a")
	s.Clear("user1")
	if got := s.GetContext("user1"); got != nil {
		t.Fatalf("expected nil after clear, got %+v", got)
	}
}

func TestSeparateSessions(t *testing.T) {
	s := NewSessionStore()
	s.AddExchange("user1", "q1", "a1")
	s.AddExchange("user2", "q2", "a2")

	ctx1 := s.GetContext("user1")
	ctx2 := s.GetContext("user2")
	if ctx1 == nil || ctx2 == nil {
		t.Fatal("expected non-nil contexts for both sessions")
	}
	if len(ctx1.Exchanges) != 1 || ctx1.Exchanges[0].Question != "q1" {
		t.Errorf("user1 context wrong: %+v", ctx1)
	}
	if len(ctx2.Exchanges) != 1 || ctx2.Exchanges[0].Question != "q2" {
		t.Errorf("user2 context wrong: %+v", ctx2)
	}

	// Clearing one doesn't affect the other.
	s.Clear("user1")
	if s.GetContext("user1") != nil {
		t.Error("user1 should be nil after clear")
	}
	if s.GetContext("user2") == nil {
		t.Error("user2 should still exist")
	}
}

func TestGetContextEmptyExchangesReturnsNil(t *testing.T) {
	s := NewSessionStore()
	// Manually create a session with empty exchanges.
	s.mu.Lock()
	s.sessions["user1"] = &SessionContext{}
	s.mu.Unlock()

	if got := s.GetContext("user1"); got != nil {
		t.Fatalf("expected nil for session with empty exchanges, got %+v", got)
	}
}
