package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	revision "github.com/lishen802/novel-studio/internal/domain/revision"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
	"strings"
	"testing"
	"time"
)

func TestFutureConstraintFlowsThroughContextRestartAndBranches(t *testing.T) {
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
	rule, err := s.CreateWorldRule(ctx, WorldRuleInput{Name: "潮汐", Description: "原规则", ScopeType: "project"})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]string{"name": rule.Name, "description": rule.Description})
	value, _ := json.Marshal(map[string]string{"kind": "world_rule", "origin": "author", "typedTable": "world_rules", "typedId": rule.ID, "payloadJson": string(payload)})
	seed, err := revision.NewChangeSet("seed", project.ID, "正式规则", "", "author", []revision.PatchOperationSpec{{ID: "seed-op", Kind: revision.OperationCreateStoryAsset, TargetID: "asset", AfterValue: string(value)}}, time.Unix(100, 0))
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
	child, err := s.CreateBranch(ctx, "旧规则线", "")
	if err != nil {
		t.Fatal(err)
	}
	request := asset.ConstraintRequest{AssetID: "asset", Intent: "future_only", ChapterID: future.ID, Text: "后续规则", Reason: "后续路线调整"}
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
		if d.Key == "world" {
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
		if json.Unmarshal([]byte(body), &doc) != nil || len(doc.Domains["formalConstraints"]) != 1 {
			t.Fatal("missing required revision")
		}
	}
	check(old.ID, "原规则", "后续规则")
	check(future.ID, "后续规则", "原规则")
	if _, _, err = s.assembleAuthoringContext(ctx, s.active.store, future.ID, 1); !errors.Is(err, ErrContextBudgetExceeded) {
		t.Fatal("required constraint silently trimmed")
	}
	if _, err = s.SwitchBranch(ctx, child.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, command); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("cross-branch authorization accepted: %v", err)
	}
	check(future.ID, "原规则", "后续规则")
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
	check(future.ID, "后续规则", "原规则")
}
