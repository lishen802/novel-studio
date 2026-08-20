package quality

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

var (
	ErrInvalidPolicy  = errors.New("invalid quality policy")
	ErrPolicyNotFound = errors.New("quality policy not found")
	ErrInvalidInput   = errors.New("invalid quality input")
)

const MaxConstraints = 100

type Constraint struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Spec 冻结一次质量检查使用的动态目标与约束。
type Spec struct {
	TargetRunes     int          `json:"target_runes,omitempty"`
	RequiredAnchors []Constraint `json:"required_anchors,omitempty"`
	ForbiddenItems  []Constraint `json:"forbidden_items,omitempty"`
	ReservedNames   []Constraint `json:"reserved_names,omitempty"`
}

func (spec Spec) Validate() error {
	if spec.TargetRunes < 0 || len(spec.RequiredAnchors)+len(spec.ForbiddenItems)+len(spec.ReservedNames) > MaxConstraints {
		return ErrInvalidInput
	}
	seen := map[string]struct{}{}
	for _, constraints := range [][]Constraint{spec.RequiredAnchors, spec.ForbiddenItems, spec.ReservedNames} {
		for _, constraint := range constraints {
			id := strings.TrimSpace(constraint.ID)
			if id == "" || strings.TrimSpace(constraint.Text) == "" {
				return ErrInvalidInput
			}
			if _, exists := seen[id]; exists {
				return ErrInvalidInput
			}
			seen[id] = struct{}{}
		}
	}
	return nil
}

type RuleDefinition struct {
	ID       string                      `json:"id"`
	Version  int                         `json:"version"`
	Category agentdomain.FindingCategory `json:"category"`
	Severity agentdomain.FindingSeverity `json:"severity"`
}

// Policy 冻结规则版本、严重度和所有确定性阈值。
type Policy struct {
	Version                  int              `json:"version"`
	MinLengthRatio           float64          `json:"min_length_ratio"`
	MaxLengthRatio           float64          `json:"max_length_ratio"`
	ShortParagraphMaxRunes   int              `json:"short_paragraph_max_runes"`
	ShortParagraphMinCount   int              `json:"short_paragraph_min_count"`
	ShortParagraphRatio      float64          `json:"short_paragraph_ratio"`
	RepeatedOpeningRunes     int              `json:"repeated_opening_runes"`
	RepeatedOpeningCount     int              `json:"repeated_opening_count"`
	HighFrequencyPhraseRunes int              `json:"high_frequency_phrase_runes"`
	HighFrequencyPhraseCount int              `json:"high_frequency_phrase_count"`
	MechanicalParallelCount  int              `json:"mechanical_parallel_count"`
	MetaPhrases              []string         `json:"meta_phrases"`
	SummaryPhrases           []string         `json:"summary_phrases"`
	Rules                    []RuleDefinition `json:"rules"`
	Hash                     string           `json:"hash"`
}

// ContentHash 计算不包含自身哈希字段的稳定策略摘要。
func (policy Policy) ContentHash() string {
	policy.Hash = ""
	encoded, _ := json.Marshal(policy)
	return agentdomain.SHA256Hex(encoded)
}

func (policy Policy) Validate() error {
	if policy.Version < 1 || policy.MinLengthRatio <= 0 || policy.MinLengthRatio >= 1 ||
		policy.MaxLengthRatio <= 1 || policy.ShortParagraphMaxRunes < 1 || policy.ShortParagraphMinCount < 2 ||
		policy.ShortParagraphRatio <= 0 || policy.ShortParagraphRatio > 1 || policy.RepeatedOpeningRunes < 1 ||
		policy.RepeatedOpeningCount < 2 || policy.HighFrequencyPhraseRunes < 2 || policy.HighFrequencyPhraseCount < 2 ||
		policy.MechanicalParallelCount < 2 || len(policy.Rules) == 0 || !agentdomain.ValidSHA256(policy.Hash) ||
		policy.Hash != policy.ContentHash() || hasBlankOrDuplicate(policy.MetaPhrases) || hasBlankOrDuplicate(policy.SummaryPhrases) {
		return ErrInvalidPolicy
	}
	seen := map[string]struct{}{}
	for _, rule := range policy.Rules {
		if strings.TrimSpace(rule.ID) == "" || rule.Version < 1 {
			return ErrInvalidPolicy
		}
		probe := agentdomain.ReviewFinding{
			ID: "probe", Category: rule.Category, Severity: rule.Severity, Status: agentdomain.FindingOpen,
			Source: agentdomain.FindingDeterministicLint, Message: "probe", ConstraintRef: "rule:probe",
		}
		if probe.Validate() != nil {
			return ErrInvalidPolicy
		}
		if _, exists := seen[rule.ID]; exists {
			return ErrInvalidPolicy
		}
		seen[rule.ID] = struct{}{}
	}
	return nil
}

type Registry struct {
	mu       sync.RWMutex
	policies map[int]Policy
}

// NewRegistry 创建只读、精确按版本查询的质量策略注册表。
func NewRegistry(policies ...Policy) (*Registry, error) {
	registry := &Registry{policies: make(map[int]Policy, len(policies))}
	for _, policy := range policies {
		if err := policy.Validate(); err != nil {
			return nil, err
		}
		if _, exists := registry.policies[policy.Version]; exists {
			return nil, fmt.Errorf("%w: duplicate version %d", ErrInvalidPolicy, policy.Version)
		}
		registry.policies[policy.Version] = copyPolicy(policy)
	}
	return registry, nil
}

// Policy 返回指定版本的防御性副本。
func (registry *Registry) Policy(version int) (Policy, error) {
	if registry == nil {
		return Policy{}, ErrPolicyNotFound
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	policy, ok := registry.policies[version]
	if !ok {
		return Policy{}, fmt.Errorf("%w: version %d", ErrPolicyNotFound, version)
	}
	return copyPolicy(policy), nil
}

// BuiltinRegistry 返回随应用发布的确定性质量策略注册表。
func BuiltinRegistry() (*Registry, error) {
	policy := Policy{
		Version: 1, MinLengthRatio: 0.7, MaxLengthRatio: 1.3,
		ShortParagraphMaxRunes: 8, ShortParagraphMinCount: 4, ShortParagraphRatio: 0.6,
		RepeatedOpeningRunes: 2, RepeatedOpeningCount: 3,
		HighFrequencyPhraseRunes: 4, HighFrequencyPhraseCount: 3, MechanicalParallelCount: 3,
		MetaPhrases:    []string{"以下是正文", "作为AI", "作为 AI", "根据你的要求", "系统提示词", "提示词如下"},
		SummaryPhrases: []string{"总而言之", "综上所述", "这一切都说明", "命运的齿轮"},
		Rules: []RuleDefinition{
			{ID: "structure.non_empty", Version: 1, Category: agentdomain.FindingFormat, Severity: agentdomain.SeverityBlocker},
			{ID: "length.target_deviation", Version: 1, Category: agentdomain.FindingInformationDensity, Severity: agentdomain.SeverityMajor},
			{ID: "format.code_fence", Version: 1, Category: agentdomain.FindingFormat, Severity: agentdomain.SeverityBlocker},
			{ID: "format.meta_narration", Version: 1, Category: agentdomain.FindingFormat, Severity: agentdomain.SeverityMajor},
			{ID: "constraint.required_anchor", Version: 1, Category: agentdomain.FindingAnchor, Severity: agentdomain.SeverityBlocker},
			{ID: "constraint.forbidden_item", Version: 1, Category: agentdomain.FindingAnchor, Severity: agentdomain.SeverityBlocker},
			{ID: "originality.reserved_name", Version: 1, Category: agentdomain.FindingOriginalityRisk, Severity: agentdomain.SeverityBlocker},
			{ID: "repetition.consecutive_sentence", Version: 1, Category: agentdomain.FindingRepetition, Severity: agentdomain.SeverityMajor},
			{ID: "repetition.duplicate_paragraph", Version: 1, Category: agentdomain.FindingRepetition, Severity: agentdomain.SeverityMajor},
			{ID: "repetition.high_frequency_phrase", Version: 1, Category: agentdomain.FindingRepetition, Severity: agentdomain.SeverityMajor},
			{ID: "style.short_paragraph_ratio", Version: 1, Category: agentdomain.FindingStyle, Severity: agentdomain.SeverityMajor},
			{ID: "style.repeated_opening", Version: 1, Category: agentdomain.FindingStyle, Severity: agentdomain.SeverityMajor},
			{ID: "style.mechanical_parallel", Version: 1, Category: agentdomain.FindingStyle, Severity: agentdomain.SeverityMajor},
			{ID: "style.summary_tone", Version: 1, Category: agentdomain.FindingStyle, Severity: agentdomain.SeverityMinor},
		},
	}
	policy.Hash = policy.ContentHash()
	return NewRegistry(policy)
}

func copyPolicy(policy Policy) Policy {
	policy.MetaPhrases = append([]string(nil), policy.MetaPhrases...)
	policy.SummaryPhrases = append([]string(nil), policy.SummaryPhrases...)
	policy.Rules = append([]RuleDefinition(nil), policy.Rules...)
	return policy
}

func hasBlankOrDuplicate(values []string) bool {
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return true
		}
		if _, exists := seen[value]; exists {
			return true
		}
		seen[value] = struct{}{}
	}
	return false
}
