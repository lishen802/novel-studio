package agentkernel

import (
	"encoding/json"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// validateStoryCandidateRevision 绑定类型、版本、保持项与实际差异，不信任模型自述修改清单。
func validateStoryCandidateRevision(output agentdomain.StoryCandidateRevisionOutput, original string, directive agentdomain.RevisionDirective) error {
	var base agentdomain.StoryCandidateRevisionOutput
	if json.Unmarshal([]byte(original), &base) != nil || base.Validate() != nil || output.Validate() != nil || base.CandidateKind != output.CandidateKind || base.PayloadSchemaRef != output.PayloadSchemaRef {
		return agentdomain.ErrInvalidRevisionDirective
	}
	locks := make(map[string]bool, len(directive.PreserveRefs))
	for _, path := range directive.PreserveRefs {
		locks[path] = true
	}
	candidate := creationdomain.Candidate{PayloadJSON: string(base.Payload), LockMap: locks}
	_, err := candidate.ReviewScopedRevision(string(output.Payload), creationdomain.RevisionScope{
		Paths: directive.TargetRefs, AllowAdd: directive.AllowAdd, AllowRemove: directive.AllowRemove,
	})
	return err
}
