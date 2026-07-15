package main

import (
	"context"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/workspace"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/timeutil"
)

type App struct {
	api *DesktopAPI
}

func newApp() *App {
	workspaceService := workspace.NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		timeutil.SystemClock{},
		ids.UUIDGenerator{},
	)
	return &App{api: newDesktopAPI(workspaceService, wailsDirectoryPicker{})}
}

func (a *App) startup(ctx context.Context) {
	a.api.startup(ctx)
}

func (a *App) shutdown(context.Context) {
	_ = a.api.workspace.Close()
}
