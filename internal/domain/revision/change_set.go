package revision

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankValue        = errors.New("required value must not be blank")
	ErrInvalidRevision   = errors.New("base revision must be positive")
	ErrNoopOperation     = errors.New("patch operation must change content")
	ErrInvalidTransition = errors.New("invalid change set status transition")
	ErrInvalidOperation  = errors.New("invalid patch operation")
	ErrOperationMissing  = errors.New("patch operation does not exist")
)

type ChangeSetStatus string

const (
	ChangeSetPending          ChangeSetStatus = "pending"
	ChangeSetApplied          ChangeSetStatus = "applied"
	ChangeSetRejected         ChangeSetStatus = "rejected"
	ChangeSetConflicted       ChangeSetStatus = "conflicted"
	ChangeSetPartiallyApplied ChangeSetStatus = "partially_applied"
)

type OperationKind string

const (
	OperationReplaceChapterBody OperationKind = "replace_chapter_body"
	OperationUpdateOutlineNode  OperationKind = "update_outline_node"
	OperationUpdateCharacter    OperationKind = "update_character"
	OperationUpdateFact         OperationKind = "update_fact"
)

// Create operations introduce brand-new story assets. They are produced by
// manuscript extraction (and future AI proposals) where no baseline entity
// exists yet. before_value is canonicalized to "{}" and base_fingerprint to
// the SHA-256 of "{}", satisfying the change_operations CHECK constraints
// inherited from migration 00010.
const (
	OperationCreateOutlineNode       OperationKind = "create_outline_node"
	OperationCreateCharacter         OperationKind = "create_character"
	OperationCreateRelationship      OperationKind = "create_relationship"
	OperationCreateWorldRule         OperationKind = "create_world_rule"
	OperationCreateStoryEvent        OperationKind = "create_story_event"
	OperationCreateFact              OperationKind = "create_fact"
	OperationCreateForeshadowingHook OperationKind = "create_foreshadowing_hook"
	OperationCreateFaction           OperationKind = "create_faction"
	OperationCreateLocation          OperationKind = "create_location"
	OperationCreateAnchor            OperationKind = "create_anchor"
	OperationCreateStoryAsset        OperationKind = "create_story_asset"
	OperationUpdateProject           OperationKind = "update_project"
)

// Evidence ties a candidate operation back to the manuscript line range that
// motivated it, so authors can verify AI-proposed assets against the source
// text before confirming.
type Evidence struct {
	ChapterID    string `json:"chapterId"`
	ChapterTitle string `json:"chapterTitle"`
	StartLine    int    `json:"startLine"`
	EndLine      int    `json:"endLine"`
	Excerpt      string `json:"excerpt"`
}

// IsCreateOperation reports whether the kind introduces a new asset.
func IsCreateOperation(kind OperationKind) bool {
	switch kind {
	case OperationCreateOutlineNode, OperationCreateCharacter, OperationCreateRelationship,
		OperationCreateWorldRule, OperationCreateStoryEvent, OperationCreateFact,
		OperationCreateForeshadowingHook, OperationCreateFaction, OperationCreateLocation,
		OperationCreateAnchor, OperationCreateStoryAsset:
		return true
	}
	return false
}

// EmptyObjectFingerprint is the base_fingerprint value for create operations.
func EmptyObjectFingerprint() string {
	sum := sha256.Sum256([]byte("{}"))
	return hex.EncodeToString(sum[:])
}

type PatchOperationSpec struct {
	ID           string
	Kind         OperationKind
	TargetID     string
	BaseRevision int64
	BeforeValue  string
	AfterValue   string
	Reason       string
	Evidence     []Evidence
}

type PatchOperation struct {
	ID              string
	ChangeSetID     string
	Kind            OperationKind
	TargetID        string
	BaseRevision    int64
	BaseFingerprint string
	BeforeValue     string
	AfterValue      string
	Reason          string
	Status          ChangeSetStatus
	Evidence        []Evidence
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type ChangeSet struct {
	ID         string
	ProjectID  string
	Title      string
	Summary    string
	CreatedBy  string
	Status     ChangeSetStatus
	Operations []PatchOperation
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewChapterBodyChangeSet(
	id string,
	operationID string,
	projectID string,
	chapterID string,
	title string,
	summary string,
	createdBy string,
	baseRevision int64,
	beforeBody string,
	afterBody string,
	reason string,
	now time.Time,
) (ChangeSet, error) {
	return NewChangeSet(
		id,
		projectID,
		title,
		summary,
		createdBy,
		[]PatchOperationSpec{{
			ID:           operationID,
			Kind:         OperationReplaceChapterBody,
			TargetID:     chapterID,
			BaseRevision: baseRevision,
			BeforeValue:  beforeBody,
			AfterValue:   afterBody,
			Reason:       reason,
		}},
		now,
	)
}

func NewChangeSet(
	id string,
	projectID string,
	title string,
	summary string,
	createdBy string,
	specs []PatchOperationSpec,
	now time.Time,
) (ChangeSet, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	title = strings.TrimSpace(title)
	createdBy = strings.TrimSpace(createdBy)
	if id == "" || projectID == "" || title == "" || createdBy == "" {
		return ChangeSet{}, ErrBlankValue
	}
	if len(specs) == 0 {
		return ChangeSet{}, ErrInvalidOperation
	}

	now = now.UTC()
	operations := make([]PatchOperation, 0, len(specs))
	seen := make(map[string]struct{}, len(specs))
	for _, spec := range specs {
		operationID := strings.TrimSpace(spec.ID)
		targetID := strings.TrimSpace(spec.TargetID)
		if operationID == "" || targetID == "" || !validOperationKind(spec.Kind) {
			return ChangeSet{}, ErrInvalidOperation
		}
		if _, exists := seen[operationID]; exists {
			return ChangeSet{}, ErrInvalidOperation
		}
		seen[operationID] = struct{}{}

		beforeValue, afterValue, err := normalizeOperationValues(
			spec.Kind,
			spec.BeforeValue,
			spec.AfterValue,
		)
		if err != nil {
			return ChangeSet{}, err
		}
		if spec.Kind == OperationReplaceChapterBody {
			if spec.BaseRevision < 1 {
				return ChangeSet{}, ErrInvalidRevision
			}
		} else if spec.BaseRevision != 0 {
			return ChangeSet{}, ErrInvalidRevision
		}
		operations = append(operations, PatchOperation{
			ID:              operationID,
			ChangeSetID:     id,
			Kind:            spec.Kind,
			TargetID:        targetID,
			BaseRevision:    spec.BaseRevision,
			BaseFingerprint: fingerprint(beforeValue),
			BeforeValue:     beforeValue,
			AfterValue:      afterValue,
			Reason:          strings.TrimSpace(spec.Reason),
			Status:          ChangeSetPending,
			Evidence:        append([]Evidence(nil), spec.Evidence...),
			CreatedAt:       now,
			UpdatedAt:       now,
		})
	}

	return ChangeSet{
		ID:         id,
		ProjectID:  projectID,
		Title:      title,
		Summary:    strings.TrimSpace(summary),
		CreatedBy:  createdBy,
		Status:     ChangeSetPending,
		Operations: operations,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (c ChangeSet) Apply(now time.Time) (ChangeSet, error) {
	if c.Status == ChangeSetApplied {
		return c, nil
	}
	if c.Status == ChangeSetRejected || c.Status == ChangeSetConflicted {
		return ChangeSet{}, ErrInvalidTransition
	}
	c = c.clone()
	now = now.UTC()
	for index := range c.Operations {
		if c.Operations[index].Status == ChangeSetPending {
			c.Operations[index].Status = ChangeSetApplied
			c.Operations[index].UpdatedAt = now
		}
	}
	c.Status = deriveChangeSetStatus(c.Operations)
	c.UpdatedAt = now
	return c, nil
}

func (c ChangeSet) Reject(now time.Time) (ChangeSet, error) {
	if c.Status == ChangeSetRejected {
		return c, nil
	}
	if c.Status == ChangeSetApplied {
		return ChangeSet{}, ErrInvalidTransition
	}
	c = c.clone()
	now = now.UTC()
	for index := range c.Operations {
		if c.Operations[index].Status == ChangeSetPending ||
			c.Operations[index].Status == ChangeSetConflicted {
			c.Operations[index].Status = ChangeSetRejected
			c.Operations[index].UpdatedAt = now
		}
	}
	c.Status = deriveChangeSetStatus(c.Operations)
	c.UpdatedAt = now
	return c, nil
}

func (c ChangeSet) Conflict(now time.Time) (ChangeSet, error) {
	if c.Status == ChangeSetConflicted {
		return c, nil
	}
	if c.Status != ChangeSetPending {
		return ChangeSet{}, ErrInvalidTransition
	}
	return c.withStatus(ChangeSetConflicted, now), nil
}

func (c ChangeSet) EditOperation(
	operationID string,
	afterValue string,
	now time.Time,
) (ChangeSet, error) {
	index := c.operationIndex(operationID)
	if index < 0 {
		return ChangeSet{}, ErrOperationMissing
	}
	if c.Operations[index].Status != ChangeSetPending {
		return ChangeSet{}, ErrInvalidTransition
	}
	beforeValue, normalizedAfter, err := normalizeOperationValues(
		c.Operations[index].Kind,
		c.Operations[index].BeforeValue,
		afterValue,
	)
	if err != nil {
		return ChangeSet{}, err
	}
	c = c.clone()
	c.Operations[index].BeforeValue = beforeValue
	c.Operations[index].AfterValue = normalizedAfter
	c.Operations[index].UpdatedAt = now.UTC()
	c.UpdatedAt = now.UTC()
	return c, nil
}

func (c ChangeSet) ApplyOperation(
	operationID string,
	now time.Time,
) (ChangeSet, error) {
	return c.decideOperation(operationID, ChangeSetApplied, now)
}

func (c ChangeSet) RejectOperation(
	operationID string,
	now time.Time,
) (ChangeSet, error) {
	return c.decideOperation(operationID, ChangeSetRejected, now)
}

func (c ChangeSet) ConflictOperation(
	operationID string,
	now time.Time,
) (ChangeSet, error) {
	return c.decideOperation(operationID, ChangeSetConflicted, now)
}

func (c ChangeSet) decideOperation(
	operationID string,
	status ChangeSetStatus,
	now time.Time,
) (ChangeSet, error) {
	index := c.operationIndex(operationID)
	if index < 0 {
		return ChangeSet{}, ErrOperationMissing
	}
	current := c.Operations[index].Status
	if current == status {
		return c, nil
	}
	if current != ChangeSetPending &&
		!(status == ChangeSetRejected && current == ChangeSetConflicted) {
		return ChangeSet{}, ErrInvalidTransition
	}
	c = c.clone()
	c.Operations[index].Status = status
	c.Operations[index].UpdatedAt = now.UTC()
	c.Status = deriveChangeSetStatus(c.Operations)
	c.UpdatedAt = now.UTC()
	return c, nil
}

func (c ChangeSet) withStatus(status ChangeSetStatus, now time.Time) ChangeSet {
	c = c.clone()
	now = now.UTC()
	c.Status = status
	c.UpdatedAt = now
	for index := range c.Operations {
		c.Operations[index].Status = status
		c.Operations[index].UpdatedAt = now
	}
	return c
}

func (c ChangeSet) operationIndex(operationID string) int {
	operationID = strings.TrimSpace(operationID)
	for index := range c.Operations {
		if c.Operations[index].ID == operationID {
			return index
		}
	}
	return -1
}

func (c ChangeSet) clone() ChangeSet {
	c.Operations = append([]PatchOperation(nil), c.Operations...)
	return c
}

func deriveChangeSetStatus(operations []PatchOperation) ChangeSetStatus {
	var pending, applied, rejected, conflicted int
	for _, operation := range operations {
		switch operation.Status {
		case ChangeSetPending:
			pending++
		case ChangeSetApplied:
			applied++
		case ChangeSetRejected:
			rejected++
		case ChangeSetConflicted:
			conflicted++
		}
	}
	if conflicted > 0 {
		return ChangeSetConflicted
	}
	if pending > 0 {
		if applied > 0 {
			return ChangeSetPartiallyApplied
		}
		return ChangeSetPending
	}
	if applied > 0 {
		return ChangeSetApplied
	}
	if rejected == len(operations) {
		return ChangeSetRejected
	}
	return ChangeSetPending
}

func validOperationKind(kind OperationKind) bool {
	switch kind {
	case OperationReplaceChapterBody,
		OperationUpdateOutlineNode,
		OperationUpdateCharacter,
		OperationUpdateFact,
		OperationCreateOutlineNode,
		OperationCreateCharacter,
		OperationCreateRelationship,
		OperationCreateWorldRule,
		OperationCreateStoryEvent,
		OperationCreateFact,
		OperationCreateForeshadowingHook,
		OperationCreateFaction,
		OperationCreateLocation,
		OperationCreateAnchor,
		OperationCreateStoryAsset,
		OperationUpdateProject:
		return true
	default:
		return false
	}
}

func normalizeOperationValues(
	kind OperationKind,
	beforeValue string,
	afterValue string,
) (string, string, error) {
	if IsCreateOperation(kind) {
		afterValue, err := canonicalJSONObject(afterValue)
		if err != nil {
			return "", "", ErrInvalidOperation
		}
		if afterValue == "{}" {
			return "", "", ErrNoopOperation
		}
		return "{}", afterValue, nil
	}
	if kind == OperationReplaceChapterBody {
		if beforeValue == afterValue {
			return "", "", ErrNoopOperation
		}
		return beforeValue, afterValue, nil
	}
	beforeValue, err := canonicalJSONObject(beforeValue)
	if err != nil {
		return "", "", ErrInvalidOperation
	}
	afterValue, err = canonicalJSONObject(afterValue)
	if err != nil {
		return "", "", ErrInvalidOperation
	}
	if beforeValue == afterValue {
		return "", "", ErrNoopOperation
	}
	return beforeValue, afterValue, nil
}

func canonicalJSONObject(value string) (string, error) {
	var object map[string]any
	if err := json.Unmarshal([]byte(value), &object); err != nil ||
		object == nil {
		return "", ErrInvalidOperation
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return "", ErrInvalidOperation
	}
	return string(encoded), nil
}

func fingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
