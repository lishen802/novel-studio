package agent

import (
	"errors"
	"testing"
)

func TestWorkflowDefinitionValidation(t *testing.T) {
	definition := validWorkflowDefinition()
	if err := definition.Validate(); err != nil {
		t.Fatalf("valid workflow: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*WorkflowDefinition)
	}{
		{"duplicate node", func(value *WorkflowDefinition) { value.Nodes = append(value.Nodes, value.Nodes[0]) }},
		{"dangling edge", func(value *WorkflowDefinition) { value.Edges[0].To = "missing" }},
		{"dangling input", func(value *WorkflowDefinition) { value.Nodes[1].InputSelectors = []string{"node:missing"} }},
		{"cycle", func(value *WorkflowDefinition) {
			value.Edges = append(value.Edges, WorkflowEdge{From: "write", To: "plan"})
		}},
		{"budget", func(value *WorkflowDefinition) { value.MaxRunTokens = 100 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			invalid := definition
			invalid.Nodes = append([]WorkflowNode(nil), definition.Nodes...)
			invalid.Edges = append([]WorkflowEdge(nil), definition.Edges...)
			test.mutate(&invalid)
			invalid.DefinitionHash = invalid.ContentHash()
			if !errors.Is(invalid.Validate(), ErrInvalidWorkflow) {
				t.Fatalf("expected workflow error, got %v", invalid.Validate())
			}
		})
	}
}

func TestOrderedNodesUsesTopologyInsteadOfLexicalOrder(t *testing.T) {
	definition := validWorkflowDefinition()
	definition.EntryNodeID, definition.ResultNodeID = "z-plan", "a-write"
	definition.Nodes[0].ID = "z-plan"
	definition.Nodes[1].ID = "a-write"
	definition.Nodes[1].InputSelectors = []string{"node:z-plan"}
	definition.Edges = []WorkflowEdge{{From: "z-plan", To: "a-write"}}
	definition.DefinitionHash = definition.ContentHash()
	ordered, err := definition.OrderedNodes()
	if err != nil {
		t.Fatal(err)
	}
	if ordered[0].ID != "z-plan" || ordered[1].ID != "a-write" {
		t.Fatalf("unexpected order: %#v", ordered)
	}
}

func validWorkflowDefinition() WorkflowDefinition {
	hash := SHA256Hex([]byte("resource"))
	binding := VersionBinding{ID: "resource", Version: 1, Hash: hash}
	definition := WorkflowDefinition{
		ID: "chapter", Version: 1, TaskKind: TaskChapterDraft, EntryNodeID: "plan", ResultNodeID: "write",
		BudgetPolicyVersion: 1, QualityPolicyVersion: 1, MaxRevisionAttempts: 1,
		MaxRunTokens: 10000, RepairReserve: 1000, RevisionReserve: 1000,
		Nodes: []WorkflowNode{
			{ID: "plan", Capability: CapabilityChapterPlanner, RoleSlot: RolePlanner, Prompt: binding, Schema: binding, Assembler: binding,
				InputSelectors: []string{"$task"}, OutputKind: OutputStructured, MaxOutputTokens: 2000, FailurePolicy: FailureFailRun},
			{ID: "write", Capability: CapabilityProseWriter, RoleSlot: RoleWriter, Prompt: binding, Schema: binding, Assembler: binding,
				InputSelectors: []string{"node:plan"}, OutputKind: OutputPrimaryContent, MaxOutputTokens: 4000, FailurePolicy: FailureFailRun},
		},
		Edges: []WorkflowEdge{{From: "plan", To: "write"}},
	}
	definition.DefinitionHash = definition.ContentHash()
	return definition
}
