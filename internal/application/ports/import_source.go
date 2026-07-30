package ports

import (
	"context"

	"github.com/lishen802/novel-studio/internal/domain/importing"
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
