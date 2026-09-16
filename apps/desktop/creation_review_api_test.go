package main

import (
	"context"
	"reflect"
	"testing"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
)

type reviewWorkspace struct {
	fakeWorkspace
	request      creationapp.ReviewRequest
	acceptAll    string
	usedVersion  string
	compareStage string
	compareBase  string
	cursor       creationapp.ReviewCursorRequest
}

func (w *reviewWorkspace) SetCreationReview(_ context.Context, request creationapp.ReviewRequest) (creationapp.WorkbenchView, error) {
	w.request = request
	return creationapp.WorkbenchView{SessionID: "session-1"}, nil
}

func (w *reviewWorkspace) AcceptAllCreationReview(_ context.Context, candidateID string) (creationapp.WorkbenchView, error) {
	w.acceptAll = candidateID
	return creationapp.WorkbenchView{SessionID: "session-1"}, nil
}

func (w *reviewWorkspace) UseCreationCandidateVersion(_ context.Context, candidateID string) (creationapp.WorkbenchView, error) {
	w.usedVersion = candidateID
	return creationapp.WorkbenchView{SessionID: "session-1"}, nil
}

func (w *reviewWorkspace) CompareCreationCandidateVersions(_ context.Context, stage, baseCandidateID string) (creationapp.CandidateCompareView, error) {
	w.compareStage, w.compareBase = stage, baseCandidateID
	return creationapp.CandidateCompareView{Stage: stage, BaseCandidateID: baseCandidateID}, nil
}

func (w *reviewWorkspace) SaveCreationReviewCursor(_ context.Context, request creationapp.ReviewCursorRequest) (creationapp.WorkbenchView, error) {
	w.cursor = request
	return creationapp.WorkbenchView{SessionID: "session-1"}, nil
}

// 绑定层只转发作者的确切决定与引用，不解释或改写审阅语义。
func TestDesktopReviewForwardsExactDecision(t *testing.T) {
	workspace := &reviewWorkspace{}
	api := newDesktopAPI(workspace, &fakeDirectoryPicker{})
	api.startup(context.Background())

	request := creationapp.ReviewRequest{CandidateID: "candidate-1", Scope: "entity", RefID: "character:char-1", Decision: "reject"}
	if _, err := api.SetCreationReview(request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(workspace.request, request) {
		t.Fatalf("request = %#v", workspace.request)
	}
	if _, err := api.AcceptAllCreationReview("candidate-1"); err != nil {
		t.Fatal(err)
	}
	if workspace.acceptAll != "candidate-1" {
		t.Fatalf("accept all = %q", workspace.acceptAll)
	}
	if _, err := api.UseCreationCandidateVersion("candidate-old"); err != nil {
		t.Fatal(err)
	}
	if workspace.usedVersion != "candidate-old" {
		t.Fatalf("used version = %q", workspace.usedVersion)
	}
	comparison, err := api.CompareCreationCandidateVersions("people_world", "candidate-1")
	if err != nil {
		t.Fatal(err)
	}
	if workspace.compareStage != "people_world" || workspace.compareBase != "candidate-1" || comparison.BaseCandidateID != "candidate-1" {
		t.Fatalf("comparison = %#v, args = %q/%q", comparison, workspace.compareStage, workspace.compareBase)
	}
	cursor := creationapp.ReviewCursorRequest{Stage: "people_world", CandidateID: "candidate-1", EntityID: "rule:潮汐律"}
	if _, err := api.SaveCreationReviewCursor(cursor); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(workspace.cursor, cursor) {
		t.Fatalf("cursor = %#v", workspace.cursor)
	}
}

// 未接通时返回可识别的未就绪错误，而不是伪造成功。
func TestDesktopReviewUnavailableWithoutWorkspace(t *testing.T) {
	api := newDesktopAPI(&fakeWorkspace{}, &fakeDirectoryPicker{})
	api.startup(context.Background())
	if _, err := api.SetCreationReview(creationapp.ReviewRequest{CandidateID: "candidate-1"}); err == nil {
		t.Fatal("review command must fail when the workspace cannot serve it")
	}
}
