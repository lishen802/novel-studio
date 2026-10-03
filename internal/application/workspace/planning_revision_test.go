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

func TestPlanningRevisionsFlowThroughContextBranchesAndRestart(t *testing.T) {
	ctx := context.Background()
	newService := func() *Service {
		return NewService(projectfs.Manager{}, sqlite.Factory{}, license.NoopService{}, fixedClock{now: time.Unix(200, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{}, branchfs.Archive{})
	}
	s := newService()
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
	node, err := s.CreateOutlineNode(ctx, CreateOutlineNodeInput{Level: "book", Title: "原全书目标", Summary: "原始命题", Goal: "守城", Conflict: "选择", Outcome: "归来"})
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"title": node.Title, "summary": node.Summary, "goal": node.Goal, "conflict": node.Conflict, "outcome": node.Outcome, "level": node.Level, "position": node.Position})
	proposal := `{"title":"原作品方案","positioning":"奇幻","core_conflict":"守城","protagonist_arc":"成长","sell_point":"代价","genre":"奇幻","growth_cost":"失去","pacing":"渐进","ending":"归来","major_milestones":["登岛","破局","归来"]}`
	specs := []revision.PatchOperationSpec{}
	for _, a := range []struct{ id, kind, table, typed, payload string }{{"plan", "proposal", "", "", proposal}, {"node-plan", "outline_node", "outline_nodes", node.ID, string(payload)}} {
		value, _ := json.Marshal(map[string]string{"kind": a.kind, "origin": "author", "typedTable": a.table, "typedId": a.typed, "payloadJson": a.payload})
		specs = append(specs, revision.PatchOperationSpec{ID: a.id + "-op", Kind: revision.OperationCreateStoryAsset, TargetID: a.id, AfterValue: string(value)})
	}
	seed, err := revision.NewChangeSet("seed-planning", project.ID, "正式规划", "", "author", specs, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.active.store.CreateChangeSet(ctx, seed); err != nil {
		t.Fatal(err)
	}
	if _, err = s.active.store.ApplyChangeSet(ctx, seed.ID, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	mainID, err := s.constraintBranchLocked()
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.CreateBranch(ctx, "原规划", "")
	if err != nil {
		t.Fatal(err)
	}
	commands := []asset.ConstraintCommand{}
	views, err := s.PlanningRevisions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, view := range views {
		fields := map[string]string{}
		_ = json.Unmarshal([]byte(view.Current), &fields)
		fields["/title"] = "未来" + view.Name
		text, _ := json.Marshal(fields)
		p, err := s.PreviewConstraintChange(ctx, asset.ConstraintRequest{AssetID: view.AssetID, Intent: "future_only", ChapterID: future.ID, Text: string(text), Reason: "后续定位调整"})
		if err != nil || !p.CanApply {
			t.Fatalf("preview: %#v %v", p, err)
		}
		cmd := asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: view.AssetID}
		s.active.mode = ports.ProjectAccessReadOnly
		if _, err = s.ApplyConstraintChange(ctx, cmd); !errors.Is(err, ErrWorkspaceReadOnly) {
			t.Fatalf("readonly: %v", err)
		}
		s.active.mode = ports.ProjectAccessEditable
		if _, err = s.ApplyConstraintChange(ctx, cmd); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
	}
	check := func(chapterID string, futureVersion bool) {
		t.Helper()
		body, _, err := s.assembleAuthoringContext(ctx, s.active.store, chapterID, 100000)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(body, "未来原全书目标") != futureVersion || strings.Contains(body, "未来原作品方案") != futureVersion {
			t.Fatalf("wrong planning boundary: %s", body)
		}
		var doc contextManifestDocument
		if json.Unmarshal([]byte(body), &doc) != nil || len(doc.Domains["formalPlanningVersions"]) != 2 || len(doc.Domains["formalProposal"]) != 1 || len(doc.Domains["formalOutlinePlanning"]) != 1 {
			t.Fatal("missing required planning sources")
		}
	}
	check(old.ID, false)
	check(future.ID, true)
	if _, _, err = s.assembleAuthoringContext(ctx, s.active.store, future.ID, 100); !errors.Is(err, ErrContextBudgetExceeded) {
		t.Fatalf("required budget: %v", err)
	}
	comparison, err := s.CompareBranches(ctx, mainID, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	changed := false
	for _, domain := range comparison.Domains {
		if domain.Key == "story" {
			changed = domain.Changed
		}
	}
	if !changed {
		t.Fatal("missing planning branch diff")
	}
	if _, err = s.SwitchBranch(ctx, child.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, commands[0]); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("cross branch: %v", err)
	}
	check(future.ID, false)
	if _, err = s.SwitchBranch(ctx, mainID); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s = newService()
	if _, err = s.OpenProject(ctx, project.Path); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range commands {
		if _, err = s.ApplyConstraintChange(ctx, cmd); err != nil {
			t.Fatalf("restart replay: %v", err)
		}
	}
	check(future.ID, true)
	structure, err := s.StoryStructure(ctx)
	if err != nil || structure.OutlineNodes[0].Title != "原全书目标" {
		t.Fatal("rewrote original graph")
	}
}
