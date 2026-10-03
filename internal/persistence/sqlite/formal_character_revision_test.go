package sqlite

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	agent "github.com/lishen802/novel-studio/internal/domain/agent"
	creation "github.com/lishen802/novel-studio/internal/domain/creation"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

func seedFormalCharacterResult(t *testing.T) (*Store, asset.ConstraintRequest) {
	t.Helper()
	s, r := seedCharacterAdoption(t)
	ctx := context.Background()
	r.CandidateID = ""
	r.Text = strings.Replace(r.Text, "AI 新简介", "作者当前简介", 1)
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "manual-first"}, time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	base, err := s.FormalCharacterRevisionBase(ctx, r.ProjectID, "main", r.AssetID)
	if err != nil {
		t.Fatal(err)
	}
	if base.Revision != 2 || !strings.Contains(base.CastJSON, "作者当前简介") {
		t.Fatalf("wrong baseline: %+v", base)
	}
	candidate, err := creation.NewCandidate("formal:"+r.AssetID, "cast-session", creation.StagePeopleWorld, creation.KindPeopleWorld, 2, "", "", "", base.CastJSON, "story_cast.schema", 2, time.Unix(210, 0))
	if err != nil {
		t.Fatal(err)
	}
	path := "/characters/@" + base.EntityID + "/summary"
	after, err := candidate.EditTextFields(map[string]string{path: "最新基线上的 AI 简介"})
	if err != nil {
		t.Fatal(err)
	}
	output, _ := json.Marshal(agent.StoryCandidateRevisionOutput{CandidateKind: "people_world", PayloadSchemaRef: "story_cast.schema@2", Payload: json.RawMessage(after)})
	args, _ := json.Marshal(map[string]any{"contract": "story_revision@1", "formal": base, "base": candidate, "runId": "formal-run", "request": map[string]any{"formalAssetId": r.AssetID, "expectedRevision": 2, "paths": []string{path}}})
	stamp := formatTime(time.Unix(210, 0))
	stmts := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO agent_runs(id,project_id,chapter_base_revision,prompt,status,task_kind,workflow_id,workflow_version,quality_status,created_at,updated_at) VALUES('formal-run',?,0,'正式人物修改','completed','revise_story_candidate','story.author_revision',3,'passed',?,?)`, []any{r.ProjectID, stamp, stamp}},
		{`INSERT INTO agent_artifacts(id,run_id,kind,content,sha256,created_at) VALUES('formal-artifact','formal-run','structured_output',?,?,?)`, []any{string(output), adoptionHash(string(output)), stamp}},
		{`INSERT INTO agent_run_steps(id,run_id,role,position,status,node_id,primary_artifact_id,created_at,updated_at) VALUES('formal-step','formal-run','planner',1,'completed','revise','formal-artifact',?,?)`, []any{stamp, stamp}},
		{`INSERT INTO assistant_actions(id,session_id,project_id,branch_id,operation,risk,arguments,arguments_hash,idempotency_key,state,expires_at,created_at,updated_at) VALUES('formal-action','cast-assistant',?,'main','revise_fields','paid',?,?,'formal-key','succeeded',?,?,?)`, []any{r.ProjectID, string(args), adoptionHash(string(args)), stamp, stamp, stamp}},
	}
	for _, stmt := range stmts {
		if _, err = s.db.Exec(stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	r.CandidateID = asset.FormalCharacterCandidatePrefix + "formal-action"
	r.ChapterID = "later"
	r.Text = strings.Replace(r.Text, "作者当前简介", "最新基线上的 AI 简介", 1)
	return s, r
}

func TestFormalCharacterResultAdoptsOnNewHeadAndSupportsNextRound(t *testing.T) {
	s, r := seedFormalCharacterResult(t)
	ctx := context.Background()
	views, err := s.CharacterRevisions(ctx)
	if err != nil || len(views[0].CandidateSuggestions) != 1 {
		t.Fatalf("suggestions: %+v %v", views, err)
	}
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil || !p.CanApply {
		t.Fatalf("preview: %+v %v", p, err)
	}
	command := asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "formal-adopt"}
	for i := 0; i < 2; i++ {
		if _, err = s.ApplyConstraintChange(ctx, command, time.Unix(220, 0)); err != nil {
			t.Fatal(err)
		}
	}
	next, err := s.FormalCharacterRevisionBase(ctx, r.ProjectID, "main", r.AssetID)
	if err != nil || next.Revision != 3 || !strings.Contains(next.CastJSON, "最新基线上的 AI 简介") {
		t.Fatalf("next: %+v %v", next, err)
	}
	rev, _ := s.StoryAssetRevision(ctx, r.AssetID, 3)
	if !strings.Contains(rev.PayloadJSON, "formal-artifact") || !strings.Contains(rev.PayloadJSON, "formal-action") {
		t.Fatal("lost source audit")
	}
	original, _ := s.Character(ctx, "hero")
	if original.Summary != "原始简介" {
		t.Fatal("rewrote history")
	}
	views, err = s.CharacterRevisions(ctx)
	if err != nil || len(views[0].CandidateSuggestions) != 0 {
		t.Fatalf("consumed: %+v %v", views, err)
	}
	candidates, _ := s.ListCreationCandidates(ctx, "cast-session")
	if len(candidates) != 2 {
		t.Fatal("formal result polluted initial candidates")
	}
}

func TestFormalCharacterResultRejectsStaleOrForgedSources(t *testing.T) {
	for _, mode := range []string{"quality", "artifact", "branch", "head", "lock", "path", "source"} {
		t.Run(mode, func(t *testing.T) {
			s, r := seedFormalCharacterResult(t)
			ctx := context.Background()
			p, err := s.PreviewConstraintChange(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "quality":
				_, err = s.db.Exec(`UPDATE agent_runs SET quality_status='blocked' WHERE id='formal-run'`)
			case "artifact":
				_, err = s.db.Exec(`UPDATE agent_artifacts SET content='{}' WHERE id='formal-artifact'`)
			case "branch":
				p.Request.BranchID = "other"
			case "head":
				_, err = s.db.Exec(`UPDATE story_asset_revisions SET payload_json=replace(payload_json,'作者当前简介','迟到的新编辑') WHERE asset_id=? AND revision=2`, r.AssetID)
			case "lock":
				_, err = s.db.Exec(`UPDATE story_assets SET locked=1 WHERE id=?`, r.AssetID)
			case "source":
				_, err = s.db.Exec(`UPDATE creation_candidates SET payload_json='{}' WHERE id='cast-base'`)
			case "path":
				_, err = s.db.Exec(`UPDATE assistant_actions SET arguments=json_set(arguments,'$.request.paths',json('["/characters/@cast-hero/secret"]')) WHERE id='formal-action'`)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "reject"}, time.Unix(230, 0)); err == nil {
				t.Fatal("accepted stale or forged result")
			}
			var head int
			_ = s.db.QueryRow(`SELECT current_revision FROM story_assets WHERE id=?`, r.AssetID).Scan(&head)
			if head != 2 {
				t.Fatal("partial write")
			}
		})
	}
}
