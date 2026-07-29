package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
)

func TestAgentAuthoringCreatesReviewableChapterChangeSet(t *testing.T) {
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"planner-model":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
			"writer-model":   "夜色压在山门之上，墨老沿着石阶无声而行。",
			"reviewer-model": `{"approved":true,"summary":"未发现连续性冲突","issues":[]}`,
		},
	}
	ids := &sequenceIDs{ids: []string{
		"project-1", "chapter-1",
		"provider-1", "provider-2",
		"profile-1", "profile-2", "profile-3",
		"run-1", "step-1", "step-2", "step-3",
		"artifact-context", "artifact-plan", "artifact-draft", "artifact-review",
		"change-set-1", "operation-1",
	}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(200, 0)},
		ids,
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	project, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "墨老企图夺舍主角")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	chapter, err := service.CreateChapter(ctx, "第一章")
	if err != nil {
		t.Fatal(err)
	}

	openAI, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	anthropic, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "Anthropic", Kind: "anthropic", BaseURL: "https://api.anthropic.com", APIKey: "key-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []AgentProfileInput{
		{Role: "planner", ConnectionID: openAI.ID, Model: "planner-model", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
		{Role: "writer", ConnectionID: openAI.ID, Model: "writer-model", MaxOutputTokens: 3000, Temperature: 0.7, Enabled: true},
		{Role: "continuity_reviewer", ConnectionID: anthropic.ID, Model: "reviewer-model", MaxOutputTokens: 1000, Temperature: 0.1, Enabled: true},
	} {
		if _, err := service.SaveAgentProfile(ctx, input); err != nil {
			t.Fatal(err)
		}
	}

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章，保持克制的第三人称。")
	if err != nil {
		t.Fatal(err)
	}
	run, err = service.ExecuteAgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" || run.ChangeSetID == "" {
		t.Fatalf("unexpected run: %#v", run)
	}
	if len(providerFactory.requests) != 3 {
		t.Fatalf("provider calls = %d", len(providerFactory.requests))
	}
	if providerFactory.requests[1].Input == "" || providerFactory.requests[2].Input == "" {
		t.Fatal("writer and reviewer must receive assembled context")
	}
	changeSets, err := service.ChangeSets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(changeSets) != 1 ||
		changeSets[0].Operations[0].AfterValue != providerFactory.outputs["writer-model"] {
		t.Fatalf("unexpected change sets: %#v", changeSets)
	}
	if project.ID == "" {
		t.Fatal("project id missing")
	}
}

func TestCreateChapterAgentRunRequiresIndependentReviewer(t *testing.T) {
	profiles := []agentdomain.AgentProfile{
		{Role: agentdomain.RolePlanner, ConnectionID: "one", Model: "model", Enabled: true},
		{Role: agentdomain.RoleWriter, ConnectionID: "one", Model: "model", Enabled: true},
		{Role: agentdomain.RoleContinuityReviewer, ConnectionID: "one", Model: "model", Enabled: true},
	}
	if err := agentdomain.ValidateAuthoringProfiles(profiles); !errors.Is(err, agentdomain.ErrReviewerNotIndependent) {
		t.Fatalf("expected independent reviewer error, got %v", err)
	}
}

type fakeCredentialStore struct {
	secrets map[string]string
}

func (store *fakeCredentialStore) Save(_ context.Context, reference, secret string) error {
	store.secrets[reference] = secret
	return nil
}

func (store *fakeCredentialStore) Get(_ context.Context, reference string) (string, error) {
	value, exists := store.secrets[reference]
	if !exists {
		return "", errors.New("credential missing")
	}
	return value, nil
}

func (store *fakeCredentialStore) Delete(_ context.Context, reference string) error {
	delete(store.secrets, reference)
	return nil
}

type capturedModelRequest struct {
	Model string
	Input string
}

type fakeModelProviderFactory struct {
	outputs  map[string]string
	requests []capturedModelRequest
}

func (factory *fakeModelProviderFactory) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return fakeModelProvider{factory: factory}, nil
}

type fakeModelProvider struct {
	factory *fakeModelProviderFactory
}

func (provider fakeModelProvider) Generate(
	_ context.Context,
	_ string,
	request ports.ModelGenerateRequest,
	onDelta func(string),
) (ports.ModelGenerateResult, error) {
	provider.factory.requests = append(provider.factory.requests, capturedModelRequest{
		Model: request.Model,
		Input: request.Input,
	})
	output := provider.factory.outputs[request.Model]
	if onDelta != nil {
		onDelta(output)
	}
	return ports.ModelGenerateResult{Text: output, ProviderRequestID: "request-" + request.Model}, nil
}
