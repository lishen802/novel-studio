package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

var ErrInvalidWorkflow = errors.New("invalid workflow definition")

type FailurePolicy string

const (
	FailureFailRun  FailurePolicy = "fail_run"
	FailureSkipNode FailurePolicy = "skip_node"
)

type OutputKind string

const (
	OutputStructured     OutputKind = "structured"
	OutputPrimaryContent OutputKind = "primary_content"
	OutputFindings       OutputKind = "findings"
)

type PublicStageID string

const (
	PublicStageContextPreparing   PublicStageID = "context_preparing"
	PublicStageStoryPlanning      PublicStageID = "story_planning"
	PublicStageChapterDirecting   PublicStageID = "chapter_directing"
	PublicStageProfessionalReview PublicStageID = "professional_analysis"
	PublicStageProseWriting       PublicStageID = "prose_writing"
	PublicStageIndependentReview  PublicStageID = "independent_review"
	PublicStageBoundedRevision    PublicStageID = "bounded_revision"
	PublicStageRevisionVerify     PublicStageID = "revision_verify"
	PublicStageCandidateReady     PublicStageID = "candidate_ready"
)

func ValidPublicStageID(stage PublicStageID) bool {
	switch stage {
	case PublicStageContextPreparing, PublicStageStoryPlanning, PublicStageChapterDirecting,
		PublicStageProfessionalReview, PublicStageProseWriting, PublicStageIndependentReview,
		PublicStageBoundedRevision, PublicStageRevisionVerify, PublicStageCandidateReady:
		return true
	default:
		return false
	}
}

// DefaultPublicStageForCapability 为未携带公开阶段的历史工作流提供确定性兼容映射。
func DefaultPublicStageForCapability(capability Capability) PublicStageID {
	switch capability {
	case CapabilityStoryArchitect:
		return PublicStageStoryPlanning
	case CapabilityChapterPlanner:
		return PublicStageChapterDirecting
	case CapabilityProseWriter:
		return PublicStageProseWriting
	case CapabilityContinuityReview, CapabilityContentReview, CapabilityStyleReview:
		return PublicStageIndependentReview
	case CapabilityBoundedRevision:
		return PublicStageBoundedRevision
	case CapabilityRevisionVerify:
		return PublicStageRevisionVerify
	default:
		return ""
	}
}

type WorkflowNode struct {
	ID                  string         `json:"id"`
	PublicStageID       PublicStageID  `json:"public_stage_id,omitempty"`
	Capability          Capability     `json:"capability"`
	RoleSlot            Role           `json:"role_slot"`
	Prompt              VersionBinding `json:"prompt"`
	Schema              VersionBinding `json:"schema"`
	Assembler           VersionBinding `json:"assembler"`
	InputSelectors      []string       `json:"input_selectors"`
	OutputKind          OutputKind     `json:"output_kind"`
	MaxOutputTokens     int            `json:"max_output_tokens"`
	TemperatureOverride *float64       `json:"temperature_override,omitempty"`
	Condition           string         `json:"condition,omitempty"`
	FailurePolicy       FailurePolicy  `json:"failure_policy"`
}

type WorkflowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// WorkflowDefinition 描述不可变、有版本的智能体有向无环工作流。
type WorkflowDefinition struct {
	ID                   string         `json:"id"`
	Version              int            `json:"version"`
	TaskKind             TaskKind       `json:"task_kind"`
	EntryNodeID          string         `json:"entry_node_id"`
	ResultNodeID         string         `json:"result_node_id"`
	BudgetPolicyVersion  int            `json:"budget_policy_version"`
	QualityPolicyVersion int            `json:"quality_policy_version"`
	MaxRevisionAttempts  int            `json:"max_revision_attempts"`
	MaxRunTokens         int            `json:"max_run_tokens"`
	RepairReserve        int            `json:"repair_reserve"`
	RevisionReserve      int            `json:"revision_reserve"`
	Nodes                []WorkflowNode `json:"nodes"`
	Edges                []WorkflowEdge `json:"edges"`
	DefinitionHash       string         `json:"definition_hash"`
}

func (definition WorkflowDefinition) Validate() error {
	if blank(definition.ID) || definition.Version < 1 || !ValidTaskKind(definition.TaskKind) ||
		blank(definition.EntryNodeID) || blank(definition.ResultNodeID) || definition.BudgetPolicyVersion < 1 ||
		definition.QualityPolicyVersion < 1 || definition.MaxRevisionAttempts < 0 || !ValidSHA256(definition.DefinitionHash) ||
		definition.MaxRunTokens < 1 || definition.RepairReserve < 0 || definition.RevisionReserve < 0 || len(definition.Nodes) == 0 {
		return ErrInvalidWorkflow
	}
	if definition.DefinitionHash != definition.ContentHash() {
		return fmt.Errorf("%w: definition hash mismatch", ErrInvalidWorkflow)
	}
	nodes := make(map[string]WorkflowNode, len(definition.Nodes))
	for _, node := range definition.Nodes {
		if blank(node.ID) || (node.PublicStageID != "" && !ValidPublicStageID(node.PublicStageID)) || !ValidCapability(node.Capability) || !ValidRoleForCapability(node.Capability, node.RoleSlot) ||
			node.MaxOutputTokens < 1 || !validOutputKind(node.OutputKind) ||
			(node.FailurePolicy != FailureFailRun && node.FailurePolicy != FailureSkipNode) {
			return ErrInvalidWorkflow
		}
		if node.TemperatureOverride != nil && (*node.TemperatureOverride < 0 || *node.TemperatureOverride > 2) {
			return ErrInvalidWorkflow
		}
		if node.Condition != "" && node.Condition != "always" && node.Condition != "on_quality_issue" && node.Condition != "on_blocker" {
			return ErrInvalidWorkflow
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("%w: duplicate node %q", ErrInvalidWorkflow, node.ID)
		}
		nodes[node.ID] = node
	}
	if _, exists := nodes[definition.EntryNodeID]; !exists {
		return ErrInvalidWorkflow
	}
	if _, exists := nodes[definition.ResultNodeID]; !exists {
		return ErrInvalidWorkflow
	}

	adjacency := make(map[string][]string, len(nodes))
	reverse := make(map[string][]string, len(nodes))
	indegree := make(map[string]int, len(nodes))
	edges := make(map[string]bool, len(definition.Edges))
	for id := range nodes {
		indegree[id] = 0
	}
	for _, edge := range definition.Edges {
		if _, ok := nodes[edge.From]; !ok {
			return fmt.Errorf("%w: dangling edge", ErrInvalidWorkflow)
		}
		if _, ok := nodes[edge.To]; !ok || edge.From == edge.To {
			return fmt.Errorf("%w: dangling edge", ErrInvalidWorkflow)
		}
		edgeKey := edge.From + "\x00" + edge.To
		if edges[edgeKey] {
			return fmt.Errorf("%w: duplicate edge", ErrInvalidWorkflow)
		}
		edges[edgeKey] = true
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
		reverse[edge.To] = append(reverse[edge.To], edge.From)
		indegree[edge.To]++
	}
	order := topologicalOrder(adjacency, indegree)
	if len(order) != len(nodes) {
		return fmt.Errorf("%w: cycle", ErrInvalidWorkflow)
	}
	reachable := reachableFrom(definition.EntryNodeID, adjacency)
	if len(reachable) != len(nodes) || !reachable[definition.ResultNodeID] {
		return fmt.Errorf("%w: unreachable node", ErrInvalidWorkflow)
	}
	toResult := reachableFrom(definition.ResultNodeID, reverse)
	if len(toResult) != len(nodes) || len(adjacency[definition.ResultNodeID]) != 0 {
		return fmt.Errorf("%w: node cannot reach result", ErrInvalidWorkflow)
	}
	position := make(map[string]int, len(order))
	for index, id := range order {
		position[id] = index
	}
	for _, node := range definition.Nodes {
		if err := validateSelectors(node, nodes, adjacency, position); err != nil {
			return err
		}
		if err := node.Prompt.Validate(); err != nil {
			return err
		}
		if err := node.Schema.Validate(); err != nil {
			return err
		}
		if err := node.Assembler.Validate(); err != nil {
			return err
		}
	}
	if definition.RequiredTokenBudget() > definition.MaxRunTokens {
		return fmt.Errorf("%w: token budget exceeded", ErrInvalidWorkflow)
	}
	return nil
}

// ContentHash 计算定义正文的哈希，definition_hash 字段自身不参与计算。
func (definition WorkflowDefinition) ContentHash() string {
	definition.DefinitionHash = ""
	encoded, _ := json.Marshal(definition)
	return SHA256Hex(encoded)
}

func (definition WorkflowDefinition) RequiredTokenBudget() int {
	total := definition.RepairReserve + definition.RevisionReserve
	for _, node := range definition.Nodes {
		total += node.MaxOutputTokens
	}
	return total
}

// OrderedNodes 返回按依赖关系排序的节点；同层节点按 ID 排序，保证冻结结果可重现。
func (definition WorkflowDefinition) OrderedNodes() ([]WorkflowNode, error) {
	if err := definition.Validate(); err != nil {
		return nil, err
	}
	byID := make(map[string]WorkflowNode, len(definition.Nodes))
	adjacency := make(map[string][]string, len(definition.Nodes))
	indegree := make(map[string]int, len(definition.Nodes))
	for _, node := range definition.Nodes {
		byID[node.ID] = node
		indegree[node.ID] = 0
	}
	for _, edge := range definition.Edges {
		adjacency[edge.From] = append(adjacency[edge.From], edge.To)
		indegree[edge.To]++
	}
	ids := topologicalOrder(adjacency, indegree)
	ordered := make([]WorkflowNode, 0, len(ids))
	for _, id := range ids {
		ordered = append(ordered, byID[id])
	}
	return ordered, nil
}

func topologicalOrder(adjacency map[string][]string, indegree map[string]int) []string {
	queue := make([]string, 0, len(indegree))
	for id, degree := range indegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)
	order := make([]string, 0, len(indegree))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, id)
		children := append([]string(nil), adjacency[id]...)
		sort.Strings(children)
		for _, child := range children {
			indegree[child]--
			if indegree[child] == 0 {
				queue = append(queue, child)
				sort.Strings(queue)
			}
		}
	}
	return order
}

func reachableFrom(entry string, adjacency map[string][]string) map[string]bool {
	seen := map[string]bool{}
	stack := []string{entry}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		stack = append(stack, adjacency[id]...)
	}
	return seen
}

func validateSelectors(node WorkflowNode, nodes map[string]WorkflowNode, adjacency map[string][]string, position map[string]int) error {
	for _, selector := range node.InputSelectors {
		if selector == "$task" || selector == "$context" {
			continue
		}
		if !strings.HasPrefix(selector, "node:") {
			return fmt.Errorf("%w: invalid input selector", ErrInvalidWorkflow)
		}
		dependency := strings.TrimPrefix(selector, "node:")
		if _, exists := nodes[dependency]; !exists || position[dependency] >= position[node.ID] || !reachableFrom(dependency, adjacency)[node.ID] {
			return fmt.Errorf("%w: dangling input selector", ErrInvalidWorkflow)
		}
	}
	return nil
}

func validOutputKind(kind OutputKind) bool {
	return kind == OutputStructured || kind == OutputPrimaryContent || kind == OutputFindings
}
