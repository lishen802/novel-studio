package assistant

import (
	"strings"
	"time"
	"unicode/utf8"
)

type MessageRole string

const (
	RoleAuthor    MessageRole = "author"
	RoleAssistant MessageRole = "assistant"
)

// Message 保存作者可见的共创消息及其发送时绑定的策划对象。
type Message struct {
	ID             string
	SessionID      string
	Role           MessageRole
	Text           string
	Stage          string
	CandidateID    string
	EntityRef      string
	FieldPath      string
	ActionID       string
	IdempotencyKey string
	CreatedAt      time.Time
}

// NewMessage 校验公开消息；消息只保存展示文本，不接收模型隐藏推理。
func NewMessage(id, sessionID string, role MessageRole, text, stage, candidateID, entityRef, fieldPath, actionID, idempotencyKey string, now time.Time) (Message, error) {
	id, sessionID, text, idempotencyKey = strings.TrimSpace(id), strings.TrimSpace(sessionID), strings.TrimSpace(text), strings.TrimSpace(idempotencyKey)
	if id == "" || sessionID == "" || text == "" || idempotencyKey == "" || utf8.RuneCountInString(text) > 4000 {
		return Message{}, ErrBlankValue
	}
	if role != RoleAuthor && role != RoleAssistant {
		return Message{}, ErrBlankValue
	}
	return Message{
		ID: id, SessionID: sessionID, Role: role, Text: text,
		Stage: strings.TrimSpace(stage), CandidateID: strings.TrimSpace(candidateID),
		EntityRef: strings.TrimSpace(entityRef), FieldPath: strings.TrimSpace(fieldPath),
		ActionID: strings.TrimSpace(actionID), IdempotencyKey: idempotencyKey,
		CreatedAt: now.UTC(),
	}, nil
}
