package workspace

import (
	"context"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type formalOutlineReorderStore interface {
	PreviewFormalOutlineReorder(context.Context, asset.FormalOutlineReorderRequest) (asset.FormalOutlineReorderPreview, error)
	ConfirmFormalOutlineReorder(context.Context, asset.FormalOutlineReorderCommand, string, time.Time) (asset.FormalOutlineReorderPreview, error)
	FormalOutlinePositionsAtChapter(context.Context, string, string) (map[string]int, error)
}

// PreviewFormalOutlineReorder 将作者选择绑定到活动作品与真实分支。
func (s *Service) PreviewFormalOutlineReorder(ctx context.Context, firstAssetID, secondAssetID, chapterID, reason string) (asset.FormalOutlineReorderPreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.requireEditableLocked(); err != nil {
		return asset.FormalOutlineReorderPreview{}, err
	}
	store, ok := s.active.store.(formalOutlineReorderStore)
	if !ok {
		return asset.FormalOutlineReorderPreview{}, asset.ErrChangeUnsupported
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return asset.FormalOutlineReorderPreview{}, err
	}
	branch, err := s.constraintBranchLocked()
	if err != nil {
		return asset.FormalOutlineReorderPreview{}, err
	}
	return store.PreviewFormalOutlineReorder(ctx, asset.FormalOutlineReorderRequest{ProjectID: project.ID, BranchID: branch, FirstAssetID: firstAssetID, SecondAssetID: secondAssetID, ChapterID: chapterID, Reason: reason})
}

// ConfirmFormalOutlineReorder 在当前工作区中核对预览并追加双节点版本。
func (s *Service) ConfirmFormalOutlineReorder(ctx context.Context, command asset.FormalOutlineReorderCommand) (asset.FormalOutlineReorderPreview, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return asset.FormalOutlineReorderPreview{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return asset.FormalOutlineReorderPreview{}, err
	}
	store, ok := s.active.store.(formalOutlineReorderStore)
	if !ok {
		return asset.FormalOutlineReorderPreview{}, asset.ErrChangeUnsupported
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return asset.FormalOutlineReorderPreview{}, err
	}
	branch, err := s.constraintBranchLocked()
	if err != nil {
		return asset.FormalOutlineReorderPreview{}, err
	}
	if command.Request.ProjectID != project.ID || command.Request.BranchID != branch {
		return asset.FormalOutlineReorderPreview{}, asset.ErrChangeConflict
	}
	result, err := store.ConfirmFormalOutlineReorder(ctx, command, s.ids.NewID(), s.clock.Now())
	if err != nil {
		return result, s.handleFilesystemWriteFailureLocked(err)
	}
	return result, nil
}

// FormalOutlinePositionsAtChapter 返回该章读取的正式规划位置，不修改档案。
func (s *Service) FormalOutlinePositionsAtChapter(ctx context.Context, chapterID string) (map[string]int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	store, ok := s.active.store.(formalOutlineReorderStore)
	if !ok {
		return nil, asset.ErrChangeUnsupported
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return nil, err
	}
	return store.FormalOutlinePositionsAtChapter(ctx, project.ID, chapterID)
}
