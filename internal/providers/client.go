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
	ErrAuthentication = errors.New("provider authentication failed")
	ErrRateLimited    = errors.New("provider rate limited")
	ErrProvider       = errors.New("provider request failed")
	ErrInvalidStream  = errors.New("invalid provider stream")
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
		return nil, fmt.Errorf("%w: %w", ErrProvider, err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return nil, classifyStatus(response.StatusCode)
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
		return GenerateResult{}, fmt.Errorf("%w: %w", ErrProvider, err)
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return GenerateResult{}, classifyStatus(response.StatusCode)
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

func classifyStatus(status int) error {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrAuthentication
	case http.StatusTooManyRequests:
		return ErrRateLimited
	default:
		return fmt.Errorf("%w: HTTP %d", ErrProvider, status)
	}
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
