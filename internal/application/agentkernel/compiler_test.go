package agentkernel_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestCompileFreezesDeterministicWorkflow(t *testing.T) {
	registry := fixtureRegistry(t, false)
	registries := agentkernel.Registries{Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry}
	first, err := agentkernel.Compile(context.Background(), registries, "chapter", 1)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	second, err := agentkernel.Compile(context.Background(), registries, "chapter", 1)
	if err != nil {
		t.Fatalf("compile again: %v", err)
	}
	if first.Hash != second.Hash || !agentdomain.ValidSHA256(first.Hash) {
		t.Fatalf("unstable hash: %q / %q", first.Hash, second.Hash)
	}
	if len(first.Nodes) != 2 || first.Nodes[0].ID != "plan" || first.Nodes[1].ID != "write" {
		t.Fatalf("nodes not canonical: %#v", first.Nodes)
	}
}

func TestCompileRejectsMissingResource(t *testing.T) {
	registry := fixtureRegistry(t, true)
	_, err := agentkernel.Compile(context.Background(), agentkernel.Registries{
		Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
	}, "chapter", 1)
	if !errors.Is(err, agentkernel.ErrCompileWorkflow) {
		t.Fatalf("expected compile error, got %v", err)
	}
	if !errors.Is(err, ports.ErrAgentAssetNotFound) {
		t.Fatalf("missing resource cause lost: %v", err)
	}
}

func TestCompileRejectsPromptVariableMismatch(t *testing.T) {
	promptText := "任务：{{.Task}}"
	promptBinding := agentassets.BindingForText("prompt", 1, []byte(promptText))
	schemaJSON := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false}`)
	schemaBinding := agentassets.BindingForText("schema", 1, schemaJSON)
	assemblerJSON := []byte(`{"strategy":"json","version":1}`)
	assemblerBinding := agentassets.BindingForText("assembler", 1, assemblerJSON)
	definition := singleNodeDefinition(promptBinding, schemaBinding, assemblerBinding)
	workflow := ports.WorkflowAsset{Binding: agentdomain.VersionBinding{ID: definition.ID, Version: definition.Version, Hash: definition.DefinitionHash}, Definition: definition}
	registry, err := agentassets.New([]ports.WorkflowAsset{workflow}, []ports.PromptAsset{{Binding: promptBinding, Template: promptText, Variables: []string{"Unused"}}},
		[]ports.SchemaAsset{{Binding: schemaBinding, JSON: schemaJSON}}, []ports.AssemblerAsset{{Binding: assemblerBinding, JSON: assemblerJSON}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = agentkernel.Compile(context.Background(), agentkernel.Registries{
		Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
	}, "single", 1)
	if !errors.Is(err, agentkernel.ErrCompileWorkflow) {
		t.Fatalf("expected prompt error, got %v", err)
	}
}

func TestCompileRejectsNonStrictSchema(t *testing.T) {
	promptText := "任务：{{.Task}}"
	promptBinding := agentassets.BindingForText("prompt", 1, []byte(promptText))
	schemaJSON := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":true}`)
	schemaBinding := agentassets.BindingForText("schema", 1, schemaJSON)
	assemblerJSON := []byte(`{"strategy":"json","version":1}`)
	assemblerBinding := agentassets.BindingForText("assembler", 1, assemblerJSON)
	definition := singleNodeDefinition(promptBinding, schemaBinding, assemblerBinding)
	registry, err := agentassets.New(
		[]ports.WorkflowAsset{{Binding: agentdomain.VersionBinding{ID: definition.ID, Version: definition.Version, Hash: definition.DefinitionHash}, Definition: definition}},
		[]ports.PromptAsset{{Binding: promptBinding, Template: promptText, Variables: []string{"Task"}}},
		[]ports.SchemaAsset{{Binding: schemaBinding, JSON: schemaJSON}}, []ports.AssemblerAsset{{Binding: assemblerBinding, JSON: assemblerJSON}},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = agentkernel.Compile(context.Background(), agentkernel.Registries{
		Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
	}, "single", 1)
	if !errors.Is(err, agentkernel.ErrCompileWorkflow) {
		t.Fatalf("expected strict schema error, got %v", err)
	}
}

func fixtureRegistry(t *testing.T, omitWriterPrompt bool) *agentassets.Registry {
	t.Helper()
	planPrompt := "策划：{{.Task}}"
	writePrompt := "写作：{{.Plan}}"
	planBinding := agentassets.BindingForText("plan_prompt", 1, []byte(planPrompt))
	writeBinding := agentassets.BindingForText("write_prompt", 1, []byte(writePrompt))
	schemaJSON := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false}`)
	schemaBinding := agentassets.BindingForText("chapter_schema", 1, schemaJSON)
	assemblerJSON := []byte(`{"strategy":"json","version":1}`)
	assemblerBinding := agentassets.BindingForText("default_assembler", 1, assemblerJSON)
	definition := agentdomain.WorkflowDefinition{
		ID: "chapter", Version: 1, TaskKind: agentdomain.TaskChapterDraft, EntryNodeID: "plan", ResultNodeID: "write",
		BudgetPolicyVersion: 1, QualityPolicyVersion: 1, MaxRevisionAttempts: 1,
		MaxRunTokens: 10000, RepairReserve: 1000, RevisionReserve: 1000,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "write", Capability: agentdomain.CapabilityProseWriter, RoleSlot: agentdomain.RoleWriter, Prompt: writeBinding, Schema: schemaBinding,
				Assembler: assemblerBinding, InputSelectors: []string{"node:plan"}, OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 4000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "plan", Capability: agentdomain.CapabilityChapterPlanner, RoleSlot: agentdomain.RolePlanner, Prompt: planBinding, Schema: schemaBinding,
				Assembler: assemblerBinding, InputSelectors: []string{"$task"}, OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 2000, FailurePolicy: agentdomain.FailureFailRun},
		}, Edges: []agentdomain.WorkflowEdge{{From: "plan", To: "write"}},
	}
	definition.DefinitionHash = definition.ContentHash()
	prompts := []ports.PromptAsset{{Binding: planBinding, Template: planPrompt, Variables: []string{"Task"}}}
	if !omitWriterPrompt {
		prompts = append(prompts, ports.PromptAsset{Binding: writeBinding, Template: writePrompt, Variables: []string{"Plan"}})
	}
	registry, err := agentassets.New(
		[]ports.WorkflowAsset{{Binding: agentdomain.VersionBinding{ID: definition.ID, Version: definition.Version, Hash: definition.DefinitionHash}, Definition: definition}}, prompts,
		[]ports.SchemaAsset{{Binding: schemaBinding, JSON: schemaJSON}}, []ports.AssemblerAsset{{Binding: assemblerBinding, JSON: assemblerJSON}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

func singleNodeDefinition(prompt, schema, assembler agentdomain.VersionBinding) agentdomain.WorkflowDefinition {
	definition := agentdomain.WorkflowDefinition{ID: "single", Version: 1, TaskKind: agentdomain.TaskBaselineDraft, EntryNodeID: "write", ResultNodeID: "write",
		BudgetPolicyVersion: 1, QualityPolicyVersion: 1, MaxRevisionAttempts: 0,
		MaxRunTokens: 4000, RepairReserve: 500, RevisionReserve: 500,
		Nodes: []agentdomain.WorkflowNode{{ID: "write", Capability: agentdomain.CapabilityProseWriter, RoleSlot: agentdomain.RoleWriter,
			Prompt: prompt, Schema: schema, Assembler: assembler, InputSelectors: []string{"$task"}, OutputKind: agentdomain.OutputPrimaryContent,
			MaxOutputTokens: 2000, FailurePolicy: agentdomain.FailureFailRun}}}
	definition.DefinitionHash = definition.ContentHash()
	return definition
}
