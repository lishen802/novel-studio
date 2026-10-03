package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	creation "github.com/lishen802/novel-studio/internal/domain/creation"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

func seedCharacterAdoption(t *testing.T) (*Store, asset.ConstraintRequest) {
	t.Helper()
	s, r := seedCharacterRevision(t)
	ctx := context.Background()
	now := time.Unix(100, 0)
	session, _ := creation.NewSession("cast-session", r.ProjectID, "main", now)
	if _, err := s.CreateCreationSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	views, _ := s.CharacterRevisions(ctx)
	var profile map[string]any
	_ = json.Unmarshal([]byte(views[0].Current), &profile)
	profile["id"] = "cast-hero"
	cast := map[string]any{"characters": []any{profile}, "relationships": []any{}, "factions": []any{}, "locations": []any{}, "world_rules": []any{map[string]any{"name": "边界", "description": "不能复活", "scope_type": "project"}}, "forbidden": []string{"禁写"}, "omission_reasons": []string{}}
	before, _ := json.Marshal(cast)
	base, _ := creation.NewCandidate("cast-base", session.ID, creation.StagePeopleWorld, creation.KindPeopleWorld, 1, "", "", "", string(before), "story_cast.schema", 2, now)
	if err := s.SaveCreationCandidate(ctx, base); err != nil {
		t.Fatal(err)
	}
	profile["summary"] = "AI 新简介"
	after, _ := json.Marshal(cast)
	next, err := base.Regenerate("cast-ai", "cast-run", "cast-artifact", string(after), now)
	if err != nil {
		t.Fatal(err)
	}
	next.QualityStatus = creation.QualityPassed
	if err = s.SaveCreationCandidate(ctx, next); err != nil {
		t.Fatal(err)
	}
	stamp := formatTime(now)
	output, _ := json.Marshal(map[string]any{"candidate_kind": "people_world", "payload_schema_ref": "story_cast.schema@2", "payload": json.RawMessage(after)})
	args, _ := json.Marshal(map[string]any{"contract": "story_revision@1", "base": base, "baseId": base.ID, "resultId": next.ID, "runId": next.RunID, "request": map[string]any{"candidateId": base.ID, "paths": []string{"/characters/@cast-hero/summary"}}})
	statements := []struct {
		sql  string
		args []any
	}{
		{`UPDATE story_assets SET source_candidate_id=? WHERE id=?`, []any{base.ID, r.AssetID}},
		{`INSERT INTO agent_runs(id,project_id,chapter_base_revision,prompt,status,task_kind,workflow_id,workflow_version,quality_status,created_at,updated_at) VALUES(?,?,0,'修改','completed','revise_story_candidate','story.author_revision',3,'passed',?,?)`, []any{next.RunID, r.ProjectID, stamp, stamp}},
		{`INSERT INTO agent_artifacts(id,run_id,kind,content,sha256,created_at) VALUES(?,?,'plan',?,?,?)`, []any{next.ArtifactID, next.RunID, string(output), adoptionHash(string(output)), stamp}},
		{`INSERT INTO assistant_sessions(id,project_id,branch_id,kind,created_at,updated_at) VALUES('cast-assistant',?,'main','creation',?,?)`, []any{r.ProjectID, stamp, stamp}},
		{`INSERT INTO assistant_actions(id,session_id,project_id,branch_id,operation,risk,arguments,arguments_hash,idempotency_key,state,expires_at,created_at,updated_at) VALUES('cast-action','cast-assistant',?,'main','revise_fields','paid',?,?,'cast-key','succeeded',?,?,?)`, []any{r.ProjectID, string(args), adoptionHash(string(args)), stamp, stamp, stamp}},
	}
	for _, stmt := range statements {
		if _, err = s.db.ExecContext(ctx, stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}
	r.BranchID = "main"
	r.CandidateID = next.ID
	delete(profile, "id")
	normalized, _ := json.Marshal(profile)
	r.Text = string(normalized)
	return s, r
}

func TestCharacterAdoptionKeepsSourceAndReplaysReceipt(t *testing.T) {
	s, r := seedCharacterAdoption(t)
	ctx := context.Background()
	if _, err := s.PreviewConstraintChange(ctx, r); err != nil {
		t.Fatal(err)
	}
	views, err := s.CharacterRevisions(ctx)
	if err != nil || len(views[0].CandidateSuggestions) != 1 {
		t.Fatalf("suggestions: %#v %v", views, err)
	}
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil || !p.CanApply {
		t.Fatalf("preview: %#v %v", p, err)
	}
	cmd := asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "adopt"}
	for i := 0; i < 2; i++ {
		if _, err = s.ApplyConstraintChange(ctx, cmd, time.Unix(200, 0)); err != nil {
			t.Fatal(err)
		}
	}
	rev, err := s.StoryAssetRevision(ctx, r.AssetID, 2)
	if err != nil || !strings.Contains(rev.PayloadJSON, `"candidateId":"cast-ai"`) || !strings.Contains(rev.PayloadJSON, `"entityId":"cast-hero"`) {
		t.Fatalf("provenance: %+v %v", rev, err)
	}
	original, _ := s.Character(ctx, "hero")
	if original.Summary != "原始简介" {
		t.Fatal("rewrote original")
	}
	views, err = s.CharacterRevisions(ctx)
	if err != nil || len(views[0].CandidateSuggestions) != 0 {
		t.Fatalf("consumed suggestion: %#v %v", views, err)
	}
}

func TestCharacterAdoptionRejectsChangedSourceAndForgedText(t *testing.T) {
	for _, mode := range []string{"text", "quality", "artifact", "action", "branch", "candidate", "baseline", "lock"} {
		t.Run(mode, func(t *testing.T) {
			s, r := seedCharacterAdoption(t)
			ctx := context.Background()
			p, err := s.PreviewConstraintChange(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "text":
				p.Request.Text = strings.Replace(p.Request.Text, "AI 新简介", "伪造", 1)
			case "quality":
				_, err = s.db.Exec(`UPDATE agent_runs SET quality_status='blocked' WHERE id='cast-run'`)
			case "artifact":
				_, err = s.db.Exec(`UPDATE agent_artifacts SET content='{}' WHERE id='cast-artifact'`)
			case "action":
				_, err = s.db.Exec(`UPDATE assistant_actions SET arguments_hash='fake' WHERE id='cast-action'`)
			case "branch":
				p.Request.BranchID = "other"
			case "candidate":
				_, err = s.db.Exec(`UPDATE creation_candidates SET origin='author' WHERE id='cast-ai'`)
			case "baseline":
				_, err = s.db.Exec(`UPDATE story_asset_revisions SET payload_json=replace(payload_json,'原始简介','作者新简介') WHERE asset_id=?`, r.AssetID)
			case "lock":
				_, err = s.db.Exec(`UPDATE story_assets SET locked=1 WHERE id=?`, r.AssetID)
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "reject"}, time.Unix(200, 0))
			if err == nil {
				t.Fatal("accepted invalid source")
			}
			if !errors.Is(err, asset.ErrChangeConflict) && !errors.Is(err, asset.ErrChangeUnsupported) {
				t.Fatalf("unexpected: %v", err)
			}
			var n int
			_ = s.db.QueryRow(`SELECT count(*) FROM story_asset_revisions WHERE asset_id=?`, r.AssetID).Scan(&n)
			if n != 1 {
				t.Fatal("partial write")
			}
		})
	}
}

func TestCharacterAdoptionRollsBackWhenReceiptFails(t *testing.T) {
	s, r := seedCharacterAdoption(t)
	ctx := context.Background()
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.db.Exec(`CREATE TRIGGER fail_adoption_receipt BEFORE INSERT ON story_constraint_changes BEGIN SELECT RAISE(ABORT,'receipt failed'); END`)
	if err != nil {
		t.Fatal(err)
	}
	cmd := asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "retry"}
	if _, err = s.ApplyConstraintChange(ctx, cmd, time.Unix(200, 0)); err == nil {
		t.Fatal("missing failure")
	}
	view, err := s.CharacterRevisions(ctx)
	if err != nil || view[0].Revision != 1 || len(view[0].CandidateSuggestions) != 1 {
		t.Fatalf("partial changes: %#v %v", view, err)
	}
	if _, err = s.db.Exec(`DROP TRIGGER fail_adoption_receipt`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, cmd, time.Unix(201, 0)); err != nil {
		t.Fatal(err)
	}
}

func TestCharacterAdoptionFreezesAuthorizationAndShowsHistorySource(t *testing.T) {
	s, r := seedCharacterAdoption(t)
	ctx := context.Background()
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	var args string
	if err = s.db.QueryRow(`SELECT arguments FROM assistant_actions WHERE id='cast-action'`).Scan(&args); err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	_ = json.Unmarshal([]byte(args), &input)
	input["request"].(map[string]any)["instruction"] = "改写简介"
	encoded, _ := json.Marshal(input)
	if _, err = s.db.Exec(`UPDATE assistant_actions SET arguments=?,arguments_hash=? WHERE id='cast-action'`, string(encoded), adoptionHash(string(encoded))); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "frozen"}, time.Unix(200, 0)); !errors.Is(err, asset.ErrChangeConflict) {
		t.Fatalf("authorization changed: %v", err)
	}
	p, err = s.PreviewConstraintChange(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "new"}, time.Unix(201, 0)); err != nil {
		t.Fatal(err)
	}
	views, err := s.CharacterRevisions(ctx)
	if err != nil || len(views[0].Changes) != 1 || views[0].Changes[0].CandidateID != "cast-ai" {
		t.Fatalf("missing history source: %#v %v", views, err)
	}
	for pos, want := range map[int]string{1: "原始简介", 2: "AI 新简介"} {
		list, err := s.EffectiveConstraints(ctx, pos)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range list {
			if item.AssetID == r.AssetID {
				found = strings.Contains(item.Text, want)
			}
		}
		if !found {
			t.Fatalf("wrong effective profile at %d", pos)
		}
	}
}

func TestAuthorRevisionAfterAdoptionDoesNotClaimNewAISource(t *testing.T) {
	s, r := seedCharacterAdoption(t)
	ctx := context.Background()
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "ai"}, time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	r.CandidateID = ""
	r.ChapterID = "later"
	r.Text = strings.Replace(r.Text, "AI 新简介", "作者后续简介", 1)
	p, err = s.PreviewConstraintChange(ctx, r)
	if err != nil || !p.CanApply {
		t.Fatalf("author preview: %#v %v", p, err)
	}
	if _, err = s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "author"}, time.Unix(201, 0)); err != nil {
		t.Fatal(err)
	}
	views, err := s.CharacterRevisions(ctx)
	if err != nil || len(views[0].Changes) != 2 || views[0].Changes[0].CandidateID != "cast-ai" || views[0].Changes[1].CandidateID != "" {
		t.Fatalf("wrong attribution: %#v %v", views, err)
	}
	rev, err := s.StoryAssetRevision(ctx, r.AssetID, 3)
	if err != nil || strings.Contains(rev.PayloadJSON, "_candidateAdoption") {
		t.Fatalf("author copied AI provenance: %+v %v", rev, err)
	}
}
