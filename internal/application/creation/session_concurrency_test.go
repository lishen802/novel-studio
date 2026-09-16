package creation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// sessionWriteGate 在持久化前暂停指定写入，精确复现读取后到提交前的并发窗口。
type sessionWriteGate struct {
	Store
	beforeContent func(creationdomain.Session)
	beforeSlot    func(creationdomain.Session)
}

func (store sessionWriteGate) SaveSession(ctx context.Context, session creationdomain.Session) error {
	if store.beforeContent != nil {
		store.beforeContent(session)
	}
	return store.Store.SaveSession(ctx, session)
}

func (store sessionWriteGate) SaveRunSlot(ctx context.Context, session creationdomain.Session) error {
	if store.beforeSlot != nil {
		store.beforeSlot(session)
	}
	return store.Store.SaveRunSlot(ctx, session)
}

// awaitSessionWrite 只用超时防止测试挂起，不用等待时间决定并发顺序。
func awaitSessionWrite(t *testing.T, entered <-chan struct{}) {
	t.Helper()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("写入未到达受控阻塞点")
	}
}

func awaitSessionResult(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("写入未完成")
		return nil
	}
}

func TestConcurrentSessionCommandsRejectStaleContent(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	gate := sessionWriteGate{Store: bridge, beforeContent: func(session creationdomain.Session) {
		if session.AdvanceMode == creationdomain.ModeAutoAdvance {
			close(entered)
			<-release
		}
	}}
	service, err := New(Config{Store: gate, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.EnsureSession(ctx, project.ID, "main"); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		_, err := service.SetAdvanceMode(ctx, project.ID, creationdomain.ModeAutoAdvance)
		result <- err
	}()
	awaitSessionWrite(t, entered)
	saved, err := service.SaveIntent(ctx, project.ID, "并发保存的新意图")
	if err != nil {
		t.Fatal(err)
	}
	unblock()
	if err := awaitSessionResult(t, result); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("旧模式请求未返回冲突: %v", err)
	}
	latest, err := bridge.Session(ctx, project.ID)
	if err != nil || latest.IntentText != saved.IntentText || latest.ContentVersion != saved.ContentVersion || latest.AdvanceMode != creationdomain.ModeStepReview {
		t.Fatalf("作者修改丢失: %+v, %v", latest, err)
	}
}

func TestSessionSaveAndRunCompletionDoNotOverwriteEachOther(t *testing.T) {
	for _, holdAuthor := range []bool{true, false} {
		t.Run(map[bool]string{true: "作者保存较晚", false: "运行清槽较晚"}[holdAuthor], func(t *testing.T) {
			ctx := context.Background()
			raw, project := sqliteCreateStore(t)
			defer raw.Close()
			bridge := NewSQLiteStore(raw)
			entered, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unblock()
			gate := sessionWriteGate{Store: bridge}
			hold := func(creationdomain.Session) { close(entered); <-release }
			if holdAuthor {
				gate.beforeContent = hold
			} else {
				gate.beforeSlot = hold
			}
			service, err := New(Config{Store: gate, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
			if err != nil {
				t.Fatal(err)
			}
			session, err := service.EnsureSession(ctx, project.ID, "main")
			if err != nil {
				t.Fatal(err)
			}
			session, err = session.AttachRun("operation-a", "run-a", testClock{}.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err := bridge.SaveRunSlot(ctx, session); err != nil {
				t.Fatal(err)
			}
			writeIntent := func() error {
				_, err := service.SaveIntent(ctx, project.ID, "不能被运行回调覆盖的新意图")
				return err
			}
			clearSlot := func() error { return service.clearRunSlot(ctx, project.ID, "run-a") }
			first, second := writeIntent, clearSlot
			if !holdAuthor {
				first, second = clearSlot, writeIntent
			}
			result := make(chan error, 1)
			go func() { result <- first() }()
			awaitSessionWrite(t, entered)
			if err := second(); err != nil {
				t.Fatal(err)
			}
			unblock()
			if err := awaitSessionResult(t, result); err != nil {
				t.Fatal(err)
			}
			latest, err := bridge.Session(ctx, project.ID)
			if err != nil || latest.IntentText != "不能被运行回调覆盖的新意图" || latest.ActiveRunID != "" || latest.ActiveOperationID != "" {
				t.Fatalf("并发交错后状态错误: %+v, %v", latest, err)
			}
		})
	}
}

func TestLateCompletionCannotClearNewRun(t *testing.T) {
	ctx := context.Background()
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	session := seedIntentSession(t, service)
	old, err := session.AttachRun("operation-a", "run-a", testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRunSlot(ctx, old); err != nil {
		t.Fatal(err)
	}
	old, err = store.Session(ctx, session.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.clearRunSlot(ctx, session.ProjectID, "run-a"); err != nil {
		t.Fatal(err)
	}
	latest, err := store.Session(ctx, session.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	next, err := latest.AttachRun("operation-b", "run-b", testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRunSlot(ctx, next); err != nil {
		t.Fatal(err)
	}
	if err := service.clearRunSlot(ctx, session.ProjectID, "run-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.markInterrupted(ctx, old); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("旧恢复回调未被拒绝: %v", err)
	}
	if _, err := service.pauseSession(ctx, old, creationdomain.BlockConflict); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("旧暂停回调未被拒绝: %v", err)
	}
	latest, err = store.Session(ctx, session.ProjectID)
	if err != nil || latest.ActiveRunID != "run-b" || latest.BlockingReason != creationdomain.BlockNone {
		t.Fatalf("旧回调影响新运行: %+v, %v", latest, err)
	}
}

// C06：Provider 阻塞期间作者切换推进模式并锁定字段；运行完成后新编辑与锁都保留，
// 结果物化但不替作者接受候选、不切换当前阶段。
func TestBlockedRunKeepsModeSwitchAndFieldLock(t *testing.T) {
	ctx := context.Background()
	kernel := newFakeKernel()
	kernel.block = make(chan struct{})
	kernel.started = make(chan struct{})
	service, store, _ := newTestService(t, kernel, nil)
	people := seedPeopleWorldWithUpstream(t, store)
	if _, err := service.AcceptCandidate(ctx, "project-1", people.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AdvanceStage(ctx, "project-1"); err != nil {
		t.Fatal(err)
	}

	request := generateRequest(creationdomain.StageRoadmap)
	preview, err := service.PreviewGenerate(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	request.ConfirmationDigest = preview.ConfirmationDigest
	request.IdempotencyKey = "roadmap-blocked"
	if _, err := service.ConfirmGenerate(ctx, request); err != nil {
		t.Fatal(err)
	}
	select {
	case <-kernel.started:
	case <-time.After(2 * time.Second):
		t.Fatal("execute did not start")
	}
	// 运行期间作者切换模式并锁定人物候选字段。
	if _, err := service.SetAdvanceMode(ctx, "project-1", creationdomain.ModeAutoAdvance); err != nil {
		t.Fatal(err)
	}
	locked, err := service.LockCandidateField(ctx, "project-1", people.ID, "/characters/@char-1/summary")
	if err != nil {
		t.Fatal(err)
	}
	close(kernel.block)
	if !waitFor(t, func() bool {
		session, sessErr := store.Session(ctx, "project-1")
		return sessErr == nil && session.ActiveRunID == ""
	}) {
		t.Fatal("run did not finish")
	}

	session, err := store.Session(ctx, "project-1")
	if err != nil {
		t.Fatal(err)
	}
	if session.AdvanceMode != creationdomain.ModeAutoAdvance {
		t.Fatalf("运行覆盖了作者在运行期间切换的模式: %q", session.AdvanceMode)
	}
	// 结果物化：路线图候选存在，但作者当前阶段与已接受的人物候选不被抢走。
	candidates, err := store.ListCandidates(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := currentCandidate(candidates, creationdomain.StageRoadmap); !ok {
		t.Fatal("运行结果未物化为路线图候选")
	}
	if session.Stage != creationdomain.StageRoadmap {
		t.Fatalf("运行完成不应替作者推进阶段: %q", session.Stage)
	}
	stored, err := store.Candidate(ctx, people.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != locked.Status {
		t.Fatalf("运行改变了作者已接受的人物候选状态: %q", stored.Status)
	}
	current, ok := currentCandidate(candidates, creationdomain.StagePeopleWorld)
	if !ok || !current.Locked("/characters/@char-1/summary") {
		t.Fatalf("运行期间设置的锁未保留: %#v", current.LockMap)
	}
	workbench, err := service.Workbench(ctx, "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	if workbench.Stage != creationdomain.StageRoadmap || workbench.PeopleWorld.CandidateID != people.ID {
		t.Fatalf("工作台当前对象被运行抢走: stage=%q people=%q", workbench.Stage, workbench.PeopleWorld.CandidateID)
	}
}
