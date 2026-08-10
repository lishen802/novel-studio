package sqlite

import (
	"context"
	"errors"

	"github.com/lishen802/novel-studio/internal/application/ports"
)

var errReadOnlyStoreNotImplemented = errors.New("read-only project store is not implemented")

// OpenReadOnly reserves the project-store contract for R1.3. A true SQLite
// read-only connection is deliberately implemented in the next task; failing
// closed prevents callers from accidentally receiving a writable store.
func (Factory) OpenReadOnly(context.Context, string) (ports.ProjectStore, error) {
	return nil, errReadOnlyStoreNotImplemented
}
