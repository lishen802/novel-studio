package agentkernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/quality"
)

func TestBuildAuthorRevisionCandidateEnforcesTargetsAndStaleBase(t *testing.T) {
	now := time.Unix(10_000, 0).UTC()
	base := agentdomain.Artifact{ID: "base", RunID: "old-run", StepID: "old-step", Kind: agentdomain.ArtifactDraft, Content: "第一段保持。第二段需要修改。", CreatedAt: now}
	base.SHA256 = agentdomain.SHA256Hex([]byte(base.Content))
	directive := agentdomain.RevisionDirective{Version: 1, Mode: agentdomain.RevisionAuthorDirected, BaseArtifactID: base.ID, BaseDigest: base.SHA256, TargetRefs: []string{"paragraph:2"}, PreserveRefs: []string{"paragraph:1"}, AuthorInstructionRef: "instruction-1"}
	output := &agentdomain.ReviseDraftOutput{Title: "标题", Body: "第一段保持。第二段已经改好。", Changes: []agentdomain.RevisionChange{{TargetRef: "paragraph:2", OriginalText: "第二段需要修改", RevisedText: "第二段已经改好"}}}
	candidate, err := BuildAuthorRevisionCandidate(AuthorRevisionRequest{RunID: "run", StepID: "step", ArtifactID: "candidate", Directive: directive, Base: base, Output: output, CreatedAt: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Relation.RelatedArtifactID != base.ID || candidate.Artifact.Content != output.Body {
		t.Fatalf("candidate = %+v", candidate)
	}

	stale := directive
	stale.BaseDigest = agentdomain.SHA256Hex([]byte("changed"))
	_, err = BuildAuthorRevisionCandidate(AuthorRevisionRequest{RunID: "run", StepID: "step", ArtifactID: "candidate", Directive: stale, Base: base, Output: output, CreatedAt: now})
	if !errors.Is(err, agentdomain.ErrInvalidRevisionDirective) {
		t.Fatalf("stale base error = %v", err)
	}

	outOfScope := *output
	outOfScope.Body = "第一段被改。第二段已经改好。"
	_, err = BuildAuthorRevisionCandidate(AuthorRevisionRequest{RunID: "run", StepID: "step", ArtifactID: "candidate", Directive: directive, Base: base, Output: &outOfScope, CreatedAt: now})
	if err == nil {
		t.Fatal("out-of-scope author revision was accepted")
	}
}

func TestBuildAuthorBlueprintRevisionCandidateKeepsIndependentLineage(t *testing.T) {
	now := time.Unix(20_000, 0).UTC()
	baseContent := `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`
	base := agentdomain.Artifact{ID: "base-blueprint", RunID: "old-run", StepID: "old-step", Kind: agentdomain.ArtifactPlan, Content: baseContent, CreatedAt: now}
	base.SHA256 = agentdomain.SHA256Hex([]byte(base.Content))
	directive := agentdomain.RevisionDirective{Version: 1, Mode: agentdomain.RevisionAuthorDirected, BaseArtifactID: base.ID, BaseDigest: base.SHA256, TargetRefs: []string{"core_conflict"}, PreserveRefs: []string{"world_rules"}, AuthorInstructionRef: "instruction-2"}
	output := &agentdomain.ReviseBlueprintOutput{
		Blueprint: agentdomain.StoryBlueprintOutput{Title: "雾港", Positioning: "悬疑成长", CoreConflict: "追查失踪与守护亲情冲突", ProtagonistArc: "从逃避到承担", WorldRules: []string{"潮汐决定航路"}, MajorMilestones: []string{"失踪", "追查", "抉择"}},
		Changes:   []agentdomain.RevisionChange{{TargetRef: "core_conflict", OriginalText: "追查失踪", RevisedText: "追查失踪与守护亲情冲突"}},
	}
	candidate, err := BuildAuthorRevisionCandidate(AuthorRevisionRequest{RunID: "run", StepID: "step", ArtifactID: "candidate-blueprint", Directive: directive, Base: base, Output: output, CreatedAt: now.Add(time.Second)})
	if err != nil {
		t.Fatal(err)
	}
	var decoded agentdomain.StoryBlueprintOutput
	if json.Unmarshal([]byte(candidate.Artifact.Content), &decoded) != nil || decoded.CoreConflict != output.Blueprint.CoreConflict {
		t.Fatalf("candidate = %+v", candidate)
	}
	if candidate.Relation.Relation != agentdomain.ArtifactRevises || candidate.Relation.RelatedArtifactID != base.ID {
		t.Fatalf("relation = %+v", candidate.Relation)
	}
}

// authorDirectedBase 返回基线方案与其配套的作者定向修订指令。
func authorDirectedBase() (agentdomain.BaseCandidate, agentdomain.RevisionDirective) {
	content := `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`
	base := agentdomain.BaseCandidate{
		ArtifactID: "artifact-base-1", Digest: agentdomain.SHA256Hex([]byte(content)),
		Content: content, Blueprint: true,
	}
	directive := agentdomain.RevisionDirective{
		Version: 1, Mode: agentdomain.RevisionAuthorDirected,
		BaseArtifactID: base.ArtifactID, BaseDigest: base.Digest,
		TargetRefs: []string{"core_conflict"}, PreserveRefs: []string{"world_rules"},
		AuthorInstructionRef: "instruction-1",
	}
	return base, directive
}

// authorDirectedRegistry 构造仅含单节点作者定向修订工作流的最小注册表，
// 避免依赖 builtin 工作流选择器的并行改动；Schema 复用真实的 author_revise_blueprint 资产。
func authorDirectedRegistry(t *testing.T) *agentassets.Registry {
	t.Helper()
	promptText := "{{.Task}}|{{.Context}}|{{.Directive}}|{{.Blueprint}}"
	builtin, err := agentassets.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	schemaAsset, err := builtin.Schema(context.Background(), "author_revise_blueprint.schema", 1)
	if err != nil {
		t.Fatal(err)
	}
	schemaJSON := schemaAsset.JSON
	assemblerJSON := []byte(`{"strategy":"json","version":1}`)
	promptBinding := agentassets.BindingForText("author.directed.prompt", 1, []byte(promptText))
	schemaBinding := agentassets.BindingForText("author.directed.schema", 1, schemaJSON)
	assemblerBinding := agentassets.BindingForText("author.directed.assembler", 1, assemblerJSON)
	definition := agentdomain.WorkflowDefinition{
		ID: "author.directed.revise", Version: 1, TaskKind: agentdomain.TaskReviseBlueprint,
		EntryNodeID: "revise", ResultNodeID: "revise", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 0, MaxRunTokens: 4500, RepairReserve: 0, RevisionReserve: 0,
		Nodes: []agentdomain.WorkflowNode{{
			ID: "revise", PublicStageID: agentdomain.PublicStageBoundedRevision,
			Capability: agentdomain.CapabilityBoundedRevision, RoleSlot: agentdomain.RolePlanner,
			Prompt: promptBinding, Schema: schemaBinding, Assembler: assemblerBinding,
			InputSelectors: []string{"$task", "$context", "$base", "$directive"},
			OutputKind:     agentdomain.OutputStructured, MaxOutputTokens: 3500, FailurePolicy: agentdomain.FailureFailRun,
		}},
	}
	definition.DefinitionHash = definition.ContentHash()
	registry, err := agentassets.New(
		[]ports.WorkflowAsset{{Binding: agentdomain.VersionBinding{ID: definition.ID, Version: 1, Hash: definition.DefinitionHash}, Definition: definition}},
		[]ports.PromptAsset{{Binding: promptBinding, Template: promptText, Variables: []string{"Task", "Context", "Directive", "Blueprint"}}},
		[]ports.SchemaAsset{{Binding: schemaBinding, JSON: schemaJSON}},
		[]ports.AssemblerAsset{{Binding: assemblerBinding, JSON: assemblerJSON}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func authorDirectedRequest(base agentdomain.BaseCandidate, directive agentdomain.RevisionDirective) CreateRequest {
	return CreateRequest{
		RunID: "run-1", ProjectID: "project-1", BranchID: "main", ChapterID: "chapter-1",
		ChapterBaseRevision: 1, CanonicalHead: "head-1", AuthorInstruction: "把核心冲突改为兼顾亲情。",
		WorkflowID: "author.directed.revise", WorkflowVersion: 1,
		Profile: agentdomain.ResolvedCreativeProfile{
			ProfileID: "profile-1", Version: 1, MaxRunTokens: 4500, MaxContextChars: 20000,
			Roles: []agentdomain.ResolvedRole{resolvedRole(agentdomain.RolePlanner, "planner-model")},
		},
		Sources: []Source{}, QualitySpec: quality.Spec{TargetRunes: 0},
		RevisionDirective: &directive, BaseCandidate: &base,
		Deadline: fixedRunnerClock{}.Now().Add(time.Hour),
	}
}

func TestServiceExecutesAuthorDirectedRevisionWithDirectiveAndBase(t *testing.T) {
	base, directive := authorDirectedBase()
	revisedBlueprint := `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪与守护亲情冲突","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`
	registry := authorDirectedRegistry(t)
	provider := &sequenceProvider{replies: []providerReply{{
		text:        `{"blueprint":` + revisedBlueprint + `,"changes":[{"target_ref":"core_conflict","original_text":"追查失踪","revised_text":"追查失踪与守护亲情冲突"}]}`,
		inputTokens: 12, outputTokens: 34,
	}}}
	store := newMemoryStore()
	service := NewService(store, credentialStub{}, providerFactoryStub{provider}, Registries{
		Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
	}, fixedRunnerClock{}, &sequenceIDs{}, nil)

	run, err := service.CreateRun(context.Background(), authorDirectedRequest(base, directive))
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatalf("ExecuteRun: %v", err)
	}
	if completed.Status != agentdomain.RunCompleted || completed.QualityStatus != agentdomain.QualityPassed {
		t.Fatalf("completed run = %#v", completed)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("provider calls = %d", len(provider.requests))
	}
	directiveJSON, marshalErr := json.Marshal(directive)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	rendered := provider.requests[0].Input
	if !strings.Contains(rendered, string(directiveJSON)) {
		t.Fatalf("rendered prompt missing directive JSON: %s", rendered)
	}
	if !strings.Contains(rendered, base.Content) {
		t.Fatalf("rendered prompt missing base content: %s", rendered)
	}
	revision, ok := artifactByID(store.artifacts, completed.Steps[0].PrimaryArtifactID)
	if !ok || !strings.Contains(revision.Content, "追查失踪与守护亲情冲突") {
		t.Fatalf("revision artifact = %#v", revision)
	}
}

func TestAssembleAndRenderPromptBindAuthorDirectedInputs(t *testing.T) {
	base, directive := authorDirectedBase()
	directiveJSON, marshalErr := json.Marshal(directive)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	service := NewService(newMemoryStore(), credentialStub{}, providerFactoryStub{&sequenceProvider{}}, Registries{},
		fixedRunnerClock{}, &sequenceIDs{}, nil)
	run := agentdomain.Run{ID: "run-1", ProjectID: "project-1", CanonicalHead: "head-1", TokenBudget: 4500, ContextCharBudget: 20000}
	step := agentdomain.RunStep{ID: "step-1", RunID: "run-1", NodeID: "revise", Attempt: 1}
	node := FrozenNode{
		ID: "revise", Capability: agentdomain.CapabilityBoundedRevision,
		Assembler:      agentdomain.VersionBinding{ID: "json.assembler", Version: 1},
		InputSelectors: []string{"$task", "$context", "$base", "$directive"},
	}
	input := persistedInput{
		AuthorInstruction: "把核心冲突改为兼顾亲情。", RevisionDirective: &directive, BaseCandidate: &base,
	}
	assembled, manifest, refs, err := service.assemble(run, input, node, step, map[string]agentdomain.RunStep{}, nil)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if !strings.Contains(assembled, string(directiveJSON)) || !strings.Contains(assembled, base.Content) {
		t.Fatalf("assembled context = %s", assembled)
	}
	directiveItem, baseItem := false, false
	for _, item := range manifest.Items {
		if item.SourceID == "revision-directive" && item.SourceKind == agentdomain.ManifestAuthorInstruction &&
			item.SourceVersion == directive.Digest() && item.Priority == 900 && item.Included {
			directiveItem = true
		}
		if item.SourceID == base.ArtifactID && item.SourceKind == agentdomain.ManifestUpstreamArtifact &&
			item.SourceVersion == base.Digest && item.Priority == 850 && item.Included {
			baseItem = true
		}
	}
	if !directiveItem || !baseItem {
		t.Fatalf("manifest items = %#v", manifest.Items)
	}
	if len(refs) != 3 {
		t.Fatalf("refs = %#v", refs)
	}

	prompt := ports.PromptAsset{
		Binding:   agentdomain.VersionBinding{ID: "author.directed.prompt", Version: 1, Hash: agentdomain.SHA256Hex([]byte("{{.Task}}|{{.Context}}|{{.Directive}}|{{.Blueprint}}"))},
		Template:  "{{.Task}}|{{.Context}}|{{.Directive}}|{{.Blueprint}}",
		Variables: []string{"Task", "Context", "Directive", "Blueprint"},
	}
	rendered, err := renderPrompt(prompt, input, node, map[string]agentdomain.RunStep{}, nil, assembled)
	if err != nil {
		t.Fatalf("renderPrompt: %v", err)
	}
	if !strings.Contains(rendered, string(directiveJSON)) || !strings.Contains(rendered, base.Content) {
		t.Fatalf("rendered prompt = %s", rendered)
	}

	// 声明 $base 的节点即使携带 node: 上游，也必须优先渲染冻结基线。
	upstream := map[string]agentdomain.RunStep{"blueprint": {NodeID: "blueprint", Status: agentdomain.StepCompleted, PrimaryArtifactID: "plan-artifact"}}
	upstreamArtifacts := []agentdomain.Artifact{{ID: "plan-artifact", Kind: agentdomain.ArtifactPlan, Content: "上游方案内容"}}
	nodeWithUpstream := node
	nodeWithUpstream.InputSelectors = []string{"$task", "$context", "$base", "$directive", "node:blueprint"}
	priorityRendered, err := renderPrompt(prompt, input, nodeWithUpstream, upstream, upstreamArtifacts, assembled)
	if err != nil {
		t.Fatalf("renderPrompt priority: %v", err)
	}
	if !strings.Contains(priorityRendered, base.Content) || strings.Contains(priorityRendered, "上游方案内容") {
		t.Fatalf("base candidate lost priority: %s", priorityRendered)
	}

	// 未声明 $directive 的节点不能渲染 Directive 变量。
	undeclared := node
	undeclared.InputSelectors = []string{"$task", "$context", "$base"}
	if _, err := renderPrompt(prompt, input, undeclared, upstream, upstreamArtifacts, assembled); err == nil ||
		!strings.Contains(err.Error(), "prompt variable Directive has no input") {
		t.Fatalf("undeclared directive error = %v", err)
	}
	// 声明 $directive 但运行没有指令时同样拒绝。
	missingDirective := input
	missingDirective.RevisionDirective = nil
	if _, err := renderPrompt(prompt, missingDirective, node, upstream, upstreamArtifacts, assembled); err == nil ||
		!strings.Contains(err.Error(), "prompt variable Directive has no input") {
		t.Fatalf("missing directive error = %v", err)
	}
}

func TestCreateRunValidatesAuthorDirectedBaseCandidate(t *testing.T) {
	service, _, _ := newRunnerHarness(t, nil)
	base, directive := authorDirectedBase()
	request := authorDirectedRequest(base, directive)
	request.WorkflowID = agentassets.StoryAuthorReviseWorkflowID
	request.WorkflowVersion = agentassets.StoryAuthorReviseWorkflowVersion

	valid := request
	if _, err := service.CreateRun(context.Background(), valid); err != nil {
		t.Fatalf("valid author-directed request: %v", err)
	}

	missingBase := request
	missingBase.BaseCandidate = nil
	if _, err := service.CreateRun(context.Background(), missingBase); !errors.Is(err, agentdomain.ErrInvalidRevisionDirective) {
		t.Fatalf("missing base error = %v", err)
	}

	staleDigest := request
	staleDigest.BaseCandidate = &agentdomain.BaseCandidate{
		ArtifactID: base.ArtifactID, Digest: agentdomain.SHA256Hex([]byte("changed")), Content: base.Content, Blueprint: true,
	}
	if _, err := service.CreateRun(context.Background(), staleDigest); !errors.Is(err, agentdomain.ErrInvalidRevisionDirective) {
		t.Fatalf("stale digest error = %v", err)
	}

	foreignArtifact := request
	foreignArtifact.BaseCandidate = &agentdomain.BaseCandidate{
		ArtifactID: "artifact-other", Digest: base.Digest, Content: base.Content, Blueprint: true,
	}
	if _, err := service.CreateRun(context.Background(), foreignArtifact); !errors.Is(err, agentdomain.ErrInvalidRevisionDirective) {
		t.Fatalf("foreign artifact error = %v", err)
	}

	blankContent := request
	blankContent.BaseCandidate = &agentdomain.BaseCandidate{ArtifactID: base.ArtifactID, Digest: base.Digest}
	if _, err := service.CreateRun(context.Background(), blankContent); !errors.Is(err, agentdomain.ErrInvalidRevisionDirective) {
		t.Fatalf("blank base error = %v", err)
	}
}
