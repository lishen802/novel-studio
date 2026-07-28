package main

import (
	"context"

	"github.com/lishen802/novel-studio/internal/application/workspace"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type WorkspaceUseCases interface {
	CreateProject(context.Context, string, string, string, string) (workspace.ProjectView, error)
	OpenProject(context.Context, string) (workspace.ProjectView, error)
	ListChapters(context.Context) ([]workspace.ChapterView, error)
	CreateChapter(context.Context, string) (workspace.ChapterView, error)
	OpenChapter(context.Context, string) (workspace.ChapterView, error)
	SaveDraft(context.Context, string, string, int64) error
	DiscardDraft(context.Context, string, int64) error
	CommitChapter(context.Context, string, string, int64) (workspace.ChapterView, error)
	StoryStructure(context.Context) (workspace.StoryStructureView, error)
	CreateOutlineNode(context.Context, workspace.CreateOutlineNodeInput) (workspace.OutlineNodeView, error)
	CreateStoryLine(context.Context, workspace.CreateStoryLineInput) (workspace.StoryLineView, error)
	LinkStoryLine(context.Context, workspace.LinkStoryLineInput) (workspace.StoryLineLinkView, error)
	CreateSceneCard(context.Context, workspace.CreateSceneCardInput) (workspace.SceneCardView, error)
	ArchiveOutlineNode(context.Context, string) error
	MoveOutlineNode(context.Context, string, string) error
	StoryWorld(context.Context) (workspace.StoryWorldView, error)
	CreateCharacter(context.Context, workspace.CharacterInput) (workspace.CharacterView, error)
	UpdateCharacter(context.Context, string, workspace.CharacterInput) (workspace.CharacterView, error)
	ArchiveCharacter(context.Context, string) error
	CreateFaction(context.Context, workspace.FactionInput) (workspace.FactionView, error)
	UpdateFaction(context.Context, string, workspace.FactionInput) (workspace.FactionView, error)
	ArchiveFaction(context.Context, string) error
	UpsertMembership(context.Context, workspace.MembershipInput) (workspace.MembershipView, error)
	DeleteMembership(context.Context, string, string) error
	CreateRelationship(context.Context, workspace.RelationshipInput) (workspace.RelationshipView, error)
	ArchiveRelationship(context.Context, string) error
	LinkCharacterToChapter(context.Context, workspace.AppearanceInput) error
	UnlinkCharacterFromChapter(context.Context, string, string) error
	LinkCharacterToScene(context.Context, workspace.AppearanceInput) error
	UnlinkCharacterFromScene(context.Context, string, string) error
	WorldAssets(context.Context) (workspace.WorldAssetsView, error)
	CreateWorldRegion(context.Context, workspace.WorldRegionInput) (workspace.WorldRegionView, error)
	UpdateWorldRegion(context.Context, string, workspace.WorldRegionInput) (workspace.WorldRegionView, error)
	ArchiveWorldRegion(context.Context, string) error
	UpsertRegionAdjacency(context.Context, workspace.RegionAdjacencyInput) (workspace.RegionAdjacencyView, error)
	DeleteRegionAdjacency(context.Context, string, string) error
	CreateLocation(context.Context, workspace.LocationInput) (workspace.LocationView, error)
	UpdateLocation(context.Context, string, workspace.LocationInput) (workspace.LocationView, error)
	ArchiveLocation(context.Context, string) error
	LinkLocation(context.Context, workspace.LocationLinkInput) (workspace.LocationLinkView, error)
	UnlinkLocation(context.Context, string, string, string) error
	CreateItem(context.Context, workspace.ItemInput) (workspace.ItemView, error)
	UpdateItem(context.Context, string, workspace.ItemInput) (workspace.ItemView, error)
	TransferItem(context.Context, workspace.ItemTransferInput) (workspace.ItemTransferView, error)
	ArchiveItem(context.Context, string) error
	CreateWorldRule(context.Context, workspace.WorldRuleInput) (workspace.WorldRuleView, error)
	UpdateWorldRule(context.Context, string, workspace.WorldRuleInput) (workspace.WorldRuleView, error)
	ArchiveWorldRule(context.Context, string) error
	CreateForeshadowingHook(context.Context, workspace.ForeshadowingHookInput) (workspace.ForeshadowingHookView, error)
	UpdateForeshadowingHook(context.Context, string, workspace.ForeshadowingHookInput) (workspace.ForeshadowingHookView, error)
	ArchiveForeshadowingHook(context.Context, string) error
	Continuity(context.Context) (workspace.ContinuityView, error)
	CreateStoryEvent(context.Context, workspace.StoryEventInput) (workspace.StoryEventView, error)
	UpdateStoryEvent(context.Context, string, workspace.StoryEventInput) (workspace.StoryEventView, error)
	ArchiveStoryEvent(context.Context, string) error
	LinkStoryEvent(context.Context, workspace.StoryEventLinkInput) (workspace.StoryEventLinkView, error)
	UnlinkStoryEvent(context.Context, string, string, string) error
	CreateFact(context.Context, workspace.FactInput) (workspace.FactView, error)
	UpdateFact(context.Context, string, workspace.FactInput) (workspace.FactView, error)
	ConfirmFact(context.Context, string) (workspace.FactView, error)
	ArchiveFact(context.Context, string) error
	AddFactEvidence(context.Context, workspace.FactEvidenceInput) (workspace.FactEvidenceView, error)
	RemoveFactEvidence(context.Context, string) error
	AddFactScope(context.Context, workspace.FactScopeInput) (workspace.FactScopeView, error)
	RemoveFactScope(context.Context, string) error
	CreateFactConflict(context.Context, workspace.FactConflictInput) (workspace.FactConflictView, error)
	ResolveFactConflict(context.Context, string) (workspace.FactConflictView, error)
	DeleteFactConflict(context.Context, string) error
	Close() error
}

type DirectoryPicker interface {
	ChooseProjectParent(context.Context) (string, error)
	ChooseProjectToOpen(context.Context) (string, error)
}

type CreateProjectInput struct {
	ParentDirectory string `json:"parentDirectory"`
	Title           string `json:"title"`
	Genre           string `json:"genre"`
	Summary         string `json:"summary"`
}

type DesktopAPI struct {
	ctx           context.Context
	workspace     WorkspaceUseCases
	picker        DirectoryPicker
	completeClose func()
}

func newDesktopAPI(workspaceService WorkspaceUseCases, picker DirectoryPicker) *DesktopAPI {
	return &DesktopAPI{
		ctx:       context.Background(),
		workspace: workspaceService,
		picker:    picker,
	}
}

func (a *DesktopAPI) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *DesktopAPI) ChooseProjectParent() (string, error) {
	return safeResult(a.picker.ChooseProjectParent(a.ctx))
}

func (a *DesktopAPI) ChooseProjectToOpen() (string, error) {
	return safeResult(a.picker.ChooseProjectToOpen(a.ctx))
}

func (a *DesktopAPI) CreateProject(input CreateProjectInput) (workspace.ProjectView, error) {
	return safeResult(a.workspace.CreateProject(
		a.ctx,
		input.ParentDirectory,
		input.Title,
		input.Genre,
		input.Summary,
	))
}

func (a *DesktopAPI) OpenProject(path string) (workspace.ProjectView, error) {
	return safeResult(a.workspace.OpenProject(a.ctx, path))
}

func (a *DesktopAPI) ListChapters() ([]workspace.ChapterView, error) {
	return safeResult(a.workspace.ListChapters(a.ctx))
}

func (a *DesktopAPI) CreateChapter(title string) (workspace.ChapterView, error) {
	return safeResult(a.workspace.CreateChapter(a.ctx, title))
}

func (a *DesktopAPI) OpenChapter(id string) (workspace.ChapterView, error) {
	return safeResult(a.workspace.OpenChapter(a.ctx, id))
}

func (a *DesktopAPI) SaveDraft(id, body string, revision int64) error {
	return safeDesktopError(a.workspace.SaveDraft(a.ctx, id, body, revision))
}

func (a *DesktopAPI) DiscardDraft(id string, revision int64) error {
	return safeDesktopError(a.workspace.DiscardDraft(a.ctx, id, revision))
}

func (a *DesktopAPI) CommitChapter(id, body string, revision int64) (workspace.ChapterView, error) {
	return safeResult(a.workspace.CommitChapter(a.ctx, id, body, revision))
}

func (a *DesktopAPI) StoryStructure() (workspace.StoryStructureView, error) {
	return safeResult(a.workspace.StoryStructure(a.ctx))
}

func (a *DesktopAPI) CreateOutlineNode(input workspace.CreateOutlineNodeInput) (workspace.OutlineNodeView, error) {
	return safeResult(a.workspace.CreateOutlineNode(a.ctx, input))
}

func (a *DesktopAPI) CreateStoryLine(input workspace.CreateStoryLineInput) (workspace.StoryLineView, error) {
	return safeResult(a.workspace.CreateStoryLine(a.ctx, input))
}

func (a *DesktopAPI) LinkStoryLine(input workspace.LinkStoryLineInput) (workspace.StoryLineLinkView, error) {
	return safeResult(a.workspace.LinkStoryLine(a.ctx, input))
}

func (a *DesktopAPI) CreateSceneCard(input workspace.CreateSceneCardInput) (workspace.SceneCardView, error) {
	return safeResult(a.workspace.CreateSceneCard(a.ctx, input))
}

func (a *DesktopAPI) ArchiveOutlineNode(id string) error {
	return safeDesktopError(a.workspace.ArchiveOutlineNode(a.ctx, id))
}

func (a *DesktopAPI) MoveOutlineNode(id, direction string) error {
	return safeDesktopError(a.workspace.MoveOutlineNode(a.ctx, id, direction))
}

func (a *DesktopAPI) StoryWorld() (workspace.StoryWorldView, error) {
	return safeResult(a.workspace.StoryWorld(a.ctx))
}

func (a *DesktopAPI) CreateCharacter(input workspace.CharacterInput) (workspace.CharacterView, error) {
	return safeResult(a.workspace.CreateCharacter(a.ctx, input))
}

func (a *DesktopAPI) UpdateCharacter(id string, input workspace.CharacterInput) (workspace.CharacterView, error) {
	return safeResult(a.workspace.UpdateCharacter(a.ctx, id, input))
}

func (a *DesktopAPI) ArchiveCharacter(id string) error {
	return safeDesktopError(a.workspace.ArchiveCharacter(a.ctx, id))
}

func (a *DesktopAPI) CreateFaction(input workspace.FactionInput) (workspace.FactionView, error) {
	return safeResult(a.workspace.CreateFaction(a.ctx, input))
}

func (a *DesktopAPI) UpdateFaction(id string, input workspace.FactionInput) (workspace.FactionView, error) {
	return safeResult(a.workspace.UpdateFaction(a.ctx, id, input))
}

func (a *DesktopAPI) ArchiveFaction(id string) error {
	return safeDesktopError(a.workspace.ArchiveFaction(a.ctx, id))
}

func (a *DesktopAPI) UpsertMembership(input workspace.MembershipInput) (workspace.MembershipView, error) {
	return safeResult(a.workspace.UpsertMembership(a.ctx, input))
}

func (a *DesktopAPI) DeleteMembership(characterID, factionID string) error {
	return safeDesktopError(a.workspace.DeleteMembership(a.ctx, characterID, factionID))
}

func (a *DesktopAPI) CreateRelationship(input workspace.RelationshipInput) (workspace.RelationshipView, error) {
	return safeResult(a.workspace.CreateRelationship(a.ctx, input))
}

func (a *DesktopAPI) ArchiveRelationship(id string) error {
	return safeDesktopError(a.workspace.ArchiveRelationship(a.ctx, id))
}

func (a *DesktopAPI) LinkCharacterToChapter(input workspace.AppearanceInput) error {
	return safeDesktopError(a.workspace.LinkCharacterToChapter(a.ctx, input))
}

func (a *DesktopAPI) UnlinkCharacterFromChapter(characterID, chapterID string) error {
	return safeDesktopError(a.workspace.UnlinkCharacterFromChapter(a.ctx, characterID, chapterID))
}

func (a *DesktopAPI) LinkCharacterToScene(input workspace.AppearanceInput) error {
	return safeDesktopError(a.workspace.LinkCharacterToScene(a.ctx, input))
}

func (a *DesktopAPI) UnlinkCharacterFromScene(characterID, sceneCardID string) error {
	return safeDesktopError(a.workspace.UnlinkCharacterFromScene(a.ctx, characterID, sceneCardID))
}

func (a *DesktopAPI) WorldAssets() (workspace.WorldAssetsView, error) {
	return safeResult(a.workspace.WorldAssets(a.ctx))
}

func (a *DesktopAPI) CreateWorldRegion(input workspace.WorldRegionInput) (workspace.WorldRegionView, error) {
	return safeResult(a.workspace.CreateWorldRegion(a.ctx, input))
}

func (a *DesktopAPI) UpdateWorldRegion(id string, input workspace.WorldRegionInput) (workspace.WorldRegionView, error) {
	return safeResult(a.workspace.UpdateWorldRegion(a.ctx, id, input))
}

func (a *DesktopAPI) ArchiveWorldRegion(id string) error {
	return safeDesktopError(a.workspace.ArchiveWorldRegion(a.ctx, id))
}

func (a *DesktopAPI) UpsertRegionAdjacency(input workspace.RegionAdjacencyInput) (workspace.RegionAdjacencyView, error) {
	return safeResult(a.workspace.UpsertRegionAdjacency(a.ctx, input))
}

func (a *DesktopAPI) DeleteRegionAdjacency(firstRegionID, secondRegionID string) error {
	return safeDesktopError(a.workspace.DeleteRegionAdjacency(a.ctx, firstRegionID, secondRegionID))
}

func (a *DesktopAPI) CreateLocation(input workspace.LocationInput) (workspace.LocationView, error) {
	return safeResult(a.workspace.CreateLocation(a.ctx, input))
}

func (a *DesktopAPI) UpdateLocation(id string, input workspace.LocationInput) (workspace.LocationView, error) {
	return safeResult(a.workspace.UpdateLocation(a.ctx, id, input))
}

func (a *DesktopAPI) ArchiveLocation(id string) error {
	return safeDesktopError(a.workspace.ArchiveLocation(a.ctx, id))
}

func (a *DesktopAPI) LinkLocation(input workspace.LocationLinkInput) (workspace.LocationLinkView, error) {
	return safeResult(a.workspace.LinkLocation(a.ctx, input))
}

func (a *DesktopAPI) UnlinkLocation(locationID, targetType, targetID string) error {
	return safeDesktopError(a.workspace.UnlinkLocation(a.ctx, locationID, targetType, targetID))
}

func (a *DesktopAPI) CreateItem(input workspace.ItemInput) (workspace.ItemView, error) {
	return safeResult(a.workspace.CreateItem(a.ctx, input))
}

func (a *DesktopAPI) UpdateItem(id string, input workspace.ItemInput) (workspace.ItemView, error) {
	return safeResult(a.workspace.UpdateItem(a.ctx, id, input))
}

func (a *DesktopAPI) TransferItem(input workspace.ItemTransferInput) (workspace.ItemTransferView, error) {
	return safeResult(a.workspace.TransferItem(a.ctx, input))
}

func (a *DesktopAPI) ArchiveItem(id string) error {
	return safeDesktopError(a.workspace.ArchiveItem(a.ctx, id))
}

func (a *DesktopAPI) CreateWorldRule(input workspace.WorldRuleInput) (workspace.WorldRuleView, error) {
	return safeResult(a.workspace.CreateWorldRule(a.ctx, input))
}

func (a *DesktopAPI) UpdateWorldRule(id string, input workspace.WorldRuleInput) (workspace.WorldRuleView, error) {
	return safeResult(a.workspace.UpdateWorldRule(a.ctx, id, input))
}

func (a *DesktopAPI) ArchiveWorldRule(id string) error {
	return safeDesktopError(a.workspace.ArchiveWorldRule(a.ctx, id))
}

func (a *DesktopAPI) CreateForeshadowingHook(input workspace.ForeshadowingHookInput) (workspace.ForeshadowingHookView, error) {
	return safeResult(a.workspace.CreateForeshadowingHook(a.ctx, input))
}

func (a *DesktopAPI) UpdateForeshadowingHook(id string, input workspace.ForeshadowingHookInput) (workspace.ForeshadowingHookView, error) {
	return safeResult(a.workspace.UpdateForeshadowingHook(a.ctx, id, input))
}

func (a *DesktopAPI) ArchiveForeshadowingHook(id string) error {
	return safeDesktopError(a.workspace.ArchiveForeshadowingHook(a.ctx, id))
}

func (a *DesktopAPI) Continuity() (workspace.ContinuityView, error) {
	return safeResult(a.workspace.Continuity(a.ctx))
}

func (a *DesktopAPI) CreateStoryEvent(input workspace.StoryEventInput) (workspace.StoryEventView, error) {
	return safeResult(a.workspace.CreateStoryEvent(a.ctx, input))
}

func (a *DesktopAPI) UpdateStoryEvent(id string, input workspace.StoryEventInput) (workspace.StoryEventView, error) {
	return safeResult(a.workspace.UpdateStoryEvent(a.ctx, id, input))
}

func (a *DesktopAPI) ArchiveStoryEvent(id string) error {
	return safeDesktopError(a.workspace.ArchiveStoryEvent(a.ctx, id))
}

func (a *DesktopAPI) LinkStoryEvent(input workspace.StoryEventLinkInput) (workspace.StoryEventLinkView, error) {
	return safeResult(a.workspace.LinkStoryEvent(a.ctx, input))
}

func (a *DesktopAPI) UnlinkStoryEvent(eventID, targetType, targetID string) error {
	return safeDesktopError(a.workspace.UnlinkStoryEvent(a.ctx, eventID, targetType, targetID))
}

func (a *DesktopAPI) CreateFact(input workspace.FactInput) (workspace.FactView, error) {
	return safeResult(a.workspace.CreateFact(a.ctx, input))
}

func (a *DesktopAPI) UpdateFact(id string, input workspace.FactInput) (workspace.FactView, error) {
	return safeResult(a.workspace.UpdateFact(a.ctx, id, input))
}

func (a *DesktopAPI) ConfirmFact(id string) (workspace.FactView, error) {
	return safeResult(a.workspace.ConfirmFact(a.ctx, id))
}

func (a *DesktopAPI) ArchiveFact(id string) error {
	return safeDesktopError(a.workspace.ArchiveFact(a.ctx, id))
}

func (a *DesktopAPI) AddFactEvidence(input workspace.FactEvidenceInput) (workspace.FactEvidenceView, error) {
	return safeResult(a.workspace.AddFactEvidence(a.ctx, input))
}

func (a *DesktopAPI) RemoveFactEvidence(id string) error {
	return safeDesktopError(a.workspace.RemoveFactEvidence(a.ctx, id))
}

func (a *DesktopAPI) AddFactScope(input workspace.FactScopeInput) (workspace.FactScopeView, error) {
	return safeResult(a.workspace.AddFactScope(a.ctx, input))
}

func (a *DesktopAPI) RemoveFactScope(id string) error {
	return safeDesktopError(a.workspace.RemoveFactScope(a.ctx, id))
}

func (a *DesktopAPI) CreateFactConflict(input workspace.FactConflictInput) (workspace.FactConflictView, error) {
	return safeResult(a.workspace.CreateFactConflict(a.ctx, input))
}

func (a *DesktopAPI) ResolveFactConflict(id string) (workspace.FactConflictView, error) {
	return safeResult(a.workspace.ResolveFactConflict(a.ctx, id))
}

func (a *DesktopAPI) DeleteFactConflict(id string) error {
	return safeDesktopError(a.workspace.DeleteFactConflict(a.ctx, id))
}

func (a *DesktopAPI) CompleteClose() {
	if a.completeClose != nil {
		a.completeClose()
	}
}

type wailsDirectoryPicker struct{}

func (wailsDirectoryPicker) ChooseProjectParent(ctx context.Context) (string, error) {
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "选择小说项目保存位置",
	})
}

func (wailsDirectoryPicker) ChooseProjectToOpen(ctx context.Context) (string, error) {
	return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{
		Title: "打开小说项目",
	})
}
