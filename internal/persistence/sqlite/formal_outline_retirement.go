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

// FormalOutlineRetirements 返回未来停用节点的章序，供档案显示时间边界。
func (s *Store) FormalOutlineRetirements(ctx context.Context, projectID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,position FROM formal_outline_retirements WHERE project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var nodeID string
		var position int
		if err = rows.Scan(&nodeID, &position); err != nil {
			return nil, err
		}
		out[nodeID] = position
	}
	return out, rows.Err()
}

// PreviewFormalOutlineRetire 核对引用与未来空章，生成不得换绑的停用摘要。
func (s *Store) PreviewFormalOutlineRetire(ctx context.Context, r asset.FormalOutlineRetireRequest) (asset.FormalOutlineRetirePreview, error) {
	return previewFormalOutlineRetire(ctx, s.db, r)
}

func previewFormalOutlineRetire(ctx context.Context, q constraintReader, r asset.FormalOutlineRetireRequest) (asset.FormalOutlineRetirePreview, error) {
	p := asset.FormalOutlineRetirePreview{Request: r}
	if r.ProjectID == "" || r.BranchID == "" || r.AssetID == "" || r.ChapterID == "" || strings.TrimSpace(r.Reason) == "" || utf8.RuneCountInString(r.Reason) > 2000 {
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
	var status, level string
	var locked, stale int
	if err := q.QueryRowContext(ctx, `SELECT i.node_id,i.position,a.status,a.locked,a.stale,n.level,n.title FROM formal_outline_introductions i JOIN story_assets a ON a.id=i.asset_id JOIN outline_nodes n ON n.id=i.node_id AND n.project_id=i.project_id AND n.deleted_at IS NULL WHERE i.asset_id=? AND i.project_id=? AND i.branch_id=?`, r.AssetID, r.ProjectID, r.BranchID).Scan(&p.NodeID, &p.IntroducedAt, &status, &locked, &stale, &level, &p.Title); err != nil || level != "chapter" || status != "canonical" || locked != 0 || stale != 0 || p.Position <= p.IntroducedAt {
		return p, asset.ErrChangeConflict
	}
	var previous int
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM formal_outline_retirements WHERE asset_id=?`, r.AssetID).Scan(&previous); err != nil || previous != 0 {
		return p, asset.ErrChangeConflict
	}
	rows, err := planningConstraintRows(ctx, q)
	if err != nil {
		return p, err
	}
	valid := false
	for _, row := range rows {
		if row.AssetID == r.AssetID {
			valid = !row.Locked && !row.stale
		}
	}
	if !valid {
		return p, asset.ErrChangeConflict
	}
	var last int
	if err = q.QueryRowContext(ctx, `SELECT COALESCE(MAX(position),0) FROM story_constraint_changes WHERE asset_id=?`, r.AssetID).Scan(&last); err != nil || p.Position <= last {
		return p, asset.ErrChangeConflict
	}
	var refs int
	if err = q.QueryRowContext(ctx, `SELECT
 (SELECT COUNT(*) FROM outline_nodes WHERE parent_id=? AND deleted_at IS NULL)+
 (SELECT COUNT(*) FROM story_line_links WHERE outline_node_id=?)+
 (SELECT COUNT(*) FROM scene_cards WHERE outline_node_id=? AND deleted_at IS NULL)+
 (SELECT COUNT(*) FROM story_anchors WHERE project_id=? AND scope_kind='roadmap_node' AND scope_id IN (?,?))+
 (SELECT COUNT(*) FROM story_asset_revisions v JOIN story_assets a ON a.id=v.asset_id AND a.current_revision=v.revision WHERE a.project_id=? AND a.id<>? AND EXISTS(SELECT 1 FROM json_tree(v.payload_json) j WHERE j.type='text' AND j.value IN (?,?)))`, p.NodeID, p.NodeID, p.NodeID, r.ProjectID, p.NodeID, r.AssetID, r.ProjectID, r.AssetID, p.NodeID, r.AssetID).Scan(&refs); err != nil || refs != 0 {
		return p, asset.ErrChangeConflict
	}
	var pending int
	if err = q.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM assistant_authorizations a JOIN assistant_sessions s ON s.id=a.session_id WHERE s.project_id=? AND a.state IN ('pending_confirmation','active'))+(SELECT COUNT(*) FROM assistant_actions WHERE project_id=? AND state IN ('pending_confirmation','executing','outcome_unknown'))+(SELECT COUNT(*) FROM change_operations o JOIN change_sets c ON c.id=o.change_set_id WHERE c.project_id=? AND o.status='pending')+(SELECT COUNT(*) FROM agent_runs WHERE project_id=? AND status IN ('queued','running'))`, r.ProjectID, r.ProjectID, r.ProjectID, r.ProjectID).Scan(&pending); err != nil || pending != 0 {
		return p, asset.ErrChangeConflict
	}
	fingerprint, err := constraintFingerprint(ctx, q, asset.ConstraintRequest{ProjectID: r.ProjectID, BranchID: r.BranchID, AssetID: r.AssetID, ChapterID: r.ChapterID, Intent: "future_only", Reason: r.Reason})
	if err != nil {
		return p, err
	}
	encoded, _ := json.Marshal(struct {
		Preview     asset.FormalOutlineRetirePreview
		Fingerprint string
		Last        int
	}{p, fingerprint, last})
	sum := sha256.Sum256(encoded)
	p.Digest = hex.EncodeToString(sum[:])
	return p, nil
}

// ConfirmFormalOutlineRetire 在事务中重算摘要，只追加停用边界和成功回执。
func (s *Store) ConfirmFormalOutlineRetire(ctx context.Context, command asset.FormalOutlineRetireCommand, now time.Time) (asset.FormalOutlineRetirePreview, error) {
	if command.IdempotencyKey == "" || len(command.IdempotencyKey) > 200 || command.Digest == "" {
		return asset.FormalOutlineRetirePreview{}, asset.ErrChangeConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return asset.FormalOutlineRetirePreview{}, err
	}
	defer tx.Rollback()
	requestJSON, _ := json.Marshal(command.Request)
	var oldRequest, oldDigest, receipt string
	err = tx.QueryRowContext(ctx, `SELECT request_json,digest,receipt_json FROM formal_outline_retirements WHERE project_id=? AND branch_id=? AND idempotency_key=?`, command.Request.ProjectID, command.Request.BranchID, command.IdempotencyKey).Scan(&oldRequest, &oldDigest, &receipt)
	if err == nil {
		var previous asset.FormalOutlineRetirePreview
		if oldRequest != string(requestJSON) || oldDigest != command.Digest || json.Unmarshal([]byte(receipt), &previous) != nil {
			return previous, asset.ErrChangeConflict
		}
		return previous, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return asset.FormalOutlineRetirePreview{}, err
	}
	p, err := previewFormalOutlineRetire(ctx, tx, command.Request)
	if err != nil || p.Digest != command.Digest {
		return p, asset.ErrChangeConflict
	}
	encoded, _ := json.Marshal(p)
	_, err = tx.ExecContext(ctx, `INSERT INTO formal_outline_retirements(asset_id,node_id,project_id,branch_id,chapter_id,position,reason,digest,idempotency_key,request_json,receipt_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, command.Request.AssetID, p.NodeID, command.Request.ProjectID, command.Request.BranchID, command.Request.ChapterID, p.Position, command.Request.Reason, p.Digest, command.IdempotencyKey, string(requestJSON), string(encoded), formatTime(now))
	if err != nil {
		return p, fmt.Errorf("record future outline retirement: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return p, fmt.Errorf("commit future outline retirement: %w", err)
	}
	return p, nil
}
