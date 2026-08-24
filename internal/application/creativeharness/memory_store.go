package creativeharness

import (
	"context"
	"errors"
	"sync"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// memoryAgentStore 隔离单个 Harness case 的 Agent Kernel 轨迹，避免写入项目库。
type memoryAgentStore struct {
	mu        sync.Mutex
	runs      map[string]agentdomain.Run
	artifacts []agentdomain.Artifact
}

func newMemoryAgentStore() *memoryAgentStore {
	return &memoryAgentStore{runs: make(map[string]agentdomain.Run)}
}

func (store *memoryAgentStore) CreateAgentRunBundle(_ context.Context, bundle agentdomain.RunBundle) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.runs[bundle.Run.ID] = cloneRun(bundle.Run)
	store.artifacts = append(store.artifacts, bundle.Artifacts...)
	return nil
}

func (store *memoryAgentStore) AgentRun(_ context.Context, id string) (agentdomain.Run, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	run, ok := store.runs[id]
	if !ok {
		return agentdomain.Run{}, errors.New("实验运行不存在")
	}
	return cloneRun(run), nil
}

func (store *memoryAgentStore) UpdateAgentRun(_ context.Context, run agentdomain.Run, _ ...agentdomain.JobEventDraft) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.runs[run.ID] = cloneRun(run)
	return nil
}

func (store *memoryAgentStore) UpdateAgentRunStep(_ context.Context, step agentdomain.RunStep, _ ...agentdomain.JobEventDraft) error {
	return store.saveStep(step, nil)
}

func (store *memoryAgentStore) AppendAgentRunStep(_ context.Context, step agentdomain.RunStep, _ ...agentdomain.JobEventDraft) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	run := store.runs[step.RunID]
	run.Steps = append(run.Steps, step)
	store.runs[step.RunID] = cloneRun(run)
	return nil
}

func (store *memoryAgentStore) SaveAgentStepBundle(_ context.Context, bundle agentdomain.StepBundle, _ ...agentdomain.JobEventDraft) error {
	return store.saveStep(bundle.Step, bundle.Artifacts)
}

func (store *memoryAgentStore) saveStep(step agentdomain.RunStep, artifacts []agentdomain.Artifact) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	run := store.runs[step.RunID]
	for index := range run.Steps {
		if run.Steps[index].ID == step.ID {
			run.Steps[index] = step
			store.runs[step.RunID] = cloneRun(run)
			store.artifacts = append(store.artifacts, artifacts...)
			return nil
		}
	}
	return errors.New("实验步骤不存在")
}

func (store *memoryAgentStore) ListAgentArtifacts(_ context.Context, runID string) ([]agentdomain.Artifact, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var result []agentdomain.Artifact
	for _, artifact := range store.artifacts {
		if artifact.RunID == runID {
			result = append(result, artifact)
		}
	}
	return result, nil
}

func cloneRun(run agentdomain.Run) agentdomain.Run {
	run.Steps = append([]agentdomain.RunStep(nil), run.Steps...)
	return run
}
