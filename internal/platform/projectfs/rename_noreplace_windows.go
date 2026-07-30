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

func openImportArchiveTemporary(
	importsDirectory *importArchiveDirectory,
	name string,
) (*os.File, error) {
	nameUTF16, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(importsDirectory.handle.Fd()),
		ObjectName:    nameUTF16,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	var (
		handle     windows.Handle
		status     windows.IO_STATUS_BLOCK
		allocation int64
	)
	err = windows.NtCreateFile(
		&handle,
		windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE|windows.DELETE|windows.SYNCHRONIZE,
		attributes,
		&status,
		&allocation,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_CREATE,
		windows.FILE_NON_DIRECTORY_FILE|
			windows.FILE_SYNCHRONOUS_IO_NONALERT|
			windows.FILE_OPEN_REPARSE_POINT,
		0,
		0,
	)
	if err != nil {
		return nil, normalizeWindowsRenameError(err)
	}
	file := os.NewFile(uintptr(handle), name)
	if file == nil {
		_ = windows.CloseHandle(handle)
		return nil, errors.New("create import archive file handle")
	}
	return file, nil
}

func commitOpenImportArchiveNoReplace(
	directory *os.File,
	temporary *os.File,
	_ string,
	targetName string,
) error {
	return renameOpenImportArchiveNoReplace(
		windows.Handle(directory.Fd()),
		windows.Handle(temporary.Fd()),
		targetName,
	)
}

func renameOpenImportArchiveNoReplace(
	directoryHandle windows.Handle,
	sourceHandle windows.Handle,
	targetName string,
) error {
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
	if errors.Is(err, windows.ERROR_NOT_SUPPORTED) ||
		errors.Is(err, windows.ERROR_INVALID_FUNCTION) {
		return errImportArchivePublicationUnsupported
	}
	return err
}
