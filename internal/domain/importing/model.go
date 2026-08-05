package importing

import (
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankValue           = errors.New("required import value must not be blank")
	ErrInvalidSourceFormat  = errors.New("invalid import source format")
	ErrInvalidSourceHash    = errors.New("invalid SHA-256 hash")
	ErrInvalidSourceBytes   = errors.New("import source bytes must be positive")
	ErrInvalidChapterCount  = errors.New("invalid import chapter count")
	ErrInvalidJobTransition = errors.New("invalid import job status transition")
	ErrInvalidImportChapter = errors.New("invalid import chapter")
)

type SourceFormat string

const (
	SourceFormatTXT      SourceFormat = "txt"
	SourceFormatMarkdown SourceFormat = "markdown"
)

type JobStatus string

const (
	JobStatusPrepared JobStatus = "prepared"
	JobStatusApplied  JobStatus = "applied"
	JobStatusFailed   JobStatus = "failed"
)

// ExtractionStatus tracks whether the applied ImportJob has been processed by
// an AI extraction task. It only advances after Status reaches JobStatusApplied.
type ExtractionStatus string

const (
	ExtractionStatusNone      ExtractionStatus = "none"
	ExtractionStatusRunning   ExtractionStatus = "running"
	ExtractionStatusCompleted ExtractionStatus = "completed"
	ExtractionStatusFailed    ExtractionStatus = "failed"
)

type ImportJob struct {
	ID                   string
	ProjectID            string
	SourceName           string
	SourceFormat         SourceFormat
	SourceEncoding       string
	SourceHash           string
	SourceBytes          int64
	ArchivedRelativePath string
	Status               JobStatus
	ChapterCount         int
	SelectedChapterCount int
	ErrorCode            string
	ErrorMessage         string
	ExtractionStatus     ExtractionStatus
	ExtractionRunID      string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	AppliedAt            *time.Time
}

type ImportJobParams struct {
	ID                   string
	ProjectID            string
	SourceName           string
	SourceFormat         SourceFormat
	SourceEncoding       string
	SourceHash           string
	SourceBytes          int64
	ArchivedRelativePath string
	ChapterCount         int
}

type FailedImportJobParams struct {
	ID                   string
	ProjectID            string
	SourceName           string
	SourceFormat         SourceFormat
	SourceEncoding       string
	SourceHash           string
	SourceBytes          int64
	ArchivedRelativePath string
	ErrorCode            string
	ErrorMessage         string
}

func NewImportJob(params ImportJobParams, now time.Time) (ImportJob, error) {
	params.ID = strings.TrimSpace(params.ID)
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.SourceName = strings.TrimSpace(params.SourceName)
	params.SourceEncoding = strings.TrimSpace(params.SourceEncoding)
	params.SourceHash = strings.ToLower(strings.TrimSpace(params.SourceHash))
	params.ArchivedRelativePath = strings.TrimSpace(params.ArchivedRelativePath)

	if params.ID == "" || params.ProjectID == "" || params.SourceName == "" ||
		params.SourceEncoding == "" || params.ArchivedRelativePath == "" {
		return ImportJob{}, ErrBlankValue
	}
	if !validSourceFormat(params.SourceFormat) {
		return ImportJob{}, ErrInvalidSourceFormat
	}
	if !validSHA256(params.SourceHash) {
		return ImportJob{}, ErrInvalidSourceHash
	}
	if params.SourceBytes < 1 {
		return ImportJob{}, ErrInvalidSourceBytes
	}
	if params.ChapterCount < 1 {
		return ImportJob{}, ErrInvalidChapterCount
	}

	now = now.UTC()
	return ImportJob{
		ID:                   params.ID,
		ProjectID:            params.ProjectID,
		SourceName:           params.SourceName,
		SourceFormat:         params.SourceFormat,
		SourceEncoding:       params.SourceEncoding,
		SourceHash:           params.SourceHash,
		SourceBytes:          params.SourceBytes,
		ArchivedRelativePath: params.ArchivedRelativePath,
		Status:               JobStatusPrepared,
		ChapterCount:         params.ChapterCount,
		ExtractionStatus:     ExtractionStatusNone,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

func NewFailedImportJob(params FailedImportJobParams, now time.Time) (ImportJob, error) {
	params.ID = strings.TrimSpace(params.ID)
	params.ProjectID = strings.TrimSpace(params.ProjectID)
	params.SourceName = strings.TrimSpace(params.SourceName)
	params.SourceEncoding = strings.TrimSpace(params.SourceEncoding)
	params.SourceHash = strings.ToLower(strings.TrimSpace(params.SourceHash))
	params.ArchivedRelativePath = strings.TrimSpace(params.ArchivedRelativePath)
	params.ErrorCode = strings.TrimSpace(params.ErrorCode)
	params.ErrorMessage = strings.TrimSpace(params.ErrorMessage)

	if params.ID == "" || params.ProjectID == "" || params.SourceName == "" ||
		params.SourceEncoding == "" || params.ArchivedRelativePath == "" ||
		params.ErrorCode == "" || params.ErrorMessage == "" {
		return ImportJob{}, ErrBlankValue
	}
	if !validSourceFormat(params.SourceFormat) {
		return ImportJob{}, ErrInvalidSourceFormat
	}
	if !validSHA256(params.SourceHash) {
		return ImportJob{}, ErrInvalidSourceHash
	}
	if params.SourceBytes < 1 {
		return ImportJob{}, ErrInvalidSourceBytes
	}

	now = now.UTC()
	return ImportJob{
		ID:                   params.ID,
		ProjectID:            params.ProjectID,
		SourceName:           params.SourceName,
		SourceFormat:         params.SourceFormat,
		SourceEncoding:       params.SourceEncoding,
		SourceHash:           params.SourceHash,
		SourceBytes:          params.SourceBytes,
		ArchivedRelativePath: params.ArchivedRelativePath,
		Status:               JobStatusFailed,
		ErrorCode:            params.ErrorCode,
		ErrorMessage:         params.ErrorMessage,
		ExtractionStatus:     ExtractionStatusNone,
		CreatedAt:            now,
		UpdatedAt:            now,
	}, nil
}

func (job ImportJob) MarkApplied(selectedChapterCount int, now time.Time) (ImportJob, error) {
	if job.Status != JobStatusPrepared {
		return ImportJob{}, ErrInvalidJobTransition
	}
	if selectedChapterCount < 1 || selectedChapterCount > job.ChapterCount {
		return ImportJob{}, ErrInvalidChapterCount
	}

	now = now.UTC()
	job.Status = JobStatusApplied
	job.SelectedChapterCount = selectedChapterCount
	job.ErrorCode = ""
	job.ErrorMessage = ""
	job.UpdatedAt = now
	job.AppliedAt = &now
	return job, nil
}

func (job ImportJob) MarkFailed(errorCode, errorMessage string, now time.Time) (ImportJob, error) {
	if job.Status != JobStatusPrepared {
		return ImportJob{}, ErrInvalidJobTransition
	}
	errorCode = strings.TrimSpace(errorCode)
	errorMessage = strings.TrimSpace(errorMessage)
	if errorCode == "" || errorMessage == "" {
		return ImportJob{}, ErrBlankValue
	}

	job.Status = JobStatusFailed
	job.ErrorCode = errorCode
	job.ErrorMessage = errorMessage
	job.UpdatedAt = now.UTC()
	job.AppliedAt = nil
	return job, nil
}

// MarkExtractionStarted records that an extraction task is running for this
// applied ImportJob. The prior extraction status must be none or failed so
// that authors can retry a failed extraction without first clearing it.
func (job ImportJob) MarkExtractionStarted(runID string, now time.Time) (ImportJob, error) {
	if job.Status != JobStatusApplied {
		return ImportJob{}, ErrInvalidJobTransition
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return ImportJob{}, ErrBlankValue
	}
	if job.ExtractionStatus != ExtractionStatusNone && job.ExtractionStatus != ExtractionStatusFailed {
		return ImportJob{}, ErrInvalidJobTransition
	}

	job.ExtractionStatus = ExtractionStatusRunning
	job.ExtractionRunID = runID
	job.UpdatedAt = now.UTC()
	return job, nil
}

// MarkExtractionCompleted records that extraction has produced a ChangeSet
// for authors to review.
func (job ImportJob) MarkExtractionCompleted(now time.Time) (ImportJob, error) {
	if job.Status != JobStatusApplied || job.ExtractionStatus != ExtractionStatusRunning {
		return ImportJob{}, ErrInvalidJobTransition
	}
	job.ExtractionStatus = ExtractionStatusCompleted
	job.UpdatedAt = now.UTC()
	return job, nil
}

// MarkExtractionFailed records that extraction failed for the applied ImportJob
// so that authors can retry from the workbench. The detailed error message
// lives on the bound ExtractionTask; the ImportJob only tracks the lifecycle
// so the workbench can show "retry extraction".
func (job ImportJob) MarkExtractionFailed(now time.Time) (ImportJob, error) {
	if job.Status != JobStatusApplied || job.ExtractionStatus != ExtractionStatusRunning {
		return ImportJob{}, ErrInvalidJobTransition
	}
	job.ExtractionStatus = ExtractionStatusFailed
	job.UpdatedAt = now.UTC()
	return job, nil
}

type ImportChapter struct {
	ID               string
	JobID            string
	VolumeTitle      string
	Title            string
	Body             string
	Position         int
	SourceStartLine  int
	SourceEndLine    int
	ContentHash      string
	Selected         bool
	CreatedChapterID string
}

type ImportChapterParams struct {
	ID               string
	JobID            string
	VolumeTitle      string
	Title            string
	Body             string
	Position         int
	SourceStartLine  int
	SourceEndLine    int
	Selected         bool
	CreatedChapterID string
}

func NewImportChapter(params ImportChapterParams) (ImportChapter, error) {
	params.ID = strings.TrimSpace(params.ID)
	params.JobID = strings.TrimSpace(params.JobID)
	params.VolumeTitle = strings.TrimSpace(params.VolumeTitle)
	params.Title = strings.TrimSpace(params.Title)
	params.CreatedChapterID = strings.TrimSpace(params.CreatedChapterID)

	if params.ID == "" || params.JobID == "" || params.Title == "" {
		return ImportChapter{}, ErrBlankValue
	}
	if params.Position < 1 || params.SourceStartLine < 1 || params.SourceEndLine < params.SourceStartLine {
		return ImportChapter{}, ErrInvalidImportChapter
	}

	return ImportChapter{
		ID:               params.ID,
		JobID:            params.JobID,
		VolumeTitle:      params.VolumeTitle,
		Title:            params.Title,
		Body:             params.Body,
		Position:         params.Position,
		SourceStartLine:  params.SourceStartLine,
		SourceEndLine:    params.SourceEndLine,
		ContentHash:      hashContent(params.Body),
		Selected:         params.Selected,
		CreatedChapterID: params.CreatedChapterID,
	}, nil
}

func validSourceFormat(format SourceFormat) bool {
	return format == SourceFormatTXT || format == SourceFormatMarkdown
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32
}
