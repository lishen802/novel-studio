package creation

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestPreviewGenerateDigestIsStableAndRequiredOnConfirm(t *testing.T) {
	service, _, _ := newTestService(t, newFakeKernel(), nil)
	seedIntentSession(t, service)
	request := generateRequest(creationdomain.StageDirections)
	first, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfirmationDigest == "" || first.ConfirmationDigest != second.ConfirmationDigest {
		t.Fatalf("digest unstable: %#v vs %#v", first, second)
	}
	if first.EstimatedCalls != 2 || first.WorkflowID != agentassets.StoryDirectionsWorkflowID {
		t.Fatalf("preview = %#v", first)
	}
	request.ConfirmationDigest = "stale-digest"
	if _, err := service.ConfirmGenerate(context.Background(), request); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("stale digest error = %v", err)
	}
}

func TestConfirmGeneratePersistsDirectionCandidate(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedIntentSession(t, service)
	request := generateRequest(creationdomain.StageDirections)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "directions-1"
	result, err := service.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidate.Stage != creationdomain.StageDirections || result.Candidate.Kind != creationdomain.KindDirectionSet || result.Candidate.Version != 1 {
		t.Fatalf("candidate = %#v", result.Candidate)
	}
	if result.Session.Stage != creationdomain.StageDirections || result.Session.ActiveRunID != "" {
		t.Fatalf("session = %#v", result.Session)
	}
	listed, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil || len(listed) != 1 || listed[0].ID != result.Candidate.ID {
		t.Fatalf("listed = %#v, err=%v", listed, err)
	}
	if kernel.executeN != 1 {
		t.Fatalf("executeN = %d", kernel.executeN)
	}
	again, err := service.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if again.Candidate.ID != result.Candidate.ID || kernel.executeN != 1 {
		t.Fatalf("idempotent retry ran again: %#v executeN=%d", again, kernel.executeN)
	}
}

func TestCancelGenerateDoesNotPersistCandidate(t *testing.T) {
	kernel := newFakeKernel()
	kernel.block = make(chan struct{})
	kernel.started = make(chan struct{})
	service, store, _ := newTestService(t, kernel, nil)
	session := seedIntentSession(t, service)
	request := generateRequest(creationdomain.StageDirections)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	done := make(chan error, 1)
	go func() {
		_, confirmErr := service.ConfirmGenerate(context.Background(), request)
		done <- confirmErr
	}()
	select {
	case <-kernel.started:
	case <-time.After(2 * time.Second):
		t.Fatal("execute did not start")
	}
	if err := service.CancelRun(context.Background(), request.ProjectID); err != nil {
		t.Fatal(err)
	}
	err = <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error = %v", err)
	}
	listed, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil || len(listed) != 0 {
		t.Fatalf("candidates after cancel = %#v, err=%v", listed, err)
	}
}

func TestRecoverDoesNotResumePaidRun(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedIntentSession(t, service)
	session.ActiveRunID = "run-stale"
	if err := store.SaveSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.Recover(context.Background(), session.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.BlockingReason != creationdomain.BlockCostAuthorization || recovered.ActiveRunID != "run-stale" {
		t.Fatalf("recovered = %#v", recovered)
	}
	if kernel.createN != 0 || kernel.executeN != 0 {
		t.Fatalf("recover must not call kernel: create=%d execute=%d", kernel.createN, kernel.executeN)
	}
}

func TestAutoAdvanceStopsBeforeArchiveConfirm(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedProposalSession(t, service, store)
	if _, err := service.SetAdvanceMode(context.Background(), session.ProjectID, creationdomain.ModeAutoAdvance); err != nil {
		t.Fatal(err)
	}
	request := generateRequest(creationdomain.StageProposal)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if preview.EstimatedCalls != 7 || len(preview.RemainingStages) != 2 {
		t.Fatalf("auto preview = %#v", preview)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	result, err := service.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Session.Stage != creationdomain.StageRoadmap || result.Session.BlockingReason != creationdomain.BlockFormalWrite {
		t.Fatalf("session = %#v", result.Session)
	}
	if result.Session.Stage == creationdomain.StageArchiveConfirm {
		t.Fatal("auto advance must not enter archive confirm")
	}
	listed, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[creationdomain.CandidateKind]int{}
	for _, candidate := range listed {
		if candidate.Status == creationdomain.StatusSuperseded {
			continue
		}
		kinds[candidate.Kind]++
	}
	if kinds[creationdomain.KindProposal] != 1 || kinds[creationdomain.KindPeopleWorld] != 1 || kinds[creationdomain.KindRoadmap] != 1 {
		t.Fatalf("candidates = %#v", listed)
	}
	if kernel.executeN != 3 {
		t.Fatalf("executeN = %d, want 3", kernel.executeN)
	}
}

func TestConfirmGenerateRejectsStaleStoreGeneration(t *testing.T) {
	var generation atomic.Uint64
	generation.Store(1)
	kernel := newFakeKernel()
	kernel.onExecute = func() { generation.Add(1) }
	service, store, _ := newTestService(t, kernel, generation.Load)
	session := seedIntentSession(t, service)
	request := generateRequest(creationdomain.StageDirections)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	if _, err := service.ConfirmGenerate(context.Background(), request); !errors.Is(err, ErrStaleStore) {
		t.Fatalf("stale store error = %v", err)
	}
	listed, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil || len(listed) != 0 {
		t.Fatalf("candidates after stale = %#v, err=%v", listed, err)
	}
}

func seedIntentSession(t *testing.T, service *Service) creationdomain.Session {
	t.Helper()
	session, err := service.EnsureSession(context.Background(), "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	session, err = service.SaveIntent(context.Background(), session.ProjectID, "守关人必须放走仇敌")
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func seedProposalSession(t *testing.T, service *Service, store *memoryStore) creationdomain.Session {
	t.Helper()
	session := seedIntentSession(t, service)
	now := testClock{}.Now()
	session, err := session.Advance(now)
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := creationdomain.NewCandidate(
		"dir-set-1", session.ID, creationdomain.StageDirections, creationdomain.KindDirectionSet,
		1, "", "run-dir", "art-dir", directionPayload, "story_directions.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCandidate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}
	session, err = session.SelectDirection(candidate.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.Advance(now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	return session
}

func generateRequest(stage creationdomain.Stage) GenerateRequest {
	return GenerateRequest{
		ProjectID:     "project-1",
		BranchID:      "branch-1",
		ChapterID:     "chapter-1",
		CanonicalHead: "head-1",
		Stage:         stage,
		Profile: agentdomain.ResolvedCreativeProfile{
			ProfileID: "profile-1", Version: 1, MaxRunTokens: 20000, MaxContextChars: 120000,
		},
	}
}

func newTestService(t *testing.T, kernel *fakeKernel, generation func() uint64) (*Service, *memoryStore, *fakeKernel) {
	t.Helper()
	store := newMemoryStore()
	service, err := New(Config{
		Store: store, Kernel: kernel, Clock: testClock{}, IDs: &testIDs{}, Generation: generation,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, store, kernel
}

type fakeKernel struct {
	mu        sync.Mutex
	createN   int
	executeN  int
	payloads  map[string]string
	runs      map[string]agentdomain.Run
	artifacts map[string][]agentdomain.Artifact
	block     chan struct{}
	started   chan struct{}
	onExecute func()
}

func newFakeKernel() *fakeKernel {
	return &fakeKernel{
		payloads: map[string]string{
			agentassets.StoryDirectionsWorkflowID:  directionPayload,
			agentassets.StoryProposalWorkflowID:    proposalPayload,
			agentassets.StoryPeopleWorldWorkflowID: peopleWorldPayload,
			agentassets.StoryRoadmapWorkflowID:     roadmapPayload,
		},
		runs:      map[string]agentdomain.Run{},
		artifacts: map[string][]agentdomain.Artifact{},
	}
}

func (kernel *fakeKernel) CreateRun(_ context.Context, request agentkernel.CreateRequest) (agentdomain.Run, error) {
	kernel.mu.Lock()
	defer kernel.mu.Unlock()
	kernel.createN++
	artifactID := request.RunID + "-art"
	run := agentdomain.Run{
		ID: request.RunID, ProjectID: request.ProjectID, Status: agentdomain.RunCompleted,
		Steps: []agentdomain.RunStep{{
			ID: request.RunID + "-step", RunID: request.RunID,
			Capability: agentdomain.CapabilityStoryArchitect, Status: agentdomain.StepCompleted,
			PrimaryArtifactID: artifactID,
		}},
	}
	payload, ok := kernel.payloads[request.WorkflowID]
	if !ok {
		return agentdomain.Run{}, fmt.Errorf("missing payload for %s", request.WorkflowID)
	}
	kernel.runs[run.ID] = run
	kernel.artifacts[run.ID] = []agentdomain.Artifact{{
		ID: artifactID, RunID: run.ID, Content: payload,
	}}
	return run, nil
}

func (kernel *fakeKernel) ExecuteRun(ctx context.Context, runID, _ string) (agentdomain.Run, error) {
	kernel.mu.Lock()
	kernel.executeN++
	started := kernel.started
	block := kernel.block
	onExecute := kernel.onExecute
	run := kernel.runs[runID]
	kernel.mu.Unlock()
	if onExecute != nil {
		onExecute()
	}
	if started != nil {
		select {
		case <-started:
		default:
			close(started)
		}
	}
	if block != nil {
		select {
		case <-ctx.Done():
			return agentdomain.Run{}, ctx.Err()
		case <-block:
		}
	}
	return run, nil
}

func (kernel *fakeKernel) Artifacts(_ context.Context, runID string) ([]agentdomain.Artifact, error) {
	kernel.mu.Lock()
	defer kernel.mu.Unlock()
	return kernel.artifacts[runID], nil
}

type testClock struct{}

func (testClock) Now() time.Time { return time.Unix(1_700_000_000, 0).UTC() }

type testIDs struct {
	mu   sync.Mutex
	next int
}

func (ids *testIDs) NewID() string {
	ids.mu.Lock()
	defer ids.mu.Unlock()
	ids.next++
	return fmt.Sprintf("id-%d", ids.next)
}

const (
	directionPayload   = `{"directions":[{"id":"d1","hook":"守关人放走仇敌","genre":"奇幻悬疑","protagonist":"守关人阿绫","core_conflict":"守城与救人","growth_cost":"失去军职","pacing":"中速","ending":"开城救人"},{"id":"d2","hook":"潮汐神谕错位","genre":"历史传奇","protagonist":"潮汐祭司","core_conflict":"神谕与民生","growth_cost":"信仰崩解","pacing":"慢热","ending":"改写神谕"},{"id":"d3","hook":"假身份潜入盐帮","genre":"江湖复仇","protagonist":"失踪少主","core_conflict":"复仇与认亲","growth_cost":"放弃姓氏","pacing":"快节奏","ending":"揭穿盐帮"}]}`
	proposalPayload    = `{"title":"潮汐关","positioning":"奇幻抉择","core_conflict":"守城与救人","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["敌船靠岸","认出仇敌","开城救人"],"sell_point":"守关人必须放走仇敌","genre":"奇幻悬疑","growth_cost":"失去军职","pacing":"中速","ending":"开城救人","subplots":["旧令牌来历"],"forbidden":["现代武器"],"anchors":[{"strength":"hard","statement":"钟楼必须三响","scope_hint":"chapter"}]}`
	peopleWorldPayload = `{"characters":[{"name":"阿绫","role":"protagonist","aliases":["守关人"],"summary":"潮汐关最后的守将","motivation":"赎罪","fear":"再失一城","secret":"放过过仇敌","arc":"学会取舍"}],"relationships":[],"factions":[],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路开闭","scope_type":"project"}],"forbidden":[],"omission_reasons":["本方向不需要势力和地点"]}`
	roadmapPayload     = `{"book":{"title":"潮汐关","proposition":"守关人必须在亲情与职责间选择","ending":"开城救人","protagonist_arc":"从逃避到承担","inviolable":["潮汐决定航路"]},"volumes":[{"title":"第一卷","goal":"建立关隘危机","conflict":"敌船压境","payoff":"阿绫作出选择","position":1}],"arcs":[{"volume_ref":"第一卷","title":"开关","causal_chain":"敌船靠岸导致必须抉择","character_change":"阿绫接受代价","foreshadowing":["旧令牌"],"position":1}],"near_term_chapters":[{"title":"空船","goal":"发现异常","conflict":"无人登岸","outcome":"阿绫独自登船","anchors":["钟楼三响"],"position":1},{"title":"旧识","goal":"认出仇敌","conflict":"军令与私情","outcome":"暂时藏匿","anchors":[],"position":2},{"title":"开关","goal":"作出选择","conflict":"守城与救人","outcome":"开城","anchors":["潮汐律"],"position":3}]}`
)
