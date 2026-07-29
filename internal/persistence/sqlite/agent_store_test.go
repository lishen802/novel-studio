package sqlite

import (
	"context"
	"testing"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestAgentStorePersistsConfigurationRunAndArtifacts(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(100, 0).UTC()

	connection, err := agentdomain.NewProviderConnection(
		"provider-1", project.ID, "OpenAI", agentdomain.ProviderOpenAI,
		"https://api.openai.com/v1", "provider:provider-1", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertProviderConnection(ctx, connection); err != nil {
		t.Fatal(err)
	}
	profile, err := agentdomain.NewAgentProfile(
		"profile-1", project.ID, agentdomain.RolePlanner, connection.ID,
		"gpt-test", 1024, 0.3, true, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertAgentProfile(ctx, profile); err != nil {
		t.Fatal(err)
	}
	run, err := agentdomain.NewChapterDraftRun(
		"run-1", project.ID, chapter.ID, chapter.Revision, "续写",
		[]agentdomain.RunStepSpec{
			{ID: "step-1", Role: agentdomain.RolePlanner, ProfileID: profile.ID},
			{ID: "step-2", Role: agentdomain.RoleWriter, ProfileID: profile.ID},
			{ID: "step-3", Role: agentdomain.RoleContinuityReviewer, ProfileID: profile.ID},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	artifact := agentdomain.AgentArtifact{
		ID: "artifact-1", RunID: run.ID, StepID: run.Steps[0].ID,
		Kind: agentdomain.ArtifactPlan, Content: `{"summary":"计划"}`,
		SHA256: "hash", CreatedAt: now,
	}
	if err := store.CreateAgentArtifact(ctx, artifact); err != nil {
		t.Fatal(err)
	}

	gotRun, err := store.AgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotRun.Steps) != 3 || gotRun.Steps[1].Role != agentdomain.RoleWriter {
		t.Fatalf("unexpected run: %#v", gotRun)
	}
	artifacts, err := store.ListAgentArtifacts(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 || artifacts[0].Content != artifact.Content {
		t.Fatalf("unexpected artifacts: %#v", artifacts)
	}
}

func TestAgentStoreRecoversInterruptedRuns(t *testing.T) {
	ctx := context.Background()
	store, project, chapter := createStoreWithChapter(t)
	defer store.Close()
	now := time.Unix(100, 0).UTC()
	connection, err := agentdomain.NewProviderConnection(
		"provider-1", project.ID, "OpenAI", agentdomain.ProviderOpenAI,
		"https://api.openai.com/v1", "provider:provider-1", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertProviderConnection(ctx, connection); err != nil {
		t.Fatal(err)
	}
	for index, role := range []agentdomain.AgentRole{
		agentdomain.RolePlanner,
		agentdomain.RoleWriter,
		agentdomain.RoleContinuityReviewer,
	} {
		profile, err := agentdomain.NewAgentProfile(
			"profile-"+string(rune('1'+index)), project.ID, role, connection.ID,
			"model", 100, 0.2, true, now,
		)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.UpsertAgentProfile(ctx, profile); err != nil {
			t.Fatal(err)
		}
	}
	run, err := agentdomain.NewChapterDraftRun(
		"run-1", project.ID, chapter.ID, chapter.Revision, "续写",
		[]agentdomain.RunStepSpec{
			{ID: "step-1", Role: agentdomain.RolePlanner, ProfileID: "profile-1"},
			{ID: "step-2", Role: agentdomain.RoleWriter, ProfileID: "profile-2"},
			{ID: "step-3", Role: agentdomain.RoleContinuityReviewer, ProfileID: "profile-3"},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	run, _ = run.Transition(agentdomain.RunRunning, "", now.Add(time.Second))
	if err := store.UpdateAgentRun(ctx, run); err != nil {
		t.Fatal(err)
	}

	count, err := store.RecoverInterruptedAgentRuns(ctx, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("recovered count = %d", count)
	}
	got, err := store.AgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != agentdomain.RunRecoverable {
		t.Fatalf("status = %q", got.Status)
	}
}
