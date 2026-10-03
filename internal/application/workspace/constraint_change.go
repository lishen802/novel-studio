package workspace

import (
	"context"
	"fmt"
	"github.com/lishen802/novel-studio/internal/application/license"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
	"time"
)

// constraintStore 是此功能消费的边界，避免把版本修订协议扩散到普通项目存储。
type constraintStore interface {
	Constraints(context.Context) ([]asset.ConstraintView, error)
	PreviewConstraintChange(context.Context, asset.ConstraintRequest) (asset.ConstraintPreview, error)
	ApplyConstraintChange(context.Context, asset.ConstraintCommand, time.Time) (asset.ConstraintPreview, error)
}

type constraintContextStore interface {
	EffectiveConstraints(context.Context, int) ([]asset.EffectiveConstraint, error)
}

// Constraints 提供原文与按章生效的版本序列。
func (s *Service) Constraints(ctx context.Context) ([]asset.ConstraintView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	store, ok := s.active.store.(constraintStore)
	if !ok {
		return nil, asset.ErrChangeUnsupported
	}
	return store.Constraints(ctx)
}

// PreviewConstraintChange 冻结当前项目和目标章节；预览不调用模型。
func (s *Service) PreviewConstraintChange(ctx context.Context, r asset.ConstraintRequest) (asset.ConstraintPreview, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return asset.ConstraintPreview{}, ErrNoActiveProject
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return asset.ConstraintPreview{}, err
	}
	if r.ProjectID != "" && r.ProjectID != project.ID {
		return asset.ConstraintPreview{}, asset.ErrChangeConflict
	}
	r.ProjectID = project.ID
	branchID, err := s.constraintBranchLocked()
	if err != nil {
		return asset.ConstraintPreview{}, err
	}
	if r.BranchID != "" && r.BranchID != branchID {
		return asset.ConstraintPreview{}, asset.ErrChangeConflict
	}
	r.BranchID = branchID
	store, ok := s.active.store.(constraintStore)
	if !ok {
		return asset.ConstraintPreview{}, asset.ErrChangeUnsupported
	}
	return store.PreviewConstraintChange(ctx, r)
}

// ApplyConstraintChange 在项目生命周期锁内执行版本追加，不能切换到其他项目后误确认。
func (s *Service) ApplyConstraintChange(ctx context.Context, c asset.ConstraintCommand) (asset.ConstraintPreview, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return asset.ConstraintPreview{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return asset.ConstraintPreview{}, err
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return asset.ConstraintPreview{}, err
	}
	if c.Request.ProjectID != project.ID {
		return asset.ConstraintPreview{}, asset.ErrChangeConflict
	}
	branchID, err := s.constraintBranchLocked()
	if err != nil {
		return asset.ConstraintPreview{}, err
	}
	if c.Request.BranchID != branchID {
		return asset.ConstraintPreview{}, asset.ErrChangeConflict
	}
	store, ok := s.active.store.(constraintStore)
	if !ok {
		return asset.ConstraintPreview{}, asset.ErrChangeUnsupported
	}
	result, err := store.ApplyConstraintChange(ctx, c, s.clock.Now())
	if err != nil {
		return result, s.handleFilesystemWriteFailureLocked(err)
	}
	return result, nil
}

// constraintBranchLocked 将授权绑定到真实活动分支，避免在同作品的另一分支误确认。
func (s *Service) constraintBranchLocked() (string, error) {
	if s.branches == nil {
		return "main", nil
	}
	catalog, err := s.branches.Load(s.active.packageInfo.Branches)
	if err != nil {
		return "", fmt.Errorf("read constraint branch: %w", err)
	}
	return catalog.ActiveBranchID, nil
}

type characterRevisionReader interface {
	CharacterRevisions(context.Context) ([]asset.ConstraintView, error)
}

// CharacterRevisions 读取正式人物原档案和生效记录，无信封人物不伪造历史。
func (s *Service) CharacterRevisions(ctx context.Context) ([]asset.ConstraintView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	reader, ok := s.active.store.(characterRevisionReader)
	if !ok {
		return nil, asset.ErrChangeUnsupported
	}
	return reader.CharacterRevisions(ctx)
}
