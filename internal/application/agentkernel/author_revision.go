package agentkernel

import (
	"encoding/json"
	"strings"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

type AuthorRevisionRequest struct {
	RunID      string
	StepID     string
	ArtifactID string
	Directive  agentdomain.RevisionDirective
	Base       agentdomain.Artifact
	Output     agentdomain.TaskOutput
	CreatedAt  time.Time
}

type AuthorRevisionCandidate struct {
	Artifact agentdomain.Artifact
	Relation agentdomain.ArtifactRelationRecord
}

// BuildAuthorRevisionCandidate 验证作者定向修订边界并创建不触及 canonical 的候选及谱系。
func BuildAuthorRevisionCandidate(request AuthorRevisionRequest) (AuthorRevisionCandidate, error) {
	if strings.TrimSpace(request.RunID) == "" || strings.TrimSpace(request.StepID) == "" || strings.TrimSpace(request.ArtifactID) == "" ||
		request.CreatedAt.IsZero() || request.Directive.Validate() != nil || request.Directive.Mode != agentdomain.RevisionAuthorDirected ||
		request.Base.ID != request.Directive.BaseArtifactID || request.Base.SHA256 != request.Directive.BaseDigest || request.Base.Validate() != nil {
		return AuthorRevisionCandidate{}, agentdomain.ErrInvalidRevisionDirective
	}
	if request.Output == nil || request.Output.Validate() != nil {
		return AuthorRevisionCandidate{}, agentdomain.ErrInvalidTaskOutput
	}
	if err := validateAuthorDirectedRevision(request.Output, request.Base.Content, request.Directive); err != nil {
		return AuthorRevisionCandidate{}, err
	}
	var content string
	switch output := request.Output.(type) {
	case *agentdomain.ReviseDraftOutput:
		content = output.Body
	case *agentdomain.ReviseBlueprintOutput:
		payload, err := json.Marshal(output.Blueprint)
		if err != nil {
			return AuthorRevisionCandidate{}, err
		}
		content = string(payload)
	default:
		return AuthorRevisionCandidate{}, agentdomain.ErrInvalidTaskOutput
	}
	artifact := newArtifact(request.ArtifactID, request.RunID, request.StepID, agentdomain.ArtifactRevision, content, request.CreatedAt)
	relation := agentdomain.ArtifactRelationRecord{
		ArtifactID: artifact.ID, Relation: agentdomain.ArtifactRevises,
		RelatedArtifactID: request.Base.ID, BaseDigest: request.Base.SHA256, CreatedAt: request.CreatedAt.UTC(),
	}
	return AuthorRevisionCandidate{Artifact: artifact, Relation: relation}, nil
}
