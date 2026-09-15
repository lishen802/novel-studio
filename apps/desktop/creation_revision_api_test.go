package main

import (
	"context"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"reflect"
	"testing"
)

type revisionWorkspace struct {
	fakeWorkspace
	preview        creationapp.RevisionRequest
	message        creationapp.SendMessageRequest
	confirm, apply creationapp.RevisionCommand
	rejected       string
	discussPreview creationapp.DiscussionAuthorizationRequest
	discussConfirm creationapp.DiscussionAuthorizationCommand
	discussMessage creationapp.DiscussionMessageRequest
	discussRevoke  string
	suggest        creationapp.DiscussionSuggestionRequest
}

func (w *revisionWorkspace) SendCreationMessage(_ context.Context, r creationapp.SendMessageRequest) (creationapp.RevisionView, error) {
	w.message = r
	return creationapp.RevisionView{ActionID: "message-action"}, nil
}

func (w *revisionWorkspace) PreviewCreationRevision(_ context.Context, r creationapp.RevisionRequest) (creationapp.RevisionView, error) {
	w.preview = r
	return creationapp.RevisionView{ActionID: "a"}, nil
}
func (w *revisionWorkspace) ConfirmCreationRevision(_ context.Context, r creationapp.RevisionCommand) (creationapp.RevisionView, error) {
	w.confirm = r
	return creationapp.RevisionView{}, nil
}
func (w *revisionWorkspace) ApplyCreationRevision(_ context.Context, r creationapp.RevisionCommand) (creationapp.WorkbenchView, error) {
	w.apply = r
	return creationapp.WorkbenchView{}, nil
}
func (w *revisionWorkspace) RejectCreationRevision(_ context.Context, id string) (creationapp.WorkbenchView, error) {
	w.rejected = id
	return creationapp.WorkbenchView{}, nil
}
func (w *revisionWorkspace) PreviewCreationDiscussionAuthorization(_ context.Context, r creationapp.DiscussionAuthorizationRequest) (creationapp.DiscussionTurnView, error) {
	w.discussPreview = r
	return creationapp.DiscussionTurnView{ActionID: "discuss-action", ArgumentsHash: "hash"}, nil
}
func (w *revisionWorkspace) ConfirmCreationDiscussionAuthorization(_ context.Context, r creationapp.DiscussionAuthorizationCommand) (creationapp.DiscussionTurnView, error) {
	w.discussConfirm = r
	return creationapp.DiscussionTurnView{RunID: "run-1"}, nil
}
func (w *revisionWorkspace) SendCreationDiscussionMessage(_ context.Context, r creationapp.DiscussionMessageRequest) (creationapp.DiscussionTurnView, error) {
	w.discussMessage = r
	return creationapp.DiscussionTurnView{RunID: "run-2"}, nil
}
func (w *revisionWorkspace) RevokeCreationDiscussionAuthorization(_ context.Context, id string) (creationapp.WorkbenchView, error) {
	w.discussRevoke = id
	return creationapp.WorkbenchView{}, nil
}
func (w *revisionWorkspace) PreviewCreationDiscussionSuggestion(_ context.Context, r creationapp.DiscussionSuggestionRequest) (creationapp.RevisionView, error) {
	w.suggest = r
	return creationapp.RevisionView{ActionID: "suggest-action"}, nil
}
func TestDesktopRevisionPreservesAuthorizationAndText(t *testing.T) {
	w := &revisionWorkspace{}
	api := newDesktopAPI(w, &fakeDirectoryPicker{})
	api.startup(context.Background())
	request := creationapp.RevisionRequest{CandidateID: "c", Paths: []string{"/title"}, Instruction: " 原文😀\n", IdempotencyKey: "k"}
	message := creationapp.SendMessageRequest{CandidateID: "c", EntityRef: "proposal:main", FieldPath: "/title", Text: "讨论😀\n", IdempotencyKey: "message-key"}
	if _, err := api.SendCreationMessage(message); err != nil {
		t.Fatal(err)
	}
	if _, err := api.PreviewCreationRevision(request); err != nil {
		t.Fatal(err)
	}
	command := creationapp.RevisionCommand{ActionID: "a", ArgumentsHash: "hash", IdempotencyKey: "k"}
	if _, err := api.ConfirmCreationRevision(command); err != nil {
		t.Fatal(err)
	}
	if _, err := api.ApplyCreationRevision(command); err != nil {
		t.Fatal(err)
	}
	if _, err := api.RejectCreationRevision("a"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w.preview, request) || w.message != message || w.confirm != command || w.apply != command || w.rejected != "a" {
		t.Fatal("绑定改变了输入或授权参数")
	}
}

func TestDesktopDiscussionPreservesUnicodeTargetAndIdempotency(t *testing.T) {
	w := &revisionWorkspace{}
	api := newDesktopAPI(w, &fakeDirectoryPicker{})
	api.startup(context.Background())
	preview := creationapp.DiscussionAuthorizationRequest{CandidateID: "c", EntityRef: "character:阿绫", FieldPaths: []string{"/motivation"}, IdempotencyKey: "auth-key"}
	if _, err := api.PreviewCreationDiscussionAuthorization(preview); err != nil {
		t.Fatal(err)
	}
	confirm := creationapp.DiscussionAuthorizationCommand{ActionID: "discuss-action", ArgumentsHash: "hash", IdempotencyKey: "auth-key", Message: "动机😀\n哪里不够可信？", MessageIdempotencyKey: "msg-key"}
	if _, err := api.ConfirmCreationDiscussionAuthorization(confirm); err != nil {
		t.Fatal(err)
	}
	message := creationapp.DiscussionMessageRequest{AuthorizationID: "auth-1", CandidateID: "c", EntityRef: "character:阿绫", FieldPaths: []string{"/motivation"}, Text: "继续比较\n风险", IdempotencyKey: "msg-2"}
	if _, err := api.SendCreationDiscussionMessage(message); err != nil {
		t.Fatal(err)
	}
	if _, err := api.RevokeCreationDiscussionAuthorization("auth-1"); err != nil {
		t.Fatal(err)
	}
	suggest := creationapp.DiscussionSuggestionRequest{MessageID: "m1", SuggestionID: "s1", IdempotencyKey: "discussion:run-1:suggestion:s1"}
	if _, err := api.PreviewCreationDiscussionSuggestion(suggest); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(w.discussPreview, preview) || w.discussConfirm != confirm || !reflect.DeepEqual(w.discussMessage, message) || w.discussRevoke != "auth-1" || w.suggest != suggest {
		t.Fatal("讨论绑定改变了 Unicode、换行、目标或幂等键")
	}
}
