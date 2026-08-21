package agent

import (
	"encoding/json"
	"errors"
	"strings"
)

var ErrInvalidRevisionDirective = errors.New("invalid revision directive")

type RevisionMode string

const (
	RevisionQualityFindings RevisionMode = "quality_findings"
	RevisionAuthorDirected  RevisionMode = "author_directed"
)

type RevisionDirective struct {
	Version              int          `json:"version"`
	Mode                 RevisionMode `json:"revision_mode"`
	BaseArtifactID       string       `json:"base_artifact_id"`
	BaseDigest           string       `json:"base_digest"`
	TargetRefs           []string     `json:"target_refs"`
	PreserveRefs         []string     `json:"preserve_refs"`
	FindingIDs           []string     `json:"finding_ids,omitempty"`
	AuthorInstructionRef string       `json:"author_instruction_ref,omitempty"`
}

// Validate 校验修订来源互斥、基线冻结和目标/保持引用边界。
func (directive RevisionDirective) Validate() error {
	if directive.Version != 1 || blank(directive.BaseArtifactID) || !ValidSHA256(directive.BaseDigest) || len(directive.TargetRefs) == 0 ||
		hasBlankOrDuplicate(directive.TargetRefs) || hasBlankOrDuplicate(directive.PreserveRefs) {
		return ErrInvalidRevisionDirective
	}
	targets := make(map[string]struct{}, len(directive.TargetRefs))
	for _, target := range directive.TargetRefs {
		targets[strings.TrimSpace(target)] = struct{}{}
	}
	for _, preserve := range directive.PreserveRefs {
		if _, exists := targets[strings.TrimSpace(preserve)]; exists {
			return ErrInvalidRevisionDirective
		}
	}
	switch directive.Mode {
	case RevisionQualityFindings:
		if len(directive.FindingIDs) == 0 || hasBlankOrDuplicate(directive.FindingIDs) || !blank(directive.AuthorInstructionRef) {
			return ErrInvalidRevisionDirective
		}
	case RevisionAuthorDirected:
		if len(directive.FindingIDs) != 0 || blank(directive.AuthorInstructionRef) {
			return ErrInvalidRevisionDirective
		}
	default:
		return ErrInvalidRevisionDirective
	}
	return nil
}

func (directive RevisionDirective) Digest() string {
	payload, _ := json.Marshal(directive)
	return SHA256Hex(payload)
}
