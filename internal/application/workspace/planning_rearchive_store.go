package workspace

import (
	"context"
	"time"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type guardedPlanningRearchive interface {
	PlanningArchiveAssets(context.Context, string) ([]asset.PlanningArchiveAsset, error)
	PlanningArchivedPeopleCandidate(context.Context, string) (string, error)
	PreviewConstraintChange(context.Context, asset.ConstraintRequest) (asset.ConstraintPreview, error)
	ApplyPlanningRearchive(context.Context, asset.PlanningRearchiveApply, time.Time) (asset.PlanningRearchivePreview, error)
	PlanningRearchiveReceipt(context.Context, asset.PlanningRearchiveRequest, string, string) (asset.PlanningRearchivePreview, bool, error)
}

func (store creationStore) planningRearchive() (guardedPlanningRearchive, error) {
	reader, ok := store.inner.(guardedPlanningRearchive)
	if !ok {
		return nil, asset.ErrChangeUnsupported
	}
	return reader, nil
}

func (store creationStore) PlanningArchiveAssets(ctx context.Context, project string) (items []asset.PlanningArchiveAsset, err error) {
	reader, e := store.planningRearchive()
	if e != nil {
		return nil, e
	}
	err = store.guard.access(ctx, false, func() error { items, err = reader.PlanningArchiveAssets(ctx, project); return err })
	return
}
func (store creationStore) PlanningArchivedPeopleCandidate(ctx context.Context, project string) (candidate string, err error) {
	reader, e := store.planningRearchive()
	if e != nil {
		return "", e
	}
	err = store.guard.access(ctx, false, func() error { candidate, err = reader.PlanningArchivedPeopleCandidate(ctx, project); return err })
	return
}
func (store creationStore) PreviewConstraintChange(ctx context.Context, r asset.ConstraintRequest) (preview asset.ConstraintPreview, err error) {
	reader, e := store.planningRearchive()
	if e != nil {
		return preview, e
	}
	err = store.guard.access(ctx, false, func() error {
		branch, e := store.guard.workspace.constraintBranchLocked()
		if e != nil {
			return e
		}
		if branch != r.BranchID {
			return asset.ErrChangeConflict
		}
		preview, err = reader.PreviewConstraintChange(ctx, r)
		return err
	})
	return
}
func (store creationStore) ApplyPlanningRearchive(ctx context.Context, input asset.PlanningRearchiveApply, now time.Time) (preview asset.PlanningRearchivePreview, err error) {
	reader, e := store.planningRearchive()
	if e != nil {
		return preview, e
	}
	err = store.guard.access(ctx, true, func() error {
		branch, e := store.guard.workspace.constraintBranchLocked()
		if e != nil {
			return e
		}
		if branch != input.Preview.Request.BranchID {
			return asset.ErrChangeConflict
		}
		preview, err = reader.ApplyPlanningRearchive(ctx, input, now)
		return err
	})
	return
}
func (store creationStore) PlanningRearchiveReceipt(ctx context.Context, r asset.PlanningRearchiveRequest, digest, key string) (preview asset.PlanningRearchivePreview, found bool, err error) {
	reader, e := store.planningRearchive()
	if e != nil {
		return preview, false, e
	}
	err = store.guard.access(ctx, false, func() error {
		branch, e := store.guard.workspace.constraintBranchLocked()
		if e != nil {
			return e
		}
		if branch != r.BranchID {
			return asset.ErrChangeConflict
		}
		preview, found, err = reader.PlanningRearchiveReceipt(ctx, r, digest, key)
		return err
	})
	return
}
