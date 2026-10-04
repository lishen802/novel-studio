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
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,position FROM formal_outline_retirements WHERE project_id=? UNION ALL SELECT node_id,position FROM formal_outline_origin_retirements WHERE project_id=?`, projectID, projectID)
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
	if r.ProjectID == "" || r.BranchID == "" || r.AssetID == "" || r.ChapterID == "" || strings.TrimSpace(r.Reason) == "" || utf8.RuneCountInString(r.Reason) > 2000 || len(r.UnlinkStoryLineID) > 200 || len(r.UnlinkStoryLineIDs) == 1 || len(r.UnlinkStoryLineIDs) > 100 || (r.UnlinkStoryLineID != "" && len(r.UnlinkStoryLineIDs) != 0) {
		return p, asset.ErrChangeUnsupported
	}
	for i, id := range r.UnlinkStoryLineIDs {
		if id == "" || len(id) > 200 || (i > 0 && id <= r.UnlinkStoryLineIDs[i-1]) {
			return p, asset.ErrChangeUnsupported
		}
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
	var hasArchiveSource int
	if err := q.QueryRowContext(ctx, `SELECT a.typed_id,COALESCE(i.position,0),a.status,a.locked,a.stale,n.level,n.title,EXISTS(SELECT 1 FROM creation_candidates c JOIN creation_sessions s ON s.id=c.session_id WHERE c.id=a.source_candidate_id AND c.kind='roadmap' AND s.project_id=a.project_id AND s.branch_id=?) FROM story_assets a JOIN outline_nodes n ON n.id=a.typed_id AND n.project_id=a.project_id AND n.deleted_at IS NULL LEFT JOIN formal_outline_introductions i ON i.asset_id=a.id AND i.project_id=a.project_id AND i.branch_id=? WHERE a.id=? AND a.project_id=? AND a.kind='outline_node' AND a.typed_table='outline_nodes'`, r.BranchID, r.BranchID, r.AssetID, r.ProjectID).Scan(&p.NodeID, &p.IntroducedAt, &status, &locked, &stale, &level, &p.Title, &hasArchiveSource); err != nil || level != "chapter" || status != "canonical" || locked != 0 || stale != 0 || p.Position <= p.IntroducedAt || (p.IntroducedAt == 0 && hasArchiveSource == 0) {
		return p, asset.ErrChangeConflict
	}
	if p.IntroducedAt == 0 {
		var otherIntroduction int
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM formal_outline_introductions WHERE asset_id=?`, r.AssetID).Scan(&otherIntroduction); err != nil || otherIntroduction != 0 {
			return p, asset.ErrChangeConflict
		}
	}
	var previous int
	if err := q.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM formal_outline_retirements WHERE asset_id=?)+(SELECT COUNT(*) FROM formal_outline_origin_retirements WHERE asset_id=?)`, r.AssetID, r.AssetID).Scan(&previous); err != nil || previous != 0 {
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
	references, err := formalOutlineReferences(ctx, q, r.ProjectID, r.BranchID, r.AssetID)
	if err != nil {
		return p, asset.ErrChangeConflict
	}
	if r.UnlinkStoryLineID == "" && len(r.UnlinkStoryLineIDs) == 0 {
		if len(references.References) != 0 {
			return p, asset.ErrChangeConflict
		}
	} else {
		ids := r.UnlinkStoryLineIDs
		if r.UnlinkStoryLineID != "" {
			ids = []string{r.UnlinkStoryLineID}
		}
		if len(references.References) != len(ids) {
			return p, asset.ErrChangeConflict
		}
		for i, reference := range references.References {
			if reference.Kind != "story_line" || reference.ID != ids[i] {
				return p, asset.ErrChangeConflict
			}
		}
		var allWritten int
		if err = q.QueryRowContext(ctx, `SELECT COUNT(*) FROM chapters c WHERE c.project_id=? AND (c.body<>'' OR EXISTS(SELECT 1 FROM chapter_revisions h WHERE h.chapter_id=c.id AND h.body<>'') OR EXISTS(SELECT 1 FROM drafts d WHERE d.chapter_id=c.id AND d.body<>''))`, r.ProjectID).Scan(&allWritten); err != nil || allWritten != 0 {
			return p, asset.ErrChangeConflict
		}
		for _, id := range ids {
			unlink := asset.FormalStoryLineUnlink{StoryLineID: id}
			if err = q.QueryRowContext(ctx, `SELECT l.position,l.created_at,s.name FROM story_line_links l JOIN story_lines s ON s.id=l.story_line_id AND s.project_id=l.project_id WHERE l.project_id=? AND l.story_line_id=? AND l.outline_node_id=?`, r.ProjectID, id, p.NodeID).Scan(&unlink.Position, &unlink.CreatedAt, &unlink.Name); err != nil {
				return p, asset.ErrChangeConflict
			}
			if r.UnlinkStoryLineID != "" {
				p.UnlinkedStoryLine = &unlink
			} else {
				p.UnlinkedStoryLines = append(p.UnlinkedStoryLines, unlink)
			}
		}
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

// ConfirmFormalOutlineRetire 在事务中重算摘要，一并解除选定链接并记录停用回执。
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
	err = tx.QueryRowContext(ctx, `SELECT request_json,digest,receipt_json FROM formal_outline_retirements WHERE project_id=? AND branch_id=? AND idempotency_key=? UNION ALL SELECT request_json,digest,receipt_json FROM formal_outline_origin_retirements WHERE project_id=? AND branch_id=? AND idempotency_key=?`, command.Request.ProjectID, command.Request.BranchID, command.IdempotencyKey, command.Request.ProjectID, command.Request.BranchID, command.IdempotencyKey).Scan(&oldRequest, &oldDigest, &receipt)
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
	links := p.UnlinkedStoryLines
	if p.UnlinkedStoryLine != nil {
		links = []asset.FormalStoryLineUnlink{*p.UnlinkedStoryLine}
	}
	for _, link := range links {
		result, e := tx.ExecContext(ctx, `DELETE FROM story_line_links WHERE project_id=? AND story_line_id=? AND outline_node_id=? AND position=? AND created_at=?`, command.Request.ProjectID, link.StoryLineID, p.NodeID, link.Position, link.CreatedAt)
		if e != nil {
			return p, fmt.Errorf("unlink retired outline story line: %w", e)
		}
		if e = requireOneRow(result, "story line link", link.StoryLineID); e != nil {
			return p, e
		}
	}
	encoded, _ := json.Marshal(p)
	table := "formal_outline_retirements"
	if p.IntroducedAt == 0 {
		table = "formal_outline_origin_retirements"
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO `+table+`(asset_id,node_id,project_id,branch_id,chapter_id,position,reason,digest,idempotency_key,request_json,receipt_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, command.Request.AssetID, p.NodeID, command.Request.ProjectID, command.Request.BranchID, command.Request.ChapterID, p.Position, command.Request.Reason, p.Digest, command.IdempotencyKey, string(requestJSON), string(encoded), formatTime(now))
	if err != nil {
		return p, fmt.Errorf("record future outline retirement: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return p, fmt.Errorf("commit future outline retirement: %w", err)
	}
	return p, nil
}
