package agent

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCapabilityUsesR21RoleSlots(t *testing.T) {
	cases := map[Capability]Role{
		CapabilityStoryArchitect: RolePlanner, CapabilityChapterPlanner: RolePlanner,
		CapabilityProseWriter: RoleWriter, CapabilityBoundedRevision: RoleWriter,
		CapabilityContinuityReview: RoleContinuityReviewer, CapabilityContentReview: RoleContinuityReviewer,
		CapabilityStyleReview: RoleContinuityReviewer,
	}
	for capability, expected := range cases {
		if got := DefaultRoleForCapability(capability); got != expected {
			t.Fatalf("%s: got %s, want %s", capability, got, expected)
		}
	}
}

func TestBoundedRevisionAllowsWriterAndPlannerSlots(t *testing.T) {
	for _, role := range []Role{RoleWriter, RolePlanner} {
		if !ValidRoleForCapability(CapabilityBoundedRevision, role) {
			t.Fatalf("bounded revision should allow %s", role)
		}
	}
	if ValidRoleForCapability(CapabilityBoundedRevision, RoleContinuityReviewer) {
		t.Fatal("bounded revision must reject reviewer slot")
	}
}

func TestTaskEnvelopeRejectsInvalidFields(t *testing.T) {
	envelope := validEnvelope()
	if err := envelope.Validate(); err != nil {
		t.Fatalf("valid envelope: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*TaskEnvelope)
	}{
		{"attempt", func(value *TaskEnvelope) { value.Attempt = 0 }},
		{"hash", func(value *TaskEnvelope) { value.InputDigest = strings.Repeat("A", 64) }},
		{"role", func(value *TaskEnvelope) { value.Execution.RoleSlot = RoleWriter }},
		{"budget", func(value *TaskEnvelope) { value.Budget.RepairReserve = value.Budget.MaxRunTokens }},
		{"deadline", func(value *TaskEnvelope) { value.Deadline = time.Time{} }},
		{"project canonical", func(value *TaskEnvelope) { value.ProjectID = "project-1" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			invalid := envelope
			test.mutate(&invalid)
			if err := invalid.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestReviewFindingRequiresTextRangeOrConstraint(t *testing.T) {
	start, end := 3, 8
	finding := ReviewFinding{ID: "finding-1", Category: FindingStyle, Severity: SeverityMinor, Status: FindingOpen,
		Source: FindingModelReview, Message: "句式重复", Quote: "他看着", ScopeStart: &start, ScopeEnd: &end}
	if err := finding.Validate(); err != nil {
		t.Fatalf("valid finding: %v", err)
	}
	finding.ScopeEnd = nil
	if !errors.Is(finding.Validate(), ErrInvalidFinding) {
		t.Fatal("expected invalid range")
	}
	finding.ScopeStart, finding.Quote, finding.ConstraintRef = nil, "", "constraint:voice"
	if err := finding.Validate(); err != nil {
		t.Fatalf("constraint finding: %v", err)
	}
}

func TestDecodeTaskOutputIsStrict(t *testing.T) {
	valid := []byte(`{"title":"第一章","body":"雨落在门槛上。","completion_note":"完成目标"}`)
	output, err := DecodeTaskOutput(TaskChapterDraft, valid)
	if err != nil {
		t.Fatalf("decode valid: %v", err)
	}
	if output.(*ChapterDraftOutput).Title != "第一章" {
		t.Fatal("unexpected output")
	}
	for _, invalid := range [][]byte{
		[]byte(`{"title":"第一章","body":"正文","completion_note":"完成","unknown":true}`),
		[]byte(`{"title":"第一章","body":"","completion_note":"完成"}`),
		[]byte(`{"title":"第一章","body":"正文","completion_note":"完成"}{}`),
	} {
		if _, err := DecodeTaskOutput(TaskChapterDraft, invalid); !errors.Is(err, ErrInvalidTaskOutput) {
			t.Fatalf("expected strict error, got %v", err)
		}
	}
}

func TestDecodeTaskOutputAcceptsEveryTaskContract(t *testing.T) {
	fixtures := map[TaskKind]string{
		TaskStoryBlueprint:  `{"title":"雾港","positioning":"悬疑成长","core_conflict":"真相与亲情冲突","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`,
		TaskChapterPlan:     `{"chapter_goal":"找到船票","opening_hook":"空船靠岸","ending_hook":"船票写着明日死者姓名","scenes":[{"id":"s1","goal":"登船","conflict":"守卫阻拦","outcome":"偷渡成功"}]}`,
		TaskChapterDraft:    `{"title":"空船","body":"潮声里没有脚步。","completion_note":"完成登船目标"}`,
		TaskBaselineDraft:   `{"title":"空船","body":"潮声里没有脚步。","completion_note":"完成登船目标"}`,
		TaskReviewBundle:    `{"quality_status":"passed","summary":"满足约束","findings":[]}`,
		TaskReviseDraft:     `{"title":"空船","body":"潮声里仍没有脚步。","resolutions":[{"finding_id":"f1","status":"resolved","note":"删除重复句"}],"changes":[{"finding_id":"f1","original_text":"潮声里没有脚步。","revised_text":"潮声里仍没有脚步。"}]}`,
		TaskReviseBlueprint: `{"blueprint":{"title":"雾港","positioning":"悬疑成长","core_conflict":"真相与亲情冲突","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]},"resolutions":[{"finding_id":"f1","status":"resolved","note":"补全规则"}],"changes":[{"finding_id":"f1","original_text":"潮汐影响航路","revised_text":"潮汐决定航路"}]}`,
	}
	for kind, fixture := range fixtures {
		if _, err := DecodeTaskOutput(kind, []byte(fixture)); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
}

func TestRevisionOutputRejectsUntraceableChanges(t *testing.T) {
	fixture := []byte(`{"title":"空船","body":"修订正文","resolutions":[{"finding_id":"f1","status":"resolved","note":"已修订"}],"changes":[{"finding_id":"f2","original_text":"旧句","revised_text":"新句"}]}`)
	if _, err := DecodeTaskOutput(TaskReviseDraft, fixture); !errors.Is(err, ErrInvalidTaskOutput) {
		t.Fatalf("expected untraceable change error, got %v", err)
	}
}

func TestAgentTaskResultAllowsFailureWithoutOutputArtifact(t *testing.T) {
	now := time.Now().UTC()
	result := TaskResult{TaskID: "task", RunID: "run", StepID: "step", Attempt: 1, Status: StepFailed,
		QualityStatus: QualityBlocked, StartedAt: now, CompletedAt: now.Add(time.Second), ErrorCode: "provider_timeout"}
	if err := result.Validate(); err != nil {
		t.Fatalf("valid failed result: %v", err)
	}
	result.Status = StepCompleted
	if !errors.Is(result.Validate(), ErrInvalidTaskProtocol) {
		t.Fatal("completed result must freeze artifact and digest")
	}
}

func TestTypedOutputsRejectLimitsAndDuplicateIDs(t *testing.T) {
	plan := ChapterPlanOutput{ChapterGoal: "推进", OpeningHook: "开场", EndingHook: "结尾", Scenes: make([]SceneBeat, MaxChapterScenes+1)}
	if !errors.Is(plan.Validate(), ErrInvalidTaskOutput) {
		t.Fatal("expected scene limit error")
	}
	finding := ReviewFinding{ID: "f1", Category: FindingStyle, Severity: SeverityMinor, Status: FindingOpen,
		Source: FindingModelReview, Message: "重复", ConstraintRef: "style:no-repeat"}
	review := ReviewBundleOutput{QualityStatus: QualityNeedsReview, Summary: "需要修订", Findings: []ReviewFinding{finding, finding}}
	if !errors.Is(review.Validate(), ErrInvalidTaskOutput) {
		t.Fatal("expected duplicate finding error")
	}
}

func validEnvelope() TaskEnvelope {
	hash := SHA256Hex([]byte("fixture"))
	binding := VersionBinding{ID: "resource", Version: 1, Hash: hash}
	return TaskEnvelope{
		TaskID: "task-1", RunID: "run-1", StepID: "step-1", Attempt: 1, TaskKind: TaskStoryBlueprint,
		Capability: CapabilityStoryArchitect, Workflow: binding, InputDigest: hash, Prompt: binding, Schema: binding, Assembler: binding,
		Execution: ExecutionBinding{RoleSlot: RolePlanner, ProviderConnectionID: "provider-1", ProviderName: "OpenAI", ProviderKind: ProviderOpenAI,
			BaseURL: "https://api.openai.com/v1", CredentialRef: "secret://provider-1", Model: "model", MaxOutputTokens: 2000, Temperature: 0.7},
		InputRefs: []string{"input:seed"}, ContextManifestID: "manifest-1",
		Budget:   TaskBudget{MaxRunTokens: 10000, RemainingTokens: 10000, RepairReserve: 1000, RevisionReserve: 2000, MaxContextChars: 50000},
		Deadline: time.Now().Add(time.Minute),
	}
}
