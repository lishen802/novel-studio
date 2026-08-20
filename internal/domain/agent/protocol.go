package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidTaskProtocol = errors.New("invalid agent task protocol")
	ErrInvalidFinding      = errors.New("invalid review finding")
)

type TaskKind string

const (
	TaskStoryBlueprint TaskKind = "story_blueprint"
	TaskChapterPlan    TaskKind = "chapter_plan"
	TaskChapterDraft   TaskKind = "chapter_draft"
	TaskReviewBundle   TaskKind = "review_bundle"
	TaskReviseDraft    TaskKind = "revise_draft"
	TaskBaselineDraft  TaskKind = "baseline_draft"
)

type Capability string

const (
	CapabilityStoryArchitect   Capability = "story_architect"
	CapabilityChapterPlanner   Capability = "chapter_planner"
	CapabilityProseWriter      Capability = "prose_writer"
	CapabilityContinuityReview Capability = "continuity_review"
	CapabilityContentReview    Capability = "content_review"
	CapabilityStyleReview      Capability = "style_review"
	CapabilityBoundedRevision  Capability = "bounded_revision"
)

type QualityStatus string

const (
	QualityPassed      QualityStatus = "passed"
	QualityNeedsReview QualityStatus = "needs_review"
	QualityBlocked     QualityStatus = "blocked"
)

type FindingSeverity string

const (
	SeverityBlocker FindingSeverity = "blocker"
	SeverityMajor   FindingSeverity = "major"
	SeverityMinor   FindingSeverity = "minor"
	SeverityNote    FindingSeverity = "note"
)

type FindingStatus string

const (
	FindingOpen          FindingStatus = "open"
	FindingResolved      FindingStatus = "resolved"
	FindingRetained      FindingStatus = "retained"
	FindingCannotResolve FindingStatus = "cannot_resolve"
)

type FindingSource string

const (
	FindingModelReview       FindingSource = "model_review"
	FindingDeterministicLint FindingSource = "deterministic_lint"
	FindingAuthor            FindingSource = "author"
)

type FindingCategory string

const (
	FindingAnchor             FindingCategory = "anchor"
	FindingContinuity         FindingCategory = "continuity"
	FindingKnowledgeBoundary  FindingCategory = "knowledge_boundary"
	FindingCausality          FindingCategory = "causality"
	FindingMotivation         FindingCategory = "motivation"
	FindingPacing             FindingCategory = "pacing"
	FindingHook               FindingCategory = "hook"
	FindingInformationDensity FindingCategory = "information_density"
	FindingWorldRule          FindingCategory = "world_rule"
	FindingForeshadowing      FindingCategory = "foreshadowing"
	FindingRepetition         FindingCategory = "repetition"
	FindingStyle              FindingCategory = "style"
	FindingFormat             FindingCategory = "format"
	FindingOriginalityRisk    FindingCategory = "originality_risk"
)

// ReviewFinding 是模型审阅、确定性检查与作者反馈共享的结构化问题协议。
type ReviewFinding struct {
	ID            string          `json:"id"`
	Category      FindingCategory `json:"category"`
	Severity      FindingSeverity `json:"severity"`
	Status        FindingStatus   `json:"status"`
	Source        FindingSource   `json:"source"`
	Message       string          `json:"message"`
	Suggestion    string          `json:"suggestion,omitempty"`
	Quote         string          `json:"quote,omitempty"`
	ScopeStart    *int            `json:"scope_start,omitempty"`
	ScopeEnd      *int            `json:"scope_end,omitempty"`
	ConstraintRef string          `json:"constraint_ref,omitempty"`
}

func (finding ReviewFinding) Validate() error {
	if blank(finding.ID) || blank(finding.Message) || !validFindingCategory(finding.Category) ||
		!validFindingSeverity(finding.Severity) || !validFindingStatus(finding.Status) || !validFindingSource(finding.Source) {
		return ErrInvalidFinding
	}
	if (finding.ScopeStart == nil) != (finding.ScopeEnd == nil) {
		return ErrInvalidFinding
	}
	if finding.ScopeStart != nil {
		if *finding.ScopeStart < 0 || *finding.ScopeEnd <= *finding.ScopeStart || blank(finding.Quote) {
			return ErrInvalidFinding
		}
	} else if blank(finding.ConstraintRef) {
		return ErrInvalidFinding
	}
	return nil
}

type VersionBinding struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	Hash    string `json:"hash"`
}

func (binding VersionBinding) Validate() error {
	if blank(binding.ID) || binding.Version < 1 || !ValidSHA256(binding.Hash) {
		return ErrInvalidTaskProtocol
	}
	return nil
}

type ExecutionBinding struct {
	RoleSlot             AgentRole    `json:"role_slot"`
	ProviderConnectionID string       `json:"provider_connection_id"`
	ProviderName         string       `json:"provider_name"`
	ProviderKind         ProviderKind `json:"provider_kind"`
	BaseURL              string       `json:"base_url"`
	CredentialRef        string       `json:"credential_ref"`
	Model                string       `json:"model"`
	MaxOutputTokens      int          `json:"max_output_tokens"`
	Temperature          float64      `json:"temperature"`
}

func (binding ExecutionBinding) Validate(capability Capability) error {
	if binding.RoleSlot != DefaultRoleForCapability(capability) || blank(binding.ProviderConnectionID) ||
		blank(binding.ProviderName) || blank(binding.CredentialRef) || blank(binding.Model) ||
		(binding.ProviderKind != ProviderOpenAI && binding.ProviderKind != ProviderAnthropic) ||
		binding.MaxOutputTokens < 1 || binding.Temperature < 0 || binding.Temperature > 2 {
		return ErrInvalidTaskProtocol
	}
	if _, err := NormalizeEndpoint(binding.BaseURL); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTaskProtocol, err)
	}
	return nil
}

type TaskBudget struct {
	MaxRunTokens    int `json:"max_run_tokens"`
	RemainingTokens int `json:"remaining_tokens"`
	RepairReserve   int `json:"repair_reserve"`
	RevisionReserve int `json:"revision_reserve"`
	MaxContextChars int `json:"max_context_chars"`
}

func (budget TaskBudget) Validate() error {
	if budget.MaxRunTokens < 1 || budget.RemainingTokens < 1 || budget.RemainingTokens > budget.MaxRunTokens ||
		budget.MaxContextChars < 1 || budget.RepairReserve < 0 || budget.RevisionReserve < 0 ||
		budget.RepairReserve+budget.RevisionReserve >= budget.RemainingTokens {
		return ErrInvalidBudget
	}
	return nil
}

// AgentTaskEnvelope 冻结一次智能体步骤所需的全部可重放输入，但绝不携带明文凭据。
type AgentTaskEnvelope struct {
	TaskID               string           `json:"task_id"`
	RunID                string           `json:"run_id"`
	StepID               string           `json:"step_id"`
	Attempt              int              `json:"attempt"`
	TaskKind             TaskKind         `json:"task_kind"`
	Capability           Capability       `json:"capability"`
	Workflow             VersionBinding   `json:"workflow"`
	ProjectID            string           `json:"project_id,omitempty"`
	BranchID             string           `json:"branch_id,omitempty"`
	CanonicalVersionID   string           `json:"canonical_version_id,omitempty"`
	AuthorInstructionRef string           `json:"author_instruction_ref,omitempty"`
	InputRefs            []string         `json:"input_refs"`
	InputDigest          string           `json:"input_digest"`
	ConstraintRefs       []string         `json:"constraint_refs"`
	ContextManifestID    string           `json:"context_manifest_id,omitempty"`
	Execution            ExecutionBinding `json:"execution"`
	Prompt               VersionBinding   `json:"prompt"`
	Schema               VersionBinding   `json:"schema"`
	Assembler            VersionBinding   `json:"assembler"`
	Budget               TaskBudget       `json:"budget"`
	Deadline             time.Time        `json:"deadline"`
}

func (envelope AgentTaskEnvelope) Validate() error {
	if blank(envelope.TaskID) || blank(envelope.RunID) || blank(envelope.StepID) || envelope.Attempt < 1 ||
		!ValidTaskKind(envelope.TaskKind) || !ValidCapability(envelope.Capability) || !ValidSHA256(envelope.InputDigest) ||
		len(envelope.InputRefs) == 0 || blank(envelope.ContextManifestID) || envelope.Deadline.IsZero() {
		return ErrInvalidTaskProtocol
	}
	if (!blank(envelope.ProjectID) && (blank(envelope.BranchID) || blank(envelope.CanonicalVersionID))) ||
		(blank(envelope.ProjectID) && (!blank(envelope.BranchID) || !blank(envelope.CanonicalVersionID))) {
		return ErrInvalidTaskProtocol
	}
	if err := envelope.Workflow.Validate(); err != nil {
		return err
	}
	if err := envelope.Prompt.Validate(); err != nil {
		return err
	}
	if err := envelope.Schema.Validate(); err != nil {
		return err
	}
	if err := envelope.Assembler.Validate(); err != nil {
		return err
	}
	if err := envelope.Execution.Validate(envelope.Capability); err != nil {
		return err
	}
	if err := envelope.Budget.Validate(); err != nil {
		return err
	}
	if envelope.Execution.MaxOutputTokens+envelope.Budget.RepairReserve+envelope.Budget.RevisionReserve > envelope.Budget.RemainingTokens ||
		hasBlankOrDuplicate(envelope.InputRefs) || hasBlankOrDuplicate(envelope.ConstraintRefs) {
		return ErrInvalidTaskProtocol
	}
	return nil
}

// AgentTaskResult 保存可回放结果、消耗和结构化质量结论。
type AgentTaskResult struct {
	TaskID                     string        `json:"task_id"`
	RunID                      string        `json:"run_id"`
	StepID                     string        `json:"step_id"`
	Attempt                    int           `json:"attempt"`
	Status                     StepStatus    `json:"status"`
	QualityStatus              QualityStatus `json:"quality_status"`
	StructuredOutputArtifactID string        `json:"structured_output_artifact_id,omitempty"`
	PrimaryContentArtifactID   string        `json:"primary_content_artifact_id,omitempty"`
	FindingIDs                 []string      `json:"finding_ids,omitempty"`
	OutputDigest               string        `json:"output_digest"`
	ProviderRequestID          string        `json:"provider_request_id,omitempty"`
	InputTokens                int           `json:"input_tokens"`
	OutputTokens               int           `json:"output_tokens"`
	StartedAt                  time.Time     `json:"started_at"`
	CompletedAt                time.Time     `json:"completed_at"`
	ErrorCode                  string        `json:"error_code,omitempty"`
	ErrorMessage               string        `json:"error_message,omitempty"`
}

func (result AgentTaskResult) Validate() error {
	if blank(result.TaskID) || blank(result.RunID) || blank(result.StepID) || result.Attempt < 1 ||
		!validTerminalStepStatus(result.Status) || !ValidQualityStatus(result.QualityStatus) ||
		result.InputTokens < 0 || result.OutputTokens < 0 ||
		result.StartedAt.IsZero() || result.CompletedAt.Before(result.StartedAt) {
		return ErrInvalidTaskProtocol
	}
	if result.Status == StepCompleted && (blank(result.StructuredOutputArtifactID) || !ValidSHA256(result.OutputDigest)) {
		return ErrInvalidTaskProtocol
	}
	if result.Status != StepCompleted && result.OutputDigest != "" && !ValidSHA256(result.OutputDigest) {
		return ErrInvalidTaskProtocol
	}
	if hasBlankOrDuplicate(result.FindingIDs) {
		return ErrInvalidTaskProtocol
	}
	return nil
}

// SHA256Hex 返回资源与协议快照统一使用的小写 SHA-256。
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ValidSHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func ValidTaskKind(kind TaskKind) bool {
	switch kind {
	case TaskStoryBlueprint, TaskChapterPlan, TaskChapterDraft, TaskReviewBundle, TaskReviseDraft, TaskBaselineDraft:
		return true
	default:
		return false
	}
}

func ValidCapability(capability Capability) bool {
	switch capability {
	case CapabilityStoryArchitect, CapabilityChapterPlanner, CapabilityProseWriter, CapabilityContinuityReview,
		CapabilityContentReview, CapabilityStyleReview, CapabilityBoundedRevision:
		return true
	default:
		return false
	}
}

func DefaultRoleForCapability(capability Capability) AgentRole {
	switch capability {
	case CapabilityStoryArchitect, CapabilityChapterPlanner:
		return RolePlanner
	case CapabilityProseWriter, CapabilityBoundedRevision:
		return RoleWriter
	case CapabilityContinuityReview, CapabilityContentReview, CapabilityStyleReview:
		return RoleContinuityReviewer
	default:
		return ""
	}
}

func ValidQualityStatus(status QualityStatus) bool {
	return status == QualityPassed || status == QualityNeedsReview || status == QualityBlocked
}

func blank(value string) bool { return strings.TrimSpace(value) == "" }

func validTerminalStepStatus(status StepStatus) bool {
	return status == StepCompleted || status == StepFailed || status == StepCancelled
}

func validFindingSeverity(value FindingSeverity) bool {
	return value == SeverityBlocker || value == SeverityMajor || value == SeverityMinor || value == SeverityNote
}

func validFindingStatus(value FindingStatus) bool {
	return value == FindingOpen || value == FindingResolved || value == FindingRetained || value == FindingCannotResolve
}

func validFindingSource(value FindingSource) bool {
	return value == FindingModelReview || value == FindingDeterministicLint || value == FindingAuthor
}

func validFindingCategory(value FindingCategory) bool {
	switch value {
	case FindingAnchor, FindingContinuity, FindingKnowledgeBoundary, FindingCausality, FindingMotivation,
		FindingPacing, FindingHook, FindingInformationDensity, FindingWorldRule, FindingForeshadowing,
		FindingRepetition, FindingStyle, FindingFormat, FindingOriginalityRisk:
		return true
	default:
		return false
	}
}
