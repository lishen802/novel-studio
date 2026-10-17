package chapterloop

import (
	"errors"
	"sort"
	"strings"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// StoredRecord 是持久化层保存的不可变产品记录。
type StoredRecord struct {
	ID             string
	ProjectID      string
	Kind           string
	SubjectID      string
	IdempotencyKey string
	Payload        string
}

var (
	ErrCandidateNotSelectable = errors.New("chapter candidate is not selectable")
	ErrUnauthorizedLookup     = errors.New("lookup is not authorized")
	ErrLookupMiss             = errors.New("lookup missed")
	ErrConstraintChanged      = errors.New("author constraint changed")
	ErrBudgetStopped          = errors.New("generation stopped at limit")
	ErrResultUnknown          = errors.New("model result unknown")
	ErrCancelled              = errors.New("generation cancelled")
	ErrVersionChanged         = errors.New("frozen version changed")
	ErrPreferenceEscalation   = errors.New("preference cannot escalate to hard conflict")
	ErrInvalidProduction      = errors.New("invalid chapter production")
)

type ProductClass string

const (
	ClassHardConflict ProductClass = "hard_conflict"
	ClassUncertain    ProductClass = "uncertain"
	ClassPreference   ProductClass = "preference"
)

type VerificationStatus string

const (
	VerificationSupported    VerificationStatus = "supported"
	VerificationRefuted      VerificationStatus = "refuted"
	VerificationInsufficient VerificationStatus = "insufficient_evidence"
)

// ProductFinding 在内核问题上附加产品分类和核验，不另造一套审阅协议。
type ProductFinding struct {
	Finding       agentdomain.ReviewFinding `json:"finding"`
	Class         ProductClass              `json:"class"`
	SpeechOrGuess bool                      `json:"speechOrGuess"`
	Verification  VerificationStatus        `json:"verification,omitempty"`
	EvidenceNote  string                    `json:"evidenceNote,omitempty"`
}

// BlocksProgress 只有核验成立的硬冲突才阻断选稿。
func (finding ProductFinding) BlocksProgress() bool {
	return finding.Class == ClassHardConflict && finding.Verification == VerificationSupported
}

// DowngradeSpeech 不把人物发言或猜测直接判成世界规则硬冲突。
func DowngradeSpeech(finding ProductFinding) ProductFinding {
	if finding.SpeechOrGuess && finding.Class == ClassHardConflict {
		finding.Class = ClassUncertain
		if finding.Verification == "" || finding.Verification == VerificationSupported {
			finding.Verification = VerificationInsufficient
		}
	}
	return finding
}

// Escalate 拒绝把偏好建议自动升级为硬冲突。
func Escalate(finding ProductFinding, next ProductClass) (ProductFinding, error) {
	if finding.Class == ClassPreference && next == ClassHardConflict {
		return finding, ErrPreferenceEscalation
	}
	finding.Class = next
	return finding, nil
}

// VerifyFinding 没有证据时不能把问题判为成立。
func VerifyFinding(finding ProductFinding, status VerificationStatus, evidence string) (ProductFinding, error) {
	switch status {
	case VerificationSupported, VerificationRefuted, VerificationInsufficient:
	default:
		return finding, ErrInvalidProduction
	}
	if status == VerificationSupported && strings.TrimSpace(evidence) == "" {
		status = VerificationInsufficient
	}
	if finding.Class == ClassPreference && status == VerificationSupported {
		finding.Verification = VerificationRefuted
		finding.EvidenceNote = strings.TrimSpace(evidence)
		return finding, nil
	}
	finding.Verification = status
	finding.EvidenceNote = strings.TrimSpace(evidence)
	return finding, nil
}

// MergeProductFindings 复用确定性合并，只去重并暴露同证据冲突。
func MergeProductFindings(findings []ProductFinding) ([]ProductFinding, []agentdomain.FindingConflict, error) {
	groups := make([]agentdomain.ReviewFinding, 0, len(findings))
	byID := map[string]ProductFinding{}
	for _, finding := range findings {
		finding = DowngradeSpeech(finding)
		if finding.Class != ClassHardConflict && finding.Class != ClassUncertain && finding.Class != ClassPreference {
			return nil, nil, ErrInvalidProduction
		}
		if err := finding.Finding.Validate(); err != nil {
			return nil, nil, err
		}
		groups = append(groups, finding.Finding)
		byID[finding.Finding.ID] = finding
	}
	merged, err := agentdomain.MergeFindings(groups)
	if err != nil {
		return nil, nil, err
	}
	result := make([]ProductFinding, 0, len(merged.Findings))
	for _, finding := range merged.Findings {
		result = append(result, byID[finding.ID])
	}
	return result, merged.Conflicts, nil
}

// SameFrozenBody 要求同轮审阅读取同一正文哈希。
func SameFrozenBody(hashes []string) error {
	if len(hashes) == 0 {
		return ErrVersionChanged
	}
	first := hashes[0]
	for _, hash := range hashes {
		if hash == "" || hash != first {
			return ErrVersionChanged
		}
	}
	return nil
}

// Lookup 只在授权且命中时返回证据。
func Lookup(index map[string]string, key string, authorized bool) (string, error) {
	if !authorized {
		return "", ErrUnauthorizedLookup
	}
	value, ok := index[key]
	if !ok || strings.TrimSpace(value) == "" {
		return "", ErrLookupMiss
	}
	return value, nil
}

// Candidate 是一版可追溯正文，评估失效时不能入选。
type Candidate struct {
	ID                       string `json:"id"`
	Body                     string `json:"body"`
	EvaluationID             string `json:"evaluationId"`
	Valid                    bool   `json:"valid"`
	Sequence                 int    `json:"sequence"`
	IntroducedConflict       bool   `json:"introducedConflict"`
	KnowledgeBoundaryChanged bool   `json:"knowledgeBoundaryChanged"`
}

// SelectCandidate 按调用方指定的有效评估选稿，不默认取最后一份产物。
func SelectCandidate(candidates []Candidate, chosenID, reason string) (Candidate, error) {
	if strings.TrimSpace(chosenID) == "" || strings.TrimSpace(reason) == "" {
		return Candidate{}, ErrCandidateNotSelectable
	}
	for _, candidate := range candidates {
		if candidate.ID != chosenID {
			continue
		}
		if !candidate.Valid || candidate.EvaluationID == "" || candidate.IntroducedConflict || candidate.KnowledgeBoundaryChanged || strings.TrimSpace(candidate.Body) == "" {
			return Candidate{}, ErrCandidateNotSelectable
		}
		return candidate, nil
	}
	return Candidate{}, ErrCandidateNotSelectable
}

// NoteRevision 拒绝修订结果悄悄替换作者约束。
func NoteRevision(kept, proposed []string) error {
	left := append([]string(nil), kept...)
	right := append([]string(nil), proposed...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) != len(right) {
		return ErrConstraintChanged
	}
	for index := range left {
		if left[index] != right[index] {
			return ErrConstraintChanged
		}
	}
	return nil
}

// RunControl 汇总必须停止后续模型调用的条件。
type RunControl struct {
	Cancelled       bool
	ResultUnknown   bool
	BudgetExhausted bool
	VersionChanged  bool
}

// AllowCall 在取消、未知结果、超限或版本变化时阻止下一次调用。
func AllowCall(control RunControl) error {
	switch {
	case control.Cancelled:
		return ErrCancelled
	case control.ResultUnknown:
		return ErrResultUnknown
	case control.BudgetExhausted:
		return ErrBudgetStopped
	case control.VersionChanged:
		return ErrVersionChanged
	default:
		return nil
	}
}

// HarnessThresholds 是步骤进入默认工作流前必须冻结的门槛。
type HarnessThresholds struct {
	MaxFalsePositive float64
	MaxMiss          float64
	MinRevisionGain  float64
	MaxNewIssueRate  float64
	MaxCost          float64
	Frozen           bool
}

// HarnessObservation 是一次已记录的门槛观测。
type HarnessObservation struct {
	FalsePositive float64
	Miss          float64
	RevisionGain  float64
	NewIssueRate  float64
	Cost          float64
}

// AllowsDefault 只有冻结且全部指标达标的步骤可以成为默认工作流。
func (thresholds HarnessThresholds) AllowsDefault(observation HarnessObservation) bool {
	if !thresholds.Frozen {
		return false
	}
	return observation.FalsePositive <= thresholds.MaxFalsePositive &&
		observation.Miss <= thresholds.MaxMiss &&
		observation.RevisionGain >= thresholds.MinRevisionGain &&
		observation.NewIssueRate <= thresholds.MaxNewIssueRate &&
		observation.Cost <= thresholds.MaxCost
}
