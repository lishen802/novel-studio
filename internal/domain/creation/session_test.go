package creation

import (
	"errors"
	"testing"
	"time"
)

func TestNewSessionStartsAtIntentWithStepReview(t *testing.T) {
	now := time.Unix(100, 0)
	session, err := NewSession("session-1", "project-1", "branch-1", now)
	if err != nil {
		t.Fatal(err)
	}
	if session.Stage != StageIntent || session.AdvanceMode != ModeStepReview {
		t.Fatalf("session = %#v", session)
	}
	if session.CreatedAt.Location() != time.UTC || session.UpdatedAt.Location() != time.UTC {
		t.Fatalf("times must be UTC: %#v", session)
	}
}

func TestSaveIntentRejectsBlankText(t *testing.T) {
	session := mustSession(t)
	if _, err := session.SaveIntent("  ", time.Unix(101, 0)); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("blank intent error = %v", err)
	}
	updated, err := session.SaveIntent(" 民国江心岛灯塔 ", time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	if updated.IntentText != "民国江心岛灯塔" {
		t.Fatalf("intent = %q", updated.IntentText)
	}
}

func TestAdvanceFromIntentRequiresIntentText(t *testing.T) {
	session := mustSession(t)
	if _, err := session.Advance(time.Unix(102, 0)); !errors.Is(err, ErrStageIncomplete) {
		t.Fatalf("advance without intent error = %v", err)
	}
}

func TestAdvanceFromDirectionsRequiresSelectedDirection(t *testing.T) {
	session := mustSession(t)
	session, err := session.SaveIntent("灯塔失踪", time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.Advance(time.Unix(102, 0))
	if err != nil {
		t.Fatal(err)
	}
	if session.Stage != StageDirections {
		t.Fatalf("stage = %q", session.Stage)
	}
	if _, err := session.Advance(time.Unix(103, 0)); !errors.Is(err, ErrStageIncomplete) {
		t.Fatalf("advance without selection error = %v", err)
	}
	session, err = session.SelectDirection("dir-1", time.Unix(104, 0))
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.Advance(time.Unix(105, 0))
	if err != nil {
		t.Fatal(err)
	}
	if session.Stage != StageProposal {
		t.Fatalf("stage = %q", session.Stage)
	}
}

func TestAutoAdvancePausesBeforeArchiveWrite(t *testing.T) {
	session := mustSession(t)
	session.AdvanceMode = ModeAutoAdvance
	session.Stage = StageRoadmap
	if !session.MustPauseBefore(StageArchiveConfirm) {
		t.Fatal("auto advance must pause before archive confirm")
	}
	if session.MustPauseBefore(StageProposal) {
		t.Fatal("auto advance should not pause a candidate-only stage")
	}
}

func TestResumeDoesNotClearBlockingReason(t *testing.T) {
	session := mustSession(t)
	session.BlockingReason = BlockCostAuthorization
	if session.ShouldAutoContinue() {
		t.Fatal("resume must not auto continue while blocked")
	}
}

func TestSetAdvanceModeAndAttachRun(t *testing.T) {
	session := mustSession(t)
	updated, err := session.SetAdvanceMode(ModeAutoAdvance, time.Unix(110, 0))
	if err != nil {
		t.Fatal(err)
	}
	updated, err = updated.AttachRun("run-1", time.Unix(111, 0))
	if err != nil {
		t.Fatal(err)
	}
	if updated.AdvanceMode != ModeAutoAdvance || updated.ActiveRunID != "run-1" || updated.BlockingReason != BlockNone {
		t.Fatalf("session = %#v", updated)
	}
	paused, err := updated.Pause(BlockFormalWrite, time.Unix(112, 0))
	if err != nil {
		t.Fatal(err)
	}
	if paused.ShouldAutoContinue() {
		t.Fatal("paused session must not auto continue")
	}
	cleared := paused.ClearActiveRun(time.Unix(113, 0))
	if cleared.ActiveRunID != "" {
		t.Fatalf("active run = %q", cleared.ActiveRunID)
	}
}

func mustSession(t *testing.T) Session {
	t.Helper()
	session, err := NewSession("session-1", "project-1", "branch-1", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	return session
}
