package providers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestOpenAIResponsesStreamsTextAndUsage(t *testing.T) {
	var received map[string]any
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("Authorization = %q", request.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		stream := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"第一\"}\n\n" +
			"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"章\"}\n\n" +
			"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"usage\":{\"input_tokens\":12,\"output_tokens\":4,\"total_tokens\":16}}}\n\n"
		return sseResponse(stream), nil
	})}

	client := NewOpenAIClient(httpClient, "https://api.example.test/v1")
	var deltas []string
	result, err := client.Generate(context.Background(), "secret", GenerateRequest{
		Model:           "gpt-test",
		System:          "system",
		Input:           "input",
		MaxOutputTokens: 100,
		JSONSchema:      json.RawMessage(`{"type":"object"}`),
		SchemaName:      "plan",
	}, func(delta string) { deltas = append(deltas, delta) })
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Text != "第一章" || result.OutputTokens != 4 || result.ProviderRequestID != "resp_1" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if strings.Join(deltas, "") != result.Text {
		t.Fatalf("deltas = %#v", deltas)
	}
	if received["model"] != "gpt-test" || received["stream"] != true {
		t.Fatalf("unexpected request: %#v", received)
	}
}

func TestOpenAIOmitsMaxOutputAndSendsReasoningEffort(t *testing.T) {
	var received map[string]any
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		stream := "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"
		return sseResponse(stream), nil
	})}
	client := NewOpenAIClient(httpClient, "https://api.example.test/v1")
	_, err := client.Generate(context.Background(), "secret", GenerateRequest{
		Model: "gpt-test", Input: "input", ReasoningEffort: agentdomain.ReasoningHigh,
	}, nil)
	if err != nil && !errors.Is(err, ErrEmptyResponse) {
		t.Fatalf("Generate() error = %v", err)
	}
	if _, exists := received["max_output_tokens"]; exists {
		t.Fatalf("max_output_tokens should be omitted: %#v", received)
	}
	reasoning, _ := received["reasoning"].(map[string]any)
	if reasoning["effort"] != "high" {
		t.Fatalf("reasoning = %#v", received["reasoning"])
	}
}

func TestAnthropicSendsOutputConfigEffort(t *testing.T) {
	var received map[string]any
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		stream := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"usage\":{\"input_tokens\":1}}}\n\n" +
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"ok\"}}\n\n" +
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":1}}\n\n" +
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
		return sseResponse(stream), nil
	})}
	client := NewAnthropicClient(httpClient, "https://api.example.test")
	if _, err := client.Generate(context.Background(), "secret", GenerateRequest{
		Model: "claude-test", Input: "input", MaxOutputTokens: 32, ReasoningEffort: agentdomain.ReasoningMax,
	}, nil); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	cfg, _ := received["output_config"].(map[string]any)
	if cfg["effort"] != "max" {
		t.Fatalf("output_config = %#v", received["output_config"])
	}
}

func TestOpenAIResponsesRejectsIncompleteTerminalEvent(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		stream := "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"未完成\"}\n\n" +
			"event: response.incomplete\ndata: {\"type\":\"response.incomplete\",\"response\":{\"id\":\"resp_incomplete\",\"incomplete_details\":{\"reason\":\"max_output_tokens\"}}}\n\n"
		return sseResponse(stream), nil
	})}

	client := NewOpenAIClient(httpClient, "https://api.example.test")
	_, err := client.Generate(context.Background(), "secret", GenerateRequest{Model: "model", Input: "input", MaxOutputTokens: 10}, nil)
	if !errors.Is(err, ErrResponseIncomplete) {
		t.Fatalf("Generate() error = %v, want ErrResponseIncomplete", err)
	}
}

func TestOpenAIResponsesRejectsFailedTerminalEvent(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		stream := "event: response.failed\ndata: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_failed\",\"error\":{\"code\":\"server_error\",\"message\":\"sensitive upstream detail\"}}}\n\n"
		return sseResponse(stream), nil
	})}

	client := NewOpenAIClient(httpClient, "https://api.example.test")
	_, err := client.Generate(context.Background(), "secret", GenerateRequest{Model: "model", Input: "input", MaxOutputTokens: 10}, nil)
	if !errors.Is(err, ErrResponseFailed) {
		t.Fatalf("Generate() error = %v, want ErrResponseFailed", err)
	}
	if strings.Contains(err.Error(), "sensitive upstream detail") {
		t.Fatalf("Generate() leaked provider detail: %v", err)
	}
}

func TestOpenAIResponsesRequiresCompletedTerminalAndText(t *testing.T) {
	tests := []struct {
		name   string
		stream string
		want   error
	}{
		{name: "stream ended early", stream: "event: response.in_progress\ndata: {\"type\":\"response.in_progress\"}\n\n", want: ErrInvalidStream},
		{name: "completed without text", stream: "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_empty\",\"usage\":{}}}\n\n", want: ErrEmptyResponse},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return sseResponse(test.stream), nil
			})}
			client := NewOpenAIClient(httpClient, "https://api.example.test")
			_, err := client.Generate(context.Background(), "secret", GenerateRequest{Model: "model", Input: "input", MaxOutputTokens: 10}, nil)
			if !errors.Is(err, test.want) {
				t.Fatalf("Generate() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestAnthropicMessagesStreamsTextAndUsage(t *testing.T) {
	var received map[string]any
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("x-api-key") != "secret" || request.Header.Get("anthropic-version") == "" {
			t.Fatalf("missing Anthropic headers: %#v", request.Header)
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		stream := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\",\"usage\":{\"input_tokens\":9}}}\n\n" +
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"审阅\"}}\n\n" +
			"event: ping\ndata: {\"type\":\"ping\"}\n\n" +
			"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":3}}\n\n" +
			"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
		return sseResponse(stream), nil
	})}

	client := NewAnthropicClient(httpClient, "https://api.example.test")
	result, err := client.Generate(context.Background(), "secret", GenerateRequest{
		Model:           "claude-test",
		System:          "system",
		Input:           "input",
		MaxOutputTokens: 100,
		JSONSchema:      json.RawMessage(`{"type":"object"}`),
		SchemaName:      "review",
	}, nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Text != "审阅" || result.InputTokens != 9 || result.OutputTokens != 3 || result.ProviderRequestID != "msg_1" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if received["stream"] != true {
		t.Fatalf("unexpected request: %#v", received)
	}
}

func TestProviderClientsPreserveRequestCancellation(t *testing.T) {
	for _, test := range []struct {
		name     string
		generate func(context.Context) error
	}{
		{
			name: "OpenAI",
			generate: func(ctx context.Context) error {
				client := NewOpenAIClient(cancelledHTTPClient(), "https://api.example.test/v1")
				_, err := client.Generate(ctx, "secret", GenerateRequest{
					Model: "model", Input: "input", MaxOutputTokens: 10,
				}, nil)
				return err
			},
		},
		{
			name: "Anthropic",
			generate: func(ctx context.Context) error {
				client := NewAnthropicClient(cancelledHTTPClient(), "https://api.example.test")
				_, err := client.Generate(ctx, "secret", GenerateRequest{
					Model: "model", Input: "input", MaxOutputTokens: 10,
				}, nil)
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := test.generate(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("Generate() error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestProviderClientsPreserveStreamCancellation(t *testing.T) {
	for _, test := range []struct {
		name   string
		client func(*http.Client) ports.ModelProvider
	}{
		{
			name: "OpenAI",
			client: func(httpClient *http.Client) ports.ModelProvider {
				return NewOpenAIClient(httpClient, "https://api.example.test/v1")
			},
		},
		{
			name: "Anthropic",
			client: func(httpClient *http.Client) ports.ModelProvider {
				return NewAnthropicClient(httpClient, "https://api.example.test")
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			started := make(chan struct{})
			httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
					Body: &cancelledStreamBody{
						ctx: request.Context(), started: started,
					},
				}, nil
			})}
			ctx, cancel := context.WithCancel(context.Background())
			result := make(chan error, 1)
			go func() {
				_, err := test.client(httpClient).Generate(ctx, "secret", GenerateRequest{
					Model: "model", Input: "input", MaxOutputTokens: 10,
				}, nil)
				result <- err
			}()
			<-started
			cancel()
			if err := <-result; !errors.Is(err, context.Canceled) {
				t.Fatalf("Generate() error = %v, want context.Canceled", err)
			}
		})
	}
}

func cancelledHTTPClient() *http.Client {
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
}

type cancelledStreamBody struct {
	ctx     context.Context
	started chan struct{}
	once    sync.Once
}

func (body *cancelledStreamBody) Read([]byte) (int, error) {
	body.once.Do(func() { close(body.started) })
	<-body.ctx.Done()
	return 0, body.ctx.Err()
}

func (*cancelledStreamBody) Close() error {
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func sseResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestFactoryCreatesProtocolSpecificClient(t *testing.T) {
	factory := NewFactory(http.DefaultClient)
	if _, err := factory.New(agentdomain.ProviderConnection{Kind: agentdomain.ProviderOpenAI, BaseURL: "https://example.com/v1"}); err != nil {
		t.Fatalf("OpenAI factory error = %v", err)
	}
	if _, err := factory.New(agentdomain.ProviderConnection{Kind: agentdomain.ProviderAnthropic, BaseURL: "https://example.com"}); err != nil {
		t.Fatalf("Anthropic factory error = %v", err)
	}
}

func TestClassifyStatusMapsStableProviderErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"unauthorized", http.StatusUnauthorized, "", ErrAuthentication},
		{"forbidden", http.StatusForbidden, "", ErrAuthentication},
		{"model not found (openai)", http.StatusNotFound, `{"error":{"code":"model_not_found"}}`, ErrModelNotFound},
		{"model not found (anthropic)", http.StatusNotFound, `{"type":"error","error":{"type":"not_found_error"}}`, ErrModelNotFound},
		{"incompatible endpoint", http.StatusNotFound, "Not Found", ErrIncompatible},
		{"quota 402", http.StatusPaymentRequired, "", ErrQuotaExceeded},
		{"rate limited", http.StatusTooManyRequests, `{"error":{"type":"rate_limit_error"}}`, ErrRateLimited},
		{"quota via 429", http.StatusTooManyRequests, `{"error":{"type":"insufficient_quota"}}`, ErrQuotaExceeded},
		{"content rejected", http.StatusBadRequest, `{"error":{"type":"invalid_request_error","message":"content policy violation"}}`, ErrContentRejected},
		{"server overloaded", http.StatusInternalServerError, `{"error":{"type":"overloaded_error"}}`, ErrRateLimited},
		{"generic server error", http.StatusInternalServerError, "", ErrProvider},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classifyStatus(test.status, []byte(test.body)); !errors.Is(got, test.want) {
				t.Fatalf("classifyStatus(%d, %q) = %v, want %v", test.status, test.body, got, test.want)
			}
		})
	}
}

func TestClassifyNetworkErrorPreservesCancellation(t *testing.T) {
	if got := classifyNetworkError(context.Canceled); !errors.Is(got, context.Canceled) {
		t.Fatalf("canceled = %v", got)
	}
	if got := classifyNetworkError(context.DeadlineExceeded); !errors.Is(got, context.DeadlineExceeded) {
		t.Fatalf("deadline = %v", got)
	}
	if got := classifyNetworkError(errors.New("dial tcp: lookup failed")); !errors.Is(got, ErrNetwork) {
		t.Fatalf("network = %v, want ErrNetwork", got)
	}
}
