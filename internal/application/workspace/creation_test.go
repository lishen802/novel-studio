package workspace

import (
	"context"
	"encoding/json"
	"testing"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestCreationWorkbenchReturnsIntentRailWithoutSampleDirections(t *testing.T) {
	ctx := context.Background()
	service := newWorkbenchTestService(t)
	t.Cleanup(func() { _ = service.Close() })
	if _, err := service.CreateProject(ctx, t.TempDir(), "灯塔", "悬疑", ""); err != nil {
		t.Fatal(err)
	}
	view, err := service.CreationWorkbench(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view.Stage != creationdomain.StageIntent {
		t.Fatalf("stage = %s", view.Stage)
	}
	if len(view.StageRail) != 6 {
		t.Fatalf("rail = %#v", view.StageRail)
	}
	if len(view.DirectionSet.Cards) != 0 {
		t.Fatalf("unconfigured project must not show sample directions: %#v", view.DirectionSet)
	}
	saved, err := service.SaveCreationIntent(ctx, "守关人必须放走仇敌")
	if err != nil {
		t.Fatal(err)
	}
	if saved.IntentText != "守关人必须放走仇敌" {
		t.Fatalf("intent = %q", saved.IntentText)
	}
}

func TestRecoverCreationIntentJSONKeepsEmptyFieldSlices(t *testing.T) {
	ctx := context.Background()
	service := newWorkbenchTestService(t)
	t.Cleanup(func() { _ = service.Close() })
	if _, err := service.CreateProject(ctx, t.TempDir(), "灯塔", "悬疑", ""); err != nil {
		t.Fatal(err)
	}
	view, err := service.RecoverCreation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(view)
	if err != nil {
		t.Fatal(err)
	}
	var raw struct {
		FieldEditor struct {
			Fields []any `json:"fields"`
		} `json:"fieldEditor"`
		Impact struct {
			Items []any `json:"items"`
		} `json:"impact"`
		Assistant struct {
			Messages []any `json:"messages"`
		} `json:"assistant"`
	}
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatal(err)
	}
	if raw.FieldEditor.Fields == nil {
		t.Fatal("fieldEditor.fields 序列化成 null 会让驾驶舱白屏")
	}
	if raw.Impact.Items == nil {
		t.Fatal("impact.items 序列化成 null 会让驾驶舱白屏")
	}
	if raw.Assistant.Messages == nil {
		t.Fatal("assistant.messages 序列化成 null")
	}
}
