package creation

import (
	"errors"
	"testing"
	"time"
)

func TestNewCandidateRecordsLineageAndDigest(t *testing.T) {
	now := time.Unix(200, 0)
	parent := "cand-0"
	candidate, err := NewCandidate(
		"cand-1", "session-1", StageDirections, KindDirectionSet,
		1, parent, "run-1", "artifact-1",
		`{"directions":[{"id":"a"}]}`,
		"story_directions.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.ParentCandidateID != parent || candidate.ContentSHA256 == "" {
		t.Fatalf("candidate = %#v", candidate)
	}
	if candidate.Status != StatusDraft {
		t.Fatalf("status = %q", candidate.Status)
	}
}

func TestRegenerateDoesNotMutateCurrentCandidate(t *testing.T) {
	now := time.Unix(200, 0)
	current, err := NewCandidate("cand-1", "session-1", StageProposal, KindProposal, 1, "", "run-1", "art-1", `{"title":"旧"}`, "story_blueprint.schema", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	next, err := current.Regenerate("cand-2", "run-2", "art-2", `{"title":"新"}`, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if current.PayloadJSON != `{"title":"旧"}` || current.Status != StatusDraft {
		t.Fatalf("current mutated: %#v", current)
	}
	if next.Version != 2 || next.ParentCandidateID != current.ID || next.SupersedesCandidateID != current.ID {
		t.Fatalf("next = %#v", next)
	}
	if current.ContentSHA256 == next.ContentSHA256 {
		t.Fatal("new payload must change digest")
	}
}

func TestLockedFieldRejectsAuthorDraft(t *testing.T) {
	candidate := mustProposal(t)
	locked, err := candidate.LockField("/title", time.Unix(201, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := locked.SaveDraft("cand-draft", `{"title":"改掉锁定标题","positioning":"悬疑"}`, time.Unix(202, 0)); !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("locked draft error = %v", err)
	}
}

func TestSaveDraftCreatesNewVersionLeavingBaselineUntouched(t *testing.T) {
	candidate := mustProposal(t)
	draft, err := candidate.SaveDraft("cand-draft", `{"title":"作者改写","positioning":"悬疑"}`, time.Unix(201, 0))
	if err != nil {
		t.Fatal(err)
	}
	if draft.ID == candidate.ID || draft.RunID != "" || draft.Origin != OriginAuthor {
		t.Fatalf("draft = %#v", draft)
	}
	if candidate.PayloadJSON == draft.PayloadJSON {
		t.Fatal("baseline payload must stay")
	}
}

func TestRegenerateKeepsLockedFieldsFromBaseline(t *testing.T) {
	candidate := mustProposal(t)
	locked, err := candidate.LockField("/title", time.Unix(201, 0))
	if err != nil {
		t.Fatal(err)
	}
	next, err := locked.Regenerate("cand-2", "run-2", "art-2", `{"title":"模型想改标题","positioning":"新定位"}`, time.Unix(202, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !next.Locked("/title") {
		t.Fatal("lock must copy to new version")
	}
	title, err := next.FieldString("/title")
	if err != nil || title != "雾港" {
		t.Fatalf("locked title = %q, err=%v", title, err)
	}
	positioning, err := next.FieldString("/positioning")
	if err != nil || positioning != "新定位" {
		t.Fatalf("unlocked field = %q, err=%v", positioning, err)
	}
}

func mustProposal(t *testing.T) Candidate {
	t.Helper()
	candidate, err := NewCandidate(
		"cand-1", "session-1", StageProposal, KindProposal, 1, "", "run-1", "art-1",
		`{"title":"雾港","positioning":"悬疑"}`, "story_blueprint.schema", 2, time.Unix(200, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	return candidate
}

func TestMarkStaleAndKeepException(t *testing.T) {
	candidate := mustProposal(t)
	stale, err := candidate.MarkStale("方向已改", []string{"dir-1"}, ImpactMaybe, time.Unix(210, 0))
	if err != nil {
		t.Fatal(err)
	}
	if stale.Status != StatusStale || stale.StaleReason == "" {
		t.Fatalf("stale = %#v", stale)
	}
	kept, err := stale.KeepException("upstream-hash", time.Unix(211, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !kept.ExceptionMatches("upstream-hash") {
		t.Fatal("exception should match current upstream hash")
	}
	cleared, err := kept.ClearStale(time.Unix(212, 0))
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Status != StatusDraft || cleared.StaleReason != "" {
		t.Fatalf("cleared = %#v", cleared)
	}
}

func TestRestoreSupersededCandidate(t *testing.T) {
	candidate := mustProposal(t)
	superseded, err := candidate.MarkSuperseded(time.Unix(210, 0))
	if err != nil {
		t.Fatal(err)
	}
	restored, err := superseded.Restore(time.Unix(211, 0))
	if err != nil || restored.Status != StatusDraft {
		t.Fatalf("restore = %#v, err=%v", restored, err)
	}
}

func TestNestedLockFollowsEntityIDThroughReordering(t *testing.T) {
	now := time.Unix(200, 0)
	c, err := NewCandidate("c", "s", StagePeopleWorld, KindPeopleWorld, 1, "", "", "", `{"characters":[{"id":"a","name":"甲","secret":"守住秘密"},{"id":"b","name":"乙","secret":"旧"}]}`, "story_cast.schema", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	c, err = c.LockField("/characters/@a/secret", now)
	if err != nil {
		t.Fatal(err)
	}
	next, err := c.Regenerate("new", "run", "art", `{"characters":[{"id":"b","name":"乙","secret":"新"},{"id":"a","name":"甲","secret":"模型改写"}]}`, now)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := next.FieldString("/characters/@a/secret"); value != "守住秘密" {
		t.Fatal("锁跟随了数组位置")
	}
	if value, _ := next.FieldString("/characters/@b/secret"); value != "新" {
		t.Fatal("误覆盖其他实体")
	}
	if _, err := c.Regenerate("bad", "run", "art", `{"characters":[{"id":"b","secret":"新"}]}`, now); err == nil {
		t.Fatal("不能删除锁定实体")
	}
	if _, err := c.SaveDraft("draft", `{"characters":[{"id":"a","secret":"改掉秘密"}]}`, now); !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("绕过嵌套锁: %v", err)
	}
	if _, err := c.LockField("/characters/0/secret", now); err == nil {
		t.Fatal("数组下标不能作为稳定锁路径")
	}
}
