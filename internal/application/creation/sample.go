package creation

import (
	"context"
	"database/sql"
	"errors"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type SaveSampleRequest struct {
	ProjectID              string   `json:"projectId"`
	SessionID              string   `json:"sessionId"`
	ExpectedVersion        int64    `json:"expectedVersion"`
	ExpectedRunSlotVersion int64    `json:"expectedRunSlotVersion"`
	DirectionCandidateID   string   `json:"directionCandidateId"`
	DirectionCardID        string   `json:"directionCardId"`
	PointOfView            string   `json:"pointOfView"`
	Tone                   string   `json:"tone"`
	Constraints            []string `json:"constraints"`
	TargetRunes            int      `json:"targetRunes"`
}
type SampleView struct {
	creationdomain.SampleHandoff
	Status string `json:"status"`
}

// sampleDirection 只允许当前有效的已选方向进入试写准备，不能绕过来源和质量校验。
func sampleDirection(s creationdomain.Session, candidates []creationdomain.Candidate) (creationdomain.Candidate, error) {
	if s.EntryKind() != creationdomain.EntryIdea {
		return creationdomain.Candidate{}, creationdomain.ErrInvalidSampleHandoff
	}
	c, err := requiredSelectedDirectionCandidate(s, candidates)
	if err != nil {
		return c, err
	}
	current, ok := currentCandidate(candidates, creationdomain.StageDirections)
	if !ok || current.ID != c.ID || c.Blocked() || c.Status == creationdomain.StatusStale {
		return c, creationdomain.ErrInvalidSampleHandoff
	}
	if _, err := directionCardByID(c, s.SelectedDirectionCardID); err != nil {
		return c, err
	}
	if err := validateCandidateDependency(s, candidates, c); err != nil {
		return c, err
	}
	return c, nil
}

// SaveSample 冻结局部创作输入，仅写交接记录，不执行模型或创建正式章节。
func (service *Service) SaveSample(ctx context.Context, r SaveSampleRequest) (creationdomain.SampleHandoff, error) {
	s, err := service.loadSession(ctx, r.ProjectID)
	if err != nil {
		return creationdomain.SampleHandoff{}, err
	}
	if s.ID != r.SessionID || s.ContentVersion != r.ExpectedVersion || s.RunSlotVersion != r.ExpectedRunSlotVersion {
		return creationdomain.SampleHandoff{}, creationdomain.ErrSessionVersionConflict
	}
	if s.ActiveRunID != "" || s.ActiveOperationID != "" {
		return creationdomain.SampleHandoff{}, ErrRunActive
	}
	candidates, err := service.store.ListCandidates(ctx, s.ID)
	if err != nil {
		return creationdomain.SampleHandoff{}, err
	}
	c, err := sampleDirection(s, candidates)
	if err != nil {
		return creationdomain.SampleHandoff{}, err
	}
	if r.DirectionCandidateID != c.ID || r.DirectionCardID != s.SelectedDirectionCardID {
		return creationdomain.SampleHandoff{}, creationdomain.ErrInvalidSampleHandoff
	}
	h := creationdomain.SampleHandoff{ID: service.ids.NewID(), SessionID: s.ID, DirectionCandidateID: c.ID, DirectionContentSHA256: c.ContentSHA256, DirectionCardID: r.DirectionCardID, SourceDigest: liveUpstreamDigest(s, candidates, creationdomain.StageDirections), PointOfView: r.PointOfView, Tone: r.Tone, Constraints: append([]string{}, r.Constraints...), TargetRunes: r.TargetRunes, CreatedAt: service.clock.Now()}
	if err := h.Validate(c); err != nil {
		return h, err
	}
	return h, service.store.SaveSample(ctx, s, c, h)
}

// projectSample 按当前来源重算有效性，恢复准备不会隐式授权 R5 生成。
func (service *Service) projectSample(ctx context.Context, s creationdomain.Session, candidates []creationdomain.Candidate, v *WorkbenchView) error {
	c, sourceErr := sampleDirection(s, candidates)
	v.CanPrepareSample = sourceErr == nil && s.ActiveRunID == "" && s.ActiveOperationID == ""
	h, err := service.store.LatestSample(ctx, s.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	status := "prepared"
	if sourceErr != nil || h.Validate(c) != nil || h.DirectionCardID != s.SelectedDirectionCardID || h.SourceDigest != liveUpstreamDigest(s, candidates, creationdomain.StageDirections) {
		status = "stale"
	}
	v.SampleHandoff = &SampleView{SampleHandoff: h, Status: status}
	return nil
}
