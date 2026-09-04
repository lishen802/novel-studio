package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/appsettings"
	"github.com/lishen802/novel-studio/internal/application/appshell"
	"github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/creativeharness"
	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/application/workspace"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	appdomain "github.com/lishen802/novel-studio/internal/domain/appsettings"
	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	importdomain "github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/providers"
	"github.com/lishen802/novel-studio/internal/quality"
)

type desktopErrorCode string

var (
	errInvalidProjectOpenMode    = errors.New("project open mode is invalid")
	ErrRecoveryCopyResultMissing = errors.New("recovery copy completed without a published path")
)

const (
	errorProjectNotFound            desktopErrorCode = "PROJECT_NOT_FOUND"
	errorPermissionDenied           desktopErrorCode = "PERMISSION_DENIED"
	errorProjectCorrupt             desktopErrorCode = "PROJECT_CORRUPT"
	errorProjectTooNew              desktopErrorCode = "PROJECT_TOO_NEW"
	errorProjectReadOnly            desktopErrorCode = "PROJECT_READ_ONLY"
	errorProjectMigration           desktopErrorCode = "PROJECT_MIGRATION_REQUIRED"
	errorProjectRecovery            desktopErrorCode = "PROJECT_RECOVERY_REQUIRED"
	errorRecoveryCopyChanged        desktopErrorCode = "RECOVERY_COPY_SOURCE_CHANGED"
	errorRecoveryCopyFailed         desktopErrorCode = "RECOVERY_COPY_FAILED"
	errorProjectFilesystemPreflight desktopErrorCode = "PROJECT_FILESYSTEM_PREFLIGHT_FAILED"
	errorProjectPathTooLong         desktopErrorCode = "PROJECT_PATH_TOO_LONG"
	errorProjectSharingViolation    desktopErrorCode = "PROJECT_SHARING_VIOLATION"
	errorProjectCheckpointFailed    desktopErrorCode = "PROJECT_CHECKPOINT_FAILED"
	errorProjectAtomicReplaceFailed desktopErrorCode = "PROJECT_ATOMIC_REPLACE_FAILED"
	errorProjectFilesystemBusy      desktopErrorCode = "PROJECT_FILESYSTEM_BUSY"
	errorProjectFilesystemReadOnly  desktopErrorCode = "PROJECT_FILESYSTEM_READ_ONLY"
	errorProjectFilesystemDiskFull  desktopErrorCode = "PROJECT_FILESYSTEM_DISK_FULL"
	errorProjectFilesystemIO        desktopErrorCode = "PROJECT_FILESYSTEM_IO"
	errorRevisionConflict           desktopErrorCode = "REVISION_CONFLICT"
	errorWriteFailed                desktopErrorCode = "WRITE_FAILED"
	errorDatabaseBusy               desktopErrorCode = "DATABASE_BUSY"
	errorReferenceConflict          desktopErrorCode = "REFERENCE_CONFLICT"
	errorBranchConflict             desktopErrorCode = "BRANCH_CONFLICT"
	errorAgentBudgetExceeded        desktopErrorCode = "AGENT_BUDGET_EXCEEDED"
	errorAgentContextExceeded       desktopErrorCode = "AGENT_CONTEXT_EXCEEDED"
	errorAgentRunActive             desktopErrorCode = "AGENT_RUN_ACTIVE"
	errorOperationCancelled         desktopErrorCode = "OPERATION_CANCELLED"
	errorOperationTimeout           desktopErrorCode = "OPERATION_TIMEOUT"
	errorImportFormat               desktopErrorCode = "IMPORT_FORMAT_UNSUPPORTED"
	errorImportEmpty                desktopErrorCode = "IMPORT_SOURCE_EMPTY"
	errorImportTooLarge             desktopErrorCode = "IMPORT_SOURCE_TOO_LARGE"
	errorImportEncoding             desktopErrorCode = "IMPORT_ENCODING_UNSUPPORTED"
	errorImportUnsafe               desktopErrorCode = "IMPORT_SOURCE_UNSAFE"
	errorImportDuplicate            desktopErrorCode = "IMPORT_SOURCE_DUPLICATE"
	errorImportJobMissing           desktopErrorCode = "IMPORT_JOB_MISSING"
	errorImportJobApplied           desktopErrorCode = "IMPORT_JOB_APPLIED"
	errorImportJobNotPrepared       desktopErrorCode = "IMPORT_JOB_NOT_PREPARED"
	errorImportSelection            desktopErrorCode = "IMPORT_SELECTION_REQUIRED"
	errorImportMapping              desktopErrorCode = "IMPORT_MAPPING_INVALID"
	errorImportArchive              desktopErrorCode = "IMPORT_ARCHIVE_UNSUPPORTED"
	errorImportCleanup              desktopErrorCode = "IMPORT_ARCHIVE_CLEANUP_FAILED"
	errorImportActive               desktopErrorCode = "IMPORT_ACTIVE"
	errorImportRefresh              desktopErrorCode = "IMPORT_APPLIED_REFRESH_FAILED"
	errorImportWorkspaceRead        desktopErrorCode = "IMPORT_WORKSPACE_READ_FAILED"
	errorImportPrepare              desktopErrorCode = "IMPORT_SOURCE_PREPARE_FAILED"
	errorImportUnavailable          desktopErrorCode = "IMPORT_UNAVAILABLE"
	errorImportInvalid              desktopErrorCode = "IMPORT_DATA_INVALID"
	errorImportPersistence          desktopErrorCode = "IMPORT_PERSISTENCE_FAILED"
	errorValidationFailed           desktopErrorCode = "VALIDATION_FAILED"
	errorOperationFailed            desktopErrorCode = "OPERATION_FAILED"
	errorProjectRelocateRequired    desktopErrorCode = "PROJECT_RELOCATE_REQUIRED"
	errorFeatureNotAvailable        desktopErrorCode = "FEATURE_NOT_AVAILABLE"
	errorStaleSelection             desktopErrorCode = "STALE_SELECTION"

	// R2.1 模型基础设施稳定错误码。Provider 适配器负责把 HTTP 状态、响应错误
	// 类型与网络错误转换为领域错误，桌面层只转换为安全中文提示。
	errorProviderNotFound               desktopErrorCode = "PROVIDER_NOT_FOUND"
	errorProviderCredentialMiss         desktopErrorCode = "PROVIDER_CREDENTIAL_MISSING"
	errorProviderInUse                  desktopErrorCode = "PROVIDER_IN_USE"
	errorProviderUnauthorized           desktopErrorCode = "PROVIDER_UNAUTHORIZED"
	errorProviderModelNotFound          desktopErrorCode = "PROVIDER_MODEL_NOT_FOUND"
	errorProviderQuotaExceeded          desktopErrorCode = "PROVIDER_QUOTA_EXCEEDED"
	errorProviderRateLimited            desktopErrorCode = "PROVIDER_RATE_LIMITED"
	errorProviderNetworkFailed          desktopErrorCode = "PROVIDER_NETWORK_FAILED"
	errorProviderContentRejected        desktopErrorCode = "PROVIDER_CONTENT_REJECTED"
	errorProviderIncompatible           desktopErrorCode = "PROVIDER_INCOMPATIBLE"
	errorProviderIncomplete             desktopErrorCode = "PROVIDER_RESPONSE_INCOMPLETE"
	errorProviderResponseFailed         desktopErrorCode = "PROVIDER_RESPONSE_FAILED"
	errorProviderEmptyResponse          desktopErrorCode = "PROVIDER_EMPTY_RESPONSE"
	errorProviderReasoningWithoutOutput desktopErrorCode = "PROVIDER_REASONING_WITHOUT_OUTPUT"
	errorCredentialRollback             desktopErrorCode = "CREDENTIAL_ROLLBACK_FAILED"
	errorCreativeProfileInvalid         desktopErrorCode = "CREATIVE_PROFILE_INVALID"
	errorCreativeProfileMissing         desktopErrorCode = "CREATIVE_PROFILE_NOT_FOUND"
	errorHarnessConfirmation            desktopErrorCode = "HARNESS_CONFIRMATION_REQUIRED"
	errorHarnessRatingExists            desktopErrorCode = "HARNESS_RATING_EXISTS"
	errorCreationConfirmation           desktopErrorCode = "CREATION_CONFIRMATION_REQUIRED"
	errorCreationVersionConflict        desktopErrorCode = "CREATION_VERSION_CONFLICT"
	errorCreationImpact                 desktopErrorCode = "CREATION_IMPACT_UNRESOLVED"
)

var (
	errFeatureNotAvailable = errors.New("feature is not available in this release")
	errStaleSelection      = errors.New("selection is stale; chapter revision changed")
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
	if code, message, ok := safeProjectFilesystemError(err); ok {
		return &desktopError{Code: code, Message: message}
	}

	code := errorOperationFailed
	message := "操作失败，请重试。"
	lowerMessage := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, creativeharness.ErrConfirmationRequired):
		code, message = errorHarnessConfirmation, "运行配置或费用确认已变化，请重新预览并确认。"
	case errors.Is(err, creation.ErrConfirmationRequired):
		code, message = errorCreationConfirmation, "创作费用确认已变化，请重新预览并确认。"
	case errors.Is(err, creationdomain.ErrSessionVersionConflict):
		code, message = errorCreationVersionConflict, "创作会话已更新，本次修改未覆盖已有内容。请保留当前输入，重新读取并比较后再保存。"
	case errors.Is(err, creationdomain.ErrImpactUnresolved):
		code, message = errorCreationImpact, "上游修改后的影响尚未处理，请先选择局部重算、保留例外、继续讨论或放弃上游修改。"
	case errors.Is(err, workspace.ErrCreationUnavailable):
		code, message = errorFeatureNotAvailable, "当前项目无法打开创作工作台。"
	case errors.Is(err, creation.ErrRunActive):
		code, message = errorAgentRunActive, "已有创作运行正在进行，请等待完成或取消后再试。"
	case errors.Is(err, creation.ErrUnavailable):
		code, message = errorFeatureNotAvailable, "创作编排服务尚未就绪。"
	case errors.Is(err, creativeharness.ErrRunActive):
		code, message = errorOperationFailed, "运行中的实验不能删除，请先取消并等待任务结束。"
	case errors.Is(err, quality.ErrHarnessRatingExists):
		code, message = errorHarnessRatingExists, "该 case 已提交评分，不能重复覆盖。"
	case errors.Is(err, context.Canceled):
		code, message = errorOperationCancelled, "操作已取消。"
	case errors.Is(err, context.DeadlineExceeded):
		code, message = errorOperationTimeout, "操作超时，请重试。"
	case errors.Is(err, providers.ErrAuthentication):
		code, message = errorProviderUnauthorized, "API Key 无效或无权访问该模型。"
	case errors.Is(err, providers.ErrModelNotFound):
		code, message = errorProviderModelNotFound, "模型名称不存在或当前账户不可用。"
	case errors.Is(err, providers.ErrQuotaExceeded):
		code, message = errorProviderQuotaExceeded, "模型额度不足或账单受限。"
	case errors.Is(err, providers.ErrRateLimited):
		code, message = errorProviderRateLimited, "请求过于频繁，已被限流，请稍后重试。"
	case errors.Is(err, providers.ErrNetwork):
		code, message = errorProviderNetworkFailed, "网络连接失败，请检查网络或代理设置。"
	case errors.Is(err, providers.ErrContentRejected):
		code, message = errorProviderContentRejected, "请求被模型内容政策拒绝。"
	case errors.Is(err, providers.ErrIncompatible):
		code, message = errorProviderIncompatible, "接口地址与所选模型服务不兼容。"
	case errors.Is(err, providers.ErrResponseIncomplete):
		code, message = errorProviderIncomplete, "模型输出达到上限而未完成，请提高最大输出 Token 后重试。"
	case errors.Is(err, providers.ErrResponseFailed):
		code, message = errorProviderResponseFailed, "模型服务未能完成生成，请稍后重试。"
	case errors.Is(err, providers.ErrEmptyResponse):
		code, message = errorProviderEmptyResponse, "模型返回了空内容，请检查模型能力或生成参数。"
	case errors.Is(err, providers.ErrReasoningWithoutOutput):
		code, message = errorProviderReasoningWithoutOutput, "模型只完成了内部推理、没有返回可用正文。请在创作方案中降低该角色的推理强度后重试。"
	case errors.Is(err, appsettings.ErrProviderNotFound):
		code, message = errorProviderNotFound, "模型连接已被删除或页面数据已过期。"
	case errors.Is(err, appsettings.ErrCredentialMissing):
		code, message = errorProviderCredentialMiss, "API Key 未保存或系统凭据库不可用，请重新输入。"
	case errors.Is(err, appsettings.ErrProviderInUse):
		code, message = errorProviderInUse, "该模型连接仍被创作方案引用，请先调整方案后再删除。"
	case errors.Is(err, appsettings.ErrCredentialRollback):
		code, message = errorCredentialRollback, "配置保存失败且旧凭据未能自动恢复，请重新输入 API Key。"
	case errors.Is(err, appsettings.ErrProfileNotFound):
		code, message = errorCreativeProfileMissing, "项目锁定的创作方案版本不存在。"
	case errors.Is(err, workspace.ErrProjectProfileNotConfigured):
		code, message = errorCreativeProfileMissing, "项目尚未配置创作方案，请先在应用设置中发布并设为默认方案。"
	case errors.Is(err, appsettings.ErrInvalidProfileInput):
		code, message = errorCreativeProfileInvalid, "创作方案必须恰好包含规划、写作和审阅三个角色，且配置合法。"
	case errors.Is(err, appdomain.ErrInvalidRoleCount), errors.Is(err, appdomain.ErrInvalidVersion):
		code, message = errorCreativeProfileInvalid, "创作方案角色数量或版本不合法。"
	case errors.Is(err, appdomain.ErrInvalidGenerationParams):
		code, message = errorCreativeProfileInvalid, "创作方案角色的生成参数非法：推理强度必须合法，temperature ∈ [0, 2]，历史固定单次上限方案仍须满足输出之和不超过运行预算。"
	case errors.Is(err, agentdomain.ErrInvalidRole), errors.Is(err, agentdomain.ErrReviewerNotIndependent), errors.Is(err, agentdomain.ErrInvalidProfile):
		code, message = errorCreativeProfileInvalid, "创作方案角色配置不合法，审阅必须与写作使用不同连接或模型。"
	case errors.Is(err, appshell.ErrProjectRelocateRequired):
		code, message = errorProjectRelocateRequired, "项目文件已移动或删除，请重新定位项目。"
	case errors.Is(err, errFeatureNotAvailable):
		code, message = errorFeatureNotAvailable, "该功能将在后续版本中提供。"
	case errors.Is(err, errStaleSelection):
		code, message = errorStaleSelection, "选区已失效，章节正文已更新，请重新选择。"
	case errors.Is(err, errInvalidProjectOpenMode):
		code, message = errorValidationFailed, "项目打开模式无效。"
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
	case errors.Is(err, workspace.ErrWorkspaceReadOnly):
		code, message = errorProjectReadOnly, "项目已以只读模式打开，无法保存更改。"
	case errors.Is(err, sqlite.ErrDatabaseMigrationRequired):
		code, message = errorProjectMigration, "该项目需要升级，请以可编辑模式打开后完成升级。"
	case errors.Is(err, workspace.ErrProjectRecoveryRequired):
		code, message = errorProjectRecovery, "项目需要恢复，请以可编辑模式打开后完成恢复。"
	case errors.Is(err, workspace.ErrRecoveryCopySourceChanged):
		code, message = errorRecoveryCopyChanged, "源项目在创建恢复副本时发生变化，请重试。"
	case errors.Is(err, ErrRecoveryCopyResultMissing), strings.Contains(lowerMessage, "recovery copy"):
		code, message = errorRecoveryCopyFailed, "无法创建恢复副本，请检查目标位置后重试。"
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

func safeProjectFilesystemError(err error) (desktopErrorCode, string, bool) {
	var failure *ports.ProjectFilesystemFailure
	if !errors.As(err, &failure) {
		return "", "", false
	}
	switch failure.Category {
	case ports.ProjectFilesystemPreflight:
		return errorProjectFilesystemPreflight, "项目文件系统检查未通过，请检查位置后重试。", true
	case ports.ProjectFilesystemPathTooLong:
		return errorProjectPathTooLong, "项目路径过长，请移动到更短的位置后重试。", true
	case ports.ProjectFilesystemSharingViolation:
		return errorProjectSharingViolation, "项目文件正被同步或其他程序占用，请稍后重试。", true
	case ports.ProjectFilesystemCheckpoint:
		return errorProjectCheckpointFailed, "项目检查点写入失败，请检查位置后重试。", true
	case ports.ProjectFilesystemAtomicReplace:
		return errorProjectAtomicReplaceFailed, "项目文件替换失败，请检查位置后重试。", true
	case ports.ProjectFilesystemBusy:
		return errorProjectFilesystemBusy, "项目文件正被占用，请稍后重试。", true
	case ports.ProjectFilesystemReadOnly:
		return errorProjectFilesystemReadOnly, "项目位置当前不可写，请检查权限或同步状态后重试。", true
	case ports.ProjectFilesystemDiskFull:
		return errorProjectFilesystemDiskFull, "项目磁盘空间不足，请清理空间后重试。", true
	case ports.ProjectFilesystemIO:
		return errorProjectFilesystemIO, "项目文件读写失败，请检查位置后重试。", true
	default:
		return "", "", false
	}
}

func safeResult[T any](value T, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, safeDesktopError(err)
	}
	return value, nil
}
