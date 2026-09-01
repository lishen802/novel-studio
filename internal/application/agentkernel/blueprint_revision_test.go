package agentkernel

import (
	"testing"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func TestValidateBlueprintRevisionRestoresUnreviewedFieldChange(t *testing.T) {
	original := `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`
	review := &agentdomain.ReviewBundleOutput{QualityStatus: agentdomain.QualityNeedsReview, Summary: "冲突不足", Findings: []agentdomain.ReviewFinding{{
		ID: "f1", Category: agentdomain.FindingCausality, Severity: agentdomain.SeverityMajor,
		Status: agentdomain.FindingOpen, Source: agentdomain.FindingModelReview, Message: "补足代价", ConstraintRef: "blueprint:core_conflict",
	}}}
	revision := &agentdomain.ReviseBlueprintOutput{
		Blueprint:   agentdomain.StoryBlueprintOutput{Title: "新标题", Positioning: "悬疑成长", CoreConflict: "追查失踪与亲情冲突", ProtagonistArc: "从逃避到承担", WorldRules: []string{"潮汐决定航路"}, MajorMilestones: []string{"失踪", "追查", "抉择"}},
		Resolutions: []agentdomain.FindingResolution{{FindingID: "f1", Status: agentdomain.FindingResolved, Note: "已补足"}},
		Changes:     []agentdomain.RevisionChange{{FindingID: "f1", OriginalText: "追查", RevisedText: "亲情冲突"}},
	}
	if err := validateBlueprintRevision(original, review, revision, nil); err != nil {
		t.Fatalf("validateBlueprintRevision: %v", err)
	}
	if revision.Blueprint.Title != "雾港" || revision.Blueprint.CoreConflict != "追查失踪与亲情冲突" {
		t.Fatalf("bounded blueprint = %#v", revision.Blueprint)
	}
}

func TestValidateBlueprintRevisionAllowsMultipleFindingsForSameField(t *testing.T) {
	original := `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`
	review := &agentdomain.ReviewBundleOutput{QualityStatus: agentdomain.QualityNeedsReview, Summary: "冲突不足", Findings: []agentdomain.ReviewFinding{
		{ID: "f1", Category: agentdomain.FindingCausality, Severity: agentdomain.SeverityMajor, Status: agentdomain.FindingOpen, Source: agentdomain.FindingModelReview, Message: "补足代价", ConstraintRef: "blueprint:core_conflict"},
		{ID: "f2", Category: agentdomain.FindingMotivation, Severity: agentdomain.SeverityMajor, Status: agentdomain.FindingOpen, Source: agentdomain.FindingModelReview, Message: "补足动机", Quote: "追查失踪"},
	}}
	revision := &agentdomain.ReviseBlueprintOutput{
		Blueprint:   agentdomain.StoryBlueprintOutput{Title: "雾港", Positioning: "悬疑成长", CoreConflict: "追查失踪与守护亲情冲突", ProtagonistArc: "从逃避到承担", WorldRules: []string{"潮汐决定航路"}, MajorMilestones: []string{"失踪", "追查", "抉择"}},
		Resolutions: []agentdomain.FindingResolution{{FindingID: "f1", Status: agentdomain.FindingResolved, Note: "补足代价"}, {FindingID: "f2", Status: agentdomain.FindingResolved, Note: "补足动机"}},
		Changes: []agentdomain.RevisionChange{
			{FindingID: "f1", OriginalText: "冲突太弱", RevisedText: "加入亲情"},
			{FindingID: "f2", OriginalText: "追查", RevisedText: "守护"},
		},
	}
	if err := validateBlueprintRevision(original, review, revision, nil); err != nil {
		t.Fatalf("validateBlueprintRevision: %v", err)
	}
	for _, change := range revision.Changes {
		if change.OriginalText != "追查失踪" || change.RevisedText != "追查失踪与守护亲情冲突" {
			t.Fatalf("normalized change = %#v", change)
		}
	}
}

func TestNormalizeBlueprintRevisionRepairsTraceAndSafeShapeDrift(t *testing.T) {
	original := `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`
	review := &agentdomain.ReviewBundleOutput{QualityStatus: agentdomain.QualityNeedsReview, Summary: "需要补强", Findings: []agentdomain.ReviewFinding{
		{ID: "f1", Category: agentdomain.FindingCausality, Severity: agentdomain.SeverityMajor, Status: agentdomain.FindingOpen, Source: agentdomain.FindingModelReview, Message: "补足代价", ConstraintRef: "blueprint:core_conflict"},
		{ID: "f2", Category: agentdomain.FindingPacing, Severity: agentdomain.SeverityMajor, Status: agentdomain.FindingOpen, Source: agentdomain.FindingModelReview, Message: "补足里程碑", ConstraintRef: "blueprint:major_milestones"},
	}}
	revision := &agentdomain.ReviseBlueprintOutput{
		Blueprint: agentdomain.StoryBlueprintOutput{
			Title: "", Positioning: "悬疑成长", CoreConflict: "追查失踪并付出亲情代价", ProtagonistArc: "从逃避到承担",
			WorldRules: []string{"潮汐决定航路", "潮汐决定航路", ""}, MajorMilestones: []string{"失踪", "追查", "抉择", "偿还代价"},
		},
		Resolutions: []agentdomain.FindingResolution{
			{FindingID: "renamed", Status: agentdomain.FindingResolved, Note: ""},
			{FindingID: "renamed", Status: agentdomain.FindingResolved, Note: "已补足"},
		},
		Changes: []agentdomain.RevisionChange{{FindingID: "renamed", OriginalText: "不精确", RevisedText: "也不精确"}},
	}
	if err := normalizeBlueprintRevision(review, original, revision); err != nil {
		t.Fatalf("normalizeBlueprintRevision: %v", err)
	}
	if err := revision.Validate(); err != nil {
		t.Fatalf("normalized revision invalid: %v", err)
	}
	if revision.Blueprint.Title != "雾港" || len(revision.Blueprint.WorldRules) != 1 {
		t.Fatalf("normalized blueprint = %#v", revision.Blueprint)
	}
	if len(revision.Resolutions) != 2 || revision.Resolutions[0].FindingID != "f1" || revision.Resolutions[0].Note == "" || revision.Resolutions[1].FindingID != "f2" {
		t.Fatalf("normalized resolutions = %#v", revision.Resolutions)
	}
	if len(revision.Changes) != 2 || revision.Changes[0].FindingID != "f1" || revision.Changes[1].FindingID != "f2" {
		t.Fatalf("normalized changes = %#v", revision.Changes)
	}
}

func TestNormalizeBlueprintRevisionMarksMissingChangeUnresolved(t *testing.T) {
	original := `{"title":"雾港","positioning":"悬疑成长","core_conflict":"追查失踪","protagonist_arc":"从逃避到承担","world_rules":["潮汐决定航路"],"major_milestones":["失踪","追查","抉择"]}`
	review := &agentdomain.ReviewBundleOutput{QualityStatus: agentdomain.QualityNeedsReview, Summary: "问题", Findings: []agentdomain.ReviewFinding{{
		ID: "f1", Category: agentdomain.FindingCausality, Severity: agentdomain.SeverityMajor, Status: agentdomain.FindingOpen,
		Source: agentdomain.FindingModelReview, Message: "无法映射的问题",
	}}}
	revision := &agentdomain.ReviseBlueprintOutput{
		Blueprint:   agentdomain.StoryBlueprintOutput{Title: "雾港", Positioning: "悬疑成长", CoreConflict: "追查失踪", ProtagonistArc: "从逃避到承担", WorldRules: []string{"潮汐决定航路"}, MajorMilestones: []string{"失踪", "追查", "抉择"}},
		Resolutions: []agentdomain.FindingResolution{{FindingID: "f1", Status: agentdomain.FindingResolved, Note: "声称完成"}},
	}
	if err := normalizeBlueprintRevision(review, original, revision); err != nil {
		t.Fatalf("normalizeBlueprintRevision: %v", err)
	}
	if revision.Resolutions[0].Status != agentdomain.FindingCannotResolve || len(revision.Changes) != 0 {
		t.Fatalf("normalized trace = %#v %#v", revision.Resolutions, revision.Changes)
	}
	if err := revision.Validate(); err != nil {
		t.Fatalf("normalized revision invalid: %v", err)
	}
}
