package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	revision "github.com/lishen802/novel-studio/internal/domain/revision"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
)

func TestFutureCharacterFlowsThroughContextRestartAndBranches(t *testing.T) {
	ctx := context.Background()
	s := NewService(projectfs.Manager{}, sqlite.Factory{}, license.NoopService{}, fixedClock{now: time.Unix(200, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{}, branchfs.Archive{})
	t.Cleanup(func() { s.Close() })
	project, err := s.CreateProject(ctx, t.TempDir(), "潮汐", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	old, err := s.CreateChapter(ctx, "旧章")
	if err != nil {
		t.Fatal(err)
	}
	future, err := s.CreateChapter(ctx, "新章")
	if err != nil {
		t.Fatal(err)
	}

	hero, err := s.CreateCharacter(ctx, CharacterInput{Name: "林舟", Aliases: []string{}, Role: "protagonist", Summary: "原始人物简介", Motivation: "寻找故乡"})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"name": hero.Name, "aliases": hero.Aliases, "role": hero.Role, "summary": hero.Summary, "motivation": hero.Motivation, "fear": hero.Fear, "secret": hero.Secret, "arc": hero.Arc})
	value, _ := json.Marshal(map[string]string{"kind": "character", "origin": "author", "typedTable": "characters", "typedId": hero.ID, "payloadJson": string(payload)})
	seed, err := revision.NewChangeSet("seed", project.ID, "正式人物", "", "author", []revision.PatchOperationSpec{{ID: "seed-op", Kind: revision.OperationCreateStoryAsset, TargetID: "asset", AfterValue: string(value)}}, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.active.store.CreateChangeSet(ctx, seed); err != nil {
		t.Fatal(err)
	}
	if _, err = s.active.store.ApplyChangeSet(ctx, seed.ID, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	branches, err := s.Branches(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mainID := ""
	for _, b := range branches {
		if b.Active {
			mainID = b.ID
		}
	}
	child, err := s.CreateBranch(ctx, "旧人物线", "")
	if err != nil {
		t.Fatal(err)
	}
	request := asset.ConstraintRequest{AssetID: "asset", Intent: "future_only", ChapterID: future.ID, Text: `{"name":"林舟","aliases":[],"role":"protagonist","summary":"未来人物简介","motivation":"保护故乡","fear":"","secret":"","arc":""}`, Reason: "后续路线调整"}
	preview, err := s.PreviewConstraintChange(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	command := asset.ConstraintCommand{Request: preview.Request, Digest: preview.Digest, IdempotencyKey: "once"}
	s.active.mode = ports.ProjectAccessReadOnly
	if _, err = s.ApplyConstraintChange(ctx, command); !errors.Is(err, ErrWorkspaceReadOnly) {
		t.Fatalf("read only %v", err)
	}
	s.active.mode = ports.ProjectAccessEditable
	if _, err = s.ApplyConstraintChange(ctx, command); err != nil {
		t.Fatal(err)
	}
	comparison, err := s.CompareBranches(ctx, mainID, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	changed := false
	for _, d := range comparison.Domains {
		if d.Key == "characters" {
			changed = d.Changed
		}
	}
	if !changed {
		t.Fatal("future setting invisible in branch diff")
	}
	check := func(id, want, absent string) {
		t.Helper()
		body, _, err := s.assembleAuthoringContext(ctx, s.active.store, id, 100000)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(body, want) || strings.Contains(body, absent) {
			t.Fatalf("wrong chapter context: %s", body)
		}
		var doc contextManifestDocument
		if json.Unmarshal([]byte(body), &doc) != nil || len(doc.Domains["formalCharacterVersions"]) != 1 {
			t.Fatal("missing required revision")
		}
	}
	check(old.ID, "原始人物简介", "未来人物简介")
	check(future.ID, "未来人物简介", "原始人物简介")
	if _, _, err = s.assembleAuthoringContext(ctx, s.active.store, future.ID, 1); !errors.Is(err, ErrContextBudgetExceeded) {
		t.Fatal("required constraint silently trimmed")
	}
	if _, err = s.SwitchBranch(ctx, child.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, command); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("cross-branch authorization accepted: %v", err)
	}
	check(future.ID, "原始人物简介", "未来人物简介")
	if _, err = s.SwitchBranch(ctx, mainID); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s = NewService(projectfs.Manager{}, sqlite.Factory{}, license.NoopService{}, fixedClock{now: time.Unix(300, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{}, branchfs.Archive{})
	if _, err = s.OpenProject(ctx, project.Path); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, command); err != nil {
		t.Fatalf("restart replay: %v", err)
	}
	check(future.ID, "未来人物简介", "原始人物简介")
	history, err := s.CharacterRevisions(ctx)
	if err != nil || len(history) != 1 || history[0].Revision != 2 {
		t.Fatalf("restart history: %#v %v", history, err)
	}
}
