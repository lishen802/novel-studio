package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// PreviewFormalOutlineReorder 核对两个相邻正式章节节点的未来顺序和写作边界。
func (s *Store) PreviewFormalOutlineReorder(ctx context.Context, r asset.FormalOutlineReorderRequest) (asset.FormalOutlineReorderPreview, error) {
	return previewFormalOutlineReorder(ctx, s.db, r)
}

func previewFormalOutlineReorder(ctx context.Context, q constraintReader, r asset.FormalOutlineReorderRequest) (asset.FormalOutlineReorderPreview, error) {
	p := asset.FormalOutlineReorderPreview{Request: r}
	if r.ProjectID == "" || r.BranchID == "" || r.FirstAssetID == "" || r.SecondAssetID == "" || r.FirstAssetID == r.SecondAssetID || r.ChapterID == "" || strings.TrimSpace(r.Reason) == "" || utf8.RuneCountInString(r.Reason) > 2000 {
		return p, asset.ErrChangeUnsupported
	}
	var chapterProject, body string
	if err := q.QueryRowContext(ctx, `SELECT project_id,position,body FROM chapters WHERE id=?`, r.ChapterID).Scan(&chapterProject, &p.Position, &body); err != nil || chapterProject != r.ProjectID || body != "" {
		return p, asset.ErrChangeConflict
	}
	var written int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM chapters c WHERE c.project_id=? AND c.position>=? AND (c.body<>'' OR EXISTS(SELECT 1 FROM chapter_revisions h WHERE h.chapter_id=c.id AND h.body<>'') OR EXISTS(SELECT 1 FROM drafts d WHERE d.chapter_id=c.id AND d.body<>''))`, r.ProjectID, p.Position).Scan(&written); err != nil || written != 0 {
		return p, asset.ErrChangeConflict
	}
	rows, err := planningConstraintRows(ctx, q)
	if err != nil {
		return p, err
	}
	selected := map[string]constraintRow{}
	for _, row := range rows {
		if row.AssetID == r.FirstAssetID || row.AssetID == r.SecondAssetID {
			if row.Kind != "outline_node" || row.Locked || row.stale {
				return p, asset.ErrChangeConflict
			}
			selected[row.AssetID] = row
		}
	}
	first, firstOK := selected[r.FirstAssetID]
	second, secondOK := selected[r.SecondAssetID]
	if !firstOK || !secondOK {
		return p, asset.ErrChangeConflict
	}
	var firstParent, secondParent, firstLevel, secondLevel, firstStatus, secondStatus string
	for _, target := range []struct {
		row                   constraintRow
		parent, level, status *string
	}{
		{first, &firstParent, &firstLevel, &firstStatus}, {second, &secondParent, &secondLevel, &secondStatus},
	} {
		// 分支数据库可从已确认主分支复制，来源身份以当前数据库内的资产证据为准。
		if err := q.QueryRowContext(ctx, `SELECT COALESCE(n.parent_id,''),n.level,a.status FROM story_assets a JOIN outline_nodes n ON n.id=a.typed_id AND n.project_id=a.project_id AND n.deleted_at IS NULL WHERE a.id=? AND a.project_id=? AND a.typed_table='outline_nodes' AND EXISTS(SELECT 1 FROM formal_outline_introductions x WHERE x.asset_id=a.id UNION SELECT 1 FROM creation_candidates c JOIN creation_sessions s ON s.id=c.session_id WHERE c.id=a.source_candidate_id AND c.kind='roadmap' AND s.project_id=a.project_id)`, target.row.AssetID, r.ProjectID).Scan(target.parent, target.level, target.status); err != nil {
			return p, asset.ErrChangeConflict
		}
	}
	if firstParent == "" || firstParent != secondParent || firstLevel != "chapter" || secondLevel != "chapter" || firstStatus != "canonical" || secondStatus != "canonical" {
		return p, asset.ErrChangeConflict
	}
	p.ParentNodeID = firstParent
	p.FirstNodeID, p.SecondNodeID = first.TypedID, second.TypedID
	p.FirstTitle, p.SecondTitle = first.Name, second.Name
	p.FirstRevision, p.SecondRevision = first.Revision+1, second.Revision+1
	for _, id := range []string{r.FirstAssetID, r.SecondAssetID} {
		var intro, retired, lastRevision int
		if err := q.QueryRowContext(ctx, `SELECT COALESCE((SELECT position FROM formal_outline_introductions WHERE asset_id=?),0),COALESCE((SELECT position FROM formal_outline_retirements WHERE asset_id=?),(SELECT position FROM formal_outline_origin_retirements WHERE asset_id=?),0),COALESCE((SELECT MAX(position) FROM story_constraint_changes WHERE asset_id=?),0)`, id, id, id, id).Scan(&intro, &retired, &lastRevision); err != nil || p.Position < intro || (retired > 0 && p.Position >= retired) || p.Position <= lastRevision {
			return p, asset.ErrChangeConflict
		}
	}
	var lastOrder int
	if err := q.QueryRowContext(ctx, `SELECT COALESCE(MAX(position),0) FROM formal_outline_reorders WHERE project_id=? AND parent_node_id=?`, r.ProjectID, firstParent).Scan(&lastOrder); err != nil || p.Position <= lastOrder {
		return p, asset.ErrChangeConflict
	}
	positions, err := effectiveFormalOutlinePositions(ctx, q, r.ProjectID, p.Position)
	if err != nil {
		return p, err
	}
	p.FirstPosition, p.SecondPosition = positions[first.TypedID], positions[second.TypedID]
	if p.FirstPosition < 1 || p.SecondPosition != p.FirstPosition+1 {
		return p, asset.ErrChangeConflict
	}
	var pending int
	if err := q.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM assistant_authorizations a JOIN assistant_sessions s ON s.id=a.session_id WHERE s.project_id=? AND a.state IN ('pending_confirmation','active'))+(SELECT COUNT(*) FROM assistant_actions WHERE project_id=? AND state IN ('pending_confirmation','executing','outcome_unknown'))+(SELECT COUNT(*) FROM change_operations o JOIN change_sets c ON c.id=o.change_set_id WHERE c.project_id=? AND o.status='pending')+(SELECT COUNT(*) FROM agent_runs WHERE project_id=? AND status IN ('queued','running'))`, r.ProjectID, r.ProjectID, r.ProjectID, r.ProjectID).Scan(&pending); err != nil || pending != 0 {
		return p, asset.ErrChangeConflict
	}
	fingerprint, err := constraintFingerprint(ctx, q, asset.ConstraintRequest{ProjectID: r.ProjectID, BranchID: r.BranchID, AssetID: r.FirstAssetID, ChapterID: r.ChapterID, Intent: "future_only", Reason: r.Reason})
	if err != nil {
		return p, err
	}
	encoded, _ := json.Marshal(struct {
		Preview     asset.FormalOutlineReorderPreview
		Fingerprint string
		LastOrder   int
	}{p, fingerprint, lastOrder})
	sum := sha256.Sum256(encoded)
	p.Digest = hex.EncodeToString(sum[:])
	return p, nil
}

func effectiveFormalOutlinePositions(ctx context.Context, q constraintReader, projectID string, position int) (map[string]int, error) {
	rows, err := q.QueryContext(ctx, `SELECT a.typed_id,r.payload_json FROM story_assets a JOIN story_asset_revisions r ON r.asset_id=a.id AND r.revision=COALESCE((SELECT c.revision FROM story_constraint_changes c WHERE c.asset_id=a.id AND c.position<=? ORDER BY c.position DESC LIMIT 1),1) LEFT JOIN formal_outline_introductions i ON i.asset_id=a.id LEFT JOIN formal_outline_retirements x ON x.asset_id=a.id LEFT JOIN formal_outline_origin_retirements o ON o.asset_id=a.id WHERE a.project_id=? AND a.kind='outline_node' AND a.status='canonical' AND (i.position IS NULL OR i.position<=?) AND (x.position IS NULL OR x.position>?) AND (o.position IS NULL OR o.position>?)`, position, projectID, position, position, position)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	positions := map[string]int{}
	for rows.Next() {
		var nodeID, payload string
		var current struct {
			Position int `json:"position"`
		}
		if err := rows.Scan(&nodeID, &payload); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(payload), &current) != nil || current.Position < 1 {
			return nil, asset.ErrChangeUnsupported
		}
		positions[nodeID] = current.Position
	}
	return positions, rows.Err()
}

// FormalOutlinePositionsAtChapter 只读返回指定章节实际生效的正式节点位置。
func (s *Store) FormalOutlinePositionsAtChapter(ctx context.Context, projectID, chapterID string) (map[string]int, error) {
	var position int
	if err := s.db.QueryRowContext(ctx, `SELECT position FROM chapters WHERE project_id=? AND id=?`, projectID, chapterID).Scan(&position); err != nil {
		return nil, asset.ErrChangeConflict
	}
	return effectiveFormalOutlinePositions(ctx, s.db, projectID, position)
}

// ConfirmFormalOutlineReorder 在同一事务内追加两个位置版本和幂等回执。
func (s *Store) ConfirmFormalOutlineReorder(ctx context.Context, cmd asset.FormalOutlineReorderCommand, operationID string, now time.Time) (asset.FormalOutlineReorderPreview, error) {
	if cmd.IdempotencyKey == "" || len(cmd.IdempotencyKey) > 200 || cmd.Digest == "" || operationID == "" {
		return asset.FormalOutlineReorderPreview{}, asset.ErrChangeConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return asset.FormalOutlineReorderPreview{}, err
	}
	defer tx.Rollback()
	requestJSON, _ := json.Marshal(cmd.Request)
	var oldRequest, oldDigest, receipt string
	err = tx.QueryRowContext(ctx, `SELECT request_json,digest,receipt_json FROM formal_outline_reorders WHERE project_id=? AND branch_id=? AND idempotency_key=?`, cmd.Request.ProjectID, cmd.Request.BranchID, cmd.IdempotencyKey).Scan(&oldRequest, &oldDigest, &receipt)
	if err == nil {
		var prior asset.FormalOutlineReorderPreview
		if oldRequest != string(requestJSON) || oldDigest != cmd.Digest || json.Unmarshal([]byte(receipt), &prior) != nil {
			return prior, asset.ErrChangeConflict
		}
		return prior, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return asset.FormalOutlineReorderPreview{}, err
	}
	p, err := previewFormalOutlineReorder(ctx, tx, cmd.Request)
	if err != nil || p.Digest != cmd.Digest {
		return p, asset.ErrChangeConflict
	}
	for i, target := range []struct {
		assetID  string
		revision int64
		position int
	}{{cmd.Request.FirstAssetID, p.FirstRevision, p.SecondPosition}, {cmd.Request.SecondAssetID, p.SecondRevision, p.FirstPosition}} {
		var payload string
		if err := tx.QueryRowContext(ctx, `SELECT payload_json FROM story_asset_revisions WHERE asset_id=? AND revision=?`, target.assetID, target.revision-1).Scan(&payload); err != nil {
			return p, asset.ErrChangeConflict
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal([]byte(payload), &fields) != nil {
			return p, asset.ErrChangeUnsupported
		}
		fields["position"], _ = json.Marshal(target.position)
		encoded, _ := json.Marshal(fields)
		sum := sha256.Sum256(encoded)
		if _, err := tx.ExecContext(ctx, `INSERT INTO story_asset_revisions(asset_id,revision,payload_json,content_sha256,created_by,created_at) VALUES(?,?,?,?,?,?)`, target.assetID, target.revision, string(encoded), hex.EncodeToString(sum[:]), "author-formal-reorder", formatTime(now)); err != nil {
			return p, err
		}
		updated, err := tx.ExecContext(ctx, `UPDATE story_assets SET current_revision=?,updated_at=? WHERE id=? AND current_revision=? AND status='canonical' AND locked=0 AND stale=0`, target.revision, formatTime(now), target.assetID, target.revision-1)
		if err != nil {
			return p, err
		}
		if err := requireOneRow(updated, "formal reorder", target.assetID); err != nil {
			return p, err
		}
		serialized, _ := json.Marshal(p)
		if _, err := tx.ExecContext(ctx, `INSERT INTO story_constraint_changes(project_id,asset_id,revision,chapter_id,position,reason,digest,idempotency_key,request_json,receipt_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, cmd.Request.ProjectID, target.assetID, target.revision, cmd.Request.ChapterID, p.Position, cmd.Request.Reason, cmd.Digest, fmt.Sprintf("formal-reorder:%s:%d", operationID, i), string(requestJSON), string(serialized), formatTime(now)); err != nil {
			return p, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE creation_candidates SET status='stale',updated_at=? WHERE status IN ('draft','selected','confirmed')`, formatTime(now)); err != nil {
		return p, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE creation_sessions SET content_version=content_version+1,updated_at=?`, formatTime(now)); err != nil {
		return p, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE assistant_authorizations SET state='stale',updated_at=? WHERE state IN ('pending_confirmation','active')`, formatTime(now)); err != nil {
		return p, err
	}
	serialized, _ := json.Marshal(p)
	if _, err := tx.ExecContext(ctx, `INSERT INTO formal_outline_reorders(id,project_id,branch_id,first_asset_id,second_asset_id,parent_node_id,chapter_id,position,reason,digest,idempotency_key,request_json,receipt_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, operationID, cmd.Request.ProjectID, cmd.Request.BranchID, cmd.Request.FirstAssetID, cmd.Request.SecondAssetID, p.ParentNodeID, cmd.Request.ChapterID, p.Position, cmd.Request.Reason, cmd.Digest, cmd.IdempotencyKey, string(requestJSON), string(serialized), formatTime(now)); err != nil {
		return p, err
	}
	if err := tx.Commit(); err != nil {
		return p, fmt.Errorf("commit formal outline reorder: %w", err)
	}
	return p, nil
}
