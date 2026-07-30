package projectfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var (
	ErrBlankProjectTitle = errors.New("project title must not be blank")
	ErrInvalidPackage    = errors.New("invalid project package")
)

const packageExtension = ".zjproject"

var windowsReservedNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

type Layout struct {
	Root        string
	Database    string
	Attachments string
	Exports     string
	Imports     string
	Snapshots   string
	Branches    string
}

func NewLayout(parentDirectory, projectTitle string) (Layout, error) {
	name, err := portableProjectName(projectTitle)
	if err != nil {
		return Layout{}, err
	}
	parentDirectory = strings.TrimSpace(parentDirectory)
	if parentDirectory == "" {
		return Layout{}, fmt.Errorf("%w: parent directory is blank", ErrInvalidPackage)
	}

	return layoutForRoot(filepath.Join(filepath.Clean(parentDirectory), name+packageExtension)), nil
}

func (l Layout) Create() error {
	if err := os.Mkdir(l.Root, 0o755); err != nil {
		return fmt.Errorf("create project package: %w", err)
	}

	for _, directory := range []string{l.Attachments, l.Exports, l.Imports, l.Snapshots, l.Branches} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return fmt.Errorf("create project directory: %w", err)
		}
	}
	return nil
}

func OpenLayout(packagePath string) (Layout, error) {
	packagePath = filepath.Clean(strings.TrimSpace(packagePath))
	if !strings.EqualFold(filepath.Ext(packagePath), packageExtension) {
		return Layout{}, fmt.Errorf("%w: package must end in %s", ErrInvalidPackage, packageExtension)
	}

	info, err := os.Stat(packagePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Layout{}, fmt.Errorf("%w: %w", ErrInvalidPackage, err)
		}
		return Layout{}, fmt.Errorf("inspect project package: %w", err)
	}
	if !info.IsDir() {
		return Layout{}, fmt.Errorf("%w: package is not a directory", ErrInvalidPackage)
	}

	layout := layoutForRoot(packagePath)
	if err := os.MkdirAll(layout.Imports, 0o755); err != nil {
		return Layout{}, fmt.Errorf("create imports directory: %w", err)
	}
	if err := cleanupOrphanImportTemporaryFilesForLayout(layout); err != nil {
		return Layout{}, err
	}
	return layout, nil
}

func layoutForRoot(root string) Layout {
	return Layout{
		Root:        root,
		Database:    filepath.Join(root, "project.db"),
		Attachments: filepath.Join(root, "attachments"),
		Exports:     filepath.Join(root, "exports"),
		Imports:     filepath.Join(root, "imports"),
		Snapshots:   filepath.Join(root, "snapshots"),
		Branches:    filepath.Join(root, "branches"),
	}
}

func portableProjectName(title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", ErrBlankProjectTitle
	}

	var builder strings.Builder
	for _, r := range title {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			builder.WriteRune('-')
			continue
		}
		if unicode.IsSpace(r) {
			builder.WriteRune(' ')
			continue
		}
		builder.WriteRune(r)
	}

	name := strings.Join(strings.Fields(builder.String()), " ")
	name = strings.Trim(name, " .-")
	if name == "" {
		return "", ErrBlankProjectTitle
	}

	reservedBase := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	if _, reserved := windowsReservedNames[reservedBase]; reserved {
		name += "-project"
	}
	return name, nil
}
