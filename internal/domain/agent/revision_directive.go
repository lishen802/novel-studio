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
	// AllowAdd/AllowRemove 显式授权可增删实体的集合；为空时结构变化一律越界。
	AllowAdd    []string `json:"allow_add,omitempty"`
	AllowRemove []string `json:"allow_remove,omitempty"`
}

// Validate 校验修订来源互斥、基线冻结和目标/保持引用边界。
func (directive RevisionDirective) Validate() error {
	// 文本目标与结构授权至少需要一项；两者都为空时修订没有可执行范围。
	if directive.Version != 1 || blank(directive.BaseArtifactID) || !ValidSHA256(directive.BaseDigest) ||
		(len(directive.TargetRefs) == 0 && len(directive.AllowAdd) == 0 && len(directive.AllowRemove) == 0) ||
		hasBlankOrDuplicate(directive.TargetRefs) || hasBlankOrDuplicate(directive.PreserveRefs) ||
		hasBlankOrDuplicate(directive.AllowAdd) || hasBlankOrDuplicate(directive.AllowRemove) {
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

// BaseCandidate 是作者定向修订的不可变基线：指向生成运行中的原始 Artifact，
// 或应用层为人工版本物化的基线。内容与摘要随运行创建冻结，供修订节点渲染与校验。
type BaseCandidate struct {
	ArtifactID string `json:"artifact_id"`
	Digest     string `json:"digest"`
	Content    string `json:"content"`
	// Blueprint 为 true 时基线渲染为 Prompt 的 Blueprint 变量，否则渲染为 Draft。
	Blueprint bool `json:"blueprint"`
}

// Validate 要求基线身份、摘要与内容齐备，且摘要实际绑定冻结内容。
func (base BaseCandidate) Validate() error {
	if blank(base.ArtifactID) || !ValidSHA256(base.Digest) || blank(base.Content) || SHA256Hex([]byte(base.Content)) != base.Digest {
		return ErrInvalidRevisionDirective
	}
	return nil
}
