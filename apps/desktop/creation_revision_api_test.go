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
	confirm, apply creationapp.RevisionCommand
	rejected       string
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
func TestDesktopRevisionPreservesAuthorizationAndText(t *testing.T) {
	w := &revisionWorkspace{}
	api := newDesktopAPI(w, &fakeDirectoryPicker{})
	api.startup(context.Background())
	request := creationapp.RevisionRequest{CandidateID: "c", Paths: []string{"/title"}, Instruction: " 原文😀\n", IdempotencyKey: "k"}
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
	if !reflect.DeepEqual(w.preview, request) || w.confirm != command || w.apply != command || w.rejected != "a" {
		t.Fatal("绑定改变了输入或授权参数")
	}
}
