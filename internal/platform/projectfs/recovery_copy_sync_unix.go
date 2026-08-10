//go:build darwin || linux

package projectfs

import "os"

func syncRecoveryCopyDirectory(directory string) error {
	handle, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer handle.Close()
	return handle.Sync()
}
