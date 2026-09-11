package creation

import (
	"encoding/hex"
	"strings"
)

type GapKind string
type GapResolution string

const (
	GapRequired GapKind       = "required"
	GapOptional GapKind       = "optional"
	GapConflict GapKind       = "conflict"
	GapOpen     GapResolution = "open"
	GapResolved GapResolution = "resolved"
)

// PlanGap 表达一个缺口及其来源；解决状态必须指向待验证的准确候选。
type PlanGap struct {
	ID                        string         `json:"id"`
	FieldPath                 string         `json:"fieldPath"`
	Kind                      GapKind        `json:"kind"`
	Reason                    string         `json:"reason"`
	Evidence                  []MaterialSpan `json:"evidence"`
	SuggestedAction           string         `json:"suggestedAction"`
	Resolution                GapResolution  `json:"resolution"`
	ResolutionCandidateID     string         `json:"resolutionCandidateId"`
	ResolutionCandidateSHA256 string         `json:"resolutionCandidateSha256"`
}

// PlanGapReport 冻结缺口报告对应的原文版本；候选是否仍有效由应用确认事务核验。
type PlanGapReport struct {
	ID             string    `json:"id"`
	SchemaVersion  int       `json:"schemaVersion"`
	EntryID        string    `json:"entryId"`
	MaterialSHA256 string    `json:"materialSha256"`
	Items          []PlanGap `json:"items"`
}

// Validate 验证报告身份、来源和解决引用的结构，不冒充语义审阅通过。
func (report PlanGapReport) Validate(revision EntryRevision) error {
	if revision.Validate() != nil || revision.Kind != EntryAuthorPlan || strings.TrimSpace(report.ID) == "" ||
		report.SchemaVersion != 1 || report.EntryID != revision.ID || report.MaterialSHA256 != revision.MaterialSHA256 || len(report.Items) > 500 {
		return ErrInvalidGapReport
	}
	seen := map[string]bool{}
	for _, item := range report.Items {
		if strings.TrimSpace(item.ID) == "" || seen[item.ID] || !strings.HasPrefix(item.FieldPath, "/") || len(item.FieldPath) < 2 ||
			strings.TrimSpace(item.Reason) == "" || strings.TrimSpace(item.SuggestedAction) == "" ||
			(item.Kind != GapRequired && item.Kind != GapOptional && item.Kind != GapConflict) ||
			(item.Resolution != GapOpen && item.Resolution != GapResolved) {
			return ErrInvalidGapReport
		}
		seen[item.ID] = true
		for _, evidence := range item.Evidence {
			if _, err := evidence.Text(revision); err != nil {
				return ErrInvalidGapReport
			}
		}
		if item.Kind == GapConflict && len(item.Evidence) == 0 {
			return ErrInvalidGapReport
		}
		if item.Resolution == GapResolved {
			digest, err := hex.DecodeString(item.ResolutionCandidateSHA256)
			if strings.TrimSpace(item.ResolutionCandidateID) == "" || err != nil || len(digest) != 32 {
				return ErrInvalidGapReport
			}
		} else if item.ResolutionCandidateID != "" || item.ResolutionCandidateSHA256 != "" {
			return ErrInvalidGapReport
		}
	}
	return nil
}

// Ready 仅判断必填与冲突项是否都有解决引用；不能替代 Validate 或候选质量、锁、引用核验。
func (report PlanGapReport) Ready(revision EntryRevision) bool {
	if report.Validate(revision) != nil {
		return false
	}
	for _, item := range report.Items {
		if item.Kind != GapOptional && (item.Resolution != GapResolved || item.ResolutionCandidateID == "" || item.ResolutionCandidateSHA256 == "") {
			return false
		}
	}
	return true
}
