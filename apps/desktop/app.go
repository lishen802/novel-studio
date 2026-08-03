package main

import (
	"context"
	"net/http"
	"sync/atomic"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/workspace"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/lishen802/novel-studio/internal/platform/credentials"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
	"github.com/lishen802/novel-studio/internal/platform/timeutil"
	"github.com/lishen802/novel-studio/internal/providers"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const beforeCloseEvent = "app:before-close"

type App struct {
	api             *DesktopAPI
	emitBeforeClose func(context.Context)
	quit            func(context.Context)
	closeAllowed    atomic.Bool
}

func newApp() *App {
	workspaceService := workspace.NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		timeutil.SystemClock{},
		ids.UUIDGenerator{},
		snapshotfs.Archive{},
		branchfs.Archive{},
	)
	workspaceService.ConfigureAgentRuntime(
		credentials.NewStore(),
		providers.NewFactory(http.DefaultClient),
	)
	workspaceService.ConfigureImportSourceArchive(projectfs.ImportSourceArchive{})
	app := &App{
		api: newDesktopAPI(workspaceService, wailsDirectoryPicker{}),
		emitBeforeClose: func(ctx context.Context) {
			runtime.EventsEmit(ctx, beforeCloseEvent)
		},
		quit: runtime.Quit,
	}
	app.api.completeClose = app.completeClose
	return app
}

func (a *App) startup(ctx context.Context) {
	a.api.startup(ctx)
}

func (a *App) shutdown(context.Context) {
	_ = a.api.workspace.Close()
}

func (a *App) beforeClose(ctx context.Context) bool {
	if a.closeAllowed.Load() {
		return false
	}
	if a.emitBeforeClose != nil {
		a.emitBeforeClose(ctx)
	}
	return true
}

func (a *App) completeClose() {
	if !a.closeAllowed.CompareAndSwap(false, true) {
		return
	}
	if a.quit != nil {
		a.quit(a.api.ctx)
	}
}
