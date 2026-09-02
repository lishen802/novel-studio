package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

var (
	// ErrAuthentication 表示 API Key 无效或无权访问（401/403）。
	ErrAuthentication = errors.New("provider authentication failed")
	// ErrRateLimited 表示请求被限流（429）。
	ErrRateLimited = errors.New("provider rate limited")
	// ErrQuotaExceeded 表示额度不足或账单受限（402 或带 quota/billing 的 429）。
	ErrQuotaExceeded = errors.New("provider quota exceeded")
	// ErrModelNotFound 表示模型名称不存在或账户不可用（404 且指明 model）。
	ErrModelNotFound = errors.New("provider model not found")
	// ErrContentRejected 表示 Provider 内容政策拒绝。
	ErrContentRejected = errors.New("provider content rejected")
	// ErrNetwork 表示 DNS、TLS、代理或其他网络层失败。
	ErrNetwork = errors.New("provider network failed")
	// ErrIncompatible 表示接口协议与所选 Provider 类型不兼容（例如端点 404）。
	ErrIncompatible = errors.New("provider incompatible")
	// ErrSchemaRejected 表示 Provider 拒绝了结构化输出 Schema。
	ErrSchemaRejected = errors.New("provider schema rejected")
	// ErrInvalidRequest 表示 Provider 拒绝了当前请求参数或请求格式。
	ErrInvalidRequest = errors.New("provider request invalid")
	// ErrProvider 表示未细分的 Provider 请求失败。
	ErrProvider = errors.New("provider request failed")
	// ErrInvalidResponse 表示 HTTP 成功但响应体不是完整、可识别的 Provider 响应。
	ErrInvalidResponse = errors.New("provider response invalid")
	// ErrInvalidStream 表示流式响应格式损坏。
	ErrInvalidStream = errors.New("invalid provider stream")
	// ErrResponseIncomplete 表示 Provider 在达到输出上限等情况下提前结束生成。
	ErrResponseIncomplete = errors.New("provider response incomplete")
	// ErrResponseFailed 表示 Provider 以失败终态结束生成。
	ErrResponseFailed = errors.New("provider response failed")
	// ErrEmptyResponse 表示 Provider 声称完成但没有返回可用正文。
	ErrEmptyResponse = errors.New("provider response empty")
	// ErrReasoningWithoutOutput 表示完成态只有内部推理、没有可见正文。
	ErrReasoningWithoutOutput = errors.New("provider reasoning without visible output")
)

type GenerateRequest = ports.ModelGenerateRequest
type GenerateResult = ports.ModelGenerateResult

const maxBufferedResponseBytes = 64 * 1024 * 1024

type Factory struct {
	httpClient *http.Client
}

func NewFactory(httpClient *http.Client) *Factory {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Factory{httpClient: httpClient}
}

func (factory *Factory) New(connection agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	switch connection.Kind {
	case agentdomain.ProviderOpenAI:
		return NewOpenAIClient(factory.httpClient, connection.BaseURL), nil
	case agentdomain.ProviderAnthropic:
		return NewAnthropicClient(factory.httpClient, connection.BaseURL), nil
	default:
		return nil, agentdomain.ErrInvalidProvider
	}
}

type OpenAIClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewOpenAIClient(httpClient *http.Client, baseURL string) *OpenAIClient {
	return &OpenAIClient{httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

func (client *OpenAIClient) Generate(
	ctx context.Context,
	apiKey string,
	request GenerateRequest,
	onDelta func(string),
) (GenerateResult, error) {
	// Agent Kernel 等后台调用不消费增量，使用完整响应可避免长推理在 SSE
	// 终态前断流，也能完整读取 Provider 的失败状态和用量。
	if onDelta == nil {
		return client.generateBuffered(ctx, apiKey, request)
	}
	generationStarted := false
	result, err := client.generateOnce(ctx, apiKey, request, onDelta, &generationStarted)
	// DeepSeek Responses 流偶尔会在正文开始前无终态断开；此时重试不会
	// 混合两份正文。收到推理或正文后不再自动重试，避免重复生成与重复计费。
	if err != nil && errors.Is(err, ErrInvalidStream) && isDeepSeekEndpoint(client.baseURL) &&
		!generationStarted && ctx.Err() == nil {
		return client.generateOnce(ctx, apiKey, request, onDelta, nil)
	}
	return result, err
}

func (client *OpenAIClient) generateOnce(
	ctx context.Context,
	apiKey string,
	request GenerateRequest,
	onDelta func(string),
	generationStarted *bool,
) (GenerateResult, error) {
	payload, err := client.responsesPayload(request, true)
	if err != nil {
		return GenerateResult{}, err
	}
	response, err := client.post(ctx, "/responses", apiKey, payload, true)
	if err != nil {
		return GenerateResult{}, err
	}
	defer response.Body.Close()

	return readOpenAIStream(response.Body, onDelta, generationStarted)
}

// readOpenAIStream 统一解释 Responses SSE，供实时流和“服务端忽略
// stream=false”时的后台兼容路径复用。
func readOpenAIStream(body io.Reader, onDelta func(string), generationStarted *bool) (GenerateResult, error) {
	var result GenerateResult
	completed := false
	hadReasoning := false
	reasoningTokens := 0
	err := readSSE(body, func(event string, data []byte) error {
		var envelope responseEvent
		if err := json.Unmarshal(data, &envelope); err != nil {
			return err
		}
		eventType := envelope.Type
		if eventType == "" {
			eventType = event
		}
		switch eventType {
		case "response.reasoning_text.delta":
			if envelope.Delta != "" {
				hadReasoning = true
			}
			if generationStarted != nil && envelope.Delta != "" {
				*generationStarted = true
			}
		case "response.output_text.delta":
			if generationStarted != nil && envelope.Delta != "" {
				*generationStarted = true
			}
			result.Text += envelope.Delta
			if onDelta != nil {
				onDelta(envelope.Delta)
			}
		case "response.completed":
			completed = true
			applyResponseState(&result, envelope.Response)
			text, outputHadReasoning := extractVisibleOutput(envelope.Response.Output)
			hadReasoning = hadReasoning || outputHadReasoning
			reasoningTokens = envelope.Response.Usage.reasoningTokens()
			if strings.TrimSpace(result.Text) == "" {
				result.Text = text
			}
		case "response.incomplete":
			applyResponseState(&result, envelope.Response)
			return classifyIncompleteReason(envelope.Response.IncompleteDetails.Reason)
		case "response.failed":
			applyResponseState(&result, envelope.Response)
			return classifyResponseFailure(envelope.Response.Error.Code)
		case "error":
			return classifyResponseFailure(envelope.Error.Code)
		}
		return nil
	})
	if err != nil {
		if isStableProviderError(err) {
			return result, err
		}
		return GenerateResult{}, fmt.Errorf("%w: %w", ErrInvalidStream, err)
	}
	if !completed {
		return GenerateResult{}, ErrInvalidStream
	}
	if strings.TrimSpace(result.Text) == "" {
		return result, classifyMissingVisibleText(hadReasoning, reasoningTokens)
	}
	return result, nil
}

type responseError struct {
	Code string `json:"code"`
}

type responseUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
}

func (usage responseUsage) reasoningTokens() int {
	return usage.OutputTokensDetails.ReasoningTokens
}

type responseOutputItem struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Content json.RawMessage `json:"content"`
}

type responseState struct {
	ID                string        `json:"id"`
	Status            string        `json:"status"`
	Error             responseError `json:"error"`
	IncompleteDetails struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
	Usage  responseUsage        `json:"usage"`
	Output []responseOutputItem `json:"output"`
}

type responseEvent struct {
	Type     string        `json:"type"`
	Delta    string        `json:"delta"`
	Error    responseError `json:"error"`
	Response responseState `json:"response"`
}

func (client *OpenAIClient) generateBuffered(ctx context.Context, apiKey string, request GenerateRequest) (GenerateResult, error) {
	payload, err := client.responsesPayload(request, false)
	if err != nil {
		return GenerateResult{}, err
	}
	response, err := client.post(ctx, "/responses", apiKey, payload, false)
	if err != nil {
		return GenerateResult{}, err
	}
	defer response.Body.Close()

	data, err := io.ReadAll(io.LimitReader(response.Body, maxBufferedResponseBytes+1))
	if err != nil {
		return GenerateResult{}, classifyNetworkError(err)
	}
	if len(data) > maxBufferedResponseBytes {
		return GenerateResult{}, fmt.Errorf("%w: buffered response too large", ErrInvalidResponse)
	}
	trimmed := bytes.TrimSpace(data)
	contentType := strings.ToLower(response.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") || bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte("data:")) {
		return readOpenAIStream(bytes.NewReader(data), nil, nil)
	}

	var envelope responseState
	if len(trimmed) == 0 || json.Unmarshal(trimmed, &envelope) != nil {
		return GenerateResult{}, fmt.Errorf("%w: decode buffered response", ErrInvalidResponse)
	}
	result := GenerateResult{}
	applyResponseState(&result, envelope)
	switch strings.ToLower(strings.TrimSpace(envelope.Status)) {
	case "completed":
	case "incomplete":
		return result, classifyIncompleteReason(envelope.IncompleteDetails.Reason)
	case "failed":
		return result, classifyResponseFailure(envelope.Error.Code)
	default:
		return result, ErrResponseFailed
	}
	text, hadReasoning := extractVisibleOutput(envelope.Output)
	result.Text = text
	if strings.TrimSpace(result.Text) == "" {
		return result, classifyMissingVisibleText(hadReasoning, envelope.Usage.reasoningTokens())
	}
	return result, nil
}

// applyResponseState 把终态响应的请求 ID 和用量写入结果，失败路径也必须保留。
func applyResponseState(result *GenerateResult, state responseState) {
	if strings.TrimSpace(state.ID) != "" {
		result.ProviderRequestID = state.ID
	}
	result.InputTokens = state.Usage.InputTokens
	result.OutputTokens = state.Usage.OutputTokens
}

// extractVisibleOutput 只收集可交给领域契约的正文，不把 reasoning 当作任务输出。
func extractVisibleOutput(items []responseOutputItem) (string, bool) {
	var text strings.Builder
	hadReasoning := false
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.Type)) {
		case "reasoning":
			hadReasoning = true
		case "message":
			text.WriteString(visibleContentText(item))
		case "output_text":
			if strings.TrimSpace(item.Text) != "" {
				text.WriteString(item.Text)
				continue
			}
			text.WriteString(visibleContentText(item))
		}
	}
	return text.String(), hadReasoning
}

func visibleContentText(item responseOutputItem) string {
	if len(bytes.TrimSpace(item.Content)) == 0 {
		return item.Text
	}
	var asString string
	if json.Unmarshal(item.Content, &asString) == nil {
		return asString
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(item.Content, &parts) != nil {
		return item.Text
	}
	var text strings.Builder
	for _, part := range parts {
		switch strings.ToLower(strings.TrimSpace(part.Type)) {
		case "output_text", "text":
			text.WriteString(part.Text)
		}
	}
	return text.String()
}

// classifyMissingVisibleText 区分“真的没有正文”和“推理占满了可见输出”。
func classifyMissingVisibleText(hadReasoning bool, reasoningTokens int) error {
	if hadReasoning || reasoningTokens > 0 {
		return ErrReasoningWithoutOutput
	}
	return ErrEmptyResponse
}

func classifyIncompleteReason(reason string) error {
	if strings.EqualFold(strings.TrimSpace(reason), "content_filter") {
		return ErrContentRejected
	}
	return ErrResponseIncomplete
}

func (client *OpenAIClient) responsesPayload(request GenerateRequest, stream bool) (map[string]any, error) {
	payload := map[string]any{
		"model":        request.Model,
		"instructions": request.System,
		"input":        request.Input,
		"stream":       stream,
	}
	if request.MaxOutputTokens > 0 {
		payload["max_output_tokens"] = request.MaxOutputTokens
	}
	// 发送方案冻结的温度，使 writer/reviewer 等角色的生成参数真正生效。
	// temperature=0 通常用于探测或刻意使用 Provider 默认值，此时不发送。
	if request.Temperature > 0 {
		payload["temperature"] = request.Temperature
	}
	effort, sendEffort, err := openaiReasoningEffort(client.baseURL, request.ReasoningEffort)
	if err != nil {
		return nil, err
	}
	if sendEffort {
		payload["reasoning"] = map[string]any{"effort": effort}
	}
	if len(request.JSONSchema) > 0 {
		var schema any
		if err := json.Unmarshal(request.JSONSchema, &schema); err != nil {
			return nil, fmt.Errorf("decode output schema: %w", err)
		}
		format := map[string]any{
			"type": "json_schema", "name": request.SchemaName, "schema": schema,
		}
		// 官方 DeepSeek Responses 接口不接受 text.format.strict；仅对其他
		// OpenAI 兼容端点发送该扩展字段，避免审阅等可选字段 Schema 被拒绝。
		if !isDeepSeekEndpoint(client.baseURL) {
			format["strict"] = strictJSONSchemaCompatible(schema)
		}
		payload["text"] = map[string]any{"format": format}
	}
	return payload, nil
}

// classifyResponseFailure 只读取 Provider 的稳定错误码，避免把响应正文写入日志或界面。
func classifyResponseFailure(code string) error {
	normalized := strings.ToLower(strings.TrimSpace(code))
	switch {
	case strings.Contains(normalized, "schema"), strings.Contains(normalized, "structured_output"):
		return ErrSchemaRejected
	case strings.Contains(normalized, "auth"), strings.Contains(normalized, "api_key"), strings.Contains(normalized, "permission"):
		return ErrAuthentication
	case strings.Contains(normalized, "model") && strings.Contains(normalized, "not"):
		return ErrModelNotFound
	case strings.Contains(normalized, "quota"), strings.Contains(normalized, "billing"):
		return ErrQuotaExceeded
	case strings.Contains(normalized, "rate"), strings.Contains(normalized, "overload"):
		return ErrRateLimited
	case strings.Contains(normalized, "content"), strings.Contains(normalized, "safety"), strings.Contains(normalized, "policy"):
		return ErrContentRejected
	case strings.Contains(normalized, "invalid_request"), strings.Contains(normalized, "invalid_parameter"), strings.Contains(normalized, "bad_request"):
		return ErrInvalidRequest
	default:
		return ErrResponseFailed
	}
}

func isStableProviderError(err error) bool {
	return errors.Is(err, ErrAuthentication) || errors.Is(err, ErrRateLimited) || errors.Is(err, ErrQuotaExceeded) ||
		errors.Is(err, ErrModelNotFound) || errors.Is(err, ErrContentRejected) || errors.Is(err, ErrIncompatible) ||
		errors.Is(err, ErrSchemaRejected) || errors.Is(err, ErrInvalidRequest) || errors.Is(err, ErrInvalidResponse) ||
		errors.Is(err, ErrProvider) || errors.Is(err, ErrResponseIncomplete) || errors.Is(err, ErrResponseFailed) ||
		errors.Is(err, ErrEmptyResponse) || errors.Is(err, ErrReasoningWithoutOutput)
}

// strictJSONSchemaCompatible 判断 Schema 是否满足 OpenAI 兼容严格模式。
// 含可选字段的领域契约保留原始 Schema 并关闭 Provider 严格模式，最终仍由
// Go 强类型解码和领域 Validate 执行完整校验。
func strictJSONSchemaCompatible(value any) bool {
	switch schema := value.(type) {
	case []any:
		for _, child := range schema {
			if !strictJSONSchemaCompatible(child) {
				return false
			}
		}
		return true
	case map[string]any:
		if properties, ok := schema["properties"].(map[string]any); ok {
			required := make(map[string]struct{})
			if existing, ok := schema["required"].([]any); ok {
				for _, item := range existing {
					if name, ok := item.(string); ok {
						required[name] = struct{}{}
					}
				}
			}
			for name := range properties {
				if _, exists := required[name]; !exists {
					return false
				}
			}
		}
		for _, child := range schema {
			if !strictJSONSchemaCompatible(child) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

func (client *OpenAIClient) post(
	ctx context.Context,
	path, apiKey string,
	payload any,
	stream bool,
) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(client.baseURL, path), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+apiKey)
	request.Header.Set("Content-Type", "application/json")
	if stream {
		request.Header.Set("Accept", "text/event-stream")
	} else {
		request.Header.Set("Accept", "application/json")
	}
	// 模型长生成是独立计费单元，不复用可能已老化的长连接；牺牲一次握手
	// 换取批量运行中每次响应边界的确定性。
	request.Close = true
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, classifyNetworkError(err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		defer response.Body.Close()
		return nil, classifyStatus(response.StatusCode, readErrorBody(response.Body))
	}
	return response, nil
}

type AnthropicClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewAnthropicClient(httpClient *http.Client, baseURL string) *AnthropicClient {
	return &AnthropicClient{httpClient: httpClient, baseURL: strings.TrimRight(baseURL, "/")}
}

func (client *AnthropicClient) Generate(
	ctx context.Context,
	apiKey string,
	request GenerateRequest,
	onDelta func(string),
) (GenerateResult, error) {
	payload := map[string]any{
		"model":      request.Model,
		"max_tokens": request.MaxOutputTokens,
		"system":     request.System,
		"messages":   []map[string]string{{"role": "user", "content": request.Input}},
		"stream":     true,
	}
	if request.MaxOutputTokens < 1 {
		return GenerateResult{}, fmt.Errorf("%w: anthropic requires max_tokens", ErrIncompatible)
	}
	// 新版 Claude 的自适应思考仅接受默认温度；显式推理配置时不发送温度。
	if request.Temperature > 0 && request.ReasoningEffort.Normalized() == agentdomain.ReasoningAuto && !supportsAdaptiveThinking(request.Model) {
		payload["temperature"] = request.Temperature
	}
	effort, sendEffort, err := anthropicOutputEffort(request.ReasoningEffort)
	if err != nil {
		return GenerateResult{}, err
	}
	if sendEffort && !supportsAnthropicEffort(request.Model) {
		return GenerateResult{}, fmt.Errorf("%w: model does not support reasoning effort", ErrIncompatible)
	}
	if len(request.JSONSchema) > 0 {
		var schema any
		if err := json.Unmarshal(request.JSONSchema, &schema); err != nil {
			return GenerateResult{}, fmt.Errorf("decode output schema: %w", err)
		}
		payload["output_config"] = map[string]any{
			"format": map[string]any{"type": "json_schema", "schema": schema},
		}
	}
	if sendEffort {
		cfg, _ := payload["output_config"].(map[string]any)
		if cfg == nil {
			cfg = map[string]any{}
		}
		cfg["effort"] = effort
		payload["output_config"] = cfg
	}
	switch request.ReasoningEffort.Normalized() {
	case agentdomain.ReasoningNone:
		payload["thinking"] = map[string]any{"type": "disabled"}
	case agentdomain.ReasoningLow, agentdomain.ReasoningMedium, agentdomain.ReasoningHigh, agentdomain.ReasoningMax:
		if supportsAdaptiveThinking(request.Model) {
			payload["thinking"] = map[string]any{"type": "adaptive"}
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return GenerateResult{}, err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(client.baseURL, "/v1/messages"), bytes.NewReader(body))
	if err != nil {
		return GenerateResult{}, err
	}
	httpRequest.Header.Set("x-api-key", apiKey)
	httpRequest.Header.Set("anthropic-version", "2023-06-01")
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return GenerateResult{}, classifyNetworkError(err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		defer response.Body.Close()
		return GenerateResult{}, classifyStatus(response.StatusCode, readErrorBody(response.Body))
	}
	defer response.Body.Close()

	var result GenerateResult
	err = readSSE(response.Body, func(event string, data []byte) error {
		var envelope struct {
			Type    string `json:"type"`
			Message struct {
				ID    string `json:"id"`
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Delta struct {
				Type       string `json:"type"`
				Text       string `json:"text"`
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return err
		}
		switch envelope.Type {
		case "message_start":
			result.ProviderRequestID = envelope.Message.ID
			result.InputTokens = envelope.Message.Usage.InputTokens
		case "content_block_delta":
			if envelope.Delta.Type == "text_delta" {
				result.Text += envelope.Delta.Text
				if onDelta != nil {
					onDelta(envelope.Delta.Text)
				}
			}
		case "message_delta":
			result.OutputTokens = envelope.Usage.OutputTokens
		case "error":
			return ErrProvider
		}
		return nil
	})
	if err != nil {
		return GenerateResult{}, fmt.Errorf("%w: %w", ErrInvalidStream, err)
	}
	return result, nil
}

func readSSE(reader io.Reader, handle func(event string, data []byte) error) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var event string
	var data []string
	flush := func() error {
		if len(data) == 0 {
			event = ""
			return nil
		}
		joined := strings.Join(data, "\n")
		data = nil
		currentEvent := event
		event = ""
		if joined == "[DONE]" {
			return nil
		}
		return handle(currentEvent, []byte(joined))
	}
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return flush()
}

// classifyStatus 把 Provider 的 HTTP 状态与错误响应体转换为稳定的领域错误。
// 它只返回哨兵错误，绝不把响应正文、请求正文或密钥带入错误字符串，以免被
// 前端或日志捕获。
func classifyStatus(status int, body []byte) error {
	message := strings.ToLower(string(body))
	has := func(keys ...string) bool {
		for _, key := range keys {
			if strings.Contains(message, key) {
				return true
			}
		}
		return false
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return ErrAuthentication
	case status == http.StatusNotFound:
		if has("model_not_found", "not_found_error", "model not found", "no such model") {
			return ErrModelNotFound
		}
		return ErrIncompatible
	case status == http.StatusPaymentRequired:
		return ErrQuotaExceeded
	case status == http.StatusTooManyRequests:
		if has("quota", "insufficient_quota", "billing") {
			return ErrQuotaExceeded
		}
		return ErrRateLimited
	case status == http.StatusBadRequest || status == http.StatusUnprocessableEntity:
		if has("json_schema", "json schema", "invalid schema", "schema validation", "strict schema", "strict mode") {
			return ErrSchemaRejected
		}
		if has("content", "policy", "safety", "moderation", "refusal", "unsafe") {
			return ErrContentRejected
		}
		return ErrInvalidRequest
	case status >= 500:
		if has("overloaded") {
			return ErrRateLimited
		}
		return ErrResponseFailed
	default:
		return fmt.Errorf("%w: HTTP %d", ErrProvider, status)
	}
}

// classifyNetworkError 把网络层错误转换为领域错误。上下文取消与超时原样返回，
// 使调用方仍能通过 errors.Is 区分 context.Canceled 与 context.DeadlineExceeded；
// 其余 DNS、TLS、代理等失败统一归为 ErrNetwork。
func classifyNetworkError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrNetwork, err)
}

// readErrorBody 有界读取错误响应体，供分类使用；超过 64 KiB 的部分被丢弃，
// 避免恶意或异常的大响应占用内存。
func readErrorBody(body io.Reader) []byte {
	limited := io.LimitReader(body, 64*1024)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil
	}
	return data
}

func joinURL(baseURL, path string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + path
	}
	basePath := strings.TrimRight(parsed.Path, "/")
	if strings.HasPrefix(path, "/v1/") && strings.HasSuffix(basePath, "/v1") {
		path = strings.TrimPrefix(path, "/v1")
	}
	parsed.Path = basePath + path
	return parsed.String()
}
