package chapterloop

import (
	"errors"
	"strings"
)

var (
	ErrSpeechNotFact        = errors.New("speech or guess cannot become a world fact")
	ErrMemoryPendingRebuild = errors.New("memory pending rebuild")
	ErrPosthocSourceChanged = errors.New("posthoc source changed")
	ErrInvalidPosthoc       = errors.New("invalid chapter posthoc")
)

type StatementKind string

const (
	StatementObjective StatementKind = "objective"
	StatementSpeech    StatementKind = "speech"
	StatementGuess     StatementKind = "guess"
)

type Disposition string

const (
	DispositionAccepted Disposition = "accepted"
	DispositionPending  Disposition = "pending"
	DispositionConflict Disposition = "conflict"
)

type PosthocStatus string

const (
	PosthocDraft     PosthocStatus = "draft"
	PosthocCommitted PosthocStatus = "committed"
	PosthocFailed    PosthocStatus = "failed"
)

// Statement 是定稿后提取的一条带语境记录。
type Statement struct {
	ID          string        `json:"id"`
	Text        string        `json:"text"`
	Kind        StatementKind `json:"kind"`
	Disposition Disposition   `json:"disposition"`
	Evidence    string        `json:"evidence"`
}

// Posthoc 把后验记忆与正文定稿分开提交。
type Posthoc struct {
	ID                   string        `json:"id"`
	ChapterID            string        `json:"chapterId"`
	SourceRevision       int64         `json:"sourceRevision"`
	SourceHash           string        `json:"sourceHash"`
	Statements           []Statement   `json:"statements"`
	Status               PosthocStatus `json:"status"`
	MemoryPendingRebuild bool          `json:"memoryPendingRebuild"`
	IdempotencyKey       string        `json:"idempotencyKey"`
}

// NewPosthoc 校验发言和猜测没有被当成世界事实。
func NewPosthoc(id, chapterID string, revision int64, body string, statements []Statement, key string) (Posthoc, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(chapterID) == "" || revision < 1 || strings.TrimSpace(body) == "" || strings.TrimSpace(key) == "" || len(statements) == 0 {
		return Posthoc{}, ErrInvalidPosthoc
	}
	copied := make([]Statement, len(statements))
	seen := map[string]bool{}
	for index, statement := range statements {
		statement.ID = strings.TrimSpace(statement.ID)
		statement.Text = strings.TrimSpace(statement.Text)
		statement.Evidence = strings.TrimSpace(statement.Evidence)
		if statement.ID == "" || statement.Text == "" || statement.Evidence == "" || seen[statement.ID] {
			return Posthoc{}, ErrInvalidPosthoc
		}
		switch statement.Kind {
		case StatementObjective, StatementSpeech, StatementGuess:
		default:
			return Posthoc{}, ErrInvalidPosthoc
		}
		switch statement.Disposition {
		case DispositionAccepted, DispositionPending, DispositionConflict:
		default:
			return Posthoc{}, ErrInvalidPosthoc
		}
		if statement.Kind != StatementObjective && statement.Disposition == DispositionAccepted {
			return Posthoc{}, ErrSpeechNotFact
		}
		seen[statement.ID] = true
		copied[index] = statement
	}
	return Posthoc{
		ID: id, ChapterID: chapterID, SourceRevision: revision, SourceHash: hashText(body),
		Statements: copied, Status: PosthocDraft, IdempotencyKey: key,
	}, nil
}

// Commit 在校验通过后独立提交后验结果。
func (posthoc Posthoc) Commit() (Posthoc, error) {
	if posthoc.Status == PosthocFailed || posthoc.MemoryPendingRebuild {
		return posthoc, ErrMemoryPendingRebuild
	}
	for _, statement := range posthoc.Statements {
		if statement.Kind != StatementObjective && statement.Disposition == DispositionAccepted {
			return posthoc, ErrSpeechNotFact
		}
	}
	posthoc.Status = PosthocCommitted
	posthoc.MemoryPendingRebuild = false
	return posthoc, nil
}

// FailedPosthoc 记录模型失败，正文哈希仍绑定当时的定稿。
func FailedPosthoc(id, chapterID string, revision int64, body, key string) (Posthoc, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(chapterID) == "" || revision < 1 || strings.TrimSpace(body) == "" || strings.TrimSpace(key) == "" {
		return Posthoc{}, ErrInvalidPosthoc
	}
	return Posthoc{
		ID: id, ChapterID: chapterID, SourceRevision: revision, SourceHash: hashText(body), IdempotencyKey: key,
	}.Fail(), nil
}

// Fail 标记记忆待重建，不表示正文失效。
func (posthoc Posthoc) Fail() Posthoc {
	posthoc.Status = PosthocFailed
	posthoc.MemoryPendingRebuild = true
	return posthoc
}

// PreflightMemory 在待重建未恢复时阻止下一章生成。
func PreflightMemory(latest Posthoc, sourceHash string) error {
	if latest.MemoryPendingRebuild {
		return ErrMemoryPendingRebuild
	}
	if latest.Status == PosthocCommitted && latest.SourceHash != "" && sourceHash != "" && latest.SourceHash != sourceHash {
		return ErrPosthocSourceChanged
	}
	return nil
}

// Recover 只允许用同一来源重新提交；来源变化必须另做后验。
func (posthoc Posthoc) Recover(sourceHash string) (Posthoc, error) {
	if posthoc.SourceHash != sourceHash {
		return posthoc, ErrPosthocSourceChanged
	}
	posthoc.MemoryPendingRebuild = false
	posthoc.Status = PosthocCommitted
	return posthoc, nil
}

// ReadableFacts 只返回已提交后验中的已采纳客观陈述。
func ReadableFacts(records []Posthoc) []Statement {
	facts := make([]Statement, 0)
	for _, record := range records {
		if record.Status != PosthocCommitted || record.MemoryPendingRebuild {
			continue
		}
		for _, statement := range record.Statements {
			if statement.Kind == StatementObjective && statement.Disposition == DispositionAccepted {
				facts = append(facts, statement)
			}
		}
	}
	return facts
}
