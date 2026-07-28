package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

func (s *Store) CreateWorldRegion(ctx context.Context, region worlddomain.WorldRegion) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO world_regions(id, project_id, parent_id, kind, name, direction, summary, terrain, climate, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, region.ID, region.ProjectID, nullString(region.ParentID), region.Kind, region.Name, region.Direction,
		region.Summary, region.Terrain, region.Climate, formatTime(region.CreatedAt), formatTime(region.UpdatedAt))
	if err != nil {
		return databaseOperationError("create world region", err)
	}
	return nil
}

func (s *Store) ListWorldRegions(ctx context.Context) ([]worlddomain.WorldRegion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, parent_id, kind, name, direction, summary, terrain, climate, created_at, updated_at
		FROM world_regions WHERE deleted_at IS NULL
		ORDER BY CASE WHEN parent_id IS NULL THEN 0 ELSE 1 END, name
	`)
	if err != nil {
		return nil, databaseOperationError("list world regions", err)
	}
	defer rows.Close()
	regions := make([]worlddomain.WorldRegion, 0)
	for rows.Next() {
		region, err := scanWorldRegion(rows)
		if err != nil {
			return nil, err
		}
		regions = append(regions, region)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate world regions", err)
	}
	return regions, nil
}

func (s *Store) WorldRegion(ctx context.Context, id string) (worlddomain.WorldRegion, error) {
	return scanWorldRegion(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, parent_id, kind, name, direction, summary, terrain, climate, created_at, updated_at
		FROM world_regions WHERE id = ? AND deleted_at IS NULL
	`, id))
}

func (s *Store) UpdateWorldRegion(ctx context.Context, region worlddomain.WorldRegion) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE world_regions SET parent_id = ?, kind = ?, name = ?, direction = ?, summary = ?, terrain = ?, climate = ?, updated_at = ?
		WHERE id = ? AND project_id = ? AND deleted_at IS NULL
	`, nullString(region.ParentID), region.Kind, region.Name, region.Direction, region.Summary, region.Terrain, region.Climate,
		formatTime(region.UpdatedAt), region.ID, region.ProjectID)
	if err != nil {
		return databaseOperationError("update world region", err)
	}
	return requireOneRow(result, "world region", region.ID)
}

func (s *Store) UpsertRegionAdjacency(ctx context.Context, adjacency worlddomain.RegionAdjacency) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO region_adjacencies(project_id, first_region_id, second_region_id, travel_note, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(project_id, first_region_id, second_region_id) DO UPDATE SET
			travel_note = excluded.travel_note,
			updated_at = excluded.updated_at
	`, adjacency.ProjectID, adjacency.FirstRegionID, adjacency.SecondRegionID, adjacency.TravelNote,
		formatTime(adjacency.CreatedAt), formatTime(adjacency.UpdatedAt))
	if err != nil {
		return databaseOperationError("upsert region adjacency", err)
	}
	return nil
}

func (s *Store) ListRegionAdjacencies(ctx context.Context) ([]worlddomain.RegionAdjacency, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT project_id, first_region_id, second_region_id, travel_note, created_at, updated_at
		FROM region_adjacencies
		ORDER BY first_region_id, second_region_id
	`)
	if err != nil {
		return nil, databaseOperationError("list region adjacencies", err)
	}
	defer rows.Close()
	adjacencies := make([]worlddomain.RegionAdjacency, 0)
	for rows.Next() {
		adjacency, err := scanRegionAdjacency(rows)
		if err != nil {
			return nil, err
		}
		adjacencies = append(adjacencies, adjacency)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate region adjacencies", err)
	}
	return adjacencies, nil
}

func (s *Store) DeleteRegionAdjacency(ctx context.Context, firstRegionID, secondRegionID string) error {
	if secondRegionID < firstRegionID {
		firstRegionID, secondRegionID = secondRegionID, firstRegionID
	}
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM region_adjacencies
		WHERE first_region_id = ? AND second_region_id = ?
	`, firstRegionID, secondRegionID)
	if err != nil {
		return databaseOperationError("delete region adjacency", err)
	}
	return nil
}

func (s *Store) ArchiveWorldRegion(ctx context.Context, id string, archivedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin archive world region", err)
	}
	defer tx.Rollback()
	var references int
	if err := tx.QueryRowContext(ctx, `
		SELECT (SELECT COUNT(*) FROM world_regions WHERE parent_id = ? AND deleted_at IS NULL)
		     + (SELECT COUNT(*) FROM locations WHERE region_id = ? AND deleted_at IS NULL)
		     + (SELECT COUNT(*) FROM world_rules WHERE scope_type = 'region' AND scope_id = ? AND deleted_at IS NULL)
		     + (SELECT COUNT(*) FROM region_adjacencies WHERE first_region_id = ? OR second_region_id = ?)
	`, id, id, id, id, id).Scan(&references); err != nil {
		return databaseOperationError("count world region references", err)
	}
	continuityReferences, err := countContinuityTargetReferences(ctx, tx, "region", id)
	if err != nil {
		return err
	}
	references += continuityReferences
	if references > 0 {
		return worlddomain.ErrReferenced
	}
	result, err := tx.ExecContext(ctx, `UPDATE world_regions SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive world region", err)
	}
	if err := requireOneRow(result, "world region", id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit archive world region", err)
	}
	return nil
}

func (s *Store) CreateLocation(ctx context.Context, location worlddomain.Location) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO locations(id, project_id, region_id, kind, name, summary, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, location.ID, location.ProjectID, location.RegionID, location.Kind, location.Name, location.Summary, location.Status,
		formatTime(location.CreatedAt), formatTime(location.UpdatedAt))
	if err != nil {
		return databaseOperationError("create location", err)
	}
	return nil
}

func (s *Store) ListLocations(ctx context.Context) ([]worlddomain.Location, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, region_id, kind, name, summary, status, created_at, updated_at
		FROM locations WHERE deleted_at IS NULL ORDER BY name
	`)
	if err != nil {
		return nil, databaseOperationError("list locations", err)
	}
	defer rows.Close()
	locations := make([]worlddomain.Location, 0)
	for rows.Next() {
		location, err := scanLocation(rows)
		if err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate locations", err)
	}
	return locations, nil
}

func (s *Store) Location(ctx context.Context, id string) (worlddomain.Location, error) {
	return scanLocation(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, region_id, kind, name, summary, status, created_at, updated_at
		FROM locations WHERE id = ? AND deleted_at IS NULL
	`, id))
}

func (s *Store) UpdateLocation(ctx context.Context, location worlddomain.Location) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE locations SET region_id = ?, kind = ?, name = ?, summary = ?, status = ?, updated_at = ?
		WHERE id = ? AND project_id = ? AND deleted_at IS NULL
	`, location.RegionID, location.Kind, location.Name, location.Summary, location.Status, formatTime(location.UpdatedAt), location.ID, location.ProjectID)
	if err != nil {
		return databaseOperationError("update location", err)
	}
	return requireOneRow(result, "location", location.ID)
}

func (s *Store) ArchiveLocation(ctx context.Context, id string, archivedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin archive location", err)
	}
	defer tx.Rollback()
	var references int
	if err := tx.QueryRowContext(ctx, `
		SELECT (SELECT COUNT(*) FROM location_links WHERE location_id = ?)
		     + (SELECT COUNT(*) FROM items WHERE holder_type = 'location' AND holder_id = ? AND deleted_at IS NULL)
		     + (SELECT COUNT(*) FROM world_rules WHERE scope_type = 'location' AND scope_id = ? AND deleted_at IS NULL)
	`, id, id, id).Scan(&references); err != nil {
		return databaseOperationError("count location references", err)
	}
	continuityReferences, err := countContinuityTargetReferences(ctx, tx, "location", id)
	if err != nil {
		return err
	}
	references += continuityReferences
	if references > 0 {
		return worlddomain.ErrReferenced
	}
	result, err := tx.ExecContext(ctx, `UPDATE locations SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive location", err)
	}
	if err := requireOneRow(result, "location", id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit archive location", err)
	}
	return nil
}

func (s *Store) UpsertLocationLink(ctx context.Context, link worlddomain.LocationLink) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO location_links(project_id, location_id, target_type, target_id, role, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(location_id, target_type, target_id) DO UPDATE SET role = excluded.role
	`, link.ProjectID, link.LocationID, link.TargetType, link.TargetID, link.Role, formatTime(link.CreatedAt))
	if err != nil {
		return databaseOperationError("upsert location link", err)
	}
	return nil
}

func (s *Store) ListLocationLinks(ctx context.Context) ([]worlddomain.LocationLink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT project_id, location_id, target_type, target_id, role, created_at FROM location_links ORDER BY created_at`)
	if err != nil {
		return nil, databaseOperationError("list location links", err)
	}
	defer rows.Close()
	links := make([]worlddomain.LocationLink, 0)
	for rows.Next() {
		var link worlddomain.LocationLink
		var createdAt string
		if err := rows.Scan(&link.ProjectID, &link.LocationID, &link.TargetType, &link.TargetID, &link.Role, &createdAt); err != nil {
			return nil, databaseOperationError("scan location link", err)
		}
		link.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse location link created_at: %w", err)
		}
		links = append(links, link)
	}
	return links, rows.Err()
}

func (s *Store) DeleteLocationLink(ctx context.Context, locationID string, targetType worlddomain.LocationTargetType, targetID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM location_links WHERE location_id = ? AND target_type = ? AND target_id = ?`, locationID, targetType, targetID)
	if err != nil {
		return databaseOperationError("delete location link", err)
	}
	return nil
}

func (s *Store) CreateItem(ctx context.Context, item worlddomain.Item) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO items(id, project_id, name, summary, status, holder_type, holder_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, item.ID, item.ProjectID, item.Name, item.Summary, item.Status, nullString(string(item.HolderType)), nullString(item.HolderID),
		formatTime(item.CreatedAt), formatTime(item.UpdatedAt))
	if err != nil {
		return databaseOperationError("create item", err)
	}
	return nil
}

func (s *Store) Item(ctx context.Context, id string) (worlddomain.Item, error) {
	return scanItem(s.db.QueryRowContext(ctx, `
		SELECT id, project_id, name, summary, status, holder_type, holder_id, created_at, updated_at
		FROM items WHERE id = ? AND deleted_at IS NULL
	`, id))
}

func (s *Store) ListItems(ctx context.Context) ([]worlddomain.Item, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, name, summary, status, holder_type, holder_id, created_at, updated_at
		FROM items WHERE deleted_at IS NULL ORDER BY name
	`)
	if err != nil {
		return nil, databaseOperationError("list items", err)
	}
	defer rows.Close()
	items := make([]worlddomain.Item, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) TransferItem(ctx context.Context, item worlddomain.Item, transfer worlddomain.ItemTransfer) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin item transfer", err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		UPDATE items SET holder_type = ?, holder_id = ?, status = ?, updated_at = ?
		WHERE id = ? AND project_id = ? AND deleted_at IS NULL
	`, item.HolderType, item.HolderID, item.Status, formatTime(item.UpdatedAt), item.ID, item.ProjectID)
	if err != nil {
		return databaseOperationError("update item holder", err)
	}
	if err := requireOneRow(result, "item", item.ID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO item_transfers(id, project_id, item_id, from_type, from_id, to_type, to_id, chapter_id, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, transfer.ID, transfer.ProjectID, transfer.ItemID, nullString(string(transfer.FromType)), nullString(transfer.FromID), transfer.ToType,
		transfer.ToID, nullString(transfer.ChapterID), transfer.Note, formatTime(transfer.CreatedAt))
	if err != nil {
		return databaseOperationError("record item transfer", err)
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit item transfer", err)
	}
	return nil
}

func (s *Store) ListItemTransfers(ctx context.Context, itemID string) ([]worlddomain.ItemTransfer, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, project_id, item_id, from_type, from_id, to_type, to_id, chapter_id, note, created_at
		FROM item_transfers WHERE item_id = ? ORDER BY created_at
	`, itemID)
	if err != nil {
		return nil, databaseOperationError("list item transfers", err)
	}
	defer rows.Close()
	transfers := make([]worlddomain.ItemTransfer, 0)
	for rows.Next() {
		var transfer worlddomain.ItemTransfer
		var fromType, fromID, chapterID sql.NullString
		var createdAt string
		if err := rows.Scan(&transfer.ID, &transfer.ProjectID, &transfer.ItemID, &fromType, &fromID, &transfer.ToType, &transfer.ToID, &chapterID, &transfer.Note, &createdAt); err != nil {
			return nil, databaseOperationError("scan item transfer", err)
		}
		transfer.FromType, transfer.FromID, transfer.ChapterID = worlddomain.HolderType(fromType.String), fromID.String, chapterID.String
		transfer.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, fmt.Errorf("parse item transfer created_at: %w", err)
		}
		transfers = append(transfers, transfer)
	}
	return transfers, rows.Err()
}

func (s *Store) UpdateItem(ctx context.Context, item worlddomain.Item) error {
	result, err := s.db.ExecContext(ctx, `UPDATE items SET name = ?, summary = ?, status = ?, updated_at = ? WHERE id = ? AND project_id = ? AND deleted_at IS NULL`, item.Name, item.Summary, item.Status, formatTime(item.UpdatedAt), item.ID, item.ProjectID)
	if err != nil {
		return databaseOperationError("update item", err)
	}
	return requireOneRow(result, "item", item.ID)
}

func (s *Store) ArchiveItem(ctx context.Context, id string, archivedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin archive item transaction", err)
	}
	defer tx.Rollback()
	references, err := countContinuityTargetReferences(ctx, tx, "item", id)
	if err != nil {
		return err
	}
	if references > 0 {
		return worlddomain.ErrReferenced
	}
	result, err := tx.ExecContext(ctx, `UPDATE items SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive item", err)
	}
	if err := requireOneRow(result, "item", id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit archive item transaction", err)
	}
	return nil
}

func (s *Store) CreateWorldRule(ctx context.Context, rule worlddomain.WorldRule) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO world_rules(id, project_id, name, description, scope_type, scope_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rule.ID, rule.ProjectID, rule.Name, rule.Description, rule.ScopeType, nullString(rule.ScopeID), rule.Status, formatTime(rule.CreatedAt), formatTime(rule.UpdatedAt))
	if err != nil {
		return databaseOperationError("create world rule", err)
	}
	return nil
}

func (s *Store) ListWorldRules(ctx context.Context) ([]worlddomain.WorldRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, name, description, scope_type, scope_id, status, created_at, updated_at FROM world_rules WHERE deleted_at IS NULL ORDER BY name`)
	if err != nil {
		return nil, databaseOperationError("list world rules", err)
	}
	defer rows.Close()
	rules := make([]worlddomain.WorldRule, 0)
	for rows.Next() {
		rule, err := scanWorldRule(rows)
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (s *Store) WorldRule(ctx context.Context, id string) (worlddomain.WorldRule, error) {
	return scanWorldRule(s.db.QueryRowContext(ctx, `SELECT id, project_id, name, description, scope_type, scope_id, status, created_at, updated_at FROM world_rules WHERE id = ? AND deleted_at IS NULL`, id))
}

func (s *Store) UpdateWorldRule(ctx context.Context, rule worlddomain.WorldRule) error {
	result, err := s.db.ExecContext(ctx, `UPDATE world_rules SET name = ?, description = ?, scope_type = ?, scope_id = ?, status = ?, updated_at = ? WHERE id = ? AND project_id = ? AND deleted_at IS NULL`, rule.Name, rule.Description, rule.ScopeType, nullString(rule.ScopeID), rule.Status, formatTime(rule.UpdatedAt), rule.ID, rule.ProjectID)
	if err != nil {
		return databaseOperationError("update world rule", err)
	}
	return requireOneRow(result, "world rule", rule.ID)
}

func (s *Store) ArchiveWorldRule(ctx context.Context, id string, archivedAt time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin archive world rule transaction", err)
	}
	defer tx.Rollback()
	references, err := countContinuityTargetReferences(ctx, tx, "rule", id)
	if err != nil {
		return err
	}
	if references > 0 {
		return worlddomain.ErrReferenced
	}
	result, err := tx.ExecContext(ctx, `UPDATE world_rules SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive world rule", err)
	}
	if err := requireOneRow(result, "world rule", id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit archive world rule transaction", err)
	}
	return nil
}

func (s *Store) CreateForeshadowingHook(ctx context.Context, hook worlddomain.ForeshadowingHook) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO foreshadowing_hooks(id, project_id, title, setup, planted_chapter_id, due_chapter_id, resolved_chapter_id, status, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, hook.ID, hook.ProjectID, hook.Title, hook.Setup, hook.PlantedChapterID, nullString(hook.DueChapterID), nullString(hook.ResolvedChapterID), hook.Status, hook.Notes, formatTime(hook.CreatedAt), formatTime(hook.UpdatedAt))
	if err != nil {
		return databaseOperationError("create foreshadowing hook", err)
	}
	return nil
}

func (s *Store) ListForeshadowingHooks(ctx context.Context) ([]worlddomain.ForeshadowingHook, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, project_id, title, setup, planted_chapter_id, due_chapter_id, resolved_chapter_id, status, notes, created_at, updated_at FROM foreshadowing_hooks WHERE deleted_at IS NULL ORDER BY created_at`)
	if err != nil {
		return nil, databaseOperationError("list foreshadowing hooks", err)
	}
	defer rows.Close()
	hooks := make([]worlddomain.ForeshadowingHook, 0)
	for rows.Next() {
		hook, err := scanForeshadowingHook(rows)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, hook)
	}
	return hooks, rows.Err()
}

func (s *Store) ForeshadowingHook(ctx context.Context, id string) (worlddomain.ForeshadowingHook, error) {
	return scanForeshadowingHook(s.db.QueryRowContext(ctx, `SELECT id, project_id, title, setup, planted_chapter_id, due_chapter_id, resolved_chapter_id, status, notes, created_at, updated_at FROM foreshadowing_hooks WHERE id = ? AND deleted_at IS NULL`, id))
}

func (s *Store) UpdateForeshadowingHook(ctx context.Context, hook worlddomain.ForeshadowingHook) error {
	result, err := s.db.ExecContext(ctx, `UPDATE foreshadowing_hooks SET title = ?, setup = ?, planted_chapter_id = ?, due_chapter_id = ?, resolved_chapter_id = ?, status = ?, notes = ?, updated_at = ? WHERE id = ? AND project_id = ? AND deleted_at IS NULL`, hook.Title, hook.Setup, hook.PlantedChapterID, nullString(hook.DueChapterID), nullString(hook.ResolvedChapterID), hook.Status, hook.Notes, formatTime(hook.UpdatedAt), hook.ID, hook.ProjectID)
	if err != nil {
		return databaseOperationError("update foreshadowing hook", err)
	}
	return requireOneRow(result, "foreshadowing hook", hook.ID)
}

func (s *Store) ArchiveForeshadowingHook(ctx context.Context, id string, archivedAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE foreshadowing_hooks SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`, formatTime(archivedAt), formatTime(archivedAt), id)
	if err != nil {
		return databaseOperationError("archive foreshadowing hook", err)
	}
	return requireOneRow(result, "foreshadowing hook", id)
}

func scanWorldRegion(row scanner) (worlddomain.WorldRegion, error) {
	var region worlddomain.WorldRegion
	var parentID sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&region.ID, &region.ProjectID, &parentID, &region.Kind, &region.Name, &region.Direction, &region.Summary, &region.Terrain, &region.Climate, &createdAt, &updatedAt)
	if err != nil {
		return worlddomain.WorldRegion{}, scanAssetError("world region", err)
	}
	region.ParentID = parentID.String
	region.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return worlddomain.WorldRegion{}, err
	}
	region.UpdatedAt, err = parseTime(updatedAt)
	return region, err
}

func scanRegionAdjacency(row scanner) (worlddomain.RegionAdjacency, error) {
	var adjacency worlddomain.RegionAdjacency
	var createdAt, updatedAt string
	if err := row.Scan(&adjacency.ProjectID, &adjacency.FirstRegionID, &adjacency.SecondRegionID, &adjacency.TravelNote, &createdAt, &updatedAt); err != nil {
		return worlddomain.RegionAdjacency{}, scanAssetError("region adjacency", err)
	}
	var err error
	adjacency.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return worlddomain.RegionAdjacency{}, err
	}
	adjacency.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return worlddomain.RegionAdjacency{}, err
	}
	return adjacency, nil
}

func scanLocation(row scanner) (worlddomain.Location, error) {
	var location worlddomain.Location
	var createdAt, updatedAt string
	err := row.Scan(&location.ID, &location.ProjectID, &location.RegionID, &location.Kind, &location.Name, &location.Summary, &location.Status, &createdAt, &updatedAt)
	if err != nil {
		return worlddomain.Location{}, scanAssetError("location", err)
	}
	location.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return worlddomain.Location{}, err
	}
	location.UpdatedAt, err = parseTime(updatedAt)
	return location, err
}

func scanItem(row scanner) (worlddomain.Item, error) {
	var item worlddomain.Item
	var holderType, holderID sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&item.ID, &item.ProjectID, &item.Name, &item.Summary, &item.Status, &holderType, &holderID, &createdAt, &updatedAt)
	if err != nil {
		return worlddomain.Item{}, scanAssetError("item", err)
	}
	item.HolderType, item.HolderID = worlddomain.HolderType(holderType.String), holderID.String
	item.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return worlddomain.Item{}, err
	}
	item.UpdatedAt, err = parseTime(updatedAt)
	return item, err
}

func scanWorldRule(row scanner) (worlddomain.WorldRule, error) {
	var rule worlddomain.WorldRule
	var scopeID sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&rule.ID, &rule.ProjectID, &rule.Name, &rule.Description, &rule.ScopeType, &scopeID, &rule.Status, &createdAt, &updatedAt)
	if err != nil {
		return worlddomain.WorldRule{}, scanAssetError("world rule", err)
	}
	rule.ScopeID = scopeID.String
	rule.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return worlddomain.WorldRule{}, err
	}
	rule.UpdatedAt, err = parseTime(updatedAt)
	return rule, err
}

func scanForeshadowingHook(row scanner) (worlddomain.ForeshadowingHook, error) {
	var hook worlddomain.ForeshadowingHook
	var dueChapterID, resolvedChapterID sql.NullString
	var createdAt, updatedAt string
	err := row.Scan(&hook.ID, &hook.ProjectID, &hook.Title, &hook.Setup, &hook.PlantedChapterID, &dueChapterID, &resolvedChapterID, &hook.Status, &hook.Notes, &createdAt, &updatedAt)
	if err != nil {
		return worlddomain.ForeshadowingHook{}, scanAssetError("foreshadowing hook", err)
	}
	hook.DueChapterID, hook.ResolvedChapterID = dueChapterID.String, resolvedChapterID.String
	hook.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return worlddomain.ForeshadowingHook{}, err
	}
	hook.UpdatedAt, err = parseTime(updatedAt)
	return hook, err
}

func scanAssetError(entity string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return databaseOperationError("scan "+entity, err)
}

func nullString(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
