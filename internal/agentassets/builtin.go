package agentassets

import (
	_ "embed"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

const (
	BaselineWorkflowID      = "baseline.chapter"
	BaselineWorkflowVersion = 1
)

var (
	//go:embed assets/prompts/baseline_chapter_v1.tmpl
	baselinePrompt string
	//go:embed assets/schemas/chapter_draft_v1.json
	chapterDraftSchema []byte
	//go:embed assets/schemas/story_blueprint_v1.json
	storyBlueprintSchema []byte
	//go:embed assets/schemas/chapter_plan_v1.json
	chapterPlanSchema []byte
	//go:embed assets/schemas/review_bundle_v1.json
	reviewBundleSchema []byte
	//go:embed assets/schemas/revise_draft_v1.json
	reviseDraftSchema []byte
	//go:embed assets/assemblers/json_v1.json
	jsonAssembler []byte
)

// Builtin 构造应用随版本发布的只读资源集合；资产更新必须创建新版本。
func Builtin() (*Registry, error) {
	prompt := BindingForText("baseline.chapter.prompt", 1, []byte(baselinePrompt))
	schema := BindingForText("chapter_draft.schema", 1, chapterDraftSchema)
	blueprintSchema := BindingForText("story_blueprint.schema", 1, storyBlueprintSchema)
	planSchema := BindingForText("chapter_plan.schema", 1, chapterPlanSchema)
	reviewSchema := BindingForText("review_bundle.schema", 1, reviewBundleSchema)
	revisionSchema := BindingForText("revise_draft.schema", 1, reviseDraftSchema)
	assembler := BindingForText("json.assembler", 1, jsonAssembler)
	definition := agentdomain.WorkflowDefinition{
		ID: BaselineWorkflowID, Version: BaselineWorkflowVersion, TaskKind: agentdomain.TaskBaselineDraft,
		EntryNodeID: "write", ResultNodeID: "write", BudgetPolicyVersion: 1,
		QualityPolicyVersion: 1, MaxRevisionAttempts: 0,
		MaxRunTokens: 6500, RepairReserve: 1000, RevisionReserve: 0,
		Nodes: []agentdomain.WorkflowNode{{
			ID: "write", Capability: agentdomain.CapabilityProseWriter, RoleSlot: agentdomain.RoleWriter,
			Prompt: prompt, Schema: schema, Assembler: assembler, InputSelectors: []string{"$task", "$context"},
			OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, FailurePolicy: agentdomain.FailureFailRun,
		}},
	}
	definition.DefinitionHash = definition.ContentHash()
	workflow := ports.WorkflowAsset{Binding: agentdomain.VersionBinding{ID: definition.ID, Version: definition.Version, Hash: definition.DefinitionHash}, Definition: definition}
	return New(
		[]ports.WorkflowAsset{workflow},
		[]ports.PromptAsset{{Binding: prompt, Template: baselinePrompt, Variables: []string{"Task", "Context"}}},
		[]ports.SchemaAsset{
			{Binding: schema, JSON: chapterDraftSchema},
			{Binding: blueprintSchema, JSON: storyBlueprintSchema},
			{Binding: planSchema, JSON: chapterPlanSchema},
			{Binding: reviewSchema, JSON: reviewBundleSchema},
			{Binding: revisionSchema, JSON: reviseDraftSchema},
		},
		[]ports.AssemblerAsset{{Binding: assembler, JSON: jsonAssembler}},
	)
}
