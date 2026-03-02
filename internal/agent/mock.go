package agent

import (
	"context"
	"sync"
)

// MockProvider is a test double that records calls and returns configurable results.
type MockProvider struct {
	mu sync.Mutex

	// RunResult is returned by Run. Set before calling the code under test.
	RunResult *RunResult
	// RunErr is returned as the error from Run.
	RunErr error
	// ResumeResult is returned by Resume.
	ResumeResult *RunResult
	// ResumeErr is returned as the error from Resume.
	ResumeErr error

	// RunCalls records every RunOpts passed to Run, in order.
	RunCalls []RunOpts
	// ResumeCalls records every Resume invocation.
	ResumeCalls []ResumeCall
}

// ResumeCall records the arguments to a Resume invocation.
type ResumeCall struct {
	SessionID string
	Prompt    string
	WorkDir   string
}

func (m *MockProvider) Check() error { return nil }

func (m *MockProvider) Run(_ context.Context, opts RunOpts) (*RunResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RunCalls = append(m.RunCalls, opts)
	return m.RunResult, m.RunErr
}

func (m *MockProvider) Resume(_ context.Context, sessionID, prompt, workDir string) (*RunResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ResumeCalls = append(m.ResumeCalls, ResumeCall{
		SessionID: sessionID,
		Prompt:    prompt,
		WorkDir:   workDir,
	})
	return m.ResumeResult, m.ResumeErr
}

// LastRunOpts returns the RunOpts from the most recent Run call, or zero value if none.
func (m *MockProvider) LastRunOpts() RunOpts {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.RunCalls) == 0 {
		return RunOpts{}
	}
	return m.RunCalls[len(m.RunCalls)-1]
}
