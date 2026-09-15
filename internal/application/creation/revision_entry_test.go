package creation

import (
	"context"
	"encoding/json"
	"github.com/lishen802/novel-studio/internal/agentassets"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"testing"
	"time"
)

func TestRevisionProposalFreezesWithoutCallingModel(t *testing.T) {
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	req := proposeRevise(base.ID)
	req.Profile.Roles = append(req.Profile.Roles, agentdomain.ResolvedRole{Role: agentdomain.RoleContinuityReviewer, ProviderName: "test", Model: "review-model"})
	view, err := service.PreviewRevision(context.Background(), "project-1", RevisionRequest{CandidateID: base.ID, Paths: []string{"/title"}, Instruction: "标题更简洁", IdempotencyKey: "revision-preview"}, req.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if view.ActionID == "" || view.State != "pending_confirmation" || len(view.Targets) != 1 || kernel.createN != 0 {
		t.Fatalf("view=%+v calls=%d", view, kernel.createN)
	}
}

func TestRevisionConversationPersistsBoundMessagesAndRecoversIdempotently(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	profile := generateRequest(base.Stage).Profile
	profile.Roles = append(profile.Roles, agentdomain.ResolvedRole{Role: agentdomain.RoleContinuityReviewer, ProviderName: "test", Model: "review-model"})
	request := SendMessageRequest{CandidateID: base.ID, EntityRef: "proposal:positioning", FieldPath: "/title", Text: "保留定位，让标题更含蓄", IdempotencyKey: "chat-1"}
	first, err := service.RecordRevisionConversation(ctx, "project-1", request, profile)
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.RecordRevisionConversation(ctx, "project-1", request, profile)
	if err != nil {
		t.Fatal(err)
	}
	if first.ActionID != second.ActionID || kernel.createN != 0 {
		t.Fatalf("action ids %q %q, model calls %d", first.ActionID, second.ActionID, kernel.createN)
	}
	projection, err := service.AssistantProjection(ctx, "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Messages) != 2 || projection.Messages[0].CandidateID != base.ID || projection.Messages[0].FieldPath != "/title" || projection.Messages[0].ActionID != first.ActionID {
		t.Fatalf("messages = %#v", projection.Messages)
	}
	workbench, err := service.Workbench(ctx, "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(workbench.Assistant.Messages) != 2 || workbench.Assistant.Messages[1].ActionState != "pending_confirmation" {
		t.Fatalf("assistant view = %#v", workbench.Assistant)
	}
}

func TestRevisionConfirmIsAsyncAndAdoptionIsExplicit(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	kernel.block = make(chan struct{})
	kernel.started = make(chan struct{}, 1)
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	output := revisionEnvelope(base)
	payload, err := base.EditTextFields(map[string]string{"/title": "新标题"})
	if err != nil {
		t.Fatal(err)
	}
	output.Payload = []byte(payload)
	encoded, _ := json.Marshal(output)
	kernel.payloads[agentassets.StoryCandidateRevisionWorkflowID] = string(encoded)
	profile := generateRequest(base.Stage).Profile
	profile.Roles = append(profile.Roles, agentdomain.ResolvedRole{Role: agentdomain.RoleContinuityReviewer, ProviderName: "test", Model: "review-model"})
	view, err := service.PreviewRevision(ctx, "project-1", RevisionRequest{CandidateID: base.ID, Paths: []string{"/title"}, Instruction: "简化标题", IdempotencyKey: "preview-1"}, profile)
	if err != nil {
		t.Fatal(err)
	}
	command := RevisionCommand{ActionID: view.ActionID, ArgumentsHash: view.ArgumentsHash, IdempotencyKey: view.IdempotencyKey}
	running, err := service.ConfirmRevision(ctx, "project-1", command, profile)
	if err != nil {
		t.Fatal(err)
	}
	if running.State != "executing" {
		t.Fatalf("未异步返回: %+v", running)
	}
	select {
	case <-kernel.started:
	case <-time.After(time.Second):
		t.Fatal("未启动")
	}
	if _, err := service.ConfirmRevision(ctx, "project-1", command, profile); err != nil {
		t.Fatal(err)
	}
	current, _ := store.Candidate(ctx, base.ID)
	if current.Status != base.Status {
		t.Fatal("尚未采用就替换了基线")
	}
	close(kernel.block)
	var ready RevisionView
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		views, e := service.RevisionActions(ctx, "project-1", true)
		if e != nil {
			t.Fatal(e)
		}
		if len(views) > 0 && views[0].State == "succeeded" {
			ready = views[0]
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !ready.CanApply || len(ready.Diffs) != 1 {
		t.Fatalf("结果不可审阅: %+v", ready)
	}
	// 模拟已落盘结果尚未写回 Action 的重启窗口；恢复只核对结果，不重放模型。
	for service.coordinator.Live(ready.RunID) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	action, err := store.Action(ctx, ready.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	action.State = assistantdomain.StateExecuting
	if err = store.SaveAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.RevisionActions(ctx, "project-1", true)
	if err != nil || len(recovered) != 1 || !recovered[0].CanApply {
		t.Fatalf("恢复失败: %+v %v", recovered, err)
	}
	candidate, err := service.ApplyRevision(ctx, "project-1", command)
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Version != 2 || candidate.Status == creationdomain.StatusConfirmed {
		t.Fatal("采用应创建待确认的新版本")
	}
	replay, err := service.ApplyRevision(ctx, "project-1", command)
	if err != nil || replay.ID != candidate.ID {
		t.Fatal("重复采用创建了额外版本")
	}
	kernel.mu.Lock()
	defer kernel.mu.Unlock()
	if kernel.createN != 1 || kernel.executeN != 1 || kernel.lastCreate.ChapterID != "" || kernel.lastCreate.ChapterBaseRevision != 0 {
		t.Fatal("重复调用或使用了占位章节")
	}
}

func TestRevisionChangedSessionRejectsBeforePayment(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	profile := generateRequest(base.Stage).Profile
	profile.Roles = append(profile.Roles, agentdomain.ResolvedRole{Role: agentdomain.RoleContinuityReviewer, ProviderName: "test", Model: "review"})
	view, err := service.PreviewRevision(ctx, "project-1", RevisionRequest{CandidateID: base.ID, Paths: []string{"/title"}, Instruction: "简化标题", IdempotencyKey: "stale"}, profile)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.loadSession(ctx, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.SaveSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	_, err = service.ConfirmRevision(ctx, "project-1", RevisionCommand{ActionID: view.ActionID, ArgumentsHash: view.ArgumentsHash, IdempotencyKey: view.IdempotencyKey}, profile)
	if err == nil || kernel.createN != 0 {
		t.Fatal("变化的基线不应启动付费调用")
	}
}
