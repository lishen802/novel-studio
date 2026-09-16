package agentassets

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	"github.com/lishen802/novel-studio/internal/application/ports"
)

func TestRegistryRequiresExactVersionAndReturnsCopies(t *testing.T) {
	content := []byte(`{"type":"object"}`)
	binding := BindingForText("schema", 1, content)
	registry, err := New(nil, nil, []ports.SchemaAsset{{Binding: binding, JSON: content}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	asset, err := registry.Schema(context.Background(), "schema", 1)
	if err != nil {
		t.Fatal(err)
	}
	asset.JSON[0] = '['
	again, err := registry.Schema(context.Background(), "schema", 1)
	if err != nil || again.JSON[0] != '{' {
		t.Fatal("registry leaked mutable asset")
	}
	_, err = registry.Schema(context.Background(), "schema", 2)
	if !errors.Is(err, ports.ErrAgentAssetNotFound) {
		t.Fatalf("expected exact version failure, got %v", err)
	}
}

func TestBuiltinRegistryKeepsSingleCurrentVersionPerEmbeddedAsset(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}

	assertUnique := func(kind string, bindings []key) {
		t.Helper()
		seen := make(map[string]int, len(bindings))
		for _, binding := range bindings {
			if previous, ok := seen[binding.id]; ok {
				t.Errorf("%s %q 同时注册了版本 %d 和 %d", kind, binding.id, previous, binding.version)
			}
			seen[binding.id] = binding.version
		}
	}

	prompts := make([]key, 0, len(registry.prompts))
	for binding := range registry.prompts {
		prompts = append(prompts, binding)
	}
	schemas := make([]key, 0, len(registry.schemas))
	for binding := range registry.schemas {
		schemas = append(schemas, binding)
	}
	assemblers := make([]key, 0, len(registry.assemblers))
	for binding := range registry.assemblers {
		assemblers = append(assemblers, binding)
	}

	assertUnique("Prompt", prompts)
	assertUnique("Schema", schemas)
	assertUnique("Assembler", assemblers)
}

func TestBuiltinWorkflowsCompileWithoutModel(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		id      string
		version int
		nodes   []string
	}{
		{BaselineWorkflowID, BaselineWorkflowVersion, []string{"write"}},
		{QualityChapterWorkflowID, QualityChapterWorkflowVersion, []string{"plan", "draft", "review", "revision"}},
		{QualityBlueprintWorkflowID, QualityBlueprintWorkflowVersion, []string{"blueprint", "review", "revision"}},
		{EnhancedChapterWorkflowID, EnhancedChapterWorkflowVersion, []string{"director", "draft", "review", "revision"}},
		{SplitReviewWorkflowID, SplitReviewWorkflowVersion, []string{"director", "draft", "content", "continuity", "style", "revision"}},
		{QualityChapterWorkflowID, VerifiedChapterWorkflowVersion, []string{"plan", "draft", "review", "revision", "verify"}},
		{QualityBlueprintWorkflowID, VerifiedBlueprintWorkflowVersion, []string{"blueprint", "review", "revision", "verify"}},
		{EnhancedChapterWorkflowID, VerifiedEnhancedWorkflowVersion, []string{"director", "draft", "review", "revision", "verify"}},
		{SplitReviewWorkflowID, VerifiedSplitReviewWorkflowVersion, []string{"director", "draft", "content", "continuity", "style", "revision", "verify"}},
		{SlotParallelWorkflowID, SlotParallelWorkflowVersion, []string{"director", "skeleton", "slots_dialogue", "slots_action", "slots_interiority", "slots_description", "slots_transition", "integrate", "review", "revision"}},
		{StoryDirectionsWorkflowID, StoryProductWorkflowVersion, []string{"blueprint", "review"}},
		{StoryProposalWorkflowID, StoryProductWorkflowVersion, []string{"blueprint", "review", "revision"}},
		{StoryPeopleWorldWorkflowID, StoryPeopleWorldWorkflowVersion, []string{"blueprint", "review"}},
		{StoryRoadmapWorkflowID, StoryRoadmapWorkflowVersion, []string{"blueprint", "review"}},
		{"story.author_roadmap", StoryAuthorRoadmapWorkflowVersion, []string{"blueprint", "review"}},
		{StoryCandidateRevisionWorkflowID, StoryCandidateRevisionWorkflowVersion, []string{"revise", "review"}},
		{StoryCandidateDiscussionWorkflowID, StoryCandidateDiscussionWorkflowVersion, []string{"discuss"}},
		{StoryAuthorReviseWorkflowID, StoryAuthorReviseWorkflowVersion, []string{"revise"}},
	}
	for _, test := range cases {
		compiled, compileErr := agentkernel.Compile(context.Background(), agentkernel.Registries{
			Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
		}, test.id, test.version)
		if compileErr != nil {
			t.Fatalf("compile %s@%d: %v", test.id, test.version, compileErr)
		}
		if len(compiled.Nodes) != len(test.nodes) {
			t.Fatalf("%s nodes: %#v", test.id, compiled.Nodes)
		}
		for index, node := range compiled.Nodes {
			if node.ID != test.nodes[index] {
				t.Fatalf("%s node %d = %s, want %s", test.id, index, node.ID, test.nodes[index])
			}
			if node.PublicStageID == "" {
				t.Fatalf("%s node %s has no public stage", test.id, node.ID)
			}
		}
	}
}

func TestRegistryRejectsDuplicateAssets(t *testing.T) {
	content := []byte(`{}`)
	binding := BindingForText("schema", 1, content)
	_, err := New(nil, nil, []ports.SchemaAsset{{Binding: binding, JSON: content}, {Binding: binding, JSON: content}}, nil)
	if err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestRegistryRejectsContentHashMismatch(t *testing.T) {
	binding := BindingForText("schema", 1, []byte(`{}`))
	_, err := New(nil, nil, []ports.SchemaAsset{{Binding: binding, JSON: []byte(`{"type":"object"}`)}}, nil)
	if err == nil {
		t.Fatal("expected content hash mismatch")
	}
}

func TestBuiltinRegistersAuthorRevisionContracts(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	for _, contract := range []struct {
		prompt string
		schema string
	}{
		{prompt: "author_revise_draft.prompt", schema: "author_revise_draft.schema"},
		{prompt: "author_revise_blueprint.prompt", schema: "author_revise_blueprint.schema"},
	} {
		prompt, promptErr := registry.Prompt(context.Background(), contract.prompt, 1)
		if promptErr != nil || len(prompt.Variables) != 4 {
			t.Fatalf("prompt %s: %+v, %v", contract.prompt, prompt, promptErr)
		}
		schema, schemaErr := registry.Schema(context.Background(), contract.schema, 1)
		if schemaErr != nil || len(schema.JSON) == 0 {
			t.Fatalf("schema %s: %v", contract.schema, schemaErr)
		}
	}
}

func TestSlotParallelKeepsEnhancedDirectorAndDownstreamContract(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	director, _ := registry.Workflow(context.Background(), EnhancedChapterWorkflowID, EnhancedChapterWorkflowVersion)
	slot, _ := registry.Workflow(context.Background(), SlotParallelWorkflowID, SlotParallelWorkflowVersion)
	if !reflect.DeepEqual(director.Definition.Nodes[0], slot.Definition.Nodes[0]) {
		t.Fatalf("director contract differs: %+v vs %+v", director.Definition.Nodes[0], slot.Definition.Nodes[0])
	}
	if slot.Definition.Nodes[len(slot.Definition.Nodes)-2].Capability != director.Definition.Nodes[len(director.Definition.Nodes)-2].Capability ||
		slot.Definition.Nodes[len(slot.Definition.Nodes)-1].Capability != director.Definition.Nodes[len(director.Definition.Nodes)-1].Capability {
		t.Fatal("review or revision contract changed in slot experiment")
	}
}

// TestBuiltinRegistersCurrentPlanningAssets 验证规划流程只注册当前资源合同。
func TestBuiltinRegistersCurrentPlanningAssets(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}

	roadmap, err := registry.Workflow(context.Background(), StoryRoadmapWorkflowID, StoryRoadmapWorkflowVersion)
	if err != nil {
		t.Fatal(err)
	}
	blueprint := roadmap.Definition.Nodes[0]
	if blueprint.Prompt.Version != 3 || blueprint.Schema.Version != 3 {
		t.Fatalf("roadmap current bindings = %+v", blueprint)
	}
	roadmapSchema, err := registry.Schema(context.Background(), "story_roadmap.schema", 3)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`"reader_expectation"`, `"stage_payoff"`, `"irreversible_change"`, `"ending_contribution"`, `"commitments"`, `"retained_questions"`} {
		if !strings.Contains(string(roadmapSchema.JSON), marker) {
			t.Fatalf("roadmap schema missing contract marker %s", marker)
		}
	}

	peopleWorld, err := registry.Workflow(context.Background(), StoryPeopleWorldWorkflowID, StoryPeopleWorldWorkflowVersion)
	if err != nil {
		t.Fatal(err)
	}
	peopleNode := peopleWorld.Definition.Nodes[0]
	if peopleNode.Prompt.Version != 2 || peopleNode.Schema.Version != 2 {
		t.Fatalf("people/world current bindings = %+v", peopleNode)
	}
	castSchema, err := registry.Schema(context.Background(), "story_cast.schema", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`"id"`, `"source_character_ref"`, `"target_character_ref"`} {
		if !strings.Contains(string(castSchema.JSON), marker) {
			t.Fatalf("people/world schema missing %s", marker)
		}
	}

	authorRoadmap, err := registry.Workflow(context.Background(), "story.author_roadmap", StoryAuthorRoadmapWorkflowVersion)
	if err != nil || authorRoadmap.Definition.Nodes[0].Schema.Version != 3 {
		t.Fatalf("author roadmap current contract: %+v, %v", authorRoadmap, err)
	}
	revision, err := registry.Workflow(context.Background(), StoryCandidateRevisionWorkflowID, StoryCandidateRevisionWorkflowVersion)
	if err != nil || revision.Definition.Nodes[0].Schema.Version != 2 {
		t.Fatalf("author revision current contract: %+v, %v", revision, err)
	}
}

func TestBuiltinDoesNotRegisterSupersededPlanningAssets(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}

	obsoleteWorkflows := []struct {
		id      string
		version int
	}{
		{StoryPeopleWorldWorkflowID, 1},
		{StoryRoadmapWorkflowID, 1},
		{StoryRoadmapWorkflowID, 2},
		{"story.author_roadmap", 1},
		{StoryCandidateRevisionWorkflowID, 1},
		{StoryCandidateRevisionWorkflowID, 2},
		{StoryAuthorReviseWorkflowID, 1},
		{"story.author_material", 1},
	}
	for _, obsolete := range obsoleteWorkflows {
		if _, workflowErr := registry.Workflow(context.Background(), obsolete.id, obsolete.version); !errors.Is(workflowErr, ports.ErrAgentAssetNotFound) {
			t.Errorf("obsolete workflow %s@%d error = %v", obsolete.id, obsolete.version, workflowErr)
		}
	}

	for _, version := range []int{1, 2} {
		if _, promptErr := registry.Prompt(context.Background(), "story_roadmap.prompt", version); !errors.Is(promptErr, ports.ErrAgentAssetNotFound) {
			t.Errorf("obsolete roadmap prompt @%d error = %v", version, promptErr)
		}
		if _, schemaErr := registry.Schema(context.Background(), "story_roadmap.schema", version); !errors.Is(schemaErr, ports.ErrAgentAssetNotFound) {
			t.Errorf("obsolete roadmap schema @%d error = %v", version, schemaErr)
		}
	}
}

// TestAuthorReviseDeclaresBaseAndDirectiveSelectors 断言作者修订节点精确声明四个输入选择器，Prompt 变量保持不变。
func TestAuthorReviseDeclaresBaseAndDirectiveSelectors(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	asset, err := registry.Workflow(context.Background(), StoryAuthorReviseWorkflowID, StoryAuthorReviseWorkflowVersion)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"$task", "$context", "$base", "$directive"}
	if !reflect.DeepEqual(asset.Definition.Nodes[0].InputSelectors, want) {
		t.Fatalf("author revise selectors = %#v, want %#v", asset.Definition.Nodes[0].InputSelectors, want)
	}
	prompt, err := registry.Prompt(context.Background(), "author_revise_blueprint.prompt", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(prompt.Variables, []string{"Task", "Context", "Directive", "Blueprint"}) {
		t.Fatalf("author revise prompt variables = %#v", prompt.Variables)
	}
}
