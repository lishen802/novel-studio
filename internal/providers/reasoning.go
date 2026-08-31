package providers

import (
	"fmt"
	"net/url"
	"strings"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// openaiReasoningEffort 把冻结的推理强度映射到 OpenAI Responses 的 effort 值。
// auto 不发送覆盖；DeepSeek 保留 max，其余兼容端点映射为 xhigh。
func openaiReasoningEffort(baseURL string, effort agentdomain.ReasoningEffort) (string, bool, error) {
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
		if isDeepSeekEndpoint(baseURL) {
			return "max", true, nil
		}
		return "xhigh", true, nil
	default:
		return "", false, fmt.Errorf("%w: unsupported reasoning effort %s", ErrIncompatible, effort)
	}
}

// isDeepSeekEndpoint 只依据连接端点识别官方 DeepSeek 协议差异。
func isDeepSeekEndpoint(baseURL string) bool {
	parsed, err := url.Parse(baseURL)
	host := strings.ToLower(parsed.Hostname())
	return err == nil && (host == "api.deepseek.com" || strings.HasSuffix(host, ".deepseek.com"))
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

// supportsAdaptiveThinking 覆盖当前内置 Claude 4.6+ 模型；未知模型保留厂商默认行为。
func supportsAdaptiveThinking(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(normalized, "claude-") &&
		(strings.Contains(normalized, "-4-6") || strings.Contains(normalized, "-4.6") ||
			strings.Contains(normalized, "-4-7") || strings.Contains(normalized, "-4.7") ||
			strings.Contains(normalized, "-4-8") || strings.Contains(normalized, "-4.8") ||
			strings.Contains(normalized, "-5"))
}

// supportsAnthropicEffort 避免向不支持 output_config.effort 的旧模型发送无效参数。
func supportsAnthropicEffort(model string) bool {
	normalized := strings.ToLower(strings.TrimSpace(model))
	return supportsAdaptiveThinking(normalized) || strings.Contains(normalized, "claude-opus-4-5") || strings.Contains(normalized, "claude-opus-4.5")
}
