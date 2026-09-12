package main

import (
	"context"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"reflect"
	"testing"
)

type entryWorkspace struct {
	fakeWorkspace
	saved creationapp.SaveEntryRequest
	used  creationapp.UseMaterialRequest
}

func (w *entryWorkspace) SaveCreationEntry(_ context.Context, request creationapp.SaveEntryRequest) (creationapp.WorkbenchView, error) {
	w.saved = request
	return creationapp.WorkbenchView{SessionID: request.SessionID}, nil
}
func (w *entryWorkspace) UseCreationMaterial(_ context.Context, request creationapp.UseMaterialRequest) (creationapp.WorkbenchView, error) {
	w.used = request
	return creationapp.WorkbenchView{SessionID: request.SessionID}, nil
}

func TestDesktopEntryForwardsExactTextAndVersions(t *testing.T) {
	workspace := &entryWorkspace{}
	api := newDesktopAPI(workspace, &fakeDirectoryPicker{})
	api.startup(context.Background())
	request := creationapp.SaveEntryRequest{ProjectID: "p", SessionID: "s", ExpectedVersion: 7, ExpectedRunSlotVersion: 2, Kind: creationdomain.EntryAuthorPlan, Material: "  原文😀\n  "}
	if _, err := api.SaveCreationEntry(request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(workspace.saved, request) {
		t.Fatal("绑定改变了原文或版本")
	}
	use := creationapp.UseMaterialRequest{ProjectID: "p", SessionID: "s", ExpectedVersion: 8, AnalysisID: "a", Stage: creationdomain.StageProposal}
	if _, err := api.UseCreationMaterial(use); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(workspace.used, use) {
		t.Fatal("绑定改变来源或阶段")
	}
}
