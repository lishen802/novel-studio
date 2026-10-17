package chapterloop

import (
	"errors"
	"strings"
	"testing"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestOpeningSampleStaysInformalUntilExplicitPromotion(t *testing.T) {
	sample, err := NewOpeningSample("sample-1", "handoff-1", "direction-1", "digest-a", "雾从江面漫上来。")
	if err != nil {
		t.Fatal(err)
	}
	if sample.CountsAsMemory() || sample.CountsAsFinalizedWords() {
		t.Fatal("试写不能进入正式记忆或定稿字数")
	}
	if _, err := sample.Promote(false, true, true); !errors.Is(err, ErrSampleNotPromotable) {
		t.Fatalf("未确认档案: %v", err)
	}
	if _, err := sample.Promote(true, false, true); !errors.Is(err, ErrSampleNotPromotable) {
		t.Fatalf("未复检: %v", err)
	}
	promoted, err := sample.Promote(true, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Status != SamplePromoted || promoted.CountsAsMemory() || promoted.CountsAsFinalizedWords() {
		t.Fatalf("转正后仍应是非正式候选: %+v", promoted)
	}
	stale := sample.MarkStaleIfDigestChanged("digest-b")
	if stale.Status != SampleStale {
		t.Fatal("方向变化应使试写失效")
	}
	if _, err := stale.Promote(true, true, true); !errors.Is(err, ErrSampleNotPromotable) {
		t.Fatalf("失效试写不能转正: %v", err)
	}
	feedback, err := sample.RecordFeedback(FeedbackAdjust)
	if err != nil || feedback.Status != SampleFeedback || feedback.Feedback != FeedbackAdjust {
		t.Fatalf("反馈: %+v %v", feedback, err)
	}
	discarded, err := sample.RecordFeedback(FeedbackDiscard)
	if err != nil || discarded.Status != SampleDiscarded {
		t.Fatalf("放弃: %+v %v", discarded, err)
	}
}

func TestSelectionRewriteRejectsMovedBodyAndPartialAcceptClearsEvaluation(t *testing.T) {
	body := "雾锁江面，灯塔未亮。"
	proposal, err := PrepareSelection("chapter-1", 3, body, 0, len([]rune("雾锁江面")))
	if err != nil {
		t.Fatal(err)
	}
	proposal = proposal.WithReplacement("江雾更浓", "eval-1")
	if _, _, err := ApplySelection(proposal, "雨锁江面，灯塔未亮。", 3, "江雾更浓", false); !errors.Is(err, ErrSelectionStale) {
		t.Fatalf("正文变化: %v", err)
	}
	if _, _, err := ApplySelection(proposal, body, 4, "江雾更浓", false); !errors.Is(err, ErrSelectionStale) {
		t.Fatalf("版本变化: %v", err)
	}
	next, applied, err := ApplySelection(proposal, body, 3, "江雾更浓", false)
	if err != nil {
		t.Fatal(err)
	}
	if next != "江雾更浓，灯塔未亮。" || applied.Status != SelectionAccepted || applied.EvaluationID != "eval-1" {
		t.Fatalf("全部采纳: %q %+v", next, applied)
	}
	partialBody, partial, err := ApplySelection(proposal, body, 3, "只有前半", true)
	if err != nil {
		t.Fatal(err)
	}
	if partialBody != "只有前半，灯塔未亮。" || partial.Status != SelectionPartial || partial.EvaluationID != "" {
		t.Fatalf("部分采纳应使旧评估失效: %q %+v", partialBody, partial)
	}
	rejected := proposal.Reject()
	if rejected.Status != SelectionRejected {
		t.Fatal("放弃应保留原提案并标记拒绝")
	}
}

func TestSpeechAndGuessCannotBecomeWorldFacts(t *testing.T) {
	statements := []Statement{
		{ID: "fact", Text: "灯塔昨夜没有亮", Kind: StatementObjective, Disposition: DispositionAccepted, Evidence: "叙述"},
		{ID: "speech", Text: "船夫说岛上无人", Kind: StatementSpeech, Disposition: DispositionPending, Evidence: "对白"},
		{ID: "guess", Text: "也许有人刚熄灯", Kind: StatementGuess, Disposition: DispositionPending, Evidence: "推断"},
	}
	posthoc, err := NewPosthoc("post-1", "chapter-1", 2, "灯塔昨夜没有亮。船夫说岛上无人。", statements, "key-1")
	if err != nil {
		t.Fatal(err)
	}
	committed, err := posthoc.Commit()
	if err != nil || committed.Status != PosthocCommitted || committed.MemoryPendingRebuild {
		t.Fatalf("提交: %+v %v", committed, err)
	}
	facts := ReadableFacts([]Posthoc{committed})
	if len(facts) != 1 || facts[0].Text != "灯塔昨夜没有亮" {
		t.Fatalf("只能读取已采纳客观事实: %+v", facts)
	}
	bad := statements
	bad[1].Disposition = DispositionAccepted
	if _, err := NewPosthoc("post-2", "chapter-1", 2, posthoc.SourceHash, bad, "key-2"); !errors.Is(err, ErrSpeechNotFact) {
		t.Fatalf("发言不能当事实: %v", err)
	}
	failed := posthoc.Fail()
	if !failed.MemoryPendingRebuild || failed.Status != PosthocFailed {
		t.Fatalf("失败应标记待重建: %+v", failed)
	}
	if err := PreflightMemory(failed, failed.SourceHash); !errors.Is(err, ErrMemoryPendingRebuild) {
		t.Fatalf("预检: %v", err)
	}
	recovered, err := failed.Recover(posthoc.SourceHash)
	if err != nil || recovered.MemoryPendingRebuild || recovered.Status != PosthocCommitted {
		t.Fatalf("同源恢复: %+v %v", recovered, err)
	}
	if _, err := failed.Recover("other-hash"); !errors.Is(err, ErrPosthocSourceChanged) {
		t.Fatalf("来源变化: %v", err)
	}
}

func TestCandidateSelectionUsesValidEvaluationInsteadOfLatestArtifact(t *testing.T) {
	findings := []ProductFinding{
		productFinding("f1", agentdomain.FindingContinuity, "时间跳到次日", ClassHardConflict, false),
		productFinding("f1", agentdomain.FindingContinuity, "时间跳到次日", ClassHardConflict, false),
		productFinding("f2", agentdomain.FindingStyle, "比喻可再克制", ClassPreference, false),
	}
	merged, conflicts, err := MergeProductFindings(findings)
	if err != nil || len(merged) != 2 || len(conflicts) != 0 {
		t.Fatalf("去重: %+v %+v %v", merged, conflicts, err)
	}
	speech := productFinding("f3", agentdomain.FindingWorldRule, "船夫声称的岛规", ClassHardConflict, true)
	downgraded := DowngradeSpeech(speech)
	if downgraded.Class != ClassUncertain || downgraded.BlocksProgress() {
		t.Fatalf("发言不能直接成为硬阻断: %+v", downgraded)
	}
	preference := productFinding("f4", agentdomain.FindingStyle, "语气偏好", ClassPreference, false)
	if _, err := Escalate(preference, ClassHardConflict); !errors.Is(err, ErrPreferenceEscalation) {
		t.Fatalf("偏好不能升级: %v", err)
	}
	verified, err := VerifyFinding(downgraded, VerificationSupported, "")
	if err != nil || verified.Verification != VerificationInsufficient {
		t.Fatalf("没有证据不能判成立: %+v %v", verified, err)
	}
	if err := SameFrozenBody([]string{"hash-a", "hash-a"}); err != nil {
		t.Fatal(err)
	}
	if err := SameFrozenBody([]string{"hash-a", "hash-b"}); !errors.Is(err, ErrVersionChanged) {
		t.Fatalf("同轮审阅: %v", err)
	}
	if _, err := Lookup(map[string]string{"lamp": "灯是新灭的"}, "lamp", false); !errors.Is(err, ErrUnauthorizedLookup) {
		t.Fatal(err)
	}
	if _, err := Lookup(map[string]string{}, "missing", true); !errors.Is(err, ErrLookupMiss) {
		t.Fatal(err)
	}
	early := Candidate{ID: "c-early", Body: "前稿", EvaluationID: "eval-early", Valid: true, Sequence: 1}
	late := Candidate{ID: "c-late", Body: "后稿", EvaluationID: "", Valid: false, Sequence: 2, IntroducedConflict: true}
	selected, err := SelectCandidate([]Candidate{early, late}, "c-early", "前稿评估仍然有效")
	if err != nil || selected.ID != "c-early" {
		t.Fatalf("应选有效前稿: %+v %v", selected, err)
	}
	if _, err := SelectCandidate([]Candidate{early, late}, "c-late", "取最后一份"); !errors.Is(err, ErrCandidateNotSelectable) {
		t.Fatalf("无效后稿不能入选: %v", err)
	}
	if err := NoteRevision([]string{"不改姓名"}, []string{"改掉姓名"}); !errors.Is(err, ErrConstraintChanged) {
		t.Fatalf("作者约束: %v", err)
	}
	if err := AllowCall(RunControl{BudgetExhausted: true}); !errors.Is(err, ErrBudgetStopped) {
		t.Fatal(err)
	}
	gate := HarnessThresholds{MaxFalsePositive: 0.1, MaxMiss: 0.1, MinRevisionGain: 0.2, MaxNewIssueRate: 0.3, MaxCost: 1, Frozen: true}
	if !gate.AllowsDefault(HarnessObservation{FalsePositive: 0.05, Miss: 0.05, RevisionGain: 0.4, NewIssueRate: 0.1, Cost: 0.5}) {
		t.Fatal("达标步骤应能成为默认")
	}
	if gate.AllowsDefault(HarnessObservation{FalsePositive: 0.2, Miss: 0.05, RevisionGain: 0.4, NewIssueRate: 0.1, Cost: 0.5}) {
		t.Fatal("误报超门槛不能成为默认")
	}
	if (HarnessThresholds{Frozen: false}).AllowsDefault(HarnessObservation{}) {
		t.Fatal("未冻结门槛不能成为默认")
	}
}

func productFinding(id string, category agentdomain.FindingCategory, message string, class ProductClass, speech bool) ProductFinding {
	return ProductFinding{
		Finding: agentdomain.ReviewFinding{
			ID: id, Category: category, Severity: agentdomain.SeverityMajor, Status: agentdomain.FindingOpen,
			Source: agentdomain.FindingModelReview, Message: message, ConstraintRef: "keep",
		},
		Class: class, SpeechOrGuess: speech,
	}
}

func TestPrepareSelectionUsesUnicodeRunes(t *testing.T) {
	body := "甲乙丙"
	proposal, err := PrepareSelection("chapter-1", 1, body, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.SourceText != "乙" || strings.Contains(proposal.SourceHash, "甲") {
		t.Fatalf("应按 rune 切片: %+v", proposal)
	}
}
