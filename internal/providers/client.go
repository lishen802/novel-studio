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
	// ErrProvider 表示未细分的 Provider 请求失败。
	ErrProvider = errors.New("provider request failed")
	// ErrInvalidStream 表示流式响应格式损坏。
	ErrInvalidStream = errors.New("invalid provider stream")
)

type GenerateRequest = ports.ModelGenerateRequest
type GenerateResult = ports.ModelGenerateResult

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
	payload := map[string]any{
		"model":             request.Model,
		"instructions":      request.System,
		"input":             request.Input,
		"max_output_tokens": request.MaxOutputTokens,
		"stream":            true,
	}
	// 发送方案冻结的温度，使 writer/reviewer 等角色的生成参数真正生效。
	// temperature=0 通常用于探测或刻意使用 Provider 默认值，此时不发送。
	if request.Temperature > 0 {
		payload["temperature"] = request.Temperature
	}
	if len(request.JSONSchema) > 0 {
		var schema any
		if err := json.Unmarshal(request.JSONSchema, &schema); err != nil {
			return GenerateResult{}, fmt.Errorf("decode output schema: %w", err)
		}
		payload["text"] = map[string]any{"format": map[string]any{
			"type": "json_schema", "name": request.SchemaName, "schema": schema, "strict": true,
		}}
	}
	response, err := client.post(ctx, "/responses", apiKey, payload)
	if err != nil {
		return GenerateResult{}, err
	}
	defer response.Body.Close()

	var result GenerateResult
	err = readSSE(response.Body, func(event string, data []byte) error {
		var envelope struct {
			Type     string `json:"type"`
			Delta    string `json:"delta"`
			Response struct {
				ID    string `json:"id"`
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			} `json:"response"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return err
		}
		switch envelope.Type {
		case "response.output_text.delta":
			result.Text += envelope.Delta
			if onDelta != nil {
				onDelta(envelope.Delta)
			}
		case "response.completed":
			result.ProviderRequestID = envelope.Response.ID
			result.InputTokens = envelope.Response.Usage.InputTokens
			result.OutputTokens = envelope.Response.Usage.OutputTokens
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

func (client *OpenAIClient) post(
	ctx context.Context,
	path, apiKey string,
	payload any,
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
	if request.Temperature > 0 {
		payload["temperature"] = request.Temperature
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
		if has("content", "policy", "safety", "moderation", "refusal", "unsafe") {
			return ErrContentRejected
		}
		return ErrProvider
	case status >= 500:
		if has("overloaded") {
			return ErrRateLimited
		}
		return ErrProvider
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
