package agentassets

import (
	"context"
	"errors"
	"fmt"
	"reflect"
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
		{StoryRoadmapWorkflowID, StoryProductWorkflowVersion, []string{"blueprint", "review"}},
		{StoryAuthorReviseWorkflowID, StoryProductWorkflowVersion, []string{"revise"}},
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
