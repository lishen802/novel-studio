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
