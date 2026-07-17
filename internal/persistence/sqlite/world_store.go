package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

func (s *Store) CreateCharacter(ctx context.Context, character worlddomain.Character) error {
	aliases, err := json.Marshal(character.Aliases)
	if err != nil {
		return fmt.Errorf("encode character aliases: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO characters (
			id, project_id, name, aliases_json, role, summary, motivation, fear,
			secret, arc, status, created_at, updated_at, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
	`, character.ID, character.ProjectID, character.Name, string(aliases), character.Role,
		character.Summary, character.Motivation, character.Fear, character.Secret,
		character.Arc, character.Status, formatTime(character.CreatedAt), formatTime(character.UpdatedAt))
	if err != nil {
		return databaseOperationError("create character", err)
	}
	return nil
}

func (s *Store) UpdateCharacter(ctx context.Context, character worlddomain.Character) error {
	aliases, err := json.Marshal(character.Aliases)
	if err != nil {
		return fmt.Errorf("encode character aliases: %w", err)
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE characters SET name = ?, aliases_json = ?, role = ?, summary = ?,
			motivation = ?, fear = ?, secret = ?, arc = ?, status = ?, updated_at = ?
		WHERE id = ? AND project_id = ? AND deleted_at IS NULL
	`, character.Name, string(aliases), character.Role, character.Summary, character.Motivation,
		character.Fear, character.Secret, character.Arc, character.Status, formatTime(character.UpdatedAt),
		character.ID, character.ProjectID)
	if err != nil {
		return databaseOperationError("update character", err)
	}
	return requireOneRow(result, "character", character.ID)
}

func (s *Store) ListCharacters(ctx context.Context) ([]worlddomain.Character, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, name, aliases_json, role, summary, motivation, fear,
			secret, arc, status, created_at, updated_at
		FROM characters WHERE deleted_at IS NULL ORDER BY created_at, id
	`)
	if err != nil {
		return nil, databaseOperationError("list characters", err)
	}
	defer rows.Close()
	characters := make([]worlddomain.Character, 0)
	for rows.Next() {
		character, err := scanCharacter(rows)
		if err != nil {
			return nil, err
		}
		characters = append(characters, character)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate characters", err)
	}
	return characters, nil
}

func (s *Store) Character(ctx context.Context, id string) (worlddomain.Character, error) {
	return scanCharacter(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, aliases_json, role, summary, motivation, fear,
			secret, arc, status, created_at, updated_at
		FROM characters WHERE id = ? AND deleted_at IS NULL
	`, id))
}

func (s *Store) ArchiveCharacter(ctx context.Context, id string, archivedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin character archive transaction", err)
	}
	defer tx.Rollback()
	var references int
	err = tx.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM faction_memberships WHERE character_id = ?) +
			(SELECT COUNT(*) FROM character_relationships WHERE (source_character_id = ? OR target_character_id = ?) AND deleted_at IS NULL) +
			(SELECT COUNT(*) FROM chapter_characters WHERE character_id = ?) +
			(SELECT COUNT(*) FROM scene_card_characters WHERE character_id = ?)
	`, id, id, id, id, id).Scan(&references)
	if err != nil {
		return databaseOperationError("count character references", err)
	}
	continuityReferences, err := countContinuityTargetReferences(ctx, tx, "character", id)
	if err != nil {
		return err
	}
	references += continuityReferences
	if references > 0 {
		return worlddomain.ErrReferenced
	}
	result, err := tx.ExecContext(ctx, `UPDATE characters SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive character", err)
	}
	if err := requireOneRow(result, "character", id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit character archive transaction", err)
	}
	return nil
}

func (s *Store) CreateFaction(ctx context.Context, faction worlddomain.Faction) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO factions (id, project_id, name, summary, goal, status, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL)
	`, faction.ID, faction.ProjectID, faction.Name, faction.Summary, faction.Goal, faction.Status, formatTime(faction.CreatedAt), formatTime(faction.UpdatedAt))
	if err != nil {
		return databaseOperationError("create faction", err)
	}
	return nil
}

func (s *Store) UpdateFaction(ctx context.Context, faction worlddomain.Faction) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE factions SET name = ?, summary = ?, goal = ?, status = ?, updated_at = ?
		WHERE id = ? AND project_id = ? AND deleted_at IS NULL
	`, faction.Name, faction.Summary, faction.Goal, faction.Status, formatTime(faction.UpdatedAt), faction.ID, faction.ProjectID)
	if err != nil {
		return databaseOperationError("update faction", err)
	}
	return requireOneRow(result, "faction", faction.ID)
}

func (s *Store) ListFactions(ctx context.Context) ([]worlddomain.Faction, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, name, summary, goal, status, created_at, updated_at
		FROM factions WHERE deleted_at IS NULL ORDER BY created_at, id
	`)
	if err != nil {
		return nil, databaseOperationError("list factions", err)
	}
	defer rows.Close()
	factions := make([]worlddomain.Faction, 0)
	for rows.Next() {
		faction, err := scanFaction(rows)
		if err != nil {
			return nil, err
		}
		factions = append(factions, faction)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate factions", err)
	}
	return factions, nil
}

func (s *Store) Faction(ctx context.Context, id string) (worlddomain.Faction, error) {
	return scanFaction(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, summary, goal, status, created_at, updated_at
		FROM factions WHERE id = ? AND deleted_at IS NULL
	`, id))
}

func (s *Store) ArchiveFaction(ctx context.Context, id string, archivedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin faction archive transaction", err)
	}
	defer tx.Rollback()
	var references int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM faction_memberships WHERE faction_id = ?`, id).Scan(&references); err != nil {
		return databaseOperationError("count faction references", err)
	}
	continuityReferences, err := countContinuityTargetReferences(ctx, tx, "faction", id)
	if err != nil {
		return err
	}
	references += continuityReferences
	if references > 0 {
		return worlddomain.ErrReferenced
	}
	result, err := tx.ExecContext(ctx, `UPDATE factions SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive faction", err)
	}
	if err := requireOneRow(result, "faction", id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit faction archive transaction", err)
	}
	return nil
}

func (s *Store) UpsertMembership(ctx context.Context, membership worlddomain.Membership) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO faction_memberships (project_id, character_id, faction_id, role, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(character_id, faction_id) DO UPDATE SET
			role = excluded.role, status = excluded.status, updated_at = excluded.updated_at
	`, membership.ProjectID, membership.CharacterID, membership.FactionID, membership.Role,
		membership.Status, formatTime(membership.CreatedAt), formatTime(membership.UpdatedAt))
	if err != nil {
		return databaseOperationError("upsert faction membership", err)
	}
	return nil
}

func (s *Store) ListMemberships(ctx context.Context) ([]worlddomain.Membership, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT project_id, character_id, faction_id, role, status, created_at, updated_at
		FROM faction_memberships ORDER BY faction_id, character_id
	`)
	if err != nil {
		return nil, databaseOperationError("list faction memberships", err)
	}
	defer rows.Close()
	memberships := make([]worlddomain.Membership, 0)
	for rows.Next() {
		var membership worlddomain.Membership
		var createdAt, updatedAt string
		if err := rows.Scan(&membership.ProjectID, &membership.CharacterID, &membership.FactionID, &membership.Role, &membership.Status, &createdAt, &updatedAt); err != nil {
			return nil, databaseOperationError("scan faction membership", err)
		}
		membership.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse membership created_at: %w", err)
		}
		membership.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse membership updated_at: %w", err)
		}
		memberships = append(memberships, membership)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate faction memberships", err)
	}
	return memberships, nil
}

func (s *Store) DeleteMembership(ctx context.Context, characterID, factionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM faction_memberships WHERE character_id = ? AND faction_id = ?`, characterID, factionID)
	if err != nil {
		return databaseOperationError("delete faction membership", err)
	}
	return nil
}

func (s *Store) CreateRelationship(ctx context.Context, relationship worlddomain.Relationship) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO character_relationships (
			id, project_id, source_character_id, target_character_id, kind, label,
			strength, public_state, true_state, created_at, updated_at, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
	`, relationship.ID, relationship.ProjectID, relationship.SourceCharacterID, relationship.TargetCharacterID,
		relationship.Kind, relationship.Label, relationship.Strength, relationship.PublicState,
		relationship.TrueState, formatTime(relationship.CreatedAt), formatTime(relationship.UpdatedAt))
	if err != nil {
		return databaseOperationError("create character relationship", err)
	}
	return nil
}

func (s *Store) ListRelationships(ctx context.Context) ([]worlddomain.Relationship, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, source_character_id, target_character_id, kind, label,
			strength, public_state, true_state, created_at, updated_at
		FROM character_relationships WHERE deleted_at IS NULL ORDER BY created_at, id
	`)
	if err != nil {
		return nil, databaseOperationError("list character relationships", err)
	}
	defer rows.Close()
	relationships := make([]worlddomain.Relationship, 0)
	for rows.Next() {
		var relationship worlddomain.Relationship
		var createdAt, updatedAt string
		if err := rows.Scan(&relationship.ID, &relationship.ProjectID, &relationship.SourceCharacterID,
			&relationship.TargetCharacterID, &relationship.Kind, &relationship.Label, &relationship.Strength,
			&relationship.PublicState, &relationship.TrueState, &createdAt, &updatedAt); err != nil {
			return nil, databaseOperationError("scan character relationship", err)
		}
		relationship.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse relationship created_at: %w", err)
		}
		relationship.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse relationship updated_at: %w", err)
		}
		relationships = append(relationships, relationship)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate character relationships", err)
	}
	return relationships, nil
}

func (s *Store) ArchiveRelationship(ctx context.Context, id string, archivedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE character_relationships SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive character relationship", err)
	}
	return requireOneRow(result, "relationship", id)
}

func (s *Store) AddChapterAppearance(ctx context.Context, appearance worlddomain.ChapterAppearance) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO chapter_characters (project_id, chapter_id, character_id, role, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(chapter_id, character_id) DO UPDATE SET role = excluded.role
	`, appearance.ProjectID, appearance.ChapterID, appearance.CharacterID, appearance.Role, formatTime(appearance.CreatedAt))
	if err != nil {
		return databaseOperationError("add chapter character", err)
	}
	return nil
}

func (s *Store) ListChapterAppearances(ctx context.Context) ([]worlddomain.ChapterAppearance, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, character_id, chapter_id, role, created_at FROM chapter_characters ORDER BY chapter_id, character_id`)
	if err != nil {
		return nil, databaseOperationError("list chapter characters", err)
	}
	defer rows.Close()
	appearances := make([]worlddomain.ChapterAppearance, 0)
	for rows.Next() {
		var appearance worlddomain.ChapterAppearance
		var createdAt string
		if err := rows.Scan(&appearance.ProjectID, &appearance.CharacterID, &appearance.ChapterID, &appearance.Role, &createdAt); err != nil {
			return nil, databaseOperationError("scan chapter character", err)
		}
		appearance.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse chapter character created_at: %w", err)
		}
		appearances = append(appearances, appearance)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate chapter characters", err)
	}
	return appearances, nil
}

func (s *Store) RemoveChapterAppearance(ctx context.Context, characterID, chapterID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chapter_characters WHERE character_id = ? AND chapter_id = ?`, characterID, chapterID)
	if err != nil {
		return databaseOperationError("remove chapter character", err)
	}
	return nil
}

func (s *Store) AddSceneAppearance(ctx context.Context, appearance worlddomain.SceneAppearance) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scene_card_characters (project_id, scene_card_id, character_id, role, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(scene_card_id, character_id) DO UPDATE SET role = excluded.role
	`, appearance.ProjectID, appearance.SceneCardID, appearance.CharacterID, appearance.Role, formatTime(appearance.CreatedAt))
	if err != nil {
		return databaseOperationError("add scene character", err)
	}
	return nil
}

func (s *Store) ListSceneAppearances(ctx context.Context) ([]worlddomain.SceneAppearance, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, character_id, scene_card_id, role, created_at FROM scene_card_characters ORDER BY scene_card_id, character_id`)
	if err != nil {
		return nil, databaseOperationError("list scene characters", err)
	}
	defer rows.Close()
	appearances := make([]worlddomain.SceneAppearance, 0)
	for rows.Next() {
		var appearance worlddomain.SceneAppearance
		var createdAt string
		if err := rows.Scan(&appearance.ProjectID, &appearance.CharacterID, &appearance.SceneCardID, &appearance.Role, &createdAt); err != nil {
			return nil, databaseOperationError("scan scene character", err)
		}
		appearance.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse scene character created_at: %w", err)
		}
		appearances = append(appearances, appearance)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate scene characters", err)
	}
	return appearances, nil
}

func (s *Store) RemoveSceneAppearance(ctx context.Context, characterID, sceneCardID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scene_card_characters WHERE character_id = ? AND scene_card_id = ?`, characterID, sceneCardID)
	if err != nil {
		return databaseOperationError("remove scene character", err)
	}
	return nil
}

func scanCharacter(row scanner) (worlddomain.Character, error) {
	var character worlddomain.Character
	var aliasesJSON, createdAt, updatedAt string
	err := row.Scan(&character.ID, &character.ProjectID, &character.Name, &aliasesJSON, &character.Role,
		&character.Summary, &character.Motivation, &character.Fear, &character.Secret, &character.Arc,
		&character.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return worlddomain.Character{}, err
	}
	if err != nil {
		return worlddomain.Character{}, databaseOperationError("scan character", err)
	}
	if err := json.Unmarshal([]byte(aliasesJSON), &character.Aliases); err != nil {
		return worlddomain.Character{}, fmt.Errorf("decode character aliases: %w", err)
	}
	character.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return worlddomain.Character{}, fmt.Errorf("parse character created_at: %w", err)
	}
	character.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return worlddomain.Character{}, fmt.Errorf("parse character updated_at: %w", err)
	}
	return character, nil
}

func scanFaction(row scanner) (worlddomain.Faction, error) {
	var faction worlddomain.Faction
	var createdAt, updatedAt string
	err := row.Scan(&faction.ID, &faction.ProjectID, &faction.Name, &faction.Summary, &faction.Goal, &faction.Status, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return worlddomain.Faction{}, err
	}
	if err != nil {
		return worlddomain.Faction{}, databaseOperationError("scan faction", err)
	}
	faction.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return worlddomain.Faction{}, fmt.Errorf("parse faction created_at: %w", err)
	}
	faction.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return worlddomain.Faction{}, fmt.Errorf("parse faction updated_at: %w", err)
	}
	return faction, nil
}

func requireOneRow(result sql.Result, entity, id string) error {
	changed, err := result.RowsAffected()
	if err != nil {
		return databaseOperationError("read affected row count", err)
	}
	if changed != 1 {
		return fmt.Errorf("%s does not exist: %s", entity, id)
	}
	return nil
}
