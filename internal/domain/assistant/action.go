package assistant

import (
	"strings"
	"time"
)

var (
	ErrBlankValue        = err("assistant value must not be blank")
	ErrInvalidTransition = err("invalid assistant action transition")
	ErrArgumentMismatch  = err("assistant action arguments changed")
	ErrInvalidOperation  = err("invalid assistant operation")
	ErrReplayForbidden   = err("assistant action must not be replayed")
	ErrExpired           = err("assistant action expired")
	ErrFieldPath         = err("assistant field path is not allowed")
)

type errorString string

func (e errorString) Error() string { return string(e) }

func err(text string) errorString { return errorString(text) }

type Operation string

const (
	OpExplain             Operation = "explain"
	OpRegenerateCandidate Operation = "regenerate_candidate"
	OpReviseFields        Operation = "revise_fields"
	OpCombineDirections   Operation = "combine_directions"
	OpEditLocks           Operation = "edit_locks"
	OpConfirmArchive      Operation = "confirm_archive"
)

type Risk string

const (
	RiskRead  Risk = "read"
	RiskLow   Risk = "low"
	RiskPaid  Risk = "paid"
	RiskWrite Risk = "write"
)

type State string

const (
	StatePendingConfirmation State = "pending_confirmation"
	StateExecuting           State = "executing"
	StateSucceeded           State = "succeeded"
	StateRejected            State = "rejected"
	StateExpired             State = "expired"
	StateConflict            State = "conflict"
	StateFailed              State = "failed"
	StateOutcomeUnknown      State = "outcome_unknown"
)

type Action struct {
	ID             string
	SessionID      string
	ProjectID      string
	BranchID       string
	Operation      Operation
	Risk           Risk
	TargetRefs     []string
	BaseVersions   []string
	Arguments      string
	ArgumentsHash  string
	IdempotencyKey string
	EstimatedCost  string
	State          State
	ErrorCode      string
	ResultRefs     []string
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewAction 创建待作者确认的结构化操作，不执行写入或付费调用。
func NewAction(
	id, sessionID, projectID, branchID string,
	operation Operation, risk Risk, targetRefs, baseVersions []string,
	arguments, argumentsHash, idempotencyKey, estimatedCost string,
	now, expiresAt time.Time,
) (Action, error) {
	id, sessionID, projectID, branchID = strings.TrimSpace(id), strings.TrimSpace(sessionID), strings.TrimSpace(projectID), strings.TrimSpace(branchID)
	argumentsHash, idempotencyKey = strings.TrimSpace(argumentsHash), strings.TrimSpace(idempotencyKey)
	if id == "" || sessionID == "" || projectID == "" || branchID == "" || argumentsHash == "" || idempotencyKey == "" {
		return Action{}, ErrBlankValue
	}
	if !AllowedAsAssistantOperation(operation) {
		return Action{}, ErrInvalidOperation
	}
	if !RequiresAction(operation) {
		return Action{}, ErrInvalidOperation
	}
	if risk != RiskFor(operation) {
		return Action{}, ErrInvalidOperation
	}
	now = now.UTC()
	return Action{
		ID: id, SessionID: sessionID, ProjectID: projectID, BranchID: branchID,
		Operation: operation, Risk: risk, TargetRefs: cloneStrings(targetRefs),
		BaseVersions: cloneStrings(baseVersions), Arguments: arguments,
		ArgumentsHash: argumentsHash, IdempotencyKey: idempotencyKey,
		EstimatedCost: strings.TrimSpace(estimatedCost), State: StatePendingConfirmation,
		ResultRefs: []string{}, ExpiresAt: expiresAt.UTC(), CreatedAt: now, UpdatedAt: now,
	}, nil
}

// RiskFor 按操作返回必须先确认的风险等级；只读解释不走 Action。
func RiskFor(operation Operation) Risk {
	switch operation {
	case OpEditLocks:
		return RiskLow
	case OpRegenerateCandidate, OpReviseFields, OpCombineDirections:
		return RiskPaid
	default:
		return RiskRead
	}
}

// AllowedFieldPath 校验前端提交的字段路径是否落在服务端允许列表。
func AllowedFieldPath(kind, path string) bool {
	path = strings.TrimSpace(path)
	switch kind {
	case "proposal":
		switch path {
		case "/title", "/positioning", "/core_conflict", "/protagonist_arc",
			"/sell_point", "/genre", "/growth_cost", "/pacing", "/ending":
			return true
		}
	case "people_world":
		switch path {
		case "/omission_reasons":
			return true
		}
	case "roadmap":
		switch path {
		case "/book":
			return true
		}
	}
	return false
}

func RequiresAction(operation Operation) bool {
	switch operation {
	case OpRegenerateCandidate, OpReviseFields, OpCombineDirections, OpEditLocks, OpConfirmArchive:
		return true
	default:
		return false
	}
}

func AllowedAsAssistantOperation(operation Operation) bool {
	switch operation {
	case OpRegenerateCandidate, OpReviseFields, OpCombineDirections, OpEditLocks:
		return true
	default:
		return false
	}
}

func (action Action) Confirm(argumentsHash, idempotencyKey string, now time.Time) (Action, error) {
	if action.State == StateOutcomeUnknown {
		return Action{}, ErrReplayForbidden
	}
	if action.State != StatePendingConfirmation {
		return Action{}, ErrInvalidTransition
	}
	if !action.ExpiresAt.IsZero() && !now.Before(action.ExpiresAt) {
		return Action{}, ErrExpired
	}
	if strings.TrimSpace(argumentsHash) != action.ArgumentsHash || strings.TrimSpace(idempotencyKey) != action.IdempotencyKey {
		return Action{}, ErrArgumentMismatch
	}
	action.State = StateExecuting
	action.UpdatedAt = now.UTC()
	return action, nil
}

func (action Action) Reject(now time.Time) (Action, error) {
	if action.State != StatePendingConfirmation {
		return Action{}, ErrInvalidTransition
	}
	action.State = StateRejected
	action.UpdatedAt = now.UTC()
	return action, nil
}

func (action Action) Expire(now time.Time) (Action, error) {
	if action.State != StatePendingConfirmation {
		return Action{}, ErrInvalidTransition
	}
	action.State = StateExpired
	action.UpdatedAt = now.UTC()
	return action, nil
}

func (action Action) Conflict(code string, now time.Time) (Action, error) {
	if action.State != StatePendingConfirmation && action.State != StateExecuting {
		return Action{}, ErrInvalidTransition
	}
	action.State = StateConflict
	action.ErrorCode = strings.TrimSpace(code)
	action.UpdatedAt = now.UTC()
	return action, nil
}

func (action Action) MarkOutcomeUnknown(code string, now time.Time) (Action, error) {
	if action.State != StateExecuting {
		return Action{}, ErrInvalidTransition
	}
	action.State = StateOutcomeUnknown
	action.ErrorCode = strings.TrimSpace(code)
	action.UpdatedAt = now.UTC()
	return action, nil
}

func (action Action) Succeed(resultRefs []string, now time.Time) (Action, error) {
	if action.State != StateExecuting {
		return Action{}, ErrInvalidTransition
	}
	action.State = StateSucceeded
	action.ResultRefs = cloneStrings(resultRefs)
	action.UpdatedAt = now.UTC()
	return action, nil
}

func (action Action) Fail(code string, now time.Time) (Action, error) {
	if action.State != StateExecuting {
		return Action{}, ErrInvalidTransition
	}
	action.State = StateFailed
	action.ErrorCode = strings.TrimSpace(code)
	action.UpdatedAt = now.UTC()
	return action, nil
}

func (action Action) CanReplay() bool {
	return false
}

func cloneStrings(values []string) []string {
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
