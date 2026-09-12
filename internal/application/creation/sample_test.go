package creation

import (
	"context"
	"errors"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"testing"
)

func TestSamplePreparationRestoresAndBecomesStale(t *testing.T) {
	ctx := context.Background()
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	session := seedIntentSession(t, service)
	candidate, err := creationdomain.NewCandidate("sample-direction", session.ID, creationdomain.StageDirections, creationdomain.KindDirectionSet, 1, "", "", "", directionPayload, "story_directions.schema", 1, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	candidate = candidate.WithUpstream(liveUpstreamDigest(session, nil, creationdomain.StageDirections)).WithQuality(creationdomain.QualityPassed)
	if err := store.SaveCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	// 选择准确方向卡后才允许保存试写准备。
	if _, err := service.AdvanceStage(ctx, session.ProjectID); err != nil {
		t.Fatal(err)
	}

	session, err = service.SelectDirection(ctx, session.ProjectID, candidate.ID, "d1")
	if err != nil {
		t.Fatal(err)
	}
	req := SaveSampleRequest{ProjectID: session.ProjectID, SessionID: session.ID, ExpectedVersion: session.ContentVersion, ExpectedRunSlotVersion: session.RunSlotVersion, DirectionCandidateID: candidate.ID, DirectionCardID: "d1", PointOfView: "第三人称限知", Tone: "轻松", Constraints: []string{"不揭晓身世"}, TargetRunes: 1200}
	saved, err := service.SaveSample(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveSample(ctx, req); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("过期提交: %v", err)
	}
	restarted, _ := New(Config{Store: store, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	view, err := restarted.Workbench(ctx, session.ProjectID, session.BranchID)
	if err != nil {
		t.Fatal(err)
	}
	if view.SampleHandoff == nil || view.SampleHandoff.ID != saved.ID || view.SampleHandoff.Status != "prepared" {
		t.Fatalf("恢复: %+v", view.SampleHandoff)
	}
	if _, err := service.SaveIntent(ctx, session.ProjectID, "修改故事意图"); err != nil {
		t.Fatal(err)
	}
	view, err = service.Workbench(ctx, session.ProjectID, session.BranchID)
	if err != nil {
		t.Fatal(err)
	}
	if view.SampleHandoff.Status != "stale" {
		t.Fatal("意图修改应使交接失效")
	}
}
