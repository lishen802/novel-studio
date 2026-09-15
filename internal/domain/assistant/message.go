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

type MessageKind string

const (
	KindDiscussion      MessageKind = "discussion"
	KindRevisionRequest MessageKind = "revision_request"
	KindSystem          MessageKind = "system"
)

type MessageStatus string

const (
	MessagePending        MessageStatus = "pending"
	MessageRunning        MessageStatus = "running"
	MessageAnswered       MessageStatus = "answered"
	MessageFailed         MessageStatus = "failed"
	MessageOutcomeUnknown MessageStatus = "outcome_unknown"
)

// Suggestion 是讨论产出的字段修改建议，不能直接改候选。
type Suggestion struct {
	ID            string    `json:"id"`
	Operation     Operation `json:"operation"`
	Paths         []string  `json:"paths"`
	Instruction   string    `json:"instruction"`
	PreservePaths []string  `json:"preserve_paths"`
	Rationale     string    `json:"rationale"`
}

// Message 保存作者可见的共创消息及其发送时绑定的策划对象。
type Message struct {
	ID              string
	SessionID       string
	Role            MessageRole
	Kind            MessageKind
	Status          MessageStatus
	Text            string
	Stage           string
	CandidateID     string
	EntityRef       string
	FieldPath       string
	ActionID        string
	ReplyToID       string
	AuthorizationID string
	RunID           string
	OperationID     string
	ErrorCode       string
	Suggestions     []Suggestion
	IdempotencyKey  string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewMessage 校验公开消息；缺省按修订请求已回复处理，兼容旧共创入口。
func NewMessage(id, sessionID string, role MessageRole, text, stage, candidateID, entityRef, fieldPath, actionID, idempotencyKey string, now time.Time) (Message, error) {
	message, err := newVisibleMessage(id, sessionID, role, KindRevisionRequest, MessageAnswered, text, stage, candidateID, entityRef, fieldPath, actionID, idempotencyKey, now)
	if err != nil {
		return Message{}, err
	}
	return message, nil
}

// NewDiscussionMessage 保存讨论轮次的作者或 AI 公开文本，并冻结发送时的目标。
func NewDiscussionMessage(id, sessionID string, role MessageRole, status MessageStatus, text, stage, candidateID, entityRef, fieldPath, actionID, authorizationID, idempotencyKey string, now time.Time) (Message, error) {
	message, err := newVisibleMessage(id, sessionID, role, KindDiscussion, status, text, stage, candidateID, entityRef, fieldPath, actionID, idempotencyKey, now)
	if err != nil {
		return Message{}, err
	}
	message.AuthorizationID = strings.TrimSpace(authorizationID)
	return message, nil
}

func newVisibleMessage(id, sessionID string, role MessageRole, kind MessageKind, status MessageStatus, text, stage, candidateID, entityRef, fieldPath, actionID, idempotencyKey string, now time.Time) (Message, error) {
	id, sessionID, text, idempotencyKey = strings.TrimSpace(id), strings.TrimSpace(sessionID), strings.TrimSpace(text), strings.TrimSpace(idempotencyKey)
	if id == "" || sessionID == "" || text == "" || idempotencyKey == "" || utf8.RuneCountInString(text) > 4000 {
		return Message{}, ErrBlankValue
	}
	if role != RoleAuthor && role != RoleAssistant {
		return Message{}, ErrBlankValue
	}
	if !validMessageKind(kind) || !validMessageStatus(status) {
		return Message{}, ErrBlankValue
	}
	now = now.UTC()
	return Message{
		ID: id, SessionID: sessionID, Role: role, Kind: kind, Status: status, Text: text,
		Stage: strings.TrimSpace(stage), CandidateID: strings.TrimSpace(candidateID),
		EntityRef: strings.TrimSpace(entityRef), FieldPath: strings.TrimSpace(fieldPath),
		ActionID: strings.TrimSpace(actionID), IdempotencyKey: idempotencyKey,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func validMessageKind(kind MessageKind) bool {
	return kind == KindDiscussion || kind == KindRevisionRequest || kind == KindSystem
}

func validMessageStatus(status MessageStatus) bool {
	switch status {
	case MessagePending, MessageRunning, MessageAnswered, MessageFailed, MessageOutcomeUnknown:
		return true
	default:
		return false
	}
}

// MarkRunning 把作者消息标为正在分析，并绑定运行。
func (message Message) MarkRunning(runID, operationID string, now time.Time) (Message, error) {
	if message.Role != RoleAuthor || (message.Status != MessagePending && message.Status != MessageRunning) {
		return Message{}, ErrInvalidTransition
	}
	runID, operationID = strings.TrimSpace(runID), strings.TrimSpace(operationID)
	if runID == "" || operationID == "" {
		return Message{}, ErrBlankValue
	}
	message.Status = MessageRunning
	message.RunID = runID
	message.OperationID = operationID
	message.UpdatedAt = now.UTC()
	return message, nil
}

// MarkAnswered 记录对应的公开回复已落库。
func (message Message) MarkAnswered(now time.Time) (Message, error) {
	if message.Status != MessageRunning && message.Status != MessagePending {
		return Message{}, ErrInvalidTransition
	}
	message.Status = MessageAnswered
	message.UpdatedAt = now.UTC()
	return message, nil
}

// MarkFailed 记录确定失败，不伪造已完成回复。
func (message Message) MarkFailed(code string, now time.Time) (Message, error) {
	if message.Status != MessageRunning && message.Status != MessagePending {
		return Message{}, ErrInvalidTransition
	}
	message.Status = MessageFailed
	message.ErrorCode = strings.TrimSpace(code)
	message.UpdatedAt = now.UTC()
	return message, nil
}

// MarkOutcomeUnknown 保留未知结果，禁止当作已回复。
func (message Message) MarkOutcomeUnknown(code string, now time.Time) (Message, error) {
	if message.Status != MessageRunning {
		return Message{}, ErrInvalidTransition
	}
	message.Status = MessageOutcomeUnknown
	message.ErrorCode = strings.TrimSpace(code)
	message.UpdatedAt = now.UTC()
	return message, nil
}

// WithReply 把 AI 回复绑回原作者消息、授权和运行。
func (message Message) WithReply(replyToID, authorizationID, runID, operationID string) Message {
	message.ReplyToID = strings.TrimSpace(replyToID)
	message.AuthorizationID = strings.TrimSpace(authorizationID)
	message.RunID = strings.TrimSpace(runID)
	message.OperationID = strings.TrimSpace(operationID)
	return message
}

// WithSuggestions 保存已校验的修改建议，不执行修订。
func (message Message) WithSuggestions(suggestions []Suggestion) Message {
	cloned := make([]Suggestion, len(suggestions))
	copy(cloned, suggestions)
	for i := range cloned {
		cloned[i].Paths = cloneStrings(cloned[i].Paths)
		cloned[i].PreservePaths = cloneStrings(cloned[i].PreservePaths)
	}
	message.Suggestions = cloned
	return message
}
