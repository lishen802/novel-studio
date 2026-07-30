package workspace

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestBuildContextManifestIsDeterministicAndTracksTrimming(t *testing.T) {
	input := contextManifestInput{
		Header: map[string]any{
			"project":       map[string]any{"id": "project-1", "title": "凡人修仙"},
			"targetChapter": map[string]any{"id": "chapter-1", "title": "第一章"},
		},
		Domains: []contextDomainInput{
			{Name: "confirmedFacts", Required: true, Priority: 0, Items: rawStrings("事实一")},
			{Name: "characters", Priority: 1, Items: rawStrings("主角", "师父")},
			{Name: "locations", Priority: 4, Items: rawStrings(
				"地点一很长很长很长很长很长",
				"地点二很长很长很长很长很长",
				"地点三很长很长很长很长很长",
			)},
		},
	}
	full, fullStats, err := buildContextManifest(input, 10000)
	if err != nil {
		t.Fatal(err)
	}
	budget := len([]rune(full)) - 20
	first, stats, err := buildContextManifest(input, budget)
	if err != nil {
		t.Fatal(err)
	}
	second, secondStats, err := buildContextManifest(input, budget)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || stats != secondStats {
		t.Fatal("same input and budget must produce byte-identical manifest and stats")
	}
	if !stats.Truncated || stats.CharCount > budget {
		t.Fatalf("unexpected stats: %#v, full = %#v", stats, fullStats)
	}
	var decoded struct {
		Meta struct {
			Domains map[string]struct {
				Total    int `json:"total"`
				Included int `json:"included"`
				Omitted  int `json:"omitted"`
			} `json:"domains"`
		} `json:"meta"`
		Domains map[string][]json.RawMessage `json:"domains"`
	}
	if err := json.Unmarshal([]byte(first), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Meta.Domains["confirmedFacts"].Included != 1 ||
		decoded.Meta.Domains["locations"].Omitted == 0 {
		t.Fatalf("unexpected domain stats: %#v", decoded.Meta.Domains)
	}
}

func TestBuildContextManifestRejectsRequiredContentOverBudget(t *testing.T) {
	_, _, err := buildContextManifest(contextManifestInput{
		Header: map[string]any{"project": "必须保留的项目数据"},
		Domains: []contextDomainInput{{
			Name: "confirmedFacts", Required: true, Items: rawStrings(
				"不可删除的确认事实一",
				"不可删除的确认事实二",
			),
		}},
	}, 20)
	if !errors.Is(err, ErrContextBudgetExceeded) {
		t.Fatalf("expected ErrContextBudgetExceeded, got %v", err)
	}
}

func rawStrings(values ...string) []json.RawMessage {
	result := make([]json.RawMessage, 0, len(values))
	for _, value := range values {
		encoded, _ := json.Marshal(value)
		result = append(result, encoded)
	}
	return result
}
