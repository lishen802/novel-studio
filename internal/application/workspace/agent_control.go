package workspace

import (
	"context"
	"fmt"
	"strings"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func (s *Service) beginAgentJob(parent context.Context, runID string) (context.Context, error) {
	if err := s.requireEditable(); err != nil {
		return nil, err
	}
	s.agentJobsMu.Lock()
	defer s.agentJobsMu.Unlock()
	if s.versionOperation {
		return nil, ErrAgentRunActive
	}
	if _, exists := s.agentJobs[runID]; exists {
		return nil, fmt.Errorf("%w: %s", ErrAgentRunActive, runID)
	}
	ctx, cancel := context.WithCancel(parent)
	s.agentJobs[runID] = cancel
	return ctx, nil
}

func (s *Service) endAgentJob(runID string) {
	s.agentJobsMu.Lock()
	cancel := s.agentJobs[runID]
	delete(s.agentJobs, runID)
	s.agentJobsMu.Unlock()
	if cancel != nil {
		cancel()
	}
	// Release the lease last so switch/close drain waits until the job has
	// finished all persistence (including the failed/cancelled exit path)
	// before closing the store. release is a no-op if the lease was never
	// acquired (e.g. beginAgentJob failed before the short-lock phase).
	s.leases.release(runID)
}

func (s *Service) cancelAllAgentJobs() {
	s.agentJobsMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.agentJobs))
	for _, cancel := range s.agentJobs {
		cancels = append(cancels, cancel)
	}
	s.agentJobsMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

// beginExtractionJob registers a cancellable context for an extraction run.
// It mirrors beginAgentJob so CancelExtractionRun can interrupt a long-running
// model call, while preventing two concurrent runs on the same task.
func (s *Service) beginExtractionJob(parent context.Context, taskID string) (context.Context, error) {
	if err := s.requireEditable(); err != nil {
		return nil, err
	}
	s.extractionJobsMu.Lock()
	defer s.extractionJobsMu.Unlock()
	if _, exists := s.extractionJobs[taskID]; exists {
		return nil, fmt.Errorf("%w: %s", ErrExtractionRunActive, taskID)
	}
	ctx, cancel := context.WithCancel(parent)
	s.extractionJobs[taskID] = cancel
	return ctx, nil
}

func (s *Service) endExtractionJob(taskID string) {
	s.extractionJobsMu.Lock()
	cancel := s.extractionJobs[taskID]
	delete(s.extractionJobs, taskID)
	s.extractionJobsMu.Unlock()
	if cancel != nil {
		cancel()
	}
	// Release the lease last so switch/close drain waits until the extraction
	// job has finished all persistence before closing the store.
	s.leases.release(taskID)
}

func (s *Service) cancelAllExtractionJobs() {
	s.extractionJobsMu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.extractionJobs))
	for _, cancel := range s.extractionJobs {
		cancels = append(cancels, cancel)
	}
	s.extractionJobsMu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (s *Service) beginVersionOperation() error {
	if err := s.requireEditable(); err != nil {
		return err
	}
	s.agentJobsMu.Lock()
	defer s.agentJobsMu.Unlock()
	if s.importOperation {
		return ErrImportOperationActive
	}
	if len(s.agentJobs) > 0 || s.versionOperation {
		return ErrAgentRunActive
	}
	s.versionOperation = true
	return nil
}

func (s *Service) beginImportOperation() error {
	if err := s.requireEditable(); err != nil {
		return err
	}
	s.agentJobsMu.Lock()
	defer s.agentJobsMu.Unlock()
	if s.importOperation || s.versionOperation {
		return ErrImportOperationActive
	}
	s.importOperation = true
	return nil
}

func (s *Service) endImportOperation() {
	s.agentJobsMu.Lock()
	s.importOperation = false
	s.agentJobsMu.Unlock()
}

func (s *Service) endVersionOperation() {
	s.agentJobsMu.Lock()
	s.versionOperation = false
	s.agentJobsMu.Unlock()
}

func (s *Service) CancelAgentRun(ctx context.Context, id string) error {
	if err := s.requireEditable(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	s.agentJobsMu.Lock()
	cancel := s.agentJobs[id]
	s.agentJobsMu.Unlock()
	if cancel != nil {
		cancel()
		return nil
	}
	return s.cancelAgentRunFallback(ctx, id)
}

// cancelAgentRunFallback handles cancellation for a queued/recoverable run
// that has no in-memory job. The active workspace can change after the
// caller's initial editable check, so this path must re-check under s.mu
// before it reads or persists through the store.
func (s *Service) cancelAgentRunFallback(ctx context.Context, id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.requireEditableLocked(); err != nil {
		return err
	}
	store, err := s.agentStoreLocked()
	if err != nil {
		return err
	}
	run, err := store.AgentRun(ctx, id)
	if err != nil {
		return err
	}
	if run.Status != agentdomain.RunQueued && run.Status != agentdomain.RunRecoverable {
		return fmt.Errorf("agent run cannot be cancelled from status %s", run.Status)
	}
	return s.persistCancelledRun(ctx, store, run)
}

func (s *Service) RetryAgentRun(ctx context.Context, id string) (AgentRunView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	store, err := s.editableAgentStoreLocked()
	if err != nil {
		return AgentRunView{}, err
	}
	original, err := store.AgentRun(ctx, strings.TrimSpace(id))
	if err != nil {
		return AgentRunView{}, err
	}
	switch original.Status {
	case agentdomain.RunFailed, agentdomain.RunCancelled, agentdomain.RunRecoverable:
	default:
		return AgentRunView{}, fmt.Errorf("agent run cannot be retried from status %s", original.Status)
	}
	return s.createChapterAgentRunLocked(ctx, original.ChapterID, original.Prompt, original.ID)
}
