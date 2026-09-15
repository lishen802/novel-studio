package assistant

import (
	"strings"
	"time"
)

const (
	AuthPendingConfirmation AuthState = "pending_confirmation"
	AuthActive              AuthState = "active"
	AuthExhausted           AuthState = "exhausted"
	AuthExpired             AuthState = "expired"
	AuthRevoked             AuthState = "revoked"
	AuthStale               AuthState = "stale"
)

type AuthState string

// Authorization 冻结只读讨论的目标、模型与预算，不把额度放在界面状态里。
type Authorization struct {
	ID               string
	SessionID        string
	ProjectID        string
	BranchID         string
	ActionID         string
	CandidateID      string
	CandidateVersion int
	CandidateSHA256  string
	Stage            string
	EntityRef        string
	FieldPaths       []string
	ProfileVersionID string
	ProviderID       string
	ProviderName     string
	Model            string
	ReasoningEffort  string
	MaxCalls         int
	MaxTotalTokens   int
	MaxTurnTokens    int
	ReservedCalls    int
	ReservedTokens   int
	UsedCalls        int
	UsedTokens       int
	State            AuthState
	ExpiresAt        time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// NewAuthorization 创建待确认或已确认前的讨论授权快照，不启动运行。
func NewAuthorization(
	id, sessionID, projectID, branchID, actionID, candidateID, candidateSHA256, stage, entityRef string,
	candidateVersion int, fieldPaths []string,
	profileVersionID, providerID, providerName, model, reasoningEffort string,
	maxCalls, maxTotalTokens, maxTurnTokens int,
	now, expiresAt time.Time,
) (Authorization, error) {
	id, sessionID, projectID, branchID = strings.TrimSpace(id), strings.TrimSpace(sessionID), strings.TrimSpace(projectID), strings.TrimSpace(branchID)
	actionID, candidateID, candidateSHA256 = strings.TrimSpace(actionID), strings.TrimSpace(candidateID), strings.TrimSpace(candidateSHA256)
	profileVersionID, providerID, providerName, model = strings.TrimSpace(profileVersionID), strings.TrimSpace(providerID), strings.TrimSpace(providerName), strings.TrimSpace(model)
	if id == "" || sessionID == "" || projectID == "" || branchID == "" || actionID == "" || candidateID == "" || candidateSHA256 == "" {
		return Authorization{}, ErrBlankValue
	}
	if candidateVersion < 1 || maxCalls < 1 || maxTotalTokens < 1 || maxTurnTokens < 1 || maxTurnTokens > maxTotalTokens {
		return Authorization{}, ErrBlankValue
	}
	if providerName == "" || model == "" || profileVersionID == "" {
		return Authorization{}, ErrBlankValue
	}
	now = now.UTC()
	return Authorization{
		ID: id, SessionID: sessionID, ProjectID: projectID, BranchID: branchID, ActionID: actionID,
		CandidateID: candidateID, CandidateVersion: candidateVersion, CandidateSHA256: candidateSHA256,
		Stage: strings.TrimSpace(stage), EntityRef: strings.TrimSpace(entityRef), FieldPaths: cloneStrings(fieldPaths),
		ProfileVersionID: profileVersionID, ProviderID: providerID, ProviderName: providerName, Model: model,
		ReasoningEffort: strings.TrimSpace(reasoningEffort),
		MaxCalls:        maxCalls, MaxTotalTokens: maxTotalTokens, MaxTurnTokens: maxTurnTokens,
		State: AuthPendingConfirmation, ExpiresAt: expiresAt.UTC(), CreatedAt: now, UpdatedAt: now,
	}, nil
}

// Activate 把已确认授权变为可发送状态。
func (auth Authorization) Activate(now time.Time) (Authorization, error) {
	if auth.State != AuthPendingConfirmation {
		return Authorization{}, ErrInvalidTransition
	}
	if !auth.ExpiresAt.IsZero() && !now.UTC().Before(auth.ExpiresAt) {
		return Authorization{}, ErrExpired
	}
	auth.State = AuthActive
	auth.UpdatedAt = now.UTC()
	return auth, nil
}

// MatchesTarget 校验当前候选版本与哈希仍是授权冻结的目标。
func (auth Authorization) MatchesTarget(candidateID, sha256 string, version int) bool {
	return auth.CandidateID == strings.TrimSpace(candidateID) &&
		auth.CandidateSHA256 == strings.TrimSpace(sha256) &&
		auth.CandidateVersion == version
}

// MatchesProfile 校验模型方案与连接没有变化。
func (auth Authorization) MatchesProfile(profileVersionID, providerID, model string) bool {
	return auth.ProfileVersionID == strings.TrimSpace(profileVersionID) &&
		auth.ProviderID == strings.TrimSpace(providerID) &&
		auth.Model == strings.TrimSpace(model)
}

// CanStartTurn 仅 active 且未过期的授权可以开始新的讨论运行。
func (auth Authorization) CanStartTurn(now time.Time) error {
	if auth.State != AuthActive {
		switch auth.State {
		case AuthExpired:
			return ErrExpired
		case AuthRevoked:
			return ErrInvalidTransition
		case AuthStale:
			return ErrArgumentMismatch
		case AuthExhausted:
			return ErrReplayForbidden
		default:
			return ErrInvalidTransition
		}
	}
	if !auth.ExpiresAt.IsZero() && !now.UTC().Before(auth.ExpiresAt) {
		return ErrExpired
	}
	return nil
}

// RemainingCalls 返回尚未占用的调用次数。
func (auth Authorization) RemainingCalls() int {
	left := auth.MaxCalls - auth.UsedCalls - auth.ReservedCalls
	if left < 0 {
		return 0
	}
	return left
}

// RemainingTokens 返回尚未占用的 Token。
func (auth Authorization) RemainingTokens() int {
	left := auth.MaxTotalTokens - auth.UsedTokens - auth.ReservedTokens
	if left < 0 {
		return 0
	}
	return left
}

// ReserveTurn 原子预留一次调用和单次最大 Token。
func (auth Authorization) ReserveTurn(now time.Time) (Authorization, error) {
	if err := auth.CanStartTurn(now); err != nil {
		return Authorization{}, err
	}
	if auth.RemainingCalls() < 1 || auth.RemainingTokens() < auth.MaxTurnTokens {
		return Authorization{}, ErrReplayForbidden
	}
	auth.ReservedCalls++
	auth.ReservedTokens += auth.MaxTurnTokens
	auth.UpdatedAt = now.UTC()
	if auth.RemainingCalls() == 0 || auth.RemainingTokens() < auth.MaxTurnTokens {
		auth.State = AuthExhausted
	}
	return auth, nil
}

// SettleTurn 按实际用量结算；成功或 Provider 已收到请求时消耗调用次数。
func (auth Authorization) SettleTurn(actualTokens int, now time.Time) (Authorization, error) {
	if auth.ReservedCalls < 1 || auth.ReservedTokens < auth.MaxTurnTokens {
		return Authorization{}, ErrInvalidTransition
	}
	if actualTokens < 0 {
		actualTokens = 0
	}
	auth.ReservedCalls--
	auth.ReservedTokens -= auth.MaxTurnTokens
	auth.UsedCalls++
	auth.UsedTokens += actualTokens
	auth.UpdatedAt = now.UTC()
	if auth.RemainingCalls() < 1 || auth.RemainingTokens() < auth.MaxTurnTokens {
		auth.State = AuthExhausted
	}
	return auth, nil
}

// ReleaseTurn 释放尚未到达 Provider 的预留。
func (auth Authorization) ReleaseTurn(now time.Time) (Authorization, error) {
	if auth.ReservedCalls < 1 || auth.ReservedTokens < auth.MaxTurnTokens {
		return Authorization{}, ErrInvalidTransition
	}
	auth.ReservedCalls--
	auth.ReservedTokens -= auth.MaxTurnTokens
	auth.UpdatedAt = now.UTC()
	if auth.State == AuthExhausted && auth.RemainingCalls() > 0 && auth.RemainingTokens() >= auth.MaxTurnTokens {
		auth.State = AuthActive
	}
	return auth, nil
}

// KeepReservation 结果未知时保留预留，禁止自动重试。
func (auth Authorization) KeepReservation(now time.Time) (Authorization, error) {
	if auth.ReservedCalls < 1 {
		return Authorization{}, ErrInvalidTransition
	}
	auth.UpdatedAt = now.UTC()
	return auth, nil
}

// Expire 把过期授权标为不可再发送。
func (auth Authorization) Expire(now time.Time) (Authorization, error) {
	if auth.State != AuthActive && auth.State != AuthPendingConfirmation {
		return Authorization{}, ErrInvalidTransition
	}
	auth.State = AuthExpired
	auth.UpdatedAt = now.UTC()
	return auth, nil
}

// Revoke 阻止新消息，不删除历史。
func (auth Authorization) Revoke(now time.Time) (Authorization, error) {
	if auth.State != AuthActive && auth.State != AuthPendingConfirmation {
		return Authorization{}, ErrInvalidTransition
	}
	auth.State = AuthRevoked
	auth.UpdatedAt = now.UTC()
	return auth, nil
}

// MarkStale 在候选、锁或模型方案变化后使授权失效。
func (auth Authorization) MarkStale(now time.Time) (Authorization, error) {
	if auth.State != AuthActive && auth.State != AuthPendingConfirmation {
		return Authorization{}, ErrInvalidTransition
	}
	auth.State = AuthStale
	auth.UpdatedAt = now.UTC()
	return auth, nil
}
