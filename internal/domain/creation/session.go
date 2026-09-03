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
	BlockingReason               BlockingReason
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

func (session Session) SelectDirection(candidateID string, now time.Time) (Session, error) {
	candidateID = strings.TrimSpace(candidateID)
	if session.ID == "" || candidateID == "" {
		return Session{}, ErrBlankValue
	}
	if session.Stage != StageDirections {
		return Session{}, ErrInvalidTransition
	}
	session.SelectedDirectionCandidateID = candidateID
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

func (session Session) ensureComplete() error {
	switch session.Stage {
	case StageIntent:
		if strings.TrimSpace(session.IntentText) == "" {
			return ErrStageIncomplete
		}
	case StageDirections:
		if strings.TrimSpace(session.SelectedDirectionCandidateID) == "" {
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
