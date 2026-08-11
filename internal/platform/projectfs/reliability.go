package projectfs

import (
	"errors"
	"path/filepath"
	"unicode/utf16"

	"github.com/lishen802/novel-studio/internal/application/ports"
)

const windowsCompatiblePathLimit = 240

const longestProjectRelativePath = "recovery/.r1-4-probe-0123456789abcdef/replace-target-0123456789abcdef.sqlite"

func validateProjectPath(root string) error {
	absolute, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return err
	}
	if len(utf16.Encode([]rune(filepath.Join(absolute, longestProjectRelativePath)))) > windowsCompatiblePathLimit {
		return &ports.ProjectFilesystemFailure{Category: ports.ProjectFilesystemPathTooLong}
	}
	return nil
}

func (Manager) ValidateCreatePath(parentDirectory, projectTitle string) error {
	name, err := portableProjectName(projectTitle)
	if err != nil {
		return err
	}
	return validateProjectPath(filepath.Join(parentDirectory, name+packageExtension))
}

func (manager Manager) ValidateRecoveryCopyPath(source ports.ProjectPackage, parentDirectory string) error {
	layout, err := manager.newRecoveryCopyLayout(source, parentDirectory)
	if err != nil {
		return err
	}
	if err := validateProjectPath(layout.temporary); err != nil {
		return err
	}
	return validateProjectPath(layout.final)
}

func validateRecoveryCopyLayout(layout recoveryCopyLayout) error {
	if err := validateProjectPath(layout.temporary); err != nil {
		return err
	}
	return validateProjectPath(layout.final)
}

func (Manager) ClassifyRuntimeError(err error) *ports.ProjectFilesystemFailure {
	var failure *ports.ProjectFilesystemFailure
	if errors.As(err, &failure) {
		return failure
	}
	return nil
}
