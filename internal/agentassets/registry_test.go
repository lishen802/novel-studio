package agentassets

import (
	"context"
	"errors"
	"fmt"
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

func TestBuiltinWorkflowsCompileWithoutModel(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	legacyHashes := map[string]string{
		BaselineWorkflowID + "@1":         "d3b99323219a347153d613743dad46e50f5dfe7f060c8b531464b61c845f2b5b",
		QualityChapterWorkflowID + "@1":   "8cd791a01ce61989c61f4c4f057cb040cd0f6d1e28f06ef116e5022285582a68",
		QualityBlueprintWorkflowID + "@1": "2ae580fe57ef8ad6960717925b098c545359a419c8b0406d8533d25410fadeba",
		StoryPeopleWorldWorkflowID + "@1": "c65e4f4e77e021de2f773f0dc0cbc75e92da40afed8f24e2b566894cb1018205",
		// roadmap v1 哈希在引入 v2 前采集：定义哈希涵盖 Prompt/Schema 绑定哈希，不变即证明 v1 资产未被改动。
		StoryRoadmapWorkflowID + "@1": "e83a2df35196daf7d425d661098bbcad0973029d5982b97963d10522909c51f1",
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
		{StoryPeopleWorldWorkflowID, StoryProductWorkflowVersion, []string{"blueprint", "review"}},
		{StoryPeopleWorldWorkflowID, StoryPeopleWorldWorkflowVersionV2, []string{"blueprint", "review"}},
		{StoryRoadmapWorkflowID, StoryProductWorkflowVersion, []string{"blueprint", "review"}},
		{StoryRoadmapWorkflowID, StoryRoadmapWorkflowVersionV2, []string{"blueprint", "review"}},
		{StoryAuthorReviseWorkflowID, StoryProductWorkflowVersion, []string{"revise"}},
		{StoryAuthorReviseWorkflowID, StoryAuthorReviseWorkflowVersion, []string{"revise"}},
	}
	for _, test := range cases {
		compiled, compileErr := agentkernel.Compile(context.Background(), agentkernel.Registries{
			Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
		}, test.id, test.version)
		if compileErr != nil {
			t.Fatalf("compile %s@%d: %v", test.id, test.version, compileErr)
		}
		if want, ok := legacyHashes[fmt.Sprintf("%s@%d", test.id, test.version)]; ok && compiled.Workflow.Hash != want {
			t.Fatalf("%s hash = %s, want immutable v1 hash %s", test.id, compiled.Workflow.Hash, want)
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

// TestBuiltinRegistersRoadmapV2AlongsideImmutableV1 验证 roadmap v2 与 v1 并存注册：
// v2 生成节点绑定 v2 Prompt/Schema，v1 定义哈希保持引入 v2 之前的原值。
func TestBuiltinRegistersRoadmapV2AlongsideImmutableV1(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	v1, err := registry.Workflow(context.Background(), StoryRoadmapWorkflowID, StoryProductWorkflowVersion)
	if err != nil {
		t.Fatalf("roadmap v1: %v", err)
	}
	if v1.Definition.DefinitionHash != "e83a2df35196daf7d425d661098bbcad0973029d5982b97963d10522909c51f1" {
		t.Fatalf("roadmap v1 hash = %s, want immutable pre-v2 hash", v1.Definition.DefinitionHash)
	}
	v2, err := registry.Workflow(context.Background(), StoryRoadmapWorkflowID, StoryRoadmapWorkflowVersionV2)
	if err != nil {
		t.Fatalf("roadmap v2: %v", err)
	}
	if v2.Definition.DefinitionHash == v1.Definition.DefinitionHash {
		t.Fatal("roadmap v2 must not reuse v1 definition hash")
	}
	v2Blueprint := v2.Definition.Nodes[0]
	if v2Blueprint.ID != "blueprint" || v2Blueprint.Prompt.Version != 2 || v2Blueprint.Schema.Version != 2 {
		t.Fatalf("roadmap v2 blueprint node must bind v2 prompt/schema: %+v", v2Blueprint)
	}
	v1Blueprint := v1.Definition.Nodes[0]
	if v1Blueprint.Prompt.Version != 1 || v1Blueprint.Schema.Version != 1 {
		t.Fatalf("roadmap v1 blueprint node must keep v1 bindings: %+v", v1Blueprint)
	}
	v2Prompt, err := registry.Prompt(context.Background(), "story_roadmap.prompt", 2)
	if err != nil || !reflect.DeepEqual(v2Prompt.Variables, []string{"Task", "Context"}) {
		t.Fatalf("roadmap v2 prompt: %+v, %v", v2Prompt, err)
	}
	v2Schema, err := registry.Schema(context.Background(), "story_roadmap.schema", 2)
	if err != nil || len(v2Schema.JSON) == 0 {
		t.Fatalf("roadmap v2 schema: %v", err)
	}
	for _, marker := range []string{`"id"`, `"arc_ref"`, `"strength"`} {
		if !strings.Contains(string(v2Schema.JSON), marker) {
			t.Fatalf("roadmap v2 schema missing contract marker %s", marker)
		}
	}
}

func TestBuiltinRegistersPeopleWorldV2WithStableRelationshipReferences(t *testing.T) {
	registry, err := Builtin()
	if err != nil {
		t.Fatal(err)
	}
	v1, err := registry.Workflow(context.Background(), StoryPeopleWorldWorkflowID, StoryProductWorkflowVersion)
	if err != nil {
		t.Fatalf("people/world v1: %v", err)
	}
	v2, err := registry.Workflow(context.Background(), StoryPeopleWorldWorkflowID, StoryPeopleWorldWorkflowVersionV2)
	if err != nil {
		t.Fatalf("people/world v2: %v", err)
	}
	if v1.Definition.DefinitionHash == v2.Definition.DefinitionHash {
		t.Fatal("people/world v2 must not reuse v1 definition hash")
	}
	if v1.Definition.Nodes[0].Prompt.Version != 1 || v1.Definition.Nodes[0].Schema.Version != 1 {
		t.Fatalf("people/world v1 bindings changed: %+v", v1.Definition.Nodes[0])
	}
	if v2.Definition.Nodes[0].Prompt.Version != 2 || v2.Definition.Nodes[0].Schema.Version != 2 {
		t.Fatalf("people/world v2 bindings = %+v", v2.Definition.Nodes[0])
	}
	schema, err := registry.Schema(context.Background(), "story_cast.schema", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{`"id"`, `"source_character_ref"`, `"target_character_ref"`} {
		if !strings.Contains(string(schema.JSON), marker) {
			t.Fatalf("people/world v2 schema missing %s", marker)
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
	legacy, err := registry.Workflow(context.Background(), StoryAuthorReviseWorkflowID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacy.Definition.Nodes[0].InputSelectors, []string{"$task", "$context"}) || legacy.Binding.Hash == asset.Binding.Hash {
		t.Fatal("new revision binding mutated the legacy workflow")
	}
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
