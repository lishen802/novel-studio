package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

func TestImportStoreCreatesListsAndReloadsCompleteJobsInStableOrder(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	store, project := createImportTestStore(t, path)

	older, olderChapters := mustImportFixture(t, project.ID, "job-old", "old.txt", "a", time.Unix(100, 12))
	newer, newerChapters := mustImportFixture(t, project.ID, "job-new", "new.md", "b", time.Unix(200, 34))
	newer.SourceFormat = importing.SourceFormatMarkdown
	if err := store.CreateImportJob(ctx, older, olderChapters); err != nil {
		t.Fatalf("create older import job: %v", err)
	}
	if err := store.CreateImportJob(ctx, newer, newerChapters); err != nil {
		t.Fatalf("create newer import job: %v", err)
	}

	jobs, err := store.ListImportJobs(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 2 || jobs[0] != newer || jobs[1] != older {
		t.Fatalf("jobs = %#v", jobs)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := (Factory{}).Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	store = opened.(*Store)
	defer store.Close()

	details, err := store.ImportJob(ctx, older.ID)
	if err != nil {
		t.Fatal(err)
	}
	if details.Job != older {
		t.Fatalf("reloaded job = %#v, want %#v", details.Job, older)
	}
	if len(details.Chapters) != 2 ||
		details.Chapters[0] != olderChapters[0] ||
		details.Chapters[1] != olderChapters[1] {
		t.Fatalf("reloaded chapters = %#v", details.Chapters)
	}
	if details.Job.AppliedAt != nil {
		t.Fatalf("prepared applied_at = %v, want nil", details.Job.AppliedAt)
	}
}

func TestImportStoreCreateIsAtomicAndReturnsStableDuplicateSourceError(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	job, chapters := mustImportFixture(t, project.ID, "job-1", "novel.txt", "a", time.Unix(100, 0))

	badChapters := append([]importing.ImportChapter(nil), chapters...)
	badChapters[1].JobID = "another-job"
	err := store.CreateImportJob(ctx, job, badChapters)
	if !errors.Is(err, ports.ErrImportChapterMappingMismatch) {
		t.Fatalf("error = %v, want ErrImportChapterMappingMismatch", err)
	}
	if _, err := store.ImportJob(ctx, job.ID); !errors.Is(err, ports.ErrImportJobMissing) {
		t.Fatalf("read partially created job error = %v, want ErrImportJobMissing", err)
	}

	if err := store.CreateImportJob(ctx, job, chapters); err != nil {
		t.Fatal(err)
	}
	duplicate := job
	duplicate.ID = "job-duplicate"
	err = store.CreateImportJob(ctx, duplicate, chaptersForJob(chapters, duplicate.ID))
	if !errors.Is(err, ports.ErrDuplicateImportSource) {
		t.Fatalf("error = %v, want ErrDuplicateImportSource", err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "unique constraint") ||
		strings.Contains(err.Error(), "import_jobs.project_id") {
		t.Fatalf("duplicate error leaks SQLite details: %v", err)
	}

	conflictingID := job
	conflictingID.SourceHash = strings.Repeat("c", 64)
	err = store.CreateImportJob(ctx, conflictingID, chapters)
	if !errors.Is(err, ports.ErrImportPersistence) ||
		errors.Is(err, ports.ErrDuplicateImportSource) {
		t.Fatalf("duplicate job ID error = %v, want safe persistence error", err)
	}
}

func TestImportStoreRollsBackJobAndCandidatesWhenCandidateInsertFails(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	job, chapters := mustImportFixture(t, project.ID, "job-create-rollback", "novel.txt", "a", time.Unix(100, 0))
	if _, err := store.db.ExecContext(ctx, `
		CREATE TRIGGER fail_second_import_candidate
		BEFORE INSERT ON import_chapters
		WHEN NEW.id = 'job-create-rollback-c2'
		BEGIN
			SELECT RAISE(ABORT, 'injected candidate failure');
		END
	`); err != nil {
		t.Fatal(err)
	}

	err := store.CreateImportJob(ctx, job, chapters)
	if !errors.Is(err, ports.ErrImportPersistence) {
		t.Fatalf("error = %v, want ErrImportPersistence", err)
	}
	if strings.Contains(err.Error(), "injected candidate failure") {
		t.Fatalf("error leaks low-level SQLite detail: %v", err)
	}
	if _, err := store.ImportJob(ctx, job.ID); !errors.Is(err, ports.ErrImportJobMissing) {
		t.Fatalf("read rolled back job error = %v, want ErrImportJobMissing", err)
	}
	var candidateCount int
	if err := store.db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM import_chapters WHERE job_id = ?",
		job.ID,
	).Scan(&candidateCount); err != nil {
		t.Fatal(err)
	}
	if candidateCount != 0 {
		t.Fatalf("candidate count after rollback = %d, want 0", candidateCount)
	}
}

func TestImportStoreReportsMissingJobsAndDeletesOnlyPreparedJobs(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()

	if _, err := store.ImportJob(ctx, "missing"); !errors.Is(err, ports.ErrImportJobMissing) {
		t.Fatalf("read missing error = %v", err)
	}
	if err := store.DeletePreparedImportJob(ctx, "missing"); !errors.Is(err, ports.ErrImportJobMissing) {
		t.Fatalf("delete missing error = %v", err)
	}
	missingChapter := projectdomain.Chapter{
		ID:        "chapter-missing-job",
		ProjectID: project.ID,
		Title:     "第一章",
		Revision:  1,
	}
	if _, err := store.ApplyImportJob(
		ctx,
		"missing",
		[]string{"candidate-missing-job"},
		[]projectdomain.Chapter{missingChapter},
		time.Unix(100, 0),
	); !errors.Is(err, ports.ErrImportJobMissing) {
		t.Fatalf("apply missing error = %v, want ErrImportJobMissing", err)
	}

	prepared, candidates := mustImportFixture(t, project.ID, "job-prepared", "prepared.txt", "a", time.Unix(100, 0))
	if err := store.CreateImportJob(ctx, prepared, candidates); err != nil {
		t.Fatal(err)
	}
	if err := store.DeletePreparedImportJob(ctx, prepared.ID); err != nil {
		t.Fatalf("delete prepared job: %v", err)
	}
	if _, err := store.ImportJob(ctx, prepared.ID); !errors.Is(err, ports.ErrImportJobMissing) {
		t.Fatalf("read deleted job error = %v", err)
	}
	var candidateCount int
	if err := store.db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM import_chapters WHERE job_id = ?",
		prepared.ID,
	).Scan(&candidateCount); err != nil {
		t.Fatal(err)
	}
	if candidateCount != 0 {
		t.Fatalf("candidate count after delete = %d", candidateCount)
	}

	applied, appliedCandidates := mustImportFixture(t, project.ID, "job-applied", "applied.txt", "b", time.Unix(200, 0))
	if err := store.CreateImportJob(ctx, applied, appliedCandidates); err != nil {
		t.Fatal(err)
	}
	chapter := importChapterForCandidate(appliedCandidates[0], "chapter-imported", project.ID, time.Unix(300, 0))
	if _, err := store.ApplyImportJob(
		ctx,
		applied.ID,
		[]string{appliedCandidates[0].ID},
		[]projectdomain.Chapter{chapter},
		time.Unix(300, 0),
	); err != nil {
		t.Fatalf("apply import job: %v", err)
	}
	if err := store.DeletePreparedImportJob(ctx, applied.ID); !errors.Is(err, ports.ErrImportJobAlreadyApplied) {
		t.Fatalf("delete applied error = %v, want ErrImportJobAlreadyApplied", err)
	}
}

func TestImportStoreAppliesSelectedCandidatesAtomicallyInCandidateOrder(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "project.db")
	store, project := createImportTestStore(t, path)
	t.Cleanup(func() {
		_ = store.Close()
	})

	existing := mustChapter(t, "existing", project.ID, "已有章节", 3)
	if err := store.CreateChapter(ctx, existing); err != nil {
		t.Fatal(err)
	}
	job, candidates := mustImportFixture(t, project.ID, "job-apply", "novel.txt", "a", time.Unix(100, 0))
	third, err := importing.NewImportChapter(importing.ImportChapterParams{
		ID:              "job-apply-c3",
		JobID:           job.ID,
		VolumeTitle:     "第二卷",
		Title:           "第三章",
		Body:            "正文三",
		Position:        3,
		SourceStartLine: 7,
		SourceEndLine:   9,
	})
	if err != nil {
		t.Fatal(err)
	}
	candidates = append(candidates, third)
	job.ChapterCount = len(candidates)
	if err := store.CreateImportJob(ctx, job, candidates); err != nil {
		t.Fatal(err)
	}

	appliedAt := time.Unix(500, 987).UTC()
	firstChapter := importChapterForCandidate(candidates[0], "created-first", project.ID, appliedAt)
	thirdChapter := importChapterForCandidate(candidates[2], "created-third", project.ID, appliedAt)
	result, err := store.ApplyImportJob(
		ctx,
		job.ID,
		[]string{candidates[2].ID, candidates[0].ID},
		[]projectdomain.Chapter{thirdChapter, firstChapter},
		appliedAt,
	)
	if err != nil {
		t.Fatalf("apply import job: %v", err)
	}

	if result.Job.Status != importing.JobStatusApplied ||
		result.Job.SelectedChapterCount != 2 ||
		result.Job.AppliedAt == nil ||
		!result.Job.AppliedAt.Equal(appliedAt) ||
		!result.Job.UpdatedAt.Equal(appliedAt) {
		t.Fatalf("applied job = %#v", result.Job)
	}
	if len(result.Chapters) != 2 ||
		result.Chapters[0].ID != firstChapter.ID ||
		result.Chapters[0].Position != 4 ||
		result.Chapters[1].ID != thirdChapter.ID ||
		result.Chapters[1].Position != 5 {
		t.Fatalf("created chapters = %#v", result.Chapters)
	}
	for _, chapter := range result.Chapters {
		if chapter.ProjectID != project.ID || chapter.Revision != 1 ||
			!chapter.CreatedAt.Equal(appliedAt) || !chapter.UpdatedAt.Equal(appliedAt) {
			t.Fatalf("created chapter = %#v", chapter)
		}
	}

	details, err := store.ImportJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !details.Chapters[0].Selected ||
		details.Chapters[0].CreatedChapterID != firstChapter.ID ||
		details.Chapters[1].Selected ||
		details.Chapters[1].CreatedChapterID != "" ||
		!details.Chapters[2].Selected ||
		details.Chapters[2].CreatedChapterID != thirdChapter.ID {
		t.Fatalf("candidate mappings = %#v", details.Chapters)
	}

	for _, chapter := range result.Chapters {
		var source, summary string
		if err := store.db.QueryRowContext(ctx, `
			SELECT revision_source, revision_summary
			FROM chapters
			WHERE id = ?
		`, chapter.ID).Scan(&source, &summary); err != nil {
			t.Fatal(err)
		}
		if source != "initial" || summary != "从导入文件 novel.txt 创建" {
			t.Fatalf("chapter %s revision metadata = (%q, %q)", chapter.ID, source, summary)
		}
	}

	_, err = store.ApplyImportJob(
		ctx,
		job.ID,
		[]string{candidates[0].ID},
		[]projectdomain.Chapter{firstChapter},
		appliedAt.Add(time.Second),
	)
	if !errors.Is(err, ports.ErrImportJobAlreadyApplied) {
		t.Fatalf("second apply error = %v, want ErrImportJobAlreadyApplied", err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := (Factory{}).Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen applied import store: %v", err)
	}
	store = opened.(*Store)
	reloaded, err := store.ImportJob(ctx, job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Job.AppliedAt == nil ||
		!reloaded.Job.AppliedAt.Equal(appliedAt) ||
		reloaded.Chapters[0].CreatedChapterID != firstChapter.ID ||
		reloaded.Chapters[2].CreatedChapterID != thirdChapter.ID {
		t.Fatalf("reloaded applied details = %#v", reloaded)
	}
}

func TestImportStoreRejectsInvalidSelectionsAndChapterMappingsWithoutMutation(t *testing.T) {
	testCases := []struct {
		name        string
		selectedIDs []string
		chapters    func(projectID string, candidates []importing.ImportChapter) []projectdomain.Chapter
		wantErr     error
	}{
		{
			name:        "no selection",
			selectedIDs: nil,
			chapters: func(string, []importing.ImportChapter) []projectdomain.Chapter {
				return nil
			},
			wantErr: ports.ErrNoImportChaptersSelected,
		},
		{
			name:        "length mismatch",
			selectedIDs: []string{"candidate-1"},
			chapters: func(string, []importing.ImportChapter) []projectdomain.Chapter {
				return nil
			},
			wantErr: ports.ErrImportChapterMappingMismatch,
		},
		{
			name:        "duplicate candidate",
			selectedIDs: []string{"candidate-1", "candidate-1"},
			chapters: func(projectID string, candidates []importing.ImportChapter) []projectdomain.Chapter {
				return []projectdomain.Chapter{
					importChapterForCandidate(candidates[0], "chapter-1", projectID, time.Unix(200, 0)),
					importChapterForCandidate(candidates[0], "chapter-2", projectID, time.Unix(200, 0)),
				}
			},
			wantErr: ports.ErrImportChapterMappingMismatch,
		},
		{
			name:        "foreign candidate",
			selectedIDs: []string{"foreign"},
			chapters: func(projectID string, candidates []importing.ImportChapter) []projectdomain.Chapter {
				return []projectdomain.Chapter{
					importChapterForCandidate(candidates[0], "chapter-1", projectID, time.Unix(200, 0)),
				}
			},
			wantErr: ports.ErrImportChapterMappingMismatch,
		},
		{
			name:        "wrong project",
			selectedIDs: []string{"candidate-1"},
			chapters: func(_ string, candidates []importing.ImportChapter) []projectdomain.Chapter {
				return []projectdomain.Chapter{
					importChapterForCandidate(candidates[0], "chapter-1", "other-project", time.Unix(200, 0)),
				}
			},
			wantErr: ports.ErrImportChapterMappingMismatch,
		},
		{
			name:        "wrong title",
			selectedIDs: []string{"candidate-1"},
			chapters: func(projectID string, candidates []importing.ImportChapter) []projectdomain.Chapter {
				chapter := importChapterForCandidate(candidates[0], "chapter-1", projectID, time.Unix(200, 0))
				chapter.Title = "错误标题"
				return []projectdomain.Chapter{chapter}
			},
			wantErr: ports.ErrImportChapterMappingMismatch,
		},
		{
			name:        "wrong body",
			selectedIDs: []string{"candidate-1"},
			chapters: func(projectID string, candidates []importing.ImportChapter) []projectdomain.Chapter {
				chapter := importChapterForCandidate(candidates[0], "chapter-1", projectID, time.Unix(200, 0))
				chapter.Body = "错误正文"
				return []projectdomain.Chapter{chapter}
			},
			wantErr: ports.ErrImportChapterMappingMismatch,
		},
		{
			name:        "wrong revision",
			selectedIDs: []string{"candidate-1"},
			chapters: func(projectID string, candidates []importing.ImportChapter) []projectdomain.Chapter {
				chapter := importChapterForCandidate(candidates[0], "chapter-1", projectID, time.Unix(200, 0))
				chapter.Revision = 2
				return []projectdomain.Chapter{chapter}
			},
			wantErr: ports.ErrImportChapterMappingMismatch,
		},
		{
			name:        "preassigned wrong position",
			selectedIDs: []string{"candidate-1"},
			chapters: func(projectID string, candidates []importing.ImportChapter) []projectdomain.Chapter {
				chapter := importChapterForCandidate(candidates[0], "chapter-1", projectID, time.Unix(200, 0))
				chapter.Position = 99
				return []projectdomain.Chapter{chapter}
			},
			wantErr: ports.ErrImportChapterMappingMismatch,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			store, project := createTestStore(t)
			defer store.Close()
			job, candidates := mustImportFixture(t, project.ID, "job-validate", "novel.txt", "a", time.Unix(100, 0))
			candidates[0].ID = "candidate-1"
			candidates[1].ID = "candidate-2"
			if err := store.CreateImportJob(ctx, job, candidates); err != nil {
				t.Fatal(err)
			}

			_, err := store.ApplyImportJob(
				ctx,
				job.ID,
				testCase.selectedIDs,
				testCase.chapters(project.ID, candidates),
				time.Unix(200, 0),
			)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("error = %v, want %v", err, testCase.wantErr)
			}
			assertPreparedImportUnchanged(t, store, job.ID)
		})
	}
}

func TestImportStoreRollsBackChaptersMappingsAndJobWhenAChapterInsertFails(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	job, candidates := mustImportFixture(t, project.ID, "job-rollback", "novel.txt", "a", time.Unix(100, 0))
	if err := store.CreateImportJob(ctx, job, candidates); err != nil {
		t.Fatal(err)
	}
	if _, err := store.db.ExecContext(ctx, `
		CREATE TRIGGER fail_second_imported_chapter
		BEFORE INSERT ON chapters
		WHEN NEW.id = 'chapter-fail'
		BEGIN
			SELECT RAISE(ABORT, 'injected import failure');
		END
	`); err != nil {
		t.Fatal(err)
	}

	appliedAt := time.Unix(200, 0)
	first := importChapterForCandidate(candidates[0], "chapter-ok", project.ID, appliedAt)
	second := importChapterForCandidate(candidates[1], "chapter-fail", project.ID, appliedAt)
	_, err := store.ApplyImportJob(
		ctx,
		job.ID,
		[]string{candidates[0].ID, candidates[1].ID},
		[]projectdomain.Chapter{first, second},
		appliedAt,
	)
	if err == nil {
		t.Fatal("apply succeeded, want injected failure")
	}
	if strings.Contains(err.Error(), "injected import failure") {
		t.Fatalf("error leaks low-level SQLite detail: %v", err)
	}

	assertPreparedImportUnchanged(t, store, job.ID)
	for _, id := range []string{first.ID, second.ID} {
		var count int
		if err := store.db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM chapters WHERE id = ?",
			id,
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("chapter %s persisted after rollback", id)
		}
	}
}

func createImportTestStore(t *testing.T, path string) (*Store, projectdomain.Project) {
	t.Helper()
	project, err := projectdomain.NewProject(
		"project-1",
		"测试小说",
		"奇幻",
		"",
		time.Unix(50, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	created, err := (Factory{}).Create(context.Background(), path, project)
	if err != nil {
		t.Fatal(err)
	}
	store, ok := created.(*Store)
	if !ok {
		t.Fatalf("store type = %T, want *Store", created)
	}
	return store, project
}

func mustImportFixture(
	t *testing.T,
	projectID, jobID, sourceName, hashCharacter string,
	now time.Time,
) (importing.ImportJob, []importing.ImportChapter) {
	t.Helper()
	job, err := importing.NewImportJob(importing.ImportJobParams{
		ID:                   jobID,
		ProjectID:            projectID,
		SourceName:           sourceName,
		SourceFormat:         importing.SourceFormatTXT,
		SourceEncoding:       "UTF-8",
		SourceHash:           strings.Repeat(hashCharacter, 64),
		SourceBytes:          100,
		ArchivedRelativePath: "imports/" + jobID + ".txt",
		ChapterCount:         2,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	first, err := importing.NewImportChapter(importing.ImportChapterParams{
		ID:              jobID + "-c1",
		JobID:           jobID,
		VolumeTitle:     "第一卷",
		Title:           "第一章",
		Body:            "正文一",
		Position:        1,
		SourceStartLine: 1,
		SourceEndLine:   3,
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := importing.NewImportChapter(importing.ImportChapterParams{
		ID:              jobID + "-c2",
		JobID:           jobID,
		VolumeTitle:     "第一卷",
		Title:           "第二章",
		Body:            "正文二",
		Position:        2,
		SourceStartLine: 4,
		SourceEndLine:   6,
	})
	if err != nil {
		t.Fatal(err)
	}
	return job, []importing.ImportChapter{first, second}
}

func chaptersForJob(chapters []importing.ImportChapter, jobID string) []importing.ImportChapter {
	result := append([]importing.ImportChapter(nil), chapters...)
	for index := range result {
		result[index].ID = jobID + "-c" + string(rune('1'+index))
		result[index].JobID = jobID
	}
	return result
}

func importChapterForCandidate(
	candidate importing.ImportChapter,
	id, projectID string,
	now time.Time,
) projectdomain.Chapter {
	return projectdomain.Chapter{
		ID:        id,
		ProjectID: projectID,
		Title:     candidate.Title,
		Position:  0,
		Body:      candidate.Body,
		Revision:  1,
		CreatedAt: now.UTC(),
		UpdatedAt: now.UTC(),
	}
}

func assertPreparedImportUnchanged(t *testing.T, store *Store, jobID string) {
	t.Helper()
	details, err := store.ImportJob(context.Background(), jobID)
	if err != nil {
		t.Fatal(err)
	}
	if details.Job.Status != importing.JobStatusPrepared ||
		details.Job.SelectedChapterCount != 0 ||
		details.Job.AppliedAt != nil {
		t.Fatalf("job mutated after failed apply: %#v", details.Job)
	}
	for _, candidate := range details.Chapters {
		if candidate.Selected || candidate.CreatedChapterID != "" {
			t.Fatalf("candidate mutated after failed apply: %#v", candidate)
		}
	}
}
