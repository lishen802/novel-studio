package agent

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewWorkflowRunFreezesArbitraryNodeList(t *testing.T) {
	now := time.Unix(1_000, 0).UTC()
	hash := strings.Repeat("a", 64)
	run, err := NewWorkflowRun(
		"run", "project", "chapter", 1, "写作", VersionBinding{ID: "workflow", Version: 1, Hash: hash},
		TaskBaselineDraft, "head", hash, 1, 1, 0, 6_000, 10_000,
		[]RunStepSpec{{
			ID: "step", Role: RoleWriter, CreativeProfileID: "profile", CreativeProfileVersion: 1,
			ProviderConnectionID: "connection", ProviderName: "连接", ProviderKind: ProviderOpenAI,
			BaseURL: "https://api.example.com/v1", CredentialRef: "credential", Model: "model",
			MaxOutputTokens: 4_000, Temperature: 0.5, NodeID: "write", PublicStageID: PublicStageProseWriting, Capability: CapabilityProseWriter,
			PromptBinding: VersionBinding{ID: "prompt", Version: 1, Hash: hash},
			SchemaBinding: VersionBinding{ID: "schema", Version: 1, Hash: hash},
			AssemblerID:   "assembler", AssemblerVersion: 1, InputDigest: hash,
		}}, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if run.IsLegacy() || len(run.Steps) != 1 || run.Steps[0].Capability != CapabilityProseWriter {
		t.Fatalf("run = %#v", run)
	}
	resumed, err := run.Transition(RunRunning, "", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resumed, err = resumed.Transition(RunRecoverable, "中断", now.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := resumed.Transition(RunRunning, "", now.Add(3*time.Second)); err != nil {
		t.Fatalf("recoverable run cannot resume: %v", err)
	}
}

func TestNewProviderConnectionValidatesKindAndEndpoint(t *testing.T) {
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)

	connection, err := NewProviderConnection(
		"provider-1",
		"project-1",
		"OpenAI",
		ProviderOpenAI,
		"https://api.openai.com/v1",
		"credential:provider-1",
		now,
	)
	if err != nil {
		t.Fatalf("NewProviderConnection() error = %v", err)
	}
	if connection.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("BaseURL = %q", connection.BaseURL)
	}

	_, err = NewProviderConnection(
		"provider-2",
		"project-1",
		"unsafe",
		ProviderOpenAI,
		"http://example.com/v1",
		"credential:provider-2",
		now,
	)
	if !errors.Is(err, ErrInsecureEndpoint) {
		t.Fatalf("expected ErrInsecureEndpoint, got %v", err)
	}

	if _, err := NewProviderConnection(
		"provider-3",
		"project-1",
		"local",
		ProviderOpenAI,
		"http://127.0.0.1:11434/v1",
		"credential:provider-3",
		now,
	); err != nil {
		t.Fatalf("loopback endpoint should be accepted: %v", err)
	}
}

func TestNewAgentProfileAcceptsOnlySupportedRoles(t *testing.T) {
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	profile, err := NewAgentProfile(
		"profile-1",
		"project-1",
		RoleWriter,
		"provider-1",
		"gpt-5",
		3200,
		0.7,
		true,
		now,
	)
	if err != nil {
		t.Fatalf("NewAgentProfile() error = %v", err)
	}
	if profile.Role != RoleWriter || profile.MaxOutputTokens != 3200 {
		t.Fatalf("unexpected profile: %#v", profile)
	}

	_, err = NewAgentProfile(
		"profile-2",
		"project-1",
		Role("critic"),
		"provider-1",
		"gpt-5",
		3200,
		0.7,
		true,
		now,
	)
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("expected ErrInvalidRole, got %v", err)
	}
}

func TestNewChapterDraftRunBuildsOrderedStepsAndTransitions(t *testing.T) {
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	run, err := NewChapterDraftRun(
		"run-1",
		"project-1",
		"chapter-1",
		2,
		"prompt",
		[]RunStepSpec{
			{ID: "step-1", Role: RolePlanner, ProfileID: "planner"},
			{ID: "step-2", Role: RoleWriter, ProfileID: "writer"},
			{ID: "step-3", Role: RoleContinuityReviewer, ProfileID: "reviewer"},
		},
		now,
	)
	if err != nil {
		t.Fatalf("NewChapterDraftRun() error = %v", err)
	}
	if run.Status != RunQueued || len(run.Steps) != 3 {
		t.Fatalf("unexpected new run: %#v", run)
	}
	for index, step := range run.Steps {
		if step.Position != index+1 || step.Status != StepQueued {
			t.Fatalf("unexpected step %d: %#v", index, step)
		}
	}

	running, err := run.Transition(RunRunning, "", now.Add(time.Second))
	if err != nil {
		t.Fatalf("Transition(running) error = %v", err)
	}
	completed, err := running.Transition(RunCompleted, "", now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("Transition(completed) error = %v", err)
	}
	if completed.CompletedAt == nil {
		t.Fatal("completed run must have CompletedAt")
	}
	if _, err := completed.Transition(RunRunning, "", now.Add(3*time.Second)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestValidateHeterogeneousReviewer(t *testing.T) {
	profiles := []Profile{
		{Role: RolePlanner, ConnectionID: "provider-a", Model: "model-a", Enabled: true},
		{Role: RoleWriter, ConnectionID: "provider-a", Model: "model-a", Enabled: true},
		{Role: RoleContinuityReviewer, ConnectionID: "provider-a", Model: "model-a", Enabled: true},
	}
	if err := ValidateAuthoringProfiles(profiles); !errors.Is(err, ErrReviewerNotIndependent) {
		t.Fatalf("expected ErrReviewerNotIndependent, got %v", err)
	}

	profiles[2].Model = "model-b"
	if err := ValidateAuthoringProfiles(profiles); err != nil {
		t.Fatalf("different model should satisfy heterogeneous review: %v", err)
	}
}

func TestNewRuntimeSettingsValidatesBudgets(t *testing.T) {
	now := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	settings, err := NewRuntimeSettings("project-1", 30000, 120000, now)
	if err != nil {
		t.Fatalf("NewRuntimeSettings() error = %v", err)
	}
	if settings.MaxRunTokens != 30000 || settings.MaxContextChars != 120000 {
		t.Fatalf("unexpected settings: %#v", settings)
	}
	if _, err := NewRuntimeSettings("project-1", 0, 120000, now); !errors.Is(err, ErrInvalidBudget) {
		t.Fatalf("expected ErrInvalidBudget, got %v", err)
	}
}

func TestRunRecordsUsageAndBudgetSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	run, err := NewChapterDraftRun(
		"run-1",
		"project-1",
		"chapter-1",
		1,
		"prompt",
		[]RunStepSpec{
			{ID: "step-1", Role: RolePlanner, ProfileID: "planner"},
			{ID: "step-2", Role: RoleWriter, ProfileID: "writer"},
			{ID: "step-3", Role: RoleContinuityReviewer, ProfileID: "reviewer"},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	run, err = run.WithBudgets(8000, 64000)
	if err != nil {
		t.Fatalf("WithBudgets() error = %v", err)
	}
	run = run.RecordUsage(1200, 800)
	if run.TokenBudget != 8000 || run.ContextCharBudget != 64000 ||
		run.InputTokens != 1200 || run.OutputTokens != 800 {
		t.Fatalf("unexpected run usage: %#v", run)
	}
	if run.RemainingTokens() != 6000 {
		t.Fatalf("RemainingTokens() = %d", run.RemainingTokens())
	}
}

func TestUnlimitedRunKeepsSentinelOutOfRemainingTokensIdentity(t *testing.T) {
	run := Run{TokenBudget: 8, TokenBudgetUnlimited: true, ContextCharBudget: 8, ContextBudgetUnlimited: true}
	if run.RemainingTokens() != UnlimitedBudgetSentinel || run.RemainingContextChars() != UnlimitedBudgetSentinel {
		t.Fatalf("remaining = %d/%d", run.RemainingTokens(), run.RemainingContextChars())
	}
}
