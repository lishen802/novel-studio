package creation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrCandidateVersionConflict = errors.New("creation candidate version conflict")
	ErrCandidateResultExists    = errors.New("creation candidate result already exists")
)

type CandidateKind string

const (
	KindDirectionSet CandidateKind = "direction_set"
	KindDirection    CandidateKind = "direction"
	KindProposal     CandidateKind = "proposal"
	KindPeopleWorld  CandidateKind = "people_world"
	KindRoadmap      CandidateKind = "roadmap"
)

type CandidateStatus string

const (
	StatusDraft      CandidateStatus = "draft"
	StatusSelected   CandidateStatus = "selected"
	StatusConfirmed  CandidateStatus = "confirmed"
	StatusSuperseded CandidateStatus = "superseded"
	StatusStale      CandidateStatus = "stale"
	StatusDiscarded  CandidateStatus = "discarded"
)

type CandidateOrigin string

const (
	OriginModel  CandidateOrigin = "model"
	OriginAuthor CandidateOrigin = "author"
)

// CandidateQuality 记录生成运行对该候选的最终质量结论；空值表示尚未评估。
type CandidateQuality string

const (
	QualityNone        CandidateQuality = ""
	QualityPassed      CandidateQuality = "passed"
	QualityNeedsReview CandidateQuality = "needs_review"
	QualityBlocked     CandidateQuality = "blocked"
)

// Blocked 表示该候选被质量门禁阻断，只能审阅，不能推进或入档。
func (candidate Candidate) Blocked() bool {
	return candidate.QualityStatus == QualityBlocked
}

type Candidate struct {
	ID                     string
	SessionID              string
	Stage                  Stage
	Kind                   CandidateKind
	Version                int
	ParentCandidateID      string
	SupersedesCandidateID  string
	RunID                  string
	ArtifactID             string
	PayloadJSON            string
	PayloadSchemaID        string
	PayloadSchemaVersion   int
	ContentSHA256          string
	LockMap                map[string]bool
	Status                 CandidateStatus
	QualityStatus          CandidateQuality
	Origin                 CandidateOrigin
	UpstreamDigest         string
	StaleReason            string
	ImpactRefs             []string
	ImpactSeverity         ImpactSeverity
	ExceptedUpstreamDigest string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// NewCandidate 保存一次生成或导入的结构化候选，并计算内容摘要。
func NewCandidate(
	id, sessionID string, stage Stage, kind CandidateKind, version int,
	parentCandidateID, runID, artifactID, payloadJSON, schemaID string, schemaVersion int, now time.Time,
) (Candidate, error) {
	id, sessionID = strings.TrimSpace(id), strings.TrimSpace(sessionID)
	schemaID = strings.TrimSpace(schemaID)
	if id == "" || sessionID == "" || !stage.Valid() || !validKind(kind) || version < 1 ||
		strings.TrimSpace(payloadJSON) == "" || schemaID == "" || schemaVersion < 1 {
		return Candidate{}, ErrBlankValue
	}
	if !json.Valid([]byte(payloadJSON)) {
		return Candidate{}, ErrInvalidJSON
	}
	now = now.UTC()
	origin := OriginModel
	if strings.TrimSpace(runID) == "" {
		origin = OriginAuthor
	}
	return Candidate{
		ID: id, SessionID: sessionID, Stage: stage, Kind: kind, Version: version,
		ParentCandidateID: strings.TrimSpace(parentCandidateID),
		RunID:             strings.TrimSpace(runID), ArtifactID: strings.TrimSpace(artifactID),
		PayloadJSON: payloadJSON, PayloadSchemaID: schemaID, PayloadSchemaVersion: schemaVersion,
		ContentSHA256: digestJSON(payloadJSON), LockMap: map[string]bool{},
		Status: StatusDraft, Origin: origin, ImpactRefs: []string{}, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (candidate Candidate) LockField(path string, now time.Time) (Candidate, error) {
	path = normalizePointer(path)
	if candidate.ID == "" || path == "" {
		return Candidate{}, ErrBlankValue
	}
	if _, err := candidate.FieldString(path); err != nil {
		return Candidate{}, err
	}
	locks := cloneLocks(candidate.LockMap)
	locks[path] = true
	candidate.LockMap = locks
	candidate.UpdatedAt = now.UTC()
	return candidate, nil
}

// UnlockField 解除字段锁，之后作者草稿或定向修订才能改该路径。
func (candidate Candidate) UnlockField(path string, now time.Time) (Candidate, error) {
	path = normalizePointer(path)
	if candidate.ID == "" || path == "" {
		return Candidate{}, ErrBlankValue
	}
	locks := cloneLocks(candidate.LockMap)
	delete(locks, path)
	candidate.LockMap = locks
	candidate.UpdatedAt = now.UTC()
	return candidate, nil
}

// Confirm 把当前候选标为阶段确认，仍不写入正式资产；stale、被替代或已放弃的候选不能确认。
func (candidate Candidate) Confirm(now time.Time) (Candidate, error) {
	if candidate.ID == "" {
		return Candidate{}, ErrBlankValue
	}
	switch candidate.Status {
	case StatusSuperseded, StatusDiscarded, StatusStale:
		return Candidate{}, ErrInvalidTransition
	}
	if candidate.Blocked() {
		return Candidate{}, ErrQualityBlocked
	}
	candidate.Status = StatusConfirmed
	candidate.UpdatedAt = now.UTC()
	return candidate, nil
}

// MarkSuperseded 把被新版本替代的候选标为 superseded，不改 payload。
func (candidate Candidate) MarkSuperseded(now time.Time) (Candidate, error) {
	if candidate.ID == "" {
		return Candidate{}, ErrBlankValue
	}
	candidate.Status = StatusSuperseded
	candidate.UpdatedAt = now.UTC()
	return candidate, nil
}

func (candidate Candidate) Locked(path string) bool {
	return candidate.LockMap[normalizePointer(path)]
}

func (candidate Candidate) FieldString(path string) (string, error) {
	value, err := readPointer(candidate.PayloadJSON, path)
	if err != nil {
		return "", err
	}
	text, ok := value.(string)
	if !ok {
		return "", ErrFieldMissing
	}
	return text, nil
}

// SaveDraft 把作者直接编辑存为新候选版本，不覆盖正在审阅的版本。
func (candidate Candidate) SaveDraft(newID, payloadJSON string, now time.Time) (Candidate, error) {
	if err := candidate.rejectLockedChanges(payloadJSON); err != nil {
		return Candidate{}, err
	}
	draft, err := NewCandidate(
		newID, candidate.SessionID, candidate.Stage, candidate.Kind, candidate.Version+1,
		candidate.ID, "", "", payloadJSON, candidate.PayloadSchemaID, candidate.PayloadSchemaVersion, now,
	)
	if err != nil {
		return Candidate{}, err
	}
	draft.LockMap = cloneLocks(candidate.LockMap)
	draft.Origin = OriginAuthor
	draft.ParentCandidateID = candidate.ID
	draft.SupersedesCandidateID = candidate.ID
	draft.UpstreamDigest = candidate.UpstreamDigest
	return draft, nil
}

// Regenerate 从模型新产出创建下一版本，并强制保留锁定字段。
func (candidate Candidate) Regenerate(newID, runID, artifactID, payloadJSON string, now time.Time) (Candidate, error) {
	merged, err := mergeLockedFields(candidate.PayloadJSON, payloadJSON, candidate.LockMap)
	if err != nil {
		return Candidate{}, err
	}
	next, err := NewCandidate(
		newID, candidate.SessionID, candidate.Stage, candidate.Kind, candidate.Version+1,
		candidate.ID, runID, artifactID, merged, candidate.PayloadSchemaID, candidate.PayloadSchemaVersion, now,
	)
	if err != nil {
		return Candidate{}, err
	}
	next.LockMap = cloneLocks(candidate.LockMap)
	next.ParentCandidateID = candidate.ID
	next.SupersedesCandidateID = candidate.ID
	next.UpstreamDigest = candidate.UpstreamDigest
	return next, nil
}

// WithQuality 记录生成运行对该候选的最终质量结论。
func (candidate Candidate) WithQuality(quality CandidateQuality) Candidate {
	candidate.QualityStatus = quality
	return candidate
}

func (candidate Candidate) rejectLockedChanges(payloadJSON string) error {
	for path, locked := range candidate.LockMap {
		if !locked {
			continue
		}
		before, err := candidate.FieldString(path)
		if err != nil {
			return err
		}
		afterValue, err := readPointer(payloadJSON, path)
		if err != nil {
			return err
		}
		after, _ := afterValue.(string)
		if after != before {
			return ErrFieldLocked
		}
	}
	return nil
}

func mergeLockedFields(baseline, incoming string, locks map[string]bool) (string, error) {
	baseMap, err := decodeObject(baseline)
	if err != nil {
		return "", err
	}
	nextMap, err := decodeObject(incoming)
	if err != nil {
		return "", err
	}
	for path, locked := range locks {
		if !locked {
			continue
		}
		value, err := pointerValue(baseMap, path)
		if err != nil {
			return "", err
		}
		if err := setPointerValue(nextMap, path, value); err != nil {
			return "", err
		}
	}
	encoded, err := json.Marshal(nextMap)
	if err != nil {
		return "", ErrInvalidJSON
	}
	return string(encoded), nil
}

func decodeObject(payload string) (map[string]any, error) {
	var object map[string]any
	if err := json.Unmarshal([]byte(payload), &object); err != nil {
		return nil, ErrInvalidJSON
	}
	return object, nil
}

func readPointer(payload, path string) (any, error) {
	object, err := decodeObject(payload)
	if err != nil {
		return nil, err
	}
	return pointerValue(object, path)
}

func normalizePointer(path string) string {
	return strings.TrimSpace(path)
}

func cloneLocks(locks map[string]bool) map[string]bool {
	cloned := make(map[string]bool, len(locks))
	for path, locked := range locks {
		cloned[path] = locked
	}
	return cloned
}

func digestJSON(payload string) string {
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func validKind(kind CandidateKind) bool {
	switch kind {
	case KindDirectionSet, KindDirection, KindProposal, KindPeopleWorld, KindRoadmap:
		return true
	default:
		return false
	}
}

// CandidateCommitGuard 在候选采用事务中核对作者会话和项目版本。
type CandidateCommitGuard struct {
	SessionID        string
	ContentVersion   int64
	ProjectID        string
	ProjectUpdatedAt time.Time
}
