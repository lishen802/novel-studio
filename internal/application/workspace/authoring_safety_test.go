package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

func TestChapterAgentRequiresSavedVersionAtCreationAndExecution(t *testing.T) {
	ctx := context.Background()
	service, chapter := configuredAgentServiceWithProfile(t, failingModelProviderFactory{}, defaultResolvedProfile())
	defer service.Close()
	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "继续")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SaveDraft(ctx, chapter.ID, "屏幕上的新稿", chapter.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateChapterAgentRun(ctx, chapter.ID, "继续"); !errors.Is(err, projectdomain.ErrUnsavedDraft) {
		t.Fatalf("create error = %v", err)
	}
	if _, err := service.ExecuteAgentRun(ctx, run.ID); !errors.Is(err, projectdomain.ErrUnsavedDraft) {
		t.Fatalf("execute error = %v", err)
	}
	if _, err := service.CommitChapter(ctx, chapter.ID, "屏幕上的新稿", chapter.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteAgentRun(ctx, run.ID); !errors.Is(err, projectdomain.ErrRevisionConflict) {
		t.Fatalf("stale run = %v", err)
	}
}

func TestAuthoringContextPrefersNearestPrecedingChapters(t *testing.T) {
	ctx := context.Background()
	service, first := configuredAgentServiceWithProfile(t, failingModelProviderFactory{}, defaultResolvedProfile())
	defer service.Close()
	if _, err := service.CommitChapter(ctx, first.ID, strings.Repeat("旧", 500), first.Revision); err != nil {
		t.Fatal(err)
	}
	second, err := service.CreateChapter(ctx, "紧邻前章")
	if err != nil {
		t.Fatal(err)
	}
	target, err := service.CreateChapter(ctx, "目标章")
	if err != nil {
		t.Fatal(err)
	}
	future, err := service.CreateChapter(ctx, "后续章")
	if err != nil {
		t.Fatal(err)
	}
	manifest, _, err := service.assembleAuthoringContext(ctx, service.active.store, target.ID, 1000000)
	if err != nil {
		t.Fatal(err)
	}
	var doc contextManifestDocument
	if err := json.Unmarshal([]byte(manifest), &doc); err != nil {
		t.Fatal(err)
	}
	items := doc.Domains["chapters"]
	if len(items) != 2 || !strings.Contains(string(items[0]), second.ID) || !strings.Contains(string(items[1]), first.ID) {
		t.Fatalf("recent order: %s", items)
	}
	if strings.Contains(manifest, future.ID) {
		t.Fatal("future chapter leaked into context")
	}
	budget := len([]rune(manifest)) - 200
	trimmed, _, err := service.assembleAuthoringContext(ctx, service.active.store, target.ID, budget)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(trimmed, second.ID) {
		t.Fatal("nearest preceding chapter was trimmed before oldest")
	}
}

func TestManuscriptExportIncludesWorkingDraftWithoutChangingVersion(t *testing.T) {
	ctx := context.Background()
	service, chapter := configuredAgentServiceWithProfile(t, failingModelProviderFactory{}, defaultResolvedProfile())
	defer service.Close()
	saved, err := service.CommitChapter(ctx, chapter.ID, "旧正文", chapter.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SaveDraft(ctx, chapter.ID, "", saved.Revision); err != nil {
		t.Fatal(err)
	}
	next, err := service.CreateChapter(ctx, "第二章")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SaveDraft(ctx, next.ID, "最新稿", next.Revision); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"plain", "markdown"} {
		exported, err := service.ExportManuscript(ctx, format)
		if err != nil {
			t.Fatal(err)
		}
		body, err := os.ReadFile(exported.Path)
		if err != nil {
			t.Fatal(err)
		}
		if exported.ChapterCount != 2 || exported.DraftCount != 2 || strings.Contains(string(body), "旧正文") || !strings.Contains(string(body), "最新稿") {
			t.Fatalf("export = %#v %s", exported, body)
		}
	}
	current, err := service.OpenChapter(ctx, chapter.ID)
	if err != nil || current.Revision != saved.Revision || !current.HasRecoveryDraft {
		t.Fatalf("export mutated source: %#v %v", current, err)
	}
	if _, err := service.ExportManuscript(ctx, "epub"); err == nil {
		t.Fatal("unsupported format accepted")
	}
}
