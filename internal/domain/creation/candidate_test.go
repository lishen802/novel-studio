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
