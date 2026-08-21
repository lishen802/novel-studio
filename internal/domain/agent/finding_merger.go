package agent

import (
	"errors"
	"sort"
	"strconv"
	"strings"
)

var ErrInvalidFindingMerge = errors.New("invalid finding merge input")

type ConstraintNote struct {
	ID         string          `json:"id"`
	Category   FindingCategory `json:"category"`
	Constraint string          `json:"constraint"`
	Risk       string          `json:"risk,omitempty"`
	SourceRefs []string        `json:"source_refs"`
}

func (note ConstraintNote) Validate() error {
	if blank(note.ID) || !validFindingCategory(note.Category) || blank(note.Constraint) || len(note.SourceRefs) == 0 || hasBlankOrDuplicate(note.SourceRefs) {
		return ErrInvalidFindingMerge
	}
	return nil
}

type FindingConflict struct {
	EvidenceKey string   `json:"evidence_key"`
	FindingIDs  []string `json:"finding_ids"`
}

type FindingMergeResult struct {
	Findings  []ReviewFinding   `json:"findings"`
	Conflicts []FindingConflict `json:"conflicts"`
}

// MergeFindings 仅规范化、精确去重、稳定排序并标记同证据冲突，不创造新的 Finding。
func MergeFindings(groups ...[]ReviewFinding) (FindingMergeResult, error) {
	all := make([]ReviewFinding, 0)
	for _, group := range groups {
		for _, finding := range group {
			if finding.Validate() != nil || finding.Source != FindingModelReview {
				return FindingMergeResult{}, ErrInvalidFindingMerge
			}
			all = append(all, finding)
		}
	}
	if len(all) == 0 {
		return FindingMergeResult{Findings: []ReviewFinding{}, Conflicts: []FindingConflict{}}, nil
	}
	sort.Slice(all, func(left, right int) bool { return findingSortKey(all[left]) < findingSortKey(all[right]) })
	result := FindingMergeResult{Findings: make([]ReviewFinding, 0, len(all)), Conflicts: []FindingConflict{}}
	seenExact := map[string]bool{}
	evidenceMessages := map[string]map[string][]string{}
	for _, finding := range all {
		exact := findingEvidenceKey(finding) + "\x00" + finding.Message + "\x00" + finding.Suggestion
		if !seenExact[exact] {
			seenExact[exact] = true
			result.Findings = append(result.Findings, finding)
		}
		evidence := findingEvidenceKey(finding)
		if evidenceMessages[evidence] == nil {
			evidenceMessages[evidence] = map[string][]string{}
		}
		evidenceMessages[evidence][finding.Message] = append(evidenceMessages[evidence][finding.Message], finding.ID)
	}
	for evidence, messages := range evidenceMessages {
		if len(messages) < 2 {
			continue
		}
		ids := make([]string, 0)
		for _, messageIDs := range messages {
			ids = append(ids, messageIDs...)
		}
		sort.Strings(ids)
		result.Conflicts = append(result.Conflicts, FindingConflict{EvidenceKey: evidence, FindingIDs: ids})
	}
	sort.Slice(result.Conflicts, func(left, right int) bool {
		return result.Conflicts[left].EvidenceKey < result.Conflicts[right].EvidenceKey
	})
	return result, nil
}

func findingSortKey(finding ReviewFinding) string {
	return findingEvidenceKey(finding) + "\x00" + finding.Message + "\x00" + finding.Suggestion + "\x00" + finding.ID
}

func findingEvidenceKey(finding ReviewFinding) string {
	start, end := -1, -1
	if finding.ScopeStart != nil {
		start, end = *finding.ScopeStart, *finding.ScopeEnd
	}
	return strings.Join([]string{string(finding.Category), finding.ConstraintRef, strconv.Itoa(start), strconv.Itoa(end), finding.Quote}, "\x00")
}
