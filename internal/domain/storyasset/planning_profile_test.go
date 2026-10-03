package storyasset

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPlanningEditPreservesGraphAndPlanningEvidence(t *testing.T) {
	payload := `{"parentId":"volume","level":"arc","title":"潮汐弧","summary":"寻找","goal":"守城","conflict":"选择","outcome":"线索","position":1,"planning":{"reader_expectation":"破局","stage_payoff":"守城成功","irreversible_change":"失去退路","ending_contribution":"找到终局线索","commitments":[{"id":"promise","statement":"解谜","fulfillment_range":"本卷","related_refs":["character:hero"]}],"retained_questions":[{"id":"q","statement":"信是谁寄的","reason":"保留悬念"}],"reference_bindings":{"character:hero":{"table":"characters","id":"actual-hero"}},"schema_version":1},"extension":{"keep":true,"integer":9007199254740993}}`
	fields, err := PlanningFields("outline_node", payload)
	if err != nil {
		t.Fatal(err)
	}
	edits := map[string]string{}
	for _, f := range fields {
		edits[f.Path] = f.Value
	}
	edits["/planning/commitments/0/statement"] = "兑现承诺"
	text, _ := json.Marshal(edits)
	next, _, err := EditPlanning("outline_node", payload, string(text))
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{`"parentId":"volume"`, `"id":"promise"`, `"actual-hero"`, `"position":1`, `"keep":true`, `"integer":9007199254740993`, `"related_refs":["character:hero"]`} {
		if !strings.Contains(next, keep) {
			t.Fatalf("lost evidence %s: %s", keep, next)
		}
	}
	if !strings.Contains(next, "兑现承诺") {
		t.Fatal("no edit")
	}
	for _, mode := range []string{"identity", "remove", "blank", "type", "null"} {
		t.Run(mode, func(t *testing.T) {
			var bad map[string]any
			_ = json.Unmarshal(text, &bad)
			switch mode {
			case "identity":
				bad["/parentId"] = "another"
			case "remove":
				delete(bad, "/goal")
			case "blank":
				bad["/title"] = " "
			case "null":
				bad["/summary"] = nil
			case "type":
				bad["/title"] = true
			}
			encoded, _ := json.Marshal(bad)
			if _, _, err := EditPlanning("outline_node", payload, string(encoded)); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
}

func TestProposalTextCannotRewriteRulesOrAnchorScope(t *testing.T) {
	payload := `{"title":"潮汐","positioning":"奇幻","core_conflict":"守城","protagonist_arc":"成长","sell_point":"代价","genre":"奇幻","growth_cost":"失去","pacing":"渐进","ending":"归来","major_milestones":["登岛","破局","归来"],"subplots":["旧友"],"world_rules":["不能复活"],"forbidden":["禁写"],"anchors":[{"strength":"hard","statement":"保卫","scope_hint":"proposal"}]}`
	fields, err := PlanningFields("proposal", payload)
	if err != nil {
		t.Fatal(err)
	}
	edits := map[string]string{}
	for _, f := range fields {
		edits[f.Path] = f.Value
	}
	edits["/ending"] = "带着代价归来"
	text, _ := json.Marshal(edits)
	next, _, err := EditPlanning("proposal", payload, string(text))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(next, `"world_rules":["不能复活"]`) || !strings.Contains(next, `"scope_hint":"proposal"`) {
		t.Fatal("changed constraints")
	}
	edits["/world_rules/0"] = "可以复活"
	text, _ = json.Marshal(edits)
	if _, _, err = EditPlanning("proposal", payload, string(text)); err == nil {
		t.Fatal("rule bypass")
	}
}
