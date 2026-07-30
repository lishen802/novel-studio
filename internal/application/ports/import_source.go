package ports

import (
	"context"
	"errors"

	"github.com/lishen802/novel-studio/internal/domain/importing"
)

var (
	ErrUnsupportedImportFormat   = errors.New("unsupported import source format")
	ErrEmptyImportSource         = errors.New("import source is empty")
	ErrImportSourceTooLarge      = errors.New("import source is too large")
	ErrUnsupportedImportEncoding = errors.New("unsupported import source encoding")
	ErrUnsafeImportSource        = errors.New("unsafe import source")
	ErrImportArchiveExists       = errors.New("import source archive already exists")
	ErrImportArchiveUnsupported  = errors.New("atomic import source publication is unsupported")
	ErrInvalidImportArchivePath  = errors.New("invalid import archive path")
)

type PreparedImportSource struct {
	Name                 string
	Format               importing.SourceFormat
	Encoding             string
	SHA256               string
	Bytes                int64
	ArchivedRelativePath string
	Text                 string
}

type ImportSourceArchive interface {
	Prepare(
		ctx context.Context,
		projectPackage ProjectPackage,
		jobID string,
		sourcePath string,
	) (PreparedImportSource, error)
	Delete(ctx context.Context, projectPackage ProjectPackage, relativePath string) error
}
