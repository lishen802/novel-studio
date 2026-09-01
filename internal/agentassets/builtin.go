package agentassets

import (
	_ "embed"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

const (
	BaselineWorkflowID              = "baseline.chapter"
	BaselineWorkflowVersion         = 1
	QualityChapterWorkflowID        = "quality.chapter"
	QualityChapterWorkflowVersion   = 1
	QualityBlueprintWorkflowID      = "quality.blueprint"
	QualityBlueprintWorkflowVersion = 1
	EnhancedChapterWorkflowID       = "quality.chapter.director"
	EnhancedChapterWorkflowVersion  = 1
	SplitReviewWorkflowID           = "quality.chapter.split-review"
	SplitReviewWorkflowVersion      = 1
)

// v2 在既有工作流末尾追加修订复核节点；v1 保留不变，供 Harness 判断复核节点是否值得这次调用。
const (
	VerifiedChapterWorkflowVersion     = 2
	VerifiedBlueprintWorkflowVersion   = 2
	VerifiedEnhancedWorkflowVersion    = 2
	VerifiedSplitReviewWorkflowVersion = 2
	verifyMaxOutputTokens              = 2500
)

var (
	//go:embed assets/prompts/baseline_chapter_v1.tmpl
	baselinePrompt string
	//go:embed assets/prompts/story_architect_v1.tmpl
	storyArchitectPrompt string
	//go:embed assets/prompts/chapter_plan_v1.tmpl
	chapterPlanPrompt string
	//go:embed assets/prompts/chapter_director_v2.tmpl
	chapterDirectorPrompt string
	//go:embed assets/prompts/prose_writer_v1.tmpl
	proseWriterPrompt string
	//go:embed assets/prompts/review_chapter_v1.tmpl
	reviewChapterPrompt string
	//go:embed assets/prompts/review_blueprint_v1.tmpl
	reviewBlueprintPrompt string
	//go:embed assets/prompts/revise_draft_v1.tmpl
	reviseDraftPrompt string
	//go:embed assets/prompts/revise_blueprint_v1.tmpl
	reviseBlueprintPrompt string
	//go:embed assets/prompts/author_revise_draft_v1.tmpl
	authorReviseDraftPrompt string
	//go:embed assets/prompts/author_revise_blueprint_v1.tmpl
	authorReviseBlueprintPrompt string
	//go:embed assets/prompts/verify_revision_v1.tmpl
	verifyRevisionPrompt string
	//go:embed assets/prompts/verify_revision_blueprint_v1.tmpl
	verifyRevisionBlueprintPrompt string
	//go:embed assets/schemas/chapter_draft_v1.json
	chapterDraftSchema []byte
	//go:embed assets/schemas/story_blueprint_v1.json
	storyBlueprintSchema []byte
	//go:embed assets/schemas/chapter_plan_v1.json
	chapterPlanSchema []byte
	//go:embed assets/schemas/chapter_plan_v2.json
	chapterDirectorSchema []byte
	//go:embed assets/schemas/review_bundle_v1.json
	reviewBundleSchema []byte
	//go:embed assets/schemas/revise_draft_v1.json
	reviseDraftSchema []byte
	//go:embed assets/schemas/revise_blueprint_v1.json
	reviseBlueprintSchema []byte
	//go:embed assets/schemas/author_revise_draft_v1.json
	authorReviseDraftSchema []byte
	//go:embed assets/schemas/author_revise_blueprint_v1.json
	authorReviseBlueprintSchema []byte
	//go:embed assets/schemas/verify_revision_v1.json
	verifyRevisionSchema []byte
	//go:embed assets/assemblers/json_v1.json
	jsonAssembler []byte
)

// Builtin 构造应用随版本发布的只读资源集合；资产更新必须创建新版本。
func Builtin() (*Registry, error) {
	baselinePromptBinding := BindingForText("baseline.chapter.prompt", 1, []byte(baselinePrompt))
	architectPromptBinding := BindingForText("story_architect.prompt", 1, []byte(storyArchitectPrompt))
	planPromptBinding := BindingForText("chapter_plan.prompt", 1, []byte(chapterPlanPrompt))
	directorPromptBinding := BindingForText("chapter_plan.prompt", 2, []byte(chapterDirectorPrompt))
	writerPromptBinding := BindingForText("prose_writer.prompt", 1, []byte(proseWriterPrompt))
	chapterReviewPromptBinding := BindingForText("review_chapter.prompt", 1, []byte(reviewChapterPrompt))
	blueprintReviewPromptBinding := BindingForText("review_blueprint.prompt", 1, []byte(reviewBlueprintPrompt))
	draftRevisionPromptBinding := BindingForText("revise_draft.prompt", 1, []byte(reviseDraftPrompt))
	blueprintRevisionPromptBinding := BindingForText("revise_blueprint.prompt", 1, []byte(reviseBlueprintPrompt))
	authorDraftPromptBinding := BindingForText("author_revise_draft.prompt", 1, []byte(authorReviseDraftPrompt))
	authorBlueprintPromptBinding := BindingForText("author_revise_blueprint.prompt", 1, []byte(authorReviseBlueprintPrompt))
	draftSchema := BindingForText("chapter_draft.schema", 1, chapterDraftSchema)
	blueprintSchema := BindingForText("story_blueprint.schema", 1, storyBlueprintSchema)
	planSchema := BindingForText("chapter_plan.schema", 1, chapterPlanSchema)
	directorSchema := BindingForText("chapter_plan.schema", 2, chapterDirectorSchema)
	reviewSchema := BindingForText("review_bundle.schema", 1, reviewBundleSchema)
	draftRevisionSchema := BindingForText("revise_draft.schema", 1, reviseDraftSchema)
	blueprintRevisionSchema := BindingForText("revise_blueprint.schema", 1, reviseBlueprintSchema)
	authorDraftSchemaBinding := BindingForText("author_revise_draft.schema", 1, authorReviseDraftSchema)
	authorBlueprintSchemaBinding := BindingForText("author_revise_blueprint.schema", 1, authorReviseBlueprintSchema)
	verifyPromptBinding := BindingForText("verify_revision.prompt", 1, []byte(verifyRevisionPrompt))
	verifyBlueprintPromptBinding := BindingForText("verify_revision_blueprint.prompt", 1, []byte(verifyRevisionBlueprintPrompt))
	verifySchema := BindingForText("verify_revision.schema", 1, verifyRevisionSchema)
	assembler := BindingForText("json.assembler", 1, jsonAssembler)
	baseline := agentdomain.WorkflowDefinition{
		ID: BaselineWorkflowID, Version: BaselineWorkflowVersion, TaskKind: agentdomain.TaskBaselineDraft,
		EntryNodeID: "write", ResultNodeID: "write", BudgetPolicyVersion: 1,
		QualityPolicyVersion: 1, MaxRevisionAttempts: 0,
		MaxRunTokens: 6500, RepairReserve: 1000, RevisionReserve: 0,
		Nodes: []agentdomain.WorkflowNode{{
			ID: "write", Capability: agentdomain.CapabilityProseWriter, RoleSlot: agentdomain.RoleWriter,
			Prompt: baselinePromptBinding, Schema: draftSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context"},
			OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, FailurePolicy: agentdomain.FailureFailRun,
		}},
	}
	chapter := agentdomain.WorkflowDefinition{
		ID: QualityChapterWorkflowID, Version: QualityChapterWorkflowVersion, TaskKind: agentdomain.TaskChapterDraft,
		EntryNodeID: "plan", ResultNodeID: "revision", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 1, MaxRunTokens: 17500, RepairReserve: 1000, RevisionReserve: 1000,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "plan", Capability: agentdomain.CapabilityChapterPlanner, RoleSlot: agentdomain.RolePlanner,
				Prompt: planPromptBinding, Schema: planSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context"},
				OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 2500, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "draft", Capability: agentdomain.CapabilityProseWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: writerPromptBinding, Schema: draftSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:plan"},
				OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "review", Capability: agentdomain.CapabilityContinuityReview, RoleSlot: agentdomain.RoleContinuityReviewer,
				Prompt: chapterReviewPromptBinding, Schema: reviewSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:draft"},
				OutputKind: agentdomain.OutputFindings, MaxOutputTokens: 3000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "revision", Capability: agentdomain.CapabilityBoundedRevision, RoleSlot: agentdomain.RoleWriter,
				Prompt: draftRevisionPromptBinding, Schema: draftRevisionSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:draft", "node:review"},
				OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, Condition: "on_quality_issue", FailurePolicy: agentdomain.FailureFailRun},
		},
		Edges: []agentdomain.WorkflowEdge{{From: "plan", To: "draft"}, {From: "draft", To: "review"}, {From: "review", To: "revision"}},
	}
	blueprint := agentdomain.WorkflowDefinition{
		ID: QualityBlueprintWorkflowID, Version: QualityBlueprintWorkflowVersion, TaskKind: agentdomain.TaskStoryBlueprint,
		EntryNodeID: "blueprint", ResultNodeID: "revision", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 1, MaxRunTokens: 11500, RepairReserve: 1000, RevisionReserve: 1000,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "blueprint", Capability: agentdomain.CapabilityStoryArchitect, RoleSlot: agentdomain.RolePlanner,
				Prompt: architectPromptBinding, Schema: blueprintSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context"},
				OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3500, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "review", Capability: agentdomain.CapabilityContinuityReview, RoleSlot: agentdomain.RoleContinuityReviewer,
				Prompt: blueprintReviewPromptBinding, Schema: reviewSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:blueprint"},
				OutputKind: agentdomain.OutputFindings, MaxOutputTokens: 2500, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "revision", Capability: agentdomain.CapabilityBoundedRevision, RoleSlot: agentdomain.RolePlanner,
				Prompt: blueprintRevisionPromptBinding, Schema: blueprintRevisionSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:blueprint", "node:review"},
				OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3500, Condition: "on_quality_issue", FailurePolicy: agentdomain.FailureFailRun},
		},
		Edges: []agentdomain.WorkflowEdge{{From: "blueprint", To: "review"}, {From: "review", To: "revision"}},
	}
	enhanced := agentdomain.WorkflowDefinition{
		ID: EnhancedChapterWorkflowID, Version: EnhancedChapterWorkflowVersion, TaskKind: agentdomain.TaskChapterDraft,
		EntryNodeID: "director", ResultNodeID: "revision", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 1, MaxRunTokens: 18000, RepairReserve: 1000, RevisionReserve: 1000,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "director", PublicStageID: agentdomain.PublicStageChapterDirecting, Capability: agentdomain.CapabilityChapterPlanner, RoleSlot: agentdomain.RolePlanner,
				Prompt: directorPromptBinding, Schema: directorSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context"},
				OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "draft", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilityProseWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: writerPromptBinding, Schema: draftSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:director"},
				OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "review", PublicStageID: agentdomain.PublicStageIndependentReview, Capability: agentdomain.CapabilityContinuityReview, RoleSlot: agentdomain.RoleContinuityReviewer,
				Prompt: chapterReviewPromptBinding, Schema: reviewSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:draft"},
				OutputKind: agentdomain.OutputFindings, MaxOutputTokens: 3000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "revision", PublicStageID: agentdomain.PublicStageBoundedRevision, Capability: agentdomain.CapabilityBoundedRevision, RoleSlot: agentdomain.RoleWriter,
				Prompt: draftRevisionPromptBinding, Schema: draftRevisionSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:draft", "node:review"},
				OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, Condition: "on_quality_issue", FailurePolicy: agentdomain.FailureFailRun},
		},
		Edges: []agentdomain.WorkflowEdge{{From: "director", To: "draft"}, {From: "draft", To: "review"}, {From: "review", To: "revision"}},
	}
	split := agentdomain.WorkflowDefinition{
		ID: SplitReviewWorkflowID, Version: SplitReviewWorkflowVersion, TaskKind: agentdomain.TaskChapterDraft,
		EntryNodeID: "director", ResultNodeID: "revision", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 1, MaxRunTokens: 25000, RepairReserve: 1000, RevisionReserve: 1000,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "director", PublicStageID: agentdomain.PublicStageChapterDirecting, Capability: agentdomain.CapabilityChapterPlanner, RoleSlot: agentdomain.RolePlanner,
				Prompt: directorPromptBinding, Schema: directorSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "draft", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilityProseWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: writerPromptBinding, Schema: draftSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:director"}, OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "continuity", PublicStageID: agentdomain.PublicStageIndependentReview, Capability: agentdomain.CapabilityContinuityReview, RoleSlot: agentdomain.RoleContinuityReviewer,
				Prompt: chapterReviewPromptBinding, Schema: reviewSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:draft"}, OutputKind: agentdomain.OutputFindings, MaxOutputTokens: 3000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "content", PublicStageID: agentdomain.PublicStageIndependentReview, Capability: agentdomain.CapabilityContentReview, RoleSlot: agentdomain.RoleContinuityReviewer,
				Prompt: chapterReviewPromptBinding, Schema: reviewSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:draft"}, OutputKind: agentdomain.OutputFindings, MaxOutputTokens: 3000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "style", PublicStageID: agentdomain.PublicStageIndependentReview, Capability: agentdomain.CapabilityStyleReview, RoleSlot: agentdomain.RoleContinuityReviewer,
				Prompt: chapterReviewPromptBinding, Schema: reviewSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:draft"}, OutputKind: agentdomain.OutputFindings, MaxOutputTokens: 3000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "revision", PublicStageID: agentdomain.PublicStageBoundedRevision, Capability: agentdomain.CapabilityBoundedRevision, RoleSlot: agentdomain.RoleWriter,
				Prompt: draftRevisionPromptBinding, Schema: draftRevisionSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:draft", "node:continuity", "node:content", "node:style"}, OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, Condition: "on_quality_issue", FailurePolicy: agentdomain.FailureFailRun},
		},
		Edges: []agentdomain.WorkflowEdge{{From: "director", To: "draft"}, {From: "draft", To: "continuity"}, {From: "draft", To: "content"}, {From: "draft", To: "style"}, {From: "continuity", To: "revision"}, {From: "content", To: "revision"}, {From: "style", To: "revision"}},
	}
	verifiedChapter := withVerification(chapter, VerifiedChapterWorkflowVersion, agentdomain.WorkflowNode{
		ID: "verify", PublicStageID: agentdomain.PublicStageRevisionVerify, Capability: agentdomain.CapabilityRevisionVerify,
		RoleSlot: agentdomain.RoleContinuityReviewer, Prompt: verifyPromptBinding, Schema: verifySchema, Assembler: assembler,
		InputSelectors: []string{"$task", "$context", "node:draft", "node:review", "node:revision"},
		OutputKind:     agentdomain.OutputFindings, MaxOutputTokens: verifyMaxOutputTokens,
		Condition: "on_quality_issue", FailurePolicy: agentdomain.FailureFailRun,
	})
	verifiedBlueprint := withVerification(blueprint, VerifiedBlueprintWorkflowVersion, agentdomain.WorkflowNode{
		ID: "verify", PublicStageID: agentdomain.PublicStageRevisionVerify, Capability: agentdomain.CapabilityRevisionVerify,
		RoleSlot: agentdomain.RoleContinuityReviewer, Prompt: verifyBlueprintPromptBinding, Schema: verifySchema, Assembler: assembler,
		InputSelectors: []string{"$task", "$context", "node:blueprint", "node:review", "node:revision"},
		OutputKind:     agentdomain.OutputFindings, MaxOutputTokens: verifyMaxOutputTokens,
		Condition: "on_quality_issue", FailurePolicy: agentdomain.FailureFailRun,
	})
	verifiedEnhanced := withVerification(enhanced, VerifiedEnhancedWorkflowVersion, agentdomain.WorkflowNode{
		ID: "verify", PublicStageID: agentdomain.PublicStageRevisionVerify, Capability: agentdomain.CapabilityRevisionVerify,
		RoleSlot: agentdomain.RoleContinuityReviewer, Prompt: verifyPromptBinding, Schema: verifySchema, Assembler: assembler,
		InputSelectors: []string{"$task", "$context", "node:draft", "node:review", "node:revision"},
		OutputKind:     agentdomain.OutputFindings, MaxOutputTokens: verifyMaxOutputTokens,
		Condition: "on_quality_issue", FailurePolicy: agentdomain.FailureFailRun,
	})
	verifiedSplit := withVerification(split, VerifiedSplitReviewWorkflowVersion, agentdomain.WorkflowNode{
		ID: "verify", PublicStageID: agentdomain.PublicStageRevisionVerify, Capability: agentdomain.CapabilityRevisionVerify,
		RoleSlot: agentdomain.RoleContinuityReviewer, Prompt: verifyPromptBinding, Schema: verifySchema, Assembler: assembler,
		InputSelectors: []string{"$task", "$context", "node:draft", "node:continuity", "node:content", "node:style", "node:revision"},
		OutputKind:     agentdomain.OutputFindings, MaxOutputTokens: verifyMaxOutputTokens,
		Condition: "on_quality_issue", FailurePolicy: agentdomain.FailureFailRun,
	})
	workflows := []ports.WorkflowAsset{
		workflowAsset(baseline), workflowAsset(chapter), workflowAsset(blueprint), workflowAsset(enhanced), workflowAsset(split),
		workflowAsset(verifiedChapter), workflowAsset(verifiedBlueprint), workflowAsset(verifiedEnhanced), workflowAsset(verifiedSplit),
	}
	return New(
		workflows,
		[]ports.PromptAsset{
			{Binding: baselinePromptBinding, Template: baselinePrompt, Variables: []string{"Task", "Context"}},
			{Binding: architectPromptBinding, Template: storyArchitectPrompt, Variables: []string{"Task", "Context"}},
			{Binding: planPromptBinding, Template: chapterPlanPrompt, Variables: []string{"Task", "Context"}},
			{Binding: directorPromptBinding, Template: chapterDirectorPrompt, Variables: []string{"Task", "Context"}},
			{Binding: writerPromptBinding, Template: proseWriterPrompt, Variables: []string{"Task", "Context", "Plan"}},
			{Binding: chapterReviewPromptBinding, Template: reviewChapterPrompt, Variables: []string{"Task", "Context", "Draft"}},
			{Binding: blueprintReviewPromptBinding, Template: reviewBlueprintPrompt, Variables: []string{"Task", "Context", "Blueprint"}},
			{Binding: draftRevisionPromptBinding, Template: reviseDraftPrompt, Variables: []string{"Task", "Context", "Draft", "Review"}},
			{Binding: blueprintRevisionPromptBinding, Template: reviseBlueprintPrompt, Variables: []string{"Task", "Context", "Blueprint", "Review"}},
			{Binding: authorDraftPromptBinding, Template: authorReviseDraftPrompt, Variables: []string{"Task", "Context", "Directive", "Draft"}},
			{Binding: authorBlueprintPromptBinding, Template: authorReviseBlueprintPrompt, Variables: []string{"Task", "Context", "Directive", "Blueprint"}},
			{Binding: verifyPromptBinding, Template: verifyRevisionPrompt, Variables: []string{"Task", "Context", "Draft", "Review", "Changes", "Revision"}},
			{Binding: verifyBlueprintPromptBinding, Template: verifyRevisionBlueprintPrompt, Variables: []string{"Task", "Context", "Blueprint", "Review", "Changes", "Revision"}},
		},
		[]ports.SchemaAsset{
			{Binding: draftSchema, JSON: chapterDraftSchema},
			{Binding: blueprintSchema, JSON: storyBlueprintSchema},
			{Binding: planSchema, JSON: chapterPlanSchema},
			{Binding: directorSchema, JSON: chapterDirectorSchema},
			{Binding: reviewSchema, JSON: reviewBundleSchema},
			{Binding: draftRevisionSchema, JSON: reviseDraftSchema},
			{Binding: blueprintRevisionSchema, JSON: reviseBlueprintSchema},
			{Binding: authorDraftSchemaBinding, JSON: authorReviseDraftSchema},
			{Binding: authorBlueprintSchemaBinding, JSON: authorReviseBlueprintSchema},
			{Binding: verifySchema, JSON: verifyRevisionSchema},
		},
		[]ports.AssemblerAsset{{Binding: assembler, JSON: jsonAssembler}},
	)
}

// withVerification 在既有工作流末尾追加修订复核节点，并把复核节点定为结果节点。
// 基础定义按值复制，切片深拷贝，保证已发布的 v1 定义哈希不受影响。
func withVerification(base agentdomain.WorkflowDefinition, version int, verify agentdomain.WorkflowNode) agentdomain.WorkflowDefinition {
	definition := base
	definition.Version = version
	definition.ResultNodeID = verify.ID
	definition.MaxRunTokens = base.MaxRunTokens + verify.MaxOutputTokens
	definition.Nodes = append(append([]agentdomain.WorkflowNode(nil), base.Nodes...), verify)
	definition.Edges = append(append([]agentdomain.WorkflowEdge(nil), base.Edges...), agentdomain.WorkflowEdge{From: base.ResultNodeID, To: verify.ID})
	definition.DefinitionHash = ""
	return definition
}

func workflowAsset(definition agentdomain.WorkflowDefinition) ports.WorkflowAsset {
	definition.DefinitionHash = definition.ContentHash()
	return ports.WorkflowAsset{
		Binding:    agentdomain.VersionBinding{ID: definition.ID, Version: definition.Version, Hash: definition.DefinitionHash},
		Definition: definition,
	}
}
