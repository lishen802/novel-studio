package chapterloop

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	ErrInvalidSelection = errors.New("invalid selection rewrite")
	ErrSelectionStale   = errors.New("selection rewrite source changed")
)

type SelectionStatus string

const (
	SelectionProposed SelectionStatus = "proposed"
	SelectionAccepted SelectionStatus = "accepted"
	SelectionPartial  SelectionStatus = "partial"
	SelectionRejected SelectionStatus = "rejected"
)

// SelectionProposal 绑定章节版本、rune 范围和原文哈希，避免正文移动后错位覆盖。
type SelectionProposal struct {
	ID           string          `json:"id"`
	ChapterID    string          `json:"chapterId"`
	Revision     int64           `json:"revision"`
	StartRune    int             `json:"startRune"`
	EndRune      int             `json:"endRune"`
	SourceHash   string          `json:"sourceHash"`
	SourceText   string          `json:"sourceText"`
	Replacement  string          `json:"replacement"`
	EvaluationID string          `json:"evaluationId,omitempty"`
	Status       SelectionStatus `json:"status"`
	Mode         string          `json:"mode,omitempty"`
}

// PrepareSelection 按 Unicode rune 切出待改写原文。
func PrepareSelection(chapterID string, revision int64, body string, start, end int) (SelectionProposal, error) {
	runes := []rune(body)
	if strings.TrimSpace(chapterID) == "" || revision < 1 || start < 0 || end <= start || end > len(runes) {
		return SelectionProposal{}, ErrInvalidSelection
	}
	source := string(runes[start:end])
	return SelectionProposal{
		ChapterID: chapterID, Revision: revision, StartRune: start, EndRune: end,
		SourceText: source, SourceHash: hashText(source), Status: SelectionProposed,
	}, nil
}

// WithReplacement 记录候选文本和产生它的评估。
func (proposal SelectionProposal) WithReplacement(text, evaluationID string) SelectionProposal {
	proposal.Replacement = text
	proposal.EvaluationID = strings.TrimSpace(evaluationID)
	proposal.Status = SelectionProposed
	return proposal
}

// Reject 放弃候选，不改正文。
func (proposal SelectionProposal) Reject() SelectionProposal {
	proposal.Status = SelectionRejected
	return proposal
}

// ApplySelection 在当前正文仍与提案一致时拼接替换；部分采纳会使旧评估失效。
func ApplySelection(proposal SelectionProposal, body string, revision int64, replacement string, partial bool) (string, SelectionProposal, error) {
	if proposal.Status == SelectionRejected || strings.TrimSpace(replacement) == "" {
		return "", SelectionProposal{}, ErrInvalidSelection
	}
	runes := []rune(body)
	if revision != proposal.Revision || proposal.StartRune < 0 || proposal.EndRune > len(runes) || proposal.EndRune <= proposal.StartRune {
		return "", SelectionProposal{}, ErrSelectionStale
	}
	current := string(runes[proposal.StartRune:proposal.EndRune])
	if hashText(current) != proposal.SourceHash || current != proposal.SourceText {
		return "", SelectionProposal{}, ErrSelectionStale
	}
	updated := string(runes[:proposal.StartRune]) + replacement + string(runes[proposal.EndRune:])
	proposal.Replacement = replacement
	if partial {
		proposal.Status = SelectionPartial
		proposal.EvaluationID = ""
	} else {
		proposal.Status = SelectionAccepted
	}
	return updated, proposal, nil
}

// BodyHash 计算正文或选区的稳定摘要。
func BodyHash(value string) string {
	return hashText(value)
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
