package agentkernel

import (
	"errors"
	"fmt"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

var ErrInvalidLegacyRun = errors.New("invalid legacy agent run")

// LegacyNode 将旧三角色步骤映射为稳定能力，供旧项目继续通过统一调度边界执行。
type LegacyNode struct {
	StepIndex  int
	NodeID     string
	Capability agentdomain.Capability
}

// AdaptLegacyRun 验证旧运行的固定顺序并生成只读兼容节点列表。
func AdaptLegacyRun(run agentdomain.Run) ([]LegacyNode, error) {
	if !run.IsLegacy() || len(run.Steps) != 3 {
		return nil, ErrInvalidLegacyRun
	}
	expected := []struct {
		role       agentdomain.Role
		capability agentdomain.Capability
	}{
		{role: agentdomain.RolePlanner, capability: agentdomain.CapabilityChapterPlanner},
		{role: agentdomain.RoleWriter, capability: agentdomain.CapabilityProseWriter},
		{role: agentdomain.RoleContinuityReviewer, capability: agentdomain.CapabilityContinuityReview},
	}
	result := make([]LegacyNode, len(expected))
	for index, expectation := range expected {
		step := run.Steps[index]
		if step.Role != expectation.role || step.Position != index+1 {
			return nil, fmt.Errorf("%w: position %d", ErrInvalidLegacyRun, index+1)
		}
		result[index] = LegacyNode{
			StepIndex: index, NodeID: string(expectation.role), Capability: expectation.capability,
		}
	}
	return result, nil
}
