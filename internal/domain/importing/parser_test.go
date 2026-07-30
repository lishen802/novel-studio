package importing

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"strings"
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

func TestParseChaptersPreservesPrefaceBeforeFirstChapter(t *testing.T) {
	chapters, err := ParseChapters("序言一\n\n序言二\n第一章 开始\n正文")
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("len(chapters) = %d, want 2: %#v", len(chapters), chapters)
	}

	preface := chapters[0]
	if preface.Title != "导入正文" || preface.VolumeTitle != "" || preface.Body != "序言一\n\n序言二" {
		t.Fatalf("preface = %#v, want independent imported-body candidate", preface)
	}
	if preface.Position != 1 || preface.SourceStartLine != 1 || preface.SourceEndLine != 3 {
		t.Fatalf("preface source metadata = %#v, want lines 1-3 at position 1", preface)
	}

	chapter := chapters[1]
	if chapter.Title != "第一章 开始" || chapter.Body != "正文" || chapter.Position != 2 {
		t.Fatalf("chapter = %#v, want recognized chapter after preface", chapter)
	}
}

func TestParseChaptersPreservesBodyForEachVolumeWithoutChapters(t *testing.T) {
	text := "第一卷 起点\n卷一正文\n第二卷 转折\n卷二正文\n第三卷 尾声"
	chapters, err := ParseChapters(text)
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("len(chapters) = %d, want 2: %#v", len(chapters), chapters)
	}

	want := []ParsedChapter{
		{
			VolumeTitle:     "第一卷 起点",
			Title:           "导入正文",
			Body:            "卷一正文",
			Position:        1,
			SourceStartLine: 2,
			SourceEndLine:   2,
		},
		{
			VolumeTitle:     "第二卷 转折",
			Title:           "导入正文",
			Body:            "卷二正文",
			Position:        2,
			SourceStartLine: 4,
			SourceEndLine:   4,
		},
	}
	for i := range want {
		want[i].ContentHash = contentHash(want[i].Body)
		if !reflect.DeepEqual(chapters[i], want[i]) {
			t.Errorf("chapter %d = %#v, want %#v", i, chapters[i], want[i])
		}
	}
}

func TestParseChaptersRejectsVolumeHeadingWithoutBody(t *testing.T) {
	_, err := ParseChapters("第一卷 起点")
	if !errors.Is(err, ErrEmptyManuscript) {
		t.Fatalf("ParseChapters error = %v, want %v", err, ErrEmptyManuscript)
	}
}

func TestParseChaptersDropsTrailingEmptyVolumeWithoutLosingEarlierBody(t *testing.T) {
	chapters, err := ParseChapters("卷前正文\n第二卷 尾声")
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("len(chapters) = %d, want 1: %#v", len(chapters), chapters)
	}
	if chapters[0].Title != "导入正文" || chapters[0].Body != "卷前正文" {
		t.Fatalf("chapter = %#v, want preserved body without empty volume candidate", chapters[0])
	}
}

func TestParseChaptersRecognizesUnicodeWhitespaceInChineseHeadings(t *testing.T) {
	tests := []string{
		"第\u3000十二章\u3000开始",
		"第\u00a0十二章\u00a0开始",
	}
	for _, title := range tests {
		t.Run(title, func(t *testing.T) {
			chapters, err := ParseChapters(title + "\n正文")
			if err != nil {
				t.Fatalf("ParseChapters returned error: %v", err)
			}
			if len(chapters) != 1 {
				t.Fatalf("len(chapters) = %d, want 1: %#v", len(chapters), chapters)
			}
			if chapters[0].Title != title || chapters[0].Body != "正文" {
				t.Fatalf("chapter = %#v, want title display preserved", chapters[0])
			}
		})
	}
}

func TestParseChaptersAllowsTwentyMiBSingleLine(t *testing.T) {
	const approvedImportBytes = 20 * 1024 * 1024
	body := strings.Repeat("a", approvedImportBytes)
	chapters, err := ParseChapters(body)
	if err != nil {
		t.Fatalf("ParseChapters returned error for approved-size line: %v", err)
	}
	if len(chapters) != 1 || len(chapters[0].Body) != approvedImportBytes {
		t.Fatalf("approved-size line was not preserved")
	}
}

func TestParseChaptersRejectsLineBeyondStableLimit(t *testing.T) {
	const expectedError = "manuscript line exceeds maximum length"
	text := strings.Repeat("a", MaxDecodedManuscriptBytes+1)

	_, err := ParseChapters(text)
	if !errors.Is(err, ErrManuscriptLineTooLong) || err.Error() != expectedError {
		t.Fatalf("ParseChapters error = %v, want stable error %q", err, expectedError)
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

func TestParseChaptersKeepsAdjacentHeadingsAsSeparateChapters(t *testing.T) {
	chapters, err := ParseChapters("第一章 重逢\n第一章 重逢\n正文")
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("len(chapters) = %d, want 2: %#v", len(chapters), chapters)
	}

	first := chapters[0]
	if first.Title != "第一章 重逢" || first.Body != "" {
		t.Fatalf("first chapter = %#v, want empty chapter with recognized title", first)
	}
	if first.Position != 1 || first.SourceStartLine != 1 || first.SourceEndLine != 1 {
		t.Fatalf("first chapter source metadata = %#v, want title-line anchor 1-1", first)
	}
	if first.ContentHash != contentHash("") {
		t.Fatalf("first ContentHash = %q, want SHA-256 of empty body", first.ContentHash)
	}

	second := chapters[1]
	if second.Title != "第一章 重逢" || second.Body != "正文" {
		t.Fatalf("second chapter = %#v, want chapter with body", second)
	}
	if second.Position != 2 || second.SourceStartLine != 3 || second.SourceEndLine != 3 {
		t.Fatalf("second chapter source metadata = %#v, want body lines 3-3", second)
	}
}

func TestParseChaptersKeepsRecognizedHeadingWithoutBody(t *testing.T) {
	chapters, err := ParseChapters("第一章 重逢")
	if err != nil {
		t.Fatalf("ParseChapters returned error: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("len(chapters) = %d, want 1: %#v", len(chapters), chapters)
	}

	chapter := chapters[0]
	if chapter.Title != "第一章 重逢" || chapter.Body != "" || chapter.Position != 1 {
		t.Fatalf("chapter = %#v, want one empty recognized chapter", chapter)
	}
	if chapter.SourceStartLine != 1 || chapter.SourceEndLine != 1 {
		t.Fatalf("chapter source lines = %d-%d, want title-line anchor 1-1", chapter.SourceStartLine, chapter.SourceEndLine)
	}
	if chapter.ContentHash != contentHash("") {
		t.Fatalf("ContentHash = %q, want SHA-256 of empty body", chapter.ContentHash)
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
