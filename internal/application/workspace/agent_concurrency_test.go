package workspace

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// gatedModelProviderFactory produces providers that block on a release channel
// (and ctx.Done) so tests can hold a job inside its Model IO phase while
// exercising the workspace concurrently. started is closed on the first
// Generate call so the test knows the run has reached the provider.
type gatedModelProviderFactory struct {
	outputs map[string]string
	release chan struct{}
	started chan struct{}
	once    sync.Once
}

func newGatedModelProviderFactory(outputs map[string]string) *gatedModelProviderFactory {
	return &gatedModelProviderFactory{
		outputs: outputs,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
}

func (f *gatedModelProviderFactory) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return gatedModelProvider{factory: f}, nil
}

type gatedModelProvider struct {
	factory *gatedModelProviderFactory
}

func (provider gatedModelProvider) Generate(
	ctx context.Context,
	_ string,
	request ports.ModelGenerateRequest,
	onDelta func(string),
) (ports.ModelGenerateResult, error) {
	provider.factory.once.Do(func() { close(provider.factory.started) })
	select {
	case <-provider.factory.release:
	case <-ctx.Done():
		return ports.ModelGenerateResult{}, ctx.Err()
	}
	output := provider.factory.outputs[request.Model]
	if onDelta != nil {
		onDelta(output)
	}
	return ports.ModelGenerateResult{
		Text:              output,
		ProviderRequestID: "request-" + request.Model,
		InputTokens:       10,
		OutputTokens:      5,
	}, nil
}

// gatedAuthoringOutputs returns the structured outputs the three-role authoring
// pipeline expects (planner JSON, writer prose, reviewer JSON).
func gatedAuthoringOutputs() map[string]string {
	return map[string]string{
		"planner":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
		"writer":   "夜色压在山门之上，墨老沿着石阶无声而行。",
		"reviewer": `{"approved":true,"summary":"未发现连续性冲突","issues":[]}`,
	}
}

// waitForProviderStarted blocks until the gated provider signals its first
// Generate call or the test times out, proving the run reached Model IO.
func waitForProviderStarted(t *testing.T, factory *gatedModelProviderFactory) {
	t.Helper()
	select {
	case <-factory.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider never started; run did not reach Model IO")
	}
}

// TestExecuteAgentRunDoesNotBlockCreateChapterDuringProviderCall asserts that a
// chapter creation (which needs the workspace write lock) succeeds while an
// agent run is blocked inside provider.Generate. Today the run holds
// s.mu.RLock() across the whole provider call, so CreateChapter's Lock blocks
// until the run finishes; this must change so Model IO no longer holds the
// workspace lock.
func TestExecuteAgentRunDoesNotBlockCreateChapterDuringProviderCall(t *testing.T) {
	ctx := context.Background()
	factory := newGatedModelProviderFactory(gatedAuthoringOutputs())
	service, chapter := configuredAgentService(t, factory)
	defer service.Close()

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}

	runDone := make(chan struct{})
	go func() {
		_, _ = service.ExecuteAgentRun(ctx, run.ID)
		close(runDone)
	}()

	waitForProviderStarted(t, factory)

	// CreateChapter needs the workspace write lock. While the provider call is
	// in flight it must succeed rather than wait for the run to finish.
	createDone := make(chan struct{})
	var created ChapterView
	var createErr error
	go func() {
		created, createErr = service.CreateChapter(ctx, "第二章")
		close(createDone)
	}()
	select {
	case <-createDone:
		if createErr != nil {
			t.Fatalf("CreateChapter during provider call: %v", createErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CreateChapter was blocked behind the in-flight provider call")
	}

	close(factory.release)
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteAgentRun did not complete after releasing the provider")
	}
	if created.ID == "" {
		t.Fatal("created chapter has no id")
	}
}

// TestExecuteAgentRunDoesNotBlockSaveDraftDuringProviderCall is a regression
// guard: SaveDraft uses a read lock and must continue to succeed while a
// provider call is in flight, both before and after the lock split.
func TestExecuteAgentRunDoesNotBlockSaveDraftDuringProviderCall(t *testing.T) {
	ctx := context.Background()
	factory := newGatedModelProviderFactory(gatedAuthoringOutputs())
	service, chapter := configuredAgentService(t, factory)
	defer service.Close()

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}

	runDone := make(chan struct{})
	go func() {
		_, _ = service.ExecuteAgentRun(ctx, run.ID)
		close(runDone)
	}()

	waitForProviderStarted(t, factory)

	saveDone := make(chan error, 1)
	go func() {
		saveDone <- service.SaveDraft(ctx, chapter.ID, "新恢复草稿", chapter.Revision)
	}()
	select {
	case err := <-saveDone:
		if err != nil {
			t.Fatalf("SaveDraft during provider call: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SaveDraft was blocked behind the in-flight provider call")
	}

	close(factory.release)
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteAgentRun did not complete after releasing the provider")
	}

	view, err := service.OpenChapter(ctx, chapter.ID)
	if err != nil {
		t.Fatalf("OpenChapter: %v", err)
	}
	if !view.HasRecoveryDraft {
		t.Fatal("recovery draft was not persisted")
	}
}
