package sqlite

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	storyassetdomain "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// TestCreationSessionRejectsLostUpdate 固定两个请求先读后写的顺序，验证旧快照不能覆盖已保存的意图。
func TestCreationSessionRejectsLostUpdate(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	session, err := creationdomain.NewSession("session-cas", project.ID, "main", time.Unix(500, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateCreationSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	author, err := store.CreationSession(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	mode := author
	author.IntentText = "作者刚保存的新意图"
	if err := store.SaveCreationSession(ctx, author); err != nil {
		t.Fatal(err)
	}
	mode.AdvanceMode = creationdomain.ModeAutoAdvance
	if err := store.SaveCreationSession(ctx, mode); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatal("旧内容快照应返回版本冲突，不能覆盖作者意图")
	}
	latest, err := store.CreationSession(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.IntentText != author.IntentText || latest.AdvanceMode != creationdomain.ModeStepReview {
		t.Fatalf("丢失作者修改: %+v", latest)
	}
}

// TestCreationSessionSeparatesContentAndRunSlot 覆盖两种提交顺序和延迟完成回调，不依赖调度时间。
func TestCreationSessionSeparatesContentAndRunSlot(t *testing.T) {
	for _, contentFirst := range []bool{true, false} {
		t.Run(fmt.Sprint(contentFirst), func(t *testing.T) {
			ctx := context.Background()
			store, project := createTestStore(t)
			defer store.Close()
			session, err := creationdomain.NewSession("session-slot", project.ID, "main", time.Unix(500, 0))
			if err != nil {
				t.Fatal(err)
			}
			session, err = store.CreateCreationSession(ctx, session)
			if err != nil {
				t.Fatal(err)
			}
			attached, err := session.AttachRun("operation-a", "run-a", session.UpdatedAt)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SaveCreationRunSlot(ctx, attached); err != nil {
				t.Fatal(err)
			}
			base, err := store.CreationSession(ctx, project.ID)
			if err != nil {
				t.Fatal(err)
			}
			author := base
			author.IntentText = "作者的新意图"
			author.AdvanceMode = creationdomain.ModeAutoAdvance
			author.SelectedDirectionCardID = "card-new"
			author.KeptDirectionIDs = []string{"card-kept"}
			cleared := base.ClearActiveRun(base.UpdatedAt)
			saveContent := func() {
				t.Helper()
				if err := store.SaveCreationSession(ctx, author); err != nil {
					t.Fatal(err)
				}
			}
			saveSlot := func() {
				t.Helper()
				if err := store.SaveCreationRunSlot(ctx, cleared); err != nil {
					t.Fatal(err)
				}
			}
			if contentFirst {
				saveContent()
				saveSlot()
			} else {
				saveSlot()
				saveContent()
			}
			latest, err := store.CreationSession(ctx, project.ID)
			if err != nil {
				t.Fatal(err)
			}
			if latest.IntentText != author.IntentText || latest.AdvanceMode != author.AdvanceMode || latest.SelectedDirectionCardID != author.SelectedDirectionCardID || len(latest.KeptDirectionIDs) != 1 || latest.KeptDirectionIDs[0] != "card-kept" || latest.ActiveRunID != "" || latest.ActiveOperationID != "" || latest.ContentVersion != base.ContentVersion+1 || latest.RunSlotVersion != base.RunSlotVersion+1 {
				t.Fatalf("两类写入互相覆盖: %+v", latest)
			}
			next, err := latest.AttachRun("operation-b", "run-b", latest.UpdatedAt)
			if err != nil {
				t.Fatal(err)
			}
			if err := store.SaveCreationRunSlot(ctx, next); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveCreationRunSlot(ctx, cleared); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
				t.Fatalf("旧完成回调未被拒绝: %v", err)
			}
			if err := store.SaveCreationRunSlot(ctx, attached); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
				t.Fatalf("旧启动回调未被拒绝: %v", err)
			}
			latest, err = store.CreationSession(ctx, project.ID)
			if err != nil || latest.ActiveRunID != "run-b" || latest.ActiveOperationID != "operation-b" {
				t.Fatalf("新运行被旧回调改变: %+v, %v", latest, err)
			}
		})
	}
}

// TestCreationSessionConcurrentInitialization 保证并发初始化返回同一个会话，重入不覆盖已有内容。
func TestCreationSessionConcurrentInitialization(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	const count = 8
	results := make(chan creationdomain.Session, count)
	errorsCh := make(chan error, count)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for i := range count {
		workers.Add(1)
		go func() {
			defer workers.Done()
			session, err := creationdomain.NewSession(fmt.Sprintf("session-%d", i), project.ID, "main", time.Unix(500, 0))
			if err != nil {
				errorsCh <- err
				return
			}
			<-start
			session, err = store.CreateCreationSession(ctx, session)
			if err != nil {
				errorsCh <- err
				return
			}
			results <- session
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errorsCh)
	for err := range errorsCh {
		t.Fatal(err)
	}
	latest, err := store.CreationSession(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	for session := range results {
		if session.ID != latest.ID {
			t.Fatalf("创建结果身份不一致: %s / %s", session.ID, latest.ID)
		}
	}
	latest.IntentText = "已存在的作者内容"
	if err := store.SaveCreationSession(ctx, latest); err != nil {
		t.Fatal(err)
	}
	retry, err := creationdomain.NewSession("session-retry", project.ID, "main", time.Unix(600, 0))
	if err != nil {
		t.Fatal(err)
	}
	retry, err = store.CreateCreationSession(ctx, retry)
	if err != nil || retry.ID != latest.ID || retry.IntentText != latest.IntentText {
		t.Fatalf("重复创建覆盖会话: %+v, %v", retry, err)
	}
	latest.ID = "different-session"
	if err := store.SaveCreationSession(ctx, latest); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("错误身份未被拒绝: %v", err)
	}
}

func TestCreationStoreRoundTripsSessionCandidateAndAsset(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := time.Unix(500, 0).UTC()

	session, err := creationdomain.NewSession("session-1", project.ID, "branch-1", now)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.SaveIntent("江心岛灯塔", now)
	if err != nil {
		t.Fatal(err)
	}
	session, err = session.AttachRun("operation-active", "run-active", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateCreationSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	loadedSession, err := store.CreationSession(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedSession.IntentText != "江心岛灯塔" || loadedSession.Stage != creationdomain.StageIntent || loadedSession.ActiveRunID != "run-active" {
		t.Fatalf("session = %#v", loadedSession)
	}

	candidate, err := creationdomain.NewCandidate(
		"cand-1", session.ID, creationdomain.StageDirections, creationdomain.KindDirectionSet,
		1, "", "run-1", "art-1", `{"directions":[{"id":"a"}]}`, "story_directions.schema", 1, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	loadedCandidate, err := store.CreationCandidate(ctx, candidate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedCandidate.ContentSHA256 != candidate.ContentSHA256 || loadedCandidate.RunID != "run-1" {
		t.Fatalf("candidate = %#v", loadedCandidate)
	}
	stale, err := loadedCandidate.MarkStale("故事方向已改", []string{"dir-1"}, creationdomain.ImpactMaybe, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCreationCandidate(ctx, stale); err != nil {
		t.Fatal(err)
	}
	staleLoaded, err := store.CreationCandidate(ctx, candidate.ID)
	if err != nil || staleLoaded.Status != creationdomain.StatusStale || staleLoaded.StaleReason != "故事方向已改" ||
		len(staleLoaded.ImpactRefs) != 1 || staleLoaded.ImpactRefs[0] != "dir-1" {
		t.Fatalf("stale candidate = %#v, error %v", staleLoaded, err)
	}
	listed, err := store.ListCreationCandidates(ctx, session.ID)
	if err != nil || len(listed) != 1 || listed[0].ID != candidate.ID {
		t.Fatalf("list candidates = %#v, error %v", listed, err)
	}
	superseded, err := candidate.MarkSuperseded(now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCreationCandidate(ctx, superseded); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.CreationCandidate(ctx, candidate.ID)
	if err != nil || reloaded.Status != creationdomain.StatusSuperseded {
		t.Fatalf("updated candidate = %#v, error %v", reloaded, err)
	}

	asset, err := storyassetdomain.New(
		"asset-1", project.ID, storyassetdomain.KindProposal, storyassetdomain.OriginModel,
		candidate.ID, "run-1", "art-1", "", "", now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveStoryAsset(ctx, asset); err != nil {
		t.Fatal(err)
	}
	loadedAsset, err := store.StoryAsset(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedAsset.Kind != storyassetdomain.KindProposal || loadedAsset.TypedTable != "" {
		t.Fatalf("asset = %#v", loadedAsset)
	}
}

func TestAssistantActionPersistsPendingThenExecuting(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := time.Unix(600, 0).UTC()
	if err := store.SaveAssistantSession(ctx, "asst-1", project.ID, "branch-1", formatTime(now)); err != nil {
		t.Fatal(err)
	}
	action, err := assistantdomain.NewAction(
		"act-1", "asst-1", project.ID, "branch-1",
		assistantdomain.OpReviseFields, assistantdomain.RiskPaid,
		[]string{"candidate:cand-1"}, []string{"cand-1@1"},
		`{"paths":["/title"]}`, "hash-1", "idem-1", "0.02", now, now.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAssistantAction(ctx, action); err != nil {
		t.Fatal(err)
	}
	executing, err := action.Confirm("hash-1", "idem-1", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveAssistantAction(ctx, executing); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.AssistantAction(ctx, action.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State != assistantdomain.StateExecuting || loaded.IdempotencyKey != "idem-1" {
		t.Fatalf("action = %#v", loaded)
	}
}
