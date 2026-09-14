package agentassets

import (
	_ "embed"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

const StoryCandidateRevisionWorkflowID = "story.author_revision"

var (
	//go:embed assets/prompts/story_candidate_revision_v1.tmpl
	candidateRevisionPrompt string
	//go:embed assets/prompts/review_candidate_revision_v1.tmpl
	candidateRevisionReviewPrompt string
	//go:embed assets/schemas/story_candidate_revision_v1.json
	candidateRevisionSchema []byte
	//go:embed assets/schemas/story_candidate_revision_v2.json
	candidateRevisionSchemaV2 []byte
)

const StoryCandidateRevisionWorkflowVersionV2 = 2

// candidateRevisionWorkflow 冻结先修订后独立审阅的流程，结果质量由审阅出口决定。
func candidateRevisionWorkflow(version int, revise, review, schema, reviewSchema, assembler agentdomain.VersionBinding) agentdomain.WorkflowDefinition {
	return agentdomain.WorkflowDefinition{
		ID: StoryCandidateRevisionWorkflowID, Version: version, TaskKind: agentdomain.TaskReviseStoryCandidate,
		EntryNodeID: "revise", ResultNodeID: "review", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 0, MaxRunTokens: 30000, RepairReserve: 1000,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "revise", PublicStageID: agentdomain.PublicStageBoundedRevision, Capability: agentdomain.CapabilityBoundedRevision, RoleSlot: agentdomain.RolePlanner, Prompt: revise, Schema: schema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "$base", "$directive"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 12000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "review", PublicStageID: agentdomain.PublicStageIndependentReview, Capability: agentdomain.CapabilityContinuityReview, RoleSlot: agentdomain.RoleContinuityReviewer, Prompt: review, Schema: reviewSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "$base", "$directive", "node:revise"}, OutputKind: agentdomain.OutputFindings, MaxOutputTokens: 4000, FailurePolicy: agentdomain.FailureFailRun},
		}, Edges: []agentdomain.WorkflowEdge{{From: "revise", To: "review"}},
	}
}
