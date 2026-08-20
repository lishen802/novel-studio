package agentassets

import (
	"context"
	"errors"
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
	cases := []struct {
		id      string
		version int
		nodes   []string
	}{
		{BaselineWorkflowID, BaselineWorkflowVersion, []string{"write"}},
		{QualityChapterWorkflowID, QualityChapterWorkflowVersion, []string{"plan", "draft", "review", "revision"}},
		{QualityBlueprintWorkflowID, QualityBlueprintWorkflowVersion, []string{"blueprint", "review", "revision"}},
	}
	for _, test := range cases {
		compiled, compileErr := agentkernel.Compile(context.Background(), agentkernel.Registries{
			Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
		}, test.id, test.version)
		if compileErr != nil {
			t.Fatalf("compile %s: %v", test.id, compileErr)
		}
		if len(compiled.Nodes) != len(test.nodes) {
			t.Fatalf("%s nodes: %#v", test.id, compiled.Nodes)
		}
		for index, node := range compiled.Nodes {
			if node.ID != test.nodes[index] {
				t.Fatalf("%s node %d = %s, want %s", test.id, index, node.ID, test.nodes[index])
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
