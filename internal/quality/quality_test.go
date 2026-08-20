package quality

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestBuiltinRegistryRequiresExactImmutablePolicyVersion(t *testing.T) {
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	policy, err := registry.Policy(1)
	if err != nil {
		t.Fatal(err)
	}
	if policy.Version != 1 || !agentdomain.ValidSHA256(policy.Hash) || len(policy.Rules) < 10 {
		t.Fatalf("unexpected policy: %#v", policy)
	}
	policy.Rules[0].ID = "mutated"
	again, err := registry.Policy(1)
	if err != nil || again.Rules[0].ID == "mutated" {
		t.Fatal("registry leaked mutable policy")
	}
	if _, err := registry.Policy(2); !errors.Is(err, ErrPolicyNotFound) {
		t.Fatalf("expected exact version failure, got %v", err)
	}
}

func TestBuiltinRulesShareCleanNegativeFixture(t *testing.T) {
	engine := newTestEngine(t)
	text := "钟楼三响，雾港的门缓缓开启。\n\n林舟收起短刀，沿湿滑石阶走向码头。\n\n守卫没有追来，远处船灯却突然熄灭。"
	report, err := engine.Evaluate(1, Input{
		Namespace: "step-clean", Stage: StageInitial, Text: text,
		Spec: Spec{
			TargetRunes:     utf8.RuneCountInString(text),
			RequiredAnchors: []Constraint{{ID: "a1", Text: "钟楼三响"}},
			ForbiddenItems:  []Constraint{{ID: "f1", Text: "黑刃"}},
			ReservedNames:   []Constraint{{ID: "n1", Text: "顾临渊"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("clean fixture findings=%#v", report.Findings)
	}
}

func TestParagraphRulesSupportWindowsLineEndings(t *testing.T) {
	engine := newTestEngine(t)
	report, err := engine.Evaluate(1, Input{
		Namespace: "step-crlf", Stage: StageInitial,
		Text: "潮声淹没了脚步。\r\n\r\n潮声淹没了脚步。",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !hasRule(report.Findings, "repetition.duplicate_paragraph") {
		t.Fatalf("CRLF duplicate paragraph not detected: %#v", report.Findings)
	}
}

func TestBuiltinRulesHaveMinimalPositiveFixtures(t *testing.T) {
	engine := newTestEngine(t)
	cases := []struct {
		name   string
		text   string
		spec   Spec
		ruleID string
	}{
		{"empty", "   ", Spec{}, "structure.non_empty"},
		{"length", "太短。", Spec{TargetRunes: 100}, "length.target_deviation"},
		{"code fence", "```markdown\n正文\n```", Spec{}, "format.code_fence"},
		{"meta narration", "以下是正文：雨落在门前。", Spec{}, "format.meta_narration"},
		{"required anchor", "雨落在门前。", Spec{RequiredAnchors: []Constraint{{ID: "a1", Text: "钟楼三响"}}}, "constraint.required_anchor"},
		{"forbidden item", "他拔出黑刃。", Spec{ForbiddenItems: []Constraint{{ID: "f1", Text: "黑刃"}}}, "constraint.forbidden_item"},
		{"reserved name", "顾临渊走进雨中。", Spec{ReservedNames: []Constraint{{ID: "n1", Text: "顾临渊"}}}, "originality.reserved_name"},
		{"sentence repetition", "雨落在门前。雨落在门前。", Spec{}, "repetition.consecutive_sentence"},
		{"paragraph repetition", "潮声淹没了脚步。\n\n潮声淹没了脚步。", Spec{}, "repetition.duplicate_paragraph"},
		{"short paragraphs", "雨。\n\n风。\n\n夜。\n\n他终于推开门走进屋内。", Spec{}, "style.short_paragraph_ratio"},
		{"same opening", "钟声响起。钟声停下。钟声再来。", Spec{}, "style.repeated_opening"},
		{"mechanical parallel", "不是为了荣誉，而是为了生存。不是为了复仇，而是为了真相。不是为了逃跑，而是为了回家。", Spec{}, "style.mechanical_parallel"},
		{"summary tone", "总而言之，这一切都说明他必须离开。", Spec{}, "style.summary_tone"},
		{"high frequency phrase", "潮声不断逼近。潮声不断抬高。潮声不断回荡。", Spec{}, "repetition.high_frequency_phrase"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			report, err := engine.Evaluate(1, Input{Namespace: "step-1", Stage: StageInitial, Text: test.text, Spec: test.spec})
			if err != nil {
				t.Fatal(err)
			}
			if !hasRule(report.Findings, test.ruleID) {
				t.Fatalf("missing %s in %#v", test.ruleID, report.Findings)
			}
		})
	}
}

func TestLintUsesUnicodeRuneOffsetsAndDeterministicOutput(t *testing.T) {
	engine := newTestEngine(t)
	input := Input{
		Namespace: "step-1", Stage: StageFinal, Text: "夜色里，黑刃出鞘。",
		Spec: Spec{ForbiddenItems: []Constraint{{ID: "f1", Text: "黑刃"}}},
	}
	first, err := engine.Evaluate(1, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.Evaluate(1, input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("reports differ:\n%#v\n%#v", first, second)
	}
	finding := findingByRule(t, first.Findings, "constraint.forbidden_item")
	if finding.ScopeStart == nil || finding.ScopeEnd == nil || *finding.ScopeStart != 4 || *finding.ScopeEnd != 6 || finding.Quote != "黑刃" {
		t.Fatalf("unexpected rune range: %#v", finding)
	}
}

func TestGateDecisionMatrixCannotBeOverriddenByModelSummary(t *testing.T) {
	cases := []struct {
		name     string
		findings []agentdomain.ReviewFinding
		complete bool
		budget   bool
		expected agentdomain.QualityStatus
	}{
		{"passed", nil, true, true, agentdomain.QualityPassed},
		{"minor passes", []agentdomain.ReviewFinding{testFinding("minor", agentdomain.SeverityMinor, agentdomain.FindingOpen)}, true, true, agentdomain.QualityPassed},
		{"major review", []agentdomain.ReviewFinding{testFinding("major", agentdomain.SeverityMajor, agentdomain.FindingOpen)}, true, true, agentdomain.QualityNeedsReview},
		{"blocker blocks", []agentdomain.ReviewFinding{testFinding("blocker", agentdomain.SeverityBlocker, agentdomain.FindingOpen)}, true, true, agentdomain.QualityBlocked},
		{"resolved blocker passes", []agentdomain.ReviewFinding{testFinding("blocker", agentdomain.SeverityBlocker, agentdomain.FindingResolved)}, true, true, agentdomain.QualityPassed},
		{"incomplete", nil, false, true, agentdomain.QualityNeedsReview},
		{"budget", nil, true, false, agentdomain.QualityNeedsReview},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			report := Decide(StageFinal, test.findings, test.complete, test.budget)
			if report.Status != test.expected {
				t.Fatalf("got %s, want %s: %#v", report.Status, test.expected, report)
			}
		})
	}
}

func TestReconcileAppliesResolutionAndKeepsNewFinalViolations(t *testing.T) {
	before := []agentdomain.ReviewFinding{
		testFinding("f1", agentdomain.SeverityBlocker, agentdomain.FindingOpen),
		testFinding("f2", agentdomain.SeverityMajor, agentdomain.FindingOpen),
	}
	resolutions := []agentdomain.FindingResolution{
		{FindingID: "f1", Status: agentdomain.FindingResolved, Note: "已恢复锚点"},
		{FindingID: "f2", Status: agentdomain.FindingRetained, Note: "保留风险"},
	}
	introduced := testFinding("final:new", agentdomain.SeverityBlocker, agentdomain.FindingOpen)
	result, err := Reconcile(before, resolutions, []agentdomain.ReviewFinding{introduced})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 || result[0].Status != agentdomain.FindingResolved || result[1].Status != agentdomain.FindingRetained || result[2].ID != introduced.ID {
		t.Fatalf("unexpected reconciliation: %#v", result)
	}
	if Decide(StageFinal, result, true, true).Status != agentdomain.QualityBlocked {
		t.Fatal("new final blocker must remain authoritative")
	}
}

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	registry, err := BuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	return NewEngine(registry)
}

func hasRule(findings []agentdomain.ReviewFinding, ruleID string) bool {
	for _, finding := range findings {
		if strings.Contains(finding.ID, ":"+ruleID+":") {
			return true
		}
	}
	return false
}

func findingByRule(t *testing.T, findings []agentdomain.ReviewFinding, ruleID string) agentdomain.ReviewFinding {
	t.Helper()
	for _, finding := range findings {
		if strings.Contains(finding.ID, ":"+ruleID+":") {
			return finding
		}
	}
	t.Fatalf("missing rule %s", ruleID)
	return agentdomain.ReviewFinding{}
}

func testFinding(id string, severity agentdomain.FindingSeverity, status agentdomain.FindingStatus) agentdomain.ReviewFinding {
	return agentdomain.ReviewFinding{
		ID: id, Category: agentdomain.FindingStyle, Severity: severity, Status: status,
		Source: agentdomain.FindingDeterministicLint, Message: "测试问题", ConstraintRef: "rule:test",
	}
}
