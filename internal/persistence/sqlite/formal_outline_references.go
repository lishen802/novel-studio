package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// FormalOutlineReferences 返回正式章节节点的确定性直接引用，不修改档案。
func (s *Store) FormalOutlineReferences(ctx context.Context, projectID, branchID, assetID string) (asset.FormalOutlineReferenceView, error) {
	return formalOutlineReferences(ctx, s.db, projectID, branchID, assetID)
}

func formalOutlineReferences(ctx context.Context, q constraintReader, projectID, branchID, assetID string) (asset.FormalOutlineReferenceView, error) {
	view := asset.FormalOutlineReferenceView{AssetID: assetID, References: []asset.FormalOutlineReference{}}
	if projectID == "" || branchID == "" || assetID == "" {
		return view, asset.ErrChangeUnsupported
	}
	err := q.QueryRowContext(ctx, `SELECT n.id,n.title FROM story_assets a JOIN outline_nodes n ON n.id=a.typed_id AND n.project_id=a.project_id AND n.deleted_at IS NULL WHERE a.id=? AND a.project_id=? AND a.kind='outline_node' AND a.typed_table='outline_nodes' AND a.status='canonical' AND n.level='chapter' AND (EXISTS(SELECT 1 FROM formal_outline_introductions i WHERE i.asset_id=a.id AND i.branch_id=?) OR EXISTS(SELECT 1 FROM creation_candidates c JOIN creation_sessions s ON s.id=c.session_id WHERE c.id=a.source_candidate_id AND c.kind='roadmap' AND s.project_id=a.project_id AND s.branch_id=?))`, assetID, projectID, branchID, branchID).Scan(&view.NodeID, &view.Title)
	if errors.Is(err, sql.ErrNoRows) {
		return view, asset.ErrChangeConflict
	}
	if err != nil {
		return view, fmt.Errorf("read formal outline node: %w", err)
	}
	queries := []struct {
		kind, sql string
		args      []any
	}{
		{"child", `SELECT id,title FROM outline_nodes WHERE project_id=? AND parent_id=? AND deleted_at IS NULL ORDER BY position,id`, []any{projectID, view.NodeID}},
		{"story_line", `SELECT l.story_line_id,COALESCE(s.name,l.story_line_id) FROM story_line_links l LEFT JOIN story_lines s ON s.id=l.story_line_id AND s.project_id=l.project_id WHERE l.project_id=? AND l.outline_node_id=? ORDER BY l.story_line_id`, []any{projectID, view.NodeID}},
		{"scene_card", `SELECT id,title FROM scene_cards WHERE project_id=? AND outline_node_id=? AND deleted_at IS NULL ORDER BY id`, []any{projectID, view.NodeID}},
		{"anchor", `SELECT id,statement FROM story_anchors WHERE project_id=? AND scope_kind='roadmap_node' AND scope_id IN (?,?) ORDER BY id`, []any{projectID, view.NodeID, assetID}},
		{"formal_asset", `SELECT a.id,a.kind||' · '||COALESCE(a.typed_id,'') FROM story_assets a JOIN story_asset_revisions v ON v.asset_id=a.id AND v.revision=a.current_revision WHERE a.project_id=? AND a.id<>? AND EXISTS(SELECT 1 FROM json_tree(v.payload_json) j WHERE j.type='text' AND j.value IN (?,?)) ORDER BY a.id`, []any{projectID, assetID, view.NodeID, assetID}},
	}
	for _, query := range queries {
		rows, e := q.QueryContext(ctx, query.sql, query.args...)
		if e != nil {
			return view, fmt.Errorf("read formal outline %s references: %w", query.kind, e)
		}
		for rows.Next() {
			var item asset.FormalOutlineReference
			item.Kind = query.kind
			if e = rows.Scan(&item.ID, &item.Label); e != nil {
				rows.Close()
				return view, e
			}
			view.References = append(view.References, item)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return view, e
		}
	}
	return view, nil
}
