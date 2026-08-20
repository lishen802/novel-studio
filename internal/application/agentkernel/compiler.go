package agentkernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

var ErrCompileWorkflow = errors.New("compile agent workflow")

type Registries struct {
	Workflows  ports.WorkflowRegistry
	Prompts    ports.PromptRegistry
	Schemas    ports.SchemaRegistry
	Assemblers ports.AssemblerRegistry
}

type FrozenNode struct {
	ID                  string                     `json:"id"`
	Capability          agentdomain.Capability     `json:"capability"`
	RoleSlot            agentdomain.Role      `json:"role_slot"`
	Prompt              agentdomain.VersionBinding `json:"prompt"`
	Schema              agentdomain.VersionBinding `json:"schema"`
	Assembler           agentdomain.VersionBinding `json:"assembler"`
	InputSelectors      []string                   `json:"input_selectors"`
	OutputKind          agentdomain.OutputKind     `json:"output_kind"`
	MaxOutputTokens     int                        `json:"max_output_tokens"`
	TemperatureOverride *float64                   `json:"temperature_override,omitempty"`
	Condition           string                     `json:"condition,omitempty"`
	FailurePolicy       agentdomain.FailurePolicy  `json:"failure_policy"`
}

type CompiledWorkflow struct {
	Workflow             agentdomain.VersionBinding `json:"workflow"`
	TaskKind             agentdomain.TaskKind       `json:"task_kind"`
	EntryNodeID          string                     `json:"entry_node_id"`
	ResultNodeID         string                     `json:"result_node_id"`
	BudgetPolicyVersion  int                        `json:"budget_policy_version"`
	QualityPolicyVersion int                        `json:"quality_policy_version"`
	MaxRevisionAttempts  int                        `json:"max_revision_attempts"`
	MaxRunTokens         int                        `json:"max_run_tokens"`
	RepairReserve        int                        `json:"repair_reserve"`
	RevisionReserve      int                        `json:"revision_reserve"`
	Nodes                []FrozenNode               `json:"nodes"`
	Edges                []agentdomain.WorkflowEdge `json:"edges"`
	Hash                 string                     `json:"hash"`
}

// Compile 精确解析全部资源并生成与注册顺序无关的确定性工作流快照。
func Compile(ctx context.Context, registries Registries, id string, version int) (CompiledWorkflow, error) {
	if err := ctx.Err(); err != nil {
		return CompiledWorkflow{}, fmt.Errorf("%w: %w", ErrCompileWorkflow, err)
	}
	if registries.Workflows == nil || registries.Prompts == nil || registries.Schemas == nil || registries.Assemblers == nil {
		return CompiledWorkflow{}, fmt.Errorf("%w: incomplete registries", ErrCompileWorkflow)
	}
	workflow, err := registries.Workflows.Workflow(ctx, id, version)
	if err != nil {
		return CompiledWorkflow{}, fmt.Errorf("%w: %w", ErrCompileWorkflow, err)
	}
	if err := workflow.Binding.Validate(); err != nil {
		return CompiledWorkflow{}, fmt.Errorf("%w: workflow binding", ErrCompileWorkflow)
	}
	if workflow.Binding.ID != workflow.Definition.ID || workflow.Binding.Version != workflow.Definition.Version ||
		workflow.Binding.Hash != workflow.Definition.DefinitionHash || workflow.Binding.Hash != workflow.Definition.ContentHash() {
		return CompiledWorkflow{}, fmt.Errorf("%w: workflow hash mismatch", ErrCompileWorkflow)
	}
	nodes, err := workflow.Definition.OrderedNodes()
	if err != nil {
		return CompiledWorkflow{}, fmt.Errorf("%w: %w", ErrCompileWorkflow, err)
	}
	frozen := make([]FrozenNode, 0, len(nodes))
	for _, node := range nodes {
		if err := ctx.Err(); err != nil {
			return CompiledWorkflow{}, fmt.Errorf("%w: %w", ErrCompileWorkflow, err)
		}
		prompt, promptErr := registries.Prompts.Prompt(ctx, node.Prompt.ID, node.Prompt.Version)
		if promptErr != nil {
			return CompiledWorkflow{}, fmt.Errorf("%w: node %s prompt: %w", ErrCompileWorkflow, node.ID, promptErr)
		}
		if err := validatePrompt(prompt, node.Prompt); err != nil {
			return CompiledWorkflow{}, fmt.Errorf("%w: node %s: %v", ErrCompileWorkflow, node.ID, err)
		}
		schema, schemaErr := registries.Schemas.Schema(ctx, node.Schema.ID, node.Schema.Version)
		if schemaErr != nil {
			return CompiledWorkflow{}, fmt.Errorf("%w: node %s schema: %w", ErrCompileWorkflow, node.ID, schemaErr)
		}
		if err := validateSchemaAsset(schema.Binding, node.Schema, schema.JSON); err != nil {
			return CompiledWorkflow{}, fmt.Errorf("%w: node %s schema", ErrCompileWorkflow, node.ID)
		}
		assembler, assemblerErr := registries.Assemblers.Assembler(ctx, node.Assembler.ID, node.Assembler.Version)
		if assemblerErr != nil {
			return CompiledWorkflow{}, fmt.Errorf("%w: node %s assembler: %w", ErrCompileWorkflow, node.ID, assemblerErr)
		}
		if err := validateAssemblerAsset(assembler.Binding, node.Assembler, assembler.JSON); err != nil {
			return CompiledWorkflow{}, fmt.Errorf("%w: node %s assembler", ErrCompileWorkflow, node.ID)
		}
		frozen = append(frozen, FrozenNode{
			ID: node.ID, Capability: node.Capability, RoleSlot: node.RoleSlot,
			Prompt: prompt.Binding, Schema: schema.Binding, Assembler: assembler.Binding,
			InputSelectors: append([]string(nil), node.InputSelectors...), MaxOutputTokens: node.MaxOutputTokens,
			OutputKind: node.OutputKind, TemperatureOverride: cloneFloat(node.TemperatureOverride),
			Condition: node.Condition, FailurePolicy: node.FailurePolicy,
		})
	}
	edges := append([]agentdomain.WorkflowEdge(nil), workflow.Definition.Edges...)
	sortEdges(edges)
	result := CompiledWorkflow{
		Workflow: workflow.Binding, TaskKind: workflow.Definition.TaskKind,
		EntryNodeID: workflow.Definition.EntryNodeID, ResultNodeID: workflow.Definition.ResultNodeID,
		BudgetPolicyVersion: workflow.Definition.BudgetPolicyVersion, QualityPolicyVersion: workflow.Definition.QualityPolicyVersion,
		MaxRevisionAttempts: workflow.Definition.MaxRevisionAttempts, MaxRunTokens: workflow.Definition.MaxRunTokens,
		RepairReserve: workflow.Definition.RepairReserve, RevisionReserve: workflow.Definition.RevisionReserve,
		Nodes: frozen, Edges: edges,
	}
	result.Hash = compiledHash(result)
	return result, nil
}

func compiledHash(workflow CompiledWorkflow) string {
	workflow.Hash = ""
	return hashJSON(workflow)
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func sortEdges(edges []agentdomain.WorkflowEdge) {
	slices.SortFunc(edges, func(left, right agentdomain.WorkflowEdge) int {
		if order := strings.Compare(left.From, right.From); order != 0 {
			return order
		}
		return strings.Compare(left.To, right.To)
	})
}

func validatePrompt(asset ports.PromptAsset, expected agentdomain.VersionBinding) error {
	if asset.Binding != expected || asset.Binding.Hash != agentdomain.SHA256Hex([]byte(asset.Template)) {
		return errors.New("prompt hash mismatch")
	}
	tree, err := template.New(asset.Binding.ID).Option("missingkey=error").Parse(asset.Template)
	if err != nil {
		return fmt.Errorf("invalid prompt template: %w", err)
	}
	declared := make(map[string]bool, len(asset.Variables))
	for _, variable := range asset.Variables {
		variable = strings.TrimSpace(variable)
		if variable == "" || declared[variable] {
			return errors.New("invalid prompt variables")
		}
		declared[variable] = true
	}
	used := map[string]bool{}
	for _, parsed := range tree.Templates() {
		collectTemplateFields(parsed.Root, used)
	}
	if len(used) != len(declared) {
		return errors.New("prompt variable declaration mismatch")
	}
	for variable := range used {
		if !declared[variable] {
			return errors.New("undeclared prompt variable")
		}
	}
	return nil
}

func validateJSONAsset(actual, expected agentdomain.VersionBinding, data []byte) error {
	if actual != expected || actual.Hash != agentdomain.SHA256Hex(data) || !json.Valid(data) {
		return errors.New("invalid JSON asset")
	}
	return nil
}

func validateSchemaAsset(actual, expected agentdomain.VersionBinding, data []byte) error {
	if err := validateJSONAsset(actual, expected, data); err != nil {
		return err
	}
	var schema struct {
		Dialect              string `json:"$schema"`
		Type                 string `json:"type"`
		AdditionalProperties *bool  `json:"additionalProperties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil || schema.Dialect == "" || schema.Type != "object" ||
		schema.AdditionalProperties == nil || *schema.AdditionalProperties {
		return errors.New("schema must be a strict object")
	}
	return nil
}

func validateAssemblerAsset(actual, expected agentdomain.VersionBinding, data []byte) error {
	if err := validateJSONAsset(actual, expected, data); err != nil {
		return err
	}
	var assembler struct {
		Strategy string `json:"strategy"`
		Version  int    `json:"version"`
	}
	if err := json.Unmarshal(data, &assembler); err != nil || strings.TrimSpace(assembler.Strategy) == "" || assembler.Version < 1 {
		return errors.New("invalid assembler contract")
	}
	return nil
}

func collectTemplateFields(node parse.Node, used map[string]bool) {
	if node == nil {
		return
	}
	switch value := node.(type) {
	case *parse.ListNode:
		for _, child := range value.Nodes {
			collectTemplateFields(child, used)
		}
	case *parse.ActionNode:
		collectTemplateFields(value.Pipe, used)
	case *parse.PipeNode:
		for _, command := range value.Cmds {
			collectTemplateFields(command, used)
		}
	case *parse.CommandNode:
		for _, argument := range value.Args {
			collectTemplateFields(argument, used)
		}
	case *parse.FieldNode:
		if len(value.Ident) > 0 {
			used[value.Ident[0]] = true
		}
	case *parse.IfNode:
		collectTemplateFields(value.Pipe, used)
		collectTemplateFields(value.List, used)
		collectTemplateFields(value.ElseList, used)
	case *parse.RangeNode:
		collectTemplateFields(value.Pipe, used)
		collectTemplateFields(value.List, used)
		collectTemplateFields(value.ElseList, used)
	case *parse.WithNode:
		collectTemplateFields(value.Pipe, used)
		collectTemplateFields(value.List, used)
		collectTemplateFields(value.ElseList, used)
	}
}

func hashJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return agentdomain.SHA256Hex(encoded)
}
