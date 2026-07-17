package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
)

func (s *Store) CreateStoryEvent(ctx context.Context, event timelinedomain.StoryEvent) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO story_events(id, project_id, title, summary, cause, result, story_time, story_order, narrative_order, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.ID, event.ProjectID, event.Title, event.Summary, event.Cause, event.Result, event.StoryTime, event.StoryOrder, event.NarrativeOrder, event.Status, formatTime(event.CreatedAt), formatTime(event.UpdatedAt))
	if err != nil {
		return databaseOperationError("create story event", err)
	}
	return nil
}

func (s *Store) StoryEvent(ctx context.Context, id string) (timelinedomain.StoryEvent, error) {
	return scanStoryEvent(s.db.QueryRowContext(ctx, `SELECT id, project_id, title, summary, cause, result, story_time, story_order, narrative_order, status, created_at, updated_at FROM story_events WHERE id = ? AND deleted_at IS NULL`, id))
}

func (s *Store) ListStoryEvents(ctx context.Context) ([]timelinedomain.StoryEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, title, summary, cause, result, story_time, story_order, narrative_order, status, created_at, updated_at FROM story_events WHERE deleted_at IS NULL ORDER BY story_order, narrative_order, created_at`)
	if err != nil {
		return nil, databaseOperationError("list story events", err)
	}
	defer rows.Close()
	result := make([]timelinedomain.StoryEvent, 0)
	for rows.Next() {
		event, err := scanStoryEvent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func (s *Store) UpdateStoryEvent(ctx context.Context, event timelinedomain.StoryEvent) error {
	result, err := s.db.ExecContext(ctx, `UPDATE story_events SET title = ?, summary = ?, cause = ?, result = ?, story_time = ?, story_order = ?, narrative_order = ?, status = ?, updated_at = ? WHERE id = ? AND project_id = ? AND deleted_at IS NULL`, event.Title, event.Summary, event.Cause, event.Result, event.StoryTime, event.StoryOrder, event.NarrativeOrder, event.Status, formatTime(event.UpdatedAt), event.ID, event.ProjectID)
	if err != nil {
		return databaseOperationError("update story event", err)
	}
	return requireOneRow(result, "story event", event.ID)
}

func (s *Store) ArchiveStoryEvent(ctx context.Context, id string, archivedAt time.Time) error {
	var references int
	err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM canon_facts WHERE deleted_at IS NULL AND (valid_from_event_id = ? OR valid_until_event_id = ?)) + (SELECT COUNT(*) FROM story_event_links WHERE event_id = ?)`, id, id, id).Scan(&references)
	if err != nil {
		return databaseOperationError("check story event references", err)
	}
	if references > 0 {
		return timelinedomain.ErrReferenced
	}
	result, err := s.db.ExecContext(ctx, `UPDATE story_events SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive story event", err)
	}
	return requireOneRow(result, "story event", id)
}

func scanStoryEvent(scanner interface{ Scan(...any) error }) (timelinedomain.StoryEvent, error) {
	var event timelinedomain.StoryEvent
	var createdAt, updatedAt string
	if err := scanner.Scan(&event.ID, &event.ProjectID, &event.Title, &event.Summary, &event.Cause, &event.Result, &event.StoryTime, &event.StoryOrder, &event.NarrativeOrder, &event.Status, &createdAt, &updatedAt); err != nil {
		return timelinedomain.StoryEvent{}, databaseOperationError("scan story event", err)
	}
	var err error
	event.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return timelinedomain.StoryEvent{}, fmt.Errorf("parse story event created_at: %w", err)
	}
	event.UpdatedAt, err = parseTime(updatedAt)
	return event, err
}

func (s *Store) UpsertStoryEventLink(ctx context.Context, link timelinedomain.EventLink) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO story_event_links(project_id, event_id, target_type, target_id, role, created_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(event_id, target_type, target_id) DO UPDATE SET role = excluded.role`, link.ProjectID, link.EventID, link.TargetType, link.TargetID, link.Role, formatTime(link.CreatedAt))
	if err != nil {
		return databaseOperationError("upsert story event link", err)
	}
	return nil
}

func (s *Store) ListStoryEventLinks(ctx context.Context) ([]timelinedomain.EventLink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, event_id, target_type, target_id, role, created_at FROM story_event_links ORDER BY created_at`)
	if err != nil {
		return nil, databaseOperationError("list story event links", err)
	}
	defer rows.Close()
	result := make([]timelinedomain.EventLink, 0)
	for rows.Next() {
		var link timelinedomain.EventLink
		var createdAt string
		if err := rows.Scan(&link.ProjectID, &link.EventID, &link.TargetType, &link.TargetID, &link.Role, &createdAt); err != nil {
			return nil, databaseOperationError("scan story event link", err)
		}
		link.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, link)
	}
	return result, rows.Err()
}

func (s *Store) DeleteStoryEventLink(ctx context.Context, eventID string, targetType timelinedomain.EventTargetType, targetID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM story_event_links WHERE event_id = ? AND target_type = ? AND target_id = ?`, eventID, targetType, targetID)
	if err != nil {
		return databaseOperationError("delete story event link", err)
	}
	return nil
}

func (s *Store) CreateFact(ctx context.Context, fact canondomain.Fact) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO canon_facts(id, project_id, fact_text, category, origin, status, confidence, valid_from_event_id, valid_until_event_id, created_by, confirmed_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, fact.ID, fact.ProjectID, fact.Text, fact.Category, fact.Origin, fact.Status, fact.Confidence, nullString(fact.ValidFromEventID), nullString(fact.ValidUntilEventID), fact.CreatedBy, nullString(fact.ConfirmedBy), formatTime(fact.CreatedAt), formatTime(fact.UpdatedAt))
	if err != nil {
		return databaseOperationError("create canon fact", err)
	}
	return nil
}

func (s *Store) Fact(ctx context.Context, id string) (canondomain.Fact, error) {
	return scanFact(s.db.QueryRowContext(ctx, `SELECT id, project_id, fact_text, category, origin, status, confidence, valid_from_event_id, valid_until_event_id, created_by, confirmed_by, created_at, updated_at FROM canon_facts WHERE id = ? AND deleted_at IS NULL`, id))
}

func (s *Store) ListFacts(ctx context.Context) ([]canondomain.Fact, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, fact_text, category, origin, status, confidence, valid_from_event_id, valid_until_event_id, created_by, confirmed_by, created_at, updated_at FROM canon_facts WHERE deleted_at IS NULL ORDER BY updated_at DESC`)
	if err != nil {
		return nil, databaseOperationError("list canon facts", err)
	}
	defer rows.Close()
	result := make([]canondomain.Fact, 0)
	for rows.Next() {
		fact, err := scanFact(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, fact)
	}
	return result, rows.Err()
}

func (s *Store) UpdateFact(ctx context.Context, fact canondomain.Fact) error {
	result, err := s.db.ExecContext(ctx, `UPDATE canon_facts SET fact_text = ?, category = ?, status = ?, confidence = ?, valid_from_event_id = ?, valid_until_event_id = ?, confirmed_by = ?, updated_at = ? WHERE id = ? AND project_id = ? AND deleted_at IS NULL`, fact.Text, fact.Category, fact.Status, fact.Confidence, nullString(fact.ValidFromEventID), nullString(fact.ValidUntilEventID), nullString(fact.ConfirmedBy), formatTime(fact.UpdatedAt), fact.ID, fact.ProjectID)
	if err != nil {
		return databaseOperationError("update canon fact", err)
	}
	return requireOneRow(result, "canon fact", fact.ID)
}

func (s *Store) ArchiveFact(ctx context.Context, id string, archivedAt time.Time) error {
	var references int
	err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM fact_evidence WHERE fact_id = ?) + (SELECT COUNT(*) FROM fact_scopes WHERE fact_id = ?) + (SELECT COUNT(*) FROM fact_conflicts WHERE fact_id = ? OR other_fact_id = ?)`, id, id, id, id).Scan(&references)
	if err != nil {
		return databaseOperationError("check canon fact references", err)
	}
	if references > 0 {
		return canondomain.ErrReferenced
	}
	result, err := s.db.ExecContext(ctx, `UPDATE canon_facts SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive canon fact", err)
	}
	return requireOneRow(result, "canon fact", id)
}

func scanFact(scanner interface{ Scan(...any) error }) (canondomain.Fact, error) {
	var fact canondomain.Fact
	var from, until, confirmed sql.NullString
	var createdAt, updatedAt string
	if err := scanner.Scan(&fact.ID, &fact.ProjectID, &fact.Text, &fact.Category, &fact.Origin, &fact.Status, &fact.Confidence, &from, &until, &fact.CreatedBy, &confirmed, &createdAt, &updatedAt); err != nil {
		return canondomain.Fact{}, databaseOperationError("scan canon fact", err)
	}
	fact.ValidFromEventID, fact.ValidUntilEventID, fact.ConfirmedBy = from.String, until.String, confirmed.String
	var err error
	fact.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return canondomain.Fact{}, err
	}
	fact.UpdatedAt, err = parseTime(updatedAt)
	return fact, err
}

func (s *Store) CreateFactEvidence(ctx context.Context, evidence canondomain.Evidence) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO fact_evidence(id, project_id, fact_id, chapter_id, quote, start_offset, end_offset, chapter_revision, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, evidence.ID, evidence.ProjectID, evidence.FactID, evidence.ChapterID, evidence.Quote, evidence.StartOffset, evidence.EndOffset, evidence.ChapterRevision, formatTime(evidence.CreatedAt))
	if err != nil {
		return databaseOperationError("create fact evidence", err)
	}
	return nil
}

func (s *Store) ListFactEvidence(ctx context.Context) ([]canondomain.Evidence, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, fact_id, chapter_id, quote, start_offset, end_offset, chapter_revision, created_at FROM fact_evidence ORDER BY created_at`)
	if err != nil {
		return nil, databaseOperationError("list fact evidence", err)
	}
	defer rows.Close()
	result := make([]canondomain.Evidence, 0)
	for rows.Next() {
		var value canondomain.Evidence
		var createdAt string
		if err := rows.Scan(&value.ID, &value.ProjectID, &value.FactID, &value.ChapterID, &value.Quote, &value.StartOffset, &value.EndOffset, &value.ChapterRevision, &createdAt); err != nil {
			return nil, databaseOperationError("scan fact evidence", err)
		}
		value.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) DeleteFactEvidence(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM fact_evidence WHERE id = ?`, id)
	if err != nil {
		return databaseOperationError("delete fact evidence", err)
	}
	return nil
}

func (s *Store) CreateFactScope(ctx context.Context, scope canondomain.Scope) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO fact_scopes(id, project_id, fact_id, target_type, target_id, created_at) VALUES (?, ?, ?, ?, ?, ?)`, scope.ID, scope.ProjectID, scope.FactID, scope.TargetType, scope.TargetID, formatTime(scope.CreatedAt))
	if err != nil {
		return databaseOperationError("create fact scope", err)
	}
	return nil
}

func (s *Store) ListFactScopes(ctx context.Context) ([]canondomain.Scope, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, fact_id, target_type, target_id, created_at FROM fact_scopes ORDER BY created_at`)
	if err != nil {
		return nil, databaseOperationError("list fact scopes", err)
	}
	defer rows.Close()
	result := make([]canondomain.Scope, 0)
	for rows.Next() {
		var value canondomain.Scope
		var createdAt string
		if err := rows.Scan(&value.ID, &value.ProjectID, &value.FactID, &value.TargetType, &value.TargetID, &createdAt); err != nil {
			return nil, databaseOperationError("scan fact scope", err)
		}
		value.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) DeleteFactScope(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM fact_scopes WHERE id = ?`, id)
	if err != nil {
		return databaseOperationError("delete fact scope", err)
	}
	return nil
}

func (s *Store) CreateFactConflict(ctx context.Context, conflict canondomain.Conflict) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO fact_conflicts(id, project_id, fact_id, other_fact_id, reason, status, created_at, resolved_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, conflict.ID, conflict.ProjectID, conflict.FactID, conflict.OtherFactID, conflict.Reason, conflict.Status, formatTime(conflict.CreatedAt), nullTime(conflict.ResolvedAt))
	if err != nil {
		return databaseOperationError("create fact conflict", err)
	}
	return nil
}

func (s *Store) UpdateFactConflict(ctx context.Context, conflict canondomain.Conflict) error {
	result, err := s.db.ExecContext(ctx, `UPDATE fact_conflicts SET reason = ?, status = ?, resolved_at = ? WHERE id = ? AND project_id = ?`, conflict.Reason, conflict.Status, nullTime(conflict.ResolvedAt), conflict.ID, conflict.ProjectID)
	if err != nil {
		return databaseOperationError("update fact conflict", err)
	}
	return requireOneRow(result, "fact conflict", conflict.ID)
}

func (s *Store) ListFactConflicts(ctx context.Context) ([]canondomain.Conflict, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, fact_id, other_fact_id, reason, status, created_at, resolved_at FROM fact_conflicts ORDER BY created_at`)
	if err != nil {
		return nil, databaseOperationError("list fact conflicts", err)
	}
	defer rows.Close()
	result := make([]canondomain.Conflict, 0)
	for rows.Next() {
		var value canondomain.Conflict
		var createdAt string
		var resolvedAt sql.NullString
		if err := rows.Scan(&value.ID, &value.ProjectID, &value.FactID, &value.OtherFactID, &value.Reason, &value.Status, &createdAt, &resolvedAt); err != nil {
			return nil, databaseOperationError("scan fact conflict", err)
		}
		value.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		if resolvedAt.Valid {
			parsed, err := parseTime(resolvedAt.String)
			if err != nil {
				return nil, err
			}
			value.ResolvedAt = &parsed
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *Store) DeleteFactConflict(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM fact_conflicts WHERE id = ?`, id)
	if err != nil {
		return databaseOperationError("delete fact conflict", err)
	}
	return nil
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatTime(*value)
}
