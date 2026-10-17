package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	loopdomain "github.com/lishen802/novel-studio/internal/domain/chapterloop"
)

func TestChapterLoopRecordsAppendAndRejectDuplicateKeys(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	first := loopdomain.StoredRecord{ID: "loop-1", ProjectID: project.ID, Kind: "posthoc", SubjectID: "chapter-1", IdempotencyKey: "same", Payload: `{"status":"committed"}`}
	if err := store.AppendChapterLoop(ctx, first); err != nil {
		t.Fatal(err)
	}
	duplicate := first
	duplicate.ID = "loop-2"
	if err := store.AppendChapterLoop(ctx, duplicate); err == nil {
		t.Fatal("同一幂等键不能重复提交")
	}
	second := loopdomain.StoredRecord{ID: "loop-3", ProjectID: project.ID, Kind: "posthoc", SubjectID: "chapter-1", Payload: `{"status":"failed"}`}
	third := second
	third.ID = "loop-4"
	third.Payload = `{"status":"recovered"}`
	if err := store.AppendChapterLoop(ctx, second); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendChapterLoop(ctx, third); err != nil {
		t.Fatal(err)
	}
	latest, err := store.LatestChapterLoop(ctx, project.ID, "posthoc", "chapter-1")
	if err != nil || latest.ID != "loop-4" {
		t.Fatalf("最新记录: %+v %v", latest, err)
	}
	got, err := store.GetChapterLoop(ctx, project.ID, "loop-1")
	if err != nil || got.Payload != first.Payload {
		t.Fatalf("按标识读取: %+v %v", got, err)
	}
	if _, err := store.GetChapterLoop(ctx, project.ID, "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("缺失记录: %v", err)
	}
	records, err := store.ListChapterLoop(ctx, project.ID, "posthoc")
	if err != nil || len(records) != 3 || records[0].ID != "loop-1" || records[2].ID != "loop-4" {
		t.Fatalf("追加顺序: %+v %v", records, err)
	}
}
