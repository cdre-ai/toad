package mcp

import "sync"

// Exchange records a single question-answer pair.
type Exchange struct {
	Question string
	Answer   string
}

// SessionContext holds conversation history for a single user session.
type SessionContext struct {
	Exchanges []Exchange
}

// SessionStore manages per-user conversation context for the ask tool.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*SessionContext
}

// NewSessionStore creates a new empty session store.
func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string]*SessionContext)}
}

// GetContext returns the session context for the given session ID.
// Returns nil if the session does not exist or has no exchanges.
func (s *SessionStore) GetContext(sessionID string) *SessionContext {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ctx := s.sessions[sessionID]
	if ctx != nil && len(ctx.Exchanges) == 0 {
		return nil
	}
	return ctx
}

// AddExchange appends a question-answer pair to the session.
func (s *SessionStore) AddExchange(sessionID, question, answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx, ok := s.sessions[sessionID]
	if !ok {
		ctx = &SessionContext{}
		s.sessions[sessionID] = ctx
	}
	ctx.Exchanges = append(ctx.Exchanges, Exchange{Question: question, Answer: answer})
}

// Clear removes all context for the given session.
func (s *SessionStore) Clear(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}
