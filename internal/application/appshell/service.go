// Package appshell coordinates the workspace lifecycle with application
// settings recording. It sits between the desktop API layer and the workspace
// and application settings services, ensuring that recent project entries are
// recorded after successful opens and that missing recent projects are marked
// so the UI can prompt for relocation.
package appshell

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/application/workspace"
	appdomain "github.com/lishen802/novel-studio/internal/domain/appsettings"
)

// ErrProjectRelocateRequired is returned when a recent project's recorded path
// no longer exists on disk. The shell has already marked the entry missing in
// the settings store; the caller should prompt the user to relocate or remove
// the recent entry rather than silently deleting it.
var ErrProjectRelocateRequired = errors.New("recent project path no longer exists; relocation required")

// ErrRecentProjectNotFound is returned when a recent project ID does not match
// any recorded recent project entry.
var ErrRecentProjectNotFound = errors.New("recent project not found")

// WorkspaceLifecycle is the subset of the workspace service that the shell
// coordinates. It is deliberately narrow so tests can inject a fake without
// implementing every workspace method.
type WorkspaceLifecycle interface {
	CreateProject(ctx context.Context, parentDir, title, genre, summary string) (workspace.ProjectView, error)
	OpenProject(ctx context.Context, path string) (workspace.ProjectView, error)
	OpenProjectReadOnly(ctx context.Context, path string) (workspace.ProjectView, error)
	Close() error
}

// Settings is the subset of the application settings service that the shell
// uses to record and inspect recent projects.
type Settings interface {
	RecentProjects(ctx context.Context, limit int) ([]appdomain.RecentProject, error)
	RecordRecentProject(ctx context.Context, rp appdomain.RecentProject) error
	MarkProjectMissing(ctx context.Context, projectID string, missing bool) error
}

// Service coordinates workspace lifecycle operations with recent project
// recording. It never closes a successfully opened project when the recent
// project recording fails; instead it returns a non-empty warning so the UI
// can surface it without losing the user's workspace.
type Service struct {
	workspace WorkspaceLifecycle
	settings  Settings
	clock     ports.Clock
}

// New creates a shell service that coordinates the given workspace lifecycle
// and settings.
func New(workspace WorkspaceLifecycle, settings Settings, clock ports.Clock) *Service {
	return &Service{workspace: workspace, settings: settings, clock: clock}
}

// OpenResult carries the opened project view and an optional warning emitted
// when recording the recent project failed. The project remains open either
// way so the user does not lose their workspace; the warning can be displayed
// in the UI as a non-blocking notice.
type OpenResult struct {
	Project workspace.ProjectView
	Warning string
}

// CreateProject delegates to the workspace service and records the new project
// in the recent projects list on success. A recording failure produces a
// warning but does not close the project.
func (s *Service) CreateProject(ctx context.Context, parentDir, title, genre, summary string) (OpenResult, error) {
	project, err := s.workspace.CreateProject(ctx, parentDir, title, genre, summary)
	if err != nil {
		return OpenResult{}, err
	}
	return OpenResult{Project: project, Warning: s.recordRecent(ctx, project)}, nil
}

// OpenProject delegates to the workspace service and records the project in
// the recent projects list on success. A recording failure produces a warning
// but does not close the project.
func (s *Service) OpenProject(ctx context.Context, path string) (OpenResult, error) {
	project, err := s.workspace.OpenProject(ctx, path)
	if err != nil {
		return OpenResult{}, err
	}
	return OpenResult{Project: project, Warning: s.recordRecent(ctx, project)}, nil
}

// OpenProjectReadOnly delegates to the workspace service and records the
// project in the recent projects list on success. A recording failure produces
// a warning but does not close the project.
func (s *Service) OpenProjectReadOnly(ctx context.Context, path string) (OpenResult, error) {
	project, err := s.workspace.OpenProjectReadOnly(ctx, path)
	if err != nil {
		return OpenResult{}, err
	}
	return OpenResult{Project: project, Warning: s.recordRecent(ctx, project)}, nil
}

// OpenRecentProject opens a project from the recent list by its project ID.
// If the recorded path no longer exists, the entry is marked missing and
// ErrProjectRelocateRequired is returned so the UI can prompt for relocation.
func (s *Service) OpenRecentProject(ctx context.Context, projectID string) (OpenResult, error) {
	recent, err := s.findRecentProject(ctx, projectID)
	if err != nil {
		return OpenResult{}, err
	}
	project, err := s.workspace.OpenProject(ctx, recent.Path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			_ = s.settings.MarkProjectMissing(ctx, projectID, true)
			return OpenResult{}, ErrProjectRelocateRequired
		}
		return OpenResult{}, err
	}
	return OpenResult{Project: project, Warning: s.recordRecent(ctx, project)}, nil
}

// Close delegates to the workspace service.
func (s *Service) Close() error {
	return s.workspace.Close()
}

// recordRecent writes a recent project entry derived from the project view.
// It returns a non-empty warning string when the recording fails; the project
// stays open either way so the user does not lose their workspace.
func (s *Service) recordRecent(ctx context.Context, project workspace.ProjectView) string {
	rp, err := appdomain.NewRecentProject(project.ID, project.Title, project.Path, s.clock.Now())
	if err != nil {
		return "无法记录最近项目。"
	}
	if err := s.settings.RecordRecentProject(ctx, rp); err != nil {
		return "无法记录最近项目。"
	}
	return ""
}

// findRecentProject looks up a recent project by its project ID. It requests a
// bounded number of entries so an unbounded settings store does not stall the
// shell.
func (s *Service) findRecentProject(ctx context.Context, projectID string) (appdomain.RecentProject, error) {
	recent, err := s.settings.RecentProjects(ctx, 100)
	if err != nil {
		return appdomain.RecentProject{}, fmt.Errorf("read recent projects: %w", err)
	}
	for _, rp := range recent {
		if rp.ProjectID == projectID {
			return rp, nil
		}
	}
	return appdomain.RecentProject{}, ErrRecentProjectNotFound
}

// now returns the current time from the configured clock. It is a convenience
// for tests that need to assert timestamps.
func (s *Service) now() time.Time {
	return s.clock.Now()
}
