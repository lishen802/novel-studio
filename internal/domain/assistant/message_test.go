package assistant

import (
	"strings"
	"testing"
	"time"
)

func TestNewMessageFreezesVisibleContext(t *testing.T) {
	message, err := NewMessage("msg-1", "session-1", RoleAuthor, " 调整人物动机 ", "proposal", "candidate-1", "character:hero", "/motivation", "action-1", "request-1:author", time.Unix(10, 0))
	if err != nil {
		t.Fatal(err)
	}
	if message.Text != "调整人物动机" || message.CandidateID != "candidate-1" || message.FieldPath != "/motivation" || message.ActionID != "action-1" {
		t.Fatalf("message = %#v", message)
	}
}

func TestNewMessageRejectsHiddenOrOversizedPayload(t *testing.T) {
	if _, err := NewMessage("msg", "session", "hidden", "reasoning", "", "", "", "", "", "key", time.Now()); err == nil {
		t.Fatal("unknown role must fail")
	}
	if _, err := NewMessage("msg", "session", RoleAuthor, strings.Repeat("界", 4001), "", "", "", "", "", "key", time.Now()); err == nil {
		t.Fatal("oversized visible text must fail")
	}
}

func TestDiscussionMessageTracksRunWithoutPretendingAnswered(t *testing.T) {
	now := time.Unix(20, 0)
	author, err := NewDiscussionMessage("msg-2", "session-1", RoleAuthor, MessagePending, "动机哪里不够可信", "people_world", "candidate-1", "character:hero", "/motivation", "", "auth-1", "turn-1:author", now)
	if err != nil {
		t.Fatal(err)
	}
	running, err := author.MarkRunning("run-1", "op-1", now.Add(time.Second))
	if err != nil || running.Status != MessageRunning {
		t.Fatalf("running = %#v err=%v", running, err)
	}
	unknown, err := running.MarkOutcomeUnknown("provider_timeout", now.Add(2*time.Second))
	if err != nil || unknown.Status != MessageOutcomeUnknown {
		t.Fatalf("unknown = %#v err=%v", unknown, err)
	}
	if _, err := unknown.MarkAnswered(now.Add(3 * time.Second)); err == nil {
		t.Fatal("unknown message must not become answered")
	}
}

func TestAuthorizeDiscussionIsPaidAssistantOperation(t *testing.T) {
	if !RequiresAction(OpAuthorizeDiscussion) || RiskFor(OpAuthorizeDiscussion) != RiskPaid || !AllowedAsAssistantOperation(OpAuthorizeDiscussion) {
		t.Fatal("discussion authorization must be a paid assistant action")
	}
	if OpAuthorizeDiscussion == OpReviseFields {
		t.Fatal("discussion authorization must not reuse revise_fields")
	}
}
