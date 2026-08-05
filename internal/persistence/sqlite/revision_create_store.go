package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	"github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
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
