package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	revision "github.com/lishen802/novel-studio/internal/domain/revision"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
	world "github.com/lishen802/novel-studio/internal/domain/world"
)

func seedCharacterRevision(t *testing.T) (*Store, asset.ConstraintRequest) {
	t.Helper()
	s, r := seedConstraintChange(t)
	ctx := context.Background()
	c, err := world.NewCharacter("hero", r.ProjectID, "林舟", []string{"小舟"}, world.RoleProtagonist, "寻找故乡", "遗忘", "身世", "学会信任", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	c.Summary = "原始简介"
	if err = s.CreateCharacter(ctx, c); err != nil {
		t.Fatal(err)
	}
	a, _ := asset.New("hero-asset", r.ProjectID, asset.KindCharacter, asset.OriginAuthor, "", "", "", "characters", c.ID, time.Unix(100, 0))
	payload := `{"name":"林舟","aliases":["小舟"],"role":"protagonist","summary":"原始简介","motivation":"寻找故乡","fear":"遗忘","secret":"身世","arc":"学会信任","extension":{"keep":true}}`
	tx, _ := s.db.BeginTx(ctx, nil)
	if err = insertStoryAsset(ctx, tx, a, payload, time.Unix(100, 0)); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	r.AssetID = a.ID
	r.Text = `{"name":"林舟","aliases":["小舟"],"role":"protagonist","summary":"未来简介","motivation":"保护故乡","fear":"遗忘","secret":"身世","arc":"学会信任"}`
	return s, r
}

func TestCharacterRevisionPreservesIdentityHistoryAndExtensions(t *testing.T) {
	s, r := seedCharacterRevision(t)
	ctx := context.Background()
	before, err := s.PreviewConstraintChange(ctx, r)
	if err != nil || !before.CanApply {
		t.Fatalf("preview: %#v %v", before, err)
	}
	cmd := asset.ConstraintCommand{Request: r, Digest: before.Digest, IdempotencyKey: "hero-edit"}
	if _, err = s.ApplyConstraintChange(ctx, cmd, time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, cmd, time.Unix(300, 0)); err != nil {
		t.Fatal(err)
	}
	original, _ := s.Character(ctx, "hero")
	if original.Summary != "原始简介" || original.Motivation != "寻找故乡" {
		t.Fatal("original profile changed")
	}
	rev, _ := s.StoryAssetRevision(ctx, r.AssetID, 2)
	var payload map[string]json.RawMessage
	if json.Unmarshal([]byte(rev.PayloadJSON), &payload) != nil || string(payload["extension"]) != `{"keep":true}` {
		t.Fatalf("extension lost: %s", rev.PayloadJSON)
	}
	for pos, want := range map[int]string{1: "原始简介", 2: "未来简介", 3: "未来简介"} {
		list, err := s.EffectiveConstraints(ctx, pos)
		found := false
		for _, item := range list {
			if item.AssetID == r.AssetID {
				found = true
				var profile map[string]any
				_ = json.Unmarshal([]byte(item.Text), &profile)
				if profile["summary"] != want || item.TypedID != "hero" {
					t.Fatalf("wrong profile at %d: %#v", pos, item)
				}
			}
		}
		if err != nil || !found {
			t.Fatalf("missing profile: %v", err)
		}
	}
	history, err := s.CharacterRevisions(ctx)
	if err != nil || len(history) != 1 || history[0].Revision != 2 || len(history[0].Changes) != 1 || history[0].TypedID != "hero" {
		t.Fatalf("history: %#v %v", history, err)
	}
	if _, err = s.Constraints(ctx); err != nil {
		t.Fatal(err)
	}
	original.Summary = "全局覆盖"
	if err = s.UpdateCharacter(ctx, original); !errors.Is(err, asset.ErrChangeUnsupported) {
		t.Fatalf("old update bypass: %v", err)
	}
	if err = s.ArchiveCharacter(ctx, original.ID, time.Unix(400, 0)); !errors.Is(err, asset.ErrChangeUnsupported) {
		t.Fatalf("archive bypass: %v", err)
	}
}

func TestCharacterRevisionRejectsInvalidPayloadLocksAndLateConfirmation(t *testing.T) {
	for _, mode := range []string{"identity", "role", "blank", "source_lock", "legacy_source_lock", "asset_lock", "stale", "late_body", "typed_change"} {
		t.Run(mode, func(t *testing.T) {
			s, r := seedCharacterRevision(t)
			ctx := context.Background()
			p, err := s.PreviewConstraintChange(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "identity":
				r.Text = `{"id":"other","name":"林舟","role":"protagonist"}`
			case "role":
				r.Text = `{"name":"林舟","role":"unknown"}`
			case "blank":
				r.Text = `{"name":" ","role":"protagonist"}`
			case "source_lock":
				s.db.Exec(`UPDATE story_asset_revisions SET payload_json=json_set(payload_json,'$._sourceLocks',json('{"/characters/0/summary":true}')) WHERE asset_id='hero-asset'`)
			case "legacy_source_lock":
				_, candidate := seedReviewCandidate(t, s, r.ProjectID)
				if _, err = s.db.Exec(`UPDATE creation_candidates SET lock_map_json='{"/characters/0/motivation":true}' WHERE id=?`, candidate.ID); err != nil {
					t.Fatal(err)
				}
				if _, err = s.db.Exec(`UPDATE story_assets SET source_candidate_id=? WHERE id='hero-asset'`, candidate.ID); err != nil {
					t.Fatal(err)
				}
			case "asset_lock":
				s.db.Exec(`UPDATE story_assets SET locked=1 WHERE id='hero-asset'`)
			case "stale":
				s.db.Exec(`UPDATE story_assets SET stale=1 WHERE id='hero-asset'`)
			case "typed_change":
				s.db.Exec(`UPDATE characters SET summary='预览后非法改写' WHERE id='hero'`)
			case "late_body":
				ch, _ := s.Chapter(ctx, "future")
				n := ch
				n.Body = "作者的新正文"
				n.Revision++
				if err = s.CommitChapter(ctx, n, ch); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: r, Digest: p.Digest, IdempotencyKey: "unsafe"}, time.Unix(300, 0)); err == nil {
				t.Fatal("unsafe change accepted")
			}
			a, _ := s.StoryAsset(ctx, r.AssetID)
			if a.CurrentRevision != 1 {
				t.Fatal("partial revision")
			}
		})
	}
}

func TestCharacterRevisionConcurrentConfirmation(t *testing.T) {
	for _, sameKey := range []bool{true, false} {
		t.Run(map[bool]string{true: "重复提交", false: "竞争预览"}[sameKey], func(t *testing.T) {
			s, r := seedCharacterRevision(t)
			ctx := context.Background()
			p, err := s.PreviewConstraintChange(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			start := make(chan struct{})
			results := make(chan error, 2)
			var wg sync.WaitGroup
			for i := range 2 {
				wg.Add(1)
				go func(index int) {
					defer wg.Done()
					<-start
					key := "same"
					if !sameKey && index == 1 {
						key = "other"
					}
					_, e := s.ApplyConstraintChange(ctx, asset.ConstraintCommand{Request: r, Digest: p.Digest, IdempotencyKey: key}, time.Unix(300, 0))
					results <- e
				}(i)
			}
			close(start)
			wg.Wait()
			close(results)
			success := 0
			for e := range results {
				if e == nil {
					success++
				} else if !errors.Is(e, asset.ErrChangeConflict) {
					t.Fatal(e)
				}
			}
			want := 1
			if sameKey {
				want = 2
			}
			if success != want {
				t.Fatalf("successful submissions %d want %d", success, want)
			}
			list, e := s.CharacterRevisions(ctx)
			if e != nil || len(list) != 1 || list[0].Revision != 2 || len(list[0].Changes) != 1 {
				t.Fatalf("duplicate revision %v %v", list, e)
			}
		})
	}
}

func TestCharacterRevisionRollsBackFailedReceiptAndBlocksOldChangeSet(t *testing.T) {
	s, r := seedCharacterRevision(t)
	ctx := context.Background()
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	command := asset.ConstraintCommand{Request: p.Request, Digest: p.Digest, IdempotencyKey: "rollback"}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_character_receipt BEFORE INSERT ON story_constraint_changes BEGIN SELECT RAISE(ABORT,'模拟写入失败'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, command, time.Unix(200, 0)); err == nil {
		t.Fatal("expected failure")
	}
	a, _ := s.StoryAsset(ctx, r.AssetID)
	if a.CurrentRevision != 1 {
		t.Fatal("head escaped rollback")
	}
	if _, err = s.StoryAssetRevision(ctx, r.AssetID, 2); err == nil {
		t.Fatal("revision escaped rollback")
	}
	if _, err = s.db.Exec(`DROP TRIGGER fail_character_receipt`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, command, time.Unix(200, 0)); err != nil {
		t.Fatal(err)
	}
	old, _ := s.Character(ctx, "hero")
	baseline, _ := json.Marshal(characterValue{Name: old.Name, Aliases: old.Aliases, Role: old.Role, Summary: old.Summary, Motivation: old.Motivation, Fear: old.Fear, Secret: old.Secret, Arc: old.Arc, Status: old.Status})
	proposed := old
	proposed.Summary = "不安全全局覆盖"
	after, _ := json.Marshal(characterValue{Name: proposed.Name, Aliases: proposed.Aliases, Role: proposed.Role, Summary: proposed.Summary, Motivation: proposed.Motivation, Fear: proposed.Fear, Secret: proposed.Secret, Arc: proposed.Arc, Status: proposed.Status})
	set, err := revision.NewChangeSet("old-change", r.ProjectID, "旧修改", "", "author", []revision.PatchOperationSpec{{ID: "old-op", Kind: revision.OperationUpdateCharacter, TargetID: old.ID, BeforeValue: string(baseline), AfterValue: string(after)}}, time.Unix(300, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.CreateChangeSet(ctx, set); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyChangeSet(ctx, set.ID, time.Unix(400, 0)); !errors.Is(err, asset.ErrChangeUnsupported) {
		t.Fatalf("change set bypass: %v", err)
	}
	stored, _ := s.Character(ctx, "hero")
	if stored.Summary != old.Summary {
		t.Fatal("old change set overwrote original")
	}
}

func TestCharacterRevisionRejectsSemanticallyUnchangedProfile(t *testing.T) {
	s, r := seedCharacterRevision(t)
	r.Text = `{"role":"protagonist","name":" 林舟 ","aliases":["小舟","小舟"],"summary":"原始简介","motivation":"寻找故乡","fear":"遗忘","secret":"身世","arc":"学会信任"}`
	p, err := s.PreviewConstraintChange(context.Background(), r)
	if err != nil || p.CanApply {
		t.Fatalf("unchanged profile accepted: %#v %v", p, err)
	}
}
