package creation

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
)

func TestConfirmArchiveRejectsWithoutSelectedDirection(t *testing.T) {
	service, _, _ := newTestService(t, newFakeKernel(), nil)
	session := seedIntentSession(t, service)
	if _, err := service.PreviewConfirmArchive(context.Background(), session.ProjectID); !errors.Is(err, creationdomain.ErrStageIncomplete) {
		t.Fatalf("preview without direction error = %v", err)
	}
	if _, err := service.ConfirmArchive(context.Background(), ArchiveRequest{
		ProjectID: session.ProjectID, ConfirmationDigest: "x", IdempotencyKey: "idem-1",
	}); !errors.Is(err, creationdomain.ErrStageIncomplete) {
		t.Fatalf("confirm without direction error = %v", err)
	}
}

// archiveSessionConflictStore 在入档事务前插入作者保存，验证事务不能用旧会话覆盖新内容。
type archiveSessionConflictStore struct{ Store }

func (store archiveSessionConflictStore) ApplyArchive(ctx context.Context, apply ArchiveApply) error {
	latest, err := store.Session(ctx, apply.Session.ProjectID)
	if err != nil {
		return err
	}
	latest.IntentText = "确认入档期间保存的新意图"
	if err := store.SaveSession(ctx, latest); err != nil {
		return err
	}
	return store.Store.ApplyArchive(ctx, apply)
}

func TestArchiveSessionCASRollsBackWholeTransaction(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now()}); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{Store: archiveSessionConflictStore{bridge}, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-conflict"}); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("旧会话入档未返回冲突: %v", err)
	}
	latest, err := bridge.Session(ctx, project.ID)
	if err != nil || latest.IntentText != "确认入档期间保存的新意图" || latest.ArchiveChangeSetID != "" {
		t.Fatalf("事务覆盖作者内容: %+v, %v", latest, err)
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 0 {
		t.Fatalf("失败入档留下人物: %+v, %v", characters, err)
	}
	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil || len(nodes) != 0 {
		t.Fatalf("失败入档留下路线图: %+v, %v", nodes, err)
	}
	currentProject, err := raw.Project(ctx)
	if err != nil || currentProject.Title != project.Title {
		t.Fatalf("失败入档改变项目: %+v, %v", currentProject, err)
	}
	workbench, err := raw.WorkbenchState(ctx)
	if err != nil || workbench.Stage == projectdomain.StageWriting || workbench.ArchiveStatus == "confirmed" {
		t.Fatalf("失败入档改变工作台: %+v, %v", workbench, err)
	}
	candidates, err := bridge.ListCandidates(ctx, latest.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		if candidate.Status == creationdomain.StatusConfirmed {
			t.Fatalf("失败入档确认了候选: %s", candidate.ID)
		}
	}
}

// archiveCandidateMutator 在入档事务提交前修改候选，复现“预览后改锁/改质量/候选被替代”的并发窗口。
type archiveCandidateMutator struct {
	Store
	mutate func(creationdomain.Candidate) (creationdomain.Candidate, error)
}

func (store archiveCandidateMutator) ApplyArchive(ctx context.Context, apply ArchiveApply) error {
	for _, candidate := range apply.Candidates {
		if candidate.Stage != creationdomain.StageProposal {
			continue
		}
		changed, err := store.mutate(candidate)
		if err != nil {
			return err
		}
		if err := store.UpdateCandidate(ctx, changed); err != nil {
			return err
		}
	}
	return store.Store.ApplyArchive(ctx, apply)
}

// TestConfirmArchiveRejectsCandidateChangedAfterPreview 覆盖入档事务对候选快照的条件校验：
// 预览摘要通过后、事务提交前发生的改锁、质量变化或候选被替代都必须拒绝并整单回滚。
func TestConfirmArchiveRejectsCandidateChangedAfterPreview(t *testing.T) {
	now := testClock{}.Now()
	cases := map[string]func(creationdomain.Candidate) (creationdomain.Candidate, error){
		"改锁": func(candidate creationdomain.Candidate) (creationdomain.Candidate, error) {
			return candidate.LockField("/title", now)
		},
		"改质量": func(candidate creationdomain.Candidate) (creationdomain.Candidate, error) {
			return candidate.WithQuality(creationdomain.QualityBlocked), nil
		},
		"被替代": func(candidate creationdomain.Candidate) (creationdomain.Candidate, error) {
			return candidate.MarkSuperseded(now)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			raw, project := sqliteCreateStore(t)
			defer raw.Close()
			bridge := NewSQLiteStore(raw)
			seedArchiveSession(t, bridge, project.ID)
			if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: now}); err != nil {
				t.Fatal(err)
			}
			service, err := New(Config{Store: archiveCandidateMutator{Store: bridge, mutate: mutate}, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
			if err != nil {
				t.Fatal(err)
			}
			preview, err := service.PreviewConfirmArchive(ctx, project.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-" + name}); !errors.Is(err, creationdomain.ErrConfirmationStale) {
				t.Fatalf("候选改动未返回确认过期错误: %v", err)
			}
			characters, err := raw.ListCharacters(ctx)
			if err != nil || len(characters) != 0 {
				t.Fatalf("失败入档留下人物: %+v, %v", characters, err)
			}
			nodes, err := raw.ListOutlineNodes(ctx)
			if err != nil || len(nodes) != 0 {
				t.Fatalf("失败入档留下路线图: %+v, %v", nodes, err)
			}
			latest, err := bridge.Session(ctx, project.ID)
			if err != nil {
				t.Fatal(err)
			}
			candidates, err := bridge.ListCandidates(ctx, latest.ID)
			if err != nil {
				t.Fatal(err)
			}
			for _, candidate := range candidates {
				if candidate.Status == creationdomain.StatusConfirmed {
					t.Fatalf("失败入档确认了候选: %s", candidate.ID)
				}
			}
		})
	}
}

// titleOverrideStore 覆盖项目标题读取，模拟“预览后手工修改标题”。
type titleOverrideStore struct {
	Store
	title string
}

func (store *titleOverrideStore) Project(ctx context.Context) (projectdomain.Project, error) {
	project, err := store.Store.Project(ctx)
	if err != nil {
		return project, err
	}
	project.Title = store.title
	return project, nil
}

// archiveDirectionDiscarder 在入档事务提交前废弃选中的方向，复现“上游依赖在事务前被废弃”。
type archiveDirectionDiscarder struct{ Store }

func (store archiveDirectionDiscarder) ApplyArchive(ctx context.Context, apply ArchiveApply) error {
	for _, candidate := range apply.Verify {
		discarded, err := candidate.Discard(testClock{}.Now())
		if err != nil {
			return err
		}
		if err := store.UpdateCandidate(ctx, discarded); err != nil {
			return err
		}
	}
	return store.Store.ApplyArchive(ctx, apply)
}

// TestConfirmArchiveRejectsIntentSavedAfterPreview 覆盖摘要绑定会话内容版本：预览后保存意图、
// stale 尚未传播时，旧摘要必须失效，不能再把旧方案入档。
func TestConfirmArchiveRejectsIntentSavedAfterPreview(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now()}); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 预览后保存新意图：内容版本递增，下游 stale 尚未传播。
	latest, err := bridge.Session(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	latest.IntentText = "预览后新保存的意图"
	if err := bridge.SaveSession(ctx, latest); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-intent"}); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("意图变化后入档未拒绝: %v", err)
	}
}

// TestConfirmArchiveRejectsProjectBaselineChangedAfterPreview 覆盖摘要冻结正式项目基线：
// 预览后手工修改标题，旧确认摘要必须失效，不能把新标题覆盖回方案标题。
func TestConfirmArchiveRejectsProjectBaselineChangedAfterPreview(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now()}); err != nil {
		t.Fatal(err)
	}
	store := &titleOverrideStore{Store: bridge, title: project.Title}
	service, err := New(Config{Store: store, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 预览后手工修改标题。
	store.title = "作者手工改的新标题"
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-title"}); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("项目基线变化后入档未拒绝: %v", err)
	}
}

// TestConfirmArchiveRejectsDirectionDiscardedAfterPreview 覆盖选中的方向作为上游依赖被校验：
// 预览后废弃方向，入档必须拒绝。
func TestConfirmArchiveRejectsDirectionDiscardedAfterPreview(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now()}); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	direction, err := bridge.Candidate(ctx, "dir-set-1")
	if err != nil {
		t.Fatal(err)
	}
	discarded, err := direction.Discard(testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.UpdateCandidate(ctx, discarded); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-direction"}); !errors.Is(err, creationdomain.ErrStageIncomplete) {
		t.Fatalf("方向废弃后入档未拒绝: %v", err)
	}
}

// TestConfirmArchiveRejectsDirectionDiscardedInsideTransaction 覆盖事务内对方向依赖的快照校验：
// 确认读取后、提交前废弃方向，必须拒绝并整单回滚。
func TestConfirmArchiveRejectsDirectionDiscardedInsideTransaction(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now()}); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{Store: archiveDirectionDiscarder{Store: bridge}, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-direction-tx"}); !errors.Is(err, creationdomain.ErrConfirmationStale) {
		t.Fatalf("事务内方向废弃未返回确认过期错误: %v", err)
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 0 {
		t.Fatalf("失败入档留下人物: %+v, %v", characters, err)
	}
	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil || len(nodes) != 0 {
		t.Fatalf("失败入档留下路线图: %+v, %v", nodes, err)
	}
}

// TestConfirmArchiveReplaysIdempotentlyDespiteCandidateChange 覆盖成功请求的幂等重放不依赖实时状态：
// 首次入档成功后修改候选锁，再用完全相同的请求重试，必须返回原 ChangeSet。
func TestConfirmArchiveReplaysIdempotentlyDespiteCandidateChange(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now()}); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-replay"})
	if err != nil {
		t.Fatal(err)
	}
	// 入档成功后修改候选锁。
	proposal, err := bridge.Candidate(ctx, "proposal-1")
	if err != nil {
		t.Fatal(err)
	}
	locked, err := proposal.LockField("/title", testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.UpdateCandidate(ctx, locked); err != nil {
		t.Fatal(err)
	}
	second, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-replay"})
	if err != nil {
		t.Fatalf("幂等重放失败: %v", err)
	}
	if second.ChangeSetID != first.ChangeSetID {
		t.Fatalf("幂等重放返回不同 ChangeSet: %q vs %q", second.ChangeSetID, first.ChangeSetID)
	}
}

func TestConfirmArchiveWritesAssetsIdempotently(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	session := seedArchiveSession(t, bridge, project.ID)
	session, err = session.Pause(creationdomain.BlockFormalWrite, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.SaveRunSlot(ctx, session); err != nil {
		t.Fatal(err)
	}
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{
		Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Title != "潮汐关" || preview.CharacterCount != 1 || preview.OutlineCount < 4 {
		t.Fatalf("preview = %#v", preview)
	}
	first, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	storedSession, err := bridge.Session(ctx, project.ID)
	if err != nil || storedSession.ContentVersion != first.Session.ContentVersion || storedSession.RunSlotVersion != first.Session.RunSlotVersion || storedSession.BlockingReason != creationdomain.BlockNone {
		t.Fatalf("入档版本或暂停原因未对齐: %+v, %v", storedSession, err)
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 1 || characters[0].Name != "阿绫" {
		t.Fatalf("characters = %#v, err=%v", characters, err)
	}
	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil || len(nodes) < 4 {
		t.Fatalf("outline = %#v, err=%v", nodes, err)
	}
	levels := map[string]string{}
	for _, node := range nodes {
		levels[node.ID] = string(node.Level)
	}
	for _, node := range nodes {
		if string(node.Level) == "chapter" && levels[node.ParentID] != "arc" {
			t.Fatalf("chapter %q parent level = %q, want arc", node.Title, levels[node.ParentID])
		}
	}
	updated, err := raw.Project(ctx)
	if err != nil || updated.Title != "潮汐关" {
		t.Fatalf("project = %#v, err=%v", updated, err)
	}
	workbench, err := raw.WorkbenchState(ctx)
	if err != nil || workbench.Stage != projectdomain.StageWriting || workbench.ArchiveStatus != "confirmed" {
		t.Fatalf("workbench = %#v, err=%v", workbench, err)
	}
	second, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "archive-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.ChangeSetID != first.ChangeSetID {
		t.Fatalf("idempotent change set = %q vs %q", second.ChangeSetID, first.ChangeSetID)
	}
	characters, err = raw.ListCharacters(ctx)
	if err != nil || len(characters) != 1 {
		t.Fatalf("characters after replay = %#v, err=%v", characters, err)
	}
}

func TestApplyCreationArchiveRollsBackPartialFailure(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	now := testClock{}.Now()
	session, err := creationdomain.NewSession("session-1", project.ID, "branch-1", now)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SaveIntent("守关人必须放走仇敌", now)
	if err != nil {
		t.Fatal(err)
	}
	session.Stage = creationdomain.StageArchiveConfirm
	session.SelectedDirectionCandidateID = "dir-1"
	changeSet, err := revisiondomain.NewChangeSet(
		"cs-fail", project.ID, "部分失败", "", "author",
		[]revisiondomain.PatchOperationSpec{
			{
				ID: "op-char", Kind: revisiondomain.OperationCreateCharacter, TargetID: "char-1",
				AfterValue: `{"name":"阿绫","role":"protagonist"}`, Reason: "人物",
			},
			{
				ID: "op-outline", Kind: revisiondomain.OperationCreateOutlineNode, TargetID: "outline-1",
				AfterValue: `{"parentId":"missing-parent","level":"volume","title":"坏节点","position":1}`,
				Reason:     "失败大纲",
			},
		},
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.RecordArchive("digest", "idem-fail", changeSet.ID, now)
	if err != nil {
		t.Fatal(err)
	}
	err = raw.ApplyCreationArchive(ctx, changeSet, session, nil, nil, projectdomain.WorkbenchState{
		Stage: projectdomain.StageWriting, ArchiveStatus: "confirmed", UpdatedAt: now,
	})
	if err == nil {
		t.Fatal("expected archive apply to fail")
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 0 {
		t.Fatalf("characters after rollback = %#v, err=%v", characters, err)
	}
	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil || len(nodes) != 0 {
		t.Fatalf("outline after rollback = %#v, err=%v", nodes, err)
	}
	workbench, err := raw.WorkbenchState(ctx)
	if err == nil && workbench.ArchiveStatus == "confirmed" {
		t.Fatalf("workbench should not be confirmed: %#v", workbench)
	}
}

func sqliteCreateStore(t *testing.T) (*sqlite.Store, projectdomain.Project) {
	t.Helper()
	project, err := projectdomain.NewProject("project-1", "长夜织境", "东方奇幻", "简介", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := (sqlite.Factory{}).Create(context.Background(), filepath.Join(t.TempDir(), "project.db"), project)
	if err != nil {
		t.Fatal(err)
	}
	store, ok := opened.(*sqlite.Store)
	if !ok {
		t.Fatalf("store type = %T", opened)
	}
	return store, project
}

func seedArchiveSession(t *testing.T, store Store, projectID string) creationdomain.Session {
	t.Helper()
	now := testClock{}.Now()
	session, err := creationdomain.NewSession("session-1", projectID, "branch-1", now)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SaveIntent("守关人必须放走仇敌", now)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := creationdomain.NewCandidate(
		"dir-set-1", session.ID, creationdomain.StageDirections, creationdomain.KindDirectionSet,
		1, "", "run-dir", "art-dir", directionPayload, "story_directions.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := creationdomain.NewCandidate(
		"proposal-1", session.ID, creationdomain.StageProposal, creationdomain.KindProposal,
		1, "", "run-prop", "art-prop", proposalPayload, "story_blueprint.schema", 2, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	people, err := creationdomain.NewCandidate(
		"people-1", session.ID, creationdomain.StagePeopleWorld, creationdomain.KindPeopleWorld,
		1, "", "run-cast", "art-cast", peopleWorldPayload, "story_cast.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	roadmap, err := creationdomain.NewCandidate(
		"roadmap-1", session.ID, creationdomain.StageRoadmap, creationdomain.KindRoadmap,
		1, "", "run-map", "art-map", roadmapPayload, "story_roadmap.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	session.Stage = creationdomain.StageArchiveConfirm
	session.SelectedDirectionCandidateID = dir.ID
	session.SelectedDirectionCardID = "d1"
	if session, err = store.CreateSession(context.Background(), session); err != nil {
		t.Fatal(err)
	}
	candidates := []creationdomain.Candidate{dir, proposal, people, roadmap}
	for _, candidate := range candidates {
		candidate = candidate.WithUpstream(liveUpstreamDigest(session, candidates, candidate.Stage))
		if err := store.SaveCandidate(context.Background(), candidate); err != nil {
			t.Fatal(err)
		}
	}
	return session
}
