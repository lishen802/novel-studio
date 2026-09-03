package assistant

import (
	"errors"
	"testing"
	"time"
)

func TestNewActionStartsPendingConfirmation(t *testing.T) {
	now := time.Unix(300, 0)
	action, err := NewAction(
		"act-1", "session-1", "project-1", "branch-1",
		OpReviseFields, RiskPaid, []string{"candidate:cand-1"},
		[]string{"cand-1@1"}, `{"paths":["/title"]}`, "hash-1", "idem-1",
		"0.02", now, now.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if action.State != StatePendingConfirmation {
		t.Fatalf("state = %q", action.State)
	}
}

func TestConfirmRejectsHashMismatchAndWrongState(t *testing.T) {
	action := mustAction(t)
	if _, err := action.Confirm("wrong-hash", "idem-1", time.Unix(301, 0)); !errors.Is(err, ErrArgumentMismatch) {
		t.Fatalf("hash mismatch error = %v", err)
	}
	confirmed, err := action.Confirm("hash-1", "idem-1", time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.State != StateExecuting {
		t.Fatalf("state = %q", confirmed.State)
	}
	if _, err := confirmed.Confirm("hash-1", "idem-1", time.Unix(302, 0)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("double confirm error = %v", err)
	}
}

func TestRejectAndExpireAndConflict(t *testing.T) {
	action := mustAction(t)
	rejected, err := action.Reject(time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	if rejected.State != StateRejected {
		t.Fatalf("rejected = %q", rejected.State)
	}
	expired, err := mustAction(t).Expire(time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	if expired.State != StateExpired {
		t.Fatalf("expired = %q", expired.State)
	}
	conflicted, err := mustAction(t).Conflict("base_changed", time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	if conflicted.State != StateConflict {
		t.Fatalf("conflict = %q", conflicted.State)
	}
}

func TestReadOnlyExplanationDoesNotNeedAction(t *testing.T) {
	if RequiresAction(OpExplain) {
		t.Fatal("read-only explain must not require action")
	}
	if !RequiresAction(OpReviseFields) || !RequiresAction(OpRegenerateCandidate) {
		t.Fatal("paid operations must require action")
	}
	if AllowedAsAssistantOperation(OpConfirmArchive) {
		t.Fatal("archive confirm must be an author command, not an assistant operation")
	}
}

func TestOutcomeUnknownForbidsReplay(t *testing.T) {
	action := mustAction(t)
	executing, err := action.Confirm("hash-1", "idem-1", time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := executing.MarkOutcomeUnknown("run-timeout", time.Unix(302, 0))
	if err != nil {
		t.Fatal(err)
	}
	if unknown.CanReplay() {
		t.Fatal("outcome_unknown must not replay")
	}
	if _, err := unknown.Confirm("hash-1", "idem-1", time.Unix(303, 0)); !errors.Is(err, ErrReplayForbidden) {
		t.Fatalf("replay error = %v", err)
	}
}

func TestSucceedAndFailRequireExecuting(t *testing.T) {
	if _, err := mustAction(t).Succeed([]string{"cand-2"}, time.Unix(301, 0)); !errors.Is(err, ErrInvalidTransition) {
		t.Fatal("pending action must not succeed")
	}
	executing, err := mustAction(t).Confirm("hash-1", "idem-1", time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	succeeded, err := executing.Succeed([]string{"cand-2"}, time.Unix(302, 0))
	if err != nil || succeeded.State != StateSucceeded || len(succeeded.ResultRefs) != 1 {
		t.Fatalf("succeed = %#v, err=%v", succeeded, err)
	}
}

func TestAllowedFieldPathRejectsUnknownPointers(t *testing.T) {
	if !AllowedFieldPath("proposal", "/title") || AllowedFieldPath("proposal", "/internal_node") {
		t.Fatal("proposal allowlist mismatch")
	}
	if AllowedFieldPath("proposal", "/prompt") {
		t.Fatal("arbitrary paths must be rejected")
	}
}

func TestConfirmExpiredAction(t *testing.T) {
	action := mustAction(t)
	if _, err := action.Confirm("hash-1", "idem-1", action.ExpiresAt); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired confirm error = %v", err)
	}
}

func TestNewSessionBindsProjectAndBranch(t *testing.T) {
	session, err := NewSession("asst-1", "project-1", "branch-1", time.Unix(300, 0))
	if err != nil {
		t.Fatal(err)
	}
	if session.Kind != KindCreation || session.ProjectID != "project-1" {
		t.Fatalf("session = %#v", session)
	}
	if _, err := NewSession("asst-1", "", "branch-1", time.Unix(300, 0)); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("blank project error = %v", err)
	}
}

func mustAction(t *testing.T) Action {
	t.Helper()
	action, err := NewAction(
		"act-1", "session-1", "project-1", "branch-1",
		OpReviseFields, RiskPaid, []string{"candidate:cand-1"},
		[]string{"cand-1@1"}, `{"paths":["/title"]}`, "hash-1", "idem-1",
		"0.02", time.Unix(300, 0), time.Unix(300, 0).Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	return action
}
