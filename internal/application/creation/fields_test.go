package creation

import (
	"context"
	"errors"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"strings"
	"testing"
)

func TestTextFieldEditsAcrossFourCandidateKinds(t *testing.T) {
	for _, test := range []struct {
		stage   creationdomain.Stage
		payload string
	}{{creationdomain.StageDirections, directionPayload}, {creationdomain.StageProposal, proposalPayload}, {creationdomain.StagePeopleWorld, peopleWorldPayload}, {creationdomain.StageRoadmap, roadmapPayload}} {
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
