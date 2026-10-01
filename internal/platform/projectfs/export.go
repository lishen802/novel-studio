package projectfs

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteManuscript 创建唯一导出文件；写入失败时清理临时文件，不覆盖已有交付物。
func (m Manager) WriteManuscript(directory, extension, body string) (path string, err error) {
	if extension != "txt" && extension != "md" {
		return "", fmt.Errorf("unsupported manuscript extension")
	}
	file, err := os.CreateTemp(directory, "manuscript-*."+extension)
	if err != nil {
		return "", fmt.Errorf("create manuscript export: %w", err)
	}
	path = file.Name()
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil {
			err = fmt.Errorf("close manuscript export: %w", closeErr)
		}
		if err != nil {
			_ = os.Remove(path)
			path = ""
		}
	}()
	if _, err = file.WriteString(body); err != nil {
		return path, fmt.Errorf("write manuscript export: %w", err)
	}
	if err = file.Sync(); err != nil {
		return path, fmt.Errorf("sync manuscript export: %w", err)
	}
	return filepath.Clean(path), nil
}
