package main

import (
	"context"
	"reflect"
	"testing"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
)

type structureWorkspace struct {
	fakeWorkspace
	request creationapp.StructureEditRequest
}

func (w *structureWorkspace) SaveCreationStructure(_ context.Context, request creationapp.StructureEditRequest) (creationapp.WorkbenchView, error) {
	w.request = request
	return creationapp.WorkbenchView{SessionID: "session-1"}, nil
}

// 绑定层原样转发作者的结构增删参数，不解释集合或实体标识。
func TestDesktopStructureForwardsExactEdit(t *testing.T) {
	workspace := &structureWorkspace{}
	api := newDesktopAPI(workspace, &fakeDirectoryPicker{})
	api.startup(context.Background())
	request := creationapp.StructureEditRequest{
		CandidateID: "candidate-1", Collection: "characters", Operation: "add", EntityKey: "char-2",
		Value: `{"id":"char-2","name":"老周","role":"supporting","summary":"副将","motivation":"护住守军","arc":"学会取舍"}`,
	}
	if _, err := api.SaveCreationStructure(request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(workspace.request, request) {
		t.Fatalf("request = %#v", workspace.request)
	}
}

// 未接通时返回可识别的未就绪错误，而不是伪造成功。
func TestDesktopStructureUnavailableWithoutWorkspace(t *testing.T) {
	api := newDesktopAPI(&fakeWorkspace{}, &fakeDirectoryPicker{})
	api.startup(context.Background())
	if _, err := api.SaveCreationStructure(creationapp.StructureEditRequest{CandidateID: "candidate-1"}); err == nil {
		t.Fatal("structure command must fail when the workspace cannot serve it")
	}
}
