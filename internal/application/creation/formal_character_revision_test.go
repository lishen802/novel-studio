package creation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lishen802/novel-studio/internal/agentassets"
	agent "github.com/lishen802/novel-studio/internal/domain/agent"
	domain "github.com/lishen802/novel-studio/internal/domain/creation"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type formalTestStore struct {
	Store
	base asset.CharacterRevisionBase
}

func (s *formalTestStore) FormalCharacterRevisionBase(context.Context, string, string, string) (asset.CharacterRevisionBase, error) {
	return s.base, nil
}

func TestFormalCharacterRevisionUsesLatestHeadWithoutChangingCreationCandidates(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	service, store, _ := newTestService(t, kernel, nil)
	seedIntentSession(t, service)
	cast := `{"characters":[{"id":"hero","name":"最新姓名","role":"protagonist","aliases":[],"summary":"作者新版简介","motivation":"守护","fear":"","secret":"","arc":"成长"}],"relationships":[],"factions":[],"locations":[],"world_rules":[{"name":"规则","description":"不可复活","scope_type":"project"}],"forbidden":["禁写"],"omission_reasons":[]}`
	wrapper := &formalTestStore{Store: store, base: asset.CharacterRevisionBase{ProjectID: "project-1", BranchID: "branch-1", AssetID: "asset", TypedID: "typed", EntityID: "hero", Revision: 2, CastJSON: cast, Digest: "heads", ContextJSON: `{"latest":true}`}}
	service.store = wrapper
	profile := generateRequest(domain.StagePeopleWorld).Profile
	profile.Roles = append(profile.Roles, agent.ResolvedRole{Role: agent.RoleContinuityReviewer, ProviderName: "test", Model: "review-model"})
	request := RevisionRequest{FormalAssetID: "asset", ExpectedRevision: 2, Paths: []string{"/characters/@hero/summary"}, Instruction: "写清人物选择", IdempotencyKey: "formal-key"}
	view, err := service.PreviewRevision(ctx, "project-1", request, profile)
	if err != nil {
		t.Fatal(err)
	}
	if kernel.createN != 0 || view.FormalAssetID != "asset" || view.Targets[0].Value != "作者新版简介" {
		t.Fatalf("preview: %+v", view)
	}
	action, _ := store.Action(ctx, view.ActionID)
	input, _ := decodeRevisionInput(action)
	payload, err := input.Base.EditTextFields(map[string]string{request.Paths[0]: "AI 新版简介"})
	if err != nil {
		t.Fatal(err)
	}
	output := revisionEnvelope(input.Base)
	output.Payload = json.RawMessage(payload)
	encoded, _ := json.Marshal(output)
	kernel.payloads[agentassets.StoryCandidateRevisionWorkflowID] = string(encoded)
	command := RevisionCommand{ActionID: view.ActionID, ArgumentsHash: view.ArgumentsHash, IdempotencyKey: view.IdempotencyKey}
	if _, err = service.ConfirmRevision(ctx, "project-1", command, profile); err != nil {
		t.Fatal(err)
	}
	if !waitFor(t, func() bool {
		views, e := service.RevisionActions(ctx, "project-1", true)
		return e == nil && len(views) == 1 && views[0].State == "succeeded"
	}) {
		t.Fatal("run did not finish")
	}
	candidates, _ := store.ListCandidates(ctx, input.Base.SessionID)
	if len(candidates) != 0 {
		t.Fatal("formal result polluted creation candidates")
	}
	if _, err = service.ApplyRevision(ctx, "project-1", command); !errors.Is(err, asset.ErrChangeUnsupported) {
		t.Fatalf("stage adoption: %v", err)
	}
	wrapper.base.Revision = 3
	views, err := service.RevisionActions(ctx, "project-1", false)
	if err != nil || !views[0].Stale {
		t.Fatalf("late head: %+v %v", views, err)
	}
	request.IdempotencyKey = "late"
	if _, err = service.PreviewRevision(ctx, "project-1", request, profile); err == nil {
		t.Fatal("accepted old expected revision")
	}
}
