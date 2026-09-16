package creation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

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

// 部分采纳只合入作者选中的差异，其余保持基线原文，并标记待复核。
func TestApplyRevisionAdoptsSelectedSubsetOnly(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	revised := strings.Replace(proposalPayload, `"title":"潮汐关"`, `"title":"潮汐关·修订"`, 1)
	revised = strings.Replace(revised, `"ending":"开城救人"`, `"ending":"闭城拒敌"`, 1)
	service, store, _ := newTestService(t, kernel, nil)
	base := seedProposalCandidate(t, service, store)
	// 通用修订工作流返回带信封的结果，payload_schema_ref 必须与候选 Schema 一致。
	kernel.payloads[agentassets.StoryCandidateRevisionWorkflowID] = `{"candidate_kind":"proposal","payload_schema_ref":"story_blueprint.schema@2","payload":` + revised + `}`
	profile := generateRequest(base.Stage).Profile
	profile.Roles = append(profile.Roles, agentdomain.ResolvedRole{Role: agentdomain.RoleContinuityReviewer, ProviderName: "test", Model: "review-model"})
	preview, err := service.PreviewRevision(ctx, "project-1", RevisionRequest{
		CandidateID: base.ID, Paths: []string{"/title", "/ending"}, Instruction: "标题更冷，结局更决绝", IdempotencyKey: "partial-preview",
	}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmRevision(ctx, "project-1", RevisionCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: "partial-preview",
	}, profile); err != nil {
		t.Fatal(err)
	}
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
	if len(ready.Diffs) != 2 {
		views, _ := service.RevisionActions(ctx, "project-1", true)
		t.Fatalf("修订结果不完整: state=%v summary=%v", views[0].State, views[0].Summary)
	}
	paths := map[string]bool{}
	for _, diff := range ready.Diffs {
		paths[diff.Path] = true
	}
	if !paths["/title"] || !paths["/ending"] {
		t.Fatalf("diff paths = %#v", ready.Diffs)
	}

	// 未经授权的路径即使属于候选字段也不能被部分采纳。
	if _, err := service.ApplyRevision(ctx, "project-1", RevisionCommand{
		ActionID: ready.ActionID, ArgumentsHash: ready.ArgumentsHash, IdempotencyKey: "partial-preview",
		Paths: []string{"/positioning"},
	}); !errors.Is(err, creationdomain.ErrRevisionScope) {
		t.Fatalf("未授权路径错误 = %v", err)
	}
	if current, e := store.Candidate(ctx, base.ID); e != nil || current.Status == creationdomain.StatusSuperseded {
		t.Fatalf("拒绝后基线不应被替代: %#v %v", current.Status, e)
	}
	command := RevisionCommand{
		ActionID: ready.ActionID, ArgumentsHash: ready.ArgumentsHash, IdempotencyKey: "partial-preview",
		Paths: []string{"/title"},
	}
	adopted, err := service.ApplyRevision(ctx, "project-1", command)
	if err != nil {
		t.Fatal(err)
	}
	if adopted.Origin != creationdomain.OriginAuthor || adopted.QualityStatus != creationdomain.QualityNeedsReview {
		t.Fatalf("部分采纳来源/质量 = %q/%q", adopted.Origin, adopted.QualityStatus)
	}
	title, err := adopted.FieldString("/title")
	if err != nil || title != "潮汐关·修订" {
		t.Fatalf("选中路径 = %q, %v", title, err)
	}
	ending, err := adopted.FieldString("/ending")
	if err != nil || ending != "开城救人" {
		t.Fatalf("未选中路径必须保持基线原文: %q, %v", ending, err)
	}
	replay, err := service.ApplyRevision(ctx, "project-1", command)
	if err != nil || replay.ID != adopted.ID {
		t.Fatalf("重复部分采纳创建了额外版本: %#v %v", replay, err)
	}
	// 基线已被采用替代后，同一动作不能再用另一个子集重复采用。
	if _, err := service.ApplyRevision(ctx, "project-1", RevisionCommand{
		ActionID: ready.ActionID, ArgumentsHash: ready.ArgumentsHash, IdempotencyKey: "partial-preview",
		Paths: []string{"/ending"},
	}); err == nil {
		t.Fatal("基线变化后不应再采用其他子集")
	}
}

// 授权后 AI 定向修订可以增删实体：差异标注结构变化，整组采用落到新候选；
// 未授权时同一结果被范围门禁拒绝；结构差异不能部分采用。
func TestApplyRevisionAcceptsAuthorizedStructuralChange(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	base := seedPeopleWorldWithUpstream(t, store)
	deputy := `{"id":"char-2","name":"老周","role":"supporting","aliases":[],"summary":"跟随阿绫多年的副将","motivation":"护住守军","fear":"重演旧败","secret":"隐瞒军令","arc":"接受军令之外的判断"}`
	addedPayload, err := base.ApplyStructure(creationdomain.StructureEdit{
		Collection: "characters", Operation: creationdomain.StructureAdd, EntityKey: "char-2", Value: deputy,
	})
	if err != nil {
		t.Fatal(err)
	}
	revised := strings.Replace(addedPayload, `"summary":"潮汐关最后的守将"`, `"summary":"潮汐关最后的守将，心里有债"`, 1)
	envelope := `{"candidate_kind":"people_world","payload_schema_ref":"story_cast.schema@2","payload":` + revised + `}`

	profile := generateRequest(base.Stage).Profile
	profile.Roles = append(profile.Roles, agentdomain.ResolvedRole{Role: agentdomain.RoleContinuityReviewer, ProviderName: "test", Model: "review-model"})
	// 未授权时同一结果必须被拒绝。
	kernel.payloads[agentassets.StoryCandidateRevisionWorkflowID] = envelope
	unauthorized, err := service.PreviewRevision(ctx, "project-1", RevisionRequest{
		CandidateID: base.ID, Paths: []string{"/characters/@char-1/summary"}, Instruction: "补一个副将", IdempotencyKey: "structure-unauthorized",
	}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmRevision(ctx, "project-1", RevisionCommand{
		ActionID: unauthorized.ActionID, ArgumentsHash: unauthorized.ArgumentsHash, IdempotencyKey: "structure-unauthorized",
	}, profile); err != nil {
		t.Fatal(err)
	}
	failed := waitRevisionState(t, service, unauthorized.ActionID)
	if failed.State != "failed" {
		t.Fatalf("未授权的结构增删必须失败: %#v", failed)
	}

	// 显式授权后同一结果被接受，并生成结构化差异。
	preview, err := service.PreviewRevision(ctx, "project-1", RevisionRequest{
		CandidateID: base.ID, Paths: []string{"/characters/@char-1/summary"}, AllowAdd: []string{"characters"},
		Instruction: "补一个副将并微调人物概要", IdempotencyKey: "structure-authorized",
	}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmRevision(ctx, "project-1", RevisionCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: "structure-authorized",
	}, profile); err != nil {
		t.Fatal(err)
	}
	ready := waitRevisionState(t, service, preview.ActionID)
	if ready.State != "succeeded" || !ready.CanApply {
		t.Fatalf("授权后修订应可审阅: %#v", ready)
	}
	kinds := map[string]int{}
	for _, diff := range ready.Diffs {
		kinds[diff.Kind]++
	}
	if kinds["add_entity"] != 1 || kinds["text"] != 1 {
		t.Fatalf("差异种类 = %#v", ready.Diffs)
	}

	// 结构差异必须整组采用。
	if _, err := service.ApplyRevision(ctx, "project-1", RevisionCommand{
		ActionID: ready.ActionID, ArgumentsHash: ready.ArgumentsHash, IdempotencyKey: "structure-authorized",
		Paths: []string{"/characters/@char-1/summary"},
	}); !errors.Is(err, creationdomain.ErrRevisionScope) {
		t.Fatalf("结构差异不应支持部分采用: %v", err)
	}

	adopted, err := service.ApplyRevision(ctx, "project-1", RevisionCommand{
		ActionID: ready.ActionID, ArgumentsHash: ready.ArgumentsHash, IdempotencyKey: "structure-authorized",
	})
	if err != nil {
		t.Fatal(err)
	}
	if added, err := adopted.FieldString("/characters/@char-2/name"); err != nil || added != "老周" {
		t.Fatalf("采用后应包含新增人物: %q %v", added, err)
	}
	if summary, err := adopted.FieldString("/characters/@char-1/summary"); err != nil || !strings.Contains(summary, "心里有债") {
		t.Fatalf("采用后应包含文本修改: %q %v", summary, err)
	}
}

// waitRevisionState 等待修订动作进入终态并返回其投影。
func waitRevisionState(t *testing.T, service *Service, actionID string) RevisionView {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		views, err := service.RevisionActions(context.Background(), "project-1", true)
		if err != nil {
			t.Fatal(err)
		}
		for _, view := range views {
			if view.ActionID == actionID && (view.State == "succeeded" || view.State == "failed") {
				return view
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("修订动作未进入终态: %s", actionID)
	return RevisionView{}
}
