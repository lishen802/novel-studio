package creation

import (
	"context"
	"errors"
	"sync"
	"testing"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// archiveDependencyWindow 在保存意图后、刷新影响前插入确认，确定性复现落库窗口。
type archiveDependencyWindow struct {
	Store
	afterSave func() error
}

func (store *archiveDependencyWindow) SaveSession(ctx context.Context, session creationdomain.Session) error {
	if err := store.Store.SaveSession(ctx, session); err != nil {
		return err
	}
	return store.afterSave()
}

func TestArchiveFreshPreviewRejectsUnmarkedDependencyChange(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	session := seedArchiveSession(t, bridge, project.ID)
	store := &archiveDependencyWindow{Store: bridge}
	service, err := New(Config{Store: store, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	store.afterSave = func() error {
		called = true
		if _, err := service.PreviewConfirmArchive(ctx, project.ID); !errors.Is(err, creationdomain.ErrImpactUnresolved) {
			t.Fatalf("新预览没有拒绝未标记的过期依赖: %v", err)
		}
		latest, err := bridge.Session(ctx, project.ID)
		if err != nil {
			return err
		}
		candidates, err := bridge.ListCandidates(ctx, session.ID)
		if err != nil {
			return err
		}
		// 即使提交按当前数据算出的摘要，也不能绕过确认端的依赖检查。
		digest, err := archiveDigest(latest, candidates, project)
		if err != nil {
			return err
		}
		_, err = service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: digest, IdempotencyKey: "dependency-window"})
		if !errors.Is(err, creationdomain.ErrImpactUnresolved) {
			t.Fatalf("确认放行过期依赖: %v", err)
		}
		return nil
	}
	if _, err := service.SaveIntent(ctx, project.ID, "已保存的新意图"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("没有覆盖保存窗口")
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 0 {
		t.Fatalf("拒绝后留下正式人物: %v, %v", characters, err)
	}
}

func TestArchiveReadinessValidatesFrozenDependencies(t *testing.T) {
	for _, stage := range []creationdomain.Stage{creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap} {
		for _, digest := range []string{"", "expired"} {
			t.Run(string(stage)+"/"+digest, func(t *testing.T) {
				_, store, _ := newTestService(t, newFakeKernel(), nil)
				session := seedArchiveSession(t, store, "project-1")
				candidates, err := store.ListCandidates(context.Background(), session.ID)
				if err != nil {
					t.Fatal(err)
				}
				for i := range candidates {
					if candidates[i].Stage == stage {
						candidates[i].UpstreamDigest = digest
					}
				}
				if err := validateArchiveReadiness(session, candidates); !errors.Is(err, creationdomain.ErrImpactUnresolved) {
					t.Fatalf("未拒绝未知/过期依赖: %v", err)
				}
			})
		}
	}
}

func TestArchiveDependencyMetadataCAS(t *testing.T) {
	for _, field := range []string{"upstream", "exception", "severity", "refs"} {
		t.Run(field, func(t *testing.T) {
			ctx := context.Background()
			raw, project := sqliteCreateStore(t)
			defer raw.Close()
			bridge := NewSQLiteStore(raw)
			session := seedArchiveSession(t, bridge, project.ID)
			mutate := func(c creationdomain.Candidate) (creationdomain.Candidate, error) {
				switch field {
				case "upstream":
					c.UpstreamDigest = "changed"
				case "exception":
					c.ExceptedUpstreamDigest = "changed"
				case "severity":
					c.ImpactSeverity = creationdomain.ImpactMust
				case "refs":
					c.ImpactRefs = []string{"changed"}
				}
				return c, nil
			}
			service, err := New(Config{Store: archiveCandidateMutator{Store: bridge, mutate: mutate}, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
			if err != nil {
				t.Fatal(err)
			}
			preview, err := service.PreviewConfirmArchive(ctx, project.ID)
			if err != nil {
				t.Fatal(err)
			}
			result, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: field})
			if !errors.Is(err, creationdomain.ErrConfirmationStale) {
				t.Fatalf("依赖字段变化没有回滚: %+v, %v", result, err)
			}
			latest, err := bridge.Session(ctx, project.ID)
			if err != nil || latest.ArchiveChangeSetID != "" || latest.ContentVersion != session.ContentVersion {
				t.Fatalf("失败后会话被写入: %+v, %v", latest, err)
			}
			characters, err := raw.ListCharacters(ctx)
			if err != nil || len(characters) != 0 {
				t.Fatalf("失败后留下人物: %+v, %v", characters, err)
			}
			nodes, err := raw.ListOutlineNodes(ctx)
			if err != nil || len(nodes) != 0 {
				t.Fatalf("失败后留下大纲: %+v, %v", nodes, err)
			}
		})
	}
}

func TestArchiveDependencyExceptions(t *testing.T) {
	for _, mode := range []string{"soft", "expired", "hard", "locked", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			raw, project := sqliteCreateStore(t)
			defer raw.Close()
			bridge := NewSQLiteStore(raw)
			session := seedArchiveSession(t, bridge, project.ID)
			candidates, err := bridge.ListCandidates(ctx, session.ID)
			if err != nil {
				t.Fatal(err)
			}
			candidate, _ := currentCandidate(candidates, creationdomain.StageProposal)
			candidate.UpstreamDigest = "previous-input"
			candidate, err = candidate.MarkStale("软关联变化", []string{"intent"}, creationdomain.ImpactMaybe, testClock{}.Now())
			if err != nil {
				t.Fatal(err)
			}
			candidate, err = candidate.KeepException(liveUpstreamDigest(session, candidates, candidate.Stage), testClock{}.Now())
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "expired":
				candidate.ExceptedUpstreamDigest = "previous-exception"
			case "hard":
				candidate.ImpactSeverity = creationdomain.ImpactMust
			case "locked":
				candidate.LockMap = map[string]bool{"/title": true}
			case "unknown":
				candidate.UpstreamDigest = ""
			}
			if err := bridge.UpdateCandidate(ctx, candidate); err != nil {
				t.Fatal(err)
			}
			service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
			if err != nil {
				t.Fatal(err)
			}
			preview, err := service.PreviewConfirmArchive(ctx, project.ID)
			if mode != "soft" {
				if !errors.Is(err, creationdomain.ErrImpactUnresolved) {
					t.Fatalf("非法例外放行: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: mode}); err != nil {
				t.Fatal(err)
			}
			confirmed, err := bridge.Candidate(ctx, candidate.ID)
			if err != nil || confirmed.Status != creationdomain.StatusConfirmed || confirmed.ExceptedUpstreamDigest != candidate.ExceptedUpstreamDigest {
				t.Fatalf("例外入档丢失证据: %+v, %v", confirmed, err)
			}
		})
	}
}

func TestArchiveDigestBindsDependencyMetadata(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	session := seedArchiveSession(t, bridge, project.ID)
	base, err := bridge.ListCandidates(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	original, err := archiveDigest(session, base, project)
	if err != nil {
		t.Fatal(err)
	}
	for i := range base {
		for _, field := range []string{"upstream", "exception", "severity", "refs", "status"} {
			t.Run(base[i].ID+"/"+field, func(t *testing.T) {
				changed := append([]creationdomain.Candidate(nil), base...)
				switch field {
				case "upstream":
					changed[i].UpstreamDigest = "changed"
				case "exception":
					changed[i].ExceptedUpstreamDigest = "changed"
				case "severity":
					changed[i].ImpactSeverity = creationdomain.ImpactMaybe
				case "refs":
					changed[i].ImpactRefs = []string{"changed"}
				case "status":
					changed[i].Status = creationdomain.StatusSelected
				}
				digest, err := archiveDigest(session, changed, project)
				if err != nil || digest == original {
					t.Fatalf("摘要未绑定依赖字段: %s, %v", digest, err)
				}
			})
		}
	}
}

// archiveNewHeadStore 插入更高版本而不改旧行，验证事务保护当前候选的选择结果。
type archiveNewHeadStore struct{ Store }

func (store archiveNewHeadStore) ApplyArchive(ctx context.Context, apply ArchiveApply) error {
	candidate := apply.Candidates[0]
	candidate.ID = "new-head"
	candidate.Version++
	if err := store.SaveCandidate(ctx, candidate); err != nil {
		return err
	}
	return store.Store.ApplyArchive(ctx, apply)
}

func TestArchiveRejectsChangedDependencyHead(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	service, err := New(Config{Store: archiveNewHeadStore{bridge}, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "new-head"})
	if !errors.Is(err, creationdomain.ErrConfirmationStale) {
		t.Fatalf("当前依赖候选已变化却仍入档: %v", err)
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 0 {
		t.Fatalf("失败后留下人物: %+v, %v", characters, err)
	}
}

func TestArchiveRejectsIneligibleDirection(t *testing.T) {
	cases := map[string]struct {
		want   error
		mutate func(creationdomain.Candidate) (creationdomain.Candidate, error)
	}{
		"质量阻断": {want: creationdomain.ErrQualityBlocked, mutate: func(candidate creationdomain.Candidate) (creationdomain.Candidate, error) {
			return candidate.WithQuality(creationdomain.QualityBlocked), nil
		}},
		"硬影响": {want: creationdomain.ErrImpactUnresolved, mutate: func(candidate creationdomain.Candidate) (creationdomain.Candidate, error) {
			return candidate.MarkStale("创作意图改变", []string{"intent"}, creationdomain.ImpactMust, testClock{}.Now())
		}},
		"未知依赖": {want: creationdomain.ErrImpactUnresolved, mutate: func(candidate creationdomain.Candidate) (creationdomain.Candidate, error) {
			candidate.UpstreamDigest = ""
			return candidate, nil
		}},
	}
	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			raw, project := sqliteCreateStore(t)
			defer raw.Close()
			bridge := NewSQLiteStore(raw)
			session := seedArchiveSession(t, bridge, project.ID)
			direction, err := bridge.Candidate(ctx, session.SelectedDirectionCandidateID)
			if err != nil {
				t.Fatal(err)
			}
			direction, err = test.mutate(direction)
			if err != nil {
				t.Fatal(err)
			}
			if err := bridge.UpdateCandidate(ctx, direction); err != nil {
				t.Fatal(err)
			}
			service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.PreviewConfirmArchive(ctx, project.ID); !errors.Is(err, test.want) {
				t.Fatalf("无效方向返回错误 = %v, want %v", err, test.want)
			}
		})
	}
}

func TestDirectionSoftImpactExceptionCanArchive(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	session := seedArchiveSession(t, bridge, project.ID)
	direction, err := bridge.Candidate(ctx, session.SelectedDirectionCandidateID)
	if err != nil {
		t.Fatal(err)
	}
	direction.UpstreamDigest = "previous-intent"
	direction, err = direction.MarkStale("软偏好变化", []string{"intent"}, creationdomain.ImpactMaybe, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	direction, err = direction.KeepException(liveUpstreamDigest(session, nil, creationdomain.StageDirections), testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.UpdateCandidate(ctx, direction); err != nil {
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
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "direction-soft-exception"}); err != nil {
		t.Fatal(err)
	}
}

func TestAdvanceRejectsIneligibleDirection(t *testing.T) {
	for _, blocked := range []bool{true, false} {
		t.Run(map[bool]string{true: "blocked", false: "stale"}[blocked], func(t *testing.T) {
			ctx := context.Background()
			raw, project := sqliteCreateStore(t)
			defer raw.Close()
			bridge := NewSQLiteStore(raw)
			session := seedArchiveSession(t, bridge, project.ID)
			latest, err := bridge.Session(ctx, project.ID)
			if err != nil {
				t.Fatal(err)
			}
			latest.Stage = creationdomain.StageDirections
			if err := bridge.SaveSession(ctx, latest); err != nil {
				t.Fatal(err)
			}
			direction, err := bridge.Candidate(ctx, session.SelectedDirectionCandidateID)
			if err != nil {
				t.Fatal(err)
			}
			want := creationdomain.ErrImpactUnresolved
			if blocked {
				direction = direction.WithQuality(creationdomain.QualityBlocked)
				want = creationdomain.ErrQualityBlocked
			} else {
				direction, err = direction.MarkStale("创作意图改变", []string{"intent"}, creationdomain.ImpactMust, testClock{}.Now())
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := bridge.UpdateCandidate(ctx, direction); err != nil {
				t.Fatal(err)
			}
			service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.AdvanceStage(ctx, project.ID); !errors.Is(err, want) {
				t.Fatalf("无效方向推进错误 = %v, want %v", err, want)
			}
		})
	}
}

// archiveDirectionMutator 在预览校验后改变选中方向，验证事务必须按方向完整快照回滚。
type archiveDirectionMutator struct {
	Store
	mutate func(creationdomain.Candidate) (creationdomain.Candidate, error)
}

func (store archiveDirectionMutator) ApplyArchive(ctx context.Context, apply ArchiveApply) error {
	direction := apply.Verify[0]
	changed, err := store.mutate(direction)
	if err != nil {
		return err
	}
	if err := store.UpdateCandidate(ctx, changed); err != nil {
		return err
	}
	return store.Store.ApplyArchive(ctx, apply)
}

func TestArchiveDirectionEligibilityCAS(t *testing.T) {
	for _, kind := range []string{"blocked", "stale"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			raw, project := sqliteCreateStore(t)
			defer raw.Close()
			bridge := NewSQLiteStore(raw)
			seedArchiveSession(t, bridge, project.ID)
			mutate := func(candidate creationdomain.Candidate) (creationdomain.Candidate, error) {
				if kind == "blocked" {
					return candidate.WithQuality(creationdomain.QualityBlocked), nil
				}
				return candidate.MarkStale("创作意图改变", []string{"intent"}, creationdomain.ImpactMust, testClock{}.Now())
			}
			service, err := New(Config{Store: archiveDirectionMutator{Store: bridge, mutate: mutate}, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
			if err != nil {
				t.Fatal(err)
			}
			preview, err := service.PreviewConfirmArchive(ctx, project.ID)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "direction-" + kind})
			if !errors.Is(err, creationdomain.ErrConfirmationStale) {
				t.Fatalf("方向有效性变化没有使确认过期: %v", err)
			}
			characters, readErr := raw.ListCharacters(ctx)
			if readErr != nil || len(characters) != 0 {
				t.Fatalf("失败确认留下正式人物: %+v, %v", characters, readErr)
			}
		})
	}
}

// archiveOverlapStore 在外层确认进入事务前执行同一请求，确定性复现两个请求都读到未入档会话。
type archiveOverlapStore struct {
	Store
	beforeApply func() error
}

func (store *archiveOverlapStore) ApplyArchive(ctx context.Context, apply ArchiveApply) error {
	if store.beforeApply != nil {
		if err := store.beforeApply(); err != nil {
			return err
		}
	}
	return store.Store.ApplyArchive(ctx, apply)
}

func TestOverlappingArchiveRequestsReturnSameReceipt(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	store := &archiveOverlapStore{Store: bridge}
	service, err := New(Config{Store: store, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	request := ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "overlap"}
	var winner ArchiveResult
	store.beforeApply = func() error {
		store.beforeApply = nil
		var innerErr error
		winner, innerErr = service.ConfirmArchive(ctx, request)
		return innerErr
	}
	replay, err := service.ConfirmArchive(ctx, request)
	if err != nil {
		t.Fatalf("重叠请求未返回成功回执: %v", err)
	}
	if winner.ChangeSetID == "" || replay.ChangeSetID != winner.ChangeSetID {
		t.Fatalf("重叠请求返回不同回执: winner=%q replay=%q", winner.ChangeSetID, replay.ChangeSetID)
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 1 {
		t.Fatalf("重叠请求重复或遗漏物化人物: %+v, %v", characters, err)
	}
}

func TestOverlappingArchiveDoesNotHideDifferentKeyConflict(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	store := &archiveOverlapStore{Store: bridge}
	service, err := New(Config{Store: store, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	outer := ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "loser"}
	winner := outer
	winner.IdempotencyKey = "winner"
	store.beforeApply = func() error {
		store.beforeApply = nil
		_, innerErr := service.ConfirmArchive(ctx, winner)
		return innerErr
	}
	if result, err := service.ConfirmArchive(ctx, outer); err == nil || result.ChangeSetID != "" {
		t.Fatalf("不同幂等键被错误当作成功重放: %+v, %v", result, err)
	}
}

type archiveBarrierStore struct {
	Store
	arrived sync.WaitGroup
}

func (store *archiveBarrierStore) ApplyArchive(ctx context.Context, apply ArchiveApply) error {
	store.arrived.Done()
	store.arrived.Wait()
	return store.Store.ApplyArchive(ctx, apply)
}

func TestConcurrentArchiveRequestsReturnSameReceipt(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	store := &archiveBarrierStore{Store: bridge}
	store.arrived.Add(2)
	service, err := New(Config{Store: store, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	request := ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "concurrent"}
	type outcome struct {
		result ArchiveResult
		err    error
	}
	outcomes := make(chan outcome, 2)
	for range 2 {
		go func() {
			result, err := service.ConfirmArchive(ctx, request)
			outcomes <- outcome{result: result, err: err}
		}()
	}
	first, second := <-outcomes, <-outcomes
	if first.err != nil || second.err != nil {
		t.Fatalf("并发相同请求未全部成功: first=%v second=%v", first.err, second.err)
	}
	if first.result.ChangeSetID == "" || first.result.ChangeSetID != second.result.ChangeSetID {
		t.Fatalf("并发相同请求返回不同回执: first=%q second=%q", first.result.ChangeSetID, second.result.ChangeSetID)
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 1 {
		t.Fatalf("并发相同请求重复物化人物: %+v, %v", characters, err)
	}
}
