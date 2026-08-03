package ports

import (
	"context"
	"errors"
	"time"

	"github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

var (
	ErrImportJobMissing             = errors.New("import job does not exist")
	ErrDuplicateImportSource        = errors.New("import source was already prepared for this project")
	ErrImportJobAlreadyApplied      = errors.New("import job was already applied")
	ErrImportJobNotPrepared         = errors.New("import job is not prepared")
	ErrNoImportChaptersSelected     = errors.New("at least one import chapter must be selected")
	ErrImportChapterMappingMismatch = errors.New("selected import chapters do not match chapters to create")
	ErrImportPersistence            = errors.New("import persistence operation failed")
)

type ImportJobDetails struct {
	Job      importing.ImportJob
	Chapters []importing.ImportChapter
}

type ImportApplyResult struct {
	Job      importing.ImportJob
	Chapters []projectdomain.Chapter
}

type ImportStore interface {
	// CreateImportJob persists the prepared job and every candidate in one
	// transaction. Candidates must be unselected and exactly match Job.
	CreateImportJob(
		ctx context.Context,
		job importing.ImportJob,
		chapters []importing.ImportChapter,
	) error
	// CreateFailedImportJob persists a terminal, zero-chapter parse failure.
	CreateFailedImportJob(ctx context.Context, job importing.ImportJob) error
	ListImportJobs(ctx context.Context, projectID string) ([]importing.ImportJob, error)
	ImportJob(ctx context.Context, id string) (ImportJobDetails, error)
	DeletePreparedImportJob(ctx context.Context, id string) error
	// ApplyImportJob maps selectedImportChapterIDs to chapters by slice index.
	// The store validates the mapping, then creates chapters in import candidate
	// order. A chapter Position may be zero; the store appends all created
	// chapters after the project's current maximum position and returns the
	// canonical positions. Creation/update times are canonicalized to appliedAt.
	ApplyImportJob(
		ctx context.Context,
		jobID string,
		selectedImportChapterIDs []string,
		chapters []projectdomain.Chapter,
		appliedAt time.Time,
	) (ImportApplyResult, error)
}
