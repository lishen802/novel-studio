package importing

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestImportJobParamsDoesNotExposeStatus(t *testing.T) {
	if _, exists := reflect.TypeOf(ImportJobParams{}).FieldByName("Status"); exists {
		t.Fatal("ImportJobParams exposes Status; NewImportJob must always create a prepared job")
	}
}

func TestNewImportJobCreatesPreparedJob(t *testing.T) {
	now := time.Date(2026, time.July, 30, 8, 9, 10, 0, time.FixedZone("CST", 8*60*60))
	hash := strings.Repeat("A1", 32)

	job, err := NewImportJob(ImportJobParams{
		ID:                   " job-1 ",
		ProjectID:            " project-1 ",
		SourceName:           " novel.txt ",
		SourceFormat:         SourceFormatTXT,
		SourceEncoding:       " UTF-8 ",
		SourceHash:           hash,
		SourceBytes:          2048,
		ArchivedRelativePath: "imports/job-1.txt",
		ChapterCount:         3,
	}, now)
	if err != nil {
		t.Fatalf("NewImportJob returned error: %v", err)
	}

	wantTime := now.UTC()
	if job.ID != "job-1" || job.ProjectID != "project-1" || job.SourceName != "novel.txt" {
		t.Fatalf("identifiers were not normalized: %#v", job)
	}
	if job.SourceFormat != SourceFormatTXT || job.SourceEncoding != "UTF-8" {
		t.Fatalf("source metadata mismatch: %#v", job)
	}
	if job.SourceHash != strings.ToLower(hash) {
		t.Fatalf("SourceHash = %q, want lowercase hash", job.SourceHash)
	}
	if job.SourceBytes != 2048 || job.ArchivedRelativePath != "imports/job-1.txt" {
		t.Fatalf("source archive metadata mismatch: %#v", job)
	}
	if job.Status != JobStatusPrepared || job.ChapterCount != 3 || job.SelectedChapterCount != 0 {
		t.Fatalf("prepared job state mismatch: %#v", job)
	}
	if job.CreatedAt != wantTime || job.UpdatedAt != wantTime || job.AppliedAt != nil {
		t.Fatalf("job timestamps mismatch: %#v", job)
	}
}

func TestNewImportJobRejectsInvalidValues(t *testing.T) {
	valid := ImportJobParams{
		ID:                   "job-1",
		ProjectID:            "project-1",
		SourceName:           "novel.md",
		SourceFormat:         SourceFormatMarkdown,
		SourceEncoding:       "UTF-8",
		SourceHash:           strings.Repeat("ab", 32),
		SourceBytes:          20,
		ArchivedRelativePath: "imports/job-1.md",
		ChapterCount:         2,
	}

	tests := []struct {
		name   string
		change func(*ImportJobParams)
		want   error
	}{
		{name: "blank id", change: func(p *ImportJobParams) { p.ID = " " }, want: ErrBlankValue},
		{name: "blank project id", change: func(p *ImportJobParams) { p.ProjectID = "" }, want: ErrBlankValue},
		{name: "blank source name", change: func(p *ImportJobParams) { p.SourceName = "\t" }, want: ErrBlankValue},
		{name: "invalid format", change: func(p *ImportJobParams) { p.SourceFormat = "docx" }, want: ErrInvalidSourceFormat},
		{name: "short hash", change: func(p *ImportJobParams) { p.SourceHash = "abcd" }, want: ErrInvalidSourceHash},
		{name: "non hexadecimal hash", change: func(p *ImportJobParams) { p.SourceHash = strings.Repeat("zz", 32) }, want: ErrInvalidSourceHash},
		{name: "zero bytes", change: func(p *ImportJobParams) { p.SourceBytes = 0 }, want: ErrInvalidSourceBytes},
		{name: "negative bytes", change: func(p *ImportJobParams) { p.SourceBytes = -1 }, want: ErrInvalidSourceBytes},
		{name: "zero chapter count", change: func(p *ImportJobParams) { p.ChapterCount = 0 }, want: ErrInvalidChapterCount},
		{name: "negative chapter count", change: func(p *ImportJobParams) { p.ChapterCount = -1 }, want: ErrInvalidChapterCount},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := valid
			test.change(&params)
			_, err := NewImportJob(params, time.Now())
			if !errors.Is(err, test.want) {
				t.Fatalf("NewImportJob error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestNewFailedImportJobCreatesZeroChapterTerminalJob(t *testing.T) {
	now := time.Date(2026, time.August, 3, 9, 10, 11, 12, time.FixedZone("CST", 8*60*60))
	job, err := NewFailedImportJob(FailedImportJobParams{
		ID:                   " failed-job ",
		ProjectID:            " project-1 ",
		SourceName:           " broken.txt ",
		SourceFormat:         SourceFormatTXT,
		SourceEncoding:       " UTF-8 ",
		SourceHash:           strings.Repeat("AB", 32),
		SourceBytes:          12,
		ArchivedRelativePath: " imports/failed-job.txt ",
		ErrorCode:            " parse_empty ",
		ErrorMessage:         " manuscript contains no chapters ",
	}, now)
	if err != nil {
		t.Fatalf("NewFailedImportJob returned error: %v", err)
	}
	if job.ID != "failed-job" || job.ProjectID != "project-1" ||
		job.SourceHash != strings.Repeat("ab", 32) {
		t.Fatalf("failed job metadata = %#v", job)
	}
	if job.Status != JobStatusFailed || job.ChapterCount != 0 ||
		job.SelectedChapterCount != 0 || job.ErrorCode != "parse_empty" ||
		job.ErrorMessage != "manuscript contains no chapters" {
		t.Fatalf("failed job state = %#v", job)
	}
	if job.CreatedAt != now.UTC() || job.UpdatedAt != now.UTC() || job.AppliedAt != nil {
		t.Fatalf("failed job timestamps = %#v", job)
	}
}

func TestNewFailedImportJobRejectsBlankFailureSummary(t *testing.T) {
	valid := FailedImportJobParams{
		ID:                   "failed-job",
		ProjectID:            "project-1",
		SourceName:           "broken.txt",
		SourceFormat:         SourceFormatTXT,
		SourceEncoding:       "UTF-8",
		SourceHash:           strings.Repeat("ab", 32),
		SourceBytes:          12,
		ArchivedRelativePath: "imports/failed-job.txt",
		ErrorCode:            "parse_empty",
		ErrorMessage:         "no chapters",
	}
	for name, change := range map[string]func(*FailedImportJobParams){
		"blank code":    func(params *FailedImportJobParams) { params.ErrorCode = " " },
		"blank message": func(params *FailedImportJobParams) { params.ErrorMessage = "" },
	} {
		t.Run(name, func(t *testing.T) {
			params := valid
			change(&params)
			if _, err := NewFailedImportJob(params, time.Now()); !errors.Is(err, ErrBlankValue) {
				t.Fatalf("error = %v, want ErrBlankValue", err)
			}
		})
	}
}

func TestImportJobPreparedTransitionsToApplied(t *testing.T) {
	job := newValidImportJob(t)
	now := time.Date(2026, time.July, 30, 11, 30, 0, 0, time.FixedZone("CST", 8*60*60))

	applied, err := job.MarkApplied(2, now)
	if err != nil {
		t.Fatalf("MarkApplied returned error: %v", err)
	}
	if applied.Status != JobStatusApplied || applied.SelectedChapterCount != 2 {
		t.Fatalf("applied state mismatch: %#v", applied)
	}
	if applied.AppliedAt == nil || *applied.AppliedAt != now.UTC() || applied.UpdatedAt != now.UTC() {
		t.Fatalf("applied timestamps mismatch: %#v", applied)
	}
	if applied.ErrorCode != "" || applied.ErrorMessage != "" {
		t.Fatalf("applied job retained error details: %#v", applied)
	}

	for _, transition := range []func() error{
		func() error {
			_, err := applied.MarkApplied(1, now.Add(time.Minute))
			return err
		},
		func() error {
			_, err := applied.MarkFailed("late", "too late", now.Add(time.Minute))
			return err
		},
	} {
		if err := transition(); !errors.Is(err, ErrInvalidJobTransition) {
			t.Fatalf("terminal transition error = %v, want %v", err, ErrInvalidJobTransition)
		}
	}
}

func TestImportJobPreparedTransitionsToFailed(t *testing.T) {
	job := newValidImportJob(t)
	now := time.Date(2026, time.July, 30, 4, 0, 0, 0, time.UTC)

	failed, err := job.MarkFailed(" parse_empty ", " manuscript has no content ", now)
	if err != nil {
		t.Fatalf("MarkFailed returned error: %v", err)
	}
	if failed.Status != JobStatusFailed {
		t.Fatalf("Status = %q, want %q", failed.Status, JobStatusFailed)
	}
	if failed.ErrorCode != "parse_empty" || failed.ErrorMessage != "manuscript has no content" {
		t.Fatalf("failure details mismatch: %#v", failed)
	}
	if failed.AppliedAt != nil || failed.UpdatedAt != now {
		t.Fatalf("failed timestamps mismatch: %#v", failed)
	}
	if _, err := failed.MarkFailed("again", "again", now); !errors.Is(err, ErrInvalidJobTransition) {
		t.Fatalf("terminal MarkFailed error = %v, want %v", err, ErrInvalidJobTransition)
	}
}

func TestImportJobRejectsInvalidTransitionData(t *testing.T) {
	job := newValidImportJob(t)
	now := time.Now()

	for _, selected := range []int{0, job.ChapterCount + 1} {
		if _, err := job.MarkApplied(selected, now); !errors.Is(err, ErrInvalidChapterCount) {
			t.Fatalf("MarkApplied(%d) error = %v, want %v", selected, err, ErrInvalidChapterCount)
		}
	}
	if _, err := job.MarkFailed("", "message", now); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("blank error code error = %v, want %v", err, ErrBlankValue)
	}
	if _, err := job.MarkFailed("code", " ", now); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("blank error message error = %v, want %v", err, ErrBlankValue)
	}
}

func TestNewImportChapterCapturesReviewAndSourceMetadata(t *testing.T) {
	chapter, err := NewImportChapter(ImportChapterParams{
		ID:               " import-chapter-1 ",
		JobID:            " job-1 ",
		VolumeTitle:      " 第一卷 风起 ",
		Title:            " 第一章 初见 ",
		Body:             "正文",
		Position:         1,
		SourceStartLine:  3,
		SourceEndLine:    8,
		Selected:         true,
		CreatedChapterID: " chapter-9 ",
	})
	if err != nil {
		t.Fatalf("NewImportChapter returned error: %v", err)
	}
	if chapter.ID != "import-chapter-1" || chapter.JobID != "job-1" {
		t.Fatalf("chapter identifiers mismatch: %#v", chapter)
	}
	if chapter.VolumeTitle != "第一卷 风起" || chapter.Title != "第一章 初见" {
		t.Fatalf("chapter labels mismatch: %#v", chapter)
	}
	if chapter.Body != "正文" || chapter.Position != 1 {
		t.Fatalf("chapter content mismatch: %#v", chapter)
	}
	if chapter.SourceStartLine != 3 || chapter.SourceEndLine != 8 || chapter.ContentHash != contentHash(chapter.Body) {
		t.Fatalf("chapter source metadata mismatch: %#v", chapter)
	}
	if !chapter.Selected || chapter.CreatedChapterID != "chapter-9" {
		t.Fatalf("chapter review metadata mismatch: %#v", chapter)
	}
}

func TestImportChapterParamsDoesNotExposeContentHash(t *testing.T) {
	if _, exists := reflect.TypeOf(ImportChapterParams{}).FieldByName("ContentHash"); exists {
		t.Fatal("ImportChapterParams exposes ContentHash; NewImportChapter must derive it from Body")
	}
}

func TestNewImportChapterDerivesContentHashFromBody(t *testing.T) {
	chapter, err := NewImportChapter(ImportChapterParams{
		ID:              "import-chapter-1",
		JobID:           "job-1",
		Title:           "第一章",
		Body:            "不可伪造的正文",
		Position:        1,
		SourceStartLine: 2,
		SourceEndLine:   2,
	})
	if err != nil {
		t.Fatalf("NewImportChapter returned error: %v", err)
	}
	if chapter.ContentHash != contentHash(chapter.Body) {
		t.Fatalf("ContentHash = %q, want SHA-256 derived from Body", chapter.ContentHash)
	}
}

func TestNewImportChapterRejectsInvalidValues(t *testing.T) {
	valid := ImportChapterParams{
		ID:              "import-chapter-1",
		JobID:           "job-1",
		Title:           "第一章",
		Body:            "正文",
		Position:        1,
		SourceStartLine: 2,
		SourceEndLine:   2,
	}
	tests := []struct {
		name   string
		change func(*ImportChapterParams)
		want   error
	}{
		{name: "blank id", change: func(p *ImportChapterParams) { p.ID = "" }, want: ErrBlankValue},
		{name: "blank job id", change: func(p *ImportChapterParams) { p.JobID = " " }, want: ErrBlankValue},
		{name: "blank title", change: func(p *ImportChapterParams) { p.Title = "" }, want: ErrBlankValue},
		{name: "zero position", change: func(p *ImportChapterParams) { p.Position = 0 }, want: ErrInvalidImportChapter},
		{name: "zero start line", change: func(p *ImportChapterParams) { p.SourceStartLine = 0 }, want: ErrInvalidImportChapter},
		{name: "end before start", change: func(p *ImportChapterParams) { p.SourceEndLine = 1 }, want: ErrInvalidImportChapter},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := valid
			test.change(&params)
			_, err := NewImportChapter(params)
			if !errors.Is(err, test.want) {
				t.Fatalf("NewImportChapter error = %v, want %v", err, test.want)
			}
		})
	}
}

func newValidImportJob(t *testing.T) ImportJob {
	t.Helper()
	now := time.Date(2026, time.July, 30, 1, 0, 0, 0, time.UTC)
	job, err := NewImportJob(ImportJobParams{
		ID:                   "job-1",
		ProjectID:            "project-1",
		SourceName:           "novel.txt",
		SourceFormat:         SourceFormatTXT,
		SourceEncoding:       "UTF-8",
		SourceHash:           strings.Repeat("ab", 32),
		SourceBytes:          1024,
		ArchivedRelativePath: "imports/job-1.txt",
		ChapterCount:         3,
	}, now)
	if err != nil {
		t.Fatalf("NewImportJob returned error: %v", err)
	}
	return job
}
