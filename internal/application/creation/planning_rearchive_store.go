package creation

import (
	"context"
	"time"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// planningRearchiveBackend 由当前项目数据库实现，缺失时禁用整组入档。
type planningRearchiveBackend interface {
	PlanningArchiveAssets(context.Context, string) ([]asset.PlanningArchiveAsset, error)
	PlanningArchivedPeopleCandidate(context.Context, string) (string, error)
	PreviewConstraintChange(context.Context, asset.ConstraintRequest) (asset.ConstraintPreview, error)
	ApplyPlanningRearchive(context.Context, asset.PlanningRearchiveApply, time.Time) (asset.PlanningRearchivePreview, error)
	PlanningRearchiveReceipt(context.Context, asset.PlanningRearchiveRequest, string, string) (asset.PlanningRearchivePreview, bool, error)
}

func (store sqliteStore) planningBackend() (planningRearchiveBackend, error) {
	backend, ok := store.inner.(planningRearchiveBackend)
	if !ok {
		return nil, ErrUnavailable
	}
	return backend, nil
}

func (store sqliteStore) PlanningArchiveAssets(ctx context.Context, project string) ([]asset.PlanningArchiveAsset, error) {
	backend, err := store.planningBackend()
	if err != nil {
		return nil, err
	}
	return backend.PlanningArchiveAssets(ctx, project)
}
func (store sqliteStore) PlanningArchivedPeopleCandidate(ctx context.Context, project string) (string, error) {
	backend, err := store.planningBackend()
	if err != nil {
		return "", err
	}
	return backend.PlanningArchivedPeopleCandidate(ctx, project)
}
func (store sqliteStore) PreviewConstraintChange(ctx context.Context, r asset.ConstraintRequest) (asset.ConstraintPreview, error) {
	backend, err := store.planningBackend()
	if err != nil {
		return asset.ConstraintPreview{}, err
	}
	return backend.PreviewConstraintChange(ctx, r)
}
func (store sqliteStore) ApplyPlanningRearchive(ctx context.Context, input asset.PlanningRearchiveApply, now time.Time) (asset.PlanningRearchivePreview, error) {
	backend, err := store.planningBackend()
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	return backend.ApplyPlanningRearchive(ctx, input, now)
}
func (store sqliteStore) PlanningRearchiveReceipt(ctx context.Context, r asset.PlanningRearchiveRequest, digest, key string) (asset.PlanningRearchivePreview, bool, error) {
	backend, err := store.planningBackend()
	if err != nil {
		return asset.PlanningRearchivePreview{}, false, err
	}
	return backend.PlanningRearchiveReceipt(ctx, r, digest, key)
}
