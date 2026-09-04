package creation

import (
	"strings"
	"time"
)

type Session struct {
	ID                           string
	ProjectID                    string
	BranchID                     string
	Stage                        Stage
	AdvanceMode                  AdvanceMode
	IntentText                   string
	SelectedDirectionCandidateID string
	SelectedDirectionCardID      string
	KeptDirectionIDs             []string
	ActiveOperationID            string
	ActiveRunID                  string
	BlockingReason               BlockingReason
	ArchiveDigest                string
	ArchiveIdempotencyKey        string
	ArchiveChangeSetID           string
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
}

// NewSession 为原创项目建立处于灵感阶段的构建会话。
func NewSession(id, projectID, branchID string, now time.Time) (Session, error) {
	id, projectID, branchID = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(branchID)
	if id == "" || projectID == "" || branchID == "" {
		return Session{}, ErrBlankValue
	}
	now = now.UTC()
	return Session{
		ID: id, ProjectID: projectID, BranchID: branchID,
		Stage: StageIntent, AdvanceMode: ModeStepReview,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (session Session) SaveIntent(text string, now time.Time) (Session, error) {
	text = strings.TrimSpace(text)
	if session.ID == "" || text == "" {
		return Session{}, ErrBlankValue
	}
	session.IntentText = text
	session.UpdatedAt = now.UTC()
	return session, nil
}

func (session Session) SetAdvanceMode(mode AdvanceMode, now time.Time) (Session, error) {
	if session.ID == "" || (mode != ModeStepReview && mode != ModeAutoAdvance) {
		return Session{}, ErrBlankValue
	}
	session.AdvanceMode = mode
	session.UpdatedAt = now.UTC()
	return session, nil
}

// Pause 在自动推进必须停下时记录阻断原因，避免恢复后继续付费调用。
func (session Session) Pause(reason BlockingReason, now time.Time) (Session, error) {
	if session.ID == "" || reason == BlockNone {
		return Session{}, ErrBlankValue
	}
	session.BlockingReason = reason
	session.UpdatedAt = now.UTC()
	return session, nil
}

// AttachRun 记录运行槽与所属操作回执，并清除暂停原因。
func (session Session) AttachRun(operationID, runID string, now time.Time) (Session, error) {
	operationID, runID = strings.TrimSpace(operationID), strings.TrimSpace(runID)
	if session.ID == "" || operationID == "" || runID == "" {
		return Session{}, ErrBlankValue
	}
	session.ActiveOperationID = operationID
	session.ActiveRunID = runID
	session.BlockingReason = BlockNone
	session.UpdatedAt = now.UTC()
	return session, nil
}

// ClearActiveRun 清空运行槽与操作回执，不动其他字段。
func (session Session) ClearActiveRun(now time.Time) Session {
	session.ActiveOperationID = ""
	session.ActiveRunID = ""
	session.UpdatedAt = now.UTC()
	return session
}

// RecordArchive 记录一次成功的档案确认，供幂等重放，不改变阶段。
func (session Session) RecordArchive(digest, idempotencyKey, changeSetID string, now time.Time) (Session, error) {
	digest, idempotencyKey, changeSetID = strings.TrimSpace(digest), strings.TrimSpace(idempotencyKey), strings.TrimSpace(changeSetID)
	if session.ID == "" || digest == "" || idempotencyKey == "" || changeSetID == "" {
		return Session{}, ErrBlankValue
	}
	if session.Stage != StageArchiveConfirm {
		return Session{}, ErrInvalidTransition
	}
	if strings.TrimSpace(session.SelectedDirectionCandidateID) == "" {
		return Session{}, ErrStageIncomplete
	}
	session.ArchiveDigest = digest
	session.ArchiveIdempotencyKey = idempotencyKey
	session.ArchiveChangeSetID = changeSetID
	session.BlockingReason = BlockNone
	session.UpdatedAt = now.UTC()
	return session, nil
}

// SelectDirection 记录作者选中的方向候选与其中具体一张卡；只传整套候选 ID 不足以选择方向。
func (session Session) SelectDirection(candidateID, cardID string, now time.Time) (Session, error) {
	candidateID, cardID = strings.TrimSpace(candidateID), strings.TrimSpace(cardID)
	if session.ID == "" || candidateID == "" || cardID == "" {
		return Session{}, ErrBlankValue
	}
	if session.Stage != StageDirections {
		return Session{}, ErrInvalidTransition
	}
	session.SelectedDirectionCandidateID = candidateID
	session.SelectedDirectionCardID = cardID
	session.UpdatedAt = now.UTC()
	return session, nil
}

// KeepDirection 把一张方向卡加入保留列表，重启后可恢复；重复保留幂等。
func (session Session) KeepDirection(cardID string, now time.Time) (Session, error) {
	cardID = strings.TrimSpace(cardID)
	if session.ID == "" || cardID == "" {
		return Session{}, ErrBlankValue
	}
	for _, kept := range session.KeptDirectionIDs {
		if kept == cardID {
			return session, nil
		}
	}
	if len(session.KeptDirectionIDs) >= MaxKeptDirections {
		return Session{}, ErrInvalidTransition
	}
	session.KeptDirectionIDs = append(append([]string(nil), session.KeptDirectionIDs...), cardID)
	session.UpdatedAt = now.UTC()
	return session, nil
}

// UnkeepDirection 把一张方向卡移出保留列表；未保留时幂等。
func (session Session) UnkeepDirection(cardID string, now time.Time) (Session, error) {
	cardID = strings.TrimSpace(cardID)
	if session.ID == "" || cardID == "" {
		return Session{}, ErrBlankValue
	}
	kept := make([]string, 0, len(session.KeptDirectionIDs))
	for _, item := range session.KeptDirectionIDs {
		if item != cardID {
			kept = append(kept, item)
		}
	}
	session.KeptDirectionIDs = kept
	session.UpdatedAt = now.UTC()
	return session, nil
}

// Advance 在当前阶段完成条件满足时进入下一阶段。
func (session Session) Advance(now time.Time) (Session, error) {
	if err := session.ensureComplete(); err != nil {
		return Session{}, err
	}
	next, ok := NextStage(session.Stage)
	if !ok {
		return Session{}, ErrInvalidTransition
	}
	if err := CanAdvance(session.Stage, next); err != nil {
		return Session{}, err
	}
	session.Stage = next
	session.UpdatedAt = now.UTC()
	return session, nil
}

// LookBack 回到已经到达过的更早阶段，不删除已有候选。
func (session Session) LookBack(target Stage, now time.Time) (Session, error) {
	if err := CanLookBack(session.Stage, target); err != nil {
		return Session{}, err
	}
	session.Stage = target
	session.UpdatedAt = now.UTC()
	return session, nil
}

func (session Session) ensureComplete() error {
	switch session.Stage {
	case StageIntent:
		if strings.TrimSpace(session.IntentText) == "" {
			return ErrStageIncomplete
		}
	case StageDirections:
		if strings.TrimSpace(session.SelectedDirectionCandidateID) == "" ||
			strings.TrimSpace(session.SelectedDirectionCardID) == "" {
			return ErrStageIncomplete
		}
	}
	return nil
}

// MustPauseBefore 判定自动推进是否必须在进入目标阶段前停下。
func (session Session) MustPauseBefore(next Stage) bool {
	if session.AdvanceMode != ModeAutoAdvance {
		return false
	}
	return next == StageArchiveConfirm
}

// ShouldAutoContinue 表示恢复后是否可以不经作者确认继续付费运行。
func (session Session) ShouldAutoContinue() bool {
	return session.AdvanceMode == ModeAutoAdvance && session.BlockingReason == BlockNone
}
