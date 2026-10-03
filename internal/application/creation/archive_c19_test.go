package creation

import (
	"context"
	"errors"
	"strings"
	"testing"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

// 保存正文可发生在费用预览之后或入档事务开始前；历史版本也不能被空白现稿掩盖。
func TestArchiveRequiresHistoryImpactBoundary(t *testing.T) {
	for _, scenario := range []string{"预览前已有正文", "当前空稿但历史非空", "预览后保存正文", "事务前保存正文"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			raw, project := sqliteCreateStore(t)
			defer raw.Close()
			bridge := NewSQLiteStore(raw)
			seedArchiveSessionWithRoadmapSchema(t, bridge, project.ID, peopleWorldPayload, c11RoadmapPayload, 3)
			chapter, err := projectdomain.NewChapter("saved-chapter", project.ID, "已写第一章", 1, testClock{}.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err = raw.CreateChapter(ctx, chapter); err != nil {
				t.Fatal(err)
			}
			save := func() {
				t.Helper()
				next, err := chapter.Commit("潮汐决定航路，阿绫据此开船。", chapter.Revision, testClock{}.Now())
				if err != nil {
					t.Fatal(err)
				}
				if err = raw.CommitChapter(ctx, next, chapter); err != nil {
					t.Fatal(err)
				}
				chapter = next
			}
			var store Store = bridge
			if scenario == "事务前保存正文" {
				store = c19ArchiveStore{Store: bridge, before: save}
			}
			kernel := newFakeKernel()
			service, err := New(Config{Store: store, Kernel: kernel, Clock: testClock{}, IDs: &testIDs{}})
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "预览前已有正文" || scenario == "当前空稿但历史非空" {
				save()
				if scenario == "当前空稿但历史非空" {
					empty, err := chapter.Commit("", chapter.Revision, testClock{}.Now())
					if err != nil {
						t.Fatal(err)
					}
					if err = raw.CommitChapter(ctx, empty, chapter); err != nil {
						t.Fatal(err)
					}
					chapter = empty
				}
				session, err := bridge.Session(ctx, project.ID)
				if err != nil {
					t.Fatal(err)
				}
				listed, err := bridge.ListCandidates(ctx, session.ID)
				if err != nil {
					t.Fatal(err)
				}
				road, ok := currentCandidate(listed, creationdomain.StageRoadmap)
				if !ok {
					t.Fatal("缺少路线图")
				}
				changed, err := service.SaveCandidateDraft(ctx, project.ID, road.ID, strings.Replace(c11RoadmapPayload, "潮汐决定航路", "潮汐禁止航行", 1))
				if err != nil || !strings.Contains(changed.PayloadJSON, "潮汐禁止航行") {
					t.Fatalf("变更候选未保留: %v", err)
				}
				if _, err = service.PreviewConfirmArchive(ctx, project.ID); !errors.Is(err, creationdomain.ErrHistoryImpactRequired) {
					t.Fatalf("预览未阻断: %v", err)
				}
			} else {
				preview, err := service.PreviewConfirmArchive(ctx, project.ID)
				if err != nil {
					t.Fatal(err)
				}
				if scenario == "预览后保存正文" {
					save()
				}
				if _, err = service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "c19"}); !errors.Is(err, creationdomain.ErrHistoryImpactRequired) {
					t.Fatalf("确认未阻断: %v", err)
				}
			}
			current, err := raw.Chapter(ctx, chapter.ID)
			if err != nil || current.Body != chapter.Body || current.Revision != chapter.Revision {
				t.Fatalf("正文被改写: %#v %v", current, err)
			}
			history, err := raw.ListChapterRevisions(ctx, chapter.ID)
			if err != nil {
				t.Fatal(err)
			}
			retained := false
			for _, item := range history {
				if item.Body == "潮汐决定航路，阿绫据此开船。" {
					retained = true
				}
			}
			if !retained {
				t.Fatal("已保存正文历史丢失")
			}
			chars, err := raw.ListCharacters(ctx)
			if err != nil || len(chars) != 0 {
				t.Fatalf("阻断仍写入正式人物: %#v %v", chars, err)
			}
			latest, err := bridge.Session(ctx, project.ID)
			if err != nil || latest.ArchiveChangeSetID != "" {
				t.Fatalf("阻断仍记录成功: %#v %v", latest, err)
			}
			candidates, err := bridge.ListCandidates(ctx, latest.ID)
			if err != nil || len(candidates) < 4 {
				t.Fatalf("候选丢失: %d %v", len(candidates), err)
			}
			unchanged, err := raw.Project(ctx)
			if err != nil || unchanged.Title != project.Title {
				t.Fatalf("阻断改写项目: %#v %v", unchanged, err)
			}
			if kernel.executeN != 0 {
				t.Fatal("阻断产生模型调用")
			}
		})
	}
}

type c19ArchiveStore struct {
	Store
	before func()
}

func (store c19ArchiveStore) ApplyArchive(ctx context.Context, apply ArchiveApply) error {
	store.before()
	return store.Store.ApplyArchive(ctx, apply)
}

// 初始空章及恢复草稿不表示已确认的历史约束，首次策划入档应保留这些输入并继续。
func TestArchiveAllowsEmptyChapterWithRecoveryDraft(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	chapter, err := projectdomain.NewChapter("draft-only", project.ID, "起草中的第一章", 1, testClock{}.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = raw.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	if err = raw.SaveDraft(ctx, chapter.ID, "作者尚未提交版本的想法", chapter.Revision); err != nil {
		t.Fatal(err)
	}
	kernel := newFakeKernel()
	service, err := New(Config{Store: bridge, Kernel: kernel, Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ConfirmArchive(ctx, ArchiveRequest{ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "empty-chapter"}); err != nil {
		t.Fatal(err)
	}
	body, revision, _, found, err := raw.Draft(ctx, chapter.ID)
	if err != nil || !found || body != "作者尚未提交版本的想法" || revision != chapter.Revision {
		t.Fatalf("首次入档丢失恢复草稿: %q %d %v %v", body, revision, found, err)
	}
	current, err := raw.Chapter(ctx, chapter.ID)
	if err != nil || current.Body != "" || current.Revision != chapter.Revision {
		t.Fatalf("首次入档改写初始章: %#v %v", current, err)
	}
	if kernel.executeN != 0 {
		t.Fatal("确认入档意外调用模型")
	}
}
