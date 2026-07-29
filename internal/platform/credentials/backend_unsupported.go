//go:build !darwin && !windows

package credentials

import (
	"context"
	"fmt"
)

type unsupportedBackend struct{}

func newPlatformBackend() backend {
	return unsupportedBackend{}
}

func (unsupportedBackend) save(context.Context, string, string) error {
	return fmt.Errorf("%w: unsupported operating system", ErrCredentialStore)
}

func (unsupportedBackend) get(context.Context, string) (string, error) {
	return "", fmt.Errorf("%w: unsupported operating system", ErrCredentialStore)
}

func (unsupportedBackend) delete(context.Context, string) error {
	return fmt.Errorf("%w: unsupported operating system", ErrCredentialStore)
}
