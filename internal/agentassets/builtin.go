package agentassets

import (
	_ "embed"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

const (
	BaselineWorkflowID                = "baseline.chapter"
	BaselineWorkflowVersion           = 1
	QualityChapterWorkflowID          = "quality.chapter"
	QualityChapterWorkflowVersion     = 1
	QualityBlueprintWorkflowID        = "quality.blueprint"
	QualityBlueprintWorkflowVersion   = 1
	EnhancedChapterWorkflowID         = "quality.chapter.director"
	EnhancedChapterWorkflowVersion    = 1
	SplitReviewWorkflowID             = "quality.chapter.split-review"
	SplitReviewWorkflowVersion        = 1
	SlotParallelWorkflowID            = "quality.chapter.slot-parallel"
	SlotParallelWorkflowVersion       = 1
	StoryDirectionsWorkflowID         = "story.directions"
	StoryDirectionOperationWorkflowID = "story.direction_operation"
	StoryProposalWorkflowID           = "story.proposal"
	StoryPeopleWorldWorkflowID        = "story.people_world"
	StoryRoadmapWorkflowID            = "story.roadmap"
	StoryAuthorReviseWorkflowID       = "story.author_revise"
	StoryProductWorkflowVersion       = 1
	StoryProposalWorkflowVersion      = 2
)

// v2 在既有工作流末尾追加修订复核节点；v1 保留不变，供 Harness 判断复核节点是否值得这次调用。
const (
	VerifiedChapterWorkflowVersion     = 2
	VerifiedBlueprintWorkflowVersion   = 2
	VerifiedEnhancedWorkflowVersion    = 2
	VerifiedSplitReviewWorkflowVersion = 2
	verifyMaxOutputTokens              = 2500
)

// StoryRoadmapWorkflowVersion 标识当前路线图工作流合同。
const StoryRoadmapWorkflowVersion = 4

// StoryAuthorRoadmapWorkflowVersion 标识当前已有方案路线图工作流合同。
const StoryAuthorRoadmapWorkflowVersion = 2

// StoryPeopleWorldWorkflowVersion 标识当前人物世界工作流合同。
const StoryPeopleWorldWorkflowVersion = 3

// StoryAuthorReviseWorkflowVersion 标识当前显式基线修订合同。
const StoryAuthorReviseWorkflowVersion = 2

var (
	//go:embed assets/prompts/baseline_chapter.tmpl
	baselinePrompt string
	//go:embed assets/prompts/story_architect.tmpl
	storyArchitectPrompt string
	//go:embed assets/prompts/story_directions.tmpl
	storyDirectionsPrompt string
	//go:embed assets/prompts/story_direction_operation.tmpl
	storyDirectionOperationPrompt string
	//go:embed assets/prompts/story_proposal.tmpl
	storyProposalPrompt string
	//go:embed assets/prompts/story_cast.tmpl
	storyCastPrompt string
	//go:embed assets/prompts/story_roadmap.tmpl
	storyRoadmapPrompt string
	//go:embed assets/prompts/chapter_plan.tmpl
	chapterPlanPrompt string
	//go:embed assets/prompts/chapter_director.tmpl
	chapterDirectorPrompt string
	//go:embed assets/prompts/prose_writer.tmpl
	proseWriterPrompt string
	//go:embed assets/prompts/review_chapter.tmpl
	reviewChapterPrompt string
	//go:embed assets/prompts/review_blueprint.tmpl
	reviewBlueprintPrompt string
	//go:embed assets/prompts/revise_draft.tmpl
	reviseDraftPrompt string
	//go:embed assets/prompts/revise_blueprint.tmpl
	reviseBlueprintPrompt string
	//go:embed assets/prompts/author_revise_draft.tmpl
	authorReviseDraftPrompt string
	//go:embed assets/prompts/author_revise_blueprint.tmpl
	authorReviseBlueprintPrompt string
	//go:embed assets/prompts/verify_revision.tmpl
	verifyRevisionPrompt string
	//go:embed assets/prompts/verify_revision_blueprint.tmpl
	verifyRevisionBlueprintPrompt string
	//go:embed assets/prompts/chapter_skeleton.tmpl
	chapterSkeletonPrompt string
	//go:embed assets/prompts/fill_slot_dialogue.tmpl
	fillSlotDialoguePrompt string
	//go:embed assets/prompts/fill_slot_action.tmpl
	fillSlotActionPrompt string
	//go:embed assets/prompts/fill_slot_interiority.tmpl
	fillSlotInteriorityPrompt string
	//go:embed assets/prompts/fill_slot_description.tmpl
	fillSlotDescriptionPrompt string
	//go:embed assets/prompts/fill_slot_transition.tmpl
	fillSlotTransitionPrompt string
	//go:embed assets/prompts/integrate_slots.tmpl
	integrateSlotsPrompt string
	//go:embed assets/schemas/chapter_draft.json
	chapterDraftSchema []byte
	//go:embed assets/schemas/story_blueprint.json
	storyBlueprintSchema []byte
	//go:embed assets/schemas/story_architect.json
	storyArchitectSchema []byte
	//go:embed assets/schemas/story_directions.json
	storyDirectionsSchema []byte
	//go:embed assets/schemas/story_direction.json
	storyDirectionSchema []byte
	//go:embed assets/schemas/story_cast.json
	storyCastSchema []byte
	//go:embed assets/schemas/story_roadmap.json
	storyRoadmapSchema []byte
	//go:embed assets/schemas/chapter_plan.json
	chapterPlanSchema []byte
	//go:embed assets/schemas/chapter_director.json
	chapterDirectorSchema []byte
	//go:embed assets/schemas/review_bundle.json
	reviewBundleSchema []byte
	//go:embed assets/schemas/revise_draft.json
	reviseDraftSchema []byte
	//go:embed assets/schemas/revise_blueprint.json
	reviseBlueprintSchema []byte
	//go:embed assets/schemas/author_revise_draft.json
	authorReviseDraftSchema []byte
	//go:embed assets/schemas/author_revise_blueprint.json
	authorReviseBlueprintSchema []byte
	//go:embed assets/schemas/verify_revision.json
	verifyRevisionSchema []byte
	//go:embed assets/schemas/chapter_skeleton.json
	chapterSkeletonSchema []byte
	//go:embed assets/schemas/fill_slot.json
	fillSlotSchema []byte
	//go:embed assets/assemblers/json.json
	jsonAssembler []byte
)

//go:embed assets/prompts/author_material.tmpl
var authorMaterialPrompt string

//go:embed assets/schemas/author_material.json
var authorMaterialSchema []byte

// Builtin 构造应用内置资源集合；开发阶段每种 Prompt、Schema 和 Assembler 只保留当前版本。
func Builtin() (*Registry, error) {
	candidateRevisionPromptBinding := BindingForText("story_candidate_revision.prompt", 2, []byte(candidateRevisionPrompt))
	candidateRevisionReviewBinding := BindingForText("review_candidate_revision.prompt", 1, []byte(candidateRevisionReviewPrompt))
	candidateRevisionSchemaBinding := BindingForText("story_candidate_revision.schema", 2, candidateRevisionSchema)
	candidateDiscussionPromptBinding := BindingForText("story_candidate_discussion.prompt", 1, []byte(candidateDiscussionPrompt))
	candidateDiscussionSchemaBinding := BindingForText("story_candidate_discussion.schema", 1, candidateDiscussionSchema)
	materialPromptBinding := BindingForText("author_material.prompt", 2, []byte(authorMaterialPrompt))
	materialSchemaBinding := BindingForText("author_material.schema", 2, authorMaterialSchema)
	fillPromptBinding := BindingForText("author_plan_fill.prompt", 1, []byte(authorPlanFillPrompt))
	baselinePromptBinding := BindingForText("baseline.chapter.prompt", 1, []byte(baselinePrompt))
	architectPromptBinding := BindingForText("story_architect.prompt", 1, []byte(storyArchitectPrompt))
	directionsPromptBinding := BindingForText("story_directions.prompt", 1, []byte(storyDirectionsPrompt))
	directionOperationPromptBinding := BindingForText("story_direction_operation.prompt", 1, []byte(storyDirectionOperationPrompt))
	proposalPromptBinding := BindingForText("story_proposal.prompt", 3, []byte(storyProposalPrompt))
	castPromptBinding := BindingForText("story_cast.prompt", 3, []byte(storyCastPrompt))
	roadmapPromptBinding := BindingForText("story_roadmap.prompt", 4, []byte(storyRoadmapPrompt))
	planPromptBinding := BindingForText("chapter_plan.prompt", 1, []byte(chapterPlanPrompt))
	directorPromptBinding := BindingForText("chapter_director.prompt", 1, []byte(chapterDirectorPrompt))
	writerPromptBinding := BindingForText("prose_writer.prompt", 1, []byte(proseWriterPrompt))
	chapterReviewPromptBinding := BindingForText("review_chapter.prompt", 1, []byte(reviewChapterPrompt))
	blueprintReviewPromptBinding := BindingForText("review_blueprint.prompt", 1, []byte(reviewBlueprintPrompt))
	draftRevisionPromptBinding := BindingForText("revise_draft.prompt", 1, []byte(reviseDraftPrompt))
	blueprintRevisionPromptBinding := BindingForText("revise_blueprint.prompt", 1, []byte(reviseBlueprintPrompt))
	authorDraftPromptBinding := BindingForText("author_revise_draft.prompt", 1, []byte(authorReviseDraftPrompt))
	authorBlueprintPromptBinding := BindingForText("author_revise_blueprint.prompt", 1, []byte(authorReviseBlueprintPrompt))
	draftSchema := BindingForText("chapter_draft.schema", 1, chapterDraftSchema)
	blueprintSchema := BindingForText("story_blueprint.schema", 2, storyBlueprintSchema)
	architectSchema := BindingForText("story_architect.schema", 1, storyArchitectSchema)
	directionsSchema := BindingForText("story_directions.schema", 1, storyDirectionsSchema)
	directionSchema := BindingForText("story_direction.schema", 1, storyDirectionSchema)
	castSchema := BindingForText("story_cast.schema", 2, storyCastSchema)
	roadmapSchema := BindingForText("story_roadmap.schema", 3, storyRoadmapSchema)
	planSchema := BindingForText("chapter_plan.schema", 1, chapterPlanSchema)
	directorSchema := BindingForText("chapter_director.schema", 1, chapterDirectorSchema)
	reviewSchema := BindingForText("review_bundle.schema", 1, reviewBundleSchema)
	draftRevisionSchema := BindingForText("revise_draft.schema", 1, reviseDraftSchema)
	blueprintRevisionSchema := BindingForText("revise_blueprint.schema", 1, reviseBlueprintSchema)
	authorDraftSchemaBinding := BindingForText("author_revise_draft.schema", 1, authorReviseDraftSchema)
	authorBlueprintSchemaBinding := BindingForText("author_revise_blueprint.schema", 1, authorReviseBlueprintSchema)
	verifyPromptBinding := BindingForText("verify_revision.prompt", 1, []byte(verifyRevisionPrompt))
	verifyBlueprintPromptBinding := BindingForText("verify_revision_blueprint.prompt", 1, []byte(verifyRevisionBlueprintPrompt))
	verifySchema := BindingForText("verify_revision.schema", 1, verifyRevisionSchema)
	skeletonPromptBinding := BindingForText("chapter_skeleton.prompt", 1, []byte(chapterSkeletonPrompt))
	dialoguePromptBinding := BindingForText("fill_slot.dialogue.prompt", 1, []byte(fillSlotDialoguePrompt))
	actionPromptBinding := BindingForText("fill_slot.action.prompt", 1, []byte(fillSlotActionPrompt))
	interiorityPromptBinding := BindingForText("fill_slot.interiority.prompt", 1, []byte(fillSlotInteriorityPrompt))
	descriptionPromptBinding := BindingForText("fill_slot.description.prompt", 1, []byte(fillSlotDescriptionPrompt))
	transitionPromptBinding := BindingForText("fill_slot.transition.prompt", 1, []byte(fillSlotTransitionPrompt))
	integrationPromptBinding := BindingForText("integrate_slots.prompt", 1, []byte(integrateSlotsPrompt))
	skeletonSchemaBinding := BindingForText("chapter_skeleton.schema", 1, chapterSkeletonSchema)
	fillSlotSchemaBinding := BindingForText("fill_slot.schema", 1, fillSlotSchema)
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
				Prompt: architectPromptBinding, Schema: architectSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context"},
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
	slotParallel := agentdomain.WorkflowDefinition{
		ID: SlotParallelWorkflowID, Version: SlotParallelWorkflowVersion, TaskKind: agentdomain.TaskChapterDraft,
		EntryNodeID: "director", ResultNodeID: "revision", BudgetPolicyVersion: 1, QualityPolicyVersion: 2,
		MaxRevisionAttempts: 1, MaxRunTokens: 40000, RepairReserve: 1000, RevisionReserve: 1000,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "director", PublicStageID: agentdomain.PublicStageChapterDirecting, Capability: agentdomain.CapabilityChapterPlanner, RoleSlot: agentdomain.RolePlanner,
				Prompt: directorPromptBinding, Schema: directorSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "skeleton", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilitySkeletonWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: skeletonPromptBinding, Schema: skeletonSchemaBinding, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:director"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 2500, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "slots_dialogue", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilitySlotWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: dialoguePromptBinding, Schema: fillSlotSchemaBinding, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:skeleton"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3500, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "slots_action", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilitySlotWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: actionPromptBinding, Schema: fillSlotSchemaBinding, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:skeleton"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3500, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "slots_interiority", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilitySlotWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: interiorityPromptBinding, Schema: fillSlotSchemaBinding, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:skeleton"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3500, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "slots_description", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilitySlotWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: descriptionPromptBinding, Schema: fillSlotSchemaBinding, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:skeleton"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3500, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "slots_transition", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilitySlotWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: transitionPromptBinding, Schema: fillSlotSchemaBinding, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:skeleton"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 3500, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "integrate", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilityIntegrationWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: integrationPromptBinding, Schema: draftSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:skeleton", "node:slots_dialogue", "node:slots_action", "node:slots_interiority", "node:slots_description", "node:slots_transition", "$slot_join"}, OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "review", PublicStageID: agentdomain.PublicStageIndependentReview, Capability: agentdomain.CapabilityContinuityReview, RoleSlot: agentdomain.RoleContinuityReviewer,
				Prompt: chapterReviewPromptBinding, Schema: reviewSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:integrate"}, OutputKind: agentdomain.OutputFindings, MaxOutputTokens: 3000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "revision", PublicStageID: agentdomain.PublicStageBoundedRevision, Capability: agentdomain.CapabilityBoundedRevision, RoleSlot: agentdomain.RoleWriter,
				Prompt: draftRevisionPromptBinding, Schema: draftRevisionSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:integrate", "node:review"}, OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 5000, Condition: "on_quality_issue", FailurePolicy: agentdomain.FailureFailRun},
		},
		Edges: []agentdomain.WorkflowEdge{
			{From: "director", To: "skeleton"}, {From: "skeleton", To: "slots_dialogue"}, {From: "slots_dialogue", To: "slots_action"},
			{From: "slots_action", To: "slots_interiority"}, {From: "slots_interiority", To: "slots_description"}, {From: "slots_description", To: "slots_transition"},
			{From: "slots_transition", To: "integrate"}, {From: "integrate", To: "review"}, {From: "review", To: "revision"},
		},
	}
	directions := productGenerateAndReview(
		StoryDirectionsWorkflowID, StoryProductWorkflowVersion, agentdomain.TaskStoryDirections,
		directionsPromptBinding, directionsSchema, assembler, blueprintReviewPromptBinding, reviewSchema, 3500, 7000,
	)
	directionOperation := agentdomain.WorkflowDefinition{
		ID: StoryDirectionOperationWorkflowID, Version: 1, TaskKind: agentdomain.TaskStoryDirection,
		EntryNodeID: "direction", ResultNodeID: "direction", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 0, MaxRunTokens: 3500,
		Nodes: []agentdomain.WorkflowNode{{
			ID: "direction", Capability: agentdomain.CapabilityStoryArchitect, RoleSlot: agentdomain.RolePlanner,
			Prompt: directionOperationPromptBinding, Schema: directionSchema, Assembler: assembler,
			InputSelectors: []string{"$task", "$context"}, OutputKind: agentdomain.OutputStructured,
			MaxOutputTokens: 2500, FailurePolicy: agentdomain.FailureFailRun,
		}},
	}
	proposal := agentdomain.WorkflowDefinition{
		ID: StoryProposalWorkflowID, Version: StoryProposalWorkflowVersion, TaskKind: agentdomain.TaskStoryBlueprint,
		EntryNodeID: "blueprint", ResultNodeID: "revision", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 1, MaxRunTokens: 11500, RepairReserve: 1000, RevisionReserve: 1000,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "blueprint", Capability: agentdomain.CapabilityStoryArchitect, RoleSlot: agentdomain.RolePlanner,
				Prompt: proposalPromptBinding, Schema: blueprintSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context"},
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
	peopleWorld := productGenerateAndReview(
		StoryPeopleWorldWorkflowID, StoryPeopleWorldWorkflowVersion, agentdomain.TaskStoryCast,
		castPromptBinding, castSchema, assembler, blueprintReviewPromptBinding, reviewSchema, 4000, 7500,
	)
	roadmap := productGenerateAndReview(
		StoryRoadmapWorkflowID, StoryRoadmapWorkflowVersion, agentdomain.TaskStoryRoadmap,
		roadmapPromptBinding, roadmapSchema, assembler, blueprintReviewPromptBinding, reviewSchema, 5000, 10000,
	)
	authorRevise := agentdomain.WorkflowDefinition{
		ID: StoryAuthorReviseWorkflowID, Version: StoryAuthorReviseWorkflowVersion, TaskKind: agentdomain.TaskReviseBlueprint,
		EntryNodeID: "revise", ResultNodeID: "revise", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 0, MaxRunTokens: 4500, RepairReserve: 0, RevisionReserve: 0,
		Nodes: []agentdomain.WorkflowNode{{
			ID: "revise", PublicStageID: agentdomain.PublicStageBoundedRevision,
			Capability: agentdomain.CapabilityBoundedRevision, RoleSlot: agentdomain.RolePlanner,
			Prompt: authorBlueprintPromptBinding, Schema: authorBlueprintSchemaBinding, Assembler: assembler,
			InputSelectors: []string{"$task", "$context", "$base", "$directive"}, OutputKind: agentdomain.OutputStructured,
			MaxOutputTokens: 3500, FailurePolicy: agentdomain.FailureFailRun,
		}},
	}
	materialWorkflow := productGenerateAndReview("story.author_material", 2, agentdomain.TaskStoryBlueprint,
		materialPromptBinding, materialSchemaBinding, assembler, blueprintReviewPromptBinding, reviewSchema, 12000, 30000)
	authorProposal := productGenerateAndReview("story.author_proposal", 1, agentdomain.TaskStoryBlueprint,
		fillPromptBinding, blueprintSchema, assembler, blueprintReviewPromptBinding, reviewSchema, 4000, 8000)
	authorPeople := productGenerateAndReview("story.author_people_world", 1, agentdomain.TaskStoryCast,
		fillPromptBinding, castSchema, assembler, blueprintReviewPromptBinding, reviewSchema, 4000, 8000)
	authorRoadmap := productGenerateAndReview("story.author_roadmap", StoryAuthorRoadmapWorkflowVersion, agentdomain.TaskStoryRoadmap,
		fillPromptBinding, roadmapSchema, assembler, blueprintReviewPromptBinding, reviewSchema, 5000, 9000)
	workflows := []ports.WorkflowAsset{
		workflowAsset(candidateRevisionWorkflow(StoryCandidateRevisionWorkflowVersion, candidateRevisionPromptBinding, candidateRevisionReviewBinding, candidateRevisionSchemaBinding, reviewSchema, assembler)),
		workflowAsset(candidateDiscussionWorkflow(candidateDiscussionPromptBinding, candidateDiscussionSchemaBinding, assembler)),
		workflowAsset(materialWorkflow), workflowAsset(authorProposal), workflowAsset(authorPeople), workflowAsset(authorRoadmap),
		workflowAsset(baseline), workflowAsset(chapter), workflowAsset(blueprint), workflowAsset(enhanced), workflowAsset(split),
		workflowAsset(verifiedChapter), workflowAsset(verifiedBlueprint), workflowAsset(verifiedEnhanced), workflowAsset(verifiedSplit), workflowAsset(slotParallel),
		workflowAsset(directions), workflowAsset(directionOperation), workflowAsset(proposal), workflowAsset(peopleWorld), workflowAsset(roadmap), workflowAsset(authorRevise),
	}
	return New(
		workflows,
		[]ports.PromptAsset{
			{Binding: candidateRevisionPromptBinding, Template: candidateRevisionPrompt, Variables: []string{"Task", "Context", "Directive", "Blueprint"}},
			{Binding: candidateRevisionReviewBinding, Template: candidateRevisionReviewPrompt, Variables: []string{"Task", "Context", "Directive", "Blueprint", "Revision"}},
			{Binding: candidateDiscussionPromptBinding, Template: candidateDiscussionPrompt, Variables: []string{"Task", "Context"}},
			{Binding: materialPromptBinding, Template: authorMaterialPrompt, Variables: []string{"Task", "Context"}},
			{Binding: fillPromptBinding, Template: authorPlanFillPrompt, Variables: []string{"Task", "Context"}},
			{Binding: baselinePromptBinding, Template: baselinePrompt, Variables: []string{"Task", "Context"}},
			{Binding: architectPromptBinding, Template: storyArchitectPrompt, Variables: []string{"Task", "Context"}},
			{Binding: directionsPromptBinding, Template: storyDirectionsPrompt, Variables: []string{"Task", "Context"}},
			{Binding: directionOperationPromptBinding, Template: storyDirectionOperationPrompt, Variables: []string{"Task", "Context"}},
			{Binding: proposalPromptBinding, Template: storyProposalPrompt, Variables: []string{"Task", "Context"}},
			{Binding: castPromptBinding, Template: storyCastPrompt, Variables: []string{"Task", "Context"}},
			{Binding: roadmapPromptBinding, Template: storyRoadmapPrompt, Variables: []string{"Task", "Context"}},
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
			{Binding: skeletonPromptBinding, Template: chapterSkeletonPrompt, Variables: []string{"Task", "Context"}},
			{Binding: dialoguePromptBinding, Template: fillSlotDialoguePrompt, Variables: []string{"Task", "Context", "Skeleton"}},
			{Binding: actionPromptBinding, Template: fillSlotActionPrompt, Variables: []string{"Task", "Context", "Skeleton"}},
			{Binding: interiorityPromptBinding, Template: fillSlotInteriorityPrompt, Variables: []string{"Task", "Context", "Skeleton"}},
			{Binding: descriptionPromptBinding, Template: fillSlotDescriptionPrompt, Variables: []string{"Task", "Context", "Skeleton"}},
			{Binding: transitionPromptBinding, Template: fillSlotTransitionPrompt, Variables: []string{"Task", "Context", "Skeleton"}},
			{Binding: integrationPromptBinding, Template: integrateSlotsPrompt, Variables: []string{"Task", "Context", "Skeleton", "MechanicalJoin"}},
		},
		[]ports.SchemaAsset{
			{Binding: candidateRevisionSchemaBinding, JSON: candidateRevisionSchema},
			{Binding: candidateDiscussionSchemaBinding, JSON: candidateDiscussionSchema},
			{Binding: materialSchemaBinding, JSON: authorMaterialSchema},
			{Binding: draftSchema, JSON: chapterDraftSchema},
			{Binding: blueprintSchema, JSON: storyBlueprintSchema},
			{Binding: architectSchema, JSON: storyArchitectSchema},
			{Binding: directionsSchema, JSON: storyDirectionsSchema},
			{Binding: directionSchema, JSON: storyDirectionSchema},
			{Binding: castSchema, JSON: storyCastSchema},
			{Binding: roadmapSchema, JSON: storyRoadmapSchema},
			{Binding: planSchema, JSON: chapterPlanSchema},
			{Binding: directorSchema, JSON: chapterDirectorSchema},
			{Binding: reviewSchema, JSON: reviewBundleSchema},
			{Binding: draftRevisionSchema, JSON: reviseDraftSchema},
			{Binding: blueprintRevisionSchema, JSON: reviseBlueprintSchema},
			{Binding: authorDraftSchemaBinding, JSON: authorReviseDraftSchema},
			{Binding: authorBlueprintSchemaBinding, JSON: authorReviseBlueprintSchema},
			{Binding: verifySchema, JSON: verifyRevisionSchema},
			{Binding: skeletonSchemaBinding, JSON: chapterSkeletonSchema},
			{Binding: fillSlotSchemaBinding, JSON: fillSlotSchema},
		},
		[]ports.AssemblerAsset{{Binding: assembler, JSON: jsonAssembler}},
	)
}

// withVerification 在既有工作流末尾追加修订复核节点，并把复核节点定为结果节点。
// 基础定义按值复制并深拷贝切片，避免不同 Harness 变体互相修改。
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

// productGenerateAndReview 发布方向/人物世界/路线图工作流：架构生成后独立审阅，尚无专用修订 Schema。
func productGenerateAndReview(
	id string,
	version int,
	taskKind agentdomain.TaskKind,
	architectPrompt, schema, assembler, reviewPrompt, reviewSchema agentdomain.VersionBinding,
	architectTokens, maxRunTokens int,
) agentdomain.WorkflowDefinition {
	return agentdomain.WorkflowDefinition{
		ID: id, Version: version, TaskKind: taskKind,
		EntryNodeID: "blueprint", ResultNodeID: "review", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 0, MaxRunTokens: maxRunTokens, RepairReserve: 1000, RevisionReserve: 0,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "blueprint", Capability: agentdomain.CapabilityStoryArchitect, RoleSlot: agentdomain.RolePlanner,
				Prompt: architectPrompt, Schema: schema, Assembler: assembler, InputSelectors: []string{"$task", "$context"},
				OutputKind: agentdomain.OutputStructured, MaxOutputTokens: architectTokens, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "review", Capability: agentdomain.CapabilityContinuityReview, RoleSlot: agentdomain.RoleContinuityReviewer,
				Prompt: reviewPrompt, Schema: reviewSchema, Assembler: assembler, InputSelectors: []string{"$task", "$context", "node:blueprint"},
				OutputKind: agentdomain.OutputFindings, MaxOutputTokens: 2500, FailurePolicy: agentdomain.FailureFailRun},
		},
		Edges: []agentdomain.WorkflowEdge{{From: "blueprint", To: "review"}},
	}
}

func workflowAsset(definition agentdomain.WorkflowDefinition) ports.WorkflowAsset {
	definition.DefinitionHash = definition.ContentHash()
	return ports.WorkflowAsset{
		Binding:    agentdomain.VersionBinding{ID: definition.ID, Version: definition.Version, Hash: definition.DefinitionHash},
		Definition: definition,
	}
}

var (
	//go:embed assets/prompts/author_plan_fill.tmpl
	authorPlanFillPrompt string
)
