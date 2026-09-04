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
