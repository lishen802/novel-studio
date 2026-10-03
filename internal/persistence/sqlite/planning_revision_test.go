package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	revision "github.com/lishen802/novel-studio/internal/domain/revision"
	story "github.com/lishen802/novel-studio/internal/domain/story"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

func seedPlanningRevision(t *testing.T, kind string) (*Store, asset.ConstraintRequest) {
	t.Helper()
	s, r := seedConstraintChange(t)
	ctx := context.Background()
	now := time.Unix(100, 0)
	payload := `{"title":"潮汐","positioning":"奇幻","core_conflict":"守城","protagonist_arc":"成长","sell_point":"代价","genre":"奇幻","growth_cost":"失去","pacing":"渐进","ending":"归来","world_rules":["不能复活"],"major_milestones":["登岛","破局","归来"],"anchors":[],"extension":{"keep":true}}`
	table, typedID := "", ""
	if kind == "outline_node" {
		node, err := story.NewOutlineNode("arc", r.ProjectID, nil, story.LevelBook, "潮汐", 1, now)
		if err != nil {
			t.Fatal(err)
		}
		node.Summary = "原概要"
		node.Goal = "守城"
		node.Conflict = "选择"
		node.Outcome = "线索"
		node.PlanningJSON = `{"reader_expectation":"破局","stage_payoff":"守城","irreversible_change":"失去退路","ending_contribution":"归来","commitments":[{"id":"promise","statement":"解谜","fulfillment_range":"本卷","related_refs":["character:hero"]}],"reference_bindings":{"character:hero":{"table":"characters","id":"hero"}},"schema_version":1}`
		if err = s.CreateOutlineNode(ctx, node); err != nil {
			t.Fatal(err)
		}
		object := map[string]any{"title": node.Title, "summary": node.Summary, "goal": node.Goal, "conflict": node.Conflict, "outcome": node.Outcome, "level": node.Level, "position": node.Position, "planning": json.RawMessage(node.PlanningJSON), "extension": map[string]bool{"keep": true}}
		encoded, _ := json.Marshal(object)
		payload = string(encoded)
		table = "outline_nodes"
		typedID = node.ID
	}
	a, err := asset.New("planning", r.ProjectID, asset.Kind(kind), asset.OriginAuthor, "", "", "", table, typedID, now)
	if err != nil {
		t.Fatal(err)
	}
	tx, _ := s.db.BeginTx(ctx, nil)
	if err = insertStoryAsset(ctx, tx, a, payload, now); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	fields, err := asset.PlanningFields(kind, payload)
	if err != nil {
		t.Fatal(err)
	}
	edits := map[string]string{}
	for _, field := range fields {
		edits[field.Path] = field.Value
	}
	edits["/title"] = "未来潮汐"
	text, _ := json.Marshal(edits)
	r.AssetID = a.ID
	r.Text = string(text)
	r.BranchID = "main"
	return s, r
}

func TestPlanningRevisionPreservesHistoryAndFullPayload(t *testing.T) {
	for _, kind := range []string{"proposal", "outline_node"} {
		t.Run(kind, func(t *testing.T) {
			s, r := seedPlanningRevision(t, kind)
			ctx := context.Background()
			views, err := s.PlanningRevisions(ctx)
			if err != nil || len(views) != 1 || len(views[0].Fields) == 0 {
				t.Fatalf("views: %#v %v", views, err)
			}
			p, err := s.PreviewConstraintChange(ctx, r)
			if err != nil || !p.CanApply {
				t.Fatalf("preview: %#v %v", p, err)
			}
			cmd := asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "once"}
			for i := 0; i < 2; i++ {
				if _, err = s.ApplyConstraintChange(ctx, cmd, time.Unix(200, 0)); err != nil {
					t.Fatal(err)
				}
			}
			rev, err := s.StoryAssetRevision(ctx, r.AssetID, 2)
			if err != nil || !strings.Contains(rev.PayloadJSON, `"keep":true`) || !strings.Contains(rev.PayloadJSON, "未来潮汐") {
				t.Fatalf("revision: %+v %v", rev, err)
			}
			if kind == "outline_node" {
				nodes, _ := s.ListOutlineNodes(ctx)
				if nodes[0].Title != "潮汐" || !strings.Contains(rev.PayloadJSON, `"id":"promise"`) || !strings.Contains(rev.PayloadJSON, `"reference_bindings"`) {
					t.Fatal("lost original/evidence")
				}
			}
			for pos, want := range map[int]string{1: "潮汐", 2: "未来潮汐"} {
				effective, err := s.EffectiveConstraints(ctx, pos)
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, v := range effective {
					if v.AssetID == r.AssetID {
						var edits map[string]string
						_ = json.Unmarshal([]byte(v.Text), &edits)
						found = edits["/title"] == want
					}
				}
				if !found {
					t.Fatalf("wrong version at %d", pos)
				}
			}
			views, err = s.PlanningRevisions(ctx)
			if err != nil || views[0].Revision != 2 || len(views[0].Changes) != 1 {
				t.Fatalf("history: %#v %v", views, err)
			}
			constraints, _ := s.Constraints(ctx)
			for _, v := range constraints {
				if v.Kind == kind {
					t.Fatal("planning leaked into constraint panel")
				}
			}
		})
	}
}

func TestPlanningRevisionRejectsUnsafeConfirmation(t *testing.T) {
	for _, mode := range []string{"path", "missing", "lock", "source_lock", "body", "typed", "candidate"} {
		t.Run(mode, func(t *testing.T) {
			s, r := seedPlanningRevision(t, "outline_node")
			ctx := context.Background()
			p, err := s.PreviewConstraintChange(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "path":
				var edits map[string]string
				_ = json.Unmarshal([]byte(r.Text), &edits)
				edits["/parentId"] = "other"
				text, _ := json.Marshal(edits)
				p.Request.Text = string(text)
			case "missing":
				p.Request.Text = `{"/title":"伪造"}`
			case "lock":
				_, err = s.db.Exec(`UPDATE story_assets SET locked=1 WHERE id='planning'`)
			case "source_lock":
				_, err = s.db.Exec(`UPDATE story_asset_revisions SET payload_json=json_set(payload_json,'$._sourceLocks',json('{"title":true}')) WHERE asset_id='planning' AND revision=1`)
			case "body":
				_, err = s.db.Exec(`UPDATE chapters SET body='作者新正文' WHERE id='future'`)
			case "typed":
				_, err = s.db.Exec(`UPDATE outline_nodes SET position=9 WHERE id='arc'`)
			case "candidate":
				p.Request.CandidateID = "pretend-ai"
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "reject"}, time.Unix(200, 0)); err == nil {
				t.Fatal("unsafe confirmation")
			}
			var n int
			_ = s.db.QueryRow(`SELECT count(*) FROM story_asset_revisions WHERE asset_id='planning'`).Scan(&n)
			if n != 1 {
				t.Fatal("partial write")
			}
		})
	}
}

func TestFormalPlanningCannotUseGlobalOutlineChangeSet(t *testing.T) {
	s, r := seedPlanningRevision(t, "outline_node")
	ctx := context.Background()
	value := `{"title":"全局覆盖","summary":"原概要","goal":"守城","conflict":"选择","outcome":"线索"}`
	changes, err := revision.NewChangeSet("bypass", r.ProjectID, "绕过", "", "author", []revision.PatchOperationSpec{{ID: "op", Kind: revision.OperationUpdateOutlineNode, TargetID: "arc", BeforeValue: `{"title":"潮汐","summary":"原概要","goal":"守城","conflict":"选择","outcome":"线索"}`, AfterValue: value}}, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateChangeSet(ctx, changes); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyChangeSet(ctx, changes.ID, time.Unix(201, 0)); err == nil {
		t.Fatal("global bypass")
	}
	if err = s.ArchiveOutlineNode(ctx, "arc", time.Unix(202, 0)); !errors.Is(err, asset.ErrChangeUnsupported) {
		t.Fatalf("archive: %v", err)
	}
}

// TestFormalPlanningRejectsUnversionedNodeCreation 验证正式入档后不能绕过规划审阅新增节点。
func TestFormalPlanningRejectsUnversionedNodeCreation(t *testing.T) {
	s, r := seedPlanningRevision(t, "outline_node")
	node, err := story.NewOutlineNode("new-book", r.ProjectID, nil, story.LevelBook, "未经审阅的新结构", 2, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateOutlineNode(context.Background(), node); !errors.Is(err, asset.ErrChangeUnsupported) {
		t.Fatalf("create: %v", err)
	}
	nodes, err := s.ListOutlineNodes(context.Background())
	if err != nil || len(nodes) != 1 {
		t.Fatalf("structure changed: %+v %v", nodes, err)
	}
}
