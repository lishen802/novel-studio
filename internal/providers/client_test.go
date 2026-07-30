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
