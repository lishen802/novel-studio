package importing

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

var ErrEmptyManuscript = errors.New("manuscript contains no importable text")

var (
	chineseChapterPattern = regexp.MustCompile(`^第[[:space:]]*[0-9０-９零〇一二三四五六七八九十百千万两]+[[:space:]]*(章|回)(?:[[:space:]]+.*|[[:space:]]*[:：\-—][[:space:]]*.*)?$`)
	englishChapterPattern = regexp.MustCompile(`(?i)^chapter[[:space:]]+[0-9]+(?:[[:space:]]+.*|[[:space:]]*[:：\-—][[:space:]]*.*)?$`)
	volumePattern         = regexp.MustCompile(`^第[[:space:]]*[0-9０-９零〇一二三四五六七八九十百千万两]+[[:space:]]*卷(?:[[:space:]]+.*|[[:space:]]*[:：\-—][[:space:]]*.*)?$`)
	markdownPattern       = regexp.MustCompile(`^[[:space:]]*#{1,3}[[:space:]]+(.+?)[[:space:]]*#*[[:space:]]*$`)
)

type ParsedChapter struct {
	VolumeTitle     string
	Title           string
	Body            string
	Position        int
	SourceStartLine int
	SourceEndLine   int
	ContentHash     string
}

type chapterBoundary struct {
	volumeTitle string
	title       string
	headingLine int
	bodyStart   int
}

func ParseChapters(text string) ([]ParsedChapter, error) {
	normalized := normalizeText(text)
	if strings.TrimSpace(normalized) == "" {
		return nil, ErrEmptyManuscript
	}

	lines := strings.Split(normalized, "\n")
	chapters := make([]ParsedChapter, 0)
	currentVolume := ""
	var current *chapterBoundary
	sawChapterHeading := false
	lastVolumeLine := -1

	appendCurrent := func(bodyEnd int) {
		if current == nil {
			return
		}
		body, startLine, endLine, ok := trimmedBody(lines, current.bodyStart, bodyEnd)
		if !ok {
			body = ""
			startLine = current.headingLine + 1
			endLine = startLine
		}
		chapters = append(chapters, ParsedChapter{
			VolumeTitle:     current.volumeTitle,
			Title:           current.title,
			Body:            body,
			Position:        len(chapters) + 1,
			SourceStartLine: startLine,
			SourceEndLine:   endLine,
			ContentHash:     hashContent(body),
		})
		current = nil
	}

	for lineIndex, rawLine := range lines {
		heading := unwrapMarkdownHeading(rawLine)
		if heading == "" {
			continue
		}
		if isVolumeHeading(heading) {
			appendCurrent(lineIndex - 1)
			currentVolume = heading
			lastVolumeLine = lineIndex
			continue
		}
		if !isChapterHeading(heading) {
			continue
		}

		sawChapterHeading = true
		appendCurrent(lineIndex - 1)
		current = &chapterBoundary{
			volumeTitle: currentVolume,
			title:       heading,
			headingLine: lineIndex,
			bodyStart:   lineIndex + 1,
		}
	}
	appendCurrent(len(lines) - 1)

	if len(chapters) > 0 {
		return chapters, nil
	}
	if sawChapterHeading {
		return nil, ErrEmptyManuscript
	}

	fallbackStart := 0
	volumeTitle := ""
	if lastVolumeLine >= 0 {
		fallbackStart = lastVolumeLine + 1
		volumeTitle = currentVolume
	}
	body, startLine, endLine, ok := trimmedBody(lines, fallbackStart, len(lines)-1)
	if !ok {
		return nil, ErrEmptyManuscript
	}
	return []ParsedChapter{{
		VolumeTitle:     volumeTitle,
		Title:           "导入正文",
		Body:            body,
		Position:        1,
		SourceStartLine: startLine,
		SourceEndLine:   endLine,
		ContentHash:     hashContent(body),
	}}, nil
}

func normalizeText(text string) string {
	text = strings.TrimPrefix(text, "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(text, "\r", "\n")
}

func unwrapMarkdownHeading(line string) string {
	line = strings.TrimSpace(line)
	if matches := markdownPattern.FindStringSubmatch(line); matches != nil {
		return strings.TrimSpace(matches[1])
	}
	return line
}

func isChapterHeading(line string) bool {
	return chineseChapterPattern.MatchString(line) || englishChapterPattern.MatchString(line)
}

func isVolumeHeading(line string) bool {
	return volumePattern.MatchString(line)
}

func trimmedBody(lines []string, start, end int) (string, int, int, bool) {
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	for start <= end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end >= start && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if start > end {
		return "", 0, 0, false
	}

	body := strings.TrimSpace(strings.Join(lines[start:end+1], "\n"))
	if body == "" {
		return "", 0, 0, false
	}
	return body, start + 1, end + 1, true
}

func hashContent(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}
