package creation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
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

func TestPreviewArchiveRejectsUnacceptedCandidate(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	session := seedArchiveSession(t, bridge, project.ID)

	proposal, err := bridge.Candidate(ctx, "proposal-1")
	if err != nil {
		t.Fatal(err)
	}
	proposal.Status = creationdomain.StatusDraft
	if err := bridge.UpdateCandidate(ctx, proposal); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.PreviewConfirmArchive(ctx, session.ProjectID); !errors.Is(err, creationdomain.ErrCandidateAcceptanceRequired) {
		t.Fatalf("preview with unaccepted proposal error = %v", err)
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
	chapter, err := projectdomain.NewChapter("replay-body", project.ID, "入档后保存正文", 1, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = raw.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	committed, err := chapter.Commit("已经开始创作的正文", chapter.Revision, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = raw.CommitChapter(ctx, committed, chapter); err != nil {
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

func TestConfirmArchiveRejectsV2RoadmapLabeledAsV3(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSessionWithRoadmapSchema(t, bridge, project.ID, peopleWorldPayload, roadmapPayload, 3)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{
		Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewConfirmArchive(ctx, project.ID); err == nil {
		t.Fatal("缺少规划目标的 v2 载荷不能按 v3 入档")
	}
}

func TestConfirmArchiveDoesNotInventPlanningForV2Arcs(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{
		Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now(),
	}); err != nil {
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
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "v2-no-planning",
	}); err != nil {
		t.Fatal(err)
	}
	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		if node.Level == storydomain.LevelArc && node.PlanningJSON != "" && node.PlanningJSON != "{}" {
			t.Fatalf("旧 v2 弧被伪造了规划目标: %s", node.PlanningJSON)
		}
	}
}

func TestConfirmArchivePreservesArcPlanningGoals(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSessionWithRoadmapSchema(t, bridge, project.ID, peopleWorldPayload, strings.Replace(roadmapPayloadV3, `"related_refs":["arc-1"]`, `"related_refs":["arc-1","ch-3","character:char-1"]`, 1), 3)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{
		Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	locked, err := service.LockCandidateField(ctx, project.ID, "roadmap-1", "/arcs/@arc-1/reader_expectation")
	if err != nil {
		t.Fatal(err)
	}
	edited, err := service.SaveFields(ctx, project.ID, SaveFieldsRequest{
		CandidateID: locked.ID, Changes: map[string]string{"/arcs/@arc-1/stage_payoff": "开城救人后公开承担代价"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptCandidate(ctx, project.ID, edited.ID); err != nil {
		t.Fatal(err)
	}
	// 新版本必须重新完成逐项审阅，旧版本的接受不会继承。
	if _, err := service.AcceptAllReview(ctx, project.ID, edited.ID); err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "arc-goals",
	}); err != nil {
		t.Fatal(err)
	}
	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var arc storydomain.OutlineNode
	for _, node := range nodes {
		if node.Level == storydomain.LevelArc {
			arc = node
			break
		}
	}
	if arc.ID == "" || !strings.Contains(arc.PlanningJSON, "读者应感到守关人将被亲情撕裂") || !strings.Contains(arc.PlanningJSON, "开城救人后公开承担代价") ||
		!strings.Contains(arc.PlanningJSON, "cmt-1") || !strings.Contains(arc.PlanningJSON, "潮汐神谕是否被篡改") {
		t.Fatalf("故事弧规划目标未无损入档: %+v", arc)
	}
	var planning struct {
		ReferenceBindings map[string]struct {
			ID    string `json:"id"`
			Table string `json:"table"`
		} `json:"reference_bindings"`
	}
	if err := json.Unmarshal([]byte(arc.PlanningJSON), &planning); err != nil {
		t.Fatal(err)
	}
	if planning.ReferenceBindings["arc-1"].ID != arc.ID || planning.ReferenceBindings["arc-1"].Table != "outline_nodes" {
		t.Fatal("承诺缺少正式引用映射")
	}
	nodeIDs := map[string]bool{}
	for _, n := range nodes {
		nodeIDs[n.ID] = true
	}
	if !nodeIDs[planning.ReferenceBindings["ch-3"].ID] {
		t.Fatal("前向章节引用未落到正式节点")
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(characters) == 0 || planning.ReferenceBindings["character:char-1"].ID != characters[0].ID {
		t.Fatal("人物引用未落到正式人物")
	}
	view, err := service.Workbench(ctx, project.ID, "branch-1")
	if err != nil || len(view.Roadmap.Arcs) != 1 || view.Roadmap.Arcs[0].ReaderExpectation == "" ||
		view.Roadmap.Arcs[0].StagePayoff != "开城救人后公开承担代价" || len(view.Roadmap.Arcs[0].Commitments) != 1 {
		t.Fatalf("工作台未投影规划目标: %+v, %v", view.Roadmap, err)
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

func TestConfirmArchivePreservesCompleteTypedAssetRevisions(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	const completePeopleWorld = `{"characters":[{"id":"char-a","name":"阿绫","role":"protagonist","aliases":["守关人"],"summary":"潮汐关最后的守将","motivation":"赎罪","fear":"再失一城","secret":"曾放走仇敌","arc":"学会取舍"},{"id":"char-b","name":"陆衡","role":"antagonist","aliases":["盐使"],"summary":"敌船统领","motivation":"夺关","fear":"身份败露","secret":"阿绫旧友","arc":"从利用到牺牲"}],"relationships":[{"id":"rel-1","source_character_ref":"char-a","target_character_ref":"char-b","kind":"rival","label":"旧友成敌","strength":-70}],"factions":[{"name":"潮汐卫","summary":"守卫关隘的军团","goal":"守住航路"}],"locations":[{"name":"潮汐关","kind":"settlement","summary":"控制航路的关城"}],"world_rules":[{"name":"潮汐律","description":"潮位决定航路开闭","scope_type":"project"}],"forbidden":[],"omission_reasons":["无额外禁写项"]}`
	seedArchiveSessionWithPayloads(t, bridge, project.ID, completePeopleWorld, roadmapPayload)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{
		Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now(),
	}); err != nil {
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
	result, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "complete-revisions",
	})
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err := raw.ChangeSet(ctx, result.ChangeSetID)
	if err != nil {
		t.Fatal(err)
	}
	typedPayloads := map[string]string{}
	for _, operation := range changeSet.Operations {
		switch operation.Kind {
		case revisiondomain.OperationCreateCharacter, revisiondomain.OperationCreateRelationship,
			revisiondomain.OperationCreateFaction, revisiondomain.OperationCreateLocation,
			revisiondomain.OperationCreateWorldRule, revisiondomain.OperationCreateOutlineNode:
			typedPayloads[operation.TargetID] = operation.AfterValue
		}
	}
	persistedPayloads := map[string]string{}
	remember := func(id string, value any) {
		t.Helper()
		payload, err := encodeJSON(value)
		if err != nil {
			t.Fatal(err)
		}
		persistedPayloads[id] = payload
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, character := range characters {
		remember(character.ID, map[string]any{
			"name": character.Name, "aliases": character.Aliases, "role": character.Role,
			"summary": character.Summary, "motivation": character.Motivation, "fear": character.Fear,
			"secret": character.Secret, "arc": character.Arc,
		})
	}
	relationships, err := raw.ListRelationships(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, relationship := range relationships {
		remember(relationship.ID, map[string]any{
			"sourceCharacterId": relationship.SourceCharacterID, "targetCharacterId": relationship.TargetCharacterID,
			"kind": relationship.Kind, "label": relationship.Label, "strength": relationship.Strength,
		})
	}
	factions, err := raw.ListFactions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, faction := range factions {
		remember(faction.ID, map[string]string{"name": faction.Name, "summary": faction.Summary, "goal": faction.Goal})
	}
	locations, err := raw.ListLocations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, location := range locations {
		remember(location.ID, map[string]any{"name": location.Name, "kind": location.Kind, "summary": location.Summary})
	}
	rules, err := raw.ListWorldRules(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range rules {
		remember(rule.ID, map[string]any{"name": rule.Name, "description": rule.Description, "scopeType": rule.ScopeType})
	}
	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes {
		payload := map[string]any{
			"level": node.Level, "title": node.Title, "summary": node.Summary, "goal": node.Goal,
			"conflict": node.Conflict, "outcome": node.Outcome, "position": node.Position,
		}
		if node.ParentID != "" {
			payload["parentId"] = node.ParentID
		}
		if node.PlanningJSON != "" && node.PlanningJSON != "{}" {
			var planning any
			if err := json.Unmarshal([]byte(node.PlanningJSON), &planning); err != nil {
				t.Fatal(err)
			}
			payload["planning"] = planning
		}
		remember(node.ID, payload)
	}
	wantKinds := map[string]int{
		"character": 2, "relationship": 1, "faction": 1, "location": 1, "world_rule": 1, "outline_node": 6,
	}
	gotKinds := map[string]int{}
	for _, operation := range changeSet.Operations {
		if operation.Kind != revisiondomain.OperationCreateStoryAsset {
			continue
		}
		var envelope struct {
			Kind, SourceCandidateID, SourceRunID, SourceArtifactID string
			TypedTable, TypedID, PayloadJSON                       string
		}
		if err := json.Unmarshal([]byte(operation.AfterValue), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.TypedID == "" {
			continue
		}
		wantPayload, ok := typedPayloads[envelope.TypedID]
		if !ok {
			t.Fatalf("资产 %s 未关联类型表写入: %+v", operation.TargetID, envelope)
		}
		envelopePayload := wantPayload
		if envelope.Kind == "character" || envelope.Kind == "outline_node" {
			var fields map[string]any
			if err := json.Unmarshal([]byte(wantPayload), &fields); err != nil {
				t.Fatal(err)
			}
			candidate, err := raw.CreationCandidate(ctx, envelope.SourceCandidateID)
			if err != nil {
				t.Fatal(err)
			}
			fields["_sourceLocks"] = candidate.LockMap
			encoded, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			envelopePayload = string(encoded)
		}

		if envelope.PayloadJSON != envelopePayload {
			t.Fatalf("资产 %s 操作载荷不完整:\n got %s\nwant %s", operation.TargetID, envelope.PayloadJSON, wantPayload)
		}
		if persistedPayloads[envelope.TypedID] != wantPayload {
			t.Fatalf("类型表 %s 与规范化写入载荷不一致:\n got %s\nwant %s", envelope.TypedID, persistedPayloads[envelope.TypedID], wantPayload)
		}
		asset, err := raw.StoryAsset(ctx, operation.TargetID)
		if err != nil {
			t.Fatal(err)
		}
		if asset.TypedID != envelope.TypedID || asset.TypedTable != envelope.TypedTable ||
			asset.SourceCandidateID != envelope.SourceCandidateID || asset.SourceRunID != envelope.SourceRunID ||
			asset.SourceArtifactID != envelope.SourceArtifactID {
			t.Fatalf("正式资产信封与入档来源不一致: %+v / %+v", asset, envelope)
		}
		revision, err := raw.StoryAssetRevision(ctx, asset.ID, asset.CurrentRevision)
		if err != nil {
			t.Fatal(err)
		}
		if revision.PayloadJSON != envelopePayload {
			t.Fatalf("资产 %s revision 载荷不完整:\n got %s\nwant %s", asset.ID, revision.PayloadJSON, wantPayload)
		}
		sum := sha256.Sum256([]byte(envelopePayload))
		if revision.ContentSHA256 != hex.EncodeToString(sum[:]) || revision.CreatedBy != "creation-archive" {
			t.Fatalf("资产 %s revision 审计信息不一致: %+v", asset.ID, revision)
		}
		gotKinds[string(asset.Kind)]++
	}
	for kind, want := range wantKinds {
		if gotKinds[kind] != want {
			t.Fatalf("%s 正式 revision 数量 = %d, want %d", kind, gotKinds[kind], want)
		}
	}
}

func TestConfirmArchivePersistsResolvedAnchorScopes(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	const peopleWithForbidden = `{"characters":[{"id":"char-a","name":"阿绫","role":"protagonist","aliases":["守关人"],"summary":"潮汐关最后的守将","motivation":"赎罪","fear":"再失一城","secret":"放过过仇敌","arc":"学会取舍"}],"relationships":[],"factions":[],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路开闭","scope_type":"project"}],"forbidden":["不得使用火器"],"omission_reasons":[]}`
	seedArchiveSessionWithPayloads(t, bridge, project.ID, peopleWithForbidden, roadmapPayload)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{
		Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now(),
	}); err != nil {
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
	result, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "anchor-scopes",
	})
	if err != nil {
		t.Fatal(err)
	}
	changeSet, err := raw.ChangeSet(ctx, result.ChangeSetID)
	if err != nil {
		t.Fatal(err)
	}
	proposalID := ""
	chapterIDs := map[string]string{}
	for _, operation := range changeSet.Operations {
		if operation.Kind == revisiondomain.OperationCreateStoryAsset {
			var value struct{ Kind string }
			if err := json.Unmarshal([]byte(operation.AfterValue), &value); err != nil {
				t.Fatal(err)
			}
			if value.Kind == "proposal" {
				proposalID = operation.TargetID
			}
		}
		if operation.Kind == revisiondomain.OperationCreateOutlineNode {
			var value struct {
				Level string
				Title string
			}
			if err := json.Unmarshal([]byte(operation.AfterValue), &value); err != nil {
				t.Fatal(err)
			}
			if value.Level == "chapter" {
				chapterIDs[value.Title] = operation.TargetID
			}
		}
	}
	if proposalID == "" || chapterIDs["空船"] == "" || chapterIDs["开关"] == "" {
		t.Fatalf("未建立锚点所需正式目标: proposal=%q chapters=%v", proposalID, chapterIDs)
	}
	anchors, err := raw.ListStoryAnchors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct{ kind, id string }{
		"现代武器":   {"proposal", proposalID},
		"钟楼必须三响": {"proposal", proposalID},
		"不得使用火器": {"proposal", proposalID},
		"钟楼三响":   {"roadmap_node", chapterIDs["空船"]},
		"潮汐律":    {"roadmap_node", chapterIDs["开关"]},
	}
	if len(anchors) != len(want) {
		t.Fatalf("正式锚点数量 = %d, want %d: %+v", len(anchors), len(want), anchors)
	}
	for _, anchor := range anchors {
		target, ok := want[anchor.Statement]
		if !ok || anchor.ScopeKind != target.kind || anchor.ScopeID != target.id {
			t.Fatalf("锚点作用域未解析: %+v, want=%+v", anchor, target)
		}
	}
}

func TestConfirmArchiveMapsRelationshipsByStableCharacterID(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	const stableCast = `{"characters":[{"id":"char-a","name":"无名","role":"protagonist","aliases":["甲"],"summary":"守关人","motivation":"守城","fear":"失守","secret":"旧友","arc":"承担"},{"id":"char-b","name":"无名","role":"antagonist","aliases":["乙"],"summary":"来犯者","motivation":"破城","fear":"失败","secret":"旧友","arc":"醒悟"}],"relationships":[{"id":"rel-1","source_character_ref":"char-a","target_character_ref":"char-b","kind":"rival","label":"同名宿敌","strength":-80}],"factions":[],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路","scope_type":"project"}],"forbidden":[],"omission_reasons":["无需其他分类"]}`
	seedArchiveSessionWithPayloads(t, bridge, project.ID, stableCast, roadmapPayload)
	if err := raw.SaveWorkbenchState(ctx, projectdomain.WorkbenchState{
		Stage: projectdomain.StageRoadmap, ArchiveStatus: "not_generated", UpdatedAt: testClock{}.Now(),
	}); err != nil {
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
	result, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "stable-relationship",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ChangeSetID == "" {
		t.Fatal("入档未返回 ChangeSet")
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	characterByAlias := map[string]string{}
	for _, character := range characters {
		for _, alias := range character.Aliases {
			characterByAlias[alias] = character.ID
		}
	}
	relationships, err := raw.ListRelationships(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(relationships) != 1 || relationships[0].SourceCharacterID != characterByAlias["甲"] ||
		relationships[0].TargetCharacterID != characterByAlias["乙"] {
		t.Fatalf("关系未按稳定候选 ID 映射: characters=%+v relationships=%+v", characters, relationships)
	}
}

func TestArchiveRejectsLegacyNameBasedRelationships(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	const legacyCast = `{"characters":[{"name":"阿绫","role":"protagonist","aliases":[],"summary":"守关人","motivation":"守城","fear":"失守","secret":"","arc":"承担"},{"name":"陆衡","role":"antagonist","aliases":[],"summary":"来犯者","motivation":"破城","fear":"失败","secret":"","arc":"醒悟"}],"relationships":[{"source_name":"阿绫","target_name":"陆衡","kind":"rival","label":"旧友成敌","strength":-70}],"factions":[],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路","scope_type":"project"}],"forbidden":[],"omission_reasons":["无需其他分类"]}`
	seedArchiveSessionWithCastVersion(t, bridge, project.ID, proposalPayload, legacyCast, roadmapPayload, 1)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewConfirmArchive(ctx, project.ID); !errors.Is(err, creationdomain.ErrStageIncomplete) {
		t.Fatalf("v1 姓名关系候选应阻断入档: %v", err)
	}
	if relationships, err := raw.ListRelationships(ctx); err != nil || len(relationships) != 0 {
		t.Fatalf("被拒绝的 v1 关系不应落库: %+v, %v", relationships, err)
	}
}

func TestArchiveRejectsUnresolvedChapterScopeHint(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	unresolved := strings.Replace(proposalPayload, `"scope_hint":"proposal"`, `"scope_hint":"chapter"`, 1)
	seedArchiveSessionWithAllPayloads(t, bridge, project.ID, unresolved, peopleWorldPayload, roadmapPayload)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewConfirmArchive(ctx, project.ID); !errors.Is(err, creationdomain.ErrStageIncomplete) {
		t.Fatalf("未解析 chapter scope_hint 应阻断入档预览: %v", err)
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
	return seedArchiveSessionWithPayloads(t, store, projectID, peopleWorldPayload, roadmapPayload)
}

func seedArchiveSessionWithPayloads(t *testing.T, store Store, projectID, peoplePayload, roadPayload string) creationdomain.Session {
	return seedArchiveSessionWithAllPayloads(t, store, projectID, proposalPayload, peoplePayload, roadPayload)
}

func seedArchiveSessionWithAllPayloads(t *testing.T, store Store, projectID, proposalValue, peoplePayload, roadPayload string) creationdomain.Session {
	return seedArchiveSessionWithCastVersion(t, store, projectID, proposalValue, peoplePayload, roadPayload, 2)
}

func seedArchiveSessionWithRoadmapSchema(t *testing.T, store Store, projectID, peoplePayload, roadPayload string, roadmapSchemaVersion int) creationdomain.Session {
	return seedArchiveSessionWithSchemas(t, store, projectID, proposalPayload, peoplePayload, roadPayload, 2, roadmapSchemaVersion)
}

func seedArchiveSessionWithCastVersion(t *testing.T, store Store, projectID, proposalValue, peoplePayload, roadPayload string, castSchemaVersion int) creationdomain.Session {
	return seedArchiveSessionWithSchemas(t, store, projectID, proposalValue, peoplePayload, roadPayload, castSchemaVersion, 1)
}

func seedArchiveSessionWithSchemas(t *testing.T, store Store, projectID, proposalValue, peoplePayload, roadPayload string, castSchemaVersion, roadmapSchemaVersion int) creationdomain.Session {
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
		1, "", "run-prop", "art-prop", proposalValue, "story_blueprint.schema", 2, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	people, err := creationdomain.NewCandidate(
		"people-1", session.ID, creationdomain.StagePeopleWorld, creationdomain.KindPeopleWorld,
		1, "", "run-cast", "art-cast", peoplePayload, "story_cast.schema", castSchemaVersion, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	roadmap, err := creationdomain.NewCandidate(
		"roadmap-1", session.ID, creationdomain.StageRoadmap, creationdomain.KindRoadmap,
		1, "", "run-map", "art-map", roadPayload, "story_roadmap.schema", roadmapSchemaVersion, now,
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
	for index, candidate := range candidates {
		candidate = candidate.WithUpstream(liveUpstreamDigest(session, candidates, candidate.Stage))
		if candidate.Stage == creationdomain.StageProposal || candidate.Stage == creationdomain.StagePeopleWorld || candidate.Stage == creationdomain.StageRoadmap {
			candidate, err = candidate.Confirm(now)
			if err != nil {
				t.Fatal(err)
			}
		}
		candidates[index] = candidate
		if err := store.SaveCandidate(context.Background(), candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := seedAcceptedReviews(store, candidates, now); err != nil {
		t.Fatal(err)
	}
	return session
}

// seedAcceptedReviews 为入档阶段候选写入显式全接受的审阅记录，代表作者已完成逐项审阅。
func seedAcceptedReviews(store Store, candidates []creationdomain.Candidate, now time.Time) error {
	for _, candidate := range candidates {
		switch candidate.Stage {
		case creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap:
		default:
			continue
		}
		review, err := creationdomain.NewCandidateReview(candidate, now)
		if err != nil {
			return err
		}
		refs := reviewRefs(candidate)
		all := append(append([]creationdomain.ReviewRef{}, refs[creationdomain.ReviewScopeEntity]...),
			refs[creationdomain.ReviewScopeSection]...)
		if len(all) == 0 {
			// 载荷没有可投影的审阅单位（例如旧 Schema 路线图）时不产生审阅要求。
			continue
		}
		review, err = review.AcceptAll(all, now)
		if err != nil {
			return err
		}
		if err := store.SaveReview(context.Background(), creationdomain.CandidateReview{}, review); err != nil {
			return err
		}
	}
	return nil
}

// acceptedArchiveSelection 为已完成逐项审阅的候选集合构造入档选择。
func acceptedArchiveSelection(session creationdomain.Session, candidates []creationdomain.Candidate) archiveSelection {
	_ = session
	selection := archiveSelection{Excluded: map[string]bool{}, ReviewVersions: map[string]int64{}}
	for _, stage := range []creationdomain.Stage{
		creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap,
	} {
		if current, ok := currentCandidate(candidates, stage); ok {
			selection.ReviewVersions[current.ID] = 1
		}
	}
	return selection
}

func TestArchiveRejectsMissingPlanningReferences(t *testing.T) {
	for _, ref := range []string{"missing-node", "character:missing-character"} {
		t.Run(ref, func(t *testing.T) {
			raw, project := sqliteCreateStore(t)
			defer raw.Close()
			bridge := NewSQLiteStore(raw)
			payload := strings.Replace(roadmapPayloadV3, `"related_refs":["arc-1"]`, `"related_refs":["`+ref+`"]`, 1)
			session := seedArchiveSessionWithRoadmapSchema(t, bridge, project.ID, peopleWorldPayload, payload, 3)
			candidates, err := bridge.ListCandidates(context.Background(), session.ID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := buildArchivePlan(session, candidates, project, &testIDs{}, acceptedArchiveSelection(session, candidates)); err == nil {
				t.Fatal("不存在的承诺目标不应入档")
			}
		})
	}
}
