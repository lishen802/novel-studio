package workspace

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

type ChapterRevisionView struct {
	ChapterID string `json:"chapterId"`
	Revision  int64  `json:"revision"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Source    string `json:"source"`
	Summary   string `json:"summary"`
	CreatedAt string `json:"createdAt"`
	IsCurrent bool   `json:"isCurrent"`
}

type SnapshotView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	CreatedAt string `json:"createdAt"`
}

type SnapshotRestoreView struct {
	SnapshotID string        `json:"snapshotId"`
	Project    ProjectView   `json:"project"`
	Chapters   []ChapterView `json:"chapters"`
}

func (s *Service) ChapterHistory(
	ctx context.Context,
	chapterID string,
) ([]ChapterRevisionView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	chapter, err := s.active.store.Chapter(ctx, strings.TrimSpace(chapterID))
	if err != nil {
		return nil, fmt.Errorf("read chapter for history: %w", err)
	}
	history, err := s.active.store.ListChapterRevisions(ctx, chapter.ID)
	if err != nil {
		return nil, fmt.Errorf("list chapter history: %w", err)
	}
	result := make([]ChapterRevisionView, 0, len(history))
	for _, item := range history {
		result = append(result, chapterRevisionView(item, item.Revision == chapter.Revision))
	}
	return result, nil
}

func (s *Service) RestoreChapterRevision(
	ctx context.Context,
	chapterID string,
	targetRevision int64,
	expectedRevision int64,
) (ChapterView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ChapterView{}, err
	}
	chapter, err := s.active.store.RestoreChapterRevision(
		ctx,
		strings.TrimSpace(chapterID),
		targetRevision,
		expectedRevision,
		s.clock.Now(),
	)
	if err != nil {
		return ChapterView{}, fmt.Errorf("restore chapter revision: %w", err)
	}
	return chapterView(chapter), nil
}

func (s *Service) Snapshots(ctx context.Context) ([]SnapshotView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return nil, fmt.Errorf("read project for snapshots: %w", err)
	}
	snapshots, err := s.snapshots.List(s.active.packageInfo.Snapshots)
	if err != nil {
		return nil, fmt.Errorf("list project snapshots: %w", err)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		if snapshots[i].CreatedAt.Equal(snapshots[j].CreatedAt) {
			return snapshots[i].ID > snapshots[j].ID
		}
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})
	views := make([]SnapshotView, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.ProjectID != project.ID {
			return nil, fmt.Errorf("snapshot belongs to a different project: %s", snapshot.ID)
		}
		views = append(views, snapshotView(snapshot))
	}
	return views, nil
}

func (s *Service) CreateSnapshot(ctx context.Context, name string) (SnapshotView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return SnapshotView{}, err
	}
	snapshot, err := s.createSnapshotLocked(ctx, name, revisiondomain.SnapshotNamed)
	if err != nil {
		return SnapshotView{}, err
	}
	return snapshotView(snapshot), nil
}

func (s *Service) RestoreSnapshot(
	ctx context.Context,
	id string,
) (SnapshotRestoreView, error) {
	if err := s.beginVersionOperation(); err != nil {
		return SnapshotRestoreView{}, err
	}
	defer s.endVersionOperation()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return SnapshotRestoreView{}, err
	}
	active := s.active
	currentProject, err := active.store.Project(ctx)
	if err != nil {
		return SnapshotRestoreView{}, fmt.Errorf("read project before snapshot restore: %w", err)
	}
	selected, snapshotPath, err := s.snapshots.Find(active.packageInfo.Snapshots, strings.TrimSpace(id))
	if err != nil {
		return SnapshotRestoreView{}, fmt.Errorf("find snapshot: %w", err)
	}
	if selected.ProjectID != currentProject.ID {
		return SnapshotRestoreView{}, errors.New("snapshot belongs to a different project")
	}
	recoveryName := fmt.Sprintf("恢复“%s”前", selected.Name)
	if _, err := s.createSnapshotLocked(ctx, recoveryName, revisiondomain.SnapshotRecovery); err != nil {
		return SnapshotRestoreView{}, fmt.Errorf("create pre-restore recovery point: %w", err)
	}
	if err := active.store.Close(); err != nil {
		return SnapshotRestoreView{}, fmt.Errorf("close project before snapshot restore: %w", err)
	}
	if err := s.snapshots.BeginRestore(active.packageInfo.Database, snapshotPath); err != nil {
		return SnapshotRestoreView{}, s.rollbackSnapshotRestoreLocked(ctx, active.packageInfo, s.handleFilesystemWriteFailureLocked(err))
	}
	restoredStore, err := s.stores.Open(ctx, active.packageInfo.Database)
	if err != nil {
		return SnapshotRestoreView{}, s.rollbackSnapshotRestoreLocked(ctx, active.packageInfo, err)
	}
	restoredProject, err := restoredStore.Project(ctx)
	if err != nil || restoredProject.ID != currentProject.ID {
		_ = restoredStore.Close()
		if err == nil {
			err = errors.New("restored database belongs to a different project")
		}
		return SnapshotRestoreView{}, s.rollbackSnapshotRestoreLocked(ctx, active.packageInfo, err)
	}
	chapters, err := restoredStore.ListChapters(ctx)
	if err != nil {
		_ = restoredStore.Close()
		return SnapshotRestoreView{}, s.rollbackSnapshotRestoreLocked(ctx, active.packageInfo, err)
	}
	if err := s.snapshots.CommitRestore(active.packageInfo.Database); err != nil {
		_ = restoredStore.Close()
		return SnapshotRestoreView{}, s.rollbackSnapshotRestoreLocked(ctx, active.packageInfo, s.handleFilesystemWriteFailureLocked(err))
	}
	s.active = &activeWorkspace{
		packageInfo:      active.packageInfo,
		store:            restoredStore,
		mode:             active.mode,
		projectLease:     active.projectLease,
		filesystemHealth: active.filesystemHealth,
		generation:       active.generation,
	}
	chapterViews := make([]ChapterView, 0, len(chapters))
	for _, chapter := range chapters {
		chapterViews = append(chapterViews, chapterView(chapter))
	}
	return SnapshotRestoreView{
		SnapshotID: selected.ID,
		Project:    projectViewWithHealth(restoredProject, active.packageInfo.Root, active.filesystemHealth),
		Chapters:   chapterViews,
	}, nil
}

func (s *Service) createSnapshotLocked(
	ctx context.Context,
	name string,
	kind revisiondomain.SnapshotKind,
) (revisiondomain.Snapshot, error) {
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return revisiondomain.Snapshot{}, fmt.Errorf("read project for snapshot: %w", err)
	}
	snapshot, err := revisiondomain.NewSnapshot(
		s.ids.NewID(),
		project.ID,
		name,
		kind,
		s.clock.Now(),
	)
	if err != nil {
		return revisiondomain.Snapshot{}, err
	}
	targetPath, err := s.snapshots.Prepare(s.active.packageInfo.Snapshots, snapshot.ID)
	if err != nil {
		return revisiondomain.Snapshot{}, fmt.Errorf("prepare project snapshot: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	if err := s.active.store.Backup(ctx, targetPath); err != nil {
		_ = s.snapshots.Discard(s.active.packageInfo.Snapshots, snapshot.ID)
		return revisiondomain.Snapshot{}, fmt.Errorf("backup project snapshot: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	if err := s.snapshots.Record(s.active.packageInfo.Snapshots, snapshot); err != nil {
		_ = s.snapshots.Discard(s.active.packageInfo.Snapshots, snapshot.ID)
		return revisiondomain.Snapshot{}, fmt.Errorf("record project snapshot: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	return snapshot, nil
}

func (s *Service) rollbackSnapshotRestoreLocked(
	ctx context.Context,
	projectPackage ports.ProjectPackage,
	cause error,
) error {
	active := s.active
	rollbackErr := s.snapshots.RollbackRestore(projectPackage.Database)
	reopened, reopenErr := s.stores.Open(ctx, projectPackage.Database)
	if reopenErr == nil {
		if active == nil {
			s.active = &activeWorkspace{packageInfo: projectPackage, store: reopened}
		} else {
			s.active = &activeWorkspace{
				packageInfo:      projectPackage,
				store:            reopened,
				mode:             active.mode,
				projectLease:     active.projectLease,
				filesystemHealth: active.filesystemHealth,
				generation:       active.generation,
			}
		}
	} else {
		s.active = nil
	}
	if rollbackErr != nil || reopenErr != nil {
		var releaseErr error
		if reopenErr != nil && active != nil {
			releaseErr = releaseProjectLease(active.projectLease)
		}
		var rollbackContext, reopenContext, releaseContext error
		if rollbackErr != nil {
			rollbackContext = fmt.Errorf("rollback snapshot restore: %w", rollbackErr)
		}
		if reopenErr != nil {
			reopenContext = fmt.Errorf("reopen original project: %w", reopenErr)
		}
		if releaseErr != nil {
			releaseContext = fmt.Errorf("release project lease: %w", releaseErr)
		}
		return errors.Join(
			fmt.Errorf("snapshot restore failed: %w", cause),
			rollbackContext,
			reopenContext,
			releaseContext,
		)
	}
	return fmt.Errorf("snapshot restore failed and original project was reopened: %w", cause)
}

func chapterRevisionView(
	item revisiondomain.ChapterRevision,
	current bool,
) ChapterRevisionView {
	return ChapterRevisionView{
		ChapterID: item.ChapterID,
		Revision:  item.Revision,
		Title:     item.Title,
		Body:      item.Body,
		Source:    string(item.Source),
		Summary:   item.Summary,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339Nano),
		IsCurrent: current,
	}
}

func snapshotView(snapshot revisiondomain.Snapshot) SnapshotView {
	return SnapshotView{
		ID:        snapshot.ID,
		Name:      snapshot.Name,
		Kind:      string(snapshot.Kind),
		CreatedAt: snapshot.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}
