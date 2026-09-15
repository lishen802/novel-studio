package agentassets

import (
	_ "embed"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

const (
	StoryCandidateDiscussionWorkflowID      = "story.candidate_discussion"
	StoryCandidateDiscussionWorkflowVersion = 1
)

var (
	//go:embed assets/prompts/story_candidate_discussion_v1.tmpl
	candidateDiscussionPrompt string
	//go:embed assets/schemas/story_candidate_discussion_v1.json
	candidateDiscussionSchema []byte
)

// candidateDiscussionWorkflow 冻结只读讨论：规划角色分析当前对象，不改候选。
func candidateDiscussionWorkflow(prompt, schema, assembler agentdomain.VersionBinding) agentdomain.WorkflowDefinition {
	return agentdomain.WorkflowDefinition{
		ID: StoryCandidateDiscussionWorkflowID, Version: StoryCandidateDiscussionWorkflowVersion,
		TaskKind: agentdomain.TaskDiscussStoryCandidate, EntryNodeID: "discuss", ResultNodeID: "discuss",
		BudgetPolicyVersion: 1, QualityPolicyVersion: 1, MaxRevisionAttempts: 0, MaxRunTokens: 8000, RepairReserve: 800,
		Nodes: []agentdomain.WorkflowNode{
			{
				ID: "discuss", PublicStageID: agentdomain.PublicStageStoryPlanning,
				Capability: agentdomain.CapabilityStoryArchitect, RoleSlot: agentdomain.RolePlanner,
				Prompt: prompt, Schema: schema, Assembler: assembler, InputSelectors: []string{"$task", "$context"},
				OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 4000, FailurePolicy: agentdomain.FailureFailRun,
			},
		},
	}
}
