package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	revision "github.com/lishen802/novel-studio/internal/domain/revision"
	storyasset "github.com/lishen802/novel-studio/internal/domain/storyasset"
	world "github.com/lishen802/novel-studio/internal/domain/world"
	"sync"
	"testing"
	"time"
)

func seedConstraintChange(t *testing.T) (*Store, storyasset.ConstraintRequest) {
	t.Helper()
	s, p := createTestStore(t)
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	now := time.Unix(100, 0)
	for i, id := range []string{"old", "future", "later"} {
		if err := s.CreateChapter(ctx, mustChapter(t, id, p.ID, id, i+1)); err != nil {
			t.Fatal(err)
		}
	}
	rule, _ := world.NewWorldRule("rule", p.ID, "潮汐", "决定航路", world.RuleScopeProject, "", now)
	if err := s.CreateWorldRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	asset, _ := storyasset.New("asset", p.ID, storyasset.KindWorldRule, storyasset.OriginAuthor, "", "", "", "world_rules", "rule", now)
	payload, _ := json.Marshal(map[string]string{"name": "潮汐", "description": "决定航路"})
	tx, _ := s.db.BeginTx(ctx, nil)
	if err := insertStoryAsset(ctx, tx, asset, string(payload), now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return s, storyasset.ConstraintRequest{ProjectID: p.ID, AssetID: asset.ID, Intent: "future_only", ChapterID: "future", Text: "限制夜间航路", Reason: "后续路线调整"}
}

func TestConstraintChangeKeepsHistoryAndResolvesBoundary(t *testing.T) {
	s, r := seedConstraintChange(t)
	ctx := context.Background()
	old, _ := s.Chapter(ctx, "old")
	next := old
	next.Body = "已写正文"
	next.Revision++
	if err := s.CommitChapter(ctx, next, old); err != nil {
		t.Fatal(err)
	}
	before, _ := s.PreviewConstraintChange(ctx, r)
	if !before.CanApply {
		t.Fatalf("preview: %#v", before)
	}
	command := storyasset.ConstraintCommand{Request: r, Digest: before.Digest, IdempotencyKey: "key"}
	applied, err := s.ApplyConstraintChange(ctx, command, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if applied.Revision != 2 {
		t.Fatal(applied)
	}
	// 边界章开始写作后，原操作回执仍能重放，不会重新写入版本。
	future, _ := s.Chapter(ctx, "future")
	written := future
	written.Body = "按新规则写作"
	written.Revision++
	if err := s.CommitChapter(ctx, written, future); err != nil {
		t.Fatal(err)
	}
	replay, err := s.ApplyConstraintChange(ctx, command, time.Unix(300, 0))
	if err != nil || replay.Revision != 2 {
		t.Fatalf("replay %v %v", replay, err)
	}
	command.Request.Text = "换参"
	if _, err = s.ApplyConstraintChange(ctx, command, time.Unix(300, 0)); !errors.Is(err, storyasset.ErrChangeConflict) {
		t.Fatalf("reuse: %v", err)
	}
	for position, want := range map[int]string{1: "决定航路", 2: r.Text, 3: r.Text} {
		list, e := s.EffectiveConstraints(ctx, position)
		if e != nil || len(list) != 1 || list[0].Text != want {
			t.Fatalf("at %d: %v %v", position, list, e)
		}
	}
	rule, _ := s.WorldRule(ctx, "rule")
	if rule.Description != "决定航路" {
		t.Fatal("original changed")
	}
	chapter, _ := s.Chapter(ctx, "old")
	if chapter.Body != "已写正文" || chapter.Revision != next.Revision {
		t.Fatal("manuscript changed")
	}
	original, _ := s.StoryAssetRevision(ctx, "asset", 1)
	if original.PayloadJSON == "" {
		t.Fatal("lost original")
	}
	r.ChapterID = "later"
	r.Text = "新的更晚约束"
	p, _ := s.PreviewConstraintChange(ctx, r)
	if _, err = s.ApplyConstraintChange(ctx, storyasset.ConstraintCommand{Request: r, Digest: p.Digest, IdempotencyKey: "next"}, time.Unix(400, 0)); err != nil {
		t.Fatal(err)
	}
	list, _ := s.EffectiveConstraints(ctx, 2)
	if list[0].Text != "限制夜间航路" {
		t.Fatal("earlier boundary changed")
	}
}

func TestConstraintChangeBlocksHistoryDraftsAndStalePreview(t *testing.T) {
	for _, mode := range []string{"body", "history", "draft", "after_preview", "locked", "intent", "order"} {
		t.Run(mode, func(t *testing.T) {
			s, r := seedConstraintChange(t)
			ctx := context.Background()
			p, err := s.PreviewConstraintChange(ctx, r)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "body", "history", "after_preview":
				ch, _ := s.Chapter(ctx, "future")
				n := ch
				n.Body = "必须保留"
				n.Revision++
				if err := s.CommitChapter(ctx, n, ch); err != nil {
					t.Fatal(err)
				}
				if mode == "history" {
					empty := n
					empty.Body = ""
					empty.Revision++
					if err := s.CommitChapter(ctx, empty, n); err != nil {
						t.Fatal(err)
					}
				}
			case "draft":
				ch, _ := s.Chapter(ctx, "future")
				if err := s.SaveDraft(ctx, ch.ID, "未保存输入", ch.Revision); err != nil {
					t.Fatal(err)
				}
			case "locked":
				s.db.Exec(`UPDATE story_assets SET locked=1 WHERE id='asset'`)
			case "intent":
				r.Intent = "retcon"
			case "order":
				s.db.Exec(`UPDATE chapters SET title='预览后改名' WHERE id='future'`)
			}
			if mode == "body" || mode == "history" || mode == "draft" || mode == "locked" {
				blocked, e := s.PreviewConstraintChange(ctx, r)
				if e != nil || blocked.CanApply || blocked.BlockedReason == "" {
					t.Fatalf("unsafe preview: %+v %v", blocked, e)
				}
			}

			_, err = s.ApplyConstraintChange(ctx, storyasset.ConstraintCommand{Request: r, Digest: p.Digest, IdempotencyKey: "k"}, time.Unix(300, 0))
			if err == nil {
				t.Fatal("unsafe change allowed")
			}
			a, _ := s.StoryAsset(ctx, "asset")
			if a.CurrentRevision != 1 {
				t.Fatal("partial write")
			}
		})
	}
}

func TestConstraintChangeInvalidatesFutureCandidatesOnly(t *testing.T) {
	s, r := seedConstraintChange(t)
	ctx := context.Background()
	session, candidate := seedReviewCandidate(t, s, r.ProjectID)
	for _, id := range []string{"old", "future"} {
		ch, _ := s.Chapter(ctx, id)
		set, err := revision.NewChapterBodyChangeSet("set-"+id, "op-"+id, r.ProjectID, id, "候选", "", "author", ch.Revision, ch.Body, "旧设定下的候选", "", time.Unix(200, 0))
		if err != nil {
			t.Fatal(err)
		}
		if err = s.CreateChangeSet(ctx, set); err != nil {
			t.Fatal(err)
		}
	}
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, storyasset.ConstraintCommand{Request: r, Digest: p.Digest, IdempotencyKey: "once"}, time.Unix(300, 0)); err != nil {
		t.Fatal(err)
	}
	past, _ := s.ChangeSet(ctx, "set-old")
	future, _ := s.ChangeSet(ctx, "set-future")
	if past.Status != revision.ChangeSetPending || future.Status != revision.ChangeSetConflicted {
		t.Fatalf("past %s, future %s", past.Status, future.Status)
	}
	if _, err = s.ApplyChangeSet(ctx, "set-future", time.Unix(400, 0)); err == nil {
		t.Fatal("old future candidate applied")
	}
	candidates, _ := s.ListCreationCandidates(ctx, session.ID)
	if len(candidates) != 1 || string(candidates[0].Status) != "stale" || candidates[0].ID != candidate.ID {
		t.Fatal(candidates)
	}
	updated, _ := s.CreationSession(ctx, r.ProjectID)
	if updated.ContentVersion <= session.ContentVersion {
		t.Fatal("old authorization not invalidated")
	}
}

func TestConstraintAnchorRevisionPreservesScopeAndAtomicity(t *testing.T) {
	s, r := seedConstraintChange(t)
	ctx := context.Background()
	now := time.Unix(100, 0)
	anchor, _ := storyasset.New("anchor", r.ProjectID, storyasset.KindHardAnchor, storyasset.OriginAuthor, "", "", "", "", "", now)
	tx, _ := s.db.BeginTx(ctx, nil)
	if err := insertStoryAsset(ctx, tx, anchor, `{"statement":"原硬约束"}`, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO story_anchors(id,asset_id,project_id,strength,scope_kind,scope_id,statement,created_at) VALUES('anchor','anchor',?,'hard','proposal','asset','原硬约束',?)`, r.ProjectID, formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	r.AssetID = "anchor"
	r.Text = "新硬约束"
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	command := storyasset.ConstraintCommand{Request: r, Digest: p.Digest, IdempotencyKey: "anchor-once"}
	if _, err = s.db.Exec(`CREATE TRIGGER fail_constraint_receipt BEFORE INSERT ON story_constraint_changes BEGIN SELECT RAISE(ABORT,'模拟磁盘写入失败'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, command, now); err == nil {
		t.Fatal("expected storage failure")
	}
	a, _ := s.StoryAsset(ctx, "anchor")
	if a.CurrentRevision != 1 {
		t.Fatal("partial version commit")
	}
	if _, err = s.StoryAssetRevision(ctx, "anchor", 2); err == nil {
		t.Fatal("revision escaped rollback")
	}
	if _, err = s.db.Exec(`DROP TRIGGER fail_constraint_receipt`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ApplyConstraintChange(ctx, command, now); err != nil {
		t.Fatal(err)
	}
	list, err := s.EffectiveConstraints(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, v := range list {
		if v.AssetID == "anchor" {
			found = true
			if v.ScopeKind != "proposal" || v.ScopeID != "asset" || v.Text != r.Text || v.Revision != 2 {
				t.Fatal(v)
			}
		}
	}
	if !found {
		t.Fatal("missing anchor")
	}
	anchors, _ := s.ListStoryAnchors(ctx)
	if len(anchors) != 1 || anchors[0].Statement != "原硬约束" {
		t.Fatal("original anchor changed")
	}
	same, err := s.PreviewConstraintChange(ctx, r)
	if err != nil || same.CanApply {
		t.Fatal("same boundary accepted", err)
	}
	if _, err = s.db.Exec(`UPDATE chapters SET position=10 WHERE id='future'`); err == nil {
		t.Fatal("boundary order silently changed")
	}
	rule, _ := s.WorldRule(ctx, "rule")
	rule.Description = "绕过范围"
	if err = s.UpdateWorldRule(ctx, rule); !errors.Is(err, storyasset.ErrChangeUnsupported) {
		t.Fatalf("global overwrite %v", err)
	}
	if err = s.ArchiveWorldRule(ctx, "rule", now); !errors.Is(err, storyasset.ErrChangeUnsupported) {
		t.Fatalf("global removal %v", err)
	}
}

func TestConstraintChangeConcurrentConfirmation(t *testing.T) {
	for _, sameKey := range []bool{true, false} {
		t.Run(map[bool]string{true: "重复提交", false: "竞争预览"}[sameKey], func(t *testing.T) {
			s, r := seedConstraintChange(t)
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
					_, e := s.ApplyConstraintChange(ctx, storyasset.ConstraintCommand{Request: r, Digest: p.Digest, IdempotencyKey: key}, time.Unix(300, 0))
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
				} else if !errors.Is(e, storyasset.ErrChangeConflict) {
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
			list, e := s.Constraints(ctx)
			if e != nil || len(list) != 1 || list[0].Revision != 2 || len(list[0].Changes) != 1 {
				t.Fatalf("duplicate revision %v %v", list, e)
			}
		})
	}
}

func TestConstraintChangeBlocksRunningModelWork(t *testing.T) {
	s, r := seedConstraintChange(t)
	ctx := context.Background()
	seedReviewCandidate(t, s, r.ProjectID)
	if _, err := s.db.Exec(`UPDATE creation_sessions SET active_run_id='running'`); err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewConstraintChange(ctx, r)
	if err != nil || p.CanApply || p.BlockedReason == "" {
		t.Fatalf("running work allowed %v %v", p, err)
	}
	if _, err = s.ApplyConstraintChange(ctx, storyasset.ConstraintCommand{Request: r, Digest: p.Digest, IdempotencyKey: "run"}, time.Unix(300, 0)); !errors.Is(err, storyasset.ErrChangeConflict) {
		t.Fatal(err)
	}
}
