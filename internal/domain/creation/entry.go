package creation

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidEntry      = errors.New("invalid creation entry revision")
	ErrInvalidSourceSpan = errors.New("invalid creation material source span")
	ErrInvalidGapReport  = errors.New("invalid creation plan gap report")
)

type EntryKind string

const (
	EntryIdea             EntryKind = "idea"
	EntryAuthorPlan       EntryKind = "author_plan"
	MaxEntryMaterialRunes           = 100000
)

// EntryRevision 保存不可变的起点与作者原文；切换起点也追加版本。
type EntryRevision struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"sessionId"`
	ParentID       string    `json:"parentId"`
	Version        int64     `json:"version"`
	Kind           EntryKind `json:"kind"`
	Material       string    `json:"material"`
	MaterialSHA256 string    `json:"materialSha256"`
	CreatedAt      time.Time `json:"createdAt"`
}

// NewEntryRevision 校验原创起点并逐字保留作者材料，不产生候选或正式资产。
func NewEntryRevision(id, sessionID, parentID string, version int64, kind EntryKind, material string, now time.Time) (EntryRevision, error) {
	revision := EntryRevision{ID: id, SessionID: sessionID, ParentID: parentID, Version: version,
		Kind: kind, Material: material, MaterialSHA256: materialHash(material), CreatedAt: now.UTC()}
	if err := revision.Validate(); err != nil {
		return EntryRevision{}, err
	}
	return revision, nil
}

// Validate 防止损坏来源、未知起点或无父版本的伪造历史进入存储。
func (revision EntryRevision) Validate() error {
	if strings.TrimSpace(revision.ID) == "" || strings.TrimSpace(revision.SessionID) == "" || revision.Version < 1 ||
		(revision.Kind != EntryIdea && revision.Kind != EntryAuthorPlan) || !utf8.ValidString(revision.Material) ||
		utf8.RuneCountInString(revision.Material) > MaxEntryMaterialRunes || revision.CreatedAt.IsZero() ||
		revision.MaterialSHA256 != materialHash(revision.Material) || revision.ParentID == revision.ID ||
		(revision.Version == 1 && revision.ParentID != "") || (revision.Version > 1 && strings.TrimSpace(revision.ParentID) == "") {
		return ErrInvalidEntry
	}
	if revision.Kind == EntryAuthorPlan && strings.TrimSpace(revision.Material) == "" {
		return ErrInvalidEntry
	}
	return nil
}

// Revise 形成新的来源版本，不改变旧版本及其证据引用。
func (revision EntryRevision) Revise(id string, kind EntryKind, material string, now time.Time) (EntryRevision, error) {
	if err := revision.Validate(); err != nil {
		return EntryRevision{}, err
	}
	if id == revision.ID || now.Before(revision.CreatedAt) {
		return EntryRevision{}, ErrInvalidEntry
	}
	return NewEntryRevision(id, revision.SessionID, revision.ID, revision.Version+1, kind, material, now)
}

func materialHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// MaterialSpan 使用左闭右开的 Unicode rune 范围绑定原始材料的准确版本。
type MaterialSpan struct {
	EntryID        string `json:"entryId"`
	MaterialSHA256 string `json:"materialSha256"`
	Start          int    `json:"start"`
	End            int    `json:"end"`
}

// Text 验证版本、哈希和字符边界后返回原文片段。
func (span MaterialSpan) Text(revision EntryRevision) (string, error) {
	if revision.Validate() != nil || span.EntryID != revision.ID || span.MaterialSHA256 != revision.MaterialSHA256 ||
		span.Start < 0 || span.End <= span.Start || span.End > utf8.RuneCountInString(revision.Material) {
		return "", ErrInvalidSourceSpan
	}
	return string([]rune(revision.Material)[span.Start:span.End]), nil
}
