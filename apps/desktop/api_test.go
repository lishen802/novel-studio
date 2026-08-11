package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/application/workspace"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	importdomain "github.com/lishen802/novel-studio/internal/domain/importing"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestDesktopAPICreateProjectForwardsInput(t *testing.T) {
	want := workspace.ProjectView{ID: "project-1", Title: "长夜织境", Path: "/books/长夜织境.zjproject"}
	workspaces := &fakeWorkspace{createResult: want}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())
	input := CreateProjectInput{
		ParentDirectory: "/books",
		Title:           " 长夜织境 ",
		Genre:           "东方奇幻",
		Summary:         "简介",
	}

	got, err := api.CreateProject(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
	if workspaces.createInput != input {
		t.Fatalf("forwarded input = %#v, want %#v", workspaces.createInput, input)
	}
}

func TestDesktopAPIMapsSharingViolationToSafeCode(t *testing.T) {
	api := newDesktopAPI(&fakeWorkspace{
		commitErr: &ports.ProjectFilesystemFailure{
			Category: ports.ProjectFilesystemSharingViolation,
		},
	}, &fakeDirectoryPicker{})
	api.startup(context.Background())

	_, err := api.CommitChapter("chapter-1", "不应泄露的正文", 1)
	var got *desktopError
	if !errors.As(err, &got) {
		t.Fatalf("CommitChapter() error = %#v, want desktop error", err)
	}
	if got.Code != desktopErrorCode("PROJECT_SHARING_VIOLATION") {
		t.Fatalf("CommitChapter() code = %q", got.Code)
	}
	if containsAny(got.Message, "正文", "filesystem", "sqlite", "/") {
		t.Fatalf("unsafe filesystem error message: %q", got.Message)
	}
}

func TestDesktopAPIOpenProjectUsesStructuredAccessContract(t *testing.T) {
	editable := workspace.ProjectView{
		ID:         "project-1",
		Title:      "长夜织境",
		Path:       "/books/长夜织境.zjproject",
		AccessMode: ports.ProjectAccessEditable,
	}
	readOnly := editable
	readOnly.AccessMode = ports.ProjectAccessReadOnly
	workspaces := &fakeWorkspace{openResult: editable, openReadOnlyResult: readOnly}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	opened, err := api.OpenProject(ProjectOpenInput{
		Path: "/books/长夜织境.zjproject",
		Mode: ProjectOpenModeEditable,
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened.Outcome != projectOpenOutcomeOpened || opened.Project == nil || !reflect.DeepEqual(*opened.Project, editable) {
		t.Fatalf("editable open result = %#v", opened)
	}
	if workspaces.openPath != "/books/长夜织境.zjproject" {
		t.Fatalf("editable path = %q", workspaces.openPath)
	}

	opened, err = api.OpenProject(ProjectOpenInput{
		Path: "/books/长夜织境.zjproject",
		Mode: ProjectOpenModeReadOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	if opened.Outcome != projectOpenOutcomeOpened || opened.Project == nil || !reflect.DeepEqual(*opened.Project, readOnly) {
		t.Fatalf("read-only open result = %#v", opened)
	}
	if workspaces.openReadOnlyPath != "/books/长夜织境.zjproject" {
		t.Fatalf("read-only path = %q", workspaces.openReadOnlyPath)
	}
}

func TestDesktopAPIOpenProjectReturnsLeaseConflictInsteadOfAnError(t *testing.T) {
	startedAt := time.Date(2026, time.August, 10, 8, 30, 0, 0, time.UTC)
	for _, holder := range []*ports.ProjectLeaseMetadata{
		{
			PID:                42,
			Hostname:           "author-mac",
			ApplicationVersion: "1.0.0",
			StartedAt:          startedAt,
			ProjectID:          "project-1",
		},
		nil,
	} {
		t.Run("holder", func(t *testing.T) {
			workspaces := &fakeWorkspace{openErr: &ports.ProjectLeaseConflict{
				PackagePath: "/private/secret/books/长夜织境.zjproject",
				Holder:      holder,
				Retryable:   true,
			}}
			api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
			api.startup(context.Background())

			result, err := api.OpenProject(ProjectOpenInput{
				Path: "/books/长夜织境.zjproject",
				Mode: ProjectOpenModeEditable,
			})
			if err != nil {
				t.Fatalf("OpenProject() error = %v", err)
			}
			if result.Outcome != projectOpenOutcomeLeaseConflict || result.Conflict == nil {
				t.Fatalf("result = %#v", result)
			}
			if result.Conflict.Path != "/books/长夜织境.zjproject" || result.Conflict.Retryable != true {
				t.Fatalf("conflict = %#v", result.Conflict)
			}
			if !reflect.DeepEqual(result.Conflict.Holder, holder) {
				t.Fatalf("holder = %#v, want %#v", result.Conflict.Holder, holder)
			}
		})
	}
}

func TestDesktopAPIOpenProjectRejectsUnknownAccessMode(t *testing.T) {
	api := newDesktopAPI(&fakeWorkspace{}, &fakeDirectoryPicker{})
	api.startup(context.Background())

	_, err := api.OpenProject(ProjectOpenInput{Path: "/books/长夜织境.zjproject", Mode: "preview"})
	var safe *desktopError
	if !errors.As(err, &safe) || safe.Code != errorValidationFailed {
		t.Fatalf("OpenProject() error = %#v, want validation error", err)
	}
}

func TestDesktopAPICreatesRecoveryCopyWithPartialSuccessContract(t *testing.T) {
	project := workspace.ProjectView{ID: "copy-1", Path: "/copies/长夜-恢复.zjproject", AccessMode: ports.ProjectAccessEditable}
	for _, test := range []struct {
		name   string
		result workspace.RecoveryCopyResult
		err    error
		want   recoveryCopyOutcome
	}{
		{name: "opened", result: workspace.RecoveryCopyResult{Path: project.Path, Project: &project}, want: recoveryCopyOutcomeOpened},
		{name: "published but open failed", result: workspace.RecoveryCopyResult{Path: project.Path}, err: errors.New("open published recovery copy: internal failure"), want: recoveryCopyOutcomePublished},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspaces := &fakeWorkspace{recoveryCopyResult: test.result, recoveryCopyErr: test.err}
			api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
			api.startup(context.Background())

			got, err := api.CreateProjectRecoveryCopy(RecoveryCopyInput{
				SourcePath:      "/books/长夜织境.zjproject",
				ParentDirectory: "/copies",
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Outcome != test.want || got.Path != project.Path {
				t.Fatalf("result = %#v", got)
			}
			if test.want == recoveryCopyOutcomeOpened && (got.Project == nil || !reflect.DeepEqual(*got.Project, project)) {
				t.Fatalf("opened project = %#v", got.Project)
			}
			if workspaces.recoveryCopyInput != (recoveryCopyCall{sourcePath: "/books/长夜织境.zjproject", parentDirectory: "/copies"}) {
				t.Fatalf("recovery copy input = %#v", workspaces.recoveryCopyInput)
			}
		})
	}
}

func TestDesktopAPIForwardsChapterOperations(t *testing.T) {
	chapter := workspace.ChapterView{ID: "chapter-1", Title: "第一章", Revision: 1}
	committed := workspace.ChapterView{ID: "chapter-1", Title: "第一章", Body: "正文", Revision: 2}
	workspaces := &fakeWorkspace{
		listResult:          []workspace.ChapterView{chapter},
		createChapterResult: chapter,
		openChapterResult:   chapter,
		commitResult:        committed,
	}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	listed, err := api.ListChapters()
	if err != nil || !reflect.DeepEqual(listed, []workspace.ChapterView{chapter}) {
		t.Fatalf("ListChapters = (%#v, %v)", listed, err)
	}
	created, err := api.CreateChapter("第一章")
	if err != nil || created != chapter || workspaces.createdChapterTitle != "第一章" {
		t.Fatalf("CreateChapter = (%#v, %v), title = %q", created, err, workspaces.createdChapterTitle)
	}
	opened, err := api.OpenChapter(chapter.ID)
	if err != nil || opened != chapter || workspaces.openedChapterID != chapter.ID {
		t.Fatalf("OpenChapter = (%#v, %v), id = %q", opened, err, workspaces.openedChapterID)
	}
	if err := api.SaveDraft(chapter.ID, "草稿", 1); err != nil {
		t.Fatal(err)
	}
	if workspaces.savedDraft != (draftCall{id: chapter.ID, body: "草稿", revision: 1}) {
		t.Fatalf("saved draft = %#v", workspaces.savedDraft)
	}
	if err := api.DiscardDraft(chapter.ID, chapter.Revision); err != nil {
		t.Fatal(err)
	}
	if workspaces.discardedDraftID != chapter.ID {
		t.Fatalf("discarded draft id = %q", workspaces.discardedDraftID)
	}
	if workspaces.discardedDraftRevision != chapter.Revision {
		t.Fatalf("discarded draft revision = %d", workspaces.discardedDraftRevision)
	}
	gotCommit, err := api.CommitChapter(chapter.ID, "正文", 1)
	if err != nil || gotCommit != committed {
		t.Fatalf("CommitChapter = (%#v, %v)", gotCommit, err)
	}
	if workspaces.committed != (draftCall{id: chapter.ID, body: "正文", revision: 1}) {
		t.Fatalf("commit call = %#v", workspaces.committed)
	}
}

func TestDesktopAPIForwardsReviewableChangeSetOperations(t *testing.T) {
	changeSet := workspace.ChangeSetView{ID: "change-set-1", Title: "重写开场", Status: "pending"}
	chapter := workspace.ChapterView{ID: "chapter-1", Body: "新正文", Revision: 2}
	applyResult := workspace.ChangeSetApplyView{ChangeSet: changeSet, Chapters: []workspace.ChapterView{chapter}}
	workspaces := &fakeWorkspace{
		changeSetsResult:  []workspace.ChangeSetView{changeSet},
		applyChangeResult: applyResult,
	}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())
	input := workspace.ChapterChangeInput{
		ChapterID:    chapter.ID,
		Title:        changeSet.Title,
		ProposedBody: chapter.Body,
		CreatedBy:    "agent:writer",
	}

	listed, err := api.ChangeSets()
	if err != nil || !reflect.DeepEqual(listed, []workspace.ChangeSetView{changeSet}) {
		t.Fatalf("ChangeSets = (%#v, %v)", listed, err)
	}
	if _, err := api.ProposeChapterChange(input); err != nil || workspaces.chapterChangeInput != input {
		t.Fatalf("ProposeChapterChange input = %#v, error = %v", workspaces.chapterChangeInput, err)
	}
	applied, err := api.ApplyChangeSet(changeSet.ID)
	if err != nil || !reflect.DeepEqual(applied, applyResult) || workspaces.appliedChangeSetID != changeSet.ID {
		t.Fatalf("ApplyChangeSet = (%#v, %v), id = %q", applied, err, workspaces.appliedChangeSetID)
	}
	if err := api.RejectChangeSet(changeSet.ID); err != nil || workspaces.rejectedChangeSetID != changeSet.ID {
		t.Fatalf("RejectChangeSet id = %q, error = %v", workspaces.rejectedChangeSetID, err)
	}
}

func TestDesktopAPIForwardsVersionHistoryAndSnapshotOperations(t *testing.T) {
	history := []workspace.ChapterRevisionView{{
		ChapterID: "chapter-1",
		Revision:  2,
		Body:      "当前正文",
		IsCurrent: true,
	}}
	snapshot := workspace.SnapshotView{ID: "snapshot-1", Name: "第一幕定稿", Kind: "named"}
	restore := workspace.SnapshotRestoreView{
		SnapshotID: snapshot.ID,
		Chapters:   []workspace.ChapterView{{ID: "chapter-1", Body: "快照正文", Revision: 2}},
	}
	workspaces := &fakeWorkspace{
		chapterHistoryResult:  history,
		snapshotsResult:       []workspace.SnapshotView{snapshot},
		createSnapshotResult:  snapshot,
		restoreSnapshotResult: restore,
		restoreRevisionResult: workspace.ChapterView{
			ID:       "chapter-1",
			Body:     "历史正文",
			Revision: 3,
		},
	}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	gotHistory, err := api.ChapterHistory("chapter-1")
	if err != nil || !reflect.DeepEqual(gotHistory, history) {
		t.Fatalf("ChapterHistory = (%#v, %v)", gotHistory, err)
	}
	restoredChapter, err := api.RestoreChapterRevision("chapter-1", 1, 2)
	if err != nil || restoredChapter != workspaces.restoreRevisionResult {
		t.Fatalf("RestoreChapterRevision = (%#v, %v)", restoredChapter, err)
	}
	if workspaces.restoreRevisionArgs != [2]int64{1, 2} ||
		workspaces.restoreRevisionChapterID != "chapter-1" {
		t.Fatalf("restore revision args = (%q, %#v)", workspaces.restoreRevisionChapterID, workspaces.restoreRevisionArgs)
	}
	gotSnapshots, err := api.Snapshots()
	if err != nil || !reflect.DeepEqual(gotSnapshots, []workspace.SnapshotView{snapshot}) {
		t.Fatalf("Snapshots = (%#v, %v)", gotSnapshots, err)
	}
	created, err := api.CreateSnapshot("第一幕定稿")
	if err != nil || created != snapshot || workspaces.snapshotName != "第一幕定稿" {
		t.Fatalf("CreateSnapshot = (%#v, %v), name = %q", created, err, workspaces.snapshotName)
	}
	restored, err := api.RestoreSnapshot(snapshot.ID)
	if err != nil || !reflect.DeepEqual(restored, restore) ||
		workspaces.restoreSnapshotID != snapshot.ID {
		t.Fatalf("RestoreSnapshot = (%#v, %v), id = %q", restored, err, workspaces.restoreSnapshotID)
	}
}

func TestDesktopAPIForwardsProjectBranchOperations(t *testing.T) {
	branch := workspace.BranchView{ID: "branch-1", Name: "黑化线", ParentID: "main"}
	comparison := workspace.BranchComparisonView{
		Source:           branch,
		Target:           workspace.BranchView{ID: "main", Name: "主分支"},
		ModifiedChapters: 1,
	}
	preview := workspace.BranchMergePreviewView{
		Token:      "merge-token",
		SourceID:   branch.ID,
		TargetID:   "main",
		Comparison: comparison,
	}
	switched := workspace.BranchSwitchView{
		ActiveBranchID: branch.ID,
		Chapters:       []workspace.ChapterView{{ID: "chapter-1"}},
	}
	workspaces := &fakeWorkspace{
		branchesResult:      []workspace.BranchView{branch},
		createBranchResult:  branch,
		switchBranchResult:  switched,
		compareBranchResult: comparison,
		mergePreviewResult:  preview,
		mergeConfirmResult:  switched,
	}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	if got, err := api.Branches(); err != nil || !reflect.DeepEqual(got, workspaces.branchesResult) {
		t.Fatalf("Branches() = (%#v, %v)", got, err)
	}
	input := CreateBranchInput{Name: branch.Name, Description: "主角接受魔契"}
	if got, err := api.CreateBranch(input); err != nil || got != branch ||
		workspaces.createBranchInput != input {
		t.Fatalf("CreateBranch() = (%#v, %v), input %#v", got, err, workspaces.createBranchInput)
	}
	if got, err := api.SwitchBranch(branch.ID); err != nil || !reflect.DeepEqual(got, switched) ||
		workspaces.switchBranchID != branch.ID {
		t.Fatalf("SwitchBranch() = (%#v, %v), id %q", got, err, workspaces.switchBranchID)
	}
	if got, err := api.CompareBranches(branch.ID, "main"); err != nil ||
		!reflect.DeepEqual(got, comparison) {
		t.Fatalf("CompareBranches() = (%#v, %v)", got, err)
	}
	if got, err := api.PreviewBranchMerge(branch.ID); err != nil ||
		!reflect.DeepEqual(got, preview) {
		t.Fatalf("PreviewBranchMerge() = (%#v, %v)", got, err)
	}
	if got, err := api.ConfirmBranchMerge(preview.Token); err != nil ||
		!reflect.DeepEqual(got, switched) {
		t.Fatalf("ConfirmBranchMerge() = (%#v, %v)", got, err)
	}
	if err := api.DeleteBranch(branch.ID); err != nil || workspaces.deletedBranchID != branch.ID {
		t.Fatalf("DeleteBranch() id = %q, error = %v", workspaces.deletedBranchID, err)
	}
}

func TestDesktopAPIForwardsStoryStructureOperations(t *testing.T) {
	structure := workspace.StoryStructureView{OutlineNodes: []workspace.OutlineNodeView{{ID: "book", Title: "全书大纲"}}}
	workspaces := &fakeWorkspace{storyStructureResult: structure}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	got, err := api.StoryStructure()
	if err != nil || !reflect.DeepEqual(got, structure) {
		t.Fatalf("StoryStructure = (%#v, %v)", got, err)
	}
	outlineInput := workspace.CreateOutlineNodeInput{Level: "book", Title: "全书大纲"}
	if _, err := api.CreateOutlineNode(outlineInput); err != nil || workspaces.outlineInput != outlineInput {
		t.Fatalf("CreateOutlineNode input = %#v, error = %v", workspaces.outlineInput, err)
	}
	lineInput := workspace.CreateStoryLineInput{Name: "主线"}
	if _, err := api.CreateStoryLine(lineInput); err != nil || workspaces.storyLineInput != lineInput {
		t.Fatalf("CreateStoryLine input = %#v, error = %v", workspaces.storyLineInput, err)
	}
	linkInput := workspace.LinkStoryLineInput{StoryLineID: "line", OutlineNodeID: "book"}
	if _, err := api.LinkStoryLine(linkInput); err != nil || workspaces.storyLinkInput != linkInput {
		t.Fatalf("LinkStoryLine input = %#v, error = %v", workspaces.storyLinkInput, err)
	}
	cardInput := workspace.CreateSceneCardInput{ChapterID: "chapter", OutlineNodeID: "scene", Title: "场景"}
	if _, err := api.CreateSceneCard(cardInput); err != nil || workspaces.sceneCardInput != cardInput {
		t.Fatalf("CreateSceneCard input = %#v, error = %v", workspaces.sceneCardInput, err)
	}
	if err := api.ArchiveOutlineNode("book"); err != nil || workspaces.archivedOutlineID != "book" {
		t.Fatalf("ArchiveOutlineNode id = %q, error = %v", workspaces.archivedOutlineID, err)
	}
	if err := api.MoveOutlineNode("book", "down"); err != nil || workspaces.movedOutlineID != "book" || workspaces.moveDirection != "down" {
		t.Fatalf("MoveOutlineNode = (%q, %q, %v)", workspaces.movedOutlineID, workspaces.moveDirection, err)
	}
}

func TestDesktopAPIForwardsStoryWorldOperations(t *testing.T) {
	world := workspace.StoryWorldView{Characters: []workspace.CharacterView{{ID: "character", Name: "林默"}}}
	workspaces := &fakeWorkspace{storyWorldResult: world}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	got, err := api.StoryWorld()
	if err != nil || !reflect.DeepEqual(got, world) {
		t.Fatalf("StoryWorld = (%#v, %v)", got, err)
	}
	characterInput := workspace.CharacterInput{Name: "林默", Aliases: []string{"阿默"}, Role: "protagonist"}
	if _, err := api.CreateCharacter(characterInput); err != nil || !reflect.DeepEqual(workspaces.characterInput, characterInput) {
		t.Fatalf("CreateCharacter input = %#v, error = %v", workspaces.characterInput, err)
	}
	if _, err := api.UpdateCharacter("character", characterInput); err != nil || workspaces.updatedCharacterID != "character" {
		t.Fatalf("UpdateCharacter id = %q, error = %v", workspaces.updatedCharacterID, err)
	}
	if err := api.ArchiveCharacter("character"); err != nil || workspaces.archivedCharacterID != "character" {
		t.Fatalf("ArchiveCharacter id = %q, error = %v", workspaces.archivedCharacterID, err)
	}
	factionInput := workspace.FactionInput{Name: "守夜人", Goal: "守护港城"}
	if _, err := api.CreateFaction(factionInput); err != nil || workspaces.factionInput != factionInput {
		t.Fatalf("CreateFaction input = %#v, error = %v", workspaces.factionInput, err)
	}
	if _, err := api.UpdateFaction("faction", factionInput); err != nil || workspaces.updatedFactionID != "faction" {
		t.Fatalf("UpdateFaction id = %q, error = %v", workspaces.updatedFactionID, err)
	}
	if err := api.ArchiveFaction("faction"); err != nil || workspaces.archivedFactionID != "faction" {
		t.Fatalf("ArchiveFaction id = %q, error = %v", workspaces.archivedFactionID, err)
	}
	membershipInput := workspace.MembershipInput{CharacterID: "character", FactionID: "faction", Role: "调查员", Status: "active"}
	if _, err := api.UpsertMembership(membershipInput); err != nil || workspaces.membershipInput != membershipInput {
		t.Fatalf("UpsertMembership input = %#v, error = %v", workspaces.membershipInput, err)
	}
	if err := api.DeleteMembership("character", "faction"); err != nil || workspaces.deletedMembership != "character:faction" {
		t.Fatalf("DeleteMembership = %q, error = %v", workspaces.deletedMembership, err)
	}
	relationshipInput := workspace.RelationshipInput{SourceCharacterID: "character", TargetCharacterID: "other", Kind: "ally"}
	if _, err := api.CreateRelationship(relationshipInput); err != nil || workspaces.relationshipInput != relationshipInput {
		t.Fatalf("CreateRelationship input = %#v, error = %v", workspaces.relationshipInput, err)
	}
	if err := api.ArchiveRelationship("relationship"); err != nil || workspaces.archivedRelationshipID != "relationship" {
		t.Fatalf("ArchiveRelationship id = %q, error = %v", workspaces.archivedRelationshipID, err)
	}
	appearanceInput := workspace.AppearanceInput{CharacterID: "character", TargetID: "chapter", Role: "pov"}
	if err := api.LinkCharacterToChapter(appearanceInput); err != nil || workspaces.chapterAppearanceInput != appearanceInput {
		t.Fatalf("LinkCharacterToChapter input = %#v, error = %v", workspaces.chapterAppearanceInput, err)
	}
	if err := api.UnlinkCharacterFromChapter("character", "chapter"); err != nil || workspaces.unlinkedChapter != "character:chapter" {
		t.Fatalf("UnlinkCharacterFromChapter = %q, error = %v", workspaces.unlinkedChapter, err)
	}
	appearanceInput.TargetID = "scene"
	if err := api.LinkCharacterToScene(appearanceInput); err != nil || workspaces.sceneAppearanceInput != appearanceInput {
		t.Fatalf("LinkCharacterToScene input = %#v, error = %v", workspaces.sceneAppearanceInput, err)
	}
	if err := api.UnlinkCharacterFromScene("character", "scene"); err != nil || workspaces.unlinkedScene != "character:scene" {
		t.Fatalf("UnlinkCharacterFromScene = %q, error = %v", workspaces.unlinkedScene, err)
	}
}

func TestDesktopAPIForwardsWorldAssetOperations(t *testing.T) {
	assets := workspace.WorldAssetsView{Regions: []workspace.WorldRegionView{{ID: "region-root", Name: "镜海世界"}}}
	workspaces := &fakeWorkspace{worldAssetsResult: assets}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	got, err := api.WorldAssets()
	if err != nil || !reflect.DeepEqual(got, assets) {
		t.Fatalf("WorldAssets = (%#v, %v)", got, err)
	}
	regionInput := workspace.WorldRegionInput{Kind: "world", Name: "镜海世界", Direction: "center"}
	if _, err := api.CreateWorldRegion(regionInput); err != nil || workspaces.worldRegionInput != regionInput {
		t.Fatalf("CreateWorldRegion input = %#v, error = %v", workspaces.worldRegionInput, err)
	}
	locationInput := workspace.LocationInput{RegionID: "region-root", Kind: "city", Name: "雾港城"}
	if _, err := api.UpdateLocation("location-1", locationInput); err != nil || workspaces.updatedLocationID != "location-1" {
		t.Fatalf("UpdateLocation id = %q, error = %v", workspaces.updatedLocationID, err)
	}
	transferInput := workspace.ItemTransferInput{ItemID: "item-1", HolderType: "location", HolderID: "location-1"}
	if _, err := api.TransferItem(transferInput); err != nil || workspaces.itemTransferInput != transferInput {
		t.Fatalf("TransferItem input = %#v, error = %v", workspaces.itemTransferInput, err)
	}
	ruleInput := workspace.WorldRuleInput{Name: "禁空", Description: "无法飞行", ScopeType: "project"}
	if _, err := api.CreateWorldRule(ruleInput); err != nil || workspaces.worldRuleInput != ruleInput {
		t.Fatalf("CreateWorldRule input = %#v, error = %v", workspaces.worldRuleInput, err)
	}
	hookInput := workspace.ForeshadowingHookInput{Title: "断剑之谜", Setup: "获得断剑", PlantedChapterID: "chapter-1"}
	if _, err := api.UpdateForeshadowingHook("hook-1", hookInput); err != nil || workspaces.updatedHookID != "hook-1" {
		t.Fatalf("UpdateForeshadowingHook id = %q, error = %v", workspaces.updatedHookID, err)
	}
	if err := api.ArchiveWorldRegion("region-root"); err != nil || workspaces.archivedWorldRegionID != "region-root" {
		t.Fatalf("ArchiveWorldRegion id = %q, error = %v", workspaces.archivedWorldRegionID, err)
	}
}

func TestDesktopAPIForwardsContinuityOperations(t *testing.T) {
	continuity := workspace.ContinuityView{Events: []workspace.StoryEventView{{ID: "event-1", Title: "雾港失火"}}}
	workspaces := &fakeWorkspace{continuityResult: continuity}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	got, err := api.Continuity()
	if err != nil || !reflect.DeepEqual(got, continuity) {
		t.Fatalf("Continuity = (%#v, %v)", got, err)
	}
	eventInput := workspace.StoryEventInput{Title: "雾港失火", StoryOrder: 10, NarrativeOrder: 1, Status: "occurred"}
	if _, err := api.CreateStoryEvent(eventInput); err != nil || workspaces.storyEventInput != eventInput {
		t.Fatalf("CreateStoryEvent input = %#v, error = %v", workspaces.storyEventInput, err)
	}
	if _, err := api.UpdateStoryEvent("event-1", eventInput); err != nil || workspaces.updatedStoryEventID != "event-1" {
		t.Fatalf("UpdateStoryEvent id = %q, error = %v", workspaces.updatedStoryEventID, err)
	}
	linkInput := workspace.StoryEventLinkInput{EventID: "event-1", TargetType: "chapter", TargetID: "chapter-1", Role: "发生章节"}
	if _, err := api.LinkStoryEvent(linkInput); err != nil || workspaces.storyEventLinkInput != linkInput {
		t.Fatalf("LinkStoryEvent input = %#v, error = %v", workspaces.storyEventLinkInput, err)
	}
	if err := api.UnlinkStoryEvent("event-1", "chapter", "chapter-1"); err != nil || workspaces.unlinkedStoryEvent != "event-1:chapter:chapter-1" {
		t.Fatalf("UnlinkStoryEvent = %q, error = %v", workspaces.unlinkedStoryEvent, err)
	}
	factInput := workspace.FactInput{Text: "雾港城禁止御剑", Origin: "author", Status: "author_confirmed", Confidence: 1}
	if _, err := api.CreateFact(factInput); err != nil || workspaces.factInput != factInput {
		t.Fatalf("CreateFact input = %#v, error = %v", workspaces.factInput, err)
	}
	if _, err := api.UpdateFact("fact-1", factInput); err != nil || workspaces.updatedFactID != "fact-1" {
		t.Fatalf("UpdateFact id = %q, error = %v", workspaces.updatedFactID, err)
	}
	if _, err := api.ConfirmFact("fact-1"); err != nil || workspaces.confirmedFactID != "fact-1" {
		t.Fatalf("ConfirmFact id = %q, error = %v", workspaces.confirmedFactID, err)
	}
	evidenceInput := workspace.FactEvidenceInput{FactID: "fact-1", ChapterID: "chapter-1", Quote: "不可御剑", StartOffset: 3, EndOffset: 7, ChapterRevision: 2}
	if _, err := api.AddFactEvidence(evidenceInput); err != nil || workspaces.factEvidenceInput != evidenceInput {
		t.Fatalf("AddFactEvidence input = %#v, error = %v", workspaces.factEvidenceInput, err)
	}
	if err := api.RemoveFactEvidence("evidence-1"); err != nil || workspaces.removedFactEvidenceID != "evidence-1" {
		t.Fatalf("RemoveFactEvidence id = %q, error = %v", workspaces.removedFactEvidenceID, err)
	}
	scopeInput := workspace.FactScopeInput{FactID: "fact-1", TargetType: "location", TargetID: "location-1"}
	if _, err := api.AddFactScope(scopeInput); err != nil || workspaces.factScopeInput != scopeInput {
		t.Fatalf("AddFactScope input = %#v, error = %v", workspaces.factScopeInput, err)
	}
	conflictInput := workspace.FactConflictInput{FactID: "fact-1", OtherFactID: "fact-2", Reason: "规则互斥"}
	if _, err := api.CreateFactConflict(conflictInput); err != nil || workspaces.factConflictInput != conflictInput {
		t.Fatalf("CreateFactConflict input = %#v, error = %v", workspaces.factConflictInput, err)
	}
	if _, err := api.ResolveFactConflict("conflict-1"); err != nil || workspaces.resolvedFactConflictID != "conflict-1" {
		t.Fatalf("ResolveFactConflict id = %q, error = %v", workspaces.resolvedFactConflictID, err)
	}
	if err := api.ArchiveStoryEvent("event-1"); err != nil || workspaces.archivedStoryEventID != "event-1" {
		t.Fatalf("ArchiveStoryEvent id = %q, error = %v", workspaces.archivedStoryEventID, err)
	}
	if err := api.ArchiveFact("fact-1"); err != nil || workspaces.archivedFactID != "fact-1" {
		t.Fatalf("ArchiveFact id = %q, error = %v", workspaces.archivedFactID, err)
	}
}

func TestDesktopAPIDirectoryCancellationReturnsEmpty(t *testing.T) {
	api := newDesktopAPI(&fakeWorkspace{}, &fakeDirectoryPicker{})
	api.startup(context.Background())

	parent, err := api.ChooseProjectParent()
	if err != nil || parent != "" {
		t.Fatalf("ChooseProjectParent = (%q, %v)", parent, err)
	}
	projectPath, err := api.ChooseProjectToOpen()
	if err != nil || projectPath != "" {
		t.Fatalf("ChooseProjectToOpen = (%q, %v)", projectPath, err)
	}
	importPath, err := api.ChooseManuscriptToImport()
	if err != nil || importPath != "" {
		t.Fatalf("ChooseManuscriptToImport = (%q, %v)", importPath, err)
	}
}

func TestDesktopAPIForwardsManuscriptImportOperations(t *testing.T) {
	job := workspace.ImportJobView{
		ID:               "import-1",
		Status:           "prepared",
		SourceName:       "长夜.md",
		SourceHashDigest: "0123456789ab",
	}
	applied := workspace.ImportApplyView{
		Job:      workspace.ImportJobView{ID: job.ID, Status: "applied"},
		Chapters: []workspace.ChapterView{{ID: "chapter-1", Title: "第一章"}},
	}
	workspaces := &fakeWorkspace{
		importPrepareResult: workspace.ImportPrepareResult{Job: job},
		importJobsResult:    []workspace.ImportJobView{job},
		importJobResult:     job,
		importApplyResult:   applied,
	}
	picker := &fakeDirectoryPicker{importPath: "/books/长夜.md"}
	api := newDesktopAPI(workspaces, picker)
	api.startup(context.Background())

	sourcePath, err := api.ChooseManuscriptToImport()
	if err != nil || sourcePath != "/books/长夜.md" {
		t.Fatalf("ChooseManuscriptToImport = (%q, %v)", sourcePath, err)
	}
	prepared, err := api.PrepareManuscriptImport(sourcePath)
	if err != nil || !reflect.DeepEqual(prepared, workspaces.importPrepareResult) || workspaces.preparedImportPath != sourcePath {
		t.Fatalf("PrepareManuscriptImport = (%#v, %v), path %q", prepared, err, workspaces.preparedImportPath)
	}
	jobs, err := api.ImportJobs()
	if err != nil || !reflect.DeepEqual(jobs, workspaces.importJobsResult) {
		t.Fatalf("ImportJobs = (%#v, %v)", jobs, err)
	}
	detail, err := api.ImportJob(job.ID)
	if err != nil || !reflect.DeepEqual(detail, job) || workspaces.importJobID != job.ID {
		t.Fatalf("ImportJob = (%#v, %v), id %q", detail, err, workspaces.importJobID)
	}
	selected := []string{"candidate-2", "candidate-1"}
	gotApply, err := api.ApplyImportJob(job.ID, selected)
	if err != nil || !reflect.DeepEqual(gotApply, applied) {
		t.Fatalf("ApplyImportJob = (%#v, %v)", gotApply, err)
	}
	if workspaces.appliedImportJobID != job.ID || !reflect.DeepEqual(workspaces.appliedImportChapterIDs, selected) {
		t.Fatalf("ApplyImportJob args = (%q, %#v)", workspaces.appliedImportJobID, workspaces.appliedImportChapterIDs)
	}
	if err := api.DeleteImportJob(job.ID); err != nil || workspaces.deletedImportJobID != job.ID {
		t.Fatalf("DeleteImportJob = (%q, %v)", workspaces.deletedImportJobID, err)
	}
}

func TestWailsDirectoryPickerUsesFilteredFileDialogForManuscripts(t *testing.T) {
	var options runtime.OpenDialogOptions
	picker := wailsDirectoryPicker{
		openFileDialog: func(_ context.Context, got runtime.OpenDialogOptions) (string, error) {
			options = got
			return "/books/长夜.md", nil
		},
	}

	got, err := picker.ChooseManuscriptToImport(context.Background())

	if err != nil || got != "/books/长夜.md" {
		t.Fatalf("ChooseManuscriptToImport = (%q, %v)", got, err)
	}
	if options.Title != "选择要导入的小说" {
		t.Fatalf("dialog title = %q", options.Title)
	}
	if len(options.Filters) != 1 ||
		options.Filters[0].Pattern != "*.txt;*.md;*.markdown" {
		t.Fatalf("dialog filters = %#v", options.Filters)
	}
}

func TestDesktopAPIMapsWorkspaceErrorToSafeDesktopError(t *testing.T) {
	internal := errors.New("SELECT body FROM chapters failed: secret text")
	api := newDesktopAPI(
		&fakeWorkspace{openErr: internal},
		&fakeDirectoryPicker{},
	)
	api.startup(context.Background())

	_, err := api.OpenProject(ProjectOpenInput{
		Path: "/books/broken.zjproject",
		Mode: ProjectOpenModeEditable,
	})
	var safe *desktopError
	if !errors.As(err, &safe) || safe.Code != errorOperationFailed {
		t.Fatalf("error = %#v, want safe operation error", err)
	}
	if got := err.Error(); got == "" || containsAny(got, "SELECT", "secret text") {
		t.Fatalf("unsafe error was exposed: %q", got)
	}
}

func TestSafeDesktopErrorClassifiesManuscriptImportFailures(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		code    desktopErrorCode
		message string
	}{
		{"format", ports.ErrUnsupportedImportFormat, "IMPORT_FORMAT_UNSUPPORTED", "仅支持 TXT、MD 或 MARKDOWN 小说文件。"},
		{"empty", ports.ErrEmptyImportSource, "IMPORT_SOURCE_EMPTY", "所选小说文件为空。"},
		{"too large", ports.ErrImportSourceTooLarge, "IMPORT_SOURCE_TOO_LARGE", "小说文件超过 20 MiB 上限。"},
		{"encoding", ports.ErrUnsupportedImportEncoding, "IMPORT_ENCODING_UNSUPPORTED", "无法识别小说文件编码，请转换为 UTF-8、UTF-16 或 GB18030。"},
		{"unsafe", ports.ErrUnsafeImportSource, "IMPORT_SOURCE_UNSAFE", "所选来源不是安全的普通文件。"},
		{"duplicate", ports.ErrDuplicateImportSource, "IMPORT_SOURCE_DUPLICATE", "这份小说已在当前项目中准备过。"},
		{"missing", ports.ErrImportJobMissing, "IMPORT_JOB_MISSING", "导入任务不存在或已被删除。"},
		{"applied", ports.ErrImportJobAlreadyApplied, "IMPORT_JOB_APPLIED", "该导入任务已经应用，不能重复操作。"},
		{"not prepared", ports.ErrImportJobNotPrepared, "IMPORT_JOB_NOT_PREPARED", "该导入任务尚未准备完成。"},
		{"selection", ports.ErrNoImportChaptersSelected, "IMPORT_SELECTION_REQUIRED", "请至少选择一个章节。"},
		{"mapping", ports.ErrImportChapterMappingMismatch, "IMPORT_MAPPING_INVALID", "所选章节与导入预览不一致，请刷新后重试。"},
		{"archive unsupported", ports.ErrImportArchiveUnsupported, "IMPORT_ARCHIVE_UNSUPPORTED", "当前文件系统不支持安全归档，请换到本机磁盘后重试。"},
		{"archive cleanup", workspace.ErrImportArchiveCleanup, "IMPORT_ARCHIVE_CLEANUP_FAILED", "导入未完成，且暂存文件清理失败。"},
		{"active", workspace.ErrImportOperationActive, "IMPORT_ACTIVE", "另一个导入操作正在进行，请稍后重试。"},
		{"applied refresh", workspace.ErrImportAppliedRefresh, "IMPORT_APPLIED_REFRESH_FAILED", "章节已导入，但工作区刷新失败，请重新打开项目。"},
		{"workspace read", workspace.ErrImportWorkspaceRead, "IMPORT_WORKSPACE_READ_FAILED", "无法读取导入工作区，请重新打开项目。"},
		{"generic archive", workspace.ErrImportSourceArchive, "IMPORT_SOURCE_PREPARE_FAILED", "无法安全准备小说文件，请检查文件后重试。"},
		{"store unavailable", workspace.ErrImportStoreUnavailable, "IMPORT_UNAVAILABLE", "当前项目不支持小说导入，请升级或重新打开项目。"},
		{"project mismatch", workspace.ErrImportJobProjectMismatch, "IMPORT_JOB_MISSING", "导入任务不存在或已被删除。"},
		{"invalid source metadata", importdomain.ErrInvalidSourceHash, "IMPORT_DATA_INVALID", "导入数据无效，请重新选择小说文件。"},
		{"persistence", ports.ErrImportPersistence, "IMPORT_PERSISTENCE_FAILED", "无法保存导入任务，请检查项目写入权限。"},
		{"cancelled", context.Canceled, "OPERATION_CANCELLED", "操作已取消。"},
		{"deadline", context.DeadlineExceeded, "OPERATION_TIMEOUT", "操作超时，请重试。"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var safe *desktopError
			err := safeDesktopError(errors.Join(test.err, errors.New("SELECT /Users/author/private.md")))
			if !errors.As(err, &safe) {
				t.Fatalf("safeDesktopError(%v) = %#v", test.err, err)
			}
			if safe.Code != test.code || safe.Message != test.message {
				t.Fatalf("safe error = %#v", safe)
			}
			if containsAny(err.Error(), "SELECT /Users", "/Users/author") {
				t.Fatalf("unsafe detail leaked: %q", err)
			}
		})
	}
}

func TestDesktopAPIMapsBranchConflictsAndGuardsToActionableMessages(t *testing.T) {
	tests := []struct {
		err     error
		code    desktopErrorCode
		message string
	}{
		{revisiondomain.ErrBranchDiverged, errorBranchConflict, "目标分支在分叉后也发生了变化，暂时无法自动合并。"},
		{revisiondomain.ErrBranchUnchanged, errorValidationFailed, "这个分支没有可合并的变化。"},
		{revisiondomain.ErrStaleBranchMerge, errorBranchConflict, "合并预览已过期，请重新预览后再确认。"},
		{branchfs.ErrBranchHeadChanged, errorProjectCorrupt, "剧情分支文件已被外部修改或损坏，请从完整项目备份恢复。"},
		{branchfs.ErrProtectedBranch, errorValidationFailed, "主分支或当前分支不能删除。"},
		{branchfs.ErrBranchHasChildren, errorValidationFailed, "该分支仍有子分支，请先处理子分支。"},
	}
	for _, test := range tests {
		var safe *desktopError
		err := safeDesktopError(test.err)
		if !errors.As(err, &safe) || safe.Code != test.code || safe.Message != test.message {
			t.Fatalf("safeDesktopError(%v) = %#v", test.err, safe)
		}
	}
}

func TestSafeDesktopErrorClassifiesRecoveryFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code desktopErrorCode
	}{
		{name: "missing", err: os.ErrNotExist, code: errorProjectNotFound},
		{name: "permission", err: os.ErrPermission, code: errorPermissionDenied},
		{name: "corrupt", err: sqlite.ErrDatabaseCorrupt, code: errorProjectCorrupt},
		{name: "too new", err: sqlite.ErrDatabaseTooNew, code: errorProjectTooNew},
		{name: "revision", err: projectdomain.ErrRevisionConflict, code: errorRevisionConflict},
		{name: "write", err: sqlite.ErrDatabaseWrite, code: errorWriteFailed},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var safe *desktopError
			if err := safeDesktopError(test.err); !errors.As(err, &safe) || safe.Code != test.code {
				t.Fatalf("error = %#v, want code %q", err, test.code)
			}
		})
	}
}

func TestSafeDesktopErrorClassifiesProjectLifecycleFailuresWithoutLeakingDetails(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code desktopErrorCode
	}{
		{name: "read only", err: workspace.ErrWorkspaceReadOnly, code: errorProjectReadOnly},
		{name: "migration", err: sqlite.ErrDatabaseMigrationRequired, code: errorProjectMigration},
		{name: "recovery", err: workspace.ErrProjectRecoveryRequired, code: errorProjectRecovery},
		{name: "source changed", err: workspace.ErrRecoveryCopySourceChanged, code: errorRecoveryCopyChanged},
		{name: "copy failed", err: errors.New("create recovery copy: /Users/author/private manuscript"), code: errorRecoveryCopyFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var safe *desktopError
			err := safeDesktopError(test.err)
			if !errors.As(err, &safe) || safe.Code != test.code {
				t.Fatalf("safeDesktopError(%v) = %#v, want %s", test.err, safe, test.code)
			}
			if containsAny(safe.Message, "private", "/Users", "recovery copy:") {
				t.Fatalf("unsafe lifecycle message: %q", safe.Message)
			}
		})
	}
}

func TestSafeDesktopErrorExplainsReferencedWorldEntity(t *testing.T) {
	var safe *desktopError
	err := safeDesktopError(worlddomain.ErrReferenced)
	if !errors.As(err, &safe) {
		t.Fatalf("error = %#v, want desktop error", err)
	}
	if safe.Code != desktopErrorCode("REFERENCE_CONFLICT") || safe.Message != "该内容仍被其他故事资料引用，请先解除关联后再归档。" {
		t.Fatalf("safe error = %#v", safe)
	}
}

func TestSafeDesktopErrorExplainsReferencedStoryEntity(t *testing.T) {
	var safe *desktopError
	err := safeDesktopError(storydomain.ErrReferenced)
	if !errors.As(err, &safe) {
		t.Fatalf("error = %#v, want desktop error", err)
	}
	if safe.Code != desktopErrorCode("REFERENCE_CONFLICT") || safe.Message != "该内容仍被其他故事资料引用，请先解除关联后再归档。" {
		t.Fatalf("safe error = %#v", safe)
	}
}

func TestSafeDesktopErrorExplainsBlankWorldValue(t *testing.T) {
	var safe *desktopError
	err := safeDesktopError(worlddomain.ErrBlankValue)
	if !errors.As(err, &safe) {
		t.Fatalf("error = %#v, want desktop error", err)
	}
	if safe.Code != desktopErrorCode("VALIDATION_FAILED") || safe.Message != "请填写完整的角色、势力或归属信息。" {
		t.Fatalf("safe error = %#v", safe)
	}
}

func TestSafeDesktopErrorExplainsInvalidWorldAssets(t *testing.T) {
	tests := []error{
		worlddomain.ErrInvalidHierarchy,
		worlddomain.ErrInvalidReference,
		worlddomain.ErrInvalidStatus,
	}
	for _, internal := range tests {
		var safe *desktopError
		err := safeDesktopError(internal)
		if !errors.As(err, &safe) {
			t.Fatalf("error = %#v, want desktop error", err)
		}
		if safe.Code != errorValidationFailed || safe.Message != "故事资料包含无效状态、层级或关联，请检查后重试。" {
			t.Fatalf("safe error = %#v", safe)
		}
	}
}

func TestSafeDesktopErrorExplainsInvalidContinuityData(t *testing.T) {
	tests := []error{
		timelinedomain.ErrInvalidOrder,
		timelinedomain.ErrInvalidStatus,
		canondomain.ErrInvalidEvidence,
		canondomain.ErrInvalidConfidence,
	}
	for _, internal := range tests {
		var safe *desktopError
		err := safeDesktopError(internal)
		if !errors.As(err, &safe) || safe.Code != errorValidationFailed {
			t.Fatalf("error = %#v, want continuity validation error", err)
		}
		if safe.Message != "时间线或事实资料包含无效顺序、状态、证据或关联，请检查后重试。" {
			t.Fatalf("message = %q", safe.Message)
		}
	}
}

func TestSafeDesktopErrorExplainsAgentRuntimeFailures(t *testing.T) {
	tests := []struct {
		err     error
		code    desktopErrorCode
		message string
	}{
		{workspace.ErrTokenBudgetExceeded, errorAgentBudgetExceeded, "智能创作令牌预算不足，请提高预算或降低角色输出上限。"},
		{workspace.ErrContextBudgetExceeded, errorAgentContextExceeded, "必要的故事上下文超过预算，请提高上下文字符预算。"},
		{workspace.ErrAgentRunActive, errorAgentRunActive, "智能创作任务正在运行，请先等待完成或取消任务。"},
		{agentdomain.ErrInvalidBudget, errorValidationFailed, "运行预算必须是大于零的整数。"},
	}
	for _, test := range tests {
		var safe *desktopError
		if err := safeDesktopError(test.err); !errors.As(err, &safe) {
			t.Fatalf("safeDesktopError(%v) = %#v", test.err, err)
		}
		if safe.Code != test.code || safe.Message != test.message {
			t.Fatalf("safe error = %#v", safe)
		}
	}
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func TestAppShutdownClosesWorkspace(t *testing.T) {
	workspaces := &fakeWorkspace{}
	app := &App{api: newDesktopAPI(workspaces, &fakeDirectoryPicker{})}

	app.shutdown(context.Background())

	if workspaces.closeCalls != 1 {
		t.Fatalf("Close called %d times, want 1", workspaces.closeCalls)
	}
}

func TestProcessLeaseMetadataDescribesOnlyThisApplication(t *testing.T) {
	metadata := processLeaseMetadata()
	if metadata.PID != os.Getpid() {
		t.Fatalf("PID = %d, want %d", metadata.PID, os.Getpid())
	}
	if metadata.Hostname == "" || metadata.ApplicationVersion == "" {
		t.Fatalf("incomplete metadata = %#v", metadata)
	}
	if metadata.StartedAt.IsZero() || metadata.StartedAt.Location() != time.UTC {
		t.Fatalf("startedAt = %s, want non-zero UTC time", metadata.StartedAt)
	}
}

func TestAppWaitsForFrontendFlushBeforeClosing(t *testing.T) {
	workspaces := &fakeWorkspace{}
	emitCalls := 0
	quitCalls := 0
	app := &App{
		api: newDesktopAPI(workspaces, &fakeDirectoryPicker{}),
		emitBeforeClose: func(context.Context) {
			emitCalls++
		},
		quit: func(context.Context) {
			quitCalls++
		},
	}
	app.api.completeClose = app.completeClose
	app.startup(context.Background())

	if prevent := app.beforeClose(context.Background()); !prevent {
		t.Fatal("first close was not prevented for frontend draft flush")
	}
	if emitCalls != 1 || quitCalls != 0 {
		t.Fatalf("before flush: emit=%d quit=%d", emitCalls, quitCalls)
	}

	app.api.CompleteClose()
	if quitCalls != 1 {
		t.Fatalf("quit calls = %d, want 1 after frontend flush", quitCalls)
	}
	if prevent := app.beforeClose(context.Background()); prevent {
		t.Fatal("close was prevented after frontend completed its flush")
	}
}

type fakeDirectoryPicker struct {
	parentPath string
	parentErr  error
	openPath   string
	openErr    error
	importPath string
	importErr  error
}

func (f *fakeDirectoryPicker) ChooseProjectParent(context.Context) (string, error) {
	return f.parentPath, f.parentErr
}

func (f *fakeDirectoryPicker) ChooseProjectToOpen(context.Context) (string, error) {
	return f.openPath, f.openErr
}

func (f *fakeDirectoryPicker) ChooseManuscriptToImport(context.Context) (string, error) {
	return f.importPath, f.importErr
}

type draftCall struct {
	id       string
	body     string
	revision int64
}

type jobEventsCall struct {
	jobID         string
	afterSequence uint64
	limit         int
}

type fakeWorkspace struct {
	createInput              CreateProjectInput
	createResult             workspace.ProjectView
	createErr                error
	openResult               workspace.ProjectView
	openErr                  error
	openPath                 string
	openReadOnlyResult       workspace.ProjectView
	openReadOnlyErr          error
	openReadOnlyPath         string
	recoveryCopyInput        recoveryCopyCall
	recoveryCopyResult       workspace.RecoveryCopyResult
	recoveryCopyErr          error
	listResult               []workspace.ChapterView
	listErr                  error
	createChapterResult      workspace.ChapterView
	createChapterErr         error
	createdChapterTitle      string
	openChapterResult        workspace.ChapterView
	openChapterErr           error
	openedChapterID          string
	savedDraft               draftCall
	discardedDraftID         string
	discardedDraftRevision   int64
	saveDraftErr             error
	commitResult             workspace.ChapterView
	commitErr                error
	committed                draftCall
	changeSetsResult         []workspace.ChangeSetView
	chapterChangeInput       workspace.ChapterChangeInput
	applyChangeResult        workspace.ChangeSetApplyView
	appliedChangeSetID       string
	rejectedChangeSetID      string
	chapterHistoryResult     []workspace.ChapterRevisionView
	restoreRevisionResult    workspace.ChapterView
	restoreRevisionChapterID string
	restoreRevisionArgs      [2]int64
	snapshotsResult          []workspace.SnapshotView
	createSnapshotResult     workspace.SnapshotView
	snapshotName             string
	restoreSnapshotResult    workspace.SnapshotRestoreView
	restoreSnapshotID        string
	branchesResult           []workspace.BranchView
	createBranchResult       workspace.BranchView
	createBranchInput        CreateBranchInput
	switchBranchResult       workspace.BranchSwitchView
	switchBranchID           string
	compareBranchResult      workspace.BranchComparisonView
	compareBranchIDs         [2]string
	mergePreviewResult       workspace.BranchMergePreviewView
	mergePreviewSourceID     string
	mergeConfirmResult       workspace.BranchSwitchView
	mergeConfirmToken        string
	deletedBranchID          string
	closeCalls               int
	storyStructureResult     workspace.StoryStructureView
	outlineInput             workspace.CreateOutlineNodeInput
	storyLineInput           workspace.CreateStoryLineInput
	storyLinkInput           workspace.LinkStoryLineInput
	sceneCardInput           workspace.CreateSceneCardInput
	archivedOutlineID        string
	movedOutlineID           string
	moveDirection            string
	storyWorldResult         workspace.StoryWorldView
	characterInput           workspace.CharacterInput
	updatedCharacterID       string
	archivedCharacterID      string
	factionInput             workspace.FactionInput
	updatedFactionID         string
	archivedFactionID        string
	membershipInput          workspace.MembershipInput
	deletedMembership        string
	relationshipInput        workspace.RelationshipInput
	archivedRelationshipID   string
	chapterAppearanceInput   workspace.AppearanceInput
	sceneAppearanceInput     workspace.AppearanceInput
	unlinkedChapter          string
	unlinkedScene            string
	worldAssetsResult        workspace.WorldAssetsView
	worldRegionInput         workspace.WorldRegionInput
	updatedWorldRegionID     string
	archivedWorldRegionID    string
	locationInput            workspace.LocationInput
	updatedLocationID        string
	archivedLocationID       string
	locationLinkInput        workspace.LocationLinkInput
	unlinkedLocation         string
	itemInput                workspace.ItemInput
	updatedItemID            string
	itemTransferInput        workspace.ItemTransferInput
	archivedItemID           string
	worldRuleInput           workspace.WorldRuleInput
	updatedWorldRuleID       string
	archivedWorldRuleID      string
	hookInput                workspace.ForeshadowingHookInput
	updatedHookID            string
	archivedHookID           string
	continuityResult         workspace.ContinuityView
	storyEventInput          workspace.StoryEventInput
	updatedStoryEventID      string
	archivedStoryEventID     string
	storyEventLinkInput      workspace.StoryEventLinkInput
	unlinkedStoryEvent       string
	factInput                workspace.FactInput
	updatedFactID            string
	confirmedFactID          string
	archivedFactID           string
	factEvidenceInput        workspace.FactEvidenceInput
	removedFactEvidenceID    string
	factScopeInput           workspace.FactScopeInput
	removedFactScopeID       string
	factConflictInput        workspace.FactConflictInput
	resolvedFactConflictID   string
	deletedFactConflictID    string
	importPrepareResult      workspace.ImportPrepareResult
	preparedImportPath       string
	importJobsResult         []workspace.ImportJobView
	importJobResult          workspace.ImportJobView
	importJobID              string
	deletedImportJobID       string
	importApplyResult        workspace.ImportApplyView
	appliedImportJobID       string
	appliedImportChapterIDs  []string
	extractionRunInput       workspace.ExtractionRunInput
	startExtractionResult    workspace.ExtractionRunView
	executeExtractionID      string
	executeExtractionResult  workspace.ExtractionRunView
	cancelExtractionID       string
	cancelExtractionErr      error
	retryExtractionID        string
	retryExtractionResult    workspace.ExtractionRunView
	extractionTasksResult    []workspace.ExtractionTaskView
	extractionTaskID         string
	extractionTaskResult     workspace.ExtractionRunView
	jobSnapshotResult        workspace.JobSnapshotView
	jobSnapshotID            string
	jobSnapshotErr           error
	jobEventsResult          workspace.JobEventsPageView
	jobEventsInput           jobEventsCall
	jobEventsErr             error
	recoverableJobsResult    []workspace.JobSummaryView
	recoverableJobsErr       error
}

func (f *fakeWorkspace) CreateProject(
	_ context.Context,
	parentDirectory, title, genre, summary string,
) (workspace.ProjectView, error) {
	f.createInput = CreateProjectInput{
		ParentDirectory: parentDirectory,
		Title:           title,
		Genre:           genre,
		Summary:         summary,
	}
	return f.createResult, f.createErr
}

type recoveryCopyCall struct {
	sourcePath      string
	parentDirectory string
}

func (f *fakeWorkspace) OpenProject(_ context.Context, path string) (workspace.ProjectView, error) {
	f.openPath = path
	return f.openResult, f.openErr
}

func (f *fakeWorkspace) OpenProjectReadOnly(_ context.Context, path string) (workspace.ProjectView, error) {
	f.openReadOnlyPath = path
	return f.openReadOnlyResult, f.openReadOnlyErr
}

func (f *fakeWorkspace) CreateProjectRecoveryCopy(_ context.Context, sourcePath, parentDirectory string) (workspace.RecoveryCopyResult, error) {
	f.recoveryCopyInput = recoveryCopyCall{sourcePath: sourcePath, parentDirectory: parentDirectory}
	return f.recoveryCopyResult, f.recoveryCopyErr
}

func (f *fakeWorkspace) ListChapters(context.Context) ([]workspace.ChapterView, error) {
	return f.listResult, f.listErr
}

func (f *fakeWorkspace) CreateChapter(_ context.Context, title string) (workspace.ChapterView, error) {
	f.createdChapterTitle = title
	return f.createChapterResult, f.createChapterErr
}

func (f *fakeWorkspace) OpenChapter(_ context.Context, id string) (workspace.ChapterView, error) {
	f.openedChapterID = id
	return f.openChapterResult, f.openChapterErr
}

func (f *fakeWorkspace) SaveDraft(_ context.Context, id, body string, revision int64) error {
	f.savedDraft = draftCall{id: id, body: body, revision: revision}
	return f.saveDraftErr
}

func (f *fakeWorkspace) DiscardDraft(_ context.Context, id string, revision int64) error {
	f.discardedDraftID = id
	f.discardedDraftRevision = revision
	return nil
}

func (f *fakeWorkspace) CommitChapter(
	_ context.Context,
	id, body string,
	revision int64,
) (workspace.ChapterView, error) {
	f.committed = draftCall{id: id, body: body, revision: revision}
	return f.commitResult, f.commitErr
}

func (f *fakeWorkspace) ChangeSets(context.Context) ([]workspace.ChangeSetView, error) {
	return f.changeSetsResult, nil
}

func (f *fakeWorkspace) ProposeChapterChange(_ context.Context, input workspace.ChapterChangeInput) (workspace.ChangeSetView, error) {
	f.chapterChangeInput = input
	return workspace.ChangeSetView{}, nil
}

func (f *fakeWorkspace) ProposeStructuredChange(_ context.Context, input workspace.StructuredChangeInput) (workspace.ChangeSetView, error) {
	return workspace.ChangeSetView{}, nil
}

func (f *fakeWorkspace) EditChangeOperation(_ context.Context, input workspace.ChangeOperationEditInput) (workspace.ChangeSetView, error) {
	return workspace.ChangeSetView{}, nil
}

func (f *fakeWorkspace) ApplyChangeSet(_ context.Context, id string) (workspace.ChangeSetApplyView, error) {
	f.appliedChangeSetID = id
	return f.applyChangeResult, nil
}

func (f *fakeWorkspace) ApplyChangeOperation(_ context.Context, changeSetID, operationID string) (workspace.ChangeSetApplyView, error) {
	return workspace.ChangeSetApplyView{}, nil
}

func (f *fakeWorkspace) RejectChangeSet(_ context.Context, id string) error {
	f.rejectedChangeSetID = id
	return nil
}

func (f *fakeWorkspace) RejectChangeOperation(_ context.Context, changeSetID, operationID string) error {
	return nil
}

func (f *fakeWorkspace) ChapterHistory(_ context.Context, chapterID string) ([]workspace.ChapterRevisionView, error) {
	f.restoreRevisionChapterID = chapterID
	return f.chapterHistoryResult, nil
}

func (f *fakeWorkspace) RestoreChapterRevision(_ context.Context, chapterID string, targetRevision, expectedRevision int64) (workspace.ChapterView, error) {
	f.restoreRevisionChapterID = chapterID
	f.restoreRevisionArgs = [2]int64{targetRevision, expectedRevision}
	return f.restoreRevisionResult, nil
}

func (f *fakeWorkspace) Snapshots(context.Context) ([]workspace.SnapshotView, error) {
	return f.snapshotsResult, nil
}

func (f *fakeWorkspace) CreateSnapshot(_ context.Context, name string) (workspace.SnapshotView, error) {
	f.snapshotName = name
	return f.createSnapshotResult, nil
}

func (f *fakeWorkspace) RestoreSnapshot(_ context.Context, id string) (workspace.SnapshotRestoreView, error) {
	f.restoreSnapshotID = id
	return f.restoreSnapshotResult, nil
}

func (f *fakeWorkspace) Branches(context.Context) ([]workspace.BranchView, error) {
	return f.branchesResult, nil
}

func (f *fakeWorkspace) CreateBranch(_ context.Context, name, description string) (workspace.BranchView, error) {
	f.createBranchInput = CreateBranchInput{Name: name, Description: description}
	return f.createBranchResult, nil
}

func (f *fakeWorkspace) SwitchBranch(_ context.Context, id string) (workspace.BranchSwitchView, error) {
	f.switchBranchID = id
	return f.switchBranchResult, nil
}

func (f *fakeWorkspace) CompareBranches(_ context.Context, sourceID, targetID string) (workspace.BranchComparisonView, error) {
	f.compareBranchIDs = [2]string{sourceID, targetID}
	return f.compareBranchResult, nil
}

func (f *fakeWorkspace) PreviewBranchMerge(_ context.Context, sourceID string) (workspace.BranchMergePreviewView, error) {
	f.mergePreviewSourceID = sourceID
	return f.mergePreviewResult, nil
}

func (f *fakeWorkspace) ConfirmBranchMerge(_ context.Context, token string) (workspace.BranchSwitchView, error) {
	f.mergeConfirmToken = token
	return f.mergeConfirmResult, nil
}

func (f *fakeWorkspace) DeleteBranch(_ context.Context, id string) error {
	f.deletedBranchID = id
	return nil
}

func (f *fakeWorkspace) Close() error {
	f.closeCalls++
	return nil
}

func (f *fakeWorkspace) StoryStructure(context.Context) (workspace.StoryStructureView, error) {
	return f.storyStructureResult, nil
}

func (f *fakeWorkspace) CreateOutlineNode(_ context.Context, input workspace.CreateOutlineNodeInput) (workspace.OutlineNodeView, error) {
	f.outlineInput = input
	return workspace.OutlineNodeView{}, nil
}

func (f *fakeWorkspace) CreateStoryLine(_ context.Context, input workspace.CreateStoryLineInput) (workspace.StoryLineView, error) {
	f.storyLineInput = input
	return workspace.StoryLineView{}, nil
}

func (f *fakeWorkspace) LinkStoryLine(_ context.Context, input workspace.LinkStoryLineInput) (workspace.StoryLineLinkView, error) {
	f.storyLinkInput = input
	return workspace.StoryLineLinkView{}, nil
}

func (f *fakeWorkspace) CreateSceneCard(_ context.Context, input workspace.CreateSceneCardInput) (workspace.SceneCardView, error) {
	f.sceneCardInput = input
	return workspace.SceneCardView{}, nil
}

func (f *fakeWorkspace) ArchiveOutlineNode(_ context.Context, id string) error {
	f.archivedOutlineID = id
	return nil
}

func (f *fakeWorkspace) MoveOutlineNode(_ context.Context, id, direction string) error {
	f.movedOutlineID, f.moveDirection = id, direction
	return nil
}

func (f *fakeWorkspace) StoryWorld(context.Context) (workspace.StoryWorldView, error) {
	return f.storyWorldResult, nil
}
func (f *fakeWorkspace) CreateCharacter(_ context.Context, input workspace.CharacterInput) (workspace.CharacterView, error) {
	f.characterInput = input
	return workspace.CharacterView{}, nil
}
func (f *fakeWorkspace) UpdateCharacter(_ context.Context, id string, input workspace.CharacterInput) (workspace.CharacterView, error) {
	f.updatedCharacterID, f.characterInput = id, input
	return workspace.CharacterView{}, nil
}
func (f *fakeWorkspace) ArchiveCharacter(_ context.Context, id string) error {
	f.archivedCharacterID = id
	return nil
}
func (f *fakeWorkspace) CreateFaction(_ context.Context, input workspace.FactionInput) (workspace.FactionView, error) {
	f.factionInput = input
	return workspace.FactionView{}, nil
}
func (f *fakeWorkspace) UpdateFaction(_ context.Context, id string, input workspace.FactionInput) (workspace.FactionView, error) {
	f.updatedFactionID, f.factionInput = id, input
	return workspace.FactionView{}, nil
}
func (f *fakeWorkspace) ArchiveFaction(_ context.Context, id string) error {
	f.archivedFactionID = id
	return nil
}
func (f *fakeWorkspace) UpsertMembership(_ context.Context, input workspace.MembershipInput) (workspace.MembershipView, error) {
	f.membershipInput = input
	return workspace.MembershipView{}, nil
}
func (f *fakeWorkspace) DeleteMembership(_ context.Context, characterID, factionID string) error {
	f.deletedMembership = characterID + ":" + factionID
	return nil
}
func (f *fakeWorkspace) CreateRelationship(_ context.Context, input workspace.RelationshipInput) (workspace.RelationshipView, error) {
	f.relationshipInput = input
	return workspace.RelationshipView{}, nil
}
func (f *fakeWorkspace) ArchiveRelationship(_ context.Context, id string) error {
	f.archivedRelationshipID = id
	return nil
}
func (f *fakeWorkspace) LinkCharacterToChapter(_ context.Context, input workspace.AppearanceInput) error {
	f.chapterAppearanceInput = input
	return nil
}
func (f *fakeWorkspace) UnlinkCharacterFromChapter(_ context.Context, characterID, chapterID string) error {
	f.unlinkedChapter = characterID + ":" + chapterID
	return nil
}
func (f *fakeWorkspace) LinkCharacterToScene(_ context.Context, input workspace.AppearanceInput) error {
	f.sceneAppearanceInput = input
	return nil
}
func (f *fakeWorkspace) UnlinkCharacterFromScene(_ context.Context, characterID, sceneCardID string) error {
	f.unlinkedScene = characterID + ":" + sceneCardID
	return nil
}
func (f *fakeWorkspace) WorldAssets(context.Context) (workspace.WorldAssetsView, error) {
	return f.worldAssetsResult, nil
}
func (f *fakeWorkspace) CreateWorldRegion(_ context.Context, input workspace.WorldRegionInput) (workspace.WorldRegionView, error) {
	f.worldRegionInput = input
	return workspace.WorldRegionView{}, nil
}
func (f *fakeWorkspace) UpdateWorldRegion(_ context.Context, id string, input workspace.WorldRegionInput) (workspace.WorldRegionView, error) {
	f.updatedWorldRegionID, f.worldRegionInput = id, input
	return workspace.WorldRegionView{}, nil
}
func (f *fakeWorkspace) ArchiveWorldRegion(_ context.Context, id string) error {
	f.archivedWorldRegionID = id
	return nil
}
func (f *fakeWorkspace) UpsertRegionAdjacency(_ context.Context, input workspace.RegionAdjacencyInput) (workspace.RegionAdjacencyView, error) {
	return workspace.RegionAdjacencyView{FirstRegionID: input.FirstRegionID, SecondRegionID: input.SecondRegionID, TravelNote: input.TravelNote}, nil
}
func (f *fakeWorkspace) DeleteRegionAdjacency(_ context.Context, firstRegionID, secondRegionID string) error {
	return nil
}
func (f *fakeWorkspace) CreateLocation(_ context.Context, input workspace.LocationInput) (workspace.LocationView, error) {
	f.locationInput = input
	return workspace.LocationView{}, nil
}
func (f *fakeWorkspace) UpdateLocation(_ context.Context, id string, input workspace.LocationInput) (workspace.LocationView, error) {
	f.updatedLocationID, f.locationInput = id, input
	return workspace.LocationView{}, nil
}
func (f *fakeWorkspace) ArchiveLocation(_ context.Context, id string) error {
	f.archivedLocationID = id
	return nil
}
func (f *fakeWorkspace) LinkLocation(_ context.Context, input workspace.LocationLinkInput) (workspace.LocationLinkView, error) {
	f.locationLinkInput = input
	return workspace.LocationLinkView{}, nil
}
func (f *fakeWorkspace) UnlinkLocation(_ context.Context, locationID, targetType, targetID string) error {
	f.unlinkedLocation = locationID + ":" + targetType + ":" + targetID
	return nil
}
func (f *fakeWorkspace) CreateItem(_ context.Context, input workspace.ItemInput) (workspace.ItemView, error) {
	f.itemInput = input
	return workspace.ItemView{}, nil
}
func (f *fakeWorkspace) UpdateItem(_ context.Context, id string, input workspace.ItemInput) (workspace.ItemView, error) {
	f.updatedItemID, f.itemInput = id, input
	return workspace.ItemView{}, nil
}
func (f *fakeWorkspace) TransferItem(_ context.Context, input workspace.ItemTransferInput) (workspace.ItemTransferView, error) {
	f.itemTransferInput = input
	return workspace.ItemTransferView{}, nil
}
func (f *fakeWorkspace) ArchiveItem(_ context.Context, id string) error {
	f.archivedItemID = id
	return nil
}
func (f *fakeWorkspace) CreateWorldRule(_ context.Context, input workspace.WorldRuleInput) (workspace.WorldRuleView, error) {
	f.worldRuleInput = input
	return workspace.WorldRuleView{}, nil
}
func (f *fakeWorkspace) UpdateWorldRule(_ context.Context, id string, input workspace.WorldRuleInput) (workspace.WorldRuleView, error) {
	f.updatedWorldRuleID, f.worldRuleInput = id, input
	return workspace.WorldRuleView{}, nil
}
func (f *fakeWorkspace) ArchiveWorldRule(_ context.Context, id string) error {
	f.archivedWorldRuleID = id
	return nil
}
func (f *fakeWorkspace) CreateForeshadowingHook(_ context.Context, input workspace.ForeshadowingHookInput) (workspace.ForeshadowingHookView, error) {
	f.hookInput = input
	return workspace.ForeshadowingHookView{}, nil
}
func (f *fakeWorkspace) UpdateForeshadowingHook(_ context.Context, id string, input workspace.ForeshadowingHookInput) (workspace.ForeshadowingHookView, error) {
	f.updatedHookID, f.hookInput = id, input
	return workspace.ForeshadowingHookView{}, nil
}
func (f *fakeWorkspace) ArchiveForeshadowingHook(_ context.Context, id string) error {
	f.archivedHookID = id
	return nil
}
func (f *fakeWorkspace) Continuity(context.Context) (workspace.ContinuityView, error) {
	return f.continuityResult, nil
}
func (f *fakeWorkspace) CreateStoryEvent(_ context.Context, input workspace.StoryEventInput) (workspace.StoryEventView, error) {
	f.storyEventInput = input
	return workspace.StoryEventView{}, nil
}
func (f *fakeWorkspace) UpdateStoryEvent(_ context.Context, id string, input workspace.StoryEventInput) (workspace.StoryEventView, error) {
	f.updatedStoryEventID, f.storyEventInput = id, input
	return workspace.StoryEventView{}, nil
}
func (f *fakeWorkspace) ArchiveStoryEvent(_ context.Context, id string) error {
	f.archivedStoryEventID = id
	return nil
}
func (f *fakeWorkspace) LinkStoryEvent(_ context.Context, input workspace.StoryEventLinkInput) (workspace.StoryEventLinkView, error) {
	f.storyEventLinkInput = input
	return workspace.StoryEventLinkView{}, nil
}
func (f *fakeWorkspace) UnlinkStoryEvent(_ context.Context, eventID, targetType, targetID string) error {
	f.unlinkedStoryEvent = eventID + ":" + targetType + ":" + targetID
	return nil
}
func (f *fakeWorkspace) CreateFact(_ context.Context, input workspace.FactInput) (workspace.FactView, error) {
	f.factInput = input
	return workspace.FactView{}, nil
}
func (f *fakeWorkspace) UpdateFact(_ context.Context, id string, input workspace.FactInput) (workspace.FactView, error) {
	f.updatedFactID, f.factInput = id, input
	return workspace.FactView{}, nil
}
func (f *fakeWorkspace) ConfirmFact(_ context.Context, id string) (workspace.FactView, error) {
	f.confirmedFactID = id
	return workspace.FactView{}, nil
}
func (f *fakeWorkspace) ArchiveFact(_ context.Context, id string) error {
	f.archivedFactID = id
	return nil
}
func (f *fakeWorkspace) CharacterKnowledge(context.Context) ([]workspace.CharacterKnowledgeView, error) {
	return nil, nil
}
func (f *fakeWorkspace) CreateCharacterKnowledge(_ context.Context, input workspace.CharacterKnowledgeInput) (workspace.CharacterKnowledgeView, error) {
	return workspace.CharacterKnowledgeView{CharacterID: input.CharacterID, FactID: input.FactID, State: input.State, BeliefText: input.BeliefText, AvailableFromChapterID: input.AvailableFromChapterID, Notes: input.Notes}, nil
}
func (f *fakeWorkspace) ArchiveCharacterKnowledge(_ context.Context, id string) error {
	return nil
}
func (f *fakeWorkspace) AddFactEvidence(_ context.Context, input workspace.FactEvidenceInput) (workspace.FactEvidenceView, error) {
	f.factEvidenceInput = input
	return workspace.FactEvidenceView{}, nil
}
func (f *fakeWorkspace) RemoveFactEvidence(_ context.Context, id string) error {
	f.removedFactEvidenceID = id
	return nil
}
func (f *fakeWorkspace) AddFactScope(_ context.Context, input workspace.FactScopeInput) (workspace.FactScopeView, error) {
	f.factScopeInput = input
	return workspace.FactScopeView{}, nil
}
func (f *fakeWorkspace) RemoveFactScope(_ context.Context, id string) error {
	f.removedFactScopeID = id
	return nil
}
func (f *fakeWorkspace) CreateFactConflict(_ context.Context, input workspace.FactConflictInput) (workspace.FactConflictView, error) {
	f.factConflictInput = input
	return workspace.FactConflictView{}, nil
}
func (f *fakeWorkspace) ResolveFactConflict(_ context.Context, id string) (workspace.FactConflictView, error) {
	f.resolvedFactConflictID = id
	return workspace.FactConflictView{}, nil
}
func (f *fakeWorkspace) DeleteFactConflict(_ context.Context, id string) error {
	f.deletedFactConflictID = id
	return nil
}
func (f *fakeWorkspace) ProviderConnections(context.Context) ([]workspace.ProviderConnectionView, error) {
	return nil, nil
}
func (f *fakeWorkspace) SaveProviderConnection(_ context.Context, input workspace.ProviderConnectionInput) (workspace.ProviderConnectionView, error) {
	return workspace.ProviderConnectionView{Name: input.Name, Kind: input.Kind, BaseURL: input.BaseURL}, nil
}
func (f *fakeWorkspace) DeleteProviderConnection(context.Context, string) error {
	return nil
}
func (f *fakeWorkspace) TestProviderConnection(context.Context, string) error {
	return nil
}
func (f *fakeWorkspace) AgentProfiles(context.Context) ([]workspace.AgentProfileView, error) {
	return nil, nil
}
func (f *fakeWorkspace) SaveAgentProfile(_ context.Context, input workspace.AgentProfileInput) (workspace.AgentProfileView, error) {
	return workspace.AgentProfileView{Role: input.Role, ConnectionID: input.ConnectionID, Model: input.Model}, nil
}
func (f *fakeWorkspace) AgentRuntimeSettings(context.Context) (workspace.AgentRuntimeSettingsView, error) {
	return workspace.AgentRuntimeSettingsView{MaxRunTokens: 30000, MaxContextChars: 120000}, nil
}
func (f *fakeWorkspace) SaveAgentRuntimeSettings(_ context.Context, input workspace.AgentRuntimeSettingsInput) (workspace.AgentRuntimeSettingsView, error) {
	return workspace.AgentRuntimeSettingsView{MaxRunTokens: input.MaxRunTokens, MaxContextChars: input.MaxContextChars}, nil
}
func (f *fakeWorkspace) CreateChapterAgentRun(_ context.Context, chapterID, prompt string) (workspace.AgentRunView, error) {
	return workspace.AgentRunView{ChapterID: chapterID, Prompt: prompt, Status: "queued"}, nil
}
func (f *fakeWorkspace) ExecuteAgentRun(context.Context, string) (workspace.AgentRunView, error) {
	return workspace.AgentRunView{Status: "completed"}, nil
}
func (f *fakeWorkspace) CancelAgentRun(context.Context, string) error {
	return nil
}
func (f *fakeWorkspace) RetryAgentRun(context.Context, string) (workspace.AgentRunView, error) {
	return workspace.AgentRunView{Status: "queued"}, nil
}
func (f *fakeWorkspace) AgentRuns(context.Context) ([]workspace.AgentRunView, error) {
	return nil, nil
}
func (f *fakeWorkspace) AgentRun(context.Context, string) (workspace.AgentRunView, error) {
	return workspace.AgentRunView{}, nil
}

func (f *fakeWorkspace) PrepareManuscriptImport(_ context.Context, sourcePath string) (workspace.ImportPrepareResult, error) {
	f.preparedImportPath = sourcePath
	return f.importPrepareResult, nil
}

func (f *fakeWorkspace) ImportJobs(context.Context) ([]workspace.ImportJobView, error) {
	return f.importJobsResult, nil
}

func (f *fakeWorkspace) ImportJob(_ context.Context, id string) (workspace.ImportJobView, error) {
	f.importJobID = id
	return f.importJobResult, nil
}

func (f *fakeWorkspace) DeleteImportJob(_ context.Context, id string) error {
	f.deletedImportJobID = id
	return nil
}

func (f *fakeWorkspace) ApplyImportJob(_ context.Context, id string, selectedIDs []string) (workspace.ImportApplyView, error) {
	f.appliedImportJobID = id
	f.appliedImportChapterIDs = append([]string(nil), selectedIDs...)
	return f.importApplyResult, nil
}

func (f *fakeWorkspace) StartExtractionRun(_ context.Context, input workspace.ExtractionRunInput) (workspace.ExtractionRunView, error) {
	f.extractionRunInput = input
	return f.startExtractionResult, nil
}

func (f *fakeWorkspace) ExecuteExtractionRun(_ context.Context, id string) (workspace.ExtractionRunView, error) {
	f.executeExtractionID = id
	return f.executeExtractionResult, nil
}

func (f *fakeWorkspace) CancelExtractionRun(_ context.Context, id string) error {
	f.cancelExtractionID = id
	return f.cancelExtractionErr
}

func (f *fakeWorkspace) RetryExtractionRun(_ context.Context, id string) (workspace.ExtractionRunView, error) {
	f.retryExtractionID = id
	return f.retryExtractionResult, nil
}

func (f *fakeWorkspace) ExtractionTasks(context.Context) ([]workspace.ExtractionTaskView, error) {
	return f.extractionTasksResult, nil
}

func (f *fakeWorkspace) ExtractionTask(_ context.Context, id string) (workspace.ExtractionRunView, error) {
	f.extractionTaskID = id
	return f.extractionTaskResult, nil
}

func (f *fakeWorkspace) GetJobSnapshot(_ context.Context, jobID string) (workspace.JobSnapshotView, error) {
	f.jobSnapshotID = jobID
	return f.jobSnapshotResult, f.jobSnapshotErr
}

func (f *fakeWorkspace) GetJobEvents(_ context.Context, jobID string, afterSequence uint64, limit int) (workspace.JobEventsPageView, error) {
	f.jobEventsInput = jobEventsCall{jobID: jobID, afterSequence: afterSequence, limit: limit}
	return f.jobEventsResult, f.jobEventsErr
}

func (f *fakeWorkspace) ListRecoverableJobs(context.Context) ([]workspace.JobSummaryView, error) {
	return f.recoverableJobsResult, f.recoverableJobsErr
}

// TestDesktopAPIForwardsJobEventQueries asserts that GetJobSnapshot,
// GetJobEvents, and ListRecoverableJobs forward their arguments to the
// workspace service and return its results without transformation.
func TestDesktopAPIForwardsJobEventQueries(t *testing.T) {
	snapshot := workspace.JobSnapshotView{
		JobID:       "run-1",
		JobKind:     "agent",
		Status:      "completed",
		MaxSequence: 8,
	}
	page := workspace.JobEventsPageView{
		Events:     []workspace.JobEventView{{JobID: "run-1", SequenceNumber: 1}},
		NextCursor: 1,
		HasMore:    true,
	}
	jobs := []workspace.JobSummaryView{
		{JobID: "run-1", JobKind: "agent", Status: "queued"},
		{JobID: "task-1", JobKind: "extraction", Status: "running"},
	}
	workspaces := &fakeWorkspace{
		jobSnapshotResult:     snapshot,
		jobEventsResult:       page,
		recoverableJobsResult: jobs,
	}
	api := newDesktopAPI(workspaces, &fakeDirectoryPicker{})
	api.startup(context.Background())

	gotSnapshot, err := api.GetJobSnapshot(" run-1 ")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotSnapshot, snapshot) {
		t.Fatalf("GetJobSnapshot = %#v, want %#v", gotSnapshot, snapshot)
	}
	if workspaces.jobSnapshotID != " run-1 " {
		t.Fatalf("forwarded jobID = %q, want %q", workspaces.jobSnapshotID, " run-1 ")
	}

	gotPage, err := api.GetJobEvents("run-1", 3, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotPage, page) {
		t.Fatalf("GetJobEvents = %#v, want %#v", gotPage, page)
	}
	if workspaces.jobEventsInput != (jobEventsCall{jobID: "run-1", afterSequence: 3, limit: 10}) {
		t.Fatalf("forwarded job events call = %#v", workspaces.jobEventsInput)
	}

	gotJobs, err := api.ListRecoverableJobs()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotJobs, jobs) {
		t.Fatalf("ListRecoverableJobs = %#v, want %#v", gotJobs, jobs)
	}
}

// TestWailsJobEventNotifierEmitsPayload asserts that wailsJobEventNotifier
// calls the injected emit function with the "job:updated" event name and a
// payload carrying the job identity and latest sequence.
func TestWailsJobEventNotifierEmitsPayload(t *testing.T) {
	var capturedName string
	var capturedData []any
	notifier := newWailsJobEventNotifier()
	notifier.emit = func(_ context.Context, name string, data ...any) {
		capturedName = name
		capturedData = data
	}

	notifier.NotifyJobUpdated("run-1", agentdomain.JobKindAgent, 7)

	if capturedName != jobUpdatedEvent {
		t.Fatalf("event name = %q, want %q", capturedName, jobUpdatedEvent)
	}
	if len(capturedData) != 1 {
		t.Fatalf("data len = %d, want 1", len(capturedData))
	}
	payload, ok := capturedData[0].(jobUpdatedPayload)
	if !ok {
		t.Fatalf("data[0] type = %T, want jobUpdatedPayload", capturedData[0])
	}
	if payload.JobID != "run-1" {
		t.Fatalf("payload.JobID = %q, want run-1", payload.JobID)
	}
	if payload.JobKind != "agent" {
		t.Fatalf("payload.JobKind = %q, want agent", payload.JobKind)
	}
	if payload.LatestSequence != 7 {
		t.Fatalf("payload.LatestSequence = %d, want 7", payload.LatestSequence)
	}
}
