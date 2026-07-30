package ports

import (
	"context"
	"encoding/json"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

type CredentialStore interface {
	Save(ctx context.Context, reference, secret string) error
	Get(ctx context.Context, reference string) (string, error)
	Delete(ctx context.Context, reference string) error
}

type ModelGenerateRequest struct {
	Model           string
	System          string
	Input           string
	MaxOutputTokens int
	Temperature     float64
	JSONSchema      json.RawMessage
	SchemaName      string
}

type ModelGenerateResult struct {
	Text              string
	ProviderRequestID string
	InputTokens       int
	OutputTokens      int
}

type ModelProvider interface {
	Generate(
		ctx context.Context,
		apiKey string,
		request ModelGenerateRequest,
		onDelta func(string),
	) (ModelGenerateResult, error)
}

type ModelProviderFactory interface {
	New(connection agentdomain.ProviderConnection) (ModelProvider, error)
}

type AgentStore interface {
	SaveAgentRuntimeSettings(ctx context.Context, settings agentdomain.RuntimeSettings) error
	AgentRuntimeSettings(ctx context.Context, projectID string) (agentdomain.RuntimeSettings, error)
	UpsertProviderConnection(ctx context.Context, connection agentdomain.ProviderConnection) error
	ListProviderConnections(ctx context.Context) ([]agentdomain.ProviderConnection, error)
	ProviderConnection(ctx context.Context, id string) (agentdomain.ProviderConnection, error)
	DeleteProviderConnection(ctx context.Context, id string) error
	UpsertAgentProfile(ctx context.Context, profile agentdomain.AgentProfile) error
	ListAgentProfiles(ctx context.Context) ([]agentdomain.AgentProfile, error)
	AgentProfile(ctx context.Context, id string) (agentdomain.AgentProfile, error)
	CreateAgentRun(ctx context.Context, run agentdomain.AgentRun) error
	UpdateAgentRun(ctx context.Context, run agentdomain.AgentRun) error
	AgentRun(ctx context.Context, id string) (agentdomain.AgentRun, error)
	ListAgentRuns(ctx context.Context, limit int) ([]agentdomain.AgentRun, error)
	UpdateAgentRunStep(ctx context.Context, step agentdomain.AgentRunStep) error
	CreateAgentArtifact(ctx context.Context, artifact agentdomain.AgentArtifact) error
	ListAgentArtifacts(ctx context.Context, runID string) ([]agentdomain.AgentArtifact, error)
	RecoverInterruptedAgentRuns(ctx context.Context, recoveredAt time.Time) (int64, error)
}
