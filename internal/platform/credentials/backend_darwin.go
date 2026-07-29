//go:build darwin

package credentials

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type keychainBackend struct{}

func newPlatformBackend() backend {
	return keychainBackend{}
}

func (keychainBackend) save(ctx context.Context, reference, secret string) error {
	command := exec.CommandContext(
		ctx,
		"/usr/bin/security",
		"add-generic-password",
		"-U",
		"-a", reference,
		"-s", serviceName,
		"-w", secret,
	)
	if err := command.Run(); err != nil {
		return fmt.Errorf("%w: save macOS Keychain item", ErrCredentialStore)
	}
	return nil
}

func (keychainBackend) get(ctx context.Context, reference string) (string, error) {
	command := exec.CommandContext(
		ctx,
		"/usr/bin/security",
		"find-generic-password",
		"-a", reference,
		"-s", serviceName,
		"-w",
	)
	output, err := command.Output()
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 44 {
			return "", ErrCredentialNotFound
		}
		return "", fmt.Errorf("%w: read macOS Keychain item", ErrCredentialStore)
	}
	return strings.TrimSpace(string(output)), nil
}

func (keychainBackend) delete(ctx context.Context, reference string) error {
	command := exec.CommandContext(
		ctx,
		"/usr/bin/security",
		"delete-generic-password",
		"-a", reference,
		"-s", serviceName,
	)
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) && exitError.ExitCode() == 44 {
			return nil
		}
		return fmt.Errorf("%w: delete macOS Keychain item", ErrCredentialStore)
	}
	return nil
}
