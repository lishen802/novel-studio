package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
)

func (s *Store) CreateOutlineNode(ctx context.Context, node storydomain.OutlineNode) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO outline_nodes (
			id, project_id, parent_id, level, title, summary, goal, conflict,
			outcome, position, chapter_id, created_at, updated_at, deleted_at
		) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, NULL)
	`, node.ID, node.ProjectID, node.ParentID, node.Level, node.Title, node.Summary,
		node.Goal, node.Conflict, node.Outcome, node.Position, node.ChapterID,
		formatTime(node.CreatedAt), formatTime(node.UpdatedAt))
	if err != nil {
		return databaseOperationError("create outline node", err)
	}
	return nil
}

func (s *Store) ListOutlineNodes(ctx context.Context) ([]storydomain.OutlineNode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, COALESCE(parent_id, ''), level, title, summary,
			goal, conflict, outcome, position, COALESCE(chapter_id, ''),
			created_at, updated_at
		FROM outline_nodes
		WHERE deleted_at IS NULL
		ORDER BY CASE level
			WHEN 'book' THEN 0 WHEN 'volume' THEN 1 WHEN 'arc' THEN 2
			WHEN 'chapter' THEN 3 ELSE 4 END, position, created_at
	`)
	if err != nil {
		return nil, databaseOperationError("list outline nodes", err)
	}
	defer rows.Close()
	nodes := make([]storydomain.OutlineNode, 0)
	for rows.Next() {
		node, err := scanOutlineNode(rows)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate outline nodes", err)
	}
	return nodes, nil
}

func (s *Store) ArchiveOutlineNode(ctx context.Context, id string, archivedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin archive outline transaction", err)
	}
	defer tx.Rollback()
	var references int
	err = tx.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM outline_nodes WHERE parent_id = ? AND deleted_at IS NULL) +
			(SELECT COUNT(*) FROM story_line_links WHERE outline_node_id = ?) +
			(SELECT COUNT(*) FROM scene_cards WHERE outline_node_id = ? AND deleted_at IS NULL)
	`, id, id, id).Scan(&references)
	if err != nil {
		return databaseOperationError("count outline references", err)
	}
	if references > 0 {
		return storydomain.ErrReferenced
	}
	result, err := tx.ExecContext(ctx, `UPDATE outline_nodes SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive outline node", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return databaseOperationError("read archived outline count", err)
	}
	if changed != 1 {
		return fmt.Errorf("outline node does not exist: %s", id)
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit archive outline transaction", err)
	}
	return nil
}

func (s *Store) SwapOutlineNodePositions(ctx context.Context, firstID, secondID string, updatedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin outline reorder transaction", err)
	}
	defer tx.Rollback()
	type location struct {
		projectID string
		parentID  sql.NullString
		position  int
	}
	read := func(id string) (location, error) {
		var value location
		err := tx.QueryRowContext(ctx, `SELECT project_id, parent_id, position FROM outline_nodes WHERE id = ? AND deleted_at IS NULL`, id).Scan(&value.projectID, &value.parentID, &value.position)
		if err != nil {
			return location{}, err
		}
		return value, nil
	}
	first, err := read(firstID)
	if err != nil {
		return databaseOperationError("read first outline position", err)
	}
	second, err := read(secondID)
	if err != nil {
		return databaseOperationError("read second outline position", err)
	}
	if first.projectID != second.projectID || first.parentID.Valid != second.parentID.Valid || first.parentID.String != second.parentID.String {
		return storydomain.ErrInvalidReference
	}
	stamp := formatTime(updatedAt)
	if _, err := tx.ExecContext(ctx, `UPDATE outline_nodes SET position = 2147483647, updated_at = ? WHERE id = ?`, stamp, firstID); err != nil {
		return databaseOperationError("reserve outline position", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE outline_nodes SET position = ?, updated_at = ? WHERE id = ?`, first.position, stamp, secondID); err != nil {
		return databaseOperationError("move second outline node", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE outline_nodes SET position = ?, updated_at = ? WHERE id = ?`, second.position, stamp, firstID); err != nil {
		return databaseOperationError("move first outline node", err)
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit outline reorder transaction", err)
	}
	return nil
}

func (s *Store) CreateStoryLine(ctx context.Context, line storydomain.StoryLine) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO story_lines (id, project_id, name, summary, status, created_at, updated_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
	`, line.ID, line.ProjectID, line.Name, line.Summary, line.Status, formatTime(line.CreatedAt), formatTime(line.UpdatedAt))
	if err != nil {
		return databaseOperationError("create story line", err)
	}
	return nil
}

func (s *Store) ListStoryLines(ctx context.Context) ([]storydomain.StoryLine, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, name, summary, status, created_at, updated_at
		FROM story_lines WHERE deleted_at IS NULL ORDER BY created_at, id
	`)
	if err != nil {
		return nil, databaseOperationError("list story lines", err)
	}
	defer rows.Close()
	lines := make([]storydomain.StoryLine, 0)
	for rows.Next() {
		var line storydomain.StoryLine
		var createdAt, updatedAt string
		if err := rows.Scan(&line.ID, &line.ProjectID, &line.Name, &line.Summary, &line.Status, &createdAt, &updatedAt); err != nil {
			return nil, databaseOperationError("scan story line", err)
		}
		line.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse story line created_at: %w", err)
		}
		line.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse story line updated_at: %w", err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate story lines", err)
	}
	return lines, nil
}

func (s *Store) LinkStoryLine(ctx context.Context, projectID string, link storydomain.StoryLineLink) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO story_line_links (project_id, story_line_id, outline_node_id, position, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, projectID, link.StoryLineID, link.OutlineNodeID, link.Position, formatTime(link.CreatedAt))
	if err != nil {
		return databaseOperationError("link story line", err)
	}
	return nil
}

func (s *Store) ListStoryLineLinks(ctx context.Context) ([]storydomain.StoryLineLink, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT story_line_id, outline_node_id, position, created_at
		FROM story_line_links ORDER BY story_line_id, position
	`)
	if err != nil {
		return nil, databaseOperationError("list story line links", err)
	}
	defer rows.Close()
	links := make([]storydomain.StoryLineLink, 0)
	for rows.Next() {
		var link storydomain.StoryLineLink
		var createdAt string
		if err := rows.Scan(&link.StoryLineID, &link.OutlineNodeID, &link.Position, &createdAt); err != nil {
			return nil, databaseOperationError("scan story line link", err)
		}
		link.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse story line link created_at: %w", err)
		}
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate story line links", err)
	}
	return links, nil
}

func (s *Store) CreateSceneCard(ctx context.Context, card storydomain.SceneCard) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scene_cards (
			id, project_id, chapter_id, outline_node_id, title, pov, location,
			goal, conflict, outcome, position, status, created_at, updated_at, deleted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL)
	`, card.ID, card.ProjectID, card.ChapterID, card.OutlineNodeID, card.Title, card.POV,
		card.Location, card.Goal, card.Conflict, card.Outcome, card.Position, card.Status,
		formatTime(card.CreatedAt), formatTime(card.UpdatedAt))
	if err != nil {
		return databaseOperationError("create scene card", err)
	}
	return nil
}

func (s *Store) ListSceneCards(ctx context.Context, chapterID string) ([]storydomain.SceneCard, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, chapter_id, outline_node_id, title, pov, location,
			goal, conflict, outcome, position, status, created_at, updated_at
		FROM scene_cards
		WHERE chapter_id = ? AND deleted_at IS NULL
		ORDER BY position
	`, chapterID)
	if err != nil {
		return nil, databaseOperationError("list scene cards", err)
	}
	defer rows.Close()
	cards := make([]storydomain.SceneCard, 0)
	for rows.Next() {
		var card storydomain.SceneCard
		var createdAt, updatedAt string
		if err := rows.Scan(&card.ID, &card.ProjectID, &card.ChapterID, &card.OutlineNodeID,
			&card.Title, &card.POV, &card.Location, &card.Goal, &card.Conflict,
			&card.Outcome, &card.Position, &card.Status, &createdAt, &updatedAt); err != nil {
			return nil, databaseOperationError("scan scene card", err)
		}
		card.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse scene card created_at: %w", err)
		}
		card.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return nil, fmt.Errorf("parse scene card updated_at: %w", err)
		}
		cards = append(cards, card)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate scene cards", err)
	}
	return cards, nil
}

func scanOutlineNode(row scanner) (storydomain.OutlineNode, error) {
	var node storydomain.OutlineNode
	var createdAt, updatedAt string
	err := row.Scan(&node.ID, &node.ProjectID, &node.ParentID, &node.Level, &node.Title,
		&node.Summary, &node.Goal, &node.Conflict, &node.Outcome, &node.Position,
		&node.ChapterID, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return storydomain.OutlineNode{}, err
	}
	if err != nil {
		return storydomain.OutlineNode{}, databaseOperationError("scan outline node", err)
	}
	node.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return storydomain.OutlineNode{}, fmt.Errorf("parse outline created_at: %w", err)
	}
	node.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return storydomain.OutlineNode{}, fmt.Errorf("parse outline updated_at: %w", err)
	}
	return node, nil
}
