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
	agentkernel "github.com/lishen802/novel-studio/internal/application/agentkernel"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// waitFor 轮询等待条件满足，超时返回 false；不用固定 sleep 猜时序。
func waitFor(t *testing.T, check func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return check()
}

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
	if len(first.RoleModels) == 0 {
		t.Fatalf("preview must expose role model sources: %#v", first.RoleModels)
	}
	request.ConfirmationDigest = "stale-digest"
	request.IdempotencyKey = "invalid-preview"
	if _, err := service.ConfirmGenerate(context.Background(), request); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("stale digest error = %v", err)
	}
}

// P1-2/P1-3：选中的是具体方向卡；下游输入包含选中卡与全部上游，而非整套方向。
func TestConfirmGenerateUsesSelectedCardAndUpstreamSources(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedDirectionsSelection(t, service, store, "d2")

	// 方案生成的输入必须包含选中的 d2 卡，而不是整套三张卡。
	request := generateRequest(creationdomain.StageProposal)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "proposal-1"
	if _, err := service.ConfirmGenerate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		operation, err := store.OperationByIdempotency(context.Background(), session.ProjectID, session.BranchID, "generate:proposal", "proposal-1")
		return err == nil && operation.Status == creationdomain.OperationSucceeded
	}) {
		t.Fatal("proposal run did not finish")
	}
	sources := kernel.lastCreate.Sources
	foundCard, foundSet := false, false
	for _, source := range sources {
		if source.ID == "direction:d2" {
			foundCard = true
		}
		if source.ID == session.SelectedDirectionCandidateID {
			foundSet = true
		}
	}
	if !foundCard || foundSet {
		t.Fatalf("sources must carry selected card only: %#v", sources)
	}

	// 路线图输入必须同时收到方案与人物世界。
	seedStageCandidate(t, service, store, creationdomain.StagePeopleWorld, peopleWorldPayload)
	roadmapRequest := generateRequest(creationdomain.StageRoadmap)
	roadmapPreview, err := service.PreviewGenerate(context.Background(), roadmapRequest)
	if err != nil {
		t.Fatal(err)
	}
	roadmapRequest.ConfirmationDigest = roadmapPreview.ConfirmationDigest
	roadmapRequest.IdempotencyKey = "roadmap-1"
	if _, err := service.ConfirmGenerate(context.Background(), roadmapRequest); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		operation, err := store.OperationByIdempotency(context.Background(), session.ProjectID, session.BranchID, "generate:roadmap", "roadmap-1")
		return err == nil && operation.Status == creationdomain.OperationSucceeded
	}) {
		t.Fatal("roadmap run did not finish")
	}
	ids := map[string]bool{}
	for _, source := range kernel.lastCreate.Sources {
		ids[source.ID] = true
	}
	if !ids["direction:d2"] || !ids["proposal"] || !ids["people_world"] {
		t.Fatalf("roadmap sources missing upstream: %#v", kernel.lastCreate.Sources)
	}
}

// P1-2：选择不存在的卡必须被后端拒绝。
func TestSelectDirectionRejectsUnknownCard(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	session := seedIntentSession(t, service)
	now := testClock{}.Now()
	session, err := session.Advance(now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	candidate := mustCandidate(t, store, session, directionPayload)
	if _, err := service.SelectDirection(context.Background(), session.ProjectID, candidate.ID, "d9"); !errors.Is(err, creationdomain.ErrDirectionCard) {
		t.Fatalf("unknown card error = %v", err)
	}
	if _, err := service.SelectDirection(context.Background(), session.ProjectID, candidate.ID, "d3"); err != nil {
		t.Fatalf("valid card select: %v", err)
	}
}

// P1-4：重生成保留锁定字段与锁元数据。
func TestRegenerateKeepsLockedFieldsAndLocks(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedDirectionsSelection(t, service, store, "d1")
	seedStageCandidate(t, service, store, creationdomain.StageProposal, proposalPayload)

	locked, err := service.LockCandidateField(context.Background(), session.ProjectID, currentCandidateID(t, service, store, creationdomain.StageProposal), "/ending")
	if err != nil {
		t.Fatal(err)
	}
	if !locked.Locked("/ending") {
		t.Fatal("ending must be locked")
	}
	// 换掉内核产物中的结局，重生成后锁定的结局必须保持。
	kernel.payloads[agentassets.StoryProposalWorkflowID] = proposalAltEnding
	request := generateRequest(creationdomain.StageProposal)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "proposal-regen"
	if _, err := service.ConfirmGenerate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var next creationdomain.Candidate
	if !waitFor(t, func() bool {
		candidates, listErr := store.ListCandidates(context.Background(), session.ID)
		if listErr != nil {
			return false
		}
		current, ok := currentCandidate(candidates, creationdomain.StageProposal)
		if !ok {
			return false
		}
		next = current
		return next.Version == 2
	}) {
		t.Fatal("regenerated candidate not persisted")
	}
	if ending, err := next.FieldString("/ending"); err != nil || ending != "开城救人" {
		t.Fatalf("locked ending lost: %q err=%v", ending, err)
	}
	if !next.Locked("/ending") {
		t.Fatal("lock metadata lost after regenerate")
	}
}

// P1-5：最终产物指向修订稿而非初稿；质量 needs_review/blocked 暂停自动推进并阻断阶段推进。
func TestQualityGateBlocksAdvanceAndRevisionResultWins(t *testing.T) {
	kernel := newFakeKernel()
	kernel.revisedPayload = proposalRevisedPayload
	service, store, _ := newTestService(t, kernel, nil)
	seedDirectionsSelection(t, service, store, "d1")

	request := generateRequest(creationdomain.StageProposal)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "proposal-quality"
	if _, err := service.ConfirmGenerate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	var current creationdomain.Candidate
	if !waitFor(t, func() bool {
		session, sessErr := store.Session(context.Background(), "project-1")
		if sessErr != nil || session.ActiveRunID != "" {
			return false
		}
		candidates, _ := store.ListCandidates(context.Background(), session.ID)
		item, ok := currentCandidate(candidates, creationdomain.StageProposal)
		if !ok {
			return false
		}
		current = item
		return true
	}) {
		t.Fatal("proposal candidate not persisted")
	}
	// 修订稿内容必须胜出，不能取架构初稿。
	if title, err := current.FieldString("/title"); err != nil || title != "潮汐关·修订" {
		t.Fatalf("revision result not selected: title=%q err=%v", title, err)
	}

	// 质量阻断：AdvanceStage 不得推进。
	blocked, err := current.WithQuality(creationdomain.QualityBlocked), error(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCandidate(context.Background(), blocked); err != nil {
		t.Fatal(err)
	}
	session, err := store.Session(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	session.Stage = creationdomain.StageProposal
	if err := store.SaveSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AdvanceStage(context.Background(), "project-1"); !errors.Is(err, creationdomain.ErrQualityBlocked) {
		t.Fatalf("advance with blocked candidate error = %v", err)
	}
}

func TestManualAdvanceRequiresExplicitCandidateAcceptance(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	session := seedDirectionsSelection(t, service, store, "d1")
	candidate := seedStageCandidate(t, service, store, creationdomain.StageProposal, proposalPayload)
	candidates, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	candidate = candidate.WithUpstream(liveUpstreamDigest(session, candidates, creationdomain.StageProposal))
	if err := store.UpdateCandidate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}

	if _, err := service.AdvanceStage(context.Background(), session.ProjectID); !errors.Is(err, creationdomain.ErrCandidateAcceptanceRequired) {
		t.Fatalf("advance with unaccepted proposal error = %v", err)
	}
	accepted, err := service.AcceptCandidate(context.Background(), session.ProjectID, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != creationdomain.StatusConfirmed {
		t.Fatalf("accepted candidate status = %q", accepted.Status)
	}
	if _, err := service.AdvanceStage(context.Background(), session.ProjectID); err != nil {
		t.Fatalf("advance after acceptance: %v", err)
	}
}

// P1-7：运行期间作者保存的意图不被旧会话副本覆盖。
func TestRunDoesNotOverwriteConcurrentIntentSave(t *testing.T) {
	kernel := newFakeKernel()
	kernel.block = make(chan struct{})
	kernel.started = make(chan struct{})
	service, store, _ := newTestService(t, kernel, nil)
	seedDirectionsSelection(t, service, store, "d1")

	request := generateRequest(creationdomain.StageProposal)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "proposal-concurrent"
	if _, err := service.ConfirmGenerate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-kernel.started:
	case <-time.After(2 * time.Second):
		t.Fatal("execute did not start")
	}
	if _, err := service.SaveIntent(context.Background(), "project-1", "作者运行期间的新灵感"); err != nil {
		t.Fatal(err)
	}
	close(kernel.block)
	if !waitFor(t, func() bool {
		session, sessErr := store.Session(context.Background(), "project-1")
		return sessErr == nil && session.ActiveRunID == ""
	}) {
		t.Fatal("run did not finish")
	}
	session, err := store.Session(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if session.IntentText != "作者运行期间的新灵感" {
		t.Fatalf("concurrent intent save overwritten: %q", session.IntentText)
	}
}

// P1-9：服务重建后同键请求返回原结果，不重复执行付费调用。
func TestIdempotencySurvivesServiceRebuild(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	seedIntentSession(t, service)
	request := generateRequest(creationdomain.StageDirections)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "directions-rebuild"
	first, err := service.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		session, sessErr := store.Session(context.Background(), "project-1")
		return sessErr == nil && session.ActiveRunID == ""
	}) {
		t.Fatal("run did not finish")
	}
	// 原请求回执不因作者随后修改输入而失效；新输入必须使用新的预览与操作键。
	if _, err := service.SaveIntent(context.Background(), "project-1", "作者已经改为全新的创意"); err != nil {
		t.Fatal(err)
	}
	// 模拟重启：同一 store 上重建服务与内核。
	rebuiltKernel := newFakeKernel()
	rebuilt, err := New(Config{Store: store, Kernel: rebuiltKernel, Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	again, err := rebuilt.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if again.RunID != first.RunID || rebuiltKernel.createN != 0 || rebuiltKernel.executeN != 0 {
		t.Fatalf("rebuild replayed paid run: first=%#v again=%#v create=%d execute=%d",
			first, again, rebuiltKernel.createN, rebuiltKernel.executeN)
	}
	if again.Candidate.ID == "" {
		t.Fatal("replay must return persisted candidate")
	}
}

// 同键不同参数必须冲突，不能静默重跑。
func TestSameKeyDifferentArgumentsConflicts(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedDirectionsSelection(t, service, store, "d1")
	seedStageCandidate(t, service, store, creationdomain.StageProposal, proposalPayload)

	request := generateRequest(creationdomain.StagePeopleWorld)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "people-conflict"
	if _, err := service.ConfirmGenerate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		current, lookupErr := store.OperationByIdempotency(
			context.Background(), "project-1", "branch-1", "generate:people_world", "people-conflict",
		)
		return lookupErr == nil && current.Status == creationdomain.OperationSucceeded
	}) {
		t.Fatal("run did not finish")
	}
	// 编辑方案草稿：意图与选中卡未变（预览摘要不变），但冻结输入摘要变化，
	// 同键重试必须按参数冲突拒绝，而不是静默重跑。
	if _, err := service.SaveCandidateDraft(
		context.Background(), session.ProjectID, currentCandidateID(t, service, store, creationdomain.StageProposal), proposalAltEnding,
	); err != nil {
		t.Fatal(err)
	}
	second, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = second.ConfirmationDigest
	if _, err := service.ConfirmGenerate(context.Background(), request); !errors.Is(err, ErrOperationConflict) {
		t.Fatalf("same key different arguments error = %v", err)
	}
}

func TestPreviewBindsSelectedCardAndResolvedModels(t *testing.T) {
	ctx := context.Background()
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	seedDirectionsSelection(t, service, store, "d1")
	request := generateRequest(creationdomain.StageProposal)
	first, err := service.PreviewGenerate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.LookBackStage(ctx, "project-1", creationdomain.StageDirections); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SelectDirection(ctx, "project-1", "dir-set-1", "d2"); err != nil {
		t.Fatal(err)
	}
	second, err := service.PreviewGenerate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConfirmationDigest == second.ConfirmationDigest {
		t.Fatal("card choice missing from authorization")
	}
	request.Profile.Roles[0].Model = "different-paid-model"
	third, err := service.PreviewGenerate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if second.ConfirmationDigest == third.ConfirmationDigest {
		t.Fatal("resolved model missing from authorization")
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
	request.IdempotencyKey = "directions-cancel"
	if _, err := service.ConfirmGenerate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-kernel.started:
	case <-time.After(2 * time.Second):
		t.Fatal("execute did not start")
	}
	if err := service.CancelRun(context.Background(), request.ProjectID); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		current, lookupErr := store.OperationByIdempotency(
			context.Background(), "project-1", "branch-1", "generate:directions", "directions-cancel",
		)
		return lookupErr == nil && current.Status == creationdomain.OperationFailed
	}) {
		t.Fatal("operation not marked failed after cancel")
	}
	listed, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil || len(listed) != 0 {
		t.Fatalf("candidates after cancel = %#v, err=%v", listed, err)
	}
}

// 恢复不自动续费：进程内无活跃执行且运行不可核实时暂停并保留运行引用。
func TestRecoverDoesNotResumePaidRun(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedIntentSession(t, service)
	session.ActiveRunID = "run-stale"
	if err := store.SaveRunSlot(context.Background(), session); err != nil {
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
	// 暂停期间确认生成必须被运行槽拒绝，直到作者显式取消旧运行。
	request := generateRequest(creationdomain.StageDirections)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "directions-after-recover"
	if _, err := service.ConfirmGenerate(context.Background(), request); !errors.Is(err, ErrRunActive) {
		t.Fatalf("confirm while interrupted error = %v", err)
	}
	if err := service.CancelRun(context.Background(), session.ProjectID); err != nil {
		t.Fatal(err)
	}
	after, err := store.Session(context.Background(), session.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if after.ActiveRunID != "" {
		t.Fatalf("cancel must clear stale slot: %#v", after)
	}
}

// P2-14：恢复对账——已完成运行的候选只物化一次，不重复调用模型。
func TestRecoverMaterializesCompletedRunOnce(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedIntentSession(t, service)
	request := generateRequest(creationdomain.StageDirections)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "directions-recover"
	result, err := service.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		current, lookupErr := store.OperationByIdempotency(
			context.Background(), "project-1", "branch-1", "generate:directions", "directions-recover",
		)
		return lookupErr == nil && current.Status == creationdomain.OperationSucceeded
	}) {
		t.Fatal("operation did not succeed")
	}
	// 模拟崩溃窗口：运行完成、候选落库后，会话槽位被外部置回运行 ID。
	completed, err := store.Session(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	operation, err := store.OperationByIdempotency(
		context.Background(), "project-1", "branch-1", "generate:directions", "directions-recover",
	)
	if err != nil {
		t.Fatal(err)
	}
	completed, err = completed.AttachRun(operation.ID, result.RunID, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRunSlot(context.Background(), completed); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.Recover(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if recovered.ActiveRunID != "" {
		t.Fatalf("recovered slot not released: %#v", recovered)
	}
	candidates, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	sets := 0
	for _, candidate := range candidates {
		if candidate.Kind == creationdomain.KindDirectionSet {
			sets++
		}
	}
	if sets != 1 || kernel.executeN != 1 {
		t.Fatalf("recover duplicated result: sets=%d execute=%d", sets, kernel.executeN)
	}
}

// 恢复物化必须沿用运行启动时冻结的依赖摘要，作者后续修改不能让旧结果伪装成最新结果。
func TestRecoverUsesPersistedFrozenInputDigest(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedIntentSession(t, service)
	request := generateRequest(creationdomain.StageDirections)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "directions-frozen-recover"
	result, err := service.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		current, lookupErr := store.OperationByIdempotency(
			context.Background(), "project-1", "branch-1", "generate:directions", request.IdempotencyKey,
		)
		return lookupErr == nil && current.Status == creationdomain.OperationSucceeded
	}) {
		t.Fatal("operation did not succeed")
	}
	operation, err := store.OperationByIdempotency(
		context.Background(), "project-1", "branch-1", "generate:directions", request.IdempotencyKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	originalDigest := liveUpstreamDigest(session, nil, creationdomain.StageDirections)
	if operation.FrozenInputDigest != originalDigest {
		t.Fatalf("persisted frozen digest = %q, want %q", operation.FrozenInputDigest, originalDigest)
	}

	// 模拟候选尚未落库便崩溃，并在恢复前修改作者意图。
	store.mu.Lock()
	for id, candidate := range store.candidates {
		if candidate.RunID == result.RunID {
			delete(store.candidates, id)
		}
	}
	store.mu.Unlock()
	changed, err := service.SaveIntent(context.Background(), session.ProjectID, "恢复前已经修改的创作意图")
	if err != nil {
		t.Fatal(err)
	}
	changed, err = changed.AttachRun(operation.ID, result.RunID, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRunSlot(context.Background(), changed); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Recover(context.Background(), session.ProjectID); err != nil {
		t.Fatal(err)
	}
	candidates, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("recovered candidates = %#v", candidates)
	}
	recovered := candidates[0]
	if recovered.UpstreamDigest != originalDigest {
		t.Fatalf("recovered upstream digest = %q, want frozen %q", recovered.UpstreamDigest, originalDigest)
	}
	if recovered.Status != creationdomain.StatusStale {
		t.Fatalf("recovered candidate status = %q, want stale", recovered.Status)
	}
}

func TestRecoverRejectsMissingFrozenInputEvidence(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedIntentSession(t, service)
	request := generateRequest(creationdomain.StageDirections)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "directions-missing-frozen"
	result, err := service.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		current, lookupErr := store.OperationByIdempotency(
			context.Background(), "project-1", "branch-1", "generate:directions", request.IdempotencyKey,
		)
		return lookupErr == nil && current.Status == creationdomain.OperationSucceeded
	}) {
		t.Fatal("operation did not succeed")
	}
	operation, err := store.OperationByIdempotency(
		context.Background(), "project-1", "branch-1", "generate:directions", request.IdempotencyKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	operation.FrozenInputDigest = ""
	store.operations[operationKey(operation.ProjectID, operation.BranchID, operation.Command, operation.IdempotencyKey)] = operation
	for id, candidate := range store.candidates {
		if candidate.RunID == result.RunID {
			delete(store.candidates, id)
		}
	}
	store.mu.Unlock()
	current, err := store.Session(context.Background(), session.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	current, err = current.AttachRun(operation.ID, result.RunID, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRunSlot(context.Background(), current); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Recover(context.Background(), session.ProjectID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("recover error = %v, want unavailable", err)
	}
	candidates, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 0 {
		t.Fatalf("recovery fabricated candidate without frozen evidence: %#v", candidates)
	}
}

func TestAutoAdvanceStopsBeforeArchiveConfirm(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedDirectionsSelection(t, service, store, "d1")
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
	request.IdempotencyKey = "auto-1"
	if _, err := service.ConfirmGenerate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		current, sessErr := store.Session(context.Background(), "project-1")
		return sessErr == nil && current.Stage == creationdomain.StageRoadmap &&
			current.BlockingReason == creationdomain.BlockFormalWrite
	}) {
		session, _ := store.Session(context.Background(), "project-1")
		t.Fatalf("auto advance did not pause before archive: %#v", session)
	}
	listed, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[creationdomain.CandidateKind]int{}
	var roadmap creationdomain.Candidate
	for _, candidate := range listed {
		if candidate.Status == creationdomain.StatusSuperseded {
			continue
		}
		kinds[candidate.Kind]++
		if candidate.Kind == creationdomain.KindRoadmap {
			roadmap = candidate
		}
	}
	if kinds[creationdomain.KindProposal] != 1 || kinds[creationdomain.KindPeopleWorld] != 1 || kinds[creationdomain.KindRoadmap] != 1 {
		t.Fatalf("candidates = %#v", listed)
	}
	if kernel.executeN != 3 {
		t.Fatalf("executeN = %d, want 3", kernel.executeN)
	}
	operation, err := store.OperationByIdempotency(
		context.Background(), "project-1", "branch-1", "generate:proposal", request.IdempotencyKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if operation.RunID != roadmap.RunID || operation.FrozenInputDigest != roadmap.UpstreamDigest {
		t.Fatalf("auto operation did not follow final child run: operation=%+v roadmap=%+v", operation, roadmap)
	}
}

// 自动推进在质量 needs_review 时暂停，不继续付费调用。
func TestAutoAdvancePausesOnQualityReview(t *testing.T) {
	kernel := newFakeKernel()
	kernel.quality = agentdomain.QualityNeedsReview
	service, store, _ := newTestService(t, kernel, nil)
	session := seedDirectionsSelection(t, service, store, "d1")
	if _, err := service.SetAdvanceMode(context.Background(), session.ProjectID, creationdomain.ModeAutoAdvance); err != nil {
		t.Fatal(err)
	}
	request := generateRequest(creationdomain.StageProposal)
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "auto-quality"
	if _, err := service.ConfirmGenerate(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		current, sessErr := store.Session(context.Background(), "project-1")
		return sessErr == nil && current.BlockingReason == creationdomain.BlockQualityReview
	}) {
		t.Fatal("auto advance did not pause on quality review")
	}
	if kernel.executeN != 1 {
		t.Fatalf("executeN = %d, want 1", kernel.executeN)
	}
}

func TestAutoAdvanceDoesNotResetAuthorizedBudget(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session := seedDirectionsSelection(t, service, store, "d1")
	if _, err := service.SetAdvanceMode(context.Background(), session.ProjectID, creationdomain.ModeAutoAdvance); err != nil {
		t.Fatal(err)
	}
	request := generateRequest(creationdomain.StageProposal)
	kernel.inputTokens = request.Profile.MaxRunTokens - 1
	preview, err := service.PreviewGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "auto-total-budget"
	result, err := service.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool { return !service.coordinator.Live(result.RunID) }) {
		t.Fatal("run did not finish")
	}
	latest, err := store.Session(context.Background(), session.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.BlockingReason != creationdomain.BlockCostAuthorization || kernel.executeN != 1 {
		t.Fatalf("auto chain reset budget: session=%+v calls=%d", latest, kernel.executeN)
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
	request.IdempotencyKey = "directions-stale"
	result, err := service.ConfirmGenerate(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool { return !service.coordinator.Live(result.RunID) }) {
		t.Fatal("stale execution did not release lease")
	}
	// 代次已经变化时连失败回执也不能写旧库，留待重新打开后的恢复对账。
	operation, err := store.OperationByIdempotency(context.Background(), "project-1", "branch-1", "generate:directions", "directions-stale")
	if err != nil || operation.Status != creationdomain.OperationRunning {
		t.Fatalf("stale execution wrote receipt: %+v, %v", operation, err)
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

// seedDirectionsSelection 建立到方向阶段并选中指定卡，供下游阶段测试使用。
func seedDirectionsSelection(t *testing.T, service *Service, store *memoryStore, cardID string) creationdomain.Session {
	t.Helper()
	session := seedIntentSession(t, service)
	now := testClock{}.Now()
	session, err := session.Advance(now)
	if err != nil {
		t.Fatal(err)
	}
	candidate := mustCandidate(t, store, session, directionPayload)
	session, err = session.SelectDirection(candidate.ID, cardID, now)
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

// seedStageCandidate 直接落一份某阶段的当前候选，模拟既有上游。
func seedStageCandidate(
	t *testing.T, service *Service, store *memoryStore, stage creationdomain.Stage, payload string,
) creationdomain.Candidate {
	t.Helper()
	session, err := store.Session(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	now := testClock{}.Now()
	kind := creationdomain.KindProposal
	if stage == creationdomain.StagePeopleWorld {
		kind = creationdomain.KindPeopleWorld
	} else if stage == creationdomain.StageRoadmap {
		kind = creationdomain.KindRoadmap
	}
	candidate, err := creationdomain.NewCandidate(
		"seed-"+string(stage), session.ID, stage, kind, 1, "", "run-"+string(stage), "art-"+string(stage),
		payload, "schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCandidate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}
	if stage == creationdomain.StageProposal || stage == creationdomain.StagePeopleWorld {
		session.Stage = stage
		if err := store.SaveSession(context.Background(), session); err != nil {
			t.Fatal(err)
		}
	}
	return candidate
}

func mustCandidate(t *testing.T, store *memoryStore, session creationdomain.Session, payload string) creationdomain.Candidate {
	t.Helper()
	candidate, err := creationdomain.NewCandidate(
		"dir-set-1", session.ID, creationdomain.StageDirections, creationdomain.KindDirectionSet,
		1, "", "run-dir", "art-dir", payload, "story_directions.schema", 1, testClock{}.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCandidate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}
	return candidate
}

func currentCandidateID(t *testing.T, service *Service, store *memoryStore, stage creationdomain.Stage) string {
	t.Helper()
	session, err := store.Session(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := store.ListCandidates(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := currentCandidate(candidates, stage)
	if !ok {
		t.Fatalf("no current candidate for %s", stage)
	}
	return current.ID
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
			Roles: []agentdomain.ResolvedRole{{Role: agentdomain.RolePlanner, ProviderName: "openai", Model: "gpt-test"}},
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
	mu             sync.Mutex
	createN        int
	executeN       int
	inputTokens    int
	failExecute    bool
	lastCreate     agentkernel.CreateRequest
	payloads       map[string]string
	revisedPayload string
	quality        agentdomain.QualityStatus
	runs           map[string]agentdomain.Run
	artifacts      map[string][]agentdomain.Artifact
	block          chan struct{}
	started        chan struct{}
	onExecute      func()
}

func newFakeKernel() *fakeKernel {
	return &fakeKernel{
		payloads: map[string]string{
			agentassets.StoryDirectionsWorkflowID:   directionPayload,
			agentassets.StoryProposalWorkflowID:     proposalPayload,
			agentassets.StoryPeopleWorldWorkflowID:  peopleWorldPayload,
			agentassets.StoryRoadmapWorkflowID:      roadmapPayload,
			agentassets.StoryAuthorReviseWorkflowID: proposalPayload,
		},
		quality:   agentdomain.QualityPassed,
		runs:      map[string]agentdomain.Run{},
		artifacts: map[string][]agentdomain.Artifact{},
	}
}

// fakeSteps 按工作流形态构造完成步骤与产物：方案工作流含修订节点，
// 作者定向修订是单节点 revise，其余为生成+审阅。
func (kernel *fakeKernel) fakeSteps(request agentkernel.CreateRequest, payload string) ([]agentdomain.RunStep, []agentdomain.Artifact) {
	steps := make([]agentdomain.RunStep, 0, 3)
	artifacts := make([]agentdomain.Artifact, 0, 3)
	appendStep := func(nodeID string, capability agentdomain.Capability, content string) {
		artifactID := request.RunID + "-" + nodeID
		steps = append(steps, agentdomain.RunStep{
			ID: request.RunID + "-" + nodeID, RunID: request.RunID, NodeID: nodeID,
			Capability: capability, Status: agentdomain.StepCompleted, PrimaryArtifactID: artifactID,
		})
		artifacts = append(artifacts, agentdomain.Artifact{ID: artifactID, RunID: request.RunID, Content: content})
	}
	if request.WorkflowID == agentassets.StoryAuthorReviseWorkflowID {
		appendStep("revise", agentdomain.CapabilityBoundedRevision, payload)
		return steps, artifacts
	}
	appendStep("blueprint", agentdomain.CapabilityStoryArchitect, payload)
	appendStep("review", agentdomain.CapabilityContinuityReview, `{"quality_status":"passed","summary":"通过","findings":[]}`)
	if request.WorkflowID == agentassets.StoryProposalWorkflowID {
		revised := kernel.revisedPayload
		if revised == "" {
			revised = payload
		}
		appendStep("revision", agentdomain.CapabilityBoundedRevision, revised)
	}
	return steps, artifacts
}

func (kernel *fakeKernel) CreateRun(_ context.Context, request agentkernel.CreateRequest) (agentdomain.Run, error) {
	kernel.mu.Lock()
	defer kernel.mu.Unlock()
	kernel.createN++
	kernel.lastCreate = request
	payload, ok := kernel.payloads[request.WorkflowID]
	if !ok {
		return agentdomain.Run{}, fmt.Errorf("missing payload for %s", request.WorkflowID)
	}
	run := agentdomain.Run{
		ID: request.RunID, ProjectID: request.ProjectID, Status: agentdomain.RunCompleted,
		WorkflowBinding: agentdomain.VersionBinding{ID: request.WorkflowID, Version: request.WorkflowVersion},
		QualityStatus:   kernel.quality,
		InputTokens:     kernel.inputTokens,
	}
	run.Steps, _ = kernel.fakeSteps(request, payload)
	kernel.runs[run.ID] = run
	_, artifacts := kernel.fakeSteps(request, payload)
	kernel.artifacts[run.ID] = artifacts
	return run, nil
}

func (kernel *fakeKernel) ExecuteRun(ctx context.Context, runID, _ string) (agentdomain.Run, error) {
	kernel.mu.Lock()
	kernel.executeN++
	if kernel.failExecute {
		kernel.mu.Unlock()
		return agentdomain.Run{}, fmt.Errorf("execute failed")
	}
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

// Run 读取持久化运行；不存在的运行返回错误，模拟恢复对账路径。
func (kernel *fakeKernel) Run(_ context.Context, runID string) (agentdomain.Run, error) {
	kernel.mu.Lock()
	defer kernel.mu.Unlock()
	run, ok := kernel.runs[runID]
	if !ok {
		return agentdomain.Run{}, ErrSessionNotFound
	}
	return run, nil
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
	directionPayload       = `{"directions":[{"id":"d1","hook":"守关人放走仇敌","genre":"奇幻悬疑","protagonist":"守关人阿绫","core_conflict":"守城与救人","growth_cost":"失去军职","pacing":"中速","ending":"开城救人"},{"id":"d2","hook":"潮汐神谕错位","genre":"历史传奇","protagonist":"潮汐祭司","core_conflict":"神谕与民生","growth_cost":"信仰崩解","pacing":"慢热","ending":"改写神谕"},{"id":"d3","hook":"假身份潜入盐帮","genre":"江湖复仇","protagonist":"失踪少主","core_conflict":"复仇与认亲","growth_cost":"放弃姓氏","pacing":"快节奏","ending":"揭穿盐帮"}]}`
	proposalPayload        = `{"title":"潮汐关","positioning":"奇幻抉择","core_conflict":"守城与救人","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["敌船靠岸","认出仇敌","开城救人"],"sell_point":"守关人必须放走仇敌","genre":"奇幻悬疑","growth_cost":"失去军职","pacing":"中速","ending":"开城救人","subplots":["旧令牌来历"],"forbidden":["现代武器"],"anchors":[{"strength":"hard","statement":"钟楼必须三响","scope_hint":"proposal"}]}`
	proposalAltEnding      = `{"title":"闭城之冬","positioning":"奇幻抉择","core_conflict":"守城与救人","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["敌船靠岸","认出仇敌","闭城拒敌"],"sell_point":"守关人必须放走仇敌","genre":"奇幻悬疑","growth_cost":"失去军职","pacing":"中速","ending":"闭城拒敌","subplots":["旧令牌来历"],"forbidden":["现代武器"],"anchors":[]}`
	proposalRevisedPayload = `{"title":"潮汐关·修订","positioning":"奇幻抉择","core_conflict":"守城与救人","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["敌船靠岸","认出仇敌","开城救人"],"sell_point":"守关人必须放走仇敌","genre":"奇幻悬疑","growth_cost":"失去军职","pacing":"中速","ending":"开城救人","subplots":["旧令牌来历"],"forbidden":["现代武器"],"anchors":[]}`
	peopleWorldPayload     = `{"characters":[{"id":"char-1","name":"阿绫","role":"protagonist","aliases":["守关人"],"summary":"潮汐关最后的守将","motivation":"赎罪","fear":"再失一城","secret":"放过过仇敌","arc":"学会取舍"}],"relationships":[],"factions":[],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路开闭","scope_type":"project"}],"forbidden":[],"omission_reasons":["本方向不需要势力和地点"]}`
	roadmapPayload         = `{"book":{"id":"book-1","title":"潮汐关","proposition":"守关人必须在亲情与职责间选择","ending":"开城救人","protagonist_arc":"从逃避到承担","inviolable":["潮汐决定航路"]},"volumes":[{"id":"vol-1","title":"第一卷","goal":"建立关隘危机","conflict":"敌船压境","payoff":"阿绫作出选择","position":1}],"arcs":[{"id":"arc-1","volume_ref":"vol-1","title":"开关","causal_chain":"敌船靠岸导致必须抉择","character_change":"阿绫接受代价","foreshadowing":["旧令牌"],"position":1}],"near_term_chapters":[{"id":"ch-1","arc_ref":"arc-1","title":"空船","goal":"发现异常","conflict":"无人登岸","outcome":"阿绫独自登船","anchors":[{"strength":"hard","statement":"钟楼三响"}],"position":1},{"id":"ch-2","arc_ref":"arc-1","title":"旧识","goal":"认出仇敌","conflict":"军令与私情","outcome":"暂时藏匿","anchors":[],"position":2},{"id":"ch-3","arc_ref":"arc-1","title":"开关","goal":"作出选择","conflict":"守城与救人","outcome":"开城","anchors":[{"strength":"soft","statement":"潮汐律"}],"position":3}]}`
)
