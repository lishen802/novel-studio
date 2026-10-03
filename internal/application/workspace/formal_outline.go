package workspace

import (
	"context"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type formalOutlineStore interface {
	PreviewFormalOutlineAppend(context.Context, asset.FormalOutlineAppendRequest) (asset.FormalOutlineAppendPreview, error)
	ConfirmFormalOutlineAppend(context.Context, asset.FormalOutlineAppendCommand, string, string, time.Time) (asset.FormalOutlineAppendPreview, error)
	FormalOutlineIntroductions(context.Context, string) (map[string]int, error)
}

// PreviewFormalOutlineAppend 从当前项目和真实活动分支冻结未来新增范围。
func (s *Service) PreviewFormalOutlineAppend(ctx context.Context, parentAssetID, chapterID, title, summary, goal, conflict, outcome, reason string) (asset.FormalOutlineAppendPreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.requireEditableLocked(); err != nil {
		return asset.FormalOutlineAppendPreview{}, err
	}
	store, ok := s.active.store.(formalOutlineStore)
	if !ok {
		return asset.FormalOutlineAppendPreview{}, asset.ErrChangeUnsupported
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return asset.FormalOutlineAppendPreview{}, err
	}
	branchID, err := s.constraintBranchLocked()
	if err != nil {
		return asset.FormalOutlineAppendPreview{}, err
	}
	return store.PreviewFormalOutlineAppend(ctx, asset.FormalOutlineAppendRequest{ProjectID: project.ID, BranchID: branchID, ParentAssetID: parentAssetID, ChapterID: chapterID, Title: title, Summary: summary, Goal: goal, Conflict: conflict, Outcome: outcome, Reason: reason})
}

// ConfirmFormalOutlineAppend 以预览参数和幂等键在当前项目事务中新增正式节点。
func (s *Service) ConfirmFormalOutlineAppend(ctx context.Context, command asset.FormalOutlineAppendCommand) (asset.FormalOutlineAppendPreview, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return asset.FormalOutlineAppendPreview{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return asset.FormalOutlineAppendPreview{}, err
	}
	store, ok := s.active.store.(formalOutlineStore)
	if !ok {
		return asset.FormalOutlineAppendPreview{}, asset.ErrChangeUnsupported
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return asset.FormalOutlineAppendPreview{}, err
	}
	branchID, err := s.constraintBranchLocked()
	if err != nil {
		return asset.FormalOutlineAppendPreview{}, err
	}
	if command.Request.ProjectID != project.ID || command.Request.BranchID != branchID {
		return asset.FormalOutlineAppendPreview{}, asset.ErrChangeConflict
	}
	result, err := store.ConfirmFormalOutlineAppend(ctx, command, s.ids.NewID(), s.ids.NewID(), s.clock.Now())
	if err != nil {
		return result, s.handleFilesystemWriteFailureLocked(err)
	}
	return result, nil
}
