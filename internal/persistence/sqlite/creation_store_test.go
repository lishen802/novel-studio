package sqlite

import (
	"context"
	"testing"
	"time"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	storyassetdomain "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

func TestCreationStoreRoundTripsSessionCandidateAndAsset(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := time.Unix(500, 0).UTC()

	session, err := creationdomain.NewSession("session-1", project.ID, "branch-1", now)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SaveIntent("江心岛灯塔", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	loadedSession, err := store.CreationSession(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedSession.IntentText != "江心岛灯塔" || loadedSession.Stage != creationdomain.StageIntent {
		t.Fatalf("session = %#v", loadedSession)
	}

	candidate, err := creationdomain.NewCandidate(
		"cand-1", session.ID, creationdomain.StageDirections, creationdomain.KindDirectionSet,
		1, "", "run-1", "art-1", `{"directions":[{"id":"a"}]}`, "story_directions.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	loadedCandidate, err := store.CreationCandidate(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedCandidate.ContentSHA256 != candidate.ContentSHA256 || loadedCandidate.RunID != "run-1" {
		t.Fatalf("candidate = %#v", loadedCandidate)
	}

	asset, err := storyassetdomain.New(
		"asset-1", project.ID, storyassetdomain.KindProposal, storyassetdomain.OriginModel,
		candidate.ID, "run-1", "art-1", "", "", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStoryAsset(ctx, asset); err != nil {
		t.Fatal(err)
	}
	loadedAsset, err := store.StoryAsset(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedAsset.Kind != storyassetdomain.KindProposal || loadedAsset.TypedTable != "" {
		t.Fatalf("asset = %#v", loadedAsset)
	}
}

func TestAssistantActionPersistsPendingThenExecuting(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := time.Unix(600, 0).UTC()
	if err := store.SaveAssistantSession(ctx, "asst-1", project.ID, "branch-1", formatTime(now)); err != nil {
		t.Fatal(err)
	}
	action, err := assistantdomain.NewAction(
		"act-1", "asst-1", project.ID, "branch-1",
		assistantdomain.OpReviseFields, assistantdomain.RiskPaid,
		[]string{"candidate:cand-1"}, []string{"cand-1@1"},
		`{"paths":["/title"]}`, "hash-1", "idem-1", "0.02", now, now.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAssistantAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	executing, err := action.Confirm("hash-1", "idem-1", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAssistantAction(ctx, executing); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.AssistantAction(ctx, action.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != assistantdomain.StateExecuting || loaded.IdempotencyKey != "idem-1" {
		t.Fatalf("action = %#v", loaded)
	}
}
