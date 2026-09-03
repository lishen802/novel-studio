package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	"github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	storyassetdomain "github.com/lishen802/novel-studio/internal/domain/storyasset"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

// createOutlineNodeValue mirrors the JSON shape stored in after_value for
// create_outline_node operations. parent_id is optional; level/position are
// required so the merge layer can place the candidate in the outline tree.
type createOutlineNodeValue struct {
	ParentID string                   `json:"parentId,omitempty"`
	Level    storydomain.OutlineLevel `json:"level"`
	Title    string                   `json:"title"`
	Summary  string                   `json:"summary"`
	Goal     string                   `json:"goal"`
	Conflict string                   `json:"conflict"`
	Outcome  string                   `json:"outcome"`
	Position int                      `json:"position"`
}

// createRelationshipValue references characters by their resolved IDs. The
// merge step is responsible for resolving model-supplied names to the
// target_id of co-located create_character operations in the same ChangeSet.
type createRelationshipValue struct {
	SourceCharacterID string                       `json:"sourceCharacterId"`
	TargetCharacterID string                       `json:"targetCharacterId"`
	Kind              worlddomain.RelationshipKind `json:"kind"`
	Label             string                       `json:"label"`
	Strength          int                          `json:"strength"`
	PublicState       string                       `json:"publicState"`
	TrueState         string                       `json:"trueState"`
}

type createWorldRuleValue struct {
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	ScopeType   worlddomain.RuleScopeType `json:"scopeType"`
	ScopeID     string                    `json:"scopeId,omitempty"`
}

type createStoryEventValue struct {
	Title          string                     `json:"title"`
	Summary        string                     `json:"summary,omitempty"`
	Cause          string                     `json:"cause,omitempty"`
	Result         string                     `json:"result,omitempty"`
	StoryTime      string                     `json:"storyTime,omitempty"`
	StoryOrder     int64                      `json:"storyOrder"`
	NarrativeOrder int                        `json:"narrativeOrder"`
	Status         timelinedomain.EventStatus `json:"status"`
}

type createForeshadowingHookValue struct {
	Title            string `json:"title"`
	Setup            string `json:"setup"`
	PlantedChapterID string `json:"plantedChapterId"`
	DueChapterID     string `json:"dueChapterId,omitempty"`
}

type createFactionValue struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Goal    string `json:"goal"`
}

type createLocationValue struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Summary  string `json:"summary"`
	RegionID string `json:"regionId,omitempty"`
}

type createAnchorValue struct {
	Strength          string `json:"strength"`
	ScopeKind         string `json:"scopeKind"`
	ScopeID           string `json:"scopeId,omitempty"`
	Statement         string `json:"statement"`
	Origin            string `json:"origin,omitempty"`
	SourceCandidateID string `json:"sourceCandidateId,omitempty"`
	SourceRunID       string `json:"sourceRunId,omitempty"`
	SourceArtifactID  string `json:"sourceArtifactId,omitempty"`
}

type createStoryAssetValue struct {
	Kind              string `json:"kind"`
	Origin            string `json:"origin"`
	SourceCandidateID string `json:"sourceCandidateId,omitempty"`
	SourceRunID       string `json:"sourceRunId,omitempty"`
	SourceArtifactID  string `json:"sourceArtifactId,omitempty"`
	TypedTable        string `json:"typedTable,omitempty"`
	TypedID           string `json:"typedId,omitempty"`
	PayloadJSON       string `json:"payloadJson"`
}

type projectMetaValue struct {
	Title   string `json:"title"`
	Genre   string `json:"genre"`
	Summary string `json:"summary"`
}

func prepareCreateOutlineOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed createOutlineNodeValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	if proposed.Level == storydomain.LevelBook && proposed.ParentID != "" {
		return preparedOperation{}, storydomain.ErrInvalidHierarchy
	}
	if _, valid := storyLevelOrder(proposed.Level); !valid {
		return preparedOperation{}, storydomain.ErrInvalidHierarchy
	}
	node := storydomain.OutlineNode{
		ID:        operation.TargetID,
		ProjectID: changeSet.ProjectID,
		ParentID:  strings.TrimSpace(proposed.ParentID),
		Level:     proposed.Level,
		Title:     strings.TrimSpace(proposed.Title),
		Summary:   strings.TrimSpace(proposed.Summary),
		Goal:      strings.TrimSpace(proposed.Goal),
		Conflict:  strings.TrimSpace(proposed.Conflict),
		Outcome:   strings.TrimSpace(proposed.Outcome),
		Position:  proposed.Position,
		ChapterID: "",
		CreatedAt: appliedAt,
		UpdatedAt: appliedAt,
	}
	if node.Title == "" || node.Position < 1 {
		return preparedOperation{}, storydomain.ErrBlankValue
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO outline_nodes (
				id, project_id, parent_id, level, title, summary, goal, conflict,
				outcome, position, chapter_id, created_at, updated_at, deleted_at
			) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, NULL)
		`, node.ID, node.ProjectID, node.ParentID, node.Level, node.Title, node.Summary,
			node.Goal, node.Conflict, node.Outcome, node.Position, node.ChapterID,
			formatTime(node.CreatedAt), formatTime(node.UpdatedAt))
		if err != nil {
			return nil, databaseOperationError("apply create outline node operation", err)
		}
		return nil, requireOneRow(result, "outline node", node.ID)
	}}, nil
}

func prepareCreateCharacterOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed characterValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	if proposed.Name == "" {
		return preparedOperation{}, worlddomain.ErrBlankValue
	}
	character, err := worlddomain.NewCharacter(
		operation.TargetID,
		changeSet.ProjectID,
		proposed.Name,
		proposed.Aliases,
		proposed.Role,
		proposed.Motivation,
		proposed.Fear,
		proposed.Secret,
		proposed.Arc,
		appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	character.Summary = strings.TrimSpace(proposed.Summary)
	character.Status = proposed.Status
	if character.Status == "" {
		character.Status = worlddomain.CharacterActive
	}
	aliases, err := json.Marshal(character.Aliases)
	if err != nil {
		return preparedOperation{}, fmt.Errorf("encode character aliases: %w", err)
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO characters (
				id, project_id, name, aliases_json, role, summary, motivation, fear,
				secret, arc, status, created_at, updated_at, deleted_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
		`, character.ID, character.ProjectID, character.Name, string(aliases), character.Role,
			character.Summary, character.Motivation, character.Fear, character.Secret,
			character.Arc, character.Status, formatTime(character.CreatedAt), formatTime(character.UpdatedAt))
		if err != nil {
			return nil, databaseOperationError("apply create character operation", err)
		}
		return nil, requireOneRow(result, "character", character.ID)
	}}, nil
}

func prepareCreateRelationshipOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed createRelationshipValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	proposed.SourceCharacterID = strings.TrimSpace(proposed.SourceCharacterID)
	proposed.TargetCharacterID = strings.TrimSpace(proposed.TargetCharacterID)
	if proposed.SourceCharacterID == "" || proposed.TargetCharacterID == "" ||
		proposed.SourceCharacterID == proposed.TargetCharacterID {
		return preparedOperation{}, worlddomain.ErrInvalidReference
	}
	if proposed.Strength < -100 || proposed.Strength > 100 {
		return preparedOperation{}, worlddomain.ErrInvalidStrength
	}
	switch proposed.Kind {
	case worlddomain.RelationshipAlly, worlddomain.RelationshipFamily, worlddomain.RelationshipRomantic,
		worlddomain.RelationshipRival, worlddomain.RelationshipEnemy, worlddomain.RelationshipMentor,
		worlddomain.RelationshipOther:
	default:
		return preparedOperation{}, worlddomain.ErrInvalidRelation
	}
	relationship := worlddomain.Relationship{
		ID:                operation.TargetID,
		ProjectID:         changeSet.ProjectID,
		SourceCharacterID: proposed.SourceCharacterID,
		TargetCharacterID: proposed.TargetCharacterID,
		Kind:              proposed.Kind,
		Label:             strings.TrimSpace(proposed.Label),
		Strength:          proposed.Strength,
		PublicState:       strings.TrimSpace(proposed.PublicState),
		TrueState:         strings.TrimSpace(proposed.TrueState),
		CreatedAt:         appliedAt,
		UpdatedAt:         appliedAt,
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO character_relationships (
				id, project_id, source_character_id, target_character_id, kind, label,
				strength, public_state, true_state, created_at, updated_at, deleted_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
		`, relationship.ID, relationship.ProjectID, relationship.SourceCharacterID, relationship.TargetCharacterID,
			relationship.Kind, relationship.Label, relationship.Strength, relationship.PublicState,
			relationship.TrueState, formatTime(relationship.CreatedAt), formatTime(relationship.UpdatedAt))
		if err != nil {
			return nil, databaseOperationError("apply create relationship operation", err)
		}
		return nil, requireOneRow(result, "character relationship", relationship.ID)
	}}, nil
}

func prepareCreateWorldRuleOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed createWorldRuleValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	rule, err := worlddomain.NewWorldRule(
		operation.TargetID,
		changeSet.ProjectID,
		proposed.Name,
		proposed.Description,
		proposed.ScopeType,
		proposed.ScopeID,
		appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO world_rules(id, project_id, name, description, scope_type, scope_id, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, rule.ID, rule.ProjectID, rule.Name, rule.Description, rule.ScopeType, nullString(rule.ScopeID),
			rule.Status, formatTime(rule.CreatedAt), formatTime(rule.UpdatedAt))
		if err != nil {
			return nil, databaseOperationError("apply create world rule operation", err)
		}
		return nil, requireOneRow(result, "world rule", rule.ID)
	}}, nil
}

func prepareCreateStoryEventOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed createStoryEventValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	event, err := timelinedomain.NewStoryEvent(
		operation.TargetID,
		changeSet.ProjectID,
		proposed.Title,
		proposed.StoryTime,
		proposed.StoryOrder,
		proposed.NarrativeOrder,
		proposed.Status,
		appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	event.Summary = strings.TrimSpace(proposed.Summary)
	event.Cause = strings.TrimSpace(proposed.Cause)
	event.Result = strings.TrimSpace(proposed.Result)
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO story_events(id, project_id, title, summary, cause, result, story_time, story_order, narrative_order, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, event.ID, event.ProjectID, event.Title, event.Summary, event.Cause, event.Result,
			event.StoryTime, event.StoryOrder, event.NarrativeOrder, event.Status,
			formatTime(event.CreatedAt), formatTime(event.UpdatedAt))
		if err != nil {
			return nil, databaseOperationError("apply create story event operation", err)
		}
		return nil, requireOneRow(result, "story event", event.ID)
	}}, nil
}

func prepareCreateFactOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed factValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	if proposed.Text == "" {
		return preparedOperation{}, canondomain.ErrBlankValue
	}
	// Extraction always produces model-inferred facts awaiting author review.
	origin := canondomain.OriginModel
	status := proposed.Status
	if status == "" {
		status = canondomain.StatusAIInferred
	}
	confidence := proposed.Confidence
	if confidence == 0 {
		confidence = 0.5
	}
	fact, err := canondomain.NewFact(
		operation.TargetID,
		changeSet.ProjectID,
		proposed.Text,
		proposed.Category,
		origin,
		status,
		confidence,
		proposed.ValidFromEventID,
		proposed.ValidUntilEventID,
		appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO canon_facts(id, project_id, fact_text, category, origin, status, confidence, valid_from_event_id, valid_until_event_id, created_by, confirmed_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, fact.ID, fact.ProjectID, fact.Text, fact.Category, fact.Origin, fact.Status, fact.Confidence,
			nullString(fact.ValidFromEventID), nullString(fact.ValidUntilEventID), fact.CreatedBy,
			nullString(fact.ConfirmedBy), formatTime(fact.CreatedAt), formatTime(fact.UpdatedAt))
		if err != nil {
			return nil, databaseOperationError("apply create fact operation", err)
		}
		return nil, requireOneRow(result, "canon fact", fact.ID)
	}}, nil
}

func prepareCreateForeshadowingHookOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed createForeshadowingHookValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	hook, err := worlddomain.NewForeshadowingHook(
		operation.TargetID,
		changeSet.ProjectID,
		proposed.Title,
		proposed.Setup,
		proposed.PlantedChapterID,
		proposed.DueChapterID,
		appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO foreshadowing_hooks(id, project_id, title, setup, planted_chapter_id, due_chapter_id, resolved_chapter_id, status, notes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, hook.ID, hook.ProjectID, hook.Title, hook.Setup, hook.PlantedChapterID,
			nullString(hook.DueChapterID), nullString(hook.ResolvedChapterID), hook.Status, hook.Notes,
			formatTime(hook.CreatedAt), formatTime(hook.UpdatedAt))
		if err != nil {
			return nil, databaseOperationError("apply create foreshadowing hook operation", err)
		}
		return nil, requireOneRow(result, "foreshadowing hook", hook.ID)
	}}, nil
}

// encodeEvidence serializes the operation's evidence slice as a JSON array,
// using the canonical "[]" form when empty so it round-trips through the
// change_operations.evidence CHECK constraint.
func encodeEvidence(evidence []revision.Evidence) (string, error) {
	if len(evidence) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// decodeEvidence parses the JSON array stored in change_operations.evidence.
// An empty string is treated as "[]" for backward compatibility with rows
// written before the evidence column existed.
func decodeEvidence(raw string) ([]revision.Evidence, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var evidence []revision.Evidence
	if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
		return nil, err
	}
	return evidence, nil
}

// storyLevelOrder reports the outline depth for a level and whether it is
// recognized. Mirrors the private map in the story domain package.
func storyLevelOrder(level storydomain.OutlineLevel) (int, bool) {
	switch level {
	case storydomain.LevelBook:
		return 0, true
	case storydomain.LevelVolume:
		return 1, true
	case storydomain.LevelArc:
		return 2, true
	case storydomain.LevelChapter:
		return 3, true
	case storydomain.LevelScene:
		return 4, true
	}
	return 0, false
}

func prepareCreateFactionOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed createFactionValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	faction, err := worlddomain.NewFaction(operation.TargetID, changeSet.ProjectID, proposed.Name, proposed.Summary, proposed.Goal, appliedAt)
	if err != nil {
		return preparedOperation{}, err
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO factions (id, project_id, name, summary, goal, status, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)
		`, faction.ID, faction.ProjectID, faction.Name, faction.Summary, faction.Goal, faction.Status,
			formatTime(faction.CreatedAt), formatTime(faction.UpdatedAt))
		if err != nil {
			return nil, databaseOperationError("apply create faction operation", err)
		}
		return nil, requireOneRow(result, "faction", faction.ID)
	}}, nil
}

func prepareCreateLocationOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed createLocationValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	region, err := ensureDefaultWorldRegion(ctx, tx, changeSet.ProjectID, strings.TrimSpace(proposed.RegionID), appliedAt)
	if err != nil {
		return preparedOperation{}, err
	}
	location, err := worlddomain.NewLocation(
		operation.TargetID, changeSet.ProjectID, region,
		worlddomain.LocationKind(proposed.Kind), proposed.Name, proposed.Summary, appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO locations(id, project_id, region_id, kind, name, summary, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, location.ID, location.ProjectID, location.RegionID, location.Kind, location.Name, location.Summary,
			location.Status, formatTime(location.CreatedAt), formatTime(location.UpdatedAt))
		if err != nil {
			return nil, databaseOperationError("apply create location operation", err)
		}
		return nil, requireOneRow(result, "location", location.ID)
	}}, nil
}

func prepareCreateAnchorOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed createAnchorValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	kind := storyassetdomain.KindHardAnchor
	switch strings.TrimSpace(proposed.Strength) {
	case "soft":
		kind = storyassetdomain.KindSoftAnchor
	case "forbidden":
		kind = storyassetdomain.KindForbidden
	case "hard":
		kind = storyassetdomain.KindHardAnchor
	default:
		return preparedOperation{}, storyassetdomain.ErrInvalidKind
	}
	origin := storyassetdomain.OriginModel
	if strings.TrimSpace(proposed.Origin) != "" {
		origin = storyassetdomain.Origin(proposed.Origin)
	}
	asset, err := storyassetdomain.New(
		operation.TargetID, changeSet.ProjectID, kind, origin,
		proposed.SourceCandidateID, proposed.SourceRunID, proposed.SourceArtifactID, "", "", appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	statement := strings.TrimSpace(proposed.Statement)
	scopeKind := strings.TrimSpace(proposed.ScopeKind)
	if statement == "" || scopeKind == "" {
		return preparedOperation{}, storyassetdomain.ErrBlankValue
	}
	payload, err := json.Marshal(map[string]string{"statement": statement})
	if err != nil {
		return preparedOperation{}, err
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		if err := insertStoryAsset(ctx, tx, asset, string(payload), appliedAt); err != nil {
			return nil, err
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO story_anchors (id, asset_id, project_id, strength, scope_kind, scope_id, statement, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, operation.TargetID, asset.ID, changeSet.ProjectID, proposed.Strength, scopeKind,
			strings.TrimSpace(proposed.ScopeID), statement, formatTime(appliedAt))
		if err != nil {
			return nil, databaseOperationError("apply create anchor operation", err)
		}
		return nil, requireOneRow(result, "story anchor", operation.TargetID)
	}}, nil
}

func prepareCreateStoryAssetOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	var proposed createStoryAssetValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	origin := storyassetdomain.Origin(proposed.Origin)
	if strings.TrimSpace(proposed.Origin) == "" {
		origin = storyassetdomain.OriginModel
	}
	asset, err := storyassetdomain.New(
		operation.TargetID, changeSet.ProjectID, storyassetdomain.Kind(proposed.Kind), origin,
		proposed.SourceCandidateID, proposed.SourceRunID, proposed.SourceArtifactID,
		proposed.TypedTable, proposed.TypedID, appliedAt,
	)
	if err != nil {
		return preparedOperation{}, err
	}
	payload := strings.TrimSpace(proposed.PayloadJSON)
	if payload == "" {
		payload = "{}"
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		return nil, insertStoryAsset(ctx, tx, asset, payload, appliedAt)
	}}, nil
}

func prepareUpdateProjectOperation(
	ctx context.Context,
	tx *sql.Tx,
	changeSet revision.ChangeSet,
	operation revision.PatchOperation,
	appliedAt time.Time,
) (preparedOperation, error) {
	current, err := scanProject(tx.QueryRowContext(ctx, `
		SELECT id, title, genre, summary, created_at, updated_at FROM projects WHERE id = ?
	`, changeSet.ProjectID))
	if err != nil || current.ID != changeSet.ProjectID {
		return preparedOperation{}, projectdomain.ErrRevisionConflict
	}
	if !matchesBaseline(operation, projectMetaValue{Title: current.Title, Genre: current.Genre, Summary: current.Summary}) {
		return preparedOperation{}, projectdomain.ErrRevisionConflict
	}
	var proposed projectMetaValue
	if err := decodeOperationValue(operation.AfterValue, &proposed); err != nil {
		return preparedOperation{}, err
	}
	title := strings.TrimSpace(proposed.Title)
	if title == "" {
		return preparedOperation{}, projectdomain.ErrBlankTitle
	}
	return preparedOperation{apply: func() (*projectdomain.Chapter, error) {
		result, err := tx.ExecContext(ctx, `
			UPDATE projects SET title = ?, genre = ?, summary = ?, updated_at = ? WHERE id = ?
		`, title, strings.TrimSpace(proposed.Genre), strings.TrimSpace(proposed.Summary),
			formatTime(appliedAt), changeSet.ProjectID)
		if err != nil {
			return nil, databaseOperationError("apply update project operation", err)
		}
		return nil, requireOneRow(result, "project", changeSet.ProjectID)
	}}, nil
}

func ensureDefaultWorldRegion(
	ctx context.Context,
	tx *sql.Tx,
	projectID, preferredID string,
	now time.Time,
) (worlddomain.WorldRegion, error) {
	var region worlddomain.WorldRegion
	var parent sql.NullString
	var createdAt, updatedAt string
	err := tx.QueryRowContext(ctx, `
		SELECT id, project_id, parent_id, kind, name, direction, summary, terrain, climate, created_at, updated_at
		FROM world_regions
		WHERE project_id = ? AND kind = ? AND deleted_at IS NULL
		ORDER BY created_at, id LIMIT 1
	`, projectID, worlddomain.RegionWorld).Scan(
		&region.ID, &region.ProjectID, &parent, &region.Kind, &region.Name, &region.Direction,
		&region.Summary, &region.Terrain, &region.Climate, &createdAt, &updatedAt,
	)
	if err == nil {
		region.ParentID = parent.String
		return region, nil
	}
	if err != sql.ErrNoRows {
		return worlddomain.WorldRegion{}, err
	}
	id := strings.TrimSpace(preferredID)
	if id == "" {
		id = "world-root"
	}
	created, err := worlddomain.NewWorldRegion(id, projectID, nil, worlddomain.RegionWorld, "世界", worlddomain.DirectionCenter, now)
	if err != nil {
		return worlddomain.WorldRegion{}, err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO world_regions(id, project_id, parent_id, kind, name, direction, summary, terrain, climate, created_at, updated_at)
		VALUES (?, ?, NULL, ?, ?, ?, ?, ?, ?, ?, ?)
	`, created.ID, created.ProjectID, created.Kind, created.Name, created.Direction, created.Summary,
		created.Terrain, created.Climate, formatTime(created.CreatedAt), formatTime(created.UpdatedAt))
	if err != nil {
		return worlddomain.WorldRegion{}, databaseOperationError("ensure default world region", err)
	}
	return created, nil
}

func insertStoryAsset(ctx context.Context, tx *sql.Tx, asset storyassetdomain.Asset, payload string, now time.Time) error {
	result, err := tx.ExecContext(ctx, `
		INSERT INTO story_assets (
			id, project_id, kind, status, current_revision, origin, source_candidate_id,
			source_run_id, source_artifact_id, typed_table, typed_id, locked, stale,
			stale_reason, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?)
	`,
		asset.ID, asset.ProjectID, string(asset.Kind), string(asset.Status), asset.CurrentRevision,
		string(asset.Origin), asset.SourceCandidateID, asset.SourceRunID, asset.SourceArtifactID,
		asset.TypedTable, asset.TypedID, boolToInt(asset.Locked), boolToInt(asset.Stale),
		asset.StaleReason, formatTime(asset.CreatedAt), formatTime(asset.UpdatedAt),
	)
	if err != nil {
		return databaseOperationError("apply create story asset operation", err)
	}
	if err := requireOneRow(result, "story asset", asset.ID); err != nil {
		return err
	}
	sum := sha256.Sum256([]byte(payload))
	_, err = tx.ExecContext(ctx, `
		INSERT INTO story_asset_revisions (asset_id, revision, payload_json, content_sha256, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, asset.ID, asset.CurrentRevision, payload, hex.EncodeToString(sum[:]), "creation-archive", formatTime(now))
	if err != nil {
		return databaseOperationError("apply story asset revision", err)
	}
	return nil
}
