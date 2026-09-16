package creation

import (
	"context"
	"errors"
	"testing"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// seedPeopleWorld 建立人物世界阶段的当前候选，供逐项审阅与版本管理测试使用。
func seedPeopleWorld(t *testing.T, service *Service, store *memoryStore) creationdomain.Candidate {
	t.Helper()
	seedIntentSession(t, service)
	return seedStageCandidate(t, service, store, creationdomain.StagePeopleWorld, peopleWorldPayload)
}

// 逐项接受只改审阅元数据：候选载荷、状态与锁都不变。
func TestSetReviewOnlyChangesReviewMetadata(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	candidate := seedPeopleWorld(t, service, store)
	before := candidate
	view, err := service.SetReview(context.Background(), "project-1", ReviewRequest{
		CandidateID: candidate.ID, Scope: "entity", RefID: "character:char-1", Decision: "accept",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Accepted) != 1 || view.Accepted[0].ID != "character:char-1" || view.Complete {
		t.Fatalf("review view = %#v", view)
	}
	if len(view.Unreviewed) != 1 || view.Unreviewed[0].ID != "rule:潮汐律" {
		t.Fatalf("unreviewed = %#v", view.Unreviewed)
	}
	stored, err := store.Candidate(context.Background(), candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != before.Status || stored.ContentSHA256 != before.ContentSHA256 || len(stored.LockMap) != 0 {
		t.Fatalf("review must not change candidate: %#v", stored)
	}
}

// 未出现在候选中的单位不能被决定，避免审阅记录出现悬空引用。
func TestSetReviewRejectsReferenceOutsideCandidate(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	candidate := seedPeopleWorld(t, service, store)
	_, err := service.SetReview(context.Background(), "project-1", ReviewRequest{
		CandidateID: candidate.ID, Scope: "entity", RefID: "character:不存在", Decision: "accept",
	})
	if !errors.Is(err, creationdomain.ErrReviewRefMissing) {
		t.Fatalf("error = %v", err)
	}
	if _, err := service.SetReview(context.Background(), "project-1", ReviewRequest{
		CandidateID: candidate.ID, Scope: "section", RefID: "position", Decision: "accept",
	}); !errors.Is(err, creationdomain.ErrReviewRefMissing) {
		t.Fatalf("section ref error = %v", err)
	}
}

// 历史版本只读：被替代的候选不能继续接受或排除。
func TestSetReviewRejectsSupersededCandidate(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	candidate := seedPeopleWorld(t, service, store)
	if _, err := service.SaveFields(context.Background(), "project-1", SaveFieldsRequest{
		CandidateID: candidate.ID, Changes: map[string]string{"/characters/@char-1/summary": "改过的概要"},
	}); err != nil {
		t.Fatal(err)
	}
	_, err := service.SetReview(context.Background(), "project-1", ReviewRequest{
		CandidateID: candidate.ID, Scope: "entity", RefID: "character:char-1", Decision: "accept",
	})
	if !errors.Is(err, creationdomain.ErrReviewTargetNotCurrent) {
		t.Fatalf("error = %v", err)
	}
}

// 新候选版本必须重新审阅，旧版本的接受与排除不继承。
func TestNewCandidateVersionRequiresFreshReview(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	candidate := seedPeopleWorld(t, service, store)
	if _, err := service.AcceptAllReview(context.Background(), "project-1", candidate.ID); err != nil {
		t.Fatal(err)
	}
	next, err := service.SaveFields(context.Background(), "project-1", SaveFieldsRequest{
		CandidateID: candidate.ID, Changes: map[string]string{"/characters/@char-1/summary": "改过的概要"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if next.ID == candidate.ID {
		t.Fatal("author edit must create a new candidate version")
	}
	old, err := store.Review(context.Background(), candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(old.Accepted) != 2 {
		t.Fatalf("historical review must be kept: %#v", old)
	}
	view, err := service.SetReview(context.Background(), "project-1", ReviewRequest{
		CandidateID: next.ID, Scope: "entity", RefID: "rule:潮汐律", Decision: "reject",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Accepted) != 0 || len(view.Rejected) != 1 || len(view.Unreviewed) != 1 || view.Complete {
		t.Fatalf("new version review = %#v", view)
	}
}

// 全部接受是显式命令，且不覆盖未被列举的单位。
func TestAcceptAllReviewCoversEveryUnit(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	candidate := seedPeopleWorld(t, service, store)
	view, err := service.AcceptAllReview(context.Background(), "project-1", candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Complete || len(view.Accepted) != 2 || len(view.Unreviewed) != 0 {
		t.Fatalf("accept all = %#v", view)
	}
}

// 版本选择显式切换工作基线，重复调用幂等，旧版本重新成为当前版本。
func TestUseCandidateVersionSwitchesBaseline(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	first := seedPeopleWorld(t, service, store)
	if _, err := service.SetReview(context.Background(), "project-1", ReviewRequest{
		CandidateID: first.ID, Scope: "entity", RefID: "character:char-1", Decision: "accept",
	}); err != nil {
		t.Fatal(err)
	}
	second, err := service.SaveFields(context.Background(), "project-1", SaveFieldsRequest{
		CandidateID: first.ID, Changes: map[string]string{"/characters/@char-1/summary": "改过的概要"},
	})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := service.UseCandidateVersion(context.Background(), "project-1", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.ID != first.ID || restored.Status != creationdomain.StatusDraft {
		t.Fatalf("restored = %#v", restored)
	}
	again, err := service.UseCandidateVersion(context.Background(), "project-1", first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != first.ID {
		t.Fatalf("idempotent switch changed candidate: %#v", again)
	}
	candidates, err := store.ListCandidates(context.Background(), first.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := currentCandidate(candidates, creationdomain.StagePeopleWorld)
	if !ok || current.ID != first.ID {
		t.Fatalf("working baseline = %#v", current)
	}
	superseded, err := store.Candidate(context.Background(), second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if superseded.Status != creationdomain.StatusSuperseded {
		t.Fatalf("previous version status = %q", superseded.Status)
	}
	workbench, err := service.Workbench(context.Background(), "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	if workbench.PeopleWorld.CandidateID != first.ID {
		t.Fatalf("workbench baseline = %q", workbench.PeopleWorld.CandidateID)
	}
	// 恢复的历史版本带回它自己的审阅记录。
	if len(workbench.Review.Accepted) != 1 || len(workbench.Review.Unreviewed) != 1 {
		t.Fatalf("restored review = %#v", workbench.Review)
	}
}

// seedPeopleWorldWithUpstream 建立带完整上游的人物世界阶段，供阶段接受相关测试使用。
func seedPeopleWorldWithUpstream(t *testing.T, store *memoryStore) creationdomain.Candidate {
	t.Helper()
	now := testClock{}.Now()
	session, err := creationdomain.NewSession("session-1", "project-1", "branch-1", now)
	if err != nil {
		t.Fatal(err)
	}
	if session, err = session.SaveIntent("守关人必须放走仇敌", now); err != nil {
		t.Fatal(err)
	}
	dir, err := creationdomain.NewCandidate(
		"dir-set-1", session.ID, creationdomain.StageDirections, creationdomain.KindDirectionSet,
		1, "", "run-dir", "art-dir", directionPayload, "story_directions.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := creationdomain.NewCandidate(
		"proposal-1", session.ID, creationdomain.StageProposal, creationdomain.KindProposal,
		1, "", "run-prop", "art-prop", proposalPayload, "story_blueprint.schema", 2, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	people, err := creationdomain.NewCandidate(
		"people-1", session.ID, creationdomain.StagePeopleWorld, creationdomain.KindPeopleWorld,
		1, "", "run-cast", "art-cast", peopleWorldPayload, "story_cast.schema", 2, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	session.SelectedDirectionCandidateID = dir.ID
	session.SelectedDirectionCardID = "d1"
	session.Stage = creationdomain.StagePeopleWorld
	if session, err = store.CreateSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	candidates := []creationdomain.Candidate{dir, proposal, people}
	for index, candidate := range candidates {
		candidate = candidate.WithUpstream(liveUpstreamDigest(session, candidates, candidate.Stage))
		if err := store.SaveCandidate(context.Background(), candidate); err != nil {
			t.Fatal(err)
		}
		candidates[index] = candidate
	}
	return candidates[2]
}

// 阶段已接受的候选不能被静默换版；切换需要作者先回到该阶段重新确认。
func TestUseCandidateVersionBlockedAfterStageAccept(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	first := seedPeopleWorldWithUpstream(t, store)
	second, err := service.SaveFields(context.Background(), "project-1", SaveFieldsRequest{
		CandidateID: first.ID, Changes: map[string]string{"/characters/@char-1/summary": "改过的概要"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptCandidate(context.Background(), "project-1", second.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UseCandidateVersion(context.Background(), "project-1", first.ID); !errors.Is(err, creationdomain.ErrCandidateAcceptanceRequired) {
		t.Fatalf("error = %v", err)
	}
}

// 版本比较是只读投影：返回全部版本与相对差异，不切换工作基线。
func TestCompareCandidateVersionsIsReadOnly(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	first := seedPeopleWorld(t, service, store)
	second, err := service.SaveFields(context.Background(), "project-1", SaveFieldsRequest{
		CandidateID: first.ID, Changes: map[string]string{"/characters/@char-1/summary": "改过的概要"},
	})
	if err != nil {
		t.Fatal(err)
	}
	comparison, err := service.CompareCandidateVersions(context.Background(), "project-1", "people_world", "")
	if err != nil {
		t.Fatal(err)
	}
	if comparison.BaseCandidateID != second.ID || len(comparison.Versions) != 2 {
		t.Fatalf("comparison = %#v", comparison)
	}
	newest := comparison.Versions[0]
	if newest.CandidateID != second.ID || !newest.Current || newest.Version != 2 {
		t.Fatalf("newest = %#v", newest)
	}
	if len(newest.Diff) != 0 {
		t.Fatalf("baseline must not differ from itself: %#v", newest.Diff)
	}
	older := comparison.Versions[1]
	if older.Current || len(older.Diff) != 1 || older.Diff[0].Path != "/characters/@char-1/summary" {
		t.Fatalf("older = %#v", older)
	}
	if older.Diff[0].Before != "改过的概要" || older.Diff[0].After != "潮汐关最后的守将" {
		t.Fatalf("older diff = %#v", older.Diff[0])
	}
	candidates, err := store.ListCandidates(context.Background(), first.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := currentCandidate(candidates, creationdomain.StagePeopleWorld)
	if !ok || current.ID != second.ID {
		t.Fatalf("comparison must not switch baseline: %#v", current)
	}
}

// 审阅位置独立版本化，服务重建后仍可恢复同一实体。
func TestReviewCursorSurvivesServiceRebuild(t *testing.T) {
	service, store, kernel := newTestService(t, newFakeKernel(), nil)
	candidate := seedPeopleWorld(t, service, store)
	saved, err := service.SaveReviewCursor(context.Background(), "project-1", ReviewCursorRequest{
		Stage: "people_world", CandidateID: candidate.ID, EntityID: "character:char-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved.ViewVersion != 1 || saved.EntityID != "character:char-1" {
		t.Fatalf("cursor = %#v", saved)
	}
	moved, err := service.SaveReviewCursor(context.Background(), "project-1", ReviewCursorRequest{
		Stage: "people_world", CandidateID: candidate.ID, EntityID: "rule:潮汐律", ComparisonRef: candidate.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if moved.ViewVersion != 2 || moved.EntityID != "rule:潮汐律" {
		t.Fatalf("moved cursor = %#v", moved)
	}
	if _, err := service.SaveReviewCursor(context.Background(), "project-1", ReviewCursorRequest{
		Stage: "people_world", CandidateID: candidate.ID, EntityID: "rule:不存在",
	}); !errors.Is(err, creationdomain.ErrReviewRefMissing) {
		t.Fatalf("unknown entity error = %v", err)
	}
	rebuilt, err := New(Config{Store: store, Kernel: kernel, Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	workbench, err := rebuilt.Workbench(context.Background(), "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	if workbench.ReviewCursor.EntityID != "rule:潮汐律" || workbench.ReviewCursor.ViewVersion != 2 {
		t.Fatalf("restored cursor = %#v", workbench.ReviewCursor)
	}
	if len(workbench.CandidateVersions) != 1 || !workbench.CandidateVersions[0].Current {
		t.Fatalf("versions = %#v", workbench.CandidateVersions)
	}
}

// 审阅位置不使生成输入失效：保存位置后会话内容版本保持不变。
func TestReviewCursorDoesNotChangeContentVersion(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	candidate := seedPeopleWorld(t, service, store)
	session, err := store.Session(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveReviewCursor(context.Background(), "project-1", ReviewCursorRequest{
		Stage: "people_world", CandidateID: candidate.ID, SectionID: "", EntityID: "character:char-1",
	}); err != nil {
		t.Fatal(err)
	}
	after, err := store.Session(context.Background(), "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if after.ContentVersion != session.ContentVersion || after.RunSlotVersion != session.RunSlotVersion {
		t.Fatalf("cursor must not bump session versions: %#v -> %#v", session, after)
	}
}
