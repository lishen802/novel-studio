package importing

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"
)

func TestParseChaptersRecognizesSupportedHeadingsAndVolumes(t *testing.T) {
	text := "\ufeff第一卷 起点\n" +
		"第十二章 风起\n" +
		"甲。\n" +
		"乙。\n" +
		"\n" +
		"第 12 回：相逢\n" +
		"丙。\n" +
		"第二卷 转折\n" +
		"## Chapter 13 - Storm\n" +
		"丁。\n" +
		"### 第14章 终局 ###\n" +
		"戊。"

	chapters, err := ParseChapters(text)
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}

	want := []ParsedChapter{
		{
			VolumeTitle:     "第一卷 起点",
			Title:           "第十二章 风起",
			Body:            "甲。\n乙。",
			Position:        1,
			SourceStartLine: 3,
			SourceEndLine:   4,
		},
		{
			VolumeTitle:     "第一卷 起点",
			Title:           "第 12 回：相逢",
			Body:            "丙。",
			Position:        2,
			SourceStartLine: 7,
			SourceEndLine:   7,
		},
		{
			VolumeTitle:     "第二卷 转折",
			Title:           "Chapter 13 - Storm",
			Body:            "丁。",
			Position:        3,
			SourceStartLine: 10,
			SourceEndLine:   10,
		},
		{
			VolumeTitle:     "第二卷 转折",
			Title:           "第14章 终局",
			Body:            "戊。",
			Position:        4,
			SourceStartLine: 12,
			SourceEndLine:   12,
		},
	}
	if len(chapters) != len(want) {
		t.Fatalf("len(chapters) = %d, want %d: %#v", len(chapters), len(want), chapters)
	}
	for i := range want {
		want[i].ContentHash = contentHash(want[i].Body)
		if !reflect.DeepEqual(chapters[i], want[i]) {
			t.Errorf("chapter %d = %#v, want %#v", i, chapters[i], want[i])
		}
	}
}

func TestParseChaptersNormalizesNewlinesAndPreservesInternalBlankLines(t *testing.T) {
	chapters, err := ParseChapters("# 第一章 开始\r\n第一段\r\n\r第二段\r\n")
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("len(chapters) = %d, want 1", len(chapters))
	}
	chapter := chapters[0]
	if chapter.Body != "第一段\n\n第二段" {
		t.Fatalf("Body = %q, want normalized body with internal blank line", chapter.Body)
	}
	if chapter.SourceStartLine != 2 || chapter.SourceEndLine != 4 {
		t.Fatalf("source lines = %d-%d, want 2-4", chapter.SourceStartLine, chapter.SourceEndLine)
	}
}

func TestParseChaptersDoesNotCreateChapterForVolumeHeading(t *testing.T) {
	chapters, err := ParseChapters("第一卷 起点\n\n第1章 开始\n正文")
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("len(chapters) = %d, want 1: %#v", len(chapters), chapters)
	}
	if chapters[0].VolumeTitle != "第一卷 起点" || chapters[0].Title != "第1章 开始" {
		t.Fatalf("unexpected parsed chapter: %#v", chapters[0])
	}
}

func TestParseChaptersKeepsDuplicateHeadingsSeparate(t *testing.T) {
	chapters, err := ParseChapters("第一章 重逢\n甲\n第一章 重逢\n乙")
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("len(chapters) = %d, want 2: %#v", len(chapters), chapters)
	}
	if chapters[0].Title != chapters[1].Title || chapters[0].Body != "甲" || chapters[1].Body != "乙" {
		t.Fatalf("duplicate headings were not kept as separate chapters: %#v", chapters)
	}
	if chapters[0].Position != 1 || chapters[1].Position != 2 {
		t.Fatalf("positions = %d, %d; want 1, 2", chapters[0].Position, chapters[1].Position)
	}
}

func TestParseChaptersFallsBackForUntitledText(t *testing.T) {
	chapters, err := ParseChapters("\n  序言\n\n正文  \n")
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("len(chapters) = %d, want 1", len(chapters))
	}
	chapter := chapters[0]
	if chapter.Title != "导入正文" || chapter.Body != "序言\n\n正文" {
		t.Fatalf("fallback chapter = %#v", chapter)
	}
	if chapter.Position != 1 || chapter.SourceStartLine != 2 || chapter.SourceEndLine != 4 {
		t.Fatalf("fallback source metadata = %#v", chapter)
	}
}

func TestParseChaptersRejectsEmptyTextWithStableDomainError(t *testing.T) {
	for _, text := range []string{"", " \t\r\n\r\n", "\ufeff\n\t"} {
		_, err := ParseChapters(text)
		if !errors.Is(err, ErrEmptyManuscript) {
			t.Fatalf("ParseChapters(%q) error = %v, want %v", text, err, ErrEmptyManuscript)
		}
	}
}

func TestParseChaptersIsDeterministic(t *testing.T) {
	text := "第1章 同行\n第一段\n\n第二段\n第2章 再会\n结尾"

	first, err := ParseChapters(text)
	if err != nil {
		t.Fatalf("first ParseChapters returned error: %v", err)
	}
	second, err := ParseChapters(text)
	if err != nil {
		t.Fatalf("second ParseChapters returned error: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same input produced different output:\nfirst:  %#v\nsecond: %#v", first, second)
	}
	for _, chapter := range first {
		if chapter.ContentHash != contentHash(chapter.Body) {
			t.Errorf("ContentHash = %q, want SHA-256 of %q", chapter.ContentHash, chapter.Body)
		}
	}
}

func contentHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}
