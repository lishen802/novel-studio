package workspace

import (
	"context"
	"errors"
	"path"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

const (
	ImportErrorParseEmpty       = "IMPORT_PARSE_EMPTY"
	ImportErrorLineTooLong      = "IMPORT_PARSE_LINE_TOO_LONG"
	ImportErrorParseUnsupported = "IMPORT_PARSE_FAILED"

	importChapterPreviewRunes      = 600
	importChapterPreviewParagraphs = 3
)

type ImportChapterView struct {
	ID                 string `json:"id"`
	VolumeTitle        string `json:"volumeTitle"`
	Title              string `json:"title"`
	WordCount          int    `json:"wordCount"`
	BodyPreview        string `json:"bodyPreview"`
	Position           int    `json:"position"`
	SourceStartLine    int    `json:"sourceStartLine"`
	SourceEndLine      int    `json:"sourceEndLine"`
	ContentHashSummary string `json:"contentHashSummary"`
	Selected           bool   `json:"selected"`
	CreatedChapterID   string `json:"createdChapterId,omitempty"`
}

type ImportJobView struct {
	ID                   string              `json:"id"`
	Status               string              `json:"status"`
	SourceName           string              `json:"sourceName"`
	SourceFormat         string              `json:"sourceFormat"`
	SourceEncoding       string              `json:"sourceEncoding"`
	SourceHashDigest     string              `json:"sourceHashDigest"`
	SourceBytes          int64               `json:"sourceBytes"`
	ArchivedRelativePath string              `json:"archivedRelativePath"`
	ChapterCount         int                 `json:"chapterCount"`
	SelectedChapterCount int                 `json:"selectedChapterCount"`
	ErrorCode            string              `json:"errorCode,omitempty"`
	ErrorMessage         string              `json:"errorMessage,omitempty"`
	CreatedAt            string              `json:"createdAt"`
	UpdatedAt            string              `json:"updatedAt"`
	AppliedAt            string              `json:"appliedAt,omitempty"`
	ExtractionStatus     string              `json:"extractionStatus,omitempty"`
	ExtractionRunID      string              `json:"extractionRunId,omitempty"`
	Chapters             []ImportChapterView `json:"chapters"`
}

type ImportPrepareResult struct {
	Job ImportJobView `json:"job"`
}

type ImportApplyView struct {
	Job      ImportJobView `json:"job"`
	Chapters []ChapterView `json:"chapters"`
}

func (s *Service) PrepareManuscriptImport(
	ctx context.Context,
	sourcePath string,
) (ImportPrepareResult, error) {
	if err := s.license.Require(ctx, license.ImportManuscript); err != nil {
		return ImportPrepareResult{}, err
	}
	if err := s.beginLifecycleOperation(); err != nil {
		return ImportPrepareResult{}, err
	}
	defer s.endLifecycleOperation()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ImportPrepareResult{}, err
	}
	if s.imports == nil {
		return ImportPrepareResult{}, ErrImportSourceArchive
	}
	store, project, err := s.importStoreAndProjectLocked(ctx)
	if err != nil {
		return ImportPrepareResult{}, err
	}

	jobID := s.ids.NewID()
	prepared, err := s.imports.Prepare(ctx, s.active.packageInfo, jobID, sourcePath)
	if err != nil {
		return ImportPrepareResult{}, stableImportArchiveError(err)
	}
	cleanup := func(cause error) error {
		cleanupErr := s.cleanupImportArchive(
			s.active.packageInfo,
			prepared.ArchivedRelativePath,
		)
		if cleanupErr != nil {
			return errors.Join(cause, cleanupErr)
		}
		return cause
	}
	sourceName, archivedRelativePath, err := safePreparedImportMetadata(prepared)
	if err != nil {
		return ImportPrepareResult{}, cleanup(err)
	}
	prepared.Name = sourceName
	prepared.ArchivedRelativePath = archivedRelativePath
	if err := ctx.Err(); err != nil {
		return ImportPrepareResult{}, cleanup(err)
	}

	parsed, parseErr := importing.ParseChapters(prepared.Text)
	now := s.clock.Now()
	if parseErr != nil {
		code, message := safeImportParseFailure(parseErr)
		failed, err := importing.NewFailedImportJob(importing.FailedImportJobParams{
			ID:                   jobID,
			ProjectID:            project.ID,
			SourceName:           prepared.Name,
			SourceFormat:         prepared.Format,
			SourceEncoding:       prepared.Encoding,
			SourceHash:           prepared.SHA256,
			SourceBytes:          prepared.Bytes,
			ArchivedRelativePath: prepared.ArchivedRelativePath,
			ErrorCode:            code,
			ErrorMessage:         message,
		}, now)
		if err != nil {
			return ImportPrepareResult{}, cleanup(stableImportConstructionError(err))
		}
		if err := store.CreateFailedImportJob(ctx, failed); err != nil {
			return ImportPrepareResult{}, cleanup(stableImportStoreError(err))
		}
		return ImportPrepareResult{Job: importJobView(failed, []importing.ImportChapter{})}, nil
	}
	if err := ctx.Err(); err != nil {
		return ImportPrepareResult{}, cleanup(err)
	}

	chapters := make([]importing.ImportChapter, 0, len(parsed))
	for _, candidate := range parsed {
		chapter, err := importing.NewImportChapter(importing.ImportChapterParams{
			ID:              s.ids.NewID(),
			JobID:           jobID,
			VolumeTitle:     candidate.VolumeTitle,
			Title:           candidate.Title,
			Body:            candidate.Body,
			Position:        candidate.Position,
			SourceStartLine: candidate.SourceStartLine,
			SourceEndLine:   candidate.SourceEndLine,
		})
		if err != nil {
			return ImportPrepareResult{}, cleanup(stableImportConstructionError(err))
		}
		chapters = append(chapters, chapter)
	}
	job, err := importing.NewImportJob(importing.ImportJobParams{
		ID:                   jobID,
		ProjectID:            project.ID,
		SourceName:           prepared.Name,
		SourceFormat:         prepared.Format,
		SourceEncoding:       prepared.Encoding,
		SourceHash:           prepared.SHA256,
		SourceBytes:          prepared.Bytes,
		ArchivedRelativePath: prepared.ArchivedRelativePath,
		ChapterCount:         len(chapters),
	}, now)
	if err != nil {
		return ImportPrepareResult{}, cleanup(stableImportConstructionError(err))
	}
	if err := store.CreateImportJob(ctx, job, chapters); err != nil {
		return ImportPrepareResult{}, cleanup(stableImportStoreError(err))
	}
	return ImportPrepareResult{Job: importJobView(job, chapters)}, nil
}

func (s *Service) ImportJobs(ctx context.Context) ([]ImportJobView, error) {
	if err := s.beginLifecycleOperation(); err != nil {
		return nil, err
	}
	defer s.endLifecycleOperation()

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	store, project, err := s.importStoreAndProjectLocked(ctx)
	if err != nil {
		return nil, err
	}
	jobs, err := store.ListImportJobs(ctx, project.ID)
	if err != nil {
		return nil, stableImportStoreError(err)
	}
	views := make([]ImportJobView, 0, len(jobs))
	for _, job := range jobs {
		if job.ProjectID != project.ID {
			return nil, ErrImportJobProjectMismatch
		}
		views = append(views, importJobView(job, nil))
	}
	return views, nil
}

func (s *Service) ImportJob(ctx context.Context, id string) (ImportJobView, error) {
	if err := s.beginLifecycleOperation(); err != nil {
		return ImportJobView{}, err
	}
	defer s.endLifecycleOperation()

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return ImportJobView{}, ErrNoActiveProject
	}
	store, project, err := s.importStoreAndProjectLocked(ctx)
	if err != nil {
		return ImportJobView{}, err
	}
	details, err := store.ImportJob(ctx, strings.TrimSpace(id))
	if err != nil {
		return ImportJobView{}, stableImportStoreError(err)
	}
	if details.Job.ProjectID != project.ID {
		return ImportJobView{}, ErrImportJobProjectMismatch
	}
	return importJobView(details.Job, details.Chapters), nil
}

func (s *Service) DeleteImportJob(ctx context.Context, id string) error {
	if err := s.beginLifecycleOperation(); err != nil {
		return err
	}
	defer s.endLifecycleOperation()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return err
	}
	store, project, err := s.importStoreAndProjectLocked(ctx)
	if err != nil {
		return err
	}
	details, err := store.ImportJob(ctx, strings.TrimSpace(id))
	if err != nil {
		return stableImportStoreError(err)
	}
	if details.Job.ProjectID != project.ID {
		return ErrImportJobProjectMismatch
	}
	if details.Job.Status == importing.JobStatusApplied {
		return ports.ErrImportJobAlreadyApplied
	}
	if err := store.DeleteImportJob(ctx, details.Job.ID); err != nil {
		return stableImportStoreError(err)
	}
	// The archived source is immutable project-level evidence. Snapshot and
	// branch databases may still reference it after the current job row is
	// deleted, so cross-version garbage collection must decide when it is safe
	// to remove the archive.
	return nil
}

func (s *Service) ApplyImportJob(
	ctx context.Context,
	id string,
	selectedIDs []string,
) (ImportApplyView, error) {
	if err := s.license.Require(ctx, license.ImportManuscript); err != nil {
		return ImportApplyView{}, err
	}
	if err := s.requireEditable(); err != nil {
		return ImportApplyView{}, err
	}
	if len(selectedIDs) == 0 {
		return ImportApplyView{}, ports.ErrNoImportChaptersSelected
	}
	if err := s.beginLifecycleOperation(); err != nil {
		return ImportApplyView{}, err
	}
	defer s.endLifecycleOperation()
	if err := s.beginImportOperation(); err != nil {
		return ImportApplyView{}, err
	}
	defer s.endImportOperation()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ImportApplyView{}, err
	}
	store, project, err := s.importStoreAndProjectLocked(ctx)
	if err != nil {
		return ImportApplyView{}, err
	}
	details, err := store.ImportJob(ctx, strings.TrimSpace(id))
	if err != nil {
		return ImportApplyView{}, stableImportStoreError(err)
	}
	if details.Job.ProjectID != project.ID {
		return ImportApplyView{}, ErrImportJobProjectMismatch
	}
	switch details.Job.Status {
	case importing.JobStatusApplied:
		return ImportApplyView{}, ports.ErrImportJobAlreadyApplied
	case importing.JobStatusPrepared:
	default:
		return ImportApplyView{}, ports.ErrImportJobNotPrepared
	}

	selected := make(map[string]struct{}, len(selectedIDs))
	for _, selectedID := range selectedIDs {
		selectedID = strings.TrimSpace(selectedID)
		if selectedID == "" {
			return ImportApplyView{}, ports.ErrImportChapterMappingMismatch
		}
		if _, duplicate := selected[selectedID]; duplicate {
			return ImportApplyView{}, ports.ErrImportChapterMappingMismatch
		}
		selected[selectedID] = struct{}{}
	}
	candidates := append([]importing.ImportChapter(nil), details.Chapters...)
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].Position < candidates[j].Position
	})
	now := s.clock.Now().UTC()
	orderedIDs := make([]string, 0, len(selected))
	requestedChapters := make([]projectdomain.Chapter, 0, len(selected))
	for _, candidate := range candidates {
		if _, ok := selected[candidate.ID]; !ok {
			continue
		}
		orderedIDs = append(orderedIDs, candidate.ID)
		requestedChapters = append(requestedChapters, projectdomain.Chapter{
			ID:        s.ids.NewID(),
			ProjectID: project.ID,
			Title:     candidate.Title,
			Position:  0,
			Body:      candidate.Body,
			Revision:  1,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	if len(orderedIDs) != len(selected) {
		return ImportApplyView{}, ports.ErrImportChapterMappingMismatch
	}

	applied, err := store.ApplyImportJob(ctx, details.Job.ID, orderedIDs, requestedChapters, now)
	if err != nil {
		return ImportApplyView{}, stableImportStoreError(err)
	}
	if applied.Job.ProjectID != project.ID ||
		len(applied.Chapters) != len(orderedIDs) {
		return ImportApplyView{}, ports.ErrImportChapterMappingMismatch
	}
	createdByCandidate := make(map[string]string, len(orderedIDs))
	for index, candidateID := range orderedIDs {
		createdByCandidate[candidateID] = applied.Chapters[index].ID
	}
	for index := range candidates {
		if createdID, ok := createdByCandidate[candidates[index].ID]; ok {
			candidates[index].Selected = true
			candidates[index].CreatedChapterID = createdID
		}
	}
	workspaceChapters, err := s.active.store.ListChapters(ctx)
	if err != nil {
		return ImportApplyView{}, errors.Join(
			ErrImportAppliedRefresh,
			stableImportWorkspaceReadError(err),
		)
	}
	sort.SliceStable(workspaceChapters, func(i, j int) bool {
		return workspaceChapters[i].Position < workspaceChapters[j].Position
	})
	chapterViews := make([]ChapterView, 0, len(workspaceChapters))
	for _, chapter := range workspaceChapters {
		chapterViews = append(chapterViews, chapterView(chapter))
	}
	return ImportApplyView{
		Job:      importJobView(applied.Job, candidates),
		Chapters: chapterViews,
	}, nil
}

func (s *Service) importStoreAndProjectLocked(
	ctx context.Context,
) (ports.ImportStore, projectdomain.Project, error) {
	store, ok := s.active.store.(ports.ImportStore)
	if !ok {
		return nil, projectdomain.Project{}, ErrImportStoreUnavailable
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return nil, projectdomain.Project{}, stableImportWorkspaceReadError(err)
	}
	return store, project, nil
}

func (s *Service) cleanupImportArchive(
	projectPackage ports.ProjectPackage,
	relativePath string,
) error {
	if s.imports == nil || strings.TrimSpace(relativePath) == "" {
		return nil
	}
	cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.imports.Delete(cleanupContext, projectPackage, relativePath); err != nil {
		return ErrImportArchiveCleanup
	}
	return nil
}

func safePreparedImportMetadata(
	prepared ports.PreparedImportSource,
) (string, string, error) {
	sourceName := path.Base(strings.ReplaceAll(strings.TrimSpace(prepared.Name), "\\", "/"))
	if sourceName == "" || sourceName == "." || sourceName == ".." || sourceName == "/" {
		return "", "", ErrImportSourceArchive
	}
	relativePath := strings.ReplaceAll(strings.TrimSpace(prepared.ArchivedRelativePath), "\\", "/")
	cleaned := path.Clean(relativePath)
	if cleaned != relativePath ||
		path.IsAbs(cleaned) ||
		!strings.HasPrefix(cleaned, "imports/") ||
		strings.Contains(cleaned, ":") {
		return "", "", ErrImportSourceArchive
	}
	return sourceName, cleaned, nil
}

func stableImportArchiveError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	for _, stable := range []error{
		ports.ErrUnsupportedImportFormat,
		ports.ErrEmptyImportSource,
		ports.ErrImportSourceTooLarge,
		ports.ErrUnsupportedImportEncoding,
		ports.ErrUnsafeImportSource,
		ports.ErrImportArchiveExists,
		ports.ErrImportArchiveUnsupported,
		ports.ErrInvalidImportArchivePath,
	} {
		if errors.Is(err, stable) {
			return stable
		}
	}
	return ErrImportSourceArchive
}

func stableImportStoreError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	for _, stable := range []error{
		ports.ErrImportJobMissing,
		ports.ErrDuplicateImportSource,
		ports.ErrImportJobAlreadyApplied,
		ports.ErrImportJobNotPrepared,
		ports.ErrNoImportChaptersSelected,
		ports.ErrImportChapterMappingMismatch,
		ports.ErrImportPersistence,
	} {
		if errors.Is(err, stable) {
			return stable
		}
	}
	return ports.ErrImportPersistence
}

func stableImportWorkspaceReadError(err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return ErrImportWorkspaceRead
}

func stableImportConstructionError(err error) error {
	for _, stable := range []error{
		importing.ErrBlankValue,
		importing.ErrInvalidSourceFormat,
		importing.ErrInvalidSourceHash,
		importing.ErrInvalidSourceBytes,
		importing.ErrInvalidChapterCount,
		importing.ErrInvalidImportChapter,
	} {
		if errors.Is(err, stable) {
			return stable
		}
	}
	return ports.ErrImportPersistence
}

func safeImportParseFailure(err error) (string, string) {
	switch {
	case errors.Is(err, importing.ErrEmptyManuscript):
		return ImportErrorParseEmpty, "No importable chapters were found."
	case errors.Is(err, importing.ErrManuscriptLineTooLong):
		return ImportErrorLineTooLong, "A manuscript line exceeds the supported length."
	default:
		return ImportErrorParseUnsupported, "The manuscript could not be parsed."
	}
}

func importJobView(
	job importing.ImportJob,
	chapters []importing.ImportChapter,
) ImportJobView {
	chapterViews := make([]ImportChapterView, 0, len(chapters))
	for _, chapter := range chapters {
		contentHashSummary := chapter.ContentHash
		if len(contentHashSummary) > 12 {
			contentHashSummary = contentHashSummary[:12]
		}
		wordCount, bodyPreview := importChapterSummary(chapter.Body)
		chapterViews = append(chapterViews, ImportChapterView{
			ID:                 chapter.ID,
			VolumeTitle:        chapter.VolumeTitle,
			Title:              chapter.Title,
			WordCount:          wordCount,
			BodyPreview:        bodyPreview,
			Position:           chapter.Position,
			SourceStartLine:    chapter.SourceStartLine,
			SourceEndLine:      chapter.SourceEndLine,
			ContentHashSummary: contentHashSummary,
			Selected:           chapter.Selected,
			CreatedChapterID:   chapter.CreatedChapterID,
		})
	}
	hashDigest := job.SourceHash
	if len(hashDigest) > 12 {
		hashDigest = hashDigest[:12]
	}
	view := ImportJobView{
		ID:                   job.ID,
		Status:               string(job.Status),
		SourceName:           job.SourceName,
		SourceFormat:         string(job.SourceFormat),
		SourceEncoding:       job.SourceEncoding,
		SourceHashDigest:     hashDigest,
		SourceBytes:          job.SourceBytes,
		ArchivedRelativePath: job.ArchivedRelativePath,
		ChapterCount:         job.ChapterCount,
		SelectedChapterCount: job.SelectedChapterCount,
		ErrorCode:            job.ErrorCode,
		ErrorMessage:         job.ErrorMessage,
		CreatedAt:            job.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:            job.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Chapters:             chapterViews,
	}
	if job.AppliedAt != nil {
		view.AppliedAt = job.AppliedAt.UTC().Format(time.RFC3339Nano)
	}
	// "none" is the default for applied jobs; omit it so the front end can treat
	// absence as "no extraction run yet" without special-casing the string.
	if job.ExtractionStatus != "" && job.ExtractionStatus != importing.ExtractionStatusNone {
		view.ExtractionStatus = string(job.ExtractionStatus)
	}
	view.ExtractionRunID = job.ExtractionRunID
	return view
}

func importChapterSummary(body string) (int, string) {
	if body == "" {
		return 0, ""
	}

	var preview strings.Builder
	previewCapacity := len(body)
	if maximum := importChapterPreviewRunes * 4; previewCapacity > maximum {
		previewCapacity = maximum
	}
	preview.Grow(previewCapacity)

	wordCount := 0
	bodyRunes := 0
	paragraphCount := 0
	inParagraph := false
	lineHasContent := false
	truncated := false
	for _, value := range body {
		bodyRunes++
		if !unicode.IsSpace(value) {
			wordCount++
			lineHasContent = true
			if !inParagraph {
				paragraphCount++
				inParagraph = true
				if paragraphCount > importChapterPreviewParagraphs {
					truncated = true
				}
			}
		}
		if bodyRunes > importChapterPreviewRunes {
			truncated = true
		}
		if !truncated && bodyRunes < importChapterPreviewRunes {
			preview.WriteRune(value)
		}
		if value == '\n' {
			if !lineHasContent {
				inParagraph = false
			}
			lineHasContent = false
		}
	}
	if !truncated {
		return wordCount, body
	}
	return wordCount, strings.TrimRightFunc(preview.String(), unicode.IsSpace) + "…"
}
