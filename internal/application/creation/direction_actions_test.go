package creation

import (
	"context"
	"reflect"
	"testing"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func seedDirectionStage(t *testing.T, service *Service, store *memoryStore) (creationdomain.Session, creationdomain.Candidate) {
	t.Helper()
	session := seedIntentSession(t, service)
	advanced, err := session.Advance(testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := service.saveSession(context.Background(), &advanced); err != nil {
		t.Fatal(err)
	}
	return advanced, mustCandidate(t, store, advanced, directionPayload)
}

func TestRegenerateDirectionChangesOnlyTargetCardAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session, base := seedDirectionStage(t, service, store)
	var err error
	session, err = service.SelectDirection(ctx, session.ProjectID, base.ID, "d2")
	if err != nil {
		t.Fatal(err)
	}
	beforeAny, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryDirections, []byte(base.PayloadJSON))
	if err != nil {
		t.Fatal(err)
	}
	before := beforeAny.(*agentdomain.StoryDirectionsOutput)
	profile := generateRequest(creationdomain.StageDirections).Profile
	preview, err := service.PreviewRegenerateDirection(ctx, session.ProjectID, DirectionActionRequest{
		CandidateID: base.ID, TargetCardID: "d2", Instruction: "让冲突更紧迫", IdempotencyKey: "redo-d2",
	}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if preview.State != "pending_confirmation" || kernel.createN != 0 {
		t.Fatalf("preview=%+v calls=%d", preview, kernel.createN)
	}
	command := DirectionActionCommand{ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey}
	if _, err := service.ConfirmDirectionAction(ctx, session.ProjectID, command, profile); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		views, loadErr := service.DirectionActions(ctx, session.ProjectID, false)
		return loadErr == nil && len(views) == 1 && views[0].State == "succeeded"
	}) {
		t.Fatal("单卡重做没有完成")
	}
	candidates, err := store.ListCandidates(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := currentCandidateOfKind(candidates, creationdomain.StageDirections, creationdomain.KindDirectionSet)
	if !ok || current.Version != 2 {
		t.Fatalf("current=%+v", current)
	}
	afterAny, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryDirections, []byte(current.PayloadJSON))
	if err != nil {
		t.Fatal(err)
	}
	after := afterAny.(*agentdomain.StoryDirectionsOutput)
	if !reflect.DeepEqual(before.Directions[0], after.Directions[0]) || !reflect.DeepEqual(before.Directions[2], after.Directions[2]) {
		t.Fatalf("非目标卡发生变化: before=%+v after=%+v", before.Directions, after.Directions)
	}
	if after.Directions[1].ID != "d2" || after.Directions[1].Hook == before.Directions[1].Hook {
		t.Fatalf("目标卡身份或内容错误: %+v", after.Directions[1])
	}
	selected, err := service.loadSession(ctx, session.ProjectID)
	if err != nil || selected.SelectedDirectionCandidateID != current.ID || selected.SelectedDirectionCardID != "d2" {
		t.Fatalf("选择未迁移到新版本: %+v err=%v", selected, err)
	}
	if _, err := service.ConfirmDirectionAction(ctx, session.ProjectID, command, profile); err != nil {
		t.Fatal(err)
	}
	if kernel.createN != 1 || kernel.executeN != 1 {
		t.Fatalf("重复确认触发模型: create=%d execute=%d", kernel.createN, kernel.executeN)
	}
	if !waitFor(t, func() bool { return !service.coordinator.Live(preview.RunID) }) {
		t.Fatal("运行租约未释放")
	}
	action, err := store.Action(ctx, preview.ActionID)
	if err != nil {
		t.Fatal(err)
	}
	action.State = assistantdomain.StateExecuting
	if err := store.SaveAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.DirectionActions(ctx, session.ProjectID, true)
	if err != nil || len(recovered) != 1 || recovered[0].State != "succeeded" || kernel.createN != 1 || kernel.executeN != 1 {
		t.Fatalf("恢复发生重放: views=%+v err=%v create=%d execute=%d", recovered, err, kernel.createN, kernel.executeN)
	}
}

func TestCombineDirectionsCreatesIndependentCandidateAndRequiresExplicitSelection(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session, base := seedDirectionStage(t, service, store)
	if _, err := service.KeepDirection(ctx, session.ProjectID, "d1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.KeepDirection(ctx, session.ProjectID, "d2"); err != nil {
		t.Fatal(err)
	}
	session, err := service.SelectDirection(ctx, session.ProjectID, base.ID, "d1")
	if err != nil {
		t.Fatal(err)
	}
	profile := generateRequest(creationdomain.StageDirections).Profile
	preview, err := service.PreviewCombineDirections(ctx, session.ProjectID, DirectionActionRequest{
		CandidateID: base.ID, SourceCardIDs: []string{"d2", "d1"}, Instruction: "保留悬疑卖点并采用更快节奏", IdempotencyKey: "combine-1",
	}, profile)
	if err != nil {
		t.Fatal(err)
	}
	command := DirectionActionCommand{ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey}
	if _, err := service.ConfirmDirectionAction(ctx, session.ProjectID, command, profile); err != nil {
		t.Fatal(err)
	}
	var result DirectionActionView
	if !waitFor(t, func() bool {
		views, loadErr := service.DirectionActions(ctx, session.ProjectID, false)
		if loadErr == nil && len(views) == 1 && views[0].State == "succeeded" {
			result = views[0]
			return true
		}
		return false
	}) {
		t.Fatal("方向组合没有完成")
	}
	unchanged, err := store.Candidate(ctx, base.ID)
	if err != nil || unchanged.Status == creationdomain.StatusSuperseded {
		t.Fatalf("原三卡候选被替换: %+v %v", unchanged, err)
	}
	currentSession, err := service.loadSession(ctx, session.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if currentSession.SelectedDirectionCandidateID != base.ID {
		t.Fatal("组合结果在作者采用前改变了选择")
	}
	combined, err := store.Candidate(ctx, result.ResultCandidateID)
	if err != nil || combined.Kind != creationdomain.KindDirection {
		t.Fatalf("组合候选=%+v err=%v", combined, err)
	}
	if _, err := service.SelectDirection(ctx, session.ProjectID, combined.ID, result.ResultCardID); err != nil {
		t.Fatal(err)
	}
	selected, _ := service.loadSession(ctx, session.ProjectID)
	if selected.SelectedDirectionCandidateID != combined.ID || selected.SelectedDirectionCardID != result.ResultCardID {
		t.Fatalf("未采用精确组合版本: %+v", selected)
	}
	advanced, err := service.AdvanceStage(ctx, session.ProjectID)
	if err != nil || advanced.Stage != creationdomain.StageProposal {
		t.Fatalf("组合方向无法推进: stage=%s err=%v", advanced.Stage, err)
	}
}

func TestDirectionActionRejectsChangedFrozenStateBeforeModelCall(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	session, base := seedDirectionStage(t, service, store)
	profile := generateRequest(creationdomain.StageDirections).Profile
	preview, err := service.PreviewRegenerateDirection(ctx, session.ProjectID, DirectionActionRequest{
		CandidateID: base.ID, TargetCardID: "d1", IdempotencyKey: "stale-redo",
	}, profile)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.KeepDirection(ctx, session.ProjectID, "d1"); err != nil {
		t.Fatal(err)
	}
	_, err = service.ConfirmDirectionAction(ctx, session.ProjectID, DirectionActionCommand{
		ActionID: preview.ActionID, ArgumentsHash: preview.ArgumentsHash, IdempotencyKey: preview.IdempotencyKey,
	}, profile)
	if err == nil || kernel.createN != 0 || kernel.executeN != 0 {
		t.Fatalf("冻结状态变化后仍启动模型: err=%v create=%d execute=%d", err, kernel.createN, kernel.executeN)
	}
}
