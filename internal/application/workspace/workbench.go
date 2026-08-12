package workspace

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

// WorkbenchProjection is the authoritative stage/canvas/AI-panel projection for
// the active project. React reads it from the workspace service and must never
// compute stage locally.
type WorkbenchProjection = projectdomain.WorkbenchProjection

// workbenchStore is the optional store interface for persisting the project
// profile pin and workbench state. The SQLite store implements it; test fakes
// are not required to.
type workbenchStore interface {
	SaveProfileSelection(ctx context.Context, selection projectdomain.ProfileSelection, updatedAt time.Time) error
	ProfileSelection(ctx context.Context) (projectdomain.ProfileSelection, error)
	SaveWorkbenchState(ctx context.Context, state projectdomain.WorkbenchState) error
	WorkbenchState(ctx context.Context) (projectdomain.WorkbenchState, error)
}

// defaultWorkbenchProjection returns the initial projection for a project with
// no persisted state: idea stage, blank canvas, hidden AI panel, and an
// ungenerated archive.
func defaultWorkbenchProjection() WorkbenchProjection {
	return WorkbenchProjection{
		Stage:         projectdomain.StageIdea,
		Canvas:        "",
		AIPanel:       "hidden",
		ArchiveStatus: "not_generated",
	}
}

func workbenchProjectionFromState(state projectdomain.WorkbenchState) WorkbenchProjection {
	return WorkbenchProjection{
		Stage:               state.Stage,
		Canvas:              state.Canvas,
		AIPanel:             state.AIPanel,
		PendingReviewCount:  state.PendingReviewCount,
		RecoverableJobCount: state.RecoverableJobCount,
		ArchiveStatus:       state.ArchiveStatus,
	}
}

// readProfileSelection reads the pinned profile selection from the store. When
// the store does not support profile persistence or no pin exists, it returns
// the zero value and found=false.
func readProfileSelection(ctx context.Context, store ports.ProjectStore) (ports.ProjectProfileSelection, bool, error) {
	ws, ok := store.(workbenchStore)
	if !ok {
		return ports.ProjectProfileSelection{}, false, nil
	}
	selection, err := ws.ProfileSelection(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ports.ProjectProfileSelection{}, false, nil
		}
		return ports.ProjectProfileSelection{}, false, fmt.Errorf("read project profile selection: %w", err)
	}
	return ports.ProjectProfileSelection{ProfileID: selection.ProfileID, Version: selection.Version}, true, nil
}

// computeWorkbenchProjection reads the persisted workbench state; if none exists
// it derives the projection from the chapter list. It never writes state.
func computeWorkbenchProjection(ctx context.Context, store ports.ProjectStore) (WorkbenchProjection, error) {
	projection := defaultWorkbenchProjection()

	if ws, ok := store.(workbenchStore); ok {
		state, err := ws.WorkbenchState(ctx)
		if err == nil {
			return workbenchProjectionFromState(state), nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return WorkbenchProjection{}, fmt.Errorf("read workbench state: %w", err)
		}
	}

	chapters, err := store.ListChapters(ctx)
	if err != nil {
		return WorkbenchProjection{}, fmt.Errorf("list chapters for workbench: %w", err)
	}
	if len(chapters) > 0 {
		projection.Stage = projectdomain.StageWriting
		projection.Canvas = "manuscript"
	}
	return projection, nil
}
