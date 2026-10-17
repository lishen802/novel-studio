package chapterloop

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	loopdomain "github.com/lishen802/novel-studio/internal/domain/chapterloop"
)

func TestOpeningSampleFeedbackAndPromotionDoNotEnterMemory(t *testing.T) {
	ctx := context.Background()
	service, completer := testService()
	sample, err := service.GenerateOpeningSample(ctx, OpeningSampleRequest{
		ProjectID: "project-1", HandoffID: "handoff-1", DirectionCandidateID: "direction-1",
		DirectionDigest: "digest-a", CurrentDigest: "digest-a", PointOfView: "第三人称限知", Tone: "克制",
		Constraints: []string{"不揭晓身世"}, TargetRunes: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sample.CountsAsMemory() || sample.CountsAsFinalizedWords() || !strings.Contains(sample.Body, "灯塔") {
		t.Fatalf("试写: %+v", sample)
	}
	if _, err := service.GenerateOpeningSample(ctx, OpeningSampleRequest{
		ProjectID: "project-1", HandoffID: "handoff-1", DirectionCandidateID: "direction-1",
		DirectionDigest: "digest-a", CurrentDigest: "digest-b", PointOfView: "第三人称限知", Tone: "克制", TargetRunes: 20,
	}); !errors.Is(err, loopdomain.ErrSampleNotPromotable) || completer.calls != 1 {
		t.Fatalf("旧方向不应再调用模型: %v calls=%d", err, completer.calls)
	}
	feedback, err := service.RecordOpeningSampleFeedback(ctx, "project-1", sample.ID, loopdomain.FeedbackContinuePlan)
	if err != nil || feedback.Feedback != loopdomain.FeedbackContinuePlan {
		t.Fatalf("反馈: %+v %v", feedback, err)
	}
	if _, err := service.PromoteOpeningSample(ctx, PromoteSampleRequest{ProjectID: "project-1", SampleID: sample.ID, ArchiveConfirmed: false, ExplicitAccept: true, CurrentDigest: "digest-a"}); !errors.Is(err, loopdomain.ErrSampleNotPromotable) {
		t.Fatalf("取消转正应保留候选: %v", err)
	}
	promoted, err := service.PromoteOpeningSample(ctx, PromoteSampleRequest{ProjectID: "project-1", SampleID: sample.ID, ArchiveConfirmed: true, ExplicitAccept: true, CurrentDigest: "digest-a"})
	if err != nil || promoted.CountsAsMemory() || promoted.Status != loopdomain.SamplePromoted {
		t.Fatalf("转正: %+v %v", promoted, err)
	}
}

func TestTwoChaptersFinalizeMemoryThenReadOnlyAcceptedFacts(t *testing.T) {
	ctx := context.Background()
	service, completer := testService()
	first, err := service.StartProduction(ctx, ProductionRequest{
		ProjectID: "project-1", RoadmapNodeID: "node-1", ChapterID: "chapter-1", Confirmed: true,
		Manifest: map[string]string{"goal": "登岛", "secret": "SECRET_ARCHIVE"}, CharBudget: 4000,
		Constraints: []string{"不改主角姓名"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.SelectedID == "" || first.SelectionReason == "" || first.Candidates[0].ID == first.Candidates[len(first.Candidates)-1].ID && !first.Candidates[len(first.Candidates)-1].Valid {
		t.Fatalf("应留下有效选稿: %+v", first)
	}
	selected := candidateByID(t, first, first.SelectedID)
	if selected.KnowledgeBoundaryChanged || strings.Contains(writerInput(completer), "SECRET_ARCHIVE") {
		t.Fatal("主写不应看到档案秘密，知识边界变化的修订不能入选")
	}
	if completer.reviewInputs[0] != completer.reviewInputs[1] {
		t.Fatal("同轮审阅必须读取同一冻结正文")
	}
	posthoc, err := service.CommitPosthoc(ctx, PosthocRequest{
		ProjectID: "project-1", ChapterID: "chapter-1", Revision: 2, Body: selected.Body, IdempotencyKey: "post-1",
		Statements: []loopdomain.Statement{
			{ID: "fact", Text: "灯塔昨夜没有亮", Kind: loopdomain.StatementObjective, Disposition: loopdomain.DispositionAccepted, Evidence: "叙述"},
			{ID: "speech", Text: "船夫说岛上无人", Kind: loopdomain.StatementSpeech, Disposition: loopdomain.DispositionPending, Evidence: "对白"},
		},
	})
	if err != nil || posthoc.Status != loopdomain.PosthocCommitted {
		t.Fatalf("后验: %+v %v", posthoc, err)
	}
	before := completer.calls
	again, err := service.CommitPosthoc(ctx, PosthocRequest{ProjectID: "project-1", ChapterID: "chapter-1", Revision: 2, Body: selected.Body, IdempotencyKey: "post-1"})
	if err != nil || again.ID != posthoc.ID || completer.calls != before {
		t.Fatalf("幂等提交不应再次调用模型: %+v %v", again, err)
	}
	second, err := service.StartProduction(ctx, ProductionRequest{
		ProjectID: "project-1", RoadmapNodeID: "node-2", ChapterID: "chapter-2", Confirmed: true,
		Manifest: map[string]string{"goal": "进村"}, CharBudget: 4000, Constraints: []string{"不改主角姓名"},
	})
	if err != nil {
		t.Fatal(err)
	}
	director := completer.inputsByStage["chapter_directing"]
	if !strings.Contains(director[len(director)-1], "灯塔昨夜没有亮") || strings.Contains(director[len(director)-1], "船夫说岛上无人") {
		t.Fatalf("下章只能读取已采纳事实: %s", director[len(director)-1])
	}
	if second.SelectedID == "" {
		t.Fatal("第二章应选出候选")
	}
	failedCompleter := &scriptCompleter{err: errors.New("model down")}
	failedService := NewService(&memoryStore{}, failedCompleter, fixedClock{now: time.Unix(10, 0)}, &seqIDs{})
	failed, err := failedService.CommitPosthoc(ctx, PosthocRequest{ProjectID: "project-1", ChapterID: "chapter-9", Revision: 1, Body: "已定稿正文", IdempotencyKey: "fail-1"})
	if err == nil || !failed.MemoryPendingRebuild {
		t.Fatalf("后验失败应保留待重建且不假定成功: %+v %v", failed, err)
	}
	if _, err := failedService.StartProduction(ctx, ProductionRequest{ProjectID: "project-1", RoadmapNodeID: "node-3", ChapterID: "chapter-3", Confirmed: true, Manifest: map[string]string{"goal": "继续"}, CharBudget: 1000, Constraints: []string{"不改主角姓名"}}); !errors.Is(err, loopdomain.ErrMemoryPendingRebuild) {
		t.Fatalf("预检: %v", err)
	}
}

func TestSelectionAcceptIsBoundToRevisionAndHash(t *testing.T) {
	ctx := context.Background()
	service, _ := testService()
	body := "雾锁江面，灯塔未亮。"
	proposal, err := service.PrepareSelection(ctx, SelectionRequest{ProjectID: "project-1", ChapterID: "chapter-1", Revision: 4, Body: body, StartRune: 0, EndRune: 4, Mode: "rewrite"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.AcceptSelection(ctx, AcceptSelectionRequest{ProjectID: "project-1", ProposalID: proposal.ID, Body: "雨锁江面，灯塔未亮。", Revision: 4, Partial: false}); !errors.Is(err, loopdomain.ErrSelectionStale) {
		t.Fatalf("错位: %v", err)
	}
	next, applied, err := service.AcceptSelection(ctx, AcceptSelectionRequest{ProjectID: "project-1", ProposalID: proposal.ID, Body: body, Revision: 4, Partial: true, Replacement: "江雾"})
	if err != nil || next != "江雾，灯塔未亮。" || applied.EvaluationID != "" {
		t.Fatalf("部分采纳: %q %+v %v", next, applied, err)
	}
	explanation, err := service.ExplainSelection(ctx, SelectionRequest{ProjectID: "project-1", ChapterID: "chapter-1", Revision: 4, Body: body, StartRune: 5, EndRune: 9, Mode: "explain"})
	if err != nil || !strings.Contains(explanation, "灯塔") {
		t.Fatalf("解释: %s %v", explanation, err)
	}
}

func TestHarnessAndStopConditionsDoNotCallTheModel(t *testing.T) {
	ctx := context.Background()
	service, completer := testService()
	_, err := service.StartProduction(ctx, ProductionRequest{ProjectID: "project-1", RoadmapNodeID: "node", ChapterID: "chapter", Confirmed: false, Manifest: map[string]string{"goal": "登岛"}, CharBudget: 100, Constraints: []string{"不改主角姓名"}})
	if err == nil || completer.calls != 0 {
		t.Fatalf("未确认节点: %v calls=%d", err, completer.calls)
	}
	_, err = service.StartProduction(ctx, ProductionRequest{ProjectID: "project-1", RoadmapNodeID: "node", ChapterID: "chapter", Confirmed: true, Manifest: map[string]string{"goal": "登岛"}, CharBudget: 100, Constraints: []string{"不改主角姓名"}, Control: loopdomain.RunControl{ResultUnknown: true}})
	if !errors.Is(err, loopdomain.ErrResultUnknown) || completer.calls != 0 {
		t.Fatalf("结果未知: %v calls=%d", err, completer.calls)
	}
	production, err := service.StartProduction(ctx, ProductionRequest{
		ProjectID: "project-1", RoadmapNodeID: "node", ChapterID: "chapter", Confirmed: true,
		Manifest: map[string]string{"goal": "登岛"}, CharBudget: 4000, Constraints: []string{"不改主角姓名"},
		ExtraStep: &HarnessStep{Name: "reader_style", Thresholds: loopdomain.HarnessThresholds{Frozen: true, MaxFalsePositive: 0.01}, Observation: loopdomain.HarnessObservation{FalsePositive: 0.2, RevisionGain: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, stage := range production.Stages {
		if stage == "reader_style" {
			t.Fatal("未过门槛的步骤不能进入默认流程")
		}
	}
}

type scriptCompleter struct {
	calls         int
	inputsByStage map[string][]string
	reviewInputs  []string
	err           error
}

func (completer *scriptCompleter) Complete(_ context.Context, call Call) (Completion, error) {
	completer.calls++
	if completer.inputsByStage == nil {
		completer.inputsByStage = map[string][]string{}
	}
	completer.inputsByStage[call.Stage] = append(completer.inputsByStage[call.Stage], call.Input)
	if call.Stage == "independent_review" {
		completer.reviewInputs = append(completer.reviewInputs, call.Input)
	}
	if completer.err != nil {
		return Completion{}, completer.err
	}
	switch call.Stage {
	case "prose_writing":
		if strings.Contains(call.Input, "试写") {
			return Completion{Text: "灯塔在雾里熄着。", InputTokens: 3, OutputTokens: 5}, nil
		}
		return Completion{Text: "沈知秋看见灯塔未亮。", InputTokens: 4, OutputTokens: 6}, nil
	case "chapter_directing":
		return Completion{Text: `{"task":"写登岛","pointOfView":"限知"}`, InputTokens: 2, OutputTokens: 2}, nil
	case "independent_review":
		return Completion{Text: `{"findings":[{"id":"style-1","category":"knowledge_boundary","message":"修订可能让配角提前知道真相","class":"uncertain","speech":false}]}`, InputTokens: 1, OutputTokens: 1}, nil
	case "revision_verify":
		return Completion{Text: `{"results":[{"id":"style-1","status":"supported","evidence":"原文没有该信息"}]}`, InputTokens: 1, OutputTokens: 1}, nil
	case "bounded_revision":
		return Completion{Text: `{"prose":"改写后的正文","kept":["不改主角姓名"],"knowledgeBoundaryChanged":true}`, InputTokens: 2, OutputTokens: 2}, nil
	case "explain":
		return Completion{Text: "灯塔未亮是本章的悬念。", InputTokens: 1, OutputTokens: 1}, nil
	default:
		return Completion{Text: "改写候选", InputTokens: 1, OutputTokens: 1}, nil
	}
}

func testService() (*Service, *scriptCompleter) {
	completer := &scriptCompleter{}
	return NewService(&memoryStore{}, completer, fixedClock{now: time.Unix(10, 0)}, &seqIDs{}), completer
}

func writerInput(completer *scriptCompleter) string {
	inputs := completer.inputsByStage["prose_writing"]
	if len(inputs) == 0 {
		return ""
	}
	return inputs[0]
}

func candidateByID(t *testing.T, production Production, id string) loopdomain.Candidate {
	t.Helper()
	for _, candidate := range production.Candidates {
		if candidate.ID == id {
			return candidate
		}
	}
	t.Fatalf("缺少候选 %s", id)
	return loopdomain.Candidate{}
}

type fixedClock struct{ now time.Time }

func (clock fixedClock) Now() time.Time { return clock.now }

type seqIDs struct{ n int }

func (ids *seqIDs) NewID() string {
	ids.n++
	return strings.Repeat("n", ids.n)
}

type memoryStore struct{ records []Record }

func (store *memoryStore) Append(_ context.Context, record Record) error {
	for _, existing := range store.records {
		if record.IdempotencyKey != "" && existing.ProjectID == record.ProjectID && existing.Kind == record.Kind && existing.SubjectID == record.SubjectID && existing.IdempotencyKey == record.IdempotencyKey {
			return errors.New("duplicate")
		}
	}
	store.records = append(store.records, record)
	return nil
}

func (store *memoryStore) Latest(_ context.Context, projectID, kind, subjectID string) (Record, error) {
	var found Record
	ok := false
	for _, record := range store.records {
		if record.ProjectID == projectID && record.Kind == kind && record.SubjectID == subjectID {
			found = record
			ok = true
		}
	}
	if !ok {
		return Record{}, errors.New("not found")
	}
	return found, nil
}

func (store *memoryStore) Get(_ context.Context, projectID, id string) (Record, error) {
	for _, record := range store.records {
		if record.ProjectID == projectID && record.ID == id {
			return record, nil
		}
	}
	return Record{}, errors.New("not found")
}

func (store *memoryStore) List(_ context.Context, projectID, kind string) ([]Record, error) {
	result := []Record{}
	for _, record := range store.records {
		if record.ProjectID == projectID && record.Kind == kind {
			result = append(result, record)
		}
	}
	return result, nil
}

var _ = json.RawMessage(nil)
