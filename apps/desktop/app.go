package main

import (
	"context"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
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

// applicationVersion can be set through -ldflags at release time. Development
// and local test builds intentionally expose the stable non-empty fallback in
// lease diagnostics rather than attempting to inspect other processes.
var applicationVersion = "dev/unknown"

type App struct {
	api             *DesktopAPI
	emitBeforeClose func(context.Context)
	quit            func(context.Context)
	closeAllowed    atomic.Bool
	notifier        *wailsJobEventNotifier
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
	workspaceService.ConfigureProjectLease(projectfs.LeaseManager{}, processLeaseMetadata())
	notifier := newWailsJobEventNotifier()
	workspaceService.SetJobEventNotifier(notifier)
	app := &App{
		api: newDesktopAPI(workspaceService, wailsDirectoryPicker{}),
		emitBeforeClose: func(ctx context.Context) {
			runtime.EventsEmit(ctx, beforeCloseEvent)
		},
		quit:     runtime.Quit,
		notifier: notifier,
	}
	app.api.completeClose = app.completeClose
	return app
}

func processLeaseMetadata() ports.ProjectLeaseMetadata {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	version := applicationVersion
	if version == "" {
		version = "dev/unknown"
	}
	return ports.ProjectLeaseMetadata{
		PID:                os.Getpid(),
		Hostname:           hostname,
		ApplicationVersion: version,
		StartedAt:          time.Now().UTC(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.api.startup(ctx)
	if a.notifier != nil {
		a.notifier.startup(ctx)
	}
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
