package creation

import (
	"context"
	"errors"
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
