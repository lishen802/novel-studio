package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// planningConstraintRows 保留完整正式载荷，并核对原类型节点的身份、层级和原始规划证据。
func planningConstraintRows(ctx context.Context, q constraintReader) ([]constraintRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT a.id,a.kind,a.current_revision,a.locked,CASE WHEN a.status='stale' THEN 1 ELSE a.stale END,a.typed_id,r.payload_json,original.payload_json,COALESCE(source.lock_map_json,'{}'),COALESCE(n.title,''),COALESCE(n.summary,''),COALESCE(n.goal,''),COALESCE(n.conflict,''),COALESCE(n.outcome,''),COALESCE(n.parent_id,''),COALESCE(n.level,''),COALESCE(n.position,0),COALESCE(n.chapter_id,''),COALESCE(n.planning_json,'{}')
 FROM story_assets a JOIN story_asset_revisions r ON r.asset_id=a.id AND r.revision=a.current_revision
 JOIN story_asset_revisions original ON original.asset_id=a.id AND original.revision=1
 LEFT JOIN creation_candidates source ON source.id=a.source_candidate_id
 LEFT JOIN outline_nodes n ON a.kind='outline_node' AND a.typed_table='outline_nodes' AND a.typed_id=n.id AND a.project_id=n.project_id AND n.deleted_at IS NULL
 WHERE a.status IN ('canonical','stale') AND (a.kind='proposal' OR (a.kind='outline_node' AND n.id IS NOT NULL)) ORDER BY a.kind,a.id`)
	if err != nil {
		return nil, fmt.Errorf("read planning revisions: %w", err)
	}
	defer rows.Close()
	out := []constraintRow{}
	for rows.Next() {
		var row constraintRow
		var locked, stale, position int
		var original, locks, title, summary, goal, conflict, outcome, parent, level, chapter, planning string
		if err = rows.Scan(&row.AssetID, &row.Kind, &row.Revision, &locked, &stale, &row.typedID, &row.payload, &original, &locks, &title, &summary, &goal, &conflict, &outcome, &parent, &level, &position, &chapter, &planning); err != nil {
			return nil, err
		}
		row.TypedID = row.typedID
		row.Locked = locked == 1
		row.stale = stale == 1
		row.Original = constraintText(row.Kind, original)
		row.Current = constraintText(row.Kind, row.payload)
		if row.Original == "" || row.Current == "" {
			return nil, asset.ErrChangeUnsupported
		}
		var originalFields map[string]string
		_ = json.Unmarshal([]byte(row.Original), &originalFields)
		row.Name = originalFields["/title"]
		row.Fields, err = asset.PlanningFields(row.Kind, row.payload)
		if err != nil {
			return nil, err
		}
		row.Changes = []asset.ConstraintChange{}
		var metadata map[string]json.RawMessage
		if json.Unmarshal([]byte(row.payload), &metadata) != nil {
			return nil, asset.ErrChangeUnsupported
		}
		if snapshot, ok := metadata["_sourceLocks"]; ok {
			locks = string(snapshot)
		}
		var lockMap map[string]bool
		if json.Unmarshal([]byte(locks), &lockMap) != nil {
			return nil, asset.ErrChangeUnsupported
		}
		for _, locked := range lockMap {
			if locked {
				row.Locked = true
			}
		}
		if row.Kind == "outline_node" {
			var baseline struct {
				ParentID  string          `json:"parentId"`
				Level     string          `json:"level"`
				Position  int             `json:"position"`
				ChapterID string          `json:"chapterId"`
				Planning  json.RawMessage `json:"planning"`
			}
			if json.Unmarshal([]byte(original), &baseline) != nil {
				return nil, asset.ErrChangeUnsupported
			}
			current := map[string]any{"title": title, "summary": summary, "goal": goal, "conflict": conflict, "outcome": outcome}
			if len(baseline.Planning) > 0 {
				current["planning"] = json.RawMessage(planning)
			}
			encoded, _ := json.Marshal(current)
			row.stale = row.stale || constraintText(row.Kind, string(encoded)) != row.Original || baseline.ParentID != parent || baseline.Level != level || baseline.Position != position || baseline.ChapterID != chapter
			var before, after any
			_ = json.Unmarshal(baseline.Planning, &before)
			_ = json.Unmarshal([]byte(planning), &after)
			if before == nil {
				before = map[string]any{}
			}
			if !reflect.DeepEqual(before, after) {
				row.stale = true
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// PlanningRevisions 读取正式规划的可编辑文字与历史，原始项目元信息和节点不被未来版本覆盖。
func (s *Store) PlanningRevisions(ctx context.Context) ([]asset.ConstraintView, error) {
	return s.constraintHistory(ctx, "planning")
}

// rejectFormalOutlineWrite 禁止通过旧全局命令覆盖或删除正式规划节点。
func rejectFormalOutlineWrite(ctx context.Context, q constraintReader, id string) error {
	var exists bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM story_assets WHERE kind='outline_node' AND typed_table='outline_nodes' AND typed_id=? AND status IN ('canonical','stale'))`, id).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return asset.ErrChangeUnsupported
	}
	return nil
}

// rejectFormalOutlineStructure 正式规划树不能通过新增类型行绕过版本和结构审阅。
func rejectFormalOutlineStructure(ctx context.Context, q constraintReader, projectID string) error {
	var exists bool
	if err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM story_assets WHERE project_id=? AND kind='outline_node' AND status IN ('canonical','stale'))`, projectID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return asset.ErrChangeUnsupported
	}
	return nil
}
