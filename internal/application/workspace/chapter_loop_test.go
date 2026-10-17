package workspace

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	chapterapp "github.com/lishen802/novel-studio/internal/application/chapterloop"
	"github.com/lishen802/novel-studio/internal/application/license"
	loopdomain "github.com/lishen802/novel-studio/internal/domain/chapterloop"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/branchfs"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
)

func TestChapterProductionFinalizePosthocAndSelection(t *testing.T) {
	ctx := context.Background()
	service := NewService(projectfs.Manager{}, sqlite.Factory{}, license.NoopService{}, fixedClock{now: time.Unix(300, 0)}, ids.UUIDGenerator{}, snapshotfs.Archive{}, branchfs.Archive{})
	t.Cleanup(func() { service.Close() })
	completer := &workspaceCompleter{}
	service.ConfigureChapterCompleter(completer)
	if _, err := service.CreateProject(ctx, t.TempDir(), "潮汐", "奇幻", ""); err != nil {
		t.Fatal(err)
	}
	chapter, err := service.CreateChapter(ctx, "第一章")
	if err != nil {
		t.Fatal(err)
	}
	node, err := service.CreateOutlineNode(ctx, CreateOutlineNodeInput{Level: "book", Title: "登岛", Summary: "发现灯塔", Goal: "上岸", Conflict: "大雾", Outcome: "进村"})
	if err != nil {
		t.Fatal(err)
	}
	production, err := service.StartChapterProduction(ctx, StartChapterProductionInput{RoadmapNodeID: node.ID, ChapterID: chapter.ID})
	if err != nil {
		t.Fatal(err)
	}
	if production.SelectedID == "" || !strings.Contains(production.SelectionReason, "绑定评估") {
		t.Fatalf("选稿: %+v", production)
	}
	finalized, err := service.FinalizeChapterProduction(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finalized.Body != "沈知秋看见灯塔未亮。" || finalized.Revision != chapter.Revision+1 {
		t.Fatalf("定稿: %+v", finalized)
	}
	posthoc, err := service.CommitChapterPosthoc(ctx, chapter.ID, "post-1")
	if err != nil || posthoc.Status != loopdomain.PosthocCommitted {
		t.Fatalf("后验: %+v %v", posthoc, err)
	}
	service.ConfigureChapterCompleter(&workspaceCompleter{err: errors.New("model down")})
	failed, err := service.CommitChapterPosthoc(ctx, chapter.ID, "post-fail")
	if err == nil || !failed.MemoryPendingRebuild {
		t.Fatalf("后验失败: %+v %v", failed, err)
	}
	service.ConfigureChapterCompleter(completer)
	current, err := service.OpenChapter(ctx, chapter.ID)
	if err != nil || current.Body != finalized.Body || current.Revision != finalized.Revision {
		t.Fatalf("失败不能回滚正文: %+v %v", current, err)
	}
	proposal, err := service.PrepareSelectionRewrite(ctx, SelectionRewriteInput{ChapterID: chapter.ID, Revision: current.Revision, Body: current.Body, StartRune: 0, EndRune: 3, Mode: "rewrite"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptSelectionRewrite(ctx, AcceptSelectionInput{ProposalID: proposal.ID, ChapterID: chapter.ID, Revision: current.Revision, Body: "正文已经变了。", Partial: false}); !errors.Is(err, loopdomain.ErrSelectionStale) {
		t.Fatalf("错位覆盖: %v", err)
	}
	accepted, err := service.AcceptSelectionRewrite(ctx, AcceptSelectionInput{ProposalID: proposal.ID, ChapterID: chapter.ID, Revision: current.Revision, Body: current.Body, Partial: true, Replacement: "开篇"})
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Revision != current.Revision+1 || !strings.HasPrefix(accepted.Body, "开篇") {
		t.Fatalf("部分采纳: %+v", accepted)
	}
	explanation, err := service.ExplainSelection(ctx, SelectionRewriteInput{ChapterID: accepted.ID, Revision: accepted.Revision, Body: accepted.Body, StartRune: 0, EndRune: 2, Mode: "explain"})
	if err != nil || !strings.Contains(explanation, "灯塔") {
		t.Fatalf("解释: %s %v", explanation, err)
	}
	unchanged, err := service.OpenChapter(ctx, chapter.ID)
	if err != nil || unchanged.Revision != accepted.Revision {
		t.Fatalf("解释不应再写 revision: %+v %v", unchanged, err)
	}
}

type workspaceCompleter struct{ err error }

func (completer *workspaceCompleter) Complete(_ context.Context, call chapterapp.Call) (chapterapp.Completion, error) {
	if completer.err != nil {
		return chapterapp.Completion{}, completer.err
	}
	switch call.Stage {
	case "chapter_directing":
		return chapterapp.Completion{Text: `{"task":"写登岛"}`, InputTokens: 1, OutputTokens: 1}, nil
	case "prose_writing":
		return chapterapp.Completion{Text: "沈知秋看见灯塔未亮。", InputTokens: 1, OutputTokens: 1}, nil
	case "independent_review":
		return chapterapp.Completion{Text: `{"findings":[]}`, InputTokens: 1, OutputTokens: 1}, nil
	case "chapter_posthoc":
		return chapterapp.Completion{Text: `{"statements":[{"id":"fact","text":"灯塔昨夜没有亮","kind":"objective","disposition":"accepted","evidence":"叙述"},{"id":"speech","text":"船夫说岛上无人","kind":"speech","disposition":"pending","evidence":"对白"}]}`}, nil
	case "rewrite":
		return chapterapp.Completion{Text: "改写候选", InputTokens: 1, OutputTokens: 1}, nil
	case "explain":
		return chapterapp.Completion{Text: "灯塔未亮是悬念。", InputTokens: 1, OutputTokens: 1}, nil
	default:
		return chapterapp.Completion{}, errors.New(call.Stage)
	}
}
