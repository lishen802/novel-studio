package workspace

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

func TestPrepareManuscriptImportPersistsTXTAndMarkdownMetadataAndCandidates(t *testing.T) {
	for _, test := range []struct {
		name     string
		format   importing.SourceFormat
		encoding string
		source   string
	}{
		{name: "txt utf16", format: importing.SourceFormatTXT, encoding: "utf-16le", source: "/private/books/长夜.txt"},
		{name: "markdown gb18030", format: importing.SourceFormatMarkdown, encoding: "gb18030", source: `C:\private\books\长夜.md`},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := mustProject(t, "project-1", "长夜织境")
			store := newFakeImportStore(project)
			archive := &fakeImportArchive{prepared: ports.PreparedImportSource{
				Name:                 "长夜" + sourceExtension(test.format),
				Format:               test.format,
				Encoding:             test.encoding,
				SHA256:               strings.Repeat("a", 64),
				Bytes:                321,
				ArchivedRelativePath: "imports/job-1" + sourceExtension(test.format),
				Text:                 "第一卷 起点\n第一章 相逢\n正文一\n第二章 离别\n正文二",
			}}
			service := openImportTestService(
				t,
				store,
				archive,
				&fakeLicense{},
				&sequenceIDs{ids: []string{"job-1", "candidate-1", "candidate-2"}},
			)

			result, err := service.PrepareManuscriptImport(context.Background(), test.source)
			if err != nil {
				t.Fatal(err)
			}
			if result.Job.ID != "job-1" ||
				result.Job.Status != "prepared" ||
				result.Job.SourceName != archive.prepared.Name ||
				result.Job.SourceFormat != string(test.format) ||
				result.Job.SourceEncoding != test.encoding ||
				result.Job.SourceBytes != 321 ||
				result.Job.SourceHashDigest != strings.Repeat("a", 12) ||
				result.Job.ArchivedRelativePath != archive.prepared.ArchivedRelativePath ||
				len(result.Job.Chapters) != 2 {
				t.Fatalf("result = %#v", result)
			}
			first := result.Job.Chapters[0]
			if first.ID != "candidate-1" ||
				first.VolumeTitle != "第一卷 起点" ||
				first.Title != "第一章 相逢" ||
				first.Body != "正文一" ||
				first.Position != 1 ||
				first.SourceStartLine != 3 ||
				first.SourceEndLine != 3 ||
				first.ContentHash == "" ||
				first.Selected ||
				first.CreatedChapterID != "" {
				t.Fatalf("first candidate = %#v", first)
			}
			if strings.Contains(result.Job.SourceName, "/private/") ||
				strings.Contains(result.Job.SourceName, `C:\private`) {
				t.Fatalf("view leaks source path: %#v", result.Job)
			}
			if archive.prepareCalls != 1 || archive.lastSourcePath != test.source {
				t.Fatalf("archive calls = %d, path = %q", archive.prepareCalls, archive.lastSourcePath)
			}
			if store.createCalls != 1 {
				t.Fatalf("CreateImportJob calls = %d", store.createCalls)
			}
		})
	}
}

func TestPrepareManuscriptImportPersistsParseFailureAndRetainsArchive(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeImportStore(project)
	archive := &fakeImportArchive{prepared: ports.PreparedImportSource{
		Name:                 "empty.txt",
		Format:               importing.SourceFormatTXT,
		Encoding:             "utf-8",
		SHA256:               strings.Repeat("b", 64),
		Bytes:                4,
		ArchivedRelativePath: "imports/job-failed.txt",
		Text:                 " \n\t",
	}}
	service := openImportTestService(
		t,
		store,
		archive,
		&fakeLicense{},
		&sequenceIDs{ids: []string{"job-failed"}},
	)

	result, err := service.PrepareManuscriptImport(context.Background(), "/private/empty.txt")
	if err != nil {
		t.Fatal(err)
	}
	if result.Job.Status != "failed" ||
		result.Job.ChapterCount != 0 ||
		len(result.Job.Chapters) != 0 ||
		result.Job.ErrorCode != ImportErrorParseEmpty ||
		result.Job.ErrorMessage == "" {
		t.Fatalf("failed result = %#v", result)
	}
	if store.failedCreateCalls != 1 || len(archive.deleted) != 0 {
		t.Fatalf("failed create calls = %d, deleted = %#v", store.failedCreateCalls, archive.deleted)
	}
	if strings.Contains(result.Job.ErrorMessage, "/private/empty.txt") {
		t.Fatalf("failed view leaks source path: %#v", result.Job)
	}
}

func TestPrepareManuscriptImportCleansArchiveAfterStoreOrDuplicateFailure(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "persistence", err: ports.ErrImportPersistence},
		{name: "duplicate", err: ports.ErrDuplicateImportSource},
	} {
		t.Run(test.name, func(t *testing.T) {
			project := mustProject(t, "project-1", "长夜织境")
			store := newFakeImportStore(project)
			store.createErr = test.err
			archive := validFakeImportArchive()
			service := openImportTestService(
				t,
				store,
				archive,
				&fakeLicense{},
				&sequenceIDs{ids: []string{"job-1", "candidate-1"}},
			)

			_, err := service.PrepareManuscriptImport(context.Background(), "/private/secret/novel.txt")
			if !errors.Is(err, test.err) {
				t.Fatalf("error = %v, want %v", err, test.err)
			}
			if len(archive.deleted) != 1 ||
				archive.deleted[0] != archive.prepared.ArchivedRelativePath {
				t.Fatalf("deleted archives = %#v", archive.deleted)
			}
			if strings.Contains(err.Error(), "/private/secret") {
				t.Fatalf("error leaks source path: %v", err)
			}
		})
	}
}

func TestPrepareManuscriptImportReturnsStableArchiveErrorAndPreservesContext(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	for _, test := range []struct {
		name string
		err  error
		want error
	}{
		{
			name: "unknown is sanitized",
			err:  errors.New("cannot read /private/secret/novel.txt"),
			want: ErrImportSourceArchive,
		},
		{
			name: "cancelled is preserved",
			err:  errors.Join(errors.New("source /private/secret/novel.txt"), context.Canceled),
			want: context.Canceled,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeImportStore(project)
			archive := &fakeImportArchive{prepareErr: test.err}
			service := openImportTestService(
				t,
				store,
				archive,
				&fakeLicense{},
				&sequenceIDs{ids: []string{"job-1"}},
			)

			_, err := service.PrepareManuscriptImport(context.Background(), "/private/secret/novel.txt")
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if strings.Contains(err.Error(), "/private/secret") {
				t.Fatalf("error leaks source path: %v", err)
			}
			if store.createCalls != 0 || store.failedCreateCalls != 0 {
				t.Fatalf("store was called after archive failure")
			}
		})
	}
}

func TestImportUseCasesSanitizeActiveProjectReadFailures(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	for _, test := range []struct {
		name string
		call func(*Service) error
	}{
		{
			name: "prepare",
			call: func(service *Service) error {
				_, err := service.PrepareManuscriptImport(
					context.Background(),
					"/private/secret/novel.txt",
				)
				return err
			},
		},
		{
			name: "list",
			call: func(service *Service) error {
				_, err := service.ImportJobs(context.Background())
				return err
			},
		},
		{
			name: "detail",
			call: func(service *Service) error {
				_, err := service.ImportJob(context.Background(), "job-1")
				return err
			},
		},
		{
			name: "delete",
			call: func(service *Service) error {
				return service.DeleteImportJob(context.Background(), "job-1")
			},
		},
		{
			name: "apply",
			call: func(service *Service) error {
				_, err := service.ApplyImportJob(
					context.Background(),
					"job-1",
					[]string{"candidate-1"},
				)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeImportStore(project)
			service := openImportTestService(
				t,
				store,
				validFakeImportArchive(),
				&fakeLicense{},
				&sequenceIDs{ids: []string{"job-1"}},
			)
			store.projectErr = errors.New(
				"SELECT project failed for /private/secret/project.db",
			)

			err := test.call(service)
			if !errors.Is(err, ErrImportWorkspaceRead) {
				t.Fatalf("error = %v, want ErrImportWorkspaceRead", err)
			}
			if strings.Contains(err.Error(), "SELECT project") ||
				strings.Contains(err.Error(), "/private/secret") {
				t.Fatalf("error leaks project adapter details: %v", err)
			}
		})
	}
}

func TestImportProjectReadFailurePreservesContextWithoutLeakingAdapterText(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeImportStore(project)
	service := openImportTestService(
		t,
		store,
		validFakeImportArchive(),
		&fakeLicense{},
		&sequenceIDs{},
	)
	store.projectErr = errors.Join(
		errors.New("read /private/secret/project.db"),
		context.DeadlineExceeded,
	)

	_, err := service.ImportJobs(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want context deadline preserved", err)
	}
	if strings.Contains(err.Error(), "/private/secret") {
		t.Fatalf("error leaks project adapter details: %v", err)
	}
}

func TestImportJobQueriesAreProjectIsolated(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeImportStore(project)
	ownJob, ownCandidates := mustWorkspaceImportJob(t, "job-own", project.ID, importing.JobStatusPrepared)
	otherJob, otherCandidates := mustWorkspaceImportJob(t, "job-other", "project-2", importing.JobStatusPrepared)
	store.jobs[ownJob.ID] = ports.ImportJobDetails{Job: ownJob, Chapters: ownCandidates}
	store.jobs[otherJob.ID] = ports.ImportJobDetails{Job: otherJob, Chapters: otherCandidates}
	service := openImportTestService(t, store, validFakeImportArchive(), &fakeLicense{}, &sequenceIDs{})

	jobs, err := service.ImportJobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].ID != ownJob.ID || len(jobs[0].Chapters) != 0 {
		t.Fatalf("jobs = %#v", jobs)
	}
	details, err := service.ImportJob(context.Background(), ownJob.ID)
	if err != nil || len(details.Chapters) != 1 {
		t.Fatalf("ImportJob() = (%#v, %v)", details, err)
	}
	if _, err := service.ImportJob(context.Background(), otherJob.ID); !errors.Is(err, ErrImportJobProjectMismatch) {
		t.Fatalf("other project error = %v, want ErrImportJobProjectMismatch", err)
	}
}

func TestDeleteImportJobDeletesPreparedAndFailedArchivesButRejectsApplied(t *testing.T) {
	for _, status := range []importing.JobStatus{
		importing.JobStatusPrepared,
		importing.JobStatusFailed,
		importing.JobStatusApplied,
	} {
		t.Run(string(status), func(t *testing.T) {
			project := mustProject(t, "project-1", "长夜织境")
			store := newFakeImportStore(project)
			job, candidates := mustWorkspaceImportJob(t, "job-1", project.ID, status)
			store.jobs[job.ID] = ports.ImportJobDetails{Job: job, Chapters: candidates}
			archive := validFakeImportArchive()
			service := openImportTestService(t, store, archive, &fakeLicense{}, &sequenceIDs{})

			err := service.DeleteImportJob(context.Background(), job.ID)
			if status == importing.JobStatusApplied {
				if !errors.Is(err, ports.ErrImportJobAlreadyApplied) {
					t.Fatalf("error = %v, want ErrImportJobAlreadyApplied", err)
				}
				if len(archive.deleted) != 0 {
					t.Fatalf("applied archive deleted: %#v", archive.deleted)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, found := store.jobs[job.ID]; found {
				t.Fatalf("job was not deleted")
			}
			if !reflect.DeepEqual(archive.deleted, []string{job.ArchivedRelativePath}) {
				t.Fatalf("deleted archives = %#v", archive.deleted)
			}
		})
	}
}

func TestApplyImportJobUsesCandidateOrderAndReturnsCompleteWorkspaceChapters(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeImportStore(project)
	store.chapters["existing"] = mustChapter(t, "existing", project.ID, "旧章", 3)
	job, candidates := mustWorkspaceImportJob(t, "job-1", project.ID, importing.JobStatusPrepared)
	second, err := importing.NewImportChapter(importing.ImportChapterParams{
		ID:              "candidate-2",
		JobID:           job.ID,
		Title:           "第二章",
		Body:            "正文二",
		Position:        2,
		SourceStartLine: 3,
		SourceEndLine:   4,
	})
	if err != nil {
		t.Fatal(err)
	}
	candidates = append(candidates, second)
	job.ChapterCount = 2
	store.jobs[job.ID] = ports.ImportJobDetails{Job: job, Chapters: candidates}
	licenses := &fakeLicense{}
	service := openImportTestService(
		t,
		store,
		validFakeImportArchive(),
		licenses,
		&sequenceIDs{ids: []string{"chapter-1", "chapter-2"}},
	)

	result, err := service.ApplyImportJob(
		context.Background(),
		job.ID,
		[]string{"candidate-2", "candidate-1"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if licenses.lastCapability != license.ImportManuscript {
		t.Fatalf("license capability = %q", licenses.lastCapability)
	}
	if !reflect.DeepEqual(store.lastSelectedIDs, []string{"candidate-1", "candidate-2"}) {
		t.Fatalf("selected IDs = %#v", store.lastSelectedIDs)
	}
	if len(store.lastRequestedChapters) != 2 ||
		store.lastRequestedChapters[0].ID != "chapter-1" ||
		store.lastRequestedChapters[0].Body != "正文一" ||
		store.lastRequestedChapters[0].Position != 0 ||
		store.lastRequestedChapters[1].ID != "chapter-2" ||
		store.lastRequestedChapters[1].Body != "正文二" ||
		store.lastRequestedChapters[1].Position != 0 {
		t.Fatalf("requested chapters = %#v", store.lastRequestedChapters)
	}
	if result.Job.Status != "applied" ||
		result.Job.SelectedChapterCount != 2 ||
		len(result.Job.Chapters) != 2 ||
		!result.Job.Chapters[0].Selected ||
		result.Job.Chapters[0].CreatedChapterID != "chapter-1" ||
		len(result.Chapters) != 3 {
		t.Fatalf("apply result = %#v", result)
	}
	positions := make([]int, 0, len(result.Chapters))
	for _, chapter := range result.Chapters {
		positions = append(positions, chapter.Position)
	}
	sort.Ints(positions)
	if !reflect.DeepEqual(positions, []int{3, 4, 5}) {
		t.Fatalf("workspace chapter positions = %#v", positions)
	}
}

func TestApplyImportJobReportsAppliedRefreshFailureWithoutLosingCommittedState(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeImportStore(project)
	job, candidates := mustWorkspaceImportJob(t, "job-1", project.ID, importing.JobStatusPrepared)
	store.jobs[job.ID] = ports.ImportJobDetails{Job: job, Chapters: candidates}
	service := openImportTestService(
		t,
		store,
		validFakeImportArchive(),
		&fakeLicense{},
		&sequenceIDs{ids: []string{"chapter-1", "retry-unused"}},
	)
	store.listChaptersErr = errors.New(
		"SELECT chapters failed for /private/secret/project.db",
	)

	_, err := service.ApplyImportJob(
		context.Background(),
		job.ID,
		[]string{"candidate-1"},
	)
	if !errors.Is(err, ErrImportAppliedRefresh) ||
		!errors.Is(err, ErrImportWorkspaceRead) {
		t.Fatalf(
			"error = %v, want ErrImportAppliedRefresh and ErrImportWorkspaceRead",
			err,
		)
	}
	if strings.Contains(err.Error(), "SELECT chapters") ||
		strings.Contains(err.Error(), "/private/secret") {
		t.Fatalf("refresh error leaks adapter details: %v", err)
	}
	stored, readErr := store.ImportJob(context.Background(), job.ID)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if stored.Job.Status != importing.JobStatusApplied ||
		stored.Job.SelectedChapterCount != 1 ||
		!stored.Chapters[0].Selected ||
		stored.Chapters[0].CreatedChapterID != "chapter-1" {
		t.Fatalf("committed import details = %#v", stored)
	}
	if chapter, found := store.chapters["chapter-1"]; !found ||
		chapter.Body != candidates[0].Body {
		t.Fatalf("created chapter = (%#v, %v)", chapter, found)
	}

	_, retryErr := service.ApplyImportJob(
		context.Background(),
		job.ID,
		[]string{"candidate-1"},
	)
	if !errors.Is(retryErr, ports.ErrImportJobAlreadyApplied) {
		t.Fatalf(
			"retry error = %v, want explicit ErrImportJobAlreadyApplied",
			retryErr,
		)
	}
	if store.applyCalls != 1 {
		t.Fatalf("ApplyImportJob calls after retry = %d, want 1", store.applyCalls)
	}
}

func TestApplyImportJobRejectsEmptyDuplicateUnknownAndRepeatedSelections(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	for _, test := range []struct {
		name     string
		selected []string
		want     error
	}{
		{name: "empty", selected: nil, want: ports.ErrNoImportChaptersSelected},
		{name: "duplicate", selected: []string{"candidate-1", "candidate-1"}, want: ports.ErrImportChapterMappingMismatch},
		{name: "unknown", selected: []string{"unknown"}, want: ports.ErrImportChapterMappingMismatch},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newFakeImportStore(project)
			job, candidates := mustWorkspaceImportJob(t, "job-1", project.ID, importing.JobStatusPrepared)
			store.jobs[job.ID] = ports.ImportJobDetails{Job: job, Chapters: candidates}
			service := openImportTestService(
				t,
				store,
				validFakeImportArchive(),
				&fakeLicense{},
				&sequenceIDs{ids: []string{"chapter-1"}},
			)

			_, err := service.ApplyImportJob(context.Background(), job.ID, test.selected)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if store.applyCalls != 0 {
				t.Fatalf("ApplyImportJob calls = %d", store.applyCalls)
			}
		})
	}

	store := newFakeImportStore(project)
	applied, candidates := mustWorkspaceImportJob(t, "job-applied", project.ID, importing.JobStatusApplied)
	store.jobs[applied.ID] = ports.ImportJobDetails{Job: applied, Chapters: candidates}
	service := openImportTestService(
		t,
		store,
		validFakeImportArchive(),
		&fakeLicense{},
		&sequenceIDs{ids: []string{"chapter-1"}},
	)
	if _, err := service.ApplyImportJob(
		context.Background(),
		applied.ID,
		[]string{"candidate-1"},
	); !errors.Is(err, ports.ErrImportJobAlreadyApplied) {
		t.Fatalf("repeated apply error = %v", err)
	}
}

func TestImportLicenseDenialDoesNotCallIO(t *testing.T) {
	denied := errors.New("license denied")
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeImportStore(project)
	archive := validFakeImportArchive()
	service := openImportTestService(
		t,
		store,
		archive,
		&fakeLicense{err: denied},
		&sequenceIDs{ids: []string{"job-1"}},
	)

	if _, err := service.PrepareManuscriptImport(
		context.Background(),
		"/private/novel.txt",
	); !errors.Is(err, denied) {
		t.Fatalf("prepare error = %v", err)
	}
	if _, err := service.ApplyImportJob(
		context.Background(),
		"job-1",
		[]string{"candidate-1"},
	); !errors.Is(err, denied) {
		t.Fatalf("apply error = %v", err)
	}
	if archive.prepareCalls != 0 || store.importJobCalls != 0 || store.applyCalls != 0 {
		t.Fatalf(
			"IO called after license denial: archive=%d read=%d apply=%d",
			archive.prepareCalls,
			store.importJobCalls,
			store.applyCalls,
		)
	}
}

func TestVersionOperationsAreRejectedWhileImportApplyIsActive(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeImportStore(project)
	job, candidates := mustWorkspaceImportJob(t, "job-1", project.ID, importing.JobStatusPrepared)
	store.jobs[job.ID] = ports.ImportJobDetails{Job: job, Chapters: candidates}
	store.applyStarted = make(chan struct{})
	store.applyRelease = make(chan struct{})
	service := openImportTestService(
		t,
		store,
		validFakeImportArchive(),
		&fakeLicense{},
		&sequenceIDs{ids: []string{"chapter-1"}},
	)

	applyDone := make(chan error, 1)
	go func() {
		_, err := service.ApplyImportJob(
			context.Background(),
			job.ID,
			[]string{"candidate-1"},
		)
		applyDone <- err
	}()
	<-store.applyStarted

	if _, err := service.SwitchBranch(
		context.Background(),
		"main",
	); !errors.Is(err, ErrImportOperationActive) {
		t.Fatalf("SwitchBranch error = %v, want ErrImportOperationActive", err)
	}
	if _, err := service.RestoreSnapshot(
		context.Background(),
		"snapshot-1",
	); !errors.Is(err, ErrImportOperationActive) {
		t.Fatalf("RestoreSnapshot error = %v, want ErrImportOperationActive", err)
	}
	if _, err := service.ConfirmBranchMerge(
		context.Background(),
		"merge-1",
	); !errors.Is(err, ErrImportOperationActive) {
		t.Fatalf("ConfirmBranchMerge error = %v, want ErrImportOperationActive", err)
	}

	close(store.applyRelease)
	if err := <-applyDone; err != nil {
		t.Fatal(err)
	}
}

func TestProjectReplacementWaitsForPrepareAndClosesOnlyAfterOldStoreIsUnused(t *testing.T) {
	firstProject := mustProject(t, "project-1", "第一本")
	secondProject := mustProject(t, "project-2", "第二本")
	firstStore := newFakeImportStore(firstProject)
	secondStore := newFakeImportStore(secondProject)
	archive := validFakeImportArchive()
	archive.prepareStarted = make(chan struct{})
	archive.prepareRelease = make(chan struct{})
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/first.zjproject")},
		&fakeStoreFactory{openStores: []ports.ProjectStore{firstStore, secondStore}},
		&fakeLicense{},
		fixedClock{now: time.Unix(200, 0)},
		&sequenceIDs{ids: []string{"job-1", "candidate-1"}},
		&fakeSnapshotArchive{},
	)
	service.ConfigureImportSourceArchive(archive)
	if _, err := service.OpenProject(context.Background(), "/books/first.zjproject"); err != nil {
		t.Fatal(err)
	}

	prepareDone := make(chan error, 1)
	go func() {
		_, err := service.PrepareManuscriptImport(context.Background(), "/private/novel.txt")
		prepareDone <- err
	}()
	<-archive.prepareStarted
	openDone := make(chan error, 1)
	go func() {
		_, err := service.OpenProject(context.Background(), "/books/second.zjproject")
		openDone <- err
	}()
	select {
	case err := <-openDone:
		t.Fatalf("OpenProject returned while prepare was active: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(archive.prepareRelease)
	if err := <-prepareDone; err != nil {
		t.Fatal(err)
	}
	if err := <-openDone; err != nil {
		t.Fatal(err)
	}
	if firstStore.closeCalls != 1 {
		t.Fatalf("old store close calls = %d, want 1", firstStore.closeCalls)
	}
}

func TestCloseWaitsForImportApplyBeforeClosingStore(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeImportStore(project)
	job, candidates := mustWorkspaceImportJob(t, "job-1", project.ID, importing.JobStatusPrepared)
	store.jobs[job.ID] = ports.ImportJobDetails{Job: job, Chapters: candidates}
	store.applyStarted = make(chan struct{})
	store.applyRelease = make(chan struct{})
	service := openImportTestService(
		t,
		store,
		validFakeImportArchive(),
		&fakeLicense{},
		&sequenceIDs{ids: []string{"chapter-1"}},
	)

	applyDone := make(chan error, 1)
	go func() {
		_, err := service.ApplyImportJob(
			context.Background(),
			job.ID,
			[]string{"candidate-1"},
		)
		applyDone <- err
	}()
	<-store.applyStarted
	closeDone := make(chan error, 1)
	go func() { closeDone <- service.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned while apply was active: %v", err)
	case <-time.After(20 * time.Millisecond):
	}

	close(store.applyRelease)
	if err := <-applyDone; err != nil {
		t.Fatal(err)
	}
	if err := <-closeDone; err != nil {
		t.Fatal(err)
	}
	if store.closeCalls != 1 {
		t.Fatalf("store close calls = %d, want 1", store.closeCalls)
	}
}

func openImportTestService(
	t *testing.T,
	store *fakeImportStore,
	archive *fakeImportArchive,
	licenses *fakeLicense,
	ids *sequenceIDs,
) *Service {
	t.Helper()
	service := NewService(
		&fakePackageManager{openPackage: testPackage("/books/project.zjproject")},
		&fakeStoreFactory{openStore: store},
		licenses,
		fixedClock{now: time.Unix(200, 123)},
		ids,
		&fakeSnapshotArchive{},
	)
	service.ConfigureImportSourceArchive(archive)
	if _, err := service.OpenProject(context.Background(), "/books/project.zjproject"); err != nil {
		t.Fatal(err)
	}
	return service
}

func validFakeImportArchive() *fakeImportArchive {
	return &fakeImportArchive{prepared: ports.PreparedImportSource{
		Name:                 "novel.txt",
		Format:               importing.SourceFormatTXT,
		Encoding:             "utf-8",
		SHA256:               strings.Repeat("a", 64),
		Bytes:                100,
		ArchivedRelativePath: "imports/job-1.txt",
		Text:                 "第一章 开始\n正文一",
	}}
}

func sourceExtension(format importing.SourceFormat) string {
	if format == importing.SourceFormatMarkdown {
		return ".md"
	}
	return ".txt"
}

type fakeImportArchive struct {
	mu             sync.Mutex
	prepared       ports.PreparedImportSource
	prepareErr     error
	deleteErr      error
	prepareCalls   int
	lastSourcePath string
	deleted        []string
	prepareStarted chan struct{}
	prepareRelease chan struct{}
}

func (f *fakeImportArchive) Prepare(
	ctx context.Context,
	_ ports.ProjectPackage,
	_ string,
	sourcePath string,
) (ports.PreparedImportSource, error) {
	f.mu.Lock()
	f.prepareCalls++
	f.lastSourcePath = sourcePath
	started := f.prepareStarted
	release := f.prepareRelease
	f.mu.Unlock()
	if started != nil {
		close(started)
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return ports.PreparedImportSource{}, ctx.Err()
		}
	}
	return f.prepared, f.prepareErr
}

func (f *fakeImportArchive) Delete(
	_ context.Context,
	_ ports.ProjectPackage,
	relativePath string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, relativePath)
	return f.deleteErr
}

type fakeImportStore struct {
	*fakeStore

	mu                    sync.Mutex
	jobs                  map[string]ports.ImportJobDetails
	createErr             error
	failedCreateErr       error
	deleteErr             error
	applyErr              error
	projectErr            error
	listChaptersErr       error
	createCalls           int
	failedCreateCalls     int
	importJobCalls        int
	applyCalls            int
	lastSelectedIDs       []string
	lastRequestedChapters []projectdomain.Chapter
	applyStarted          chan struct{}
	applyRelease          chan struct{}
}

func newFakeImportStore(project projectdomain.Project) *fakeImportStore {
	return &fakeImportStore{
		fakeStore: newFakeStore(project),
		jobs:      make(map[string]ports.ImportJobDetails),
	}
}

func (f *fakeImportStore) Project(ctx context.Context) (projectdomain.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.projectErr != nil {
		return projectdomain.Project{}, f.projectErr
	}
	return f.fakeStore.Project(ctx)
}

func (f *fakeImportStore) ListChapters(
	ctx context.Context,
) ([]projectdomain.Chapter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listChaptersErr != nil {
		return nil, f.listChaptersErr
	}
	return f.fakeStore.ListChapters(ctx)
}

func (f *fakeImportStore) CreateImportJob(
	_ context.Context,
	job importing.ImportJob,
	chapters []importing.ImportChapter,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	if f.createErr != nil {
		return f.createErr
	}
	f.jobs[job.ID] = ports.ImportJobDetails{
		Job:      job,
		Chapters: append([]importing.ImportChapter(nil), chapters...),
	}
	return nil
}

func (f *fakeImportStore) CreateFailedImportJob(
	_ context.Context,
	job importing.ImportJob,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failedCreateCalls++
	if f.failedCreateErr != nil {
		return f.failedCreateErr
	}
	f.jobs[job.ID] = ports.ImportJobDetails{Job: job, Chapters: []importing.ImportChapter{}}
	return nil
}

func (f *fakeImportStore) ListImportJobs(
	_ context.Context,
	projectID string,
) ([]importing.ImportJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]importing.ImportJob, 0)
	for _, details := range f.jobs {
		if details.Job.ProjectID == projectID {
			result = append(result, details.Job)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (f *fakeImportStore) ImportJob(
	_ context.Context,
	id string,
) (ports.ImportJobDetails, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.importJobCalls++
	details, found := f.jobs[id]
	if !found {
		return ports.ImportJobDetails{}, ports.ErrImportJobMissing
	}
	details.Chapters = append([]importing.ImportChapter(nil), details.Chapters...)
	return details, nil
}

func (f *fakeImportStore) DeleteImportJob(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.deleteErr != nil {
		return f.deleteErr
	}
	details, found := f.jobs[id]
	if !found {
		return ports.ErrImportJobMissing
	}
	if details.Job.Status == importing.JobStatusApplied {
		return ports.ErrImportJobAlreadyApplied
	}
	delete(f.jobs, id)
	return nil
}

func (f *fakeImportStore) ApplyImportJob(
	ctx context.Context,
	jobID string,
	selectedIDs []string,
	chapters []projectdomain.Chapter,
	appliedAt time.Time,
) (ports.ImportApplyResult, error) {
	f.mu.Lock()
	f.applyCalls++
	f.lastSelectedIDs = append([]string(nil), selectedIDs...)
	f.lastRequestedChapters = append([]projectdomain.Chapter(nil), chapters...)
	started := f.applyStarted
	release := f.applyRelease
	f.mu.Unlock()
	if started != nil {
		close(started)
	}
	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return ports.ImportApplyResult{}, ctx.Err()
		}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.applyErr != nil {
		return ports.ImportApplyResult{}, f.applyErr
	}
	details, found := f.jobs[jobID]
	if !found {
		return ports.ImportApplyResult{}, ports.ErrImportJobMissing
	}
	if details.Job.Status == importing.JobStatusApplied {
		return ports.ImportApplyResult{}, ports.ErrImportJobAlreadyApplied
	}
	if details.Job.Status != importing.JobStatusPrepared {
		return ports.ImportApplyResult{}, ports.ErrImportJobNotPrepared
	}
	candidateByID := make(map[string]int, len(details.Chapters))
	for index, candidate := range details.Chapters {
		candidateByID[candidate.ID] = index
	}
	maxPosition := 0
	for _, chapter := range f.chapters {
		if chapter.Position > maxPosition {
			maxPosition = chapter.Position
		}
	}
	created := make([]projectdomain.Chapter, 0, len(chapters))
	for index, selectedID := range selectedIDs {
		candidateIndex, ok := candidateByID[selectedID]
		if !ok || index >= len(chapters) {
			return ports.ImportApplyResult{}, ports.ErrImportChapterMappingMismatch
		}
		candidate := details.Chapters[candidateIndex]
		chapter := chapters[index]
		chapter.Position = maxPosition + index + 1
		chapter.CreatedAt = appliedAt.UTC()
		chapter.UpdatedAt = appliedAt.UTC()
		f.chapters[chapter.ID] = chapter
		candidate.Selected = true
		candidate.CreatedChapterID = chapter.ID
		details.Chapters[candidateIndex] = candidate
		created = append(created, chapter)
	}
	applied, err := details.Job.MarkApplied(len(created), appliedAt)
	if err != nil {
		return ports.ImportApplyResult{}, err
	}
	details.Job = applied
	f.jobs[jobID] = details
	return ports.ImportApplyResult{Job: applied, Chapters: created}, nil
}

func mustWorkspaceImportJob(
	t *testing.T,
	id, projectID string,
	status importing.JobStatus,
) (importing.ImportJob, []importing.ImportChapter) {
	t.Helper()
	job, err := importing.NewImportJob(importing.ImportJobParams{
		ID:                   id,
		ProjectID:            projectID,
		SourceName:           "novel.txt",
		SourceFormat:         importing.SourceFormatTXT,
		SourceEncoding:       "utf-8",
		SourceHash:           strings.Repeat("c", 64),
		SourceBytes:          123,
		ArchivedRelativePath: "imports/" + id + ".txt",
		ChapterCount:         1,
	}, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := importing.NewImportChapter(importing.ImportChapterParams{
		ID:              "candidate-1",
		JobID:           id,
		Title:           "第一章",
		Body:            "正文一",
		Position:        1,
		SourceStartLine: 1,
		SourceEndLine:   2,
	})
	if err != nil {
		t.Fatal(err)
	}
	switch status {
	case importing.JobStatusApplied:
		job, err = job.MarkApplied(1, time.Unix(200, 0))
		if err != nil {
			t.Fatal(err)
		}
		candidate.Selected = true
		candidate.CreatedChapterID = "created-1"
	case importing.JobStatusFailed:
		job, err = job.MarkFailed(ImportErrorParseEmpty, "No importable chapters were found.", time.Unix(200, 0))
		if err != nil {
			t.Fatal(err)
		}
		job.ChapterCount = 0
		return job, []importing.ImportChapter{}
	}
	return job, []importing.ImportChapter{candidate}
}

var _ ports.ImportSourceArchive = (*fakeImportArchive)(nil)
var _ ports.ImportStore = (*fakeImportStore)(nil)
