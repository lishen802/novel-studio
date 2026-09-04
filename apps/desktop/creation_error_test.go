package main

import (
	"errors"
	"fmt"
	"testing"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestCreationSessionConflictHasSafeDesktopMessage(t *testing.T) {
	err := safeDesktopError(fmt.Errorf("private storage details: %w", creationdomain.ErrSessionVersionConflict))
	var safe *desktopError
	if !errors.As(err, &safe) || safe.Code != errorCreationVersionConflict || safe.Message != "创作会话已更新，本次修改未覆盖已有内容。请保留当前输入，重新读取并比较后再保存。" {
		t.Fatalf("版本冲突未安全映射: %v", err)
	}
}
