package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/ports"
	creation "github.com/lishen802/novel-studio/internal/domain/creation"
	revision "github.com/lishen802/novel-studio/internal/domain/revision"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
)

func TestFormalCharacterPreviewBindsActualWorkspaceBranchAndReadOnly(t *testing.T) {
	ctx := context.Background()
	s := newAgentServiceWithResolver(t, failingModelProviderFactory{}, defaultResolvedProfile())
	s.branches = branchfs.Archive{}
	if _, err := s.CreateProject(ctx, t.TempDir(), "人物修订", "奇幻", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateChapter(ctx, "未来章"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	session, err := runtime.service.EnsureSession(ctx, runtime.projectID, runtime.branchID)
	if err != nil {
		t.Fatal(err)
	}
	hero, err := s.CreateCharacter(ctx, CharacterInput{Name: "林舟", Aliases: []string{}, Role: "protagonist", Summary: "原简介", Motivation: "归乡", Arc: "成长"})
	if err != nil {
		t.Fatal(err)
	}
	typed, err := s.active.store.Character(ctx, hero.ID)
	if err != nil {
		t.Fatal(err)
	}
	profile := asset.ProfileOf(typed)
	payload, _ := json.Marshal(profile)
	character := map[string]any{}
	_ = json.Unmarshal(payload, &character)
	character["id"] = "source-hero"
	cast, _ := json.Marshal(map[string]any{"characters": []any{character}, "relationships": []any{}, "factions": []any{}, "locations": []any{}, "world_rules": []any{map[string]string{"name": "界限", "description": "不能复活", "scope_type": "project"}}, "forbidden": []string{"禁写"}, "omission_reasons": []string{}})
	candidate, err := creation.NewCandidate("cast-source", session.ID, creation.StagePeopleWorld, creation.KindPeopleWorld, 1, "", "", "", string(cast), "story_cast.schema", 2, s.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	backend := s.active.store.(creationapp.SQLBackend)
	if err = backend.SaveCreationCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	value, _ := json.Marshal(map[string]string{"kind": "character", "origin": "author", "sourceCandidateId": candidate.ID, "typedTable": "characters", "typedId": hero.ID, "payloadJson": string(payload)})
	seed, err := revision.NewChangeSet("formal-seed", runtime.projectID, "正式人物", "", "author", []revision.PatchOperationSpec{{ID: "formal-op", Kind: revision.OperationCreateStoryAsset, TargetID: "formal-asset", AfterValue: string(value)}}, s.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = s.active.store.CreateChangeSet(ctx, seed); err != nil {
		t.Fatal(err)
	}
	if _, err = s.active.store.ApplyChangeSet(ctx, seed.ID, s.clock.Now()); err != nil {
		t.Fatal(err)
	}
	branch, err := s.constraintBranchLocked()
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.PreviewCharacterRevision(ctx, creationapp.FormalCharacterRequest{AssetID: "formal-asset", ExpectedRevision: 1, Field: "motivation", Instruction: "突出代价", IdempotencyKey: "formal-preview"})
	if err != nil {
		t.Fatal(err)
	}
	action, err := backend.AssistantAction(ctx, view.ActionID)
	if err != nil || action.BranchID != branch {
		t.Fatalf("wrong active branch: %+v %v", action, err)
	}
	child, err := s.CreateBranch(ctx, "平行人物线", "")
	if err != nil {
		t.Fatal(err)
	}
	s.active.mode = ports.ProjectAccessReadOnly
	if _, err = s.PreviewCharacterRevision(ctx, creationapp.FormalCharacterRequest{AssetID: "formal-asset", ExpectedRevision: 1, Field: "summary", Instruction: "简化", IdempotencyKey: "readonly"}); !errors.Is(err, ErrWorkspaceReadOnly) {
		t.Fatalf("readonly: %v", err)
	}
	views, err := s.CharacterRevisionActions(ctx)
	if err != nil || len(views) != 1 {
		t.Fatalf("read: %+v %v", views, err)
	}
	s.active.mode = ports.ProjectAccessEditable
	if _, err = s.SwitchBranch(ctx, child.ID); err != nil {
		t.Fatal(err)
	}
	command := creationapp.RevisionCommand{ActionID: view.ActionID, ArgumentsHash: view.ArgumentsHash, IdempotencyKey: view.IdempotencyKey}
	if _, err = s.ConfirmCreationRevision(ctx, command); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("cross branch payment: %v", err)
	}
	views, err = s.CharacterRevisionActions(ctx)
	if err != nil || len(views) != 1 || !views[0].Stale {
		t.Fatalf("branch stale: %+v %v", views, err)
	}
}
