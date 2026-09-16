package creation

import (
	"sort"
	"strings"
	"time"
)

var (
	ErrReviewRefMissing       = err("creation review reference is not in candidate")
	ErrReviewVersionConflict  = err("creation review version conflict")
	ErrReviewTargetNotCurrent = err("creation review target is not the working candidate")
)

// ReviewScope 区分可以逐项决定的审阅单位：候选中的实体与方案分节。
type ReviewScope string

const (
	ReviewScopeEntity  ReviewScope = "entity"
	ReviewScopeSection ReviewScope = "section"
)

func (scope ReviewScope) Valid() bool {
	return scope == ReviewScopeEntity || scope == ReviewScopeSection
}

// ReviewDecision 是作者对单个审阅单位的决定；unreviewed 表示撤销已有决定。
type ReviewDecision string

const (
	DecisionAccept     ReviewDecision = "accept"
	DecisionReject     ReviewDecision = "reject"
	DecisionUnreviewed ReviewDecision = "unreviewed"
)

func (decision ReviewDecision) Valid() bool {
	return decision == DecisionAccept || decision == DecisionReject || decision == DecisionUnreviewed
}

// ReviewRef 是审阅单位在候选中的稳定引用；ID 使用驾驶舱投影的实体/分节标识。
type ReviewRef struct {
	Scope ReviewScope `json:"scope"`
	ID    string      `json:"id"`
}

// NewReviewRef 校验作用域与标识，避免空引用或未知作用域进入审阅记录。
func NewReviewRef(scope ReviewScope, id string) (ReviewRef, error) {
	id = strings.TrimSpace(id)
	if !scope.Valid() || id == "" || len(id) > 256 {
		return ReviewRef{}, ErrBlankValue
	}
	return ReviewRef{Scope: scope, ID: id}, nil
}

func (ref ReviewRef) matches(other ReviewRef) bool {
	return ref.Scope == other.Scope && ref.ID == other.ID
}

// CandidateReview 记录作者对某个不可变候选版本的逐项审阅关系。
// 审阅绑定候选版本：新候选版本重新审阅，旧版本的接受与排除不会被继承。
type CandidateReview struct {
	CandidateID   string
	SessionID     string
	Stage         Stage
	ReviewVersion int64
	Accepted      []ReviewRef
	Rejected      []ReviewRef
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// NewCandidateReview 为候选建立空审阅记录，接受与排除都不默认全选。
func NewCandidateReview(candidate Candidate, now time.Time) (CandidateReview, error) {
	if strings.TrimSpace(candidate.ID) == "" || strings.TrimSpace(candidate.SessionID) == "" || !candidate.Stage.Valid() {
		return CandidateReview{}, ErrBlankValue
	}
	now = now.UTC()
	return CandidateReview{
		CandidateID: candidate.ID, SessionID: candidate.SessionID, Stage: candidate.Stage,
		ReviewVersion: 1, Accepted: []ReviewRef{}, Rejected: []ReviewRef{}, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// Decide 记录单个审阅单位的接受、排除或撤销决定；重复决定幂等。
func (review CandidateReview) Decide(ref ReviewRef, decision ReviewDecision, now time.Time) (CandidateReview, error) {
	if review.CandidateID == "" || !ref.Scope.Valid() || strings.TrimSpace(ref.ID) == "" || !decision.Valid() {
		return CandidateReview{}, ErrBlankValue
	}
	accepted := withoutRef(review.Accepted, ref)
	rejected := withoutRef(review.Rejected, ref)
	switch decision {
	case DecisionAccept:
		accepted = append(accepted, ref)
	case DecisionReject:
		rejected = append(rejected, ref)
	}
	if sameRefs(accepted, review.Accepted) && sameRefs(rejected, review.Rejected) {
		return review, nil
	}
	review.Accepted, review.Rejected = sortRefs(accepted), sortRefs(rejected)
	review.UpdatedAt = now.UTC()
	return review, nil
}

// AcceptAll 是显式全接受命令：接受给定单位并清除它们的排除记录。
// 未出现在列表中的单位仍保持未审阅，避免以“没有逐项选择”默认全包入档。
func (review CandidateReview) AcceptAll(refs []ReviewRef, now time.Time) (CandidateReview, error) {
	if review.CandidateID == "" || len(refs) == 0 {
		return CandidateReview{}, ErrBlankValue
	}
	accepted := append([]ReviewRef{}, review.Accepted...)
	rejected := append([]ReviewRef{}, review.Rejected...)
	for _, ref := range refs {
		if !ref.Scope.Valid() || strings.TrimSpace(ref.ID) == "" {
			return CandidateReview{}, ErrBlankValue
		}
		rejected = withoutRef(rejected, ref)
		if !hasRef(accepted, ref) {
			accepted = append(accepted, ref)
		}
	}
	if sameRefs(accepted, review.Accepted) && sameRefs(rejected, review.Rejected) {
		return review, nil
	}
	review.Accepted, review.Rejected = sortRefs(accepted), sortRefs(rejected)
	review.UpdatedAt = now.UTC()
	return review, nil
}

// State 返回该单位当前的审阅决定，未记录时返回 unreviewed。
func (review CandidateReview) State(ref ReviewRef) ReviewDecision {
	switch {
	case hasRef(review.Accepted, ref):
		return DecisionAccept
	case hasRef(review.Rejected, ref):
		return DecisionReject
	default:
		return DecisionUnreviewed
	}
}

// Unreviewed 返回给定单位中尚未决定的引用，用于判断阶段审阅是否完成。
func (review CandidateReview) Unreviewed(refs []ReviewRef) []ReviewRef {
	open := make([]ReviewRef, 0, len(refs))
	for _, ref := range refs {
		if review.State(ref) == DecisionUnreviewed {
			open = append(open, ref)
		}
	}
	return open
}

// ReviewCursor 记录作者正在查看的阶段、候选与分节/实体，供重启恢复审阅位置。
// 位置使用独立 view_version，不使生成输入或候选内容失效。
type ReviewCursor struct {
	SessionID     string
	ViewVersion   int64
	Stage         Stage
	CandidateID   string
	EntityID      string
	SectionID     string
	ComparisonRef string
	UpdatedAt     time.Time
}

// NewReviewCursor 为会话建立初始审阅位置。
func NewReviewCursor(sessionID string, now time.Time) ReviewCursor {
	return ReviewCursor{SessionID: strings.TrimSpace(sessionID), ViewVersion: 1, UpdatedAt: now.UTC()}
}

// Moved 返回移动后的审阅位置；ViewVersion 仍是被替换的旧版本，由仓储在写入时递增。
func (cursor ReviewCursor) Moved(stage Stage, candidateID, entityID, sectionID, comparisonRef string, now time.Time) (ReviewCursor, error) {
	if strings.TrimSpace(cursor.SessionID) == "" || !stage.Valid() {
		return ReviewCursor{}, ErrBlankValue
	}
	if len(entityID) > 256 || len(sectionID) > 256 || len(comparisonRef) > 256 {
		return ReviewCursor{}, ErrBlankValue
	}
	cursor.Stage = stage
	cursor.CandidateID = strings.TrimSpace(candidateID)
	cursor.EntityID = strings.TrimSpace(entityID)
	cursor.SectionID = strings.TrimSpace(sectionID)
	cursor.ComparisonRef = strings.TrimSpace(comparisonRef)
	cursor.UpdatedAt = now.UTC()
	return cursor, nil
}

func hasRef(refs []ReviewRef, ref ReviewRef) bool {
	for _, item := range refs {
		if item.matches(ref) {
			return true
		}
	}
	return false
}

func withoutRef(refs []ReviewRef, ref ReviewRef) []ReviewRef {
	kept := make([]ReviewRef, 0, len(refs))
	for _, item := range refs {
		if !item.matches(ref) {
			kept = append(kept, item)
		}
	}
	return kept
}

func sameRefs(left, right []ReviewRef) bool {
	if len(left) != len(right) {
		return false
	}
	for _, ref := range left {
		if !hasRef(right, ref) {
			return false
		}
	}
	return true
}

// sortRefs 按作用域与标识排序，保证同一决定集合的持久化结果稳定。
func sortRefs(refs []ReviewRef) []ReviewRef {
	sorted := append([]ReviewRef{}, refs...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Scope != sorted[j].Scope {
			return sorted[i].Scope < sorted[j].Scope
		}
		return sorted[i].ID < sorted[j].ID
	})
	return sorted
}
