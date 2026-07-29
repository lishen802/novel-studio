//go:build windows

package credentials

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	credTypeGeneric  = 1
	credPersistLocal = 2
	errorNotFound    = syscall.Errno(1168)
)

var (
	advapi32       = windows.NewLazySystemDLL("advapi32.dll")
	procCredWrite  = advapi32.NewProc("CredWriteW")
	procCredRead   = advapi32.NewProc("CredReadW")
	procCredDelete = advapi32.NewProc("CredDeleteW")
	procCredFree   = advapi32.NewProc("CredFree")
)

type windowsCredential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

type credentialManagerBackend struct{}

func newPlatformBackend() backend {
	return credentialManagerBackend{}
}

func (credentialManagerBackend) save(_ context.Context, reference, secret string) error {
	target, err := windows.UTF16PtrFromString(serviceName + ":" + reference)
	if err != nil {
		return ErrInvalidCredential
	}
	username, _ := windows.UTF16PtrFromString(reference)
	blob := []byte(secret)
	credential := windowsCredential{
		Type:               credTypeGeneric,
		TargetName:         target,
		CredentialBlobSize: uint32(len(blob)),
		Persist:            credPersistLocal,
		UserName:           username,
	}
	if len(blob) > 0 {
		credential.CredentialBlob = &blob[0]
	}
	result, _, callErr := procCredWrite.Call(uintptr(unsafe.Pointer(&credential)), 0)
	if result == 0 {
		return fmt.Errorf("%w: write Windows Credential Manager item: %v", ErrCredentialStore, callErr)
	}
	return nil
}

func (credentialManagerBackend) get(_ context.Context, reference string) (string, error) {
	target, err := windows.UTF16PtrFromString(serviceName + ":" + reference)
	if err != nil {
		return "", ErrInvalidCredential
	}
	var credential *windowsCredential
	result, _, callErr := procCredRead.Call(
		uintptr(unsafe.Pointer(target)),
		credTypeGeneric,
		0,
		uintptr(unsafe.Pointer(&credential)),
	)
	if result == 0 {
		if errors.Is(callErr, errorNotFound) {
			return "", ErrCredentialNotFound
		}
		return "", fmt.Errorf("%w: read Windows Credential Manager item: %v", ErrCredentialStore, callErr)
	}
	defer procCredFree.Call(uintptr(unsafe.Pointer(credential)))
	if credential.CredentialBlobSize == 0 {
		return "", nil
	}
	blob := unsafe.Slice(credential.CredentialBlob, credential.CredentialBlobSize)
	return string(blob), nil
}

func (credentialManagerBackend) delete(_ context.Context, reference string) error {
	target, err := windows.UTF16PtrFromString(serviceName + ":" + reference)
	if err != nil {
		return ErrInvalidCredential
	}
	result, _, callErr := procCredDelete.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0)
	if result == 0 && !errors.Is(callErr, errorNotFound) {
		return fmt.Errorf("%w: delete Windows Credential Manager item: %v", ErrCredentialStore, callErr)
	}
	return nil
}
