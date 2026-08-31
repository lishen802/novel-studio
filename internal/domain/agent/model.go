package agent

import (
	"errors"
	"math"
	"net"
	"net/url"
	"strings"
	"time"
)

var (
	ErrBlankValue             = errors.New("required value must not be blank")
	ErrInvalidProvider        = errors.New("invalid provider kind")
	ErrInvalidEndpoint        = errors.New("invalid provider endpoint")
	ErrInsecureEndpoint       = errors.New("provider endpoint must use HTTPS unless it is loopback")
	ErrInvalidRole            = errors.New("invalid agent role")
	ErrInvalidProfile         = errors.New("invalid agent profile")
	ErrInvalidRun             = errors.New("invalid agent run")
	ErrInvalidBudget          = errors.New("invalid agent runtime budget")
	ErrInvalidTransition      = errors.New("invalid agent run status transition")
	ErrReviewerNotIndependent = errors.New("continuity reviewer must use a different provider connection or model from the writer")
)

const (
	DefaultMaxRunTokens    = 30000
	DefaultMaxContextChars = 120000
	// BudgetWanUnit 是界面“万”与持久化基础单位的换算。
	BudgetWanUnit = 10_000
	// DefaultBudgetWan 是自定义预算首次出现时的默认值（10000 万）。
	DefaultBudgetWan = 10_000
	// MinCustomBudgetWan 是自定义预算允许的最小值（1 万）。
	MinCustomBudgetWan = 1
	// UnlimitedBudgetSentinel 仅供执行器算术使用，不得作为对外“不限制”语义。
	UnlimitedBudgetSentinel = math.MaxInt / 4
	// ProviderTechnicalMaxOutput 是协议强制要求上限时的技术封顶。
	ProviderTechnicalMaxOutput = 128_000
)

// EffectiveBudget 把作者可见预算换成执行器可用的正整数；不限制使用哨兵。
func EffectiveBudget(value int, unlimited bool) int {
	if unlimited {
		return UnlimitedBudgetSentinel
	}
	return value
}

type RuntimeSettings struct {
	ProjectID       string
	MaxRunTokens    int
	MaxContextChars int
	UpdatedAt       time.Time
}

func NewRuntimeSettings(
	projectID string,
	maxRunTokens, maxContextChars int,
	now time.Time,
) (RuntimeSettings, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return RuntimeSettings{}, ErrBlankValue
	}
	if maxRunTokens < 1 || maxContextChars < 1 {
		return RuntimeSettings{}, ErrInvalidBudget
	}
	return RuntimeSettings{
		ProjectID:       projectID,
		MaxRunTokens:    maxRunTokens,
		MaxContextChars: maxContextChars,
		UpdatedAt:       now.UTC(),
	}, nil
}

type ProviderKind string

const (
	ProviderOpenAI    ProviderKind = "openai"
	ProviderAnthropic ProviderKind = "anthropic"
)

type ProviderConnection struct {
	ID            string
	ProjectID     string
	Name          string
	Kind          ProviderKind
	BaseURL       string
	CredentialRef string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NewProviderConnection(
	id, projectID, name string,
	kind ProviderKind,
	baseURL, credentialRef string,
	now time.Time,
) (ProviderConnection, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	name = strings.TrimSpace(name)
	credentialRef = strings.TrimSpace(credentialRef)
	if id == "" || projectID == "" || name == "" || credentialRef == "" {
		return ProviderConnection{}, ErrBlankValue
	}
	if kind != ProviderOpenAI && kind != ProviderAnthropic {
		return ProviderConnection{}, ErrInvalidProvider
	}
	normalizedURL, err := NormalizeEndpoint(baseURL)
	if err != nil {
		return ProviderConnection{}, err
	}
	now = now.UTC()
	return ProviderConnection{
		ID:            id,
		ProjectID:     projectID,
		Name:          name,
		Kind:          kind,
		BaseURL:       normalizedURL,
		CredentialRef: credentialRef,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// NormalizeEndpoint 规范化 Provider Base URL：远程地址必须 HTTPS，仅本机地址允许 HTTP。
// 应用设置领域与项目级 Provider 共用此规则，避免 API Key 通过明文 HTTP 发送。
func NormalizeEndpoint(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || parsed.Path == "" && parsed.Scheme == "" {
		return "", ErrInvalidEndpoint
	}
	if parsed.Scheme != "https" {
		host := parsed.Hostname()
		ip := net.ParseIP(host)
		isLoopback := strings.EqualFold(host, "localhost") || ip != nil && ip.IsLoopback()
		if parsed.Scheme != "http" || !isLoopback {
			return "", ErrInsecureEndpoint
		}
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return parsed.String(), nil
}

type Role string

const (
	RolePlanner            Role = "planner"
	RoleWriter             Role = "writer"
	RoleContinuityReviewer Role = "continuity_reviewer"
)

// ReasoningEffort 是创作方案冻结的推理强度；auto 表示交由模型厂商决定。
type ReasoningEffort string

const (
	ReasoningAuto   ReasoningEffort = "auto"
	ReasoningNone   ReasoningEffort = "none"
	ReasoningLow    ReasoningEffort = "low"
	ReasoningMedium ReasoningEffort = "medium"
	ReasoningHigh   ReasoningEffort = "high"
	ReasoningMax    ReasoningEffort = "max"
)

func (effort ReasoningEffort) Valid() bool {
	return effort == "" || effort == ReasoningAuto || effort == ReasoningNone || effort == ReasoningLow ||
		effort == ReasoningMedium || effort == ReasoningHigh || effort == ReasoningMax
}

// Normalized 把空值视为 auto，供冻结与 Provider 映射使用。
func (effort ReasoningEffort) Normalized() ReasoningEffort {
	if effort == "" {
		return ReasoningAuto
	}
	return effort
}

// OutputLimitMode 区分历史固定单次上限与新版方案级预算模式。
type OutputLimitMode string

const (
	OutputLimitFixed         OutputLimitMode = "fixed"
	OutputLimitProfileBudget OutputLimitMode = "profile_budget"
)

func (mode OutputLimitMode) Valid() bool {
	return mode == "" || mode == OutputLimitFixed || mode == OutputLimitProfileBudget
}

// Normalized 把空值视为历史固定上限，保证旧运行恢复语义不变。
func (mode OutputLimitMode) Normalized() OutputLimitMode {
	if mode == "" {
		return OutputLimitFixed
	}
	return mode
}

// OmitsConfiguredMaxOutput 表示不再用作者配置的单次上限截断请求。
func (mode OutputLimitMode) OmitsConfiguredMaxOutput() bool {
	return mode.Normalized() == OutputLimitProfileBudget
}

type Profile struct {
	ID              string
	ProjectID       string
	Role            Role
	ConnectionID    string
	Model           string
	MaxOutputTokens int
	Temperature     float64
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func NewAgentProfile(
	id, projectID string,
	role Role,
	connectionID, model string,
	maxOutputTokens int,
	temperature float64,
	enabled bool,
	now time.Time,
) (Profile, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	connectionID = strings.TrimSpace(connectionID)
	model = strings.TrimSpace(model)
	if id == "" || projectID == "" || connectionID == "" || model == "" {
		return Profile{}, ErrBlankValue
	}
	if !validRole(role) {
		return Profile{}, ErrInvalidRole
	}
	if maxOutputTokens < 1 || temperature < 0 || temperature > 2 {
		return Profile{}, ErrInvalidProfile
	}
	now = now.UTC()
	return Profile{
		ID:              id,
		ProjectID:       projectID,
		Role:            role,
		ConnectionID:    connectionID,
		Model:           model,
		MaxOutputTokens: maxOutputTokens,
		Temperature:     temperature,
		Enabled:         enabled,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

func validRole(role Role) bool {
	return role == RolePlanner || role == RoleWriter || role == RoleContinuityReviewer
}

func ValidateAuthoringProfiles(profiles []Profile) error {
	byRole := make(map[Role]Profile, len(profiles))
	for _, profile := range profiles {
		if profile.Enabled {
			byRole[profile.Role] = profile
		}
	}
	for _, role := range []Role{RolePlanner, RoleWriter, RoleContinuityReviewer} {
		if _, exists := byRole[role]; !exists {
			return ErrInvalidProfile
		}
	}
	writer := byRole[RoleWriter]
	reviewer := byRole[RoleContinuityReviewer]
	if writer.ConnectionID == reviewer.ConnectionID && writer.Model == reviewer.Model {
		return ErrReviewerNotIndependent
	}
	return nil
}

type RunStatus string

const (
	RunQueued      RunStatus = "queued"
	RunRunning     RunStatus = "running"
	RunCompleted   RunStatus = "completed"
	RunFailed      RunStatus = "failed"
	RunCancelled   RunStatus = "cancelled"
	RunRecoverable RunStatus = "recoverable"
)

type StepStatus string

const (
	StepQueued    StepStatus = "queued"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepCancelled StepStatus = "cancelled"
)

type RunStepSpec struct {
	ID        string
	Role      Role
	ProfileID string // 旧项目级智能体档案 ID；R2.1 新运行留空。

	// R2.1 运行冻结快照字段：新运行必填，旧运行留空。
	CreativeProfileID      string
	CreativeProfileVersion int64
	ProviderConnectionID   string
	ProviderName           string
	ProviderKind           ProviderKind
	BaseURL                string
	CredentialRef          string
	Model                  string
	MaxOutputTokens        int
	ReasoningEffort        ReasoningEffort
	OutputLimitMode        OutputLimitMode
	Temperature            float64

	// R3 工作流节点冻结字段；旧三步运行保持零值。
	NodeID           string
	PublicStageID    PublicStageID
	Capability       Capability
	PromptBinding    VersionBinding
	SchemaBinding    VersionBinding
	AssemblerID      string
	AssemblerVersion int
	InputDigest      string
}

// ResolvedRole 是一次运行冻结快照中单个角色实际使用的连接、模型与参数。
// 它只包含非敏感元数据与凭据引用，不包含 API Key。
type ResolvedRole struct {
	Role            Role
	ConnectionID    string
	ProviderName    string
	ProviderKind    ProviderKind
	BaseURL         string
	CredentialRef   string
	Model           string
	MaxOutputTokens int
	ReasoningEffort ReasoningEffort
	OutputLimitMode OutputLimitMode
	Temperature     float64
}

// ResolvedCreativeProfile 是项目锁定创作方案的解析结果，包含预算与三个角色
// 的实际绑定。它由应用设置服务按确切版本解析，供创建运行冻结快照使用。
type ResolvedCreativeProfile struct {
	ProfileID              string
	Version                int64
	MaxRunTokens           int
	MaxContextChars        int
	RunBudgetUnlimited     bool
	ContextBudgetUnlimited bool
	Roles                  []ResolvedRole
}

type RunStep struct {
	ID                string
	RunID             string
	Role              Role
	ProfileID         string
	Position          int
	Status            StepStatus
	Output            string
	ErrorCode         string
	ErrorMessage      string
	ProviderRequestID string
	InputTokens       int
	OutputTokens      int
	StartedAt         *time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time

	// R2.1 运行冻结快照字段：创建运行后，执行阶段只读取这些冻结值与凭据库，
	// 不再解析可变的应用设置，避免作者修改连接或方案后影响已创建任务。
	CreativeProfileID      string
	CreativeProfileVersion int64
	ProviderConnectionID   string
	ProviderName           string
	ProviderKind           ProviderKind
	BaseURL                string
	CredentialRef          string
	Model                  string
	MaxOutputTokens        int
	ReasoningEffort        ReasoningEffort
	OutputLimitMode        OutputLimitMode
	Temperature            float64

	// R3 智能体内核冻结字段。旧运行仅有 NodeID 与 Attempt，其余字段保持空值。
	NodeID            string
	PublicStageID     PublicStageID
	Capability        Capability
	Attempt           int
	PromptBinding     VersionBinding
	SchemaBinding     VersionBinding
	AssemblerID       string
	AssemblerVersion  int
	InputDigest       string
	OutputDigest      string
	PrimaryArtifactID string
	QualityStatus     QualityStatus
}

type Run struct {
	ID                     string
	ProjectID              string
	ChapterID              string
	ChapterBaseRevision    int64
	Prompt                 string
	Status                 RunStatus
	ErrorCode              string
	ErrorMessage           string
	ChangeSetID            string
	RetryOf                string
	TokenBudget            int
	ContextCharBudget      int
	TokenBudgetUnlimited   bool
	ContextBudgetUnlimited bool
	InputTokens            int
	OutputTokens           int
	ContextChars           int
	ContextTruncated       bool
	Steps                  []RunStep
	CreatedAt              time.Time
	UpdatedAt              time.Time
	StartedAt              *time.Time
	CompletedAt            *time.Time

	// R3 智能体内核冻结字段。空 WorkflowBinding.ID 表示从 R2.1 或更早版本迁移的运行。
	TaskKind             TaskKind
	WorkflowBinding      VersionBinding
	CanonicalHead        string
	InputDigest          string
	QualityStatus        QualityStatus
	BudgetPolicyVersion  int
	QualityPolicyVersion int
	MaxRevisionAttempts  int
}

// IsLegacy 判断运行是否缺少 R3 工作流冻结信息。
func (run Run) IsLegacy() bool {
	return strings.TrimSpace(run.WorkflowBinding.ID) == ""
}

func NewChapterDraftRun(
	id, projectID, chapterID string,
	chapterBaseRevision int64,
	prompt string,
	specs []RunStepSpec,
	now time.Time,
) (Run, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	chapterID = strings.TrimSpace(chapterID)
	prompt = strings.TrimSpace(prompt)
	if id == "" || projectID == "" || chapterID == "" || prompt == "" || chapterBaseRevision < 1 {
		return Run{}, ErrInvalidRun
	}
	if len(specs) != 3 {
		return Run{}, ErrInvalidRun
	}
	expected := []Role{RolePlanner, RoleWriter, RoleContinuityReviewer}
	now = now.UTC()
	steps := make([]RunStep, len(specs))
	for index, spec := range specs {
		if strings.TrimSpace(spec.ID) == "" || spec.Role != expected[index] {
			return Run{}, ErrInvalidRun
		}
		// 旧运行要求项目级智能体档案 ID；R2.1 新运行要求冻结快照字段完整。
		if strings.TrimSpace(spec.ProfileID) == "" &&
			(strings.TrimSpace(spec.CreativeProfileID) == "" ||
				strings.TrimSpace(spec.ProviderConnectionID) == "" ||
				strings.TrimSpace(spec.CredentialRef) == "" ||
				strings.TrimSpace(spec.Model) == "") {
			return Run{}, ErrInvalidRun
		}
		steps[index] = RunStep{
			ID:        strings.TrimSpace(spec.ID),
			RunID:     id,
			Role:      spec.Role,
			ProfileID: strings.TrimSpace(spec.ProfileID),
			Position:  index + 1,
			Status:    StepQueued,

			CreativeProfileID:      strings.TrimSpace(spec.CreativeProfileID),
			CreativeProfileVersion: spec.CreativeProfileVersion,
			ProviderConnectionID:   strings.TrimSpace(spec.ProviderConnectionID),
			ProviderName:           spec.ProviderName,
			ProviderKind:           spec.ProviderKind,
			BaseURL:                spec.BaseURL,
			CredentialRef:          strings.TrimSpace(spec.CredentialRef),
			Model:                  strings.TrimSpace(spec.Model),
			MaxOutputTokens:        spec.MaxOutputTokens,
			ReasoningEffort:        spec.ReasoningEffort.Normalized(),
			OutputLimitMode:        spec.OutputLimitMode.Normalized(),
			Temperature:            spec.Temperature,

			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	return Run{
		ID:                  id,
		ProjectID:           projectID,
		ChapterID:           chapterID,
		ChapterBaseRevision: chapterBaseRevision,
		Prompt:              prompt,
		Status:              RunQueued,
		TokenBudget:         DefaultMaxRunTokens,
		ContextCharBudget:   DefaultMaxContextChars,
		Steps:               steps,
		CreatedAt:           now,
		UpdatedAt:           now,
	}, nil
}

func (run Run) WithBudgets(tokenBudget, contextCharBudget int) (Run, error) {
	if tokenBudget < 1 || contextCharBudget < 1 {
		return Run{}, ErrInvalidBudget
	}
	run.TokenBudget = tokenBudget
	run.ContextCharBudget = contextCharBudget
	return run, nil
}

func (run Run) RecordUsage(inputTokens, outputTokens int) Run {
	if inputTokens > 0 {
		run.InputTokens += inputTokens
	}
	if outputTokens > 0 {
		run.OutputTokens += outputTokens
	}
	return run
}

func (run Run) RemainingTokens() int {
	if run.TokenBudgetUnlimited {
		return UnlimitedBudgetSentinel
	}
	remaining := run.TokenBudget - run.InputTokens - run.OutputTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

// RemainingContextChars 返回本次运行剩余上下文字符；不限制使用哨兵。
func (run Run) RemainingContextChars() int {
	if run.ContextBudgetUnlimited {
		return UnlimitedBudgetSentinel
	}
	remaining := run.ContextCharBudget - run.ContextChars
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (run Run) Transition(status RunStatus, message string, now time.Time) (Run, error) {
	if !validRunTransition(run.Status, status) {
		return Run{}, ErrInvalidTransition
	}
	now = now.UTC()
	run.Status = status
	run.ErrorMessage = strings.TrimSpace(message)
	run.UpdatedAt = now
	if status == RunRunning {
		run.StartedAt = &now
	}
	if status == RunCompleted || status == RunFailed || status == RunCancelled {
		run.CompletedAt = &now
	}
	return run, nil
}

func validRunTransition(from, to RunStatus) bool {
	switch from {
	case RunQueued:
		return to == RunRunning || to == RunCancelled
	case RunRunning:
		return to == RunCompleted || to == RunFailed || to == RunCancelled || to == RunRecoverable
	case RunRecoverable:
		return to == RunRunning || to == RunCancelled
	default:
		return false
	}
}

// NewWorkflowRun 根据已冻结工作流和模型绑定创建 R3 运行，输出始终停留在候选工件中。
func NewWorkflowRun(
	id, projectID, chapterID string,
	chapterBaseRevision int64,
	authorInstruction string,
	workflow VersionBinding,
	taskKind TaskKind,
	canonicalHead, inputDigest string,
	budgetPolicyVersion, qualityPolicyVersion, maxRevisionAttempts int,
	tokenBudget, contextCharBudget int,
	specs []RunStepSpec,
	now time.Time,
) (Run, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	chapterID = strings.TrimSpace(chapterID)
	authorInstruction = strings.TrimSpace(authorInstruction)
	canonicalHead = strings.TrimSpace(canonicalHead)
	if id == "" || projectID == "" || chapterID == "" || chapterBaseRevision < 1 ||
		authorInstruction == "" || workflow.Validate() != nil || !ValidTaskKind(taskKind) ||
		canonicalHead == "" || !ValidSHA256(inputDigest) || budgetPolicyVersion < 1 ||
		qualityPolicyVersion < 1 || maxRevisionAttempts < 0 || tokenBudget < 1 ||
		contextCharBudget < 1 || len(specs) == 0 {
		return Run{}, ErrInvalidRun
	}
	now = now.UTC()
	steps := make([]RunStep, len(specs))
	seenNodes := make(map[string]struct{}, len(specs))
	for index, spec := range specs {
		nodeID := strings.TrimSpace(spec.NodeID)
		execution := ExecutionBinding{
			RoleSlot: spec.Role, ProviderConnectionID: spec.ProviderConnectionID,
			ProviderName: spec.ProviderName, ProviderKind: spec.ProviderKind, BaseURL: spec.BaseURL,
			CredentialRef: spec.CredentialRef, Model: spec.Model,
			MaxOutputTokens: spec.MaxOutputTokens, ReasoningEffort: spec.ReasoningEffort,
			OutputLimitMode: spec.OutputLimitMode, Temperature: spec.Temperature,
		}
		if strings.TrimSpace(spec.ID) == "" || nodeID == "" || !ValidPublicStageID(spec.PublicStageID) || !validRole(spec.Role) ||
			!ValidCapability(spec.Capability) || !ValidRoleForCapability(spec.Capability, spec.Role) ||
			strings.TrimSpace(spec.CreativeProfileID) == "" || spec.CreativeProfileVersion < 1 ||
			strings.TrimSpace(spec.ProviderConnectionID) == "" || strings.TrimSpace(spec.ProviderName) == "" ||
			strings.TrimSpace(spec.CredentialRef) == "" || strings.TrimSpace(spec.Model) == "" ||
			!spec.ReasoningEffort.Valid() || !spec.OutputLimitMode.Valid() ||
			spec.PromptBinding.Validate() != nil || spec.SchemaBinding.Validate() != nil ||
			strings.TrimSpace(spec.AssemblerID) == "" || spec.AssemblerVersion < 1 || !ValidSHA256(spec.InputDigest) ||
			execution.Validate(spec.Capability) != nil {
			return Run{}, ErrInvalidRun
		}
		if _, exists := seenNodes[nodeID]; exists {
			return Run{}, ErrInvalidRun
		}
		seenNodes[nodeID] = struct{}{}
		steps[index] = RunStep{
			ID: spec.ID, RunID: id, Role: spec.Role, Position: index + 1, Status: StepQueued,
			CreativeProfileID: spec.CreativeProfileID, CreativeProfileVersion: spec.CreativeProfileVersion,
			ProviderConnectionID: spec.ProviderConnectionID, ProviderName: spec.ProviderName,
			ProviderKind: spec.ProviderKind, BaseURL: spec.BaseURL, CredentialRef: spec.CredentialRef,
			Model: spec.Model, MaxOutputTokens: spec.MaxOutputTokens, ReasoningEffort: spec.ReasoningEffort.Normalized(),
			OutputLimitMode: spec.OutputLimitMode.Normalized(), Temperature: spec.Temperature,
			NodeID: nodeID, PublicStageID: spec.PublicStageID, Capability: spec.Capability, Attempt: 1,
			PromptBinding: spec.PromptBinding, SchemaBinding: spec.SchemaBinding,
			AssemblerID: spec.AssemblerID, AssemblerVersion: spec.AssemblerVersion,
			InputDigest: spec.InputDigest, CreatedAt: now, UpdatedAt: now,
		}
	}
	return Run{
		ID: id, ProjectID: projectID, ChapterID: chapterID, ChapterBaseRevision: chapterBaseRevision,
		Prompt: authorInstruction, Status: RunQueued, TokenBudget: tokenBudget,
		ContextCharBudget: contextCharBudget, Steps: steps, CreatedAt: now, UpdatedAt: now,
		TaskKind: taskKind, WorkflowBinding: workflow, CanonicalHead: canonicalHead,
		InputDigest: inputDigest, BudgetPolicyVersion: budgetPolicyVersion,
		QualityPolicyVersion: qualityPolicyVersion, MaxRevisionAttempts: maxRevisionAttempts,
	}, nil
}

type ArtifactKind string

const (
	ArtifactContext          ArtifactKind = "context"
	ArtifactTaskInput        ArtifactKind = "task_input"
	ArtifactPlan             ArtifactKind = "plan"
	ArtifactDraft            ArtifactKind = "draft"
	ArtifactReview           ArtifactKind = "review"
	ArtifactRevision         ArtifactKind = "revision"
	ArtifactStructuredOutput ArtifactKind = "structured_output"
	ArtifactLintReport       ArtifactKind = "lint_report"
	ArtifactQualityReport    ArtifactKind = "quality_report"
	ArtifactFormatRepair     ArtifactKind = "format_repair"
)

type Artifact struct {
	ID        string
	RunID     string
	StepID    string
	Kind      ArtifactKind
	Content   string
	SHA256    string
	CreatedAt time.Time
}
