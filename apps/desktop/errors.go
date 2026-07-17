package main

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
)

type desktopErrorCode string

const (
	errorProjectNotFound   desktopErrorCode = "PROJECT_NOT_FOUND"
	errorPermissionDenied  desktopErrorCode = "PERMISSION_DENIED"
	errorProjectCorrupt    desktopErrorCode = "PROJECT_CORRUPT"
	errorProjectTooNew     desktopErrorCode = "PROJECT_TOO_NEW"
	errorRevisionConflict  desktopErrorCode = "REVISION_CONFLICT"
	errorWriteFailed       desktopErrorCode = "WRITE_FAILED"
	errorDatabaseBusy      desktopErrorCode = "DATABASE_BUSY"
	errorReferenceConflict desktopErrorCode = "REFERENCE_CONFLICT"
	errorValidationFailed  desktopErrorCode = "VALIDATION_FAILED"
	errorOperationFailed   desktopErrorCode = "OPERATION_FAILED"
)

type desktopError struct {
	Code    desktopErrorCode
	Message string
}

func (e *desktopError) Error() string {
	return fmt.Sprintf("%s|%s", e.Code, e.Message)
}

func safeDesktopError(err error) error {
	if err == nil {
		return nil
	}

	code := errorOperationFailed
	message := "操作失败，请重试。"
	lowerMessage := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, sqlite.ErrDatabaseMissing):
		code, message = errorProjectNotFound, "项目不存在或已被移动。"
	case errors.Is(err, fs.ErrPermission), errors.Is(err, sqlite.ErrDatabasePermission):
		code, message = errorPermissionDenied, "没有权限读取或写入该位置。"
	case errors.Is(err, sqlite.ErrDatabaseCorrupt), errors.Is(err, sqlite.ErrProjectCount), errors.Is(err, projectfs.ErrInvalidPackage):
		code, message = errorProjectCorrupt, "项目格式无效或数据已损坏。"
	case errors.Is(err, sqlite.ErrDatabaseTooNew):
		code, message = errorProjectTooNew, "该项目由更新版本创建，请升级应用后再打开。"
	case errors.Is(err, projectdomain.ErrRevisionConflict):
		code, message = errorRevisionConflict, "章节已在其他位置更新，本地文本已保留。"
	case errors.Is(err, worlddomain.ErrReferenced), errors.Is(err, storydomain.ErrReferenced), errors.Is(err, timelinedomain.ErrReferenced), errors.Is(err, canondomain.ErrReferenced):
		code, message = errorReferenceConflict, "该内容仍被其他故事资料引用，请先解除关联后再归档。"
	case errors.Is(err, worlddomain.ErrBlankValue):
		code, message = errorValidationFailed, "请填写完整的角色、势力或归属信息。"
	case errors.Is(err, worlddomain.ErrInvalidHierarchy), errors.Is(err, worlddomain.ErrInvalidReference), errors.Is(err, worlddomain.ErrInvalidStatus):
		code, message = errorValidationFailed, "故事资料包含无效状态、层级或关联，请检查后重试。"
	case errors.Is(err, timelinedomain.ErrBlankValue), errors.Is(err, timelinedomain.ErrInvalidOrder), errors.Is(err, timelinedomain.ErrInvalidStatus), errors.Is(err, timelinedomain.ErrInvalidReference),
		errors.Is(err, canondomain.ErrBlankValue), errors.Is(err, canondomain.ErrInvalidStatus), errors.Is(err, canondomain.ErrInvalidConfidence), errors.Is(err, canondomain.ErrInvalidEvidence), errors.Is(err, canondomain.ErrInvalidReference):
		code, message = errorValidationFailed, "时间线或事实资料包含无效顺序、状态、证据或关联，请检查后重试。"
	case errors.Is(err, sqlite.ErrDatabaseWrite), strings.Contains(lowerMessage, "no space left"), strings.Contains(lowerMessage, "disk full"):
		code, message = errorWriteFailed, "无法写入项目，请检查磁盘空间和目录权限。"
	case strings.Contains(lowerMessage, "database is locked"), strings.Contains(lowerMessage, "database table is locked"):
		code, message = errorDatabaseBusy, "项目正在被其他操作占用，请稍后重试。"
	}
	return &desktopError{Code: code, Message: message}
}

func safeResult[T any](value T, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, safeDesktopError(err)
	}
	return value, nil
}
