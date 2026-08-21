package quality

import "errors"

var ErrInvalidHarnessVariant = errors.New("invalid harness workflow variant")

type HarnessWorkflowVariant struct {
	ID                     string   `json:"id"`
	WorkflowID             string   `json:"workflow_id"`
	WorkflowVersion        int      `json:"workflow_version"`
	EnhancedDirector       bool     `json:"enhanced_director"`
	ProfessionalAnalyses   []string `json:"professional_analyses"`
	SplitIndependentReview bool     `json:"split_independent_review"`
	DefaultEnabled         bool     `json:"default_enabled"`
}

// BuiltinHarnessWorkflowVariants 返回公平对照使用的固定工作流变体；实验变体默认不启用。
func BuiltinHarnessWorkflowVariants() []HarnessWorkflowVariant {
	return []HarnessWorkflowVariant{
		{ID: "minimal", WorkflowID: "quality.chapter", WorkflowVersion: 1, DefaultEnabled: true},
		{ID: "enhanced-director", WorkflowID: "quality.chapter.director", WorkflowVersion: 1, EnhancedDirector: true},
		{ID: "split-review", WorkflowID: "quality.chapter.split-review", WorkflowVersion: 1, EnhancedDirector: true, SplitIndependentReview: true},
	}
}
