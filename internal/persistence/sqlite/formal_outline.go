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

// FormalOutlineIntroductions 返回新增正式节点的生效章节，供档案视图标注。
func (s *Store) FormalOutlineIntroductions(ctx context.Context, projectID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,position FROM formal_outline_introductions WHERE project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var id string
		var position int
		if err = rows.Scan(&id, &position); err != nil {
			return nil, err
		}
		out[id] = position
	}
	return out, rows.Err()
}

// PreviewFormalOutlineAppend 以正式资产和历史正文为依据计算唯一的追加位置。
func (s *Store) PreviewFormalOutlineAppend(ctx context.Context, request asset.FormalOutlineAppendRequest) (asset.FormalOutlineAppendPreview, error) {
	return previewFormalOutlineAppend(ctx, s.db, request)
}

func previewFormalOutlineAppend(ctx context.Context, q constraintReader, r asset.FormalOutlineAppendRequest) (asset.FormalOutlineAppendPreview, error) {
	p := asset.FormalOutlineAppendPreview{Request: r}
	if r.ProjectID == "" || r.BranchID == "" || r.ParentAssetID == "" || r.ChapterID == "" || strings.TrimSpace(r.Title) == "" || strings.TrimSpace(r.Reason) == "" || utf8.RuneCountInString(r.Title) > 200 || utf8.RuneCountInString(r.Reason) > 2000 || utf8.RuneCountInString(r.Summary)+utf8.RuneCountInString(r.Goal)+utf8.RuneCountInString(r.Conflict)+utf8.RuneCountInString(r.Outcome) > 12000 {
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
	var parentID, level, status string
	var locked, stale int
	if err := q.QueryRowContext(ctx, `SELECT a.typed_id,n.level,a.status,a.locked,a.stale FROM story_assets a JOIN outline_nodes n ON n.id=a.typed_id AND n.project_id=a.project_id AND n.deleted_at IS NULL WHERE a.id=? AND a.project_id=? AND a.kind='outline_node' AND a.typed_table='outline_nodes'`, r.ParentAssetID, r.ProjectID).Scan(&parentID, &level, &status, &locked, &stale); err != nil || level != "arc" || status != "canonical" || locked != 0 || stale != 0 {
		return p, asset.ErrChangeConflict
	}
	rows, err := planningConstraintRows(ctx, q)
	if err != nil {
		return p, err
	}
	validParent := false
	for _, row := range rows {
		if row.AssetID == r.ParentAssetID {
			validParent = !row.Locked && !row.stale
			for _, field := range row.Fields {
				if field.Path == "/title" {
					p.ParentTitle = field.Value
				}
			}
		}
	}
	if !validParent {
		return p, asset.ErrChangeConflict
	}
	var lastBoundary, parentIntroduction int
	if err = q.QueryRowContext(ctx, `SELECT COALESCE(MAX(position),0) FROM story_constraint_changes WHERE asset_id=?`, r.ParentAssetID).Scan(&lastBoundary); err != nil || p.Position <= lastBoundary {
		return p, asset.ErrChangeConflict
	}
	if err = q.QueryRowContext(ctx, `SELECT COALESCE((SELECT position FROM formal_outline_introductions WHERE asset_id=?),0)`, r.ParentAssetID).Scan(&parentIntroduction); err != nil || p.Position < parentIntroduction {
		return p, asset.ErrChangeConflict
	}
	if err = q.QueryRowContext(ctx, `SELECT COALESCE(MAX(position),0)+1 FROM outline_nodes WHERE project_id=? AND parent_id=? AND deleted_at IS NULL`, r.ProjectID, parentID).Scan(&p.NodePosition); err != nil {
		return p, err
	}
	var pending int
	if err = q.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM assistant_authorizations a JOIN assistant_sessions s ON s.id=a.session_id WHERE s.project_id=? AND a.state IN ('pending_confirmation','active'))+(SELECT COUNT(*) FROM assistant_actions WHERE project_id=? AND state IN ('pending_confirmation','executing','outcome_unknown'))+(SELECT COUNT(*) FROM change_operations o JOIN change_sets c ON c.id=o.change_set_id WHERE c.project_id=? AND o.status='pending')+(SELECT COUNT(*) FROM agent_runs WHERE project_id=? AND status IN ('queued','running'))`, r.ProjectID, r.ProjectID, r.ProjectID, r.ProjectID).Scan(&pending); err != nil || pending != 0 {
		return p, asset.ErrChangeConflict
	}
	fingerprint, err := constraintFingerprint(ctx, q, asset.ConstraintRequest{ProjectID: r.ProjectID, BranchID: r.BranchID, AssetID: r.ParentAssetID, ChapterID: r.ChapterID, Intent: "future_only", Reason: r.Reason})
	if err != nil {
		return p, err
	}
	encoded, _ := json.Marshal(struct {
		Preview      asset.FormalOutlineAppendPreview
		Fingerprint  string
		LastBoundary int
	}{p, fingerprint, lastBoundary})
	sum := sha256.Sum256(encoded)
	p.Digest = hex.EncodeToString(sum[:])
	return p, nil
}

// ConfirmFormalOutlineAppend 在同一事务内复算预览并追加类型行、资产与生效回执。
func (s *Store) ConfirmFormalOutlineAppend(ctx context.Context, cmd asset.FormalOutlineAppendCommand, nodeID, assetID string, now time.Time) (asset.FormalOutlineAppendPreview, error) {
	if cmd.IdempotencyKey == "" || len(cmd.IdempotencyKey) > 200 || cmd.Digest == "" || nodeID == "" || assetID == "" {
		return asset.FormalOutlineAppendPreview{}, asset.ErrChangeConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return asset.FormalOutlineAppendPreview{}, err
	}
	defer tx.Rollback()
	requestJSON, _ := json.Marshal(cmd.Request)
	var oldRequest, oldDigest, receipt string
	err = tx.QueryRowContext(ctx, `SELECT request_json,digest,receipt_json FROM formal_outline_introductions WHERE project_id=? AND branch_id=? AND idempotency_key=?`, cmd.Request.ProjectID, cmd.Request.BranchID, cmd.IdempotencyKey).Scan(&oldRequest, &oldDigest, &receipt)
	if err == nil {
		var previous asset.FormalOutlineAppendPreview
		if oldRequest != string(requestJSON) || oldDigest != cmd.Digest || json.Unmarshal([]byte(receipt), &previous) != nil {
			return previous, asset.ErrChangeConflict
		}
		return previous, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return asset.FormalOutlineAppendPreview{}, err
	}
	p, err := previewFormalOutlineAppend(ctx, tx, cmd.Request)
	if err != nil || p.Digest != cmd.Digest {
		return p, asset.ErrChangeConflict
	}
	var parentNode string
	if err = tx.QueryRowContext(ctx, `SELECT typed_id FROM story_assets WHERE id=? AND project_id=?`, cmd.Request.ParentAssetID, cmd.Request.ProjectID).Scan(&parentNode); err != nil {
		return p, asset.ErrChangeConflict
	}
	p.NodeID, p.AssetID = nodeID, assetID
	r := cmd.Request
	_, err = tx.ExecContext(ctx, `INSERT INTO outline_nodes(id,project_id,parent_id,level,title,summary,goal,conflict,outcome,position,chapter_id,planning_json,created_at,updated_at,deleted_at) VALUES(?,?,?,'chapter',?,?,?,?,?,?,NULL,'{}',?,?,NULL)`, nodeID, r.ProjectID, parentNode, strings.TrimSpace(r.Title), strings.TrimSpace(r.Summary), strings.TrimSpace(r.Goal), strings.TrimSpace(r.Conflict), strings.TrimSpace(r.Outcome), p.NodePosition, formatTime(now), formatTime(now))
	if err != nil {
		return p, fmt.Errorf("insert future outline node: %w", err)
	}
	payload, _ := json.Marshal(map[string]any{"parentId": parentNode, "level": "chapter", "title": strings.TrimSpace(r.Title), "summary": strings.TrimSpace(r.Summary), "goal": strings.TrimSpace(r.Goal), "conflict": strings.TrimSpace(r.Conflict), "outcome": strings.TrimSpace(r.Outcome), "position": p.NodePosition})
	formal, err := asset.New(assetID, r.ProjectID, asset.KindOutlineNode, asset.OriginAuthor, "", "", "", "outline_nodes", nodeID, now)
	if err != nil {
		return p, err
	}
	if err = insertStoryAsset(ctx, tx, formal, string(payload), now); err != nil {
		return p, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE story_asset_revisions SET created_by='author-formal-outline' WHERE asset_id=? AND revision=1`, assetID); err != nil {
		return p, err
	}
	encoded, _ := json.Marshal(p)
	_, err = tx.ExecContext(ctx, `INSERT INTO formal_outline_introductions(asset_id,node_id,project_id,branch_id,parent_asset_id,chapter_id,position,reason,digest,idempotency_key,request_json,receipt_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, assetID, nodeID, r.ProjectID, r.BranchID, r.ParentAssetID, r.ChapterID, p.Position, r.Reason, p.Digest, cmd.IdempotencyKey, string(requestJSON), string(encoded), formatTime(now))
	if err != nil {
		return p, fmt.Errorf("record future outline introduction: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return p, fmt.Errorf("commit future outline node: %w", err)
	}
	return p, nil
}
