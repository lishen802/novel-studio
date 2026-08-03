package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/application/workspace"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	importdomain "github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
)

type desktopErrorCode string

const (
	errorProjectNotFound      desktopErrorCode = "PROJECT_NOT_FOUND"
	errorPermissionDenied     desktopErrorCode = "PERMISSION_DENIED"
	errorProjectCorrupt       desktopErrorCode = "PROJECT_CORRUPT"
	errorProjectTooNew        desktopErrorCode = "PROJECT_TOO_NEW"
	errorRevisionConflict     desktopErrorCode = "REVISION_CONFLICT"
	errorWriteFailed          desktopErrorCode = "WRITE_FAILED"
	errorDatabaseBusy         desktopErrorCode = "DATABASE_BUSY"
	errorReferenceConflict    desktopErrorCode = "REFERENCE_CONFLICT"
	errorBranchConflict       desktopErrorCode = "BRANCH_CONFLICT"
	errorAgentBudgetExceeded  desktopErrorCode = "AGENT_BUDGET_EXCEEDED"
	errorAgentContextExceeded desktopErrorCode = "AGENT_CONTEXT_EXCEEDED"
	errorAgentRunActive       desktopErrorCode = "AGENT_RUN_ACTIVE"
	errorOperationCancelled   desktopErrorCode = "OPERATION_CANCELLED"
	errorOperationTimeout     desktopErrorCode = "OPERATION_TIMEOUT"
	errorImportFormat         desktopErrorCode = "IMPORT_FORMAT_UNSUPPORTED"
	errorImportEmpty          desktopErrorCode = "IMPORT_SOURCE_EMPTY"
	errorImportTooLarge       desktopErrorCode = "IMPORT_SOURCE_TOO_LARGE"
	errorImportEncoding       desktopErrorCode = "IMPORT_ENCODING_UNSUPPORTED"
	errorImportUnsafe         desktopErrorCode = "IMPORT_SOURCE_UNSAFE"
	errorImportDuplicate      desktopErrorCode = "IMPORT_SOURCE_DUPLICATE"
	errorImportJobMissing     desktopErrorCode = "IMPORT_JOB_MISSING"
	errorImportJobApplied     desktopErrorCode = "IMPORT_JOB_APPLIED"
	errorImportJobNotPrepared desktopErrorCode = "IMPORT_JOB_NOT_PREPARED"
	errorImportSelection      desktopErrorCode = "IMPORT_SELECTION_REQUIRED"
	errorImportMapping        desktopErrorCode = "IMPORT_MAPPING_INVALID"
	errorImportArchive        desktopErrorCode = "IMPORT_ARCHIVE_UNSUPPORTED"
	errorImportCleanup        desktopErrorCode = "IMPORT_ARCHIVE_CLEANUP_FAILED"
	errorImportActive         desktopErrorCode = "IMPORT_ACTIVE"
	errorImportRefresh        desktopErrorCode = "IMPORT_APPLIED_REFRESH_FAILED"
	errorImportWorkspaceRead  desktopErrorCode = "IMPORT_WORKSPACE_READ_FAILED"
	errorImportPrepare        desktopErrorCode = "IMPORT_SOURCE_PREPARE_FAILED"
	errorImportUnavailable    desktopErrorCode = "IMPORT_UNAVAILABLE"
	errorImportInvalid        desktopErrorCode = "IMPORT_DATA_INVALID"
	errorImportPersistence    desktopErrorCode = "IMPORT_PERSISTENCE_FAILED"
	errorValidationFailed     desktopErrorCode = "VALIDATION_FAILED"
	errorOperationFailed      desktopErrorCode = "OPERATION_FAILED"
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
	case errors.Is(err, context.Canceled):
		code, message = errorOperationCancelled, "操作已取消。"
	case errors.Is(err, context.DeadlineExceeded):
		code, message = errorOperationTimeout, "操作超时，请重试。"
	case errors.Is(err, ports.ErrUnsupportedImportFormat):
		code, message = errorImportFormat, "仅支持 TXT、MD 或 MARKDOWN 小说文件。"
	case errors.Is(err, ports.ErrEmptyImportSource), errors.Is(err, importdomain.ErrEmptyManuscript):
		code, message = errorImportEmpty, "所选小说文件为空。"
	case errors.Is(err, ports.ErrImportSourceTooLarge):
		code, message = errorImportTooLarge, "小说文件超过 20 MiB 上限。"
	case errors.Is(err, ports.ErrUnsupportedImportEncoding):
		code, message = errorImportEncoding, "无法识别小说文件编码，请转换为 UTF-8、UTF-16 或 GB18030。"
	case errors.Is(err, ports.ErrUnsafeImportSource), errors.Is(err, ports.ErrInvalidImportArchivePath):
		code, message = errorImportUnsafe, "所选来源不是安全的普通文件。"
	case errors.Is(err, ports.ErrDuplicateImportSource):
		code, message = errorImportDuplicate, "这份小说已在当前项目中准备过。"
	case errors.Is(err, ports.ErrImportJobMissing), errors.Is(err, workspace.ErrImportJobProjectMismatch):
		code, message = errorImportJobMissing, "导入任务不存在或已被删除。"
	case errors.Is(err, ports.ErrImportJobAlreadyApplied):
		code, message = errorImportJobApplied, "该导入任务已经应用，不能重复操作。"
	case errors.Is(err, ports.ErrImportJobNotPrepared):
		code, message = errorImportJobNotPrepared, "该导入任务尚未准备完成。"
	case errors.Is(err, ports.ErrNoImportChaptersSelected):
		code, message = errorImportSelection, "请至少选择一个章节。"
	case errors.Is(err, ports.ErrImportChapterMappingMismatch):
		code, message = errorImportMapping, "所选章节与导入预览不一致，请刷新后重试。"
	case errors.Is(err, ports.ErrImportArchiveUnsupported):
		code, message = errorImportArchive, "当前文件系统不支持安全归档，请换到本机磁盘后重试。"
	case errors.Is(err, workspace.ErrImportArchiveCleanup):
		code, message = errorImportCleanup, "导入未完成，且暂存文件清理失败。"
	case errors.Is(err, workspace.ErrImportOperationActive):
		code, message = errorImportActive, "另一个导入操作正在进行，请稍后重试。"
	case errors.Is(err, workspace.ErrImportAppliedRefresh):
		code, message = errorImportRefresh, "章节已导入，但工作区刷新失败，请重新打开项目。"
	case errors.Is(err, workspace.ErrImportWorkspaceRead):
		code, message = errorImportWorkspaceRead, "无法读取导入工作区，请重新打开项目。"
	case errors.Is(err, workspace.ErrImportSourceArchive), errors.Is(err, ports.ErrImportArchiveExists):
		code, message = errorImportPrepare, "无法安全准备小说文件，请检查文件后重试。"
	case errors.Is(err, workspace.ErrImportStoreUnavailable):
		code, message = errorImportUnavailable, "当前项目不支持小说导入，请升级或重新打开项目。"
	case errors.Is(err, importdomain.ErrBlankValue),
		errors.Is(err, importdomain.ErrInvalidSourceFormat),
		errors.Is(err, importdomain.ErrInvalidSourceHash),
		errors.Is(err, importdomain.ErrInvalidSourceBytes),
		errors.Is(err, importdomain.ErrInvalidChapterCount),
		errors.Is(err, importdomain.ErrInvalidJobTransition),
		errors.Is(err, importdomain.ErrInvalidImportChapter),
		errors.Is(err, importdomain.ErrManuscriptLineTooLong):
		code, message = errorImportInvalid, "导入数据无效，请重新选择小说文件。"
	case errors.Is(err, ports.ErrImportPersistence):
		code, message = errorImportPersistence, "无法保存导入任务，请检查项目写入权限。"
	case errors.Is(err, workspace.ErrTokenBudgetExceeded):
		code, message = errorAgentBudgetExceeded, "智能创作令牌预算不足，请提高预算或降低角色输出上限。"
	case errors.Is(err, workspace.ErrContextBudgetExceeded):
		code, message = errorAgentContextExceeded, "必要的故事上下文超过预算，请提高上下文字符预算。"
	case errors.Is(err, workspace.ErrAgentRunActive):
		code, message = errorAgentRunActive, "智能创作任务正在运行，请先等待完成或取消任务。"
	case errors.Is(err, agentdomain.ErrInvalidBudget):
		code, message = errorValidationFailed, "运行预算必须是大于零的整数。"
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, sqlite.ErrDatabaseMissing):
		code, message = errorProjectNotFound, "项目不存在或已被移动。"
	case errors.Is(err, fs.ErrPermission), errors.Is(err, sqlite.ErrDatabasePermission):
		code, message = errorPermissionDenied, "没有权限读取或写入该位置。"
	case errors.Is(err, sqlite.ErrDatabaseCorrupt), errors.Is(err, sqlite.ErrProjectCount), errors.Is(err, projectfs.ErrInvalidPackage):
		code, message = errorProjectCorrupt, "项目格式无效或数据已损坏。"
	case errors.Is(err, branchfs.ErrBranchHeadChanged):
		code, message = errorProjectCorrupt, "剧情分支文件已被外部修改或损坏，请从完整项目备份恢复。"
	case errors.Is(err, sqlite.ErrDatabaseTooNew):
		code, message = errorProjectTooNew, "该项目由更新版本创建，请升级应用后再打开。"
	case errors.Is(err, projectdomain.ErrRevisionConflict):
		code, message = errorRevisionConflict, "章节已在其他位置更新，本地文本已保留。"
	case errors.Is(err, revisiondomain.ErrBranchDiverged):
		code, message = errorBranchConflict, "目标分支在分叉后也发生了变化，暂时无法自动合并。"
	case errors.Is(err, revisiondomain.ErrStaleBranchMerge):
		code, message = errorBranchConflict, "合并预览已过期，请重新预览后再确认。"
	case errors.Is(err, revisiondomain.ErrBranchUnchanged):
		code, message = errorValidationFailed, "这个分支没有可合并的变化。"
	case errors.Is(err, branchfs.ErrProtectedBranch):
		code, message = errorValidationFailed, "主分支或当前分支不能删除。"
	case errors.Is(err, branchfs.ErrBranchHasChildren):
		code, message = errorValidationFailed, "该分支仍有子分支，请先处理子分支。"
	case errors.Is(err, revisiondomain.ErrBlankBranchName), errors.Is(err, branchfs.ErrDuplicateBranch):
		code, message = errorValidationFailed, "请填写不重复的分支名称。"
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
