package workspace

import (
	"context"
	"fmt"
	"strings"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func (s *Service) beginAgentJob(parent context.Context, runID string) (context.Context, error) {
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

func (s *Service) beginVersionOperation() error {
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
	id = strings.TrimSpace(id)
	s.agentJobsMu.Lock()
	cancel := s.agentJobs[id]
	s.agentJobsMu.Unlock()
	if cancel != nil {
		cancel()
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
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
	store, err := s.agentStoreLocked()
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
