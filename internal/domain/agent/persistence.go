package agent

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxStepPreviewRunes = 800

var ErrInvalidAgentPersistence = errors.New("invalid agent persistence record")

// BoundedStepPreview 将运行步骤中的展示文本限制为固定长度，完整内容必须保存为 Artifact。
func BoundedStepPreview(content string) string {
	if utf8.RuneCountInString(content) <= MaxStepPreviewRunes {
		return content
	}
	runes := []rune(content)
	return string(runes[:MaxStepPreviewRunes])
}

func ValidArtifactKind(kind ArtifactKind) bool {
	switch kind {
	case ArtifactContext, ArtifactTaskInput, ArtifactPlan, ArtifactDraft, ArtifactReview,
		ArtifactRevision, ArtifactStructuredOutput, ArtifactLintReport, ArtifactQualityReport,
		ArtifactFormatRepair:
		return true
	default:
		return false
	}
}

func (artifact Artifact) Validate() error {
	if blank(artifact.ID) || blank(artifact.RunID) || !ValidArtifactKind(artifact.Kind) ||
		!ValidSHA256(artifact.SHA256) || artifact.CreatedAt.IsZero() ||
		SHA256Hex([]byte(artifact.Content)) != artifact.SHA256 {
		return ErrInvalidAgentPersistence
	}
	return nil
}

type ManifestSourceKind string

const (
	ManifestHarnessSpec       ManifestSourceKind = "harness_spec"
	ManifestAuthorInstruction ManifestSourceKind = "author_instruction"
	ManifestChapterRevision   ManifestSourceKind = "chapter_revision"
	ManifestStoryAsset        ManifestSourceKind = "story_asset"
	ManifestUpstreamArtifact  ManifestSourceKind = "upstream_artifact"
	ManifestAnchor            ManifestSourceKind = "anchor"
)

type ContextManifestItem struct {
	ManifestID      string
	SourceKind      ManifestSourceKind
	SourceID        string
	SourceVersion   string
	RangeStart      *int
	RangeEnd        *int
	ContentHash     string
	Position        int
	Priority        int
	SelectionReason string
	Provisional     bool
	Included        bool
	OmissionReason  string
}

func (item ContextManifestItem) Validate() error {
	if blank(item.ManifestID) || !validManifestSourceKind(item.SourceKind) || blank(item.SourceID) ||
		blank(item.SourceVersion) || !ValidSHA256(item.ContentHash) || item.Position < 1 ||
		item.Priority < 0 || item.Priority > 1000 || blank(item.SelectionReason) {
		return ErrInvalidAgentPersistence
	}
	if (item.RangeStart == nil) != (item.RangeEnd == nil) {
		return ErrInvalidAgentPersistence
	}
	if item.RangeStart != nil && (*item.RangeStart < 0 || *item.RangeEnd <= *item.RangeStart) {
		return ErrInvalidAgentPersistence
	}
	if item.Included == (strings.TrimSpace(item.OmissionReason) != "") {
		return ErrInvalidAgentPersistence
	}
	return nil
}

type ContextManifest struct {
	ID                   string
	Version              int
	RunID                string
	StepID               string
	Attempt              int
	CanonicalHead        string
	AssemblerID          string
	AssemblerVersion     int
	CharacterBudget      int
	EstimatedTokenBudget int
	UsedChars            int
	Truncated            bool
	AssembledContentHash string
	Items                []ContextManifestItem
	CreatedAt            time.Time
}

func (manifest ContextManifest) Validate() error {
	if blank(manifest.ID) || manifest.Version < 1 || blank(manifest.RunID) || blank(manifest.StepID) ||
		manifest.Attempt < 1 || blank(manifest.AssemblerID) || manifest.AssemblerVersion < 1 ||
		manifest.CharacterBudget < 1 || manifest.EstimatedTokenBudget < 0 || manifest.UsedChars < 0 ||
		manifest.UsedChars > manifest.CharacterBudget || !ValidSHA256(manifest.AssembledContentHash) ||
		manifest.CreatedAt.IsZero() {
		return ErrInvalidAgentPersistence
	}
	positions := make(map[int]struct{}, len(manifest.Items))
	for _, item := range manifest.Items {
		if item.ManifestID != manifest.ID || item.Validate() != nil {
			return ErrInvalidAgentPersistence
		}
		if _, exists := positions[item.Position]; exists {
			return ErrInvalidAgentPersistence
		}
		positions[item.Position] = struct{}{}
	}
	return nil
}

type ReviewFindingRecord struct {
	ReviewFinding
	RunID     string
	StepID    string
	Attempt   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (finding ReviewFindingRecord) Validate() error {
	if blank(finding.RunID) || blank(finding.StepID) || finding.Attempt < 1 ||
		finding.CreatedAt.IsZero() || finding.UpdatedAt.Before(finding.CreatedAt) ||
		finding.ReviewFinding.Validate() != nil {
		return ErrInvalidAgentPersistence
	}
	return nil
}

// RunBundle 是创建运行时需要原子提交的完整持久化单元。
type RunBundle struct {
	Run       Run
	Artifacts []Artifact
	Relations []ArtifactRelationRecord
	Manifests []ContextManifest
	Findings  []ReviewFindingRecord
}

// StepBundle 是更新一次步骤尝试时需要原子提交的完整持久化单元。
type StepBundle struct {
	Step      RunStep
	Artifacts []Artifact
	Relations []ArtifactRelationRecord
	Manifests []ContextManifest
	Findings  []ReviewFindingRecord
}

type ArtifactRelation string

const (
	ArtifactDerivedFrom ArtifactRelation = "derived_from"
	ArtifactRevises     ArtifactRelation = "revises"
	ArtifactSupersedes  ArtifactRelation = "supersedes"
)

type ArtifactRelationRecord struct {
	ArtifactID        string
	Relation          ArtifactRelation
	RelatedArtifactID string
	BaseDigest        string
	CreatedAt         time.Time
}

func (relation ArtifactRelationRecord) Validate() error {
	if blank(relation.ArtifactID) || blank(relation.RelatedArtifactID) || relation.ArtifactID == relation.RelatedArtifactID ||
		!ValidSHA256(relation.BaseDigest) || relation.CreatedAt.IsZero() {
		return ErrInvalidAgentPersistence
	}
	switch relation.Relation {
	case ArtifactDerivedFrom, ArtifactRevises, ArtifactSupersedes:
		return nil
	default:
		return ErrInvalidAgentPersistence
	}
}

func validManifestSourceKind(kind ManifestSourceKind) bool {
	switch kind {
	case ManifestHarnessSpec, ManifestAuthorInstruction, ManifestChapterRevision,
		ManifestStoryAsset, ManifestUpstreamArtifact, ManifestAnchor:
		return true
	default:
		return false
	}
}
