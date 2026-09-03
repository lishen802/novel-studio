package storyasset

import (
	"strings"
	"time"
)

var (
	ErrBlankValue       = err("story asset value must not be blank")
	ErrInvalidReference = err("story asset typed reference is invalid")
	ErrInvalidKind      = err("invalid story asset kind")
)

type errorString string

func (e errorString) Error() string { return string(e) }

func err(text string) errorString { return errorString(text) }

type Kind string

const (
	KindProposal     Kind = "proposal"
	KindCharacter    Kind = "character"
	KindRelationship Kind = "relationship"
	KindFaction      Kind = "faction"
	KindLocation     Kind = "location"
	KindWorldRule    Kind = "world_rule"
	KindItem         Kind = "item"
	KindOutlineNode  Kind = "outline_node"
	KindHardAnchor   Kind = "hard_anchor"
	KindSoftAnchor   Kind = "soft_anchor"
	KindForbidden    Kind = "forbidden"
	KindStoryEvent   Kind = "story_event"
)

type Origin string

const (
	OriginAuthor Origin = "author"
	OriginModel  Origin = "model"
	OriginImport Origin = "import"
)

type Status string

const (
	StatusCanonical Status = "canonical"
	StatusStale     Status = "stale"
	StatusArchived  Status = "archived"
)

type Asset struct {
	ID                string
	ProjectID         string
	Kind              Kind
	Status            Status
	CurrentRevision   int64
	Origin            Origin
	SourceCandidateID string
	SourceRunID       string
	SourceArtifactID  string
	TypedTable        string
	TypedID           string
	Locked            bool
	Stale             bool
	StaleReason       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// New 在确认路径上建立正式资产信封，类型表行由 ChangeSet 负责写入。
func New(
	id, projectID string, kind Kind, origin Origin,
	sourceCandidateID, sourceRunID, sourceArtifactID, typedTable, typedID string, now time.Time,
) (Asset, error) {
	id, projectID = strings.TrimSpace(id), strings.TrimSpace(projectID)
	typedTable, typedID = strings.TrimSpace(typedTable), strings.TrimSpace(typedID)
	if id == "" || projectID == "" {
		return Asset{}, ErrBlankValue
	}
	if !validKind(kind) {
		return Asset{}, ErrInvalidKind
	}
	if !validOrigin(origin) {
		return Asset{}, ErrBlankValue
	}
	if requiresTypedRow(kind) && (typedTable == "" || typedID == "") {
		return Asset{}, ErrInvalidReference
	}
	if !requiresTypedRow(kind) && (typedTable != "" || typedID != "") {
		return Asset{}, ErrInvalidReference
	}
	now = now.UTC()
	return Asset{
		ID: id, ProjectID: projectID, Kind: kind, Status: StatusCanonical, CurrentRevision: 1,
		Origin: origin, SourceCandidateID: strings.TrimSpace(sourceCandidateID),
		SourceRunID: strings.TrimSpace(sourceRunID), SourceArtifactID: strings.TrimSpace(sourceArtifactID),
		TypedTable: typedTable, TypedID: typedID, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (asset Asset) MarkStale(reason string, now time.Time) (Asset, error) {
	reason = strings.TrimSpace(reason)
	if asset.ID == "" || reason == "" {
		return Asset{}, ErrBlankValue
	}
	asset.Stale = true
	asset.StaleReason = reason
	asset.Status = StatusStale
	asset.UpdatedAt = now.UTC()
	return asset, nil
}

func requiresTypedRow(kind Kind) bool {
	switch kind {
	case KindProposal, KindHardAnchor, KindSoftAnchor, KindForbidden:
		return false
	default:
		return true
	}
}

func validKind(kind Kind) bool {
	switch kind {
	case KindProposal, KindCharacter, KindRelationship, KindFaction, KindLocation, KindWorldRule,
		KindItem, KindOutlineNode, KindHardAnchor, KindSoftAnchor, KindForbidden, KindStoryEvent:
		return true
	default:
		return false
	}
}

func validOrigin(origin Origin) bool {
	switch origin {
	case OriginAuthor, OriginModel, OriginImport:
		return true
	default:
		return false
	}
}
