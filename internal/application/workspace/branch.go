package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

var ErrBranchFeatureUnavailable = errors.New("project branch feature is unavailable")

type BranchView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ParentID    string `json:"parentId"`
	Active      bool   `json:"active"`
	Main        bool   `json:"main"`
	Generation  int64  `json:"generation"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type BranchSwitchView struct {
	ActiveBranchID string        `json:"activeBranchId"`
	Project        ProjectView   `json:"project"`
	Chapters       []ChapterView `json:"chapters"`
}

type BranchChapterChangeView struct {
	ChapterID string `json:"chapterId"`
	Title     string `json:"title"`
	Kind      string `json:"kind"`
}

type BranchDomainChangeView struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	SourceCount int    `json:"sourceCount"`
	TargetCount int    `json:"targetCount"`
	Changed     bool   `json:"changed"`
}

type BranchComparisonView struct {
	Source           BranchView                `json:"source"`
	Target           BranchView                `json:"target"`
	AddedChapters    int                       `json:"addedChapters"`
	ModifiedChapters int                       `json:"modifiedChapters"`
	RemovedChapters  int                       `json:"removedChapters"`
	ChapterChanges   []BranchChapterChangeView `json:"chapterChanges"`
	Domains          []BranchDomainChangeView  `json:"domains"`
}

type BranchMergePreviewView struct {
	Token      string               `json:"token"`
	SourceID   string               `json:"sourceId"`
	TargetID   string               `json:"targetId"`
	Comparison BranchComparisonView `json:"comparison"`
}

type branchMergePreview struct {
	plan revisiondomain.BranchMergePlan
}

type branchDomainState struct {
	key         string
	label       string
	count       int
	fingerprint string
}

type immutableReadOnlyStoreFactory interface {
	OpenReadOnlyImmutable(context.Context, string) (ports.ProjectStore, error)
}

func (s *Service) Branches(ctx context.Context) ([]BranchView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	if s.branches == nil {
		return nil, ErrBranchFeatureUnavailable
	}
	var catalog revisiondomain.BranchCatalog
	var err error
	if s.active.mode == ports.ProjectAccessReadOnly {
		catalog, err = s.branches.Load(s.active.packageInfo.Branches)
	} else {
		catalog, err = s.checkpointActiveBranchLocked(ctx)
	}
	if err != nil {
		return nil, err
	}
	return branchViews(catalog), nil
}

func (s *Service) CreateBranch(
	ctx context.Context,
	name, description string,
) (BranchView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return BranchView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return BranchView{}, err
	}
	if s.branches == nil {
		return BranchView{}, ErrBranchFeatureUnavailable
	}
	catalog, err := s.branches.Load(s.active.packageInfo.Branches)
	if err != nil {
		return BranchView{}, fmt.Errorf("load project branches: %w", err)
	}
	preparedID, preparedPath, err := s.prepareActiveBranchBackupLocked(ctx)
	if err != nil {
		return BranchView{}, err
	}
	defer s.branches.DiscardPrepared(s.active.packageInfo.Branches, preparedID)
	created, err := s.branches.Fork(
		s.active.packageInfo.Branches,
		s.ids.NewID(),
		name,
		description,
		catalog.ActiveBranchID,
		preparedPath,
		s.clock.Now(),
	)
	if err != nil {
		return BranchView{}, fmt.Errorf("create project branch: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	return branchView(created, false), nil
}

func (s *Service) SwitchBranch(
	ctx context.Context,
	id string,
) (BranchSwitchView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return BranchSwitchView{}, err
	}
	if err := s.beginVersionOperation(); err != nil {
		return BranchSwitchView{}, err
	}
	defer s.endVersionOperation()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return BranchSwitchView{}, err
	}
	if s.branches == nil {
		return BranchSwitchView{}, ErrBranchFeatureUnavailable
	}
	id = strings.TrimSpace(id)
	catalog, err := s.branches.Load(s.active.packageInfo.Branches)
	if err != nil {
		return BranchSwitchView{}, fmt.Errorf("load project branches: %w", err)
	}
	if id == catalog.ActiveBranchID {
		return s.branchWorkspaceViewLocked(ctx, catalog.ActiveBranchID)
	}
	preparedID, preparedPath, err := s.prepareActiveBranchBackupLocked(ctx)
	if err != nil {
		return BranchSwitchView{}, err
	}
	defer s.branches.DiscardPrepared(s.active.packageInfo.Branches, preparedID)
	active := s.active
	project, err := active.store.Project(ctx)
	if err != nil {
		return BranchSwitchView{}, fmt.Errorf("read project before branch switch: %w", err)
	}
	if err := active.store.Close(); err != nil {
		return BranchSwitchView{}, fmt.Errorf("close project before branch switch: %w", err)
	}
	nextCatalog, err := s.branches.Switch(
		active.packageInfo.Branches,
		active.packageInfo.Database,
		id,
		preparedPath,
		s.clock.Now(),
	)
	if err != nil {
		return BranchSwitchView{}, s.reopenAfterBranchFailureLocked(ctx, active.packageInfo, s.handleFilesystemWriteFailureLocked(err))
	}
	store, err := s.stores.Open(ctx, active.packageInfo.Database)
	if err != nil {
		return BranchSwitchView{}, s.rollbackBranchSwitchLocked(
			ctx,
			active.packageInfo,
			catalog.ActiveBranchID,
			id,
			err,
		)
	}
	nextProject, err := store.Project(ctx)
	if err != nil || nextProject.ID != project.ID {
		_ = store.Close()
		if err == nil {
			err = errors.New("branch database belongs to another project")
		}
		return BranchSwitchView{}, s.rollbackBranchSwitchLocked(
			ctx,
			active.packageInfo,
			catalog.ActiveBranchID,
			id,
			err,
		)
	}
	s.active = &activeWorkspace{
		packageInfo:      active.packageInfo,
		store:            store,
		mode:             active.mode,
		projectLease:     active.projectLease,
		filesystemHealth: active.filesystemHealth,
		generation:       active.generation,
	}
	s.branchMergePreviews = make(map[string]branchMergePreview)
	return s.branchWorkspaceViewLocked(ctx, nextCatalog.ActiveBranchID)
}

func (s *Service) CompareBranches(
	ctx context.Context,
	sourceID, targetID string,
) (BranchComparisonView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return BranchComparisonView{}, ErrNoActiveProject
	}
	if s.branches == nil {
		return BranchComparisonView{}, ErrBranchFeatureUnavailable
	}
	if s.active.mode != ports.ProjectAccessReadOnly {
		if _, err := s.checkpointActiveBranchLocked(ctx); err != nil {
			return BranchComparisonView{}, err
		}
	}
	return s.compareBranchesLocked(ctx, strings.TrimSpace(sourceID), strings.TrimSpace(targetID))
}

func (s *Service) PreviewBranchMerge(
	ctx context.Context,
	sourceID string,
) (BranchMergePreviewView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return BranchMergePreviewView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return BranchMergePreviewView{}, err
	}
	if s.branches == nil {
		return BranchMergePreviewView{}, ErrBranchFeatureUnavailable
	}
	catalog, err := s.checkpointActiveBranchLocked(ctx)
	if err != nil {
		return BranchMergePreviewView{}, err
	}
	plan, err := s.branches.PlanMerge(
		s.active.packageInfo.Branches,
		strings.TrimSpace(sourceID),
		catalog.ActiveBranchID,
	)
	if err != nil {
		return BranchMergePreviewView{}, fmt.Errorf("prepare branch merge: %w", err)
	}
	comparison, err := s.compareBranchesLocked(ctx, plan.SourceID, plan.TargetID)
	if err != nil {
		return BranchMergePreviewView{}, err
	}
	token := s.ids.NewID()
	s.branchMergePreviews[token] = branchMergePreview{plan: plan}
	return BranchMergePreviewView{
		Token:      token,
		SourceID:   plan.SourceID,
		TargetID:   plan.TargetID,
		Comparison: comparison,
	}, nil
}

func (s *Service) ConfirmBranchMerge(
	ctx context.Context,
	token string,
) (BranchSwitchView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return BranchSwitchView{}, err
	}
	if err := s.beginVersionOperation(); err != nil {
		return BranchSwitchView{}, err
	}
	defer s.endVersionOperation()
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return BranchSwitchView{}, err
	}
	if s.branches == nil {
		return BranchSwitchView{}, ErrBranchFeatureUnavailable
	}
	preview, found := s.branchMergePreviews[strings.TrimSpace(token)]
	if !found {
		return BranchSwitchView{}, revisiondomain.ErrStaleBranchMerge
	}
	delete(s.branchMergePreviews, strings.TrimSpace(token))
	preparedID, preparedPath, err := s.prepareActiveBranchBackupLocked(ctx)
	if err != nil {
		return BranchSwitchView{}, err
	}
	defer s.branches.DiscardPrepared(s.active.packageInfo.Branches, preparedID)
	if _, err := s.branches.Checkpoint(
		s.active.packageInfo.Branches,
		preview.plan.TargetID,
		preparedPath,
		s.clock.Now(),
	); err != nil {
		return BranchSwitchView{}, fmt.Errorf("refresh merge target: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	currentPlan, err := s.branches.PlanMerge(
		s.active.packageInfo.Branches,
		preview.plan.SourceID,
		preview.plan.TargetID,
	)
	if err != nil {
		return BranchSwitchView{}, fmt.Errorf("revalidate branch merge: %w", err)
	}
	if currentPlan != preview.plan {
		return BranchSwitchView{}, revisiondomain.ErrStaleBranchMerge
	}
	if _, err := s.createSnapshotLocked(
		ctx,
		"合并剧情分支前",
		revisiondomain.SnapshotRecovery,
	); err != nil {
		return BranchSwitchView{}, fmt.Errorf("create pre-merge recovery point: %w", err)
	}
	active := s.active
	if err := active.store.Close(); err != nil {
		return BranchSwitchView{}, fmt.Errorf("close project before branch merge: %w", err)
	}
	catalog, err := s.branches.Merge(
		active.packageInfo.Branches,
		active.packageInfo.Database,
		preview.plan,
		preparedPath,
		s.clock.Now(),
	)
	if err != nil {
		return BranchSwitchView{}, s.reopenAfterBranchFailureLocked(ctx, active.packageInfo, s.handleFilesystemWriteFailureLocked(err))
	}
	store, err := s.stores.Open(ctx, active.packageInfo.Database)
	if err != nil {
		return BranchSwitchView{}, s.reopenAfterBranchFailureLocked(ctx, active.packageInfo, err)
	}
	s.active = &activeWorkspace{
		packageInfo:      active.packageInfo,
		store:            store,
		mode:             active.mode,
		projectLease:     active.projectLease,
		filesystemHealth: active.filesystemHealth,
		generation:       active.generation,
	}
	return s.branchWorkspaceViewLocked(ctx, catalog.ActiveBranchID)
}

func (s *Service) DeleteBranch(ctx context.Context, id string) error {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return err
	}
	if s.branches == nil {
		return ErrBranchFeatureUnavailable
	}
	if err := s.branches.Delete(s.active.packageInfo.Branches, strings.TrimSpace(id)); err != nil {
		return fmt.Errorf("delete project branch: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	return nil
}

func (s *Service) ensureBranchCatalog(ctx context.Context, active *activeWorkspace) error {
	if s.branches == nil {
		return nil
	}
	project, err := active.store.Project(ctx)
	if err != nil {
		return fmt.Errorf("read project for branch archive: %w", err)
	}
	preparedID := s.ids.NewID()
	preparedPath, err := s.branches.Prepare(active.packageInfo.Branches, preparedID)
	if err != nil {
		return fmt.Errorf("prepare initial branch backup: %w", s.handleFilesystemWriteFailureForPackage(active.packageInfo.Root, err))
	}
	defer s.branches.DiscardPrepared(active.packageInfo.Branches, preparedID)
	if err := active.store.Backup(ctx, preparedPath); err != nil {
		return fmt.Errorf("backup initial branch state: %w", s.handleFilesystemWriteFailureForPackage(active.packageInfo.Root, err))
	}
	if _, err := s.branches.Initialize(
		active.packageInfo.Branches,
		project.ID,
		s.ids.NewID(),
		preparedPath,
		s.clock.Now(),
	); err != nil {
		return fmt.Errorf("initialize project branches: %w", s.handleFilesystemWriteFailureForPackage(active.packageInfo.Root, err))
	}
	return nil
}

func (s *Service) checkpointActiveBranchLocked(
	ctx context.Context,
) (revisiondomain.BranchCatalog, error) {
	catalog, err := s.branches.Load(s.active.packageInfo.Branches)
	if err != nil {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("load project branches: %w", err)
	}
	preparedID, preparedPath, err := s.prepareActiveBranchBackupLocked(ctx)
	if err != nil {
		return revisiondomain.BranchCatalog{}, err
	}
	defer s.branches.DiscardPrepared(s.active.packageInfo.Branches, preparedID)
	if _, err := s.branches.Checkpoint(
		s.active.packageInfo.Branches,
		catalog.ActiveBranchID,
		preparedPath,
		s.clock.Now(),
	); err != nil {
		return revisiondomain.BranchCatalog{}, fmt.Errorf("checkpoint active branch: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	return s.branches.Load(s.active.packageInfo.Branches)
}

func (s *Service) prepareActiveBranchBackupLocked(
	ctx context.Context,
) (string, string, error) {
	id := s.ids.NewID()
	path, err := s.branches.Prepare(s.active.packageInfo.Branches, id)
	if err != nil {
		return "", "", fmt.Errorf("prepare branch backup: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	if err := s.active.store.Backup(ctx, path); err != nil {
		_ = s.branches.DiscardPrepared(s.active.packageInfo.Branches, id)
		return "", "", fmt.Errorf("backup active branch: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	return id, path, nil
}

func (s *Service) branchWorkspaceViewLocked(
	ctx context.Context,
	activeBranchID string,
) (BranchSwitchView, error) {
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return BranchSwitchView{}, fmt.Errorf("read branch project: %w", err)
	}
	chapters, err := s.active.store.ListChapters(ctx)
	if err != nil {
		return BranchSwitchView{}, fmt.Errorf("list branch chapters: %w", err)
	}
	views := make([]ChapterView, 0, len(chapters))
	for _, chapter := range chapters {
		views = append(views, chapterView(chapter))
	}
	return BranchSwitchView{
		ActiveBranchID: activeBranchID,
		Project:        projectViewWithHealth(project, s.active.packageInfo.Root, s.active.filesystemHealth),
		Chapters:       views,
	}, nil
}

func (s *Service) reopenAfterBranchFailureLocked(
	ctx context.Context,
	projectPackage ports.ProjectPackage,
	cause error,
) error {
	active := s.active
	store, reopenErr := s.stores.Open(ctx, projectPackage.Database)
	if reopenErr == nil {
		if active == nil {
			s.active = &activeWorkspace{packageInfo: projectPackage, store: store}
		} else {
			s.active = &activeWorkspace{
				packageInfo:      projectPackage,
				store:            store,
				mode:             active.mode,
				projectLease:     active.projectLease,
				filesystemHealth: active.filesystemHealth,
				generation:       active.generation,
			}
		}
	} else {
		s.active = nil
	}
	if reopenErr != nil {
		var releaseErr error
		if active != nil {
			releaseErr = releaseProjectLease(active.projectLease)
		}
		var releaseContext error
		if releaseErr != nil {
			releaseContext = fmt.Errorf("release project lease: %w", releaseErr)
		}
		return errors.Join(
			fmt.Errorf("branch operation failed: %w", cause),
			fmt.Errorf("reopen project: %w", reopenErr),
			releaseContext,
		)
	}
	return fmt.Errorf("branch operation failed and project was reopened: %w", cause)
}

func (s *Service) rollbackBranchSwitchLocked(
	ctx context.Context,
	projectPackage ports.ProjectPackage,
	previousID, currentID string,
	cause error,
) error {
	active := s.active
	_, switchErr := s.branches.Switch(
		projectPackage.Branches,
		projectPackage.Database,
		previousID,
		s.branches.HeadPath(projectPackage.Branches, currentID),
		s.clock.Now(),
	)
	if switchErr != nil {
		s.active = nil
		var releaseContext error
		if active != nil {
			if releaseErr := releaseProjectLease(active.projectLease); releaseErr != nil {
				releaseContext = fmt.Errorf("release project lease: %w", releaseErr)
			}
		}
		return errors.Join(
			fmt.Errorf("branch switch verification failed: %w", cause),
			fmt.Errorf("rollback branch switch: %w", switchErr),
			releaseContext,
		)
	}
	return s.reopenAfterBranchFailureLocked(ctx, projectPackage, cause)
}

func (s *Service) compareBranchesLocked(
	ctx context.Context,
	sourceID, targetID string,
) (BranchComparisonView, error) {
	catalog, err := s.branches.Load(s.active.packageInfo.Branches)
	if err != nil {
		return BranchComparisonView{}, err
	}
	source, found := catalog.Find(sourceID)
	if !found {
		return BranchComparisonView{}, fmt.Errorf("source branch does not exist: %s", sourceID)
	}
	target, found := catalog.Find(targetID)
	if !found {
		return BranchComparisonView{}, fmt.Errorf("target branch does not exist: %s", targetID)
	}
	openStore := s.stores.Open
	if s.active.mode == ports.ProjectAccessReadOnly {
		openStore = s.stores.OpenReadOnly
		if immutableFactory, ok := s.stores.(immutableReadOnlyStoreFactory); ok {
			openStore = immutableFactory.OpenReadOnlyImmutable
		}
	}
	sourceStore, err := openStore(ctx, s.branches.HeadPath(s.active.packageInfo.Branches, sourceID))
	if err != nil {
		return BranchComparisonView{}, fmt.Errorf("open source branch: %w", err)
	}
	defer sourceStore.Close()
	targetStore, err := openStore(ctx, s.branches.HeadPath(s.active.packageInfo.Branches, targetID))
	if err != nil {
		return BranchComparisonView{}, fmt.Errorf("open target branch: %w", err)
	}
	defer targetStore.Close()

	sourceChapters, err := sourceStore.ListChapters(ctx)
	if err != nil {
		return BranchComparisonView{}, err
	}
	targetChapters, err := targetStore.ListChapters(ctx)
	if err != nil {
		return BranchComparisonView{}, err
	}
	comparison := compareChapters(sourceChapters, targetChapters)
	comparison.Source = branchView(source, catalog.ActiveBranchID == source.ID)
	comparison.Target = branchView(target, catalog.ActiveBranchID == target.ID)
	sourceDomains, err := collectBranchDomains(ctx, sourceStore, sourceChapters)
	if err != nil {
		return BranchComparisonView{}, fmt.Errorf("summarize source branch: %w", err)
	}
	targetDomains, err := collectBranchDomains(ctx, targetStore, targetChapters)
	if err != nil {
		return BranchComparisonView{}, fmt.Errorf("summarize target branch: %w", err)
	}
	for index := range sourceDomains {
		comparison.Domains = append(comparison.Domains, BranchDomainChangeView{
			Key:         sourceDomains[index].key,
			Label:       sourceDomains[index].label,
			SourceCount: sourceDomains[index].count,
			TargetCount: targetDomains[index].count,
			Changed:     sourceDomains[index].fingerprint != targetDomains[index].fingerprint,
		})
	}
	return comparison, nil
}

func compareChapters(
	source, target []projectdomain.Chapter,
) BranchComparisonView {
	sourceByID := make(map[string]projectdomain.Chapter, len(source))
	targetByID := make(map[string]projectdomain.Chapter, len(target))
	for _, chapter := range source {
		sourceByID[chapter.ID] = chapter
	}
	for _, chapter := range target {
		targetByID[chapter.ID] = chapter
	}
	result := BranchComparisonView{ChapterChanges: []BranchChapterChangeView{}}
	for _, chapter := range source {
		targetChapter, found := targetByID[chapter.ID]
		if !found {
			result.AddedChapters++
			result.ChapterChanges = append(result.ChapterChanges, BranchChapterChangeView{
				ChapterID: chapter.ID, Title: chapter.Title, Kind: "added",
			})
			continue
		}
		if chapter.Title != targetChapter.Title || chapter.Position != targetChapter.Position ||
			chapter.Body != targetChapter.Body || chapter.Revision != targetChapter.Revision {
			result.ModifiedChapters++
			result.ChapterChanges = append(result.ChapterChanges, BranchChapterChangeView{
				ChapterID: chapter.ID, Title: chapter.Title, Kind: "modified",
			})
		}
	}
	for _, chapter := range target {
		if _, found := sourceByID[chapter.ID]; !found {
			result.RemovedChapters++
			result.ChapterChanges = append(result.ChapterChanges, BranchChapterChangeView{
				ChapterID: chapter.ID, Title: chapter.Title, Kind: "removed",
			})
		}
	}
	sort.Slice(result.ChapterChanges, func(i, j int) bool {
		return result.ChapterChanges[i].Title < result.ChapterChanges[j].Title
	})
	return result
}

func collectBranchDomains(
	ctx context.Context,
	store ports.ProjectStore,
	chapters []projectdomain.Chapter,
) ([]branchDomainState, error) {
	outlineNodes, err := store.ListOutlineNodes(ctx)
	if err != nil {
		return nil, err
	}
	storyLines, err := store.ListStoryLines(ctx)
	if err != nil {
		return nil, err
	}
	storyLinks, err := store.ListStoryLineLinks(ctx)
	if err != nil {
		return nil, err
	}
	sceneCards := make([]any, 0)
	for _, chapter := range chapters {
		values, err := store.ListSceneCards(ctx, chapter.ID)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			sceneCards = append(sceneCards, value)
		}
	}
	characters, err := store.ListCharacters(ctx)
	if err != nil {
		return nil, err
	}
	factions, err := store.ListFactions(ctx)
	if err != nil {
		return nil, err
	}
	memberships, err := store.ListMemberships(ctx)
	if err != nil {
		return nil, err
	}
	relationships, err := store.ListRelationships(ctx)
	if err != nil {
		return nil, err
	}
	chapterAppearances, err := store.ListChapterAppearances(ctx)
	if err != nil {
		return nil, err
	}
	sceneAppearances, err := store.ListSceneAppearances(ctx)
	if err != nil {
		return nil, err
	}
	regions, err := store.ListWorldRegions(ctx)
	if err != nil {
		return nil, err
	}
	adjacencies, err := store.ListRegionAdjacencies(ctx)
	if err != nil {
		return nil, err
	}
	locations, err := store.ListLocations(ctx)
	if err != nil {
		return nil, err
	}
	locationLinks, err := store.ListLocationLinks(ctx)
	if err != nil {
		return nil, err
	}
	items, err := store.ListItems(ctx)
	if err != nil {
		return nil, err
	}
	itemTransfers := make([]any, 0)
	for _, item := range items {
		values, err := store.ListItemTransfers(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			itemTransfers = append(itemTransfers, value)
		}
	}
	rules, err := store.ListWorldRules(ctx)
	if err != nil {
		return nil, err
	}
	hooks, err := store.ListForeshadowingHooks(ctx)
	if err != nil {
		return nil, err
	}
	events, err := store.ListStoryEvents(ctx)
	if err != nil {
		return nil, err
	}
	eventLinks, err := store.ListStoryEventLinks(ctx)
	if err != nil {
		return nil, err
	}
	facts, err := store.ListFacts(ctx)
	if err != nil {
		return nil, err
	}
	knowledge, err := store.ListCharacterKnowledge(ctx)
	if err != nil {
		return nil, err
	}
	evidence, err := store.ListFactEvidence(ctx)
	if err != nil {
		return nil, err
	}
	scopes, err := store.ListFactScopes(ctx)
	if err != nil {
		return nil, err
	}
	conflicts, err := store.ListFactConflicts(ctx)
	if err != nil {
		return nil, err
	}
	changeSets, err := store.ListChangeSets(ctx)
	if err != nil {
		return nil, err
	}
	return []branchDomainState{
		newBranchDomainState(
			"story", "故事架构",
			len(outlineNodes)+len(storyLines)+len(storyLinks)+len(sceneCards),
			outlineNodes, storyLines, storyLinks, sceneCards,
		),
		newBranchDomainState(
			"characters", "人物与势力",
			len(characters)+len(factions)+len(memberships)+len(relationships)+
				len(chapterAppearances)+len(sceneAppearances),
			characters, factions, memberships, relationships, chapterAppearances, sceneAppearances,
		),
		newBranchDomainState(
			"world", "世界设定",
			len(regions)+len(adjacencies)+len(locations)+len(locationLinks)+
				len(items)+len(itemTransfers)+len(rules)+len(hooks),
			regions, adjacencies, locations, locationLinks, items, itemTransfers, rules, hooks,
		),
		newBranchDomainState(
			"continuity", "时间线与事实",
			len(events)+len(eventLinks)+len(facts)+len(knowledge)+len(evidence)+len(scopes)+len(conflicts),
			events, eventLinks, facts, knowledge, evidence, scopes, conflicts,
		),
		newBranchDomainState(
			"changes", "候选变更",
			len(changeSets),
			changeSets,
		),
	}, nil
}

func newBranchDomainState(key, label string, count int, values ...any) branchDomainState {
	body, _ := json.Marshal(values)
	sum := sha256.Sum256(body)
	return branchDomainState{
		key:         key,
		label:       label,
		count:       count,
		fingerprint: hex.EncodeToString(sum[:]),
	}
}

func branchViews(catalog revisiondomain.BranchCatalog) []BranchView {
	result := make([]BranchView, 0, len(catalog.Branches))
	for _, branch := range catalog.Branches {
		result = append(result, branchView(branch, catalog.ActiveBranchID == branch.ID))
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Main != result[j].Main {
			return result[i].Main
		}
		return result[i].CreatedAt < result[j].CreatedAt
	})
	return result
}

func branchView(branch revisiondomain.Branch, active bool) BranchView {
	return BranchView{
		ID:          branch.ID,
		Name:        branch.Name,
		Description: branch.Description,
		ParentID:    branch.ParentID,
		Active:      active,
		Main:        branch.ParentID == "",
		Generation:  branch.Generation,
		CreatedAt:   branch.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:   branch.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
