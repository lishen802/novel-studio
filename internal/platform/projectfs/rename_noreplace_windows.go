//go:build windows

package projectfs

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type fileRenameInformation struct {
	ReplaceIfExists uint32
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

func renameImportArchiveNoReplace(
	directory *os.File,
	sourceName string,
	targetName string,
) error {
	directoryHandle := windows.Handle(directory.Fd())
	sourceHandle, err := openImportArchiveForRename(directoryHandle, sourceName)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(sourceHandle)

	targetUTF16, err := windows.UTF16FromString(targetName)
	if err != nil {
		return err
	}
	targetLength := (len(targetUTF16) - 1) * 2
	var header fileRenameInformation
	bufferSize := int(unsafe.Offsetof(header.FileName)) + targetLength
	buffer := make([]byte, bufferSize)
	information := (*fileRenameInformation)(unsafe.Pointer(&buffer[0]))
	information.RootDirectory = directoryHandle
	information.FileNameLength = uint32(targetLength)
	copy(
		unsafe.Slice(&information.FileName[0], targetLength/2),
		targetUTF16[:len(targetUTF16)-1],
	)

	var status windows.IO_STATUS_BLOCK
	err = windows.NtSetInformationFile(
		sourceHandle,
		&status,
		&buffer[0],
		uint32(bufferSize),
		windows.FileRenameInformation,
	)
	return normalizeWindowsRenameError(err)
}

func openImportArchiveForRename(
	directory windows.Handle,
	sourceName string,
) (windows.Handle, error) {
	name, err := windows.NewNTUnicodeString(sourceName)
	if err != nil {
		return 0, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: directory,
		ObjectName:    name,
		Attributes:    windows.OBJ_CASE_INSENSITIVE,
	}
	var (
		handle     windows.Handle
		status     windows.IO_STATUS_BLOCK
		allocation int64
	)
	err = windows.NtCreateFile(
		&handle,
		windows.DELETE|windows.SYNCHRONIZE,
		attributes,
		&status,
		&allocation,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_NON_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT,
		0,
		0,
	)
	if err != nil {
		return 0, normalizeWindowsRenameError(err)
	}
	return handle, nil
}

func normalizeWindowsRenameError(err error) error {
	if err == nil {
		return nil
	}
	var status windows.NTStatus
	if errors.As(err, &status) {
		err = status.Errno()
	}
	if errors.Is(err, syscall.ERROR_ALREADY_EXISTS) ||
		errors.Is(err, syscall.ERROR_FILE_EXISTS) {
		return fs.ErrExist
	}
	return err
}
