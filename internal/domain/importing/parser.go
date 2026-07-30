package importing

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"unicode"
)

const MaxManuscriptBytes = 20 * 1024 * 1024

var (
	ErrEmptyManuscript       = errors.New("manuscript contains no importable text")
	ErrManuscriptLineTooLong = errors.New("manuscript line exceeds maximum length")
)

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

type chapterCandidate struct {
	volumeTitle     string
	title           string
	anchorLine      int
	explicitChapter bool
	body            chapterBody
}

type chapterBody struct {
	text              strings.Builder
	startLine         int
	endLine           int
	pendingBlankLines int
}

func (body *chapterBody) add(line string, lineNumber int) {
	if strings.TrimSpace(line) == "" {
		if body.startLine != 0 {
			body.pendingBlankLines++
		}
		return
	}

	if body.startLine == 0 {
		body.startLine = lineNumber
	} else {
		body.text.WriteByte('\n')
		for range body.pendingBlankLines {
			body.text.WriteByte('\n')
		}
	}
	body.text.WriteString(line)
	body.endLine = lineNumber
	body.pendingBlankLines = 0
}

func (body *chapterBody) value() (string, int, int, bool) {
	if body.startLine == 0 {
		return "", 0, 0, false
	}
	return strings.TrimSpace(body.text.String()), body.startLine, body.endLine, true
}

func ParseChapters(text string) ([]ParsedChapter, error) {
	chapters := make([]ParsedChapter, 0)
	currentVolume := ""
	var current *chapterCandidate

	appendCurrent := func() {
		if current == nil {
			return
		}
		body, startLine, endLine, ok := current.body.value()
		if !ok {
			if !current.explicitChapter {
				current = nil
				return
			}
			startLine = current.anchorLine
			endLine = current.anchorLine
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

	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 64*1024), MaxManuscriptBytes+64*1024)
	scanner.Split(splitUniversalLines)

	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		rawLine := scanner.Text()
		if lineNumber == 1 {
			rawLine = strings.TrimPrefix(rawLine, "\ufeff")
		}
		if len(rawLine) > MaxManuscriptBytes {
			return nil, ErrManuscriptLineTooLong
		}

		heading := unwrapMarkdownHeading(rawLine)
		if heading != "" && isVolumeHeading(heading) {
			appendCurrent()
			currentVolume = heading
			current = &chapterCandidate{
				volumeTitle: currentVolume,
				title:       "导入正文",
				anchorLine:  lineNumber,
			}
			continue
		}
		if heading != "" && isChapterHeading(heading) {
			appendCurrent()
			current = &chapterCandidate{
				volumeTitle:     currentVolume,
				title:           heading,
				anchorLine:      lineNumber,
				explicitChapter: true,
			}
			continue
		}

		if current == nil {
			if strings.TrimSpace(rawLine) == "" {
				continue
			}
			current = &chapterCandidate{
				volumeTitle: currentVolume,
				title:       "导入正文",
				anchorLine:  lineNumber,
			}
		}
		current.body.add(rawLine, lineNumber)
	}
	if scanner.Err() != nil {
		return nil, ErrManuscriptLineTooLong
	}
	appendCurrent()

	if len(chapters) == 0 {
		return nil, ErrEmptyManuscript
	}
	return chapters, nil
}

func splitUniversalLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	for index, value := range data {
		switch value {
		case '\n':
			return index + 1, data[:index], nil
		case '\r':
			if index+1 == len(data) && !atEOF {
				return 0, nil, nil
			}
			advance = index + 1
			if index+1 < len(data) && data[index+1] == '\n' {
				advance++
			}
			return advance, data[:index], nil
		}
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func unwrapMarkdownHeading(line string) string {
	line = strings.TrimSpace(line)
	if matches := markdownPattern.FindStringSubmatch(line); matches != nil {
		return strings.TrimSpace(matches[1])
	}
	return line
}

func isChapterHeading(line string) bool {
	normalized := normalizeHeadingWhitespace(line)
	return chineseChapterPattern.MatchString(normalized) || englishChapterPattern.MatchString(normalized)
}

func isVolumeHeading(line string) bool {
	return volumePattern.MatchString(normalizeHeadingWhitespace(line))
}

func normalizeHeadingWhitespace(line string) string {
	return strings.Map(func(value rune) rune {
		if unicode.IsSpace(value) {
			return ' '
		}
		return value
	}, line)
}

func hashContent(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}
