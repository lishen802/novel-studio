package agentkernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/providers"
)

func TestServiceRunsFrozenWorkflowAndPersistsTrace(t *testing.T) {
	service, store, provider := newRunnerHarness(t, []providerReply{{
		text:        `{"title":"雨夜","body":"钟声穿过雨幕。","completion_note":"完成"}`,
		inputTokens: 20, outputTokens: 30,
	}})
	run := createBaselineRun(t, service)
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatalf("ExecuteRun: %v", err)
	}
	if completed.Status != agentdomain.RunCompleted || completed.QualityStatus != agentdomain.QualityPassed {
		t.Fatalf("completed run = %#v", completed)
	}
	if len(provider.requests) != 1 || provider.requests[0].Model != "writer-model" {
		t.Fatalf("provider requests = %#v", provider.requests)
	}
	if len(store.manifests) != 1 || len(store.artifacts) != 5 {
		t.Fatalf("trace manifests=%d artifacts=%d", len(store.manifests), len(store.artifacts))
	}
	loaded := store.runs[run.ID]
	if loaded.InputTokens != 20 || loaded.OutputTokens != 30 || loaded.Steps[0].PrimaryArtifactID == "" {
		t.Fatalf("usage or primary artifact missing: %#v", loaded)
	}
	assertEventTypes(t, store.events, agentdomain.EventJobStarted, agentdomain.EventStepStarted, agentdomain.EventStepCompleted, agentdomain.EventJobCompleted)
}

func TestServiceRepairsInvalidJSONOnceAndAccountsUsage(t *testing.T) {
	service, store, provider := newRunnerHarness(t, []providerReply{
		{text: `{"title":`, inputTokens: 10, outputTokens: 5},
		{text: `{"title":"修复","body":"正文未改变。","completion_note":"完成"}`, inputTokens: 8, outputTokens: 12},
	})
	run := createBaselineRun(t, service)
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatalf("ExecuteRun: %v", err)
	}
	if len(provider.requests) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(provider.requests))
	}
	if completed.InputTokens != 18 || completed.OutputTokens != 17 {
		t.Fatalf("repair usage = %d/%d", completed.InputTokens, completed.OutputTokens)
	}
	if countArtifacts(store.artifacts, agentdomain.ArtifactFormatRepair) != 1 {
		t.Fatalf("format repair artifact missing: %#v", store.artifacts)
	}
}

func TestServiceRejectsInsufficientFrozenBudgetBeforeProviderCall(t *testing.T) {
	service, _, provider := newRunnerHarness(t, nil)
	request := baselineRequest()
	request.Profile.MaxRunTokens = 5_500
	_, err := service.CreateRun(context.Background(), request)
	if !errors.Is(err, ErrTokenBudget) {
		t.Fatalf("CreateRun error = %v", err)
	}
	if len(provider.requests) != 0 {
		t.Fatal("provider was called during rejected creation")
	}
}

func TestServiceRejectsRequiredContextThatCannotFit(t *testing.T) {
	service, _, provider := newRunnerHarness(t, nil)
	request := baselineRequest()
	request.Profile.MaxContextChars = 5
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if !errors.Is(err, ErrContextBudget) || failed.Status != agentdomain.RunFailed {
		t.Fatalf("ExecuteRun = %#v, %v", failed, err)
	}
	if len(provider.requests) != 0 {
		t.Fatal("context overflow reached provider")
	}
}

func TestServiceKeepsProviderDetailsOutOfPersistedError(t *testing.T) {
	service, store, _ := newRunnerHarness(t, []providerReply{{err: errors.New("secret upstream response")}})
	run := createBaselineRun(t, service)
	failed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err == nil || failed.Status != agentdomain.RunFailed || failed.ErrorCode != "provider_error" {
		t.Fatalf("ExecuteRun = %#v, %v", failed, err)
	}
	loaded := store.runs[run.ID]
	if loaded.ErrorMessage == "secret upstream response" || loaded.Steps[0].ErrorMessage == "secret upstream response" {
		t.Fatalf("provider response leaked: %#v", loaded)
	}
}

func TestProviderTerminalFailuresHaveStablePublicMessages(t *testing.T) {
	tests := []struct {
		err     error
		code    string
		message string
	}{
		{providers.ErrResponseIncomplete, "provider_response_incomplete", "模型输出达到上限而未完成，请提高最大输出 Token 后重试。"},
		{providers.ErrResponseFailed, "provider_response_failed", "模型服务未能完成生成，请稍后重试。"},
		{providers.ErrEmptyResponse, "provider_empty_response", "模型返回了空内容，请检查模型能力或生成参数。"},
		{providers.ErrInvalidStream, "provider_invalid_stream", "模型流式响应提前中断或格式不兼容。"},
	}
	for _, test := range tests {
		if got := errorCode(test.err); got != test.code {
			t.Fatalf("errorCode(%v) = %q, want %q", test.err, got, test.code)
		}
		if got := safeErrorMessage(test.code); got != test.message {
			t.Fatalf("safeErrorMessage(%q) = %q, want %q", test.code, got, test.message)
		}
	}
}

func TestServiceSkipsConditionalNodeWithoutProviderCall(t *testing.T) {
	registry := twoNodeRegistry(t, "on_quality_issue")
	store := newMemoryStore()
	provider := &sequenceProvider{replies: []providerReply{{
		text:        `{"chapter_goal":"潜入","opening_hook":"钟响","ending_hook":"门开","scenes":[{"id":"s1","goal":"入门","conflict":"守卫","outcome":"成功"}]}`,
		inputTokens: 10, outputTokens: 20,
	}}}
	service := NewService(store, credentialStub{}, providerFactoryStub{provider}, Registries{
		Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
	}, fixedRunnerClock{}, &sequenceIDs{}, nil)
	request := baselineRequest()
	request.WorkflowID = "conditional"
	request.Profile.Roles = append(request.Profile.Roles, resolvedRole(agentdomain.RolePlanner, "planner-model"))
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(provider.requests) != 1 {
		t.Fatalf("provider calls = %d, conditional writer should be skipped", len(provider.requests))
	}
	if completed.Steps[1].Status != agentdomain.StepCompleted || completed.Steps[1].InputTokens != 0 {
		t.Fatalf("skipped step = %#v", completed.Steps[1])
	}
}

func TestServiceExecutesNodesInDependencyOrderWithUpstreamArtifact(t *testing.T) {
	registry := twoNodeRegistry(t, "always")
	store := newMemoryStore()
	provider := &sequenceProvider{replies: []providerReply{
		{text: `{"chapter_goal":"潜入","opening_hook":"钟响","ending_hook":"门开","scenes":[{"id":"s1","goal":"入门","conflict":"守卫","outcome":"成功"}]}`, inputTokens: 10, outputTokens: 20},
		{text: `{"title":"门后","body":"石门无声开启。","completion_note":"完成"}`, inputTokens: 15, outputTokens: 25},
	}}
	service := NewService(store, credentialStub{}, providerFactoryStub{provider}, Registries{
		Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
	}, fixedRunnerClock{}, &sequenceIDs{}, nil)
	request := baselineRequest()
	request.WorkflowID = "conditional"
	request.Profile.Roles = append(request.Profile.Roles, resolvedRole(agentdomain.RolePlanner, "planner-model"))
	run, err := service.CreateRun(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != agentdomain.RunCompleted || len(provider.requests) != 2 {
		t.Fatalf("completed=%#v requests=%d", completed, len(provider.requests))
	}
	if provider.requests[0].Model != "planner-model" || provider.requests[1].Model != "writer-model" ||
		!strings.Contains(provider.requests[1].Input, `"chapter_goal":"潜入"`) {
		t.Fatalf("dependency order/input = %#v", provider.requests)
	}
}

func TestServiceResumeCreatesNewAttemptAndDoesNotRepeatCompletedNode(t *testing.T) {
	service, store, provider := newRunnerHarness(t, []providerReply{{
		text:        `{"title":"恢复","body":"只调用新 attempt。","completion_note":"完成"}`,
		inputTokens: 9, outputTokens: 11,
	}})
	run := createBaselineRun(t, service)
	interrupted := run.Steps[0]
	interrupted.Status = agentdomain.StepFailed
	interrupted.ErrorCode = "app_interrupted"
	interrupted.ErrorMessage = "应用退出导致步骤中断。"
	interrupted.QualityStatus = agentdomain.QualityBlocked
	now := fixedRunnerClock{}.Now()
	interrupted.CompletedAt = &now
	interrupted.UpdatedAt = now
	run.Steps[0] = interrupted
	run.Status = agentdomain.RunRecoverable
	store.runs[run.ID] = cloneRun(run)

	completed, err := service.ResumeRun(context.Background(), run.ID, "head-1")
	if err != nil {
		t.Fatalf("ResumeRun: %v", err)
	}
	if completed.Status != agentdomain.RunCompleted || len(completed.Steps) != 2 {
		t.Fatalf("resumed run = %#v", completed)
	}
	if completed.Steps[0].Attempt != 1 || completed.Steps[1].Attempt != 2 || len(provider.requests) != 1 {
		t.Fatalf("attempts/calls = %#v / %d", completed.Steps, len(provider.requests))
	}
}

func TestServiceCancellationPreservesArtifactsAndCancelsRun(t *testing.T) {
	service, store, provider := newRunnerHarness(t, []providerReply{{err: context.Canceled}})
	run := createBaselineRun(t, service)
	cancelled, err := service.ExecuteRun(context.Background(), run.ID, "head-1")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecuteRun error = %v", err)
	}
	if cancelled.Status != agentdomain.RunCancelled || len(provider.requests) != 1 {
		t.Fatalf("cancelled run = %#v", cancelled)
	}
	if store.runs[run.ID].Steps[0].Status != agentdomain.StepCancelled {
		t.Fatalf("step status = %s", store.runs[run.ID].Steps[0].Status)
	}
}

func TestServiceRejectsChangedCanonicalHead(t *testing.T) {
	service, _, provider := newRunnerHarness(t, nil)
	run := createBaselineRun(t, service)
	_, err := service.ExecuteRun(context.Background(), run.ID, "head-2")
	if !errors.Is(err, ErrRunStale) {
		t.Fatalf("ExecuteRun error = %v", err)
	}
	if len(provider.requests) != 0 {
		t.Fatal("stale run reached provider")
	}
}

func newRunnerHarness(t *testing.T, replies []providerReply) (*Service, *memoryStore, *sequenceProvider) {
	t.Helper()
	registry, err := agentassets.Builtin()
	if err != nil {
		t.Fatal(err)
	}
	store := newMemoryStore()
	provider := &sequenceProvider{replies: replies}
	service := NewService(store, credentialStub{}, providerFactoryStub{provider}, Registries{
		Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
	}, fixedRunnerClock{}, &sequenceIDs{}, nil)
	return service, store, provider
}

func createBaselineRun(t *testing.T, service *Service) agentdomain.Run {
	t.Helper()
	run, err := service.CreateRun(context.Background(), baselineRequest())
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	return run
}

func baselineRequest() CreateRequest {
	return CreateRequest{
		RunID: "run-1", ProjectID: "project-1", BranchID: "main", ChapterID: "chapter-1",
		ChapterBaseRevision: 1, CanonicalHead: "head-1", AuthorInstruction: "写一个克制的雨夜开篇。",
		WorkflowID: agentassets.BaselineWorkflowID, WorkflowVersion: agentassets.BaselineWorkflowVersion,
		Profile: agentdomain.ResolvedCreativeProfile{
			ProfileID: "profile-1", Version: 1, MaxRunTokens: 6_500, MaxContextChars: 10_000,
			Roles: []agentdomain.ResolvedRole{resolvedRole(agentdomain.RoleWriter, "writer-model")},
		},
		Sources: []Source{{
			Kind: agentdomain.ManifestHarnessSpec, ID: "case-1", Version: "1", Content: "主角不能说谎。",
			Priority: 900, SelectionReason: "固定测试规格", Required: true,
		}},
		Deadline: fixedRunnerClock{}.Now().Add(time.Hour),
	}
}

func resolvedRole(role agentdomain.Role, model string) agentdomain.ResolvedRole {
	return agentdomain.ResolvedRole{
		Role: role, ConnectionID: "connection-" + string(role), ProviderName: "测试连接",
		ProviderKind: agentdomain.ProviderOpenAI, BaseURL: "https://api.example.com/v1",
		CredentialRef: "credential-" + string(role), Model: model, MaxOutputTokens: 5_000, Temperature: 0.4,
	}
}

func twoNodeRegistry(t *testing.T, condition string) *agentassets.Registry {
	t.Helper()
	planPrompt := "任务：{{.Task}}"
	writePrompt := "细纲：{{.Plan}}"
	planBinding := agentassets.BindingForText("conditional.plan.prompt", 1, []byte(planPrompt))
	writeBinding := agentassets.BindingForText("conditional.write.prompt", 1, []byte(writePrompt))
	planSchema := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false}`)
	draftSchema := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false}`)
	planSchemaBinding := agentassets.BindingForText("conditional.plan.schema", 1, planSchema)
	draftSchemaBinding := agentassets.BindingForText("conditional.draft.schema", 1, draftSchema)
	assemblerJSON := []byte(`{"strategy":"json","version":1}`)
	assembler := agentassets.BindingForText("conditional.assembler", 1, assemblerJSON)
	definition := agentdomain.WorkflowDefinition{
		ID: "conditional", Version: 1, TaskKind: agentdomain.TaskChapterDraft,
		EntryNodeID: "plan", ResultNodeID: "write", BudgetPolicyVersion: 1, QualityPolicyVersion: 1,
		MaxRevisionAttempts: 1, MaxRunTokens: 6_500, RepairReserve: 500, RevisionReserve: 500,
		Nodes: []agentdomain.WorkflowNode{
			{ID: "plan", PublicStageID: agentdomain.PublicStageChapterDirecting, Capability: agentdomain.CapabilityChapterPlanner, RoleSlot: agentdomain.RolePlanner,
				Prompt: planBinding, Schema: planSchemaBinding, Assembler: assembler, InputSelectors: []string{"$task"},
				OutputKind: agentdomain.OutputStructured, MaxOutputTokens: 2_000, FailurePolicy: agentdomain.FailureFailRun},
			{ID: "write", PublicStageID: agentdomain.PublicStageProseWriting, Capability: agentdomain.CapabilityProseWriter, RoleSlot: agentdomain.RoleWriter,
				Prompt: writeBinding, Schema: draftSchemaBinding, Assembler: assembler, InputSelectors: []string{"node:plan"},
				OutputKind: agentdomain.OutputPrimaryContent, MaxOutputTokens: 3_000, Condition: condition, FailurePolicy: agentdomain.FailureFailRun},
		},
		Edges: []agentdomain.WorkflowEdge{{From: "plan", To: "write"}},
	}
	definition.DefinitionHash = definition.ContentHash()
	registry, err := agentassets.New(
		[]ports.WorkflowAsset{{Binding: agentdomain.VersionBinding{ID: definition.ID, Version: 1, Hash: definition.DefinitionHash}, Definition: definition}},
		[]ports.PromptAsset{{Binding: planBinding, Template: planPrompt, Variables: []string{"Task"}}, {Binding: writeBinding, Template: writePrompt, Variables: []string{"Plan"}}},
		[]ports.SchemaAsset{{Binding: planSchemaBinding, JSON: planSchema}, {Binding: draftSchemaBinding, JSON: draftSchema}},
		[]ports.AssemblerAsset{{Binding: assembler, JSON: assemblerJSON}},
	)
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

type providerReply struct {
	text                      string
	inputTokens, outputTokens int
	err                       error
}

type sequenceProvider struct {
	mu       sync.Mutex
	replies  []providerReply
	requests []ports.ModelGenerateRequest
}

func (provider *sequenceProvider) Generate(_ context.Context, _ string, request ports.ModelGenerateRequest, _ func(string)) (ports.ModelGenerateResult, error) {
	provider.mu.Lock()
	defer provider.mu.Unlock()
	provider.requests = append(provider.requests, request)
	if len(provider.replies) == 0 {
		return ports.ModelGenerateResult{}, errors.New("unexpected provider call")
	}
	reply := provider.replies[0]
	provider.replies = provider.replies[1:]
	return ports.ModelGenerateResult{
		Text: reply.text, ProviderRequestID: fmt.Sprintf("request-%d", len(provider.requests)),
		InputTokens: reply.inputTokens, OutputTokens: reply.outputTokens,
	}, reply.err
}

type providerFactoryStub struct{ provider ports.ModelProvider }

func (factory providerFactoryStub) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return factory.provider, nil
}

type credentialStub struct{}

func (credentialStub) Save(context.Context, string, string) error  { return nil }
func (credentialStub) Get(context.Context, string) (string, error) { return "secret", nil }
func (credentialStub) Delete(context.Context, string) error        { return nil }

type fixedRunnerClock struct{}

func (fixedRunnerClock) Now() time.Time { return time.Unix(10_000, 0).UTC() }

type sequenceIDs struct {
	mu   sync.Mutex
	next int
}

func (ids *sequenceIDs) NewID() string {
	ids.mu.Lock()
	defer ids.mu.Unlock()
	ids.next++
	return fmt.Sprintf("id-%d", ids.next)
}

type memoryStore struct {
	mu        sync.Mutex
	runs      map[string]agentdomain.Run
	artifacts []agentdomain.Artifact
	manifests []agentdomain.ContextManifest
	findings  []agentdomain.ReviewFindingRecord
	events    []agentdomain.JobEventDraft
}

func newMemoryStore() *memoryStore { return &memoryStore{runs: map[string]agentdomain.Run{}} }

func (store *memoryStore) CreateAgentRunBundle(_ context.Context, bundle agentdomain.RunBundle) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.runs[bundle.Run.ID] = cloneRun(bundle.Run)
	store.artifacts = append(store.artifacts, bundle.Artifacts...)
	store.manifests = append(store.manifests, bundle.Manifests...)
	store.findings = append(store.findings, bundle.Findings...)
	return nil
}

func (store *memoryStore) AgentRun(_ context.Context, id string) (agentdomain.Run, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	run, ok := store.runs[id]
	if !ok {
		return agentdomain.Run{}, errors.New("run missing")
	}
	return cloneRun(run), nil
}

func (store *memoryStore) UpdateAgentRun(_ context.Context, run agentdomain.Run, drafts ...agentdomain.JobEventDraft) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.runs[run.ID] = cloneRun(run)
	store.events = append(store.events, drafts...)
	return nil
}

func (store *memoryStore) UpdateAgentRunStep(_ context.Context, step agentdomain.RunStep, drafts ...agentdomain.JobEventDraft) error {
	return store.saveStep(step, nil, nil, nil, drafts)
}

func (store *memoryStore) AppendAgentRunStep(_ context.Context, step agentdomain.RunStep, drafts ...agentdomain.JobEventDraft) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	run := store.runs[step.RunID]
	run.Steps = append(run.Steps, step)
	store.runs[step.RunID] = cloneRun(run)
	store.events = append(store.events, drafts...)
	return nil
}

func (store *memoryStore) SaveAgentStepBundle(_ context.Context, bundle agentdomain.StepBundle, drafts ...agentdomain.JobEventDraft) error {
	return store.saveStep(bundle.Step, bundle.Artifacts, bundle.Manifests, bundle.Findings, drafts)
}

func (store *memoryStore) saveStep(step agentdomain.RunStep, artifacts []agentdomain.Artifact, manifests []agentdomain.ContextManifest, findings []agentdomain.ReviewFindingRecord, drafts []agentdomain.JobEventDraft) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	run := store.runs[step.RunID]
	found := false
	for index := range run.Steps {
		if run.Steps[index].ID == step.ID {
			run.Steps[index] = step
			found = true
			break
		}
	}
	if !found {
		return errors.New("step missing")
	}
	store.runs[step.RunID] = cloneRun(run)
	store.artifacts = append(store.artifacts, artifacts...)
	store.manifests = append(store.manifests, manifests...)
	store.findings = append(store.findings, findings...)
	store.events = append(store.events, drafts...)
	return nil
}

func (store *memoryStore) ListAgentArtifacts(_ context.Context, runID string) ([]agentdomain.Artifact, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]agentdomain.Artifact, 0)
	for _, artifact := range store.artifacts {
		if artifact.RunID == runID {
			result = append(result, artifact)
		}
	}
	return result, nil
}

func cloneRun(run agentdomain.Run) agentdomain.Run {
	run.Steps = append([]agentdomain.RunStep(nil), run.Steps...)
	return run
}

func countArtifacts(artifacts []agentdomain.Artifact, kind agentdomain.ArtifactKind) int {
	count := 0
	for _, artifact := range artifacts {
		if artifact.Kind == kind {
			count++
		}
	}
	return count
}

func assertEventTypes(t *testing.T, events []agentdomain.JobEventDraft, expected ...agentdomain.JobEventType) {
	t.Helper()
	if len(events) != len(expected) {
		t.Fatalf("events = %#v", events)
	}
	for index, eventType := range expected {
		if events[index].EventType != eventType {
			t.Fatalf("event %d = %s, want %s", index, events[index].EventType, eventType)
		}
	}
}
