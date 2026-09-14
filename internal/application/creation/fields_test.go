package creation

import (
	"context"
	"errors"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"strings"
	"testing"
)

func TestRoadmapTextFieldsExposePlanningGoals(t *testing.T) {
	binding, err := bindingFor(creationdomain.StageRoadmap)
	if err != nil || binding.SchemaVersion != 3 {
		t.Fatalf("路线图绑定未切到 v3: %+v, %v", binding, err)
	}
	candidate, err := creationdomain.NewCandidate(
		"roadmap-fields", "session-1", creationdomain.StageRoadmap, binding.Kind, 1, "", "", "",
		roadmapPayloadV3, binding.SchemaID, binding.SchemaVersion, testClock{}.Now(),
	)
	if err != nil {
		t.Fatal(err)
	}
	fields := candidateTextFields(candidate)
	found := map[string]string{}
	for _, field := range fields.Fields {
		if strings.HasSuffix(field.Path, "/reader_expectation") || strings.HasSuffix(field.Path, "/stage_payoff") ||
			strings.HasSuffix(field.Path, "/irreversible_change") || strings.HasSuffix(field.Path, "/ending_contribution") ||
			strings.HasSuffix(field.Path, "/statement") || strings.HasSuffix(field.Path, "/fulfillment_range") {
			found[field.Path] = field.Label
		}
	}
	if found["/arcs/@arc-1/reader_expectation"] != "读者期待" || found["/arcs/@arc-1/stage_payoff"] != "阶段回报" ||
		found["/arcs/@arc-1/commitments/@cmt-1/statement"] == "" || found["/arcs/@arc-1/retained_questions/@q-1/statement"] == "" {
		t.Fatalf("规划目标未进入字段编辑器: %+v", found)
	}
}

func TestTextFieldEditsAcrossFourCandidateKinds(t *testing.T) {
	for _, test := range []struct {
		stage   creationdomain.Stage
		payload string
	}{{creationdomain.StageDirections, directionPayload}, {creationdomain.StageProposal, proposalPayload}, {creationdomain.StagePeopleWorld, peopleWorldPayload}, {creationdomain.StageRoadmap, roadmapPayloadV3}} {
		t.Run(string(test.stage), func(t *testing.T) {
			ctx := context.Background()
			service, store, _ := newTestService(t, newFakeKernel(), nil)
			s := seedIntentSession(t, service)
			binding, _ := bindingFor(test.stage)
			c, err := creationdomain.NewCandidate("field-base", s.ID, test.stage, binding.Kind, 1, "", "", "", test.payload, binding.SchemaID, binding.SchemaVersion, testClock{}.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SaveCandidate(ctx, c); err != nil {
				t.Fatal(err)
			}
			fields := candidateTextFields(c)
			var chosen TextFieldView
			for _, field := range fields.Fields {
				if strings.HasSuffix(field.Path, "/title") || strings.HasSuffix(field.Path, "/hook") || strings.HasSuffix(field.Path, "/name") {
					chosen = field
					break
				}
			}
			if chosen.Path == "" {
				t.Fatal("缺少可编辑文本")
			}
			locked, err := service.LockCandidateField(ctx, s.ProjectID, c.ID, chosen.Path)
			if err != nil {
				t.Fatal(err)
			}
			req := SaveFieldsRequest{CandidateID: c.ID, Changes: map[string]string{chosen.Path: chosen.Value + "（修订）"}}
			if _, err := service.SaveFields(ctx, s.ProjectID, req); !errors.Is(err, creationdomain.ErrFieldLocked) {
				t.Fatalf("绕过锁: %v", err)
			}
			if _, err := service.UnlockCandidateField(ctx, s.ProjectID, locked.ID, chosen.Path); err != nil {
				t.Fatal(err)
			}
			edited, err := service.SaveFields(ctx, s.ProjectID, req)
			if err != nil {
				t.Fatal(err)
			}
			if edited.ID == c.ID || edited.Origin != creationdomain.OriginAuthor || edited.Status == creationdomain.StatusConfirmed {
				t.Fatal("手工编辑应形成未接受的新候选")
			}
			if _, err := service.LockCandidateField(ctx, s.ProjectID, c.ID, chosen.Path); !errors.Is(err, creationdomain.ErrCandidateVersionConflict) {
				t.Fatalf("不能改历史版本锁: %v", err)
			}
			if _, err := service.SaveFields(ctx, s.ProjectID, req); !errors.Is(err, creationdomain.ErrCandidateVersionConflict) {
				t.Fatalf("不能覆盖新版: %v", err)
			}
		})
	}
}
