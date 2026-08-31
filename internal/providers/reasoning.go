package providers

import (
	"fmt"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// openaiReasoningEffort 把冻结的推理强度映射到 OpenAI Responses 的 effort 值。
// auto 不发送覆盖；max 映射为 xhigh。未知值必须失败，不能静默降级。
func openaiReasoningEffort(effort agentdomain.ReasoningEffort) (string, bool, error) {
	switch effort.Normalized() {
	case agentdomain.ReasoningAuto:
		return "", false, nil
	case agentdomain.ReasoningNone:
		return "none", true, nil
	case agentdomain.ReasoningLow:
		return "low", true, nil
	case agentdomain.ReasoningMedium:
		return "medium", true, nil
	case agentdomain.ReasoningHigh:
		return "high", true, nil
	case agentdomain.ReasoningMax:
		return "xhigh", true, nil
	default:
		return "", false, fmt.Errorf("%w: unsupported reasoning effort %s", ErrIncompatible, effort)
	}
}

// anthropicOutputEffort 把冻结的推理强度映射到 Anthropic output_config.effort。
// auto 与 none 不发送额外推理配置。
func anthropicOutputEffort(effort agentdomain.ReasoningEffort) (string, bool, error) {
	switch effort.Normalized() {
	case agentdomain.ReasoningAuto, agentdomain.ReasoningNone:
		return "", false, nil
	case agentdomain.ReasoningLow, agentdomain.ReasoningMedium, agentdomain.ReasoningHigh, agentdomain.ReasoningMax:
		return string(effort.Normalized()), true, nil
	default:
		return "", false, fmt.Errorf("%w: unsupported reasoning effort %s", ErrIncompatible, effort)
	}
}
