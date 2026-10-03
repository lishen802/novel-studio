package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// characterConstraintRows 读取完整版本并保守继承人物锁，原类型档案仍代表生效前的资料。
func characterConstraintRows(ctx context.Context, q constraintReader) ([]constraintRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT a.id,a.current_revision,a.locked,CASE WHEN a.status='stale' THEN 1 ELSE a.stale END,a.typed_id,c.name,r.payload_json,original.payload_json,COALESCE(source.lock_map_json,'{}'),c.aliases_json,c.role,c.summary,c.motivation,c.fear,c.secret,c.arc
 FROM story_assets a JOIN characters c ON a.typed_table='characters' AND a.typed_id=c.id AND a.project_id=c.project_id
 JOIN story_asset_revisions r ON r.asset_id=a.id AND r.revision=a.current_revision
 JOIN story_asset_revisions original ON original.asset_id=a.id AND original.revision=1
 LEFT JOIN creation_candidates source ON source.id=a.source_candidate_id
 WHERE a.kind='character' AND a.status IN ('canonical','stale') AND c.deleted_at IS NULL ORDER BY a.id`)
	if err != nil {
		return nil, fmt.Errorf("read character revisions: %w", err)
	}
	defer rows.Close()
	result := []constraintRow{}
	for rows.Next() {
		var row constraintRow
		var locked, stale int
		var original, sourceLocks, aliases string
		var typed asset.CharacterProfile
		if err = rows.Scan(&row.AssetID, &row.Revision, &locked, &stale, &row.typedID, &row.Name, &row.payload, &original, &sourceLocks, &aliases, &typed.Role, &typed.Summary, &typed.Motivation, &typed.Fear, &typed.Secret, &typed.Arc); err != nil {
			return nil, err
		}
		typed.Name = row.Name
		if err = json.Unmarshal([]byte(aliases), &typed.Aliases); err != nil {
			return nil, err
		}
		row.Kind = "character"
		row.TypedID = row.typedID
		row.Original = constraintText(row.Kind, original)
		row.Current = constraintText(row.Kind, row.payload)
		var baseline asset.CharacterProfile
		if err = json.Unmarshal([]byte(original), &baseline); err != nil {
			return nil, err
		}
		baselineJSON := []byte(row.Original)
		typedJSON, _ := json.Marshal(typed)
		row.stale = stale == 1 || string(baselineJSON) != string(typedJSON)
		row.Locked = locked == 1
		row.Changes = []asset.ConstraintChange{}
		var metadata map[string]json.RawMessage
		if err = json.Unmarshal([]byte(row.payload), &metadata); err != nil {
			return nil, err
		}
		locksJSON := []byte(sourceLocks)
		if snapshot, ok := metadata["_sourceLocks"]; ok {
			locksJSON = snapshot
		}
		var locks map[string]bool
		if err = json.Unmarshal(locksJSON, &locks); err != nil {
			return nil, err
		}
		for path, locked := range locks {
			if locked && (path == "" || path == "/" || path == "/characters" || strings.HasPrefix(path, "/characters/")) {
				row.Locked = true
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// rejectFormalCharacterWrite 防止旧手工或 ChangeSet 命令绕过正式人物版本和历史保护。
func rejectFormalCharacterWrite(ctx context.Context, q constraintReader, id string) error {
	var formal bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM story_assets WHERE kind='character' AND typed_table='characters' AND typed_id=? AND status IN ('canonical','stale'))`, id).Scan(&formal); err != nil {
		return err
	}
	if formal {
		return asset.ErrChangeUnsupported
	}
	return nil
}
