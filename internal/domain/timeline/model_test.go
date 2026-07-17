package timeline

import (
	"errors"
	"testing"
	"time"
)

func TestStoryEventKeepsStoryAndNarrativeOrderSeparate(t *testing.T) {
	now := time.Unix(100, 0)
	event, err := NewStoryEvent("event-1", "project-1", "收到匿名信", "雾历 312 年七月初十", 120, 3, EventOccurred, now)
	if err != nil {
		t.Fatal(err)
	}
	if event.StoryOrder != 120 || event.NarrativeOrder != 3 || event.StoryTime != "雾历 312 年七月初十" {
		t.Fatalf("event = %#v", event)
	}
	updated, err := event.Update("收到匿名信", "主线启动", "神秘人寄信", "沈砚重启调查", "雾历 312 年七月初十深夜", 121, 1, EventUncertain, now.Add(time.Minute))
	if err != nil || updated.Status != EventUncertain || updated.NarrativeOrder != 1 {
		t.Fatalf("updated = %#v, error = %v", updated, err)
	}
}

func TestStoryEventRejectsInvalidIdentityOrderAndStatus(t *testing.T) {
	now := time.Unix(100, 0)
	tests := []struct {
		name           string
		id             string
		projectID      string
		title          string
		storyOrder     int64
		narrativeOrder int
		status         EventStatus
		want           error
	}{
		{name: "blank", projectID: "project-1", title: "事件", narrativeOrder: 1, status: EventPlanned, want: ErrBlankValue},
		{name: "negative story order", id: "event-1", projectID: "project-1", title: "事件", storyOrder: -1, narrativeOrder: 1, status: EventPlanned, want: ErrInvalidOrder},
		{name: "zero narrative order", id: "event-1", projectID: "project-1", title: "事件", narrativeOrder: 0, status: EventPlanned, want: ErrInvalidOrder},
		{name: "bad status", id: "event-1", projectID: "project-1", title: "事件", narrativeOrder: 1, status: EventStatus("bad"), want: ErrInvalidStatus},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewStoryEvent(test.id, test.projectID, test.title, "", test.storyOrder, test.narrativeOrder, test.status, now)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestEventLinkValidatesTarget(t *testing.T) {
	event, _ := NewStoryEvent("event-1", "project-1", "收到匿名信", "", 1, 1, EventOccurred, time.Unix(100, 0))
	link, err := NewEventLink(event, TargetCharacter, "character-1", "参与者", time.Unix(101, 0))
	if err != nil || link.ProjectID != event.ProjectID || link.TargetID != "character-1" {
		t.Fatalf("link = %#v, error = %v", link, err)
	}
	if _, err := NewEventLink(event, EventTargetType("bad"), "target-1", "", time.Now()); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("invalid target error = %v", err)
	}
}
