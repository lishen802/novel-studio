package appdirs

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds the resolved application data directory locations. All paths
// are computed without touching the filesystem.
type Paths struct {
	Database string
}

// Resolve computes the application settings database path from a base
// directory resolver. It does not create any directories or files.
func Resolve(baseDirFn func() (string, error)) (Paths, error) {
	base, err := baseDirFn()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve application data directory: %w", err)
	}
	return Paths{
		Database: filepath.Join(base, "StoryStudio", "settings.db"),
	}, nil
}

// DefaultBaseDir returns the OS-specific application data directory. On macOS
// it uses ~/Library/Application Support; on Windows it uses %APPDATA%; on
// other platforms it falls back to XDG_DATA_HOME or ~/.local/share.
func DefaultBaseDir() (string, error) {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share"), nil
}

// ResolveDefault is the production entry point that resolves paths using the
// OS-specific application data directory.
func ResolveDefault() (Paths, error) {
	return Resolve(DefaultBaseDir)
}
