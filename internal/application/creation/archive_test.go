package creation

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
)

func TestConfirmArchiveRejectsWithoutSelectedDirection(t *testing.T) {
	service, _, _ := newTestService(t, newFakeKernel(), nil)
	session := seedIntentSession(t, service)
	if _, err := service.PreviewConfirmArchive(context.Background(), session.ProjectID); !errors.Is(err, creationdomain.ErrStageIncomplete) {
		t.Fatalf("preview without direction error = %v", err)
	}
	if _, err := service.ConfirmArchive(context.Background(), ArchiveRequest{
		ProjectID: session.ProjectID, ConfirmationDigest: "x", IdempotencyKey: "idem-1",
	}); !errors.Is(err, creationdomain.ErrStageIncomplete) {
		t.Fatalf("confirm without direction error = %v", err)
	}
}

func TestConfirmArchiveWritesAssetsIdempotently(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	seedArchiveSession(t, bridge, project.ID)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{
		Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Title != "潮汐关" || preview.CharacterCount != 1 || preview.OutlineCount < 4 {
		t.Fatalf("preview = %#v", preview)
	}
	first, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 1 || characters[0].Name != "阿绫" {
		t.Fatalf("characters = %#v, err=%v", characters, err)
	}
	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil || len(nodes) < 4 {
		t.Fatalf("outline = %#v, err=%v", nodes, err)
	}
	levels := map[string]string{}
	for _, node := range nodes {
		levels[node.ID] = string(node.Level)
	}
	for _, node := range nodes {
		if string(node.Level) == "chapter" && levels[node.ParentID] != "arc" {
			t.Fatalf("chapter %q parent level = %q, want arc", node.Title, levels[node.ParentID])
		}
	}
	updated, err := raw.Project(ctx)
	if err != nil || updated.Title != "潮汐关" {
		t.Fatalf("project = %#v, err=%v", updated, err)
	}
	workbench, err := raw.WorkbenchState(ctx)
	if err != nil || workbench.Stage != projectdomain.StageWriting || workbench.ArchiveStatus != "confirmed" {
		t.Fatalf("workbench = %#v, err=%v", workbench, err)
	}
	second, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.ChangeSetID != first.ChangeSetID {
		t.Fatalf("idempotent change set = %q vs %q", second.ChangeSetID, first.ChangeSetID)
	}
	characters, err = raw.ListCharacters(ctx)
	if err != nil || len(characters) != 1 {
		t.Fatalf("characters after replay = %#v, err=%v", characters, err)
	}
}

func TestApplyCreationArchiveRollsBackPartialFailure(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	now := testClock{}.Now()
	session, err := creationdomain.NewSession("session-1", project.ID, "branch-1", now)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SaveIntent("守关人必须放走仇敌", now)
	if err != nil {
		t.Fatal(err)
	}
	session.Stage = creationdomain.StageArchiveConfirm
	session.SelectedDirectionCandidateID = "dir-1"
	changeSet, err := revisiondomain.NewChangeSet(
		"cs-fail", project.ID, "部分失败", "", "author",
		[]revisiondomain.PatchOperationSpec{
			{
				ID: "op-char", Kind: revisiondomain.OperationCreateCharacter, TargetID: "char-1",
				AfterValue: `{"name":"阿绫","role":"protagonist"}`, Reason: "人物",
			},
			{
				ID: "op-outline", Kind: revisiondomain.OperationCreateOutlineNode, TargetID: "outline-1",
				AfterValue: `{"parentId":"missing-parent","level":"volume","title":"坏节点","position":1}`,
				Reason:     "失败大纲",
			},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.RecordArchive("digest", "idem-fail", changeSet.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	err = raw.ApplyCreationArchive(ctx, changeSet, session, nil, projectdomain.WorkbenchState{
		Stage: projectdomain.StageWriting, ArchiveStatus: "confirmed", UpdatedAt: now,
	})
	if err == nil {
		t.Fatal("expected archive apply to fail")
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 0 {
		t.Fatalf("characters after rollback = %#v, err=%v", characters, err)
	}
	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil || len(nodes) != 0 {
		t.Fatalf("outline after rollback = %#v, err=%v", nodes, err)
	}
	workbench, err := raw.WorkbenchState(ctx)
	if err == nil && workbench.ArchiveStatus == "confirmed" {
		t.Fatalf("workbench should not be confirmed: %#v", workbench)
	}
}

func sqliteCreateStore(t *testing.T) (*sqlite.Store, projectdomain.Project) {
	t.Helper()
	project, err := projectdomain.NewProject("project-1", "长夜织境", "东方奇幻", "简介", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := (sqlite.Factory{}).Create(context.Background(), filepath.Join(t.TempDir(), "project.db"), project)
	if err != nil {
		t.Fatal(err)
	}
	store, ok := opened.(*sqlite.Store)
	if !ok {
		t.Fatalf("store type = %T", opened)
	}
	return store, project
}

func seedArchiveSession(t *testing.T, store Store, projectID string) creationdomain.Session {
	t.Helper()
	now := testClock{}.Now()
	session, err := creationdomain.NewSession("session-1", projectID, "branch-1", now)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SaveIntent("守关人必须放走仇敌", now)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := creationdomain.NewCandidate(
		"dir-set-1", session.ID, creationdomain.StageDirections, creationdomain.KindDirectionSet,
		1, "", "run-dir", "art-dir", directionPayload, "story_directions.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := creationdomain.NewCandidate(
		"proposal-1", session.ID, creationdomain.StageProposal, creationdomain.KindProposal,
		1, "", "run-prop", "art-prop", proposalPayload, "story_blueprint.schema", 2, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	people, err := creationdomain.NewCandidate(
		"people-1", session.ID, creationdomain.StagePeopleWorld, creationdomain.KindPeopleWorld,
		1, "", "run-cast", "art-cast", peopleWorldPayload, "story_cast.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	roadmap, err := creationdomain.NewCandidate(
		"roadmap-1", session.ID, creationdomain.StageRoadmap, creationdomain.KindRoadmap,
		1, "", "run-map", "art-map", roadmapPayload, "story_roadmap.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	session.Stage = creationdomain.StageArchiveConfirm
	session.SelectedDirectionCandidateID = dir.ID
	if err := store.SaveSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []creationdomain.Candidate{dir, proposal, people, roadmap} {
		if err := store.SaveCandidate(context.Background(), candidate); err != nil {
			t.Fatal(err)
		}
	}
	return session
}
