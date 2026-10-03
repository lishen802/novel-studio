package workspace

import (
	"context"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type formalOutlineRetireStore interface {
	PreviewFormalOutlineRetire(context.Context, asset.FormalOutlineRetireRequest) (asset.FormalOutlineRetirePreview, error)
	ConfirmFormalOutlineRetire(context.Context, asset.FormalOutlineRetireCommand, time.Time) (asset.FormalOutlineRetirePreview, error)
}

// PreviewFormalOutlineRetire 绑定当前项目分支，预览作者未来停用的节点。
func (s *Service) PreviewFormalOutlineRetire(ctx context.Context, assetID, chapterID, reason string) (asset.FormalOutlineRetirePreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := s.requireEditableLocked(); err != nil {
		return asset.FormalOutlineRetirePreview{}, err
	}
	store, ok := s.active.store.(formalOutlineRetireStore)
	if !ok {
		return asset.FormalOutlineRetirePreview{}, asset.ErrChangeUnsupported
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return asset.FormalOutlineRetirePreview{}, err
	}
	branchID, err := s.constraintBranchLocked()
	if err != nil {
		return asset.FormalOutlineRetirePreview{}, err
	}
	return store.PreviewFormalOutlineRetire(ctx, asset.FormalOutlineRetireRequest{ProjectID: project.ID, BranchID: branchID, AssetID: assetID, ChapterID: chapterID, Reason: reason})
}

// ConfirmFormalOutlineRetire 在可写工作区内校验预览摘要和活动分支。
func (s *Service) ConfirmFormalOutlineRetire(ctx context.Context, command asset.FormalOutlineRetireCommand) (asset.FormalOutlineRetirePreview, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return asset.FormalOutlineRetirePreview{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return asset.FormalOutlineRetirePreview{}, err
	}
	store, ok := s.active.store.(formalOutlineRetireStore)
	if !ok {
		return asset.FormalOutlineRetirePreview{}, asset.ErrChangeUnsupported
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return asset.FormalOutlineRetirePreview{}, err
	}
	branchID, err := s.constraintBranchLocked()
	if err != nil {
		return asset.FormalOutlineRetirePreview{}, err
	}
	if command.Request.ProjectID != project.ID || command.Request.BranchID != branchID {
		return asset.FormalOutlineRetirePreview{}, asset.ErrChangeConflict
	}
	result, err := store.ConfirmFormalOutlineRetire(ctx, command, s.clock.Now())
	if err != nil {
		return result, s.handleFilesystemWriteFailureLocked(err)
	}
	return result, nil
}
