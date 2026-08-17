package agent

import (
	"errors"
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
)

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
	normalizedURL, err := normalizeEndpoint(baseURL)
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

func normalizeEndpoint(raw string) (string, error) {
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

type AgentRole string

const (
	RolePlanner            AgentRole = "planner"
	RoleWriter             AgentRole = "writer"
	RoleContinuityReviewer AgentRole = "continuity_reviewer"
)

type AgentProfile struct {
	ID              string
	ProjectID       string
	Role            AgentRole
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
	role AgentRole,
	connectionID, model string,
	maxOutputTokens int,
	temperature float64,
	enabled bool,
	now time.Time,
) (AgentProfile, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	connectionID = strings.TrimSpace(connectionID)
	model = strings.TrimSpace(model)
	if id == "" || projectID == "" || connectionID == "" || model == "" {
		return AgentProfile{}, ErrBlankValue
	}
	if !validRole(role) {
		return AgentProfile{}, ErrInvalidRole
	}
	if maxOutputTokens < 1 || temperature < 0 || temperature > 2 {
		return AgentProfile{}, ErrInvalidProfile
	}
	now = now.UTC()
	return AgentProfile{
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

func validRole(role AgentRole) bool {
	return role == RolePlanner || role == RoleWriter || role == RoleContinuityReviewer
}

func ValidateAuthoringProfiles(profiles []AgentProfile) error {
	byRole := make(map[AgentRole]AgentProfile, len(profiles))
	for _, profile := range profiles {
		if profile.Enabled {
			byRole[profile.Role] = profile
		}
	}
	for _, role := range []AgentRole{RolePlanner, RoleWriter, RoleContinuityReviewer} {
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
	Role      AgentRole
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
	Temperature            float64
}

// ResolvedRole 是一次运行冻结快照中单个角色实际使用的连接、模型与参数。
// 它只包含非敏感元数据与凭据引用，不包含 API Key。
type ResolvedRole struct {
	Role            AgentRole
	ConnectionID    string
	ProviderName    string
	ProviderKind    ProviderKind
	BaseURL         string
	CredentialRef   string
	Model           string
	MaxOutputTokens int
	Temperature     float64
}

// ResolvedCreativeProfile 是项目锁定创作方案的解析结果，包含预算与三个角色
// 的实际绑定。它由应用设置服务按确切版本解析，供创建运行冻结快照使用。
type ResolvedCreativeProfile struct {
	ProfileID       string
	Version         int64
	MaxRunTokens    int
	MaxContextChars int
	Roles           []ResolvedRole
}

type AgentRunStep struct {
	ID                string
	RunID             string
	Role              AgentRole
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
	Temperature            float64
}

type AgentRun struct {
	ID                  string
	ProjectID           string
	ChapterID           string
	ChapterBaseRevision int64
	Prompt              string
	Status              RunStatus
	ErrorCode           string
	ErrorMessage        string
	ChangeSetID         string
	RetryOf             string
	TokenBudget         int
	ContextCharBudget   int
	InputTokens         int
	OutputTokens        int
	ContextChars        int
	ContextTruncated    bool
	Steps               []AgentRunStep
	CreatedAt           time.Time
	UpdatedAt           time.Time
	StartedAt           *time.Time
	CompletedAt         *time.Time
}

func NewChapterDraftRun(
	id, projectID, chapterID string,
	chapterBaseRevision int64,
	prompt string,
	specs []RunStepSpec,
	now time.Time,
) (AgentRun, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	chapterID = strings.TrimSpace(chapterID)
	prompt = strings.TrimSpace(prompt)
	if id == "" || projectID == "" || chapterID == "" || prompt == "" || chapterBaseRevision < 1 {
		return AgentRun{}, ErrInvalidRun
	}
	if len(specs) != 3 {
		return AgentRun{}, ErrInvalidRun
	}
	expected := []AgentRole{RolePlanner, RoleWriter, RoleContinuityReviewer}
	now = now.UTC()
	steps := make([]AgentRunStep, len(specs))
	for index, spec := range specs {
		if strings.TrimSpace(spec.ID) == "" || spec.Role != expected[index] {
			return AgentRun{}, ErrInvalidRun
		}
		// 旧运行要求项目级智能体档案 ID；R2.1 新运行要求冻结快照字段完整。
		if strings.TrimSpace(spec.ProfileID) == "" &&
			(strings.TrimSpace(spec.CreativeProfileID) == "" ||
				strings.TrimSpace(spec.ProviderConnectionID) == "" ||
				strings.TrimSpace(spec.CredentialRef) == "" ||
				strings.TrimSpace(spec.Model) == "") {
			return AgentRun{}, ErrInvalidRun
		}
		steps[index] = AgentRunStep{
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
			Temperature:            spec.Temperature,

			CreatedAt: now,
			UpdatedAt: now,
		}
	}
	return AgentRun{
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

func (run AgentRun) WithBudgets(tokenBudget, contextCharBudget int) (AgentRun, error) {
	if tokenBudget < 1 || contextCharBudget < 1 {
		return AgentRun{}, ErrInvalidBudget
	}
	run.TokenBudget = tokenBudget
	run.ContextCharBudget = contextCharBudget
	return run, nil
}

func (run AgentRun) RecordUsage(inputTokens, outputTokens int) AgentRun {
	if inputTokens > 0 {
		run.InputTokens += inputTokens
	}
	if outputTokens > 0 {
		run.OutputTokens += outputTokens
	}
	return run
}

func (run AgentRun) RemainingTokens() int {
	remaining := run.TokenBudget - run.InputTokens - run.OutputTokens
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (run AgentRun) Transition(status RunStatus, message string, now time.Time) (AgentRun, error) {
	if !validRunTransition(run.Status, status) {
		return AgentRun{}, ErrInvalidTransition
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
		return to == RunCancelled
	default:
		return false
	}
}

type ArtifactKind string

const (
	ArtifactContext ArtifactKind = "context"
	ArtifactPlan    ArtifactKind = "plan"
	ArtifactDraft   ArtifactKind = "draft"
	ArtifactReview  ArtifactKind = "review"
)

type AgentArtifact struct {
	ID        string
	RunID     string
	StepID    string
	Kind      ArtifactKind
	Content   string
	SHA256    string
	CreatedAt time.Time
}
