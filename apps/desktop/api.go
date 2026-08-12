package main

import (
	"context"
	"errors"
	"time"

	"github.com/lishen802/novel-studio/internal/application/appsettings"
	"github.com/lishen802/novel-studio/internal/application/appshell"
	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/application/workspace"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	appdomain "github.com/lishen802/novel-studio/internal/domain/appsettings"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type WorkspaceUseCases interface {
	CreateProject(context.Context, string, string, string, string) (workspace.ProjectView, error)
	OpenProject(context.Context, string) (workspace.ProjectView, error)
	OpenProjectReadOnly(context.Context, string) (workspace.ProjectView, error)
	CreateProjectRecoveryCopy(context.Context, string, string) (workspace.RecoveryCopyResult, error)
	FilesystemHealth(context.Context) (ports.ProjectFilesystemHealth, error)
	ListChapters(context.Context) ([]workspace.ChapterView, error)
	CreateChapter(context.Context, string) (workspace.ChapterView, error)
	OpenChapter(context.Context, string) (workspace.ChapterView, error)
	SaveDraft(context.Context, string, string, int64) error
	DiscardDraft(context.Context, string, int64) error
	CommitChapter(context.Context, string, string, int64) (workspace.ChapterView, error)
	ChangeSets(context.Context) ([]workspace.ChangeSetView, error)
	ProposeChapterChange(context.Context, workspace.ChapterChangeInput) (workspace.ChangeSetView, error)
	ProposeStructuredChange(context.Context, workspace.StructuredChangeInput) (workspace.ChangeSetView, error)
	EditChangeOperation(context.Context, workspace.ChangeOperationEditInput) (workspace.ChangeSetView, error)
	ApplyChangeSet(context.Context, string) (workspace.ChangeSetApplyView, error)
	ApplyChangeOperation(context.Context, string, string) (workspace.ChangeSetApplyView, error)
	RejectChangeSet(context.Context, string) error
	RejectChangeOperation(context.Context, string, string) error
	ChapterHistory(context.Context, string) ([]workspace.ChapterRevisionView, error)
	RestoreChapterRevision(context.Context, string, int64, int64) (workspace.ChapterView, error)
	Snapshots(context.Context) ([]workspace.SnapshotView, error)
	CreateSnapshot(context.Context, string) (workspace.SnapshotView, error)
	RestoreSnapshot(context.Context, string) (workspace.SnapshotRestoreView, error)
	Branches(context.Context) ([]workspace.BranchView, error)
	CreateBranch(context.Context, string, string) (workspace.BranchView, error)
	SwitchBranch(context.Context, string) (workspace.BranchSwitchView, error)
	CompareBranches(context.Context, string, string) (workspace.BranchComparisonView, error)
	PreviewBranchMerge(context.Context, string) (workspace.BranchMergePreviewView, error)
	ConfirmBranchMerge(context.Context, string) (workspace.BranchSwitchView, error)
	DeleteBranch(context.Context, string) error
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
	CharacterKnowledge(context.Context) ([]workspace.CharacterKnowledgeView, error)
	CreateCharacterKnowledge(context.Context, workspace.CharacterKnowledgeInput) (workspace.CharacterKnowledgeView, error)
	ArchiveCharacterKnowledge(context.Context, string) error
	AddFactEvidence(context.Context, workspace.FactEvidenceInput) (workspace.FactEvidenceView, error)
	RemoveFactEvidence(context.Context, string) error
	AddFactScope(context.Context, workspace.FactScopeInput) (workspace.FactScopeView, error)
	RemoveFactScope(context.Context, string) error
	CreateFactConflict(context.Context, workspace.FactConflictInput) (workspace.FactConflictView, error)
	ResolveFactConflict(context.Context, string) (workspace.FactConflictView, error)
	DeleteFactConflict(context.Context, string) error
	ProviderConnections(context.Context) ([]workspace.ProviderConnectionView, error)
	SaveProviderConnection(context.Context, workspace.ProviderConnectionInput) (workspace.ProviderConnectionView, error)
	DeleteProviderConnection(context.Context, string) error
	TestProviderConnection(context.Context, string) error
	AgentProfiles(context.Context) ([]workspace.AgentProfileView, error)
	SaveAgentProfile(context.Context, workspace.AgentProfileInput) (workspace.AgentProfileView, error)
	AgentRuntimeSettings(context.Context) (workspace.AgentRuntimeSettingsView, error)
	SaveAgentRuntimeSettings(context.Context, workspace.AgentRuntimeSettingsInput) (workspace.AgentRuntimeSettingsView, error)
	CreateChapterAgentRun(context.Context, string, string) (workspace.AgentRunView, error)
	ExecuteAgentRun(context.Context, string) (workspace.AgentRunView, error)
	CancelAgentRun(context.Context, string) error
	RetryAgentRun(context.Context, string) (workspace.AgentRunView, error)
	AgentRuns(context.Context) ([]workspace.AgentRunView, error)
	AgentRun(context.Context, string) (workspace.AgentRunView, error)
	PrepareManuscriptImport(context.Context, string) (workspace.ImportPrepareResult, error)
	ImportJobs(context.Context) ([]workspace.ImportJobView, error)
	ImportJob(context.Context, string) (workspace.ImportJobView, error)
	DeleteImportJob(context.Context, string) error
	ApplyImportJob(context.Context, string, []string) (workspace.ImportApplyView, error)
	StartExtractionRun(context.Context, workspace.ExtractionRunInput) (workspace.ExtractionRunView, error)
	ExecuteExtractionRun(context.Context, string) (workspace.ExtractionRunView, error)
	CancelExtractionRun(context.Context, string) error
	RetryExtractionRun(context.Context, string) (workspace.ExtractionRunView, error)
	ExtractionTasks(context.Context) ([]workspace.ExtractionTaskView, error)
	ExtractionTask(context.Context, string) (workspace.ExtractionRunView, error)
	GetJobSnapshot(context.Context, string) (workspace.JobSnapshotView, error)
	GetJobEvents(context.Context, string, uint64, int) (workspace.JobEventsPageView, error)
	ListRecoverableJobs(context.Context) ([]workspace.JobSummaryView, error)
	Workbench(context.Context) (workspace.WorkbenchProjection, error)
	Close() error
}

type DirectoryPicker interface {
	ChooseProjectParent(context.Context) (string, error)
	ChooseProjectToOpen(context.Context) (string, error)
	ChooseManuscriptToImport(context.Context) (string, error)
}

type CreateProjectInput struct {
	ParentDirectory string `json:"parentDirectory"`
	Title           string `json:"title"`
	Genre           string `json:"genre"`
	Summary         string `json:"summary"`
}

type ProjectOpenMode string

const (
	ProjectOpenModeEditable ProjectOpenMode = "editable"
	ProjectOpenModeReadOnly ProjectOpenMode = "read_only"
)

type ProjectOpenInput struct {
	Path string          `json:"path"`
	Mode ProjectOpenMode `json:"mode"`
}

type projectOpenOutcome string

const (
	projectOpenOutcomeOpened        projectOpenOutcome = "opened"
	projectOpenOutcomeLeaseConflict projectOpenOutcome = "lease_conflict"
)

type ProjectLeaseConflictView struct {
	// Path echoes the user-selected package path. The platform conflict's
	// package path is deliberately not forwarded because it can contain an
	// internal or otherwise sensitive absolute path.
	Path      string                      `json:"path"`
	Holder    *ports.ProjectLeaseMetadata `json:"holder,omitempty"`
	Retryable bool                        `json:"retryable"`
}

type ProjectOpenResultView struct {
	Outcome  projectOpenOutcome        `json:"outcome"`
	Project  *workspace.ProjectView    `json:"project,omitempty"`
	Conflict *ProjectLeaseConflictView `json:"conflict,omitempty"`
	Warning  string                    `json:"warning,omitempty"`
}

type RecoveryCopyInput struct {
	SourcePath      string `json:"sourcePath"`
	ParentDirectory string `json:"parentDirectory"`
}

type recoveryCopyOutcome string

const (
	recoveryCopyOutcomeOpened    recoveryCopyOutcome = "opened"
	recoveryCopyOutcomePublished recoveryCopyOutcome = "published"
)

type RecoveryCopyResultView struct {
	Outcome recoveryCopyOutcome    `json:"outcome"`
	Path    string                 `json:"path"`
	Project *workspace.ProjectView `json:"project,omitempty"`
}

type CreateBranchInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ApplicationPreferencesView is the safe projection of application-level
// preferences. It never includes credentials or internal paths.
type ApplicationPreferencesView struct {
	Theme                    string `json:"theme"`
	DefaultCreativeProfileID string `json:"defaultCreativeProfileId,omitempty"`
	UpdatedAt                string `json:"updatedAt"`
}

// ApplicationPreferencesInput carries the user's preferences update.
type ApplicationPreferencesInput struct {
	Theme                    string `json:"theme"`
	DefaultCreativeProfileID string `json:"defaultCreativeProfileId,omitempty"`
}

// RecentProjectView is the safe projection of a recent project entry.
type RecentProjectView struct {
	ProjectID    string `json:"projectId"`
	Title        string `json:"title"`
	Path         string `json:"path"`
	LastOpenedAt string `json:"lastOpenedAt"`
	Missing      bool   `json:"missing"`
}

// ProviderView is the safe projection of a provider connection. It never
// includes the credential reference or API key value.
type ProviderView struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	BaseURL       string `json:"baseUrl"`
	HasCredential bool   `json:"hasCredential"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// ApplicationProviderInput carries the data needed to save a provider
// connection. The API key is forwarded to the OS credential vault and never
// persisted in the settings database.
type ApplicationProviderInput struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	BaseURL string `json:"baseUrl"`
	APIKey  string `json:"apiKey"`
}

// RoleBindingView is the safe projection of a role binding in a creative
// profile version.
type RoleBindingView struct {
	Role            string  `json:"role"`
	ConnectionID    string  `json:"connectionId"`
	Model           string  `json:"model"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Temperature     float64 `json:"temperature"`
	Enabled         bool    `json:"enabled"`
}

// CreativeProfileView is the safe projection of a creative profile version.
type CreativeProfileView struct {
	ProfileID       string            `json:"profileId"`
	Version         int64             `json:"version"`
	Name            string            `json:"name"`
	Roles           []RoleBindingView `json:"roles"`
	MaxRunTokens    int               `json:"maxRunTokens"`
	MaxContextChars int               `json:"maxContextChars"`
	CreatedAt       string            `json:"createdAt"`
}

// PublishProfileInput carries the data needed to publish a new creative
// profile version.
type PublishProfileInput struct {
	ProfileID       string             `json:"profileId"`
	Name            string             `json:"name"`
	Roles           []PublishRoleInput `json:"roles"`
	MaxRunTokens    int                `json:"maxRunTokens"`
	MaxContextChars int                `json:"maxContextChars"`
}

// PublishRoleInput is a single role binding in a publish profile input.
type PublishRoleInput struct {
	Role            string  `json:"role"`
	ConnectionID    string  `json:"connectionId"`
	Model           string  `json:"model"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Temperature     float64 `json:"temperature"`
	Enabled         bool    `json:"enabled"`
}

// ApplicationBootstrapView is the initial state payload sent to the frontend
// on startup. It contains only safe, non-sensitive data.
type ApplicationBootstrapView struct {
	Preferences    ApplicationPreferencesView `json:"preferences"`
	RecentProjects []RecentProjectView        `json:"recentProjects"`
	Providers      []ProviderView             `json:"providers"`
	Profiles       []CreativeProfileView      `json:"profiles"`
}

type DesktopAPI struct {
	ctx           context.Context
	workspace     WorkspaceUseCases
	picker        DirectoryPicker
	completeClose func()
	settings      *appsettings.Service
	shell         *appshell.Service
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

func (a *DesktopAPI) ProviderConnections() ([]workspace.ProviderConnectionView, error) {
	return safeResult(a.workspace.ProviderConnections(a.ctx))
}

func (a *DesktopAPI) SaveProviderConnection(input workspace.ProviderConnectionInput) (workspace.ProviderConnectionView, error) {
	return safeResult(a.workspace.SaveProviderConnection(a.ctx, input))
}

func (a *DesktopAPI) DeleteProviderConnection(id string) error {
	return safeDesktopError(a.workspace.DeleteProviderConnection(a.ctx, id))
}

func (a *DesktopAPI) TestProviderConnection(id string) error {
	return safeDesktopError(a.workspace.TestProviderConnection(a.ctx, id))
}

func (a *DesktopAPI) AgentProfiles() ([]workspace.AgentProfileView, error) {
	return safeResult(a.workspace.AgentProfiles(a.ctx))
}

func (a *DesktopAPI) SaveAgentProfile(input workspace.AgentProfileInput) (workspace.AgentProfileView, error) {
	return safeResult(a.workspace.SaveAgentProfile(a.ctx, input))
}

func (a *DesktopAPI) AgentRuntimeSettings() (workspace.AgentRuntimeSettingsView, error) {
	return safeResult(a.workspace.AgentRuntimeSettings(a.ctx))
}

func (a *DesktopAPI) SaveAgentRuntimeSettings(input workspace.AgentRuntimeSettingsInput) (workspace.AgentRuntimeSettingsView, error) {
	return safeResult(a.workspace.SaveAgentRuntimeSettings(a.ctx, input))
}

func (a *DesktopAPI) CreateChapterAgentRun(chapterID, prompt string) (workspace.AgentRunView, error) {
	return safeResult(a.workspace.CreateChapterAgentRun(a.ctx, chapterID, prompt))
}

func (a *DesktopAPI) ExecuteAgentRun(id string) (workspace.AgentRunView, error) {
	return safeResult(a.workspace.ExecuteAgentRun(a.ctx, id))
}

func (a *DesktopAPI) CancelAgentRun(id string) error {
	return safeDesktopError(a.workspace.CancelAgentRun(a.ctx, id))
}

func (a *DesktopAPI) RetryAgentRun(id string) (workspace.AgentRunView, error) {
	return safeResult(a.workspace.RetryAgentRun(a.ctx, id))
}

func (a *DesktopAPI) AgentRuns() ([]workspace.AgentRunView, error) {
	return safeResult(a.workspace.AgentRuns(a.ctx))
}

func (a *DesktopAPI) AgentRun(id string) (workspace.AgentRunView, error) {
	return safeResult(a.workspace.AgentRun(a.ctx, id))
}

func (a *DesktopAPI) ChooseProjectParent() (string, error) {
	return safeResult(a.picker.ChooseProjectParent(a.ctx))
}

func (a *DesktopAPI) ChooseProjectToOpen() (string, error) {
	return safeResult(a.picker.ChooseProjectToOpen(a.ctx))
}

func (a *DesktopAPI) ChooseManuscriptToImport() (string, error) {
	return safeResult(a.picker.ChooseManuscriptToImport(a.ctx))
}

func (a *DesktopAPI) PrepareManuscriptImport(sourcePath string) (workspace.ImportPrepareResult, error) {
	return safeResult(a.workspace.PrepareManuscriptImport(a.ctx, sourcePath))
}

func (a *DesktopAPI) ImportJobs() ([]workspace.ImportJobView, error) {
	return safeResult(a.workspace.ImportJobs(a.ctx))
}

func (a *DesktopAPI) ImportJob(id string) (workspace.ImportJobView, error) {
	return safeResult(a.workspace.ImportJob(a.ctx, id))
}

func (a *DesktopAPI) DeleteImportJob(id string) error {
	return safeDesktopError(a.workspace.DeleteImportJob(a.ctx, id))
}

func (a *DesktopAPI) ApplyImportJob(id string, selectedChapterIDs []string) (workspace.ImportApplyView, error) {
	return safeResult(a.workspace.ApplyImportJob(a.ctx, id, selectedChapterIDs))
}

func (a *DesktopAPI) StartExtractionRun(input workspace.ExtractionRunInput) (workspace.ExtractionRunView, error) {
	return safeResult(a.workspace.StartExtractionRun(a.ctx, input))
}

func (a *DesktopAPI) ExecuteExtractionRun(id string) (workspace.ExtractionRunView, error) {
	return safeResult(a.workspace.ExecuteExtractionRun(a.ctx, id))
}

func (a *DesktopAPI) CancelExtractionRun(id string) error {
	return safeDesktopError(a.workspace.CancelExtractionRun(a.ctx, id))
}

func (a *DesktopAPI) RetryExtractionRun(id string) (workspace.ExtractionRunView, error) {
	return safeResult(a.workspace.RetryExtractionRun(a.ctx, id))
}

func (a *DesktopAPI) ExtractionTasks() ([]workspace.ExtractionTaskView, error) {
	return safeResult(a.workspace.ExtractionTasks(a.ctx))
}

func (a *DesktopAPI) ExtractionTask(id string) (workspace.ExtractionRunView, error) {
	return safeResult(a.workspace.ExtractionTask(a.ctx, id))
}

func (a *DesktopAPI) GetJobSnapshot(jobID string) (workspace.JobSnapshotView, error) {
	return safeResult(a.workspace.GetJobSnapshot(a.ctx, jobID))
}

func (a *DesktopAPI) GetJobEvents(jobID string, afterSequence uint64, limit int) (workspace.JobEventsPageView, error) {
	return safeResult(a.workspace.GetJobEvents(a.ctx, jobID, afterSequence, limit))
}

func (a *DesktopAPI) ListRecoverableJobs() ([]workspace.JobSummaryView, error) {
	return safeResult(a.workspace.ListRecoverableJobs(a.ctx))
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

func (a *DesktopAPI) OpenProject(input ProjectOpenInput) (ProjectOpenResultView, error) {
	var (
		project workspace.ProjectView
		err     error
	)
	switch input.Mode {
	case ProjectOpenModeEditable:
		project, err = a.workspace.OpenProject(a.ctx, input.Path)
	case ProjectOpenModeReadOnly:
		project, err = a.workspace.OpenProjectReadOnly(a.ctx, input.Path)
	default:
		return ProjectOpenResultView{}, safeDesktopError(errInvalidProjectOpenMode)
	}
	if err == nil {
		return ProjectOpenResultView{
			Outcome: projectOpenOutcomeOpened,
			Project: &project,
		}, nil
	}
	var conflict *ports.ProjectLeaseConflict
	if input.Mode == ProjectOpenModeEditable && errors.As(err, &conflict) {
		return ProjectOpenResultView{
			Outcome: projectOpenOutcomeLeaseConflict,
			Conflict: &ProjectLeaseConflictView{
				Path:      input.Path,
				Holder:    conflict.Holder,
				Retryable: conflict.Retryable,
			},
		}, nil
	}
	return ProjectOpenResultView{}, safeDesktopError(err)
}

func (a *DesktopAPI) CreateProjectRecoveryCopy(input RecoveryCopyInput) (RecoveryCopyResultView, error) {
	result, err := a.workspace.CreateProjectRecoveryCopy(a.ctx, input.SourcePath, input.ParentDirectory)
	if result.Path != "" {
		outcome := recoveryCopyOutcomePublished
		if result.Project != nil {
			outcome = recoveryCopyOutcomeOpened
		}
		return RecoveryCopyResultView{
			Outcome: outcome,
			Path:    result.Path,
			Project: result.Project,
		}, nil
	}
	if err != nil {
		return RecoveryCopyResultView{}, safeDesktopError(err)
	}
	return RecoveryCopyResultView{}, safeDesktopError(ErrRecoveryCopyResultMissing)
}

func (a *DesktopAPI) FilesystemHealth() (ports.ProjectFilesystemHealth, error) {
	return safeResult(a.workspace.FilesystemHealth(a.ctx))
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

func (a *DesktopAPI) ChangeSets() ([]workspace.ChangeSetView, error) {
	return safeResult(a.workspace.ChangeSets(a.ctx))
}

func (a *DesktopAPI) ProposeChapterChange(input workspace.ChapterChangeInput) (workspace.ChangeSetView, error) {
	return safeResult(a.workspace.ProposeChapterChange(a.ctx, input))
}

func (a *DesktopAPI) ProposeStructuredChange(input workspace.StructuredChangeInput) (workspace.ChangeSetView, error) {
	return safeResult(a.workspace.ProposeStructuredChange(a.ctx, input))
}

func (a *DesktopAPI) EditChangeOperation(input workspace.ChangeOperationEditInput) (workspace.ChangeSetView, error) {
	return safeResult(a.workspace.EditChangeOperation(a.ctx, input))
}

func (a *DesktopAPI) ApplyChangeSet(id string) (workspace.ChangeSetApplyView, error) {
	return safeResult(a.workspace.ApplyChangeSet(a.ctx, id))
}

func (a *DesktopAPI) ApplyChangeOperation(changeSetID, operationID string) (workspace.ChangeSetApplyView, error) {
	return safeResult(a.workspace.ApplyChangeOperation(a.ctx, changeSetID, operationID))
}

func (a *DesktopAPI) RejectChangeSet(id string) error {
	return safeDesktopError(a.workspace.RejectChangeSet(a.ctx, id))
}

func (a *DesktopAPI) RejectChangeOperation(changeSetID, operationID string) error {
	return safeDesktopError(a.workspace.RejectChangeOperation(a.ctx, changeSetID, operationID))
}

func (a *DesktopAPI) ChapterHistory(id string) ([]workspace.ChapterRevisionView, error) {
	return safeResult(a.workspace.ChapterHistory(a.ctx, id))
}

func (a *DesktopAPI) RestoreChapterRevision(
	id string,
	targetRevision int64,
	expectedRevision int64,
) (workspace.ChapterView, error) {
	return safeResult(a.workspace.RestoreChapterRevision(
		a.ctx,
		id,
		targetRevision,
		expectedRevision,
	))
}

func (a *DesktopAPI) Snapshots() ([]workspace.SnapshotView, error) {
	return safeResult(a.workspace.Snapshots(a.ctx))
}

func (a *DesktopAPI) CreateSnapshot(name string) (workspace.SnapshotView, error) {
	return safeResult(a.workspace.CreateSnapshot(a.ctx, name))
}

func (a *DesktopAPI) RestoreSnapshot(id string) (workspace.SnapshotRestoreView, error) {
	return safeResult(a.workspace.RestoreSnapshot(a.ctx, id))
}

func (a *DesktopAPI) Branches() ([]workspace.BranchView, error) {
	return safeResult(a.workspace.Branches(a.ctx))
}

func (a *DesktopAPI) CreateBranch(input CreateBranchInput) (workspace.BranchView, error) {
	return safeResult(a.workspace.CreateBranch(a.ctx, input.Name, input.Description))
}

func (a *DesktopAPI) SwitchBranch(id string) (workspace.BranchSwitchView, error) {
	return safeResult(a.workspace.SwitchBranch(a.ctx, id))
}

func (a *DesktopAPI) CompareBranches(sourceID, targetID string) (workspace.BranchComparisonView, error) {
	return safeResult(a.workspace.CompareBranches(a.ctx, sourceID, targetID))
}

func (a *DesktopAPI) PreviewBranchMerge(sourceID string) (workspace.BranchMergePreviewView, error) {
	return safeResult(a.workspace.PreviewBranchMerge(a.ctx, sourceID))
}

func (a *DesktopAPI) ConfirmBranchMerge(token string) (workspace.BranchSwitchView, error) {
	return safeResult(a.workspace.ConfirmBranchMerge(a.ctx, token))
}

func (a *DesktopAPI) DeleteBranch(id string) error {
	return safeDesktopError(a.workspace.DeleteBranch(a.ctx, id))
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

func (a *DesktopAPI) CharacterKnowledge() ([]workspace.CharacterKnowledgeView, error) {
	return safeResult(a.workspace.CharacterKnowledge(a.ctx))
}

func (a *DesktopAPI) CreateCharacterKnowledge(input workspace.CharacterKnowledgeInput) (workspace.CharacterKnowledgeView, error) {
	return safeResult(a.workspace.CreateCharacterKnowledge(a.ctx, input))
}

func (a *DesktopAPI) ArchiveCharacterKnowledge(id string) error {
	return safeDesktopError(a.workspace.ArchiveCharacterKnowledge(a.ctx, id))
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

// ApplicationBootstrap returns the initial application state: preferences,
// recent projects, provider connections (without credentials), and the
// latest version of the default creative profile (if set).
func (a *DesktopAPI) ApplicationBootstrap() (ApplicationBootstrapView, error) {
	if a.settings == nil {
		return ApplicationBootstrapView{}, safeDesktopError(errFeatureNotAvailable)
	}
	ctx := a.ctx

	prefs, err := a.settings.Preferences(ctx)
	if err != nil {
		return ApplicationBootstrapView{}, safeDesktopError(err)
	}

	recent, err := a.settings.RecentProjects(ctx, 20)
	if err != nil {
		return ApplicationBootstrapView{}, safeDesktopError(err)
	}

	providers, err := a.settings.Providers(ctx)
	if err != nil {
		return ApplicationBootstrapView{}, safeDesktopError(err)
	}

	providerViews := make([]ProviderView, 0, len(providers))
	for _, p := range providers {
		providerViews = append(providerViews, ProviderView{
			ID:            p.ID,
			Name:          p.Name,
			Kind:          string(p.Kind),
			BaseURL:       p.BaseURL,
			HasCredential: a.settings.HasCredential(ctx, p),
			CreatedAt:     formatDesktopTime(p.CreatedAt),
			UpdatedAt:     formatDesktopTime(p.UpdatedAt),
		})
	}

	recentViews := make([]RecentProjectView, 0, len(recent))
	for _, rp := range recent {
		recentViews = append(recentViews, RecentProjectView{
			ProjectID:    rp.ProjectID,
			Title:        rp.Title,
			Path:         rp.Path,
			LastOpenedAt: formatDesktopTime(rp.LastOpenedAt),
			Missing:      rp.Missing,
		})
	}

	profiles := make([]CreativeProfileView, 0)
	if prefs.DefaultCreativeProfileID != "" {
		if latest, err := a.settings.LatestProfileVersion(ctx, prefs.DefaultCreativeProfileID); err == nil {
			profiles = append(profiles, creativeProfileView(latest))
		}
	}

	return ApplicationBootstrapView{
		Preferences: ApplicationPreferencesView{
			Theme:                    string(prefs.Theme),
			DefaultCreativeProfileID: prefs.DefaultCreativeProfileID,
			UpdatedAt:                formatDesktopTime(prefs.UpdatedAt),
		},
		RecentProjects: recentViews,
		Providers:      providerViews,
		Profiles:       profiles,
	}, nil
}

// SaveApplicationPreferences updates the application-level preferences.
func (a *DesktopAPI) SaveApplicationPreferences(input ApplicationPreferencesInput) error {
	if a.settings == nil {
		return safeDesktopError(errFeatureNotAvailable)
	}
	prefs, err := a.settings.Preferences(a.ctx)
	if err != nil {
		return safeDesktopError(err)
	}
	if input.Theme != "" {
		prefs.Theme = appdomain.Theme(input.Theme)
	}
	if input.DefaultCreativeProfileID != "" {
		prefs.DefaultCreativeProfileID = input.DefaultCreativeProfileID
	}
	prefs.UpdatedAt = time.Now().UTC()
	return safeDesktopError(a.settings.SavePreferences(a.ctx, prefs))
}

// SaveApplicationProvider creates or updates a provider connection. The API
// key is stored in the OS credential vault and never persisted in the settings
// database.
func (a *DesktopAPI) SaveApplicationProvider(input ApplicationProviderInput) (ProviderView, error) {
	if a.settings == nil {
		return ProviderView{}, safeDesktopError(errFeatureNotAvailable)
	}
	conn, err := a.settings.SaveProvider(a.ctx, appsettings.ProviderInput{
		Name:    input.Name,
		Kind:    toProviderKind(input.Kind),
		BaseURL: input.BaseURL,
		APIKey:  input.APIKey,
	})
	if err != nil {
		return ProviderView{}, safeDesktopError(err)
	}
	return ProviderView{
		ID:            conn.ID,
		Name:          conn.Name,
		Kind:          string(conn.Kind),
		BaseURL:       conn.BaseURL,
		HasCredential: a.settings.HasCredential(a.ctx, conn),
		CreatedAt:     formatDesktopTime(conn.CreatedAt),
		UpdatedAt:     formatDesktopTime(conn.UpdatedAt),
	}, nil
}

// DeleteApplicationProvider removes a provider connection after verifying it
// is not referenced by any creative profile.
func (a *DesktopAPI) DeleteApplicationProvider(id string) error {
	if a.settings == nil {
		return safeDesktopError(errFeatureNotAvailable)
	}
	return safeDesktopError(a.settings.DeleteProvider(a.ctx, id))
}

// TestApplicationProvider verifies that a credential exists for the provider.
func (a *DesktopAPI) TestApplicationProvider(id string) error {
	if a.settings == nil {
		return safeDesktopError(errFeatureNotAvailable)
	}
	conn, err := a.settings.Providers(a.ctx)
	if err != nil {
		return safeDesktopError(err)
	}
	for _, p := range conn {
		if p.ID == id {
			if a.settings.HasCredential(a.ctx, p) {
				return nil
			}
			return safeDesktopError(errors.New("provider credential not found"))
		}
	}
	return safeDesktopError(appsettings.ErrProviderNotFound)
}

// PublishCreativeProfile publishes a new version of a creative profile.
func (a *DesktopAPI) PublishCreativeProfile(input PublishProfileInput) (CreativeProfileView, error) {
	if a.settings == nil {
		return CreativeProfileView{}, safeDesktopError(errFeatureNotAvailable)
	}
	roles := make([]appsettings.RoleInput, len(input.Roles))
	for i, r := range input.Roles {
		roles[i] = appsettings.RoleInput{
			Role:            toAgentRole(r.Role),
			ConnectionID:    r.ConnectionID,
			Model:           r.Model,
			MaxOutputTokens: r.MaxOutputTokens,
			Temperature:     r.Temperature,
			Enabled:         r.Enabled,
		}
	}
	version, err := a.settings.PublishCreativeProfile(a.ctx, appsettings.ProfileInput{
		ProfileID:       input.ProfileID,
		Name:            input.Name,
		Roles:           roles,
		MaxRunTokens:    input.MaxRunTokens,
		MaxContextChars: input.MaxContextChars,
	})
	if err != nil {
		return CreativeProfileView{}, safeDesktopError(err)
	}
	return creativeProfileView(version), nil
}

// ProjectWorkbench returns the authoritative workbench projection for the
// active project.
func (a *DesktopAPI) ProjectWorkbench() (workspace.WorkbenchProjection, error) {
	return safeResult(a.workspace.Workbench(a.ctx))
}

// CloseProject closes the active project via the shell service when
// configured, otherwise falls back to the workspace service.
func (a *DesktopAPI) CloseProject() error {
	if a.shell != nil {
		return safeDesktopError(a.shell.Close())
	}
	return safeDesktopError(a.workspace.Close())
}

// OpenRecentProject opens a project from the recent list by its project ID.
// When the recorded path no longer exists, the shell marks the entry missing
// and returns PROJECT_RELOCATE_REQUIRED.
func (a *DesktopAPI) OpenRecentProject(projectID string) (ProjectOpenResultView, error) {
	if a.shell == nil {
		return ProjectOpenResultView{}, safeDesktopError(errFeatureNotAvailable)
	}
	result, err := a.shell.OpenRecentProject(a.ctx, projectID)
	if err != nil {
		var conflict *ports.ProjectLeaseConflict
		if errors.As(err, &conflict) {
			return ProjectOpenResultView{
				Outcome: projectOpenOutcomeLeaseConflict,
				Conflict: &ProjectLeaseConflictView{
					Holder:    conflict.Holder,
					Retryable: conflict.Retryable,
				},
			}, nil
		}
		return ProjectOpenResultView{}, safeDesktopError(err)
	}
	return ProjectOpenResultView{
		Outcome: projectOpenOutcomeOpened,
		Project: &result.Project,
		Warning: result.Warning,
	}, nil
}

// formatDesktopTime formats a time value as RFC3339Nano in UTC for JSON
// serialization.
func formatDesktopTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// creativeProfileView converts a domain creative profile version to the safe
// desktop view.
func creativeProfileView(v appdomain.CreativeProfileVersion) CreativeProfileView {
	roles := make([]RoleBindingView, len(v.Roles))
	for i, r := range v.Roles {
		roles[i] = RoleBindingView{
			Role:            string(r.Role),
			ConnectionID:    r.ConnectionID,
			Model:           r.Model,
			MaxOutputTokens: r.MaxOutputTokens,
			Temperature:     r.Temperature,
			Enabled:         r.Enabled,
		}
	}
	return CreativeProfileView{
		ProfileID:       v.ProfileID,
		Version:         v.Version,
		Name:            v.Name,
		Roles:           roles,
		MaxRunTokens:    v.MaxRunTokens,
		MaxContextChars: v.MaxContextChars,
		CreatedAt:       formatDesktopTime(v.CreatedAt),
	}
}

// toProviderKind converts a string kind to the domain type.
func toProviderKind(kind string) agentdomain.ProviderKind {
	return agentdomain.ProviderKind(kind)
}

// toAgentRole converts a string role to the domain type.
func toAgentRole(role string) agentdomain.AgentRole {
	return agentdomain.AgentRole(role)
}

type wailsDirectoryPicker struct {
	openFileDialog func(context.Context, runtime.OpenDialogOptions) (string, error)
}

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

func (p wailsDirectoryPicker) ChooseManuscriptToImport(ctx context.Context) (string, error) {
	openFileDialog := p.openFileDialog
	if openFileDialog == nil {
		openFileDialog = runtime.OpenFileDialog
	}
	return openFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "选择要导入的小说",
		Filters: []runtime.FileFilter{{
			DisplayName: "小说文本 (*.txt, *.md, *.markdown)",
			Pattern:     "*.txt;*.md;*.markdown",
		}},
	})
}
