package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// PlanningArchiveAssets 读取首次入档与当前正式版本，用于唯一稳定身份匹配。
func (s *Store) PlanningArchiveAssets(ctx context.Context, projectID string) ([]asset.PlanningArchiveAsset, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.kind,a.typed_id,COALESCE(a.source_candidate_id,''),o.payload_json,r.payload_json,a.current_revision
 FROM story_assets a JOIN story_asset_revisions o ON o.asset_id=a.id AND o.revision=1
 JOIN story_asset_revisions r ON r.asset_id=a.id AND r.revision=a.current_revision
 WHERE a.project_id=? AND a.status='canonical' AND a.kind IN ('proposal','outline_node') ORDER BY a.kind,a.id`, projectID)
	if err != nil {
		return nil, fmt.Errorf("read planning archive assets: %w", err)
	}
	defer rows.Close()
	out := []asset.PlanningArchiveAsset{}
	for rows.Next() {
		var item asset.PlanningArchiveAsset
		if err = rows.Scan(&item.AssetID, &item.Kind, &item.TypedID, &item.SourceCandidateID, &item.Original, &item.Current, &item.Revision); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// PlanningArchivedPeopleCandidate 核对人物仍指向首次入档的同一候选版本。
func (s *Store) PlanningArchivedPeopleCandidate(ctx context.Context, projectID string) (string, error) {
	var count int
	var candidate string
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT COALESCE(source_candidate_id,'')),COALESCE(MIN(source_candidate_id),'') FROM story_assets WHERE project_id=? AND kind='character' AND status IN ('canonical','stale')`, projectID).Scan(&count, &candidate); err != nil {
		return "", err
	}
	if count != 1 || candidate == "" {
		return "", asset.ErrChangeConflict
	}
	return candidate, nil
}

// PlanningRearchiveReceipt 在重试时读取已提交回执，不受后续正文或候选变化影响。
func (s *Store) PlanningRearchiveReceipt(ctx context.Context, r asset.PlanningRearchiveRequest, digest, key string) (asset.PlanningRearchivePreview, bool, error) {
	var request, stored, receipt string
	err := s.db.QueryRowContext(ctx, `SELECT request_json,digest,receipt_json FROM planning_rearchives WHERE project_id=? AND branch_id=? AND idempotency_key=?`, r.ProjectID, r.BranchID, key).Scan(&request, &stored, &receipt)
	if errors.Is(err, sql.ErrNoRows) {
		return asset.PlanningRearchivePreview{}, false, nil
	}
	if err != nil {
		return asset.PlanningRearchivePreview{}, false, err
	}
	encoded, _ := json.Marshal(r)
	if request != string(encoded) || stored != digest || digest == "" {
		return asset.PlanningRearchivePreview{}, false, asset.ErrChangeConflict
	}
	var p asset.PlanningRearchivePreview
	if err = json.Unmarshal([]byte(receipt), &p); err != nil {
		return p, false, err
	}
	return p, true, nil
}

// ApplyPlanningRearchive 只追加预览列出的正式资产版本；有待处理操作时整组拒绝。
func (s *Store) ApplyPlanningRearchive(ctx context.Context, input asset.PlanningRearchiveApply, now time.Time) (asset.PlanningRearchivePreview, error) {
	p := input.Preview
	if input.Key == "" || len(input.Key) > 200 || p.Digest == "" || len(p.Changes) == 0 || len(p.Changes) > 400 || len(input.Guards) != 3 {
		return p, asset.ErrChangeConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return p, err
	}
	defer tx.Rollback()
	requestJSON, _ := json.Marshal(p.Request)
	var oldRequest, oldDigest, receipt string
	err = tx.QueryRowContext(ctx, `SELECT request_json,digest,receipt_json FROM planning_rearchives WHERE project_id=? AND branch_id=? AND idempotency_key=?`, p.Request.ProjectID, p.Request.BranchID, input.Key).Scan(&oldRequest, &oldDigest, &receipt)
	if err == nil {
		var old asset.PlanningRearchivePreview
		if oldRequest != string(requestJSON) || oldDigest != p.Digest || json.Unmarshal([]byte(receipt), &old) != nil {
			return p, asset.ErrChangeConflict
		}
		return old, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return p, err
	}
	var pending int
	if err = tx.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM assistant_authorizations a JOIN assistant_sessions s ON s.id=a.session_id WHERE s.project_id=? AND a.state IN ('pending_confirmation','active'))+(SELECT COUNT(*) FROM assistant_actions WHERE project_id=? AND state IN ('pending_confirmation','executing','outcome_unknown'))+(SELECT COUNT(*) FROM change_operations o JOIN change_sets c ON c.id=o.change_set_id WHERE c.project_id=? AND o.status='pending')`, p.Request.ProjectID, p.Request.ProjectID, p.Request.ProjectID).Scan(&pending); err != nil || pending != 0 {
		return p, asset.ErrChangeConflict
	}
	var sessionID string
	for index, guard := range input.Guards {
		var hash, status, candidateSession, stage string
		var version int64
		if err = tx.QueryRowContext(ctx, `SELECT c.content_sha256,c.status,c.session_id,c.stage,COALESCE(r.review_version,0) FROM creation_candidates c LEFT JOIN creation_candidate_reviews r ON r.candidate_id=c.id WHERE c.id=?`, guard.CandidateID).Scan(&hash, &status, &candidateSession, &stage, &version); err != nil {
			return p, asset.ErrChangeConflict
		}
		allowedStatus := status == "confirmed" || index == 1 && status == "stale"
		if hash != guard.ContentSHA256 || !allowedStatus || version != guard.ReviewVersion || stage != []string{"proposal", "people_world", "roadmap"}[index] || (sessionID != "" && candidateSession != sessionID) {
			return p, asset.ErrChangeConflict
		}
		var newest string
		if err = tx.QueryRowContext(ctx, `SELECT id FROM creation_candidates WHERE session_id=? AND stage=? ORDER BY version DESC,id DESC LIMIT 1`, candidateSession, stage).Scan(&newest); err != nil || newest != guard.CandidateID {
			return p, asset.ErrChangeConflict
		}
		sessionID = candidateSession
	}
	var owner, sessionBranch, archiveID string
	if err = tx.QueryRowContext(ctx, `SELECT project_id,branch_id,archive_change_set_id FROM creation_sessions WHERE id=?`, sessionID).Scan(&owner, &sessionBranch, &archiveID); err != nil || owner != p.Request.ProjectID || sessionBranch != p.Request.BranchID || archiveID == "" {
		return p, asset.ErrChangeConflict
	}
	var peopleSources int
	var peopleSource string
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(DISTINCT COALESCE(source_candidate_id,'')),COALESCE(MIN(source_candidate_id),'') FROM story_assets WHERE project_id=? AND kind='character' AND status IN ('canonical','stale')`, p.Request.ProjectID).Scan(&peopleSources, &peopleSource); err != nil || peopleSources != 1 || peopleSource != input.Guards[1].CandidateID {
		return p, asset.ErrChangeConflict
	}
	if len(input.Assets) == 0 {
		return p, asset.ErrChangeConflict
	}
	for _, source := range input.Assets {
		var candidate, original, current string
		var revision int64
		err = tx.QueryRowContext(ctx, `SELECT COALESCE(a.source_candidate_id,''),o.payload_json,r.payload_json,a.current_revision FROM story_assets a JOIN story_asset_revisions o ON o.asset_id=a.id AND o.revision=1 JOIN story_asset_revisions r ON r.asset_id=a.id AND r.revision=a.current_revision WHERE a.id=? AND a.project_id=? AND a.kind=? AND a.typed_id=? AND a.status='canonical'`, source.AssetID, p.Request.ProjectID, source.Kind, source.TypedID).Scan(&candidate, &original, &current, &revision)
		if err != nil || candidate != source.SourceCandidateID || original != source.Original || current != source.Current || revision != source.Revision {
			return p, asset.ErrChangeConflict
		}
	}
	validated := make([]constraintRow, 0, len(p.Changes))
	seen := map[string]bool{}
	for _, change := range p.Changes {
		if seen[change.AssetID] || change.Request.ProjectID != p.Request.ProjectID || change.Request.BranchID != p.Request.BranchID || change.Request.ChapterID != p.Request.ChapterID || change.Request.Reason != p.Request.Reason || change.Request.AssetID != change.AssetID {
			return p, asset.ErrChangeConflict
		}
		seen[change.AssetID] = true
		fresh, row, e := previewConstraint(ctx, tx, change.Request)
		if e != nil || !fresh.CanApply || fresh.Digest != change.Digest || fresh.Before != change.Before || fresh.Request.Text != change.After || fresh.Position != p.Position {
			return p, asset.ErrChangeConflict
		}
		validated = append(validated, row)
	}
	for i, change := range p.Changes {
		row := validated[i]
		updated, _, e := asset.EditPlanning(row.Kind, row.payload, change.Request.Text)
		if e != nil {
			return p, e
		}
		var payload map[string]json.RawMessage
		if json.Unmarshal([]byte(updated), &payload) != nil {
			return p, asset.ErrChangeUnsupported
		}
		payload["_planningRearchive"], _ = json.Marshal(map[string]string{"groupId": p.Digest, "reason": p.Request.Reason})
		encoded, _ := json.Marshal(payload)
		sum := sha256.Sum256(encoded)
		revision := row.Revision + 1
		if _, e = tx.ExecContext(ctx, `INSERT INTO story_asset_revisions(asset_id,revision,payload_json,content_sha256,created_by,created_at) VALUES(?,?,?,?,?,?)`, row.AssetID, revision, string(encoded), hex.EncodeToString(sum[:]), "planning-candidate-future-only", formatTime(now)); e != nil {
			return p, e
		}
		result, e := tx.ExecContext(ctx, `UPDATE story_assets SET current_revision=?,updated_at=? WHERE id=? AND project_id=? AND current_revision=? AND locked=0 AND stale=0 AND status='canonical'`, revision, formatTime(now), row.AssetID, p.Request.ProjectID, row.Revision)
		if e != nil {
			return p, e
		}
		if e = requireOneRow(result, "planning asset", row.AssetID); e != nil {
			return p, e
		}
		receiptJSON, _ := json.Marshal(p)
		if _, e = tx.ExecContext(ctx, `INSERT INTO story_constraint_changes(project_id,asset_id,revision,chapter_id,position,reason,digest,idempotency_key,request_json,receipt_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, p.Request.ProjectID, row.AssetID, revision, p.Request.ChapterID, p.Position, p.Request.Reason, change.Digest, p.Digest+":"+row.AssetID, string(requestJSON), string(receiptJSON), formatTime(now)); e != nil {
			return p, e
		}
	}
	encoded, _ := json.Marshal(p)
	if _, err = tx.ExecContext(ctx, `INSERT INTO planning_rearchives(id,project_id,branch_id,chapter_id,position,reason,digest,idempotency_key,request_json,receipt_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, p.Digest, p.Request.ProjectID, p.Request.BranchID, p.Request.ChapterID, p.Position, p.Request.Reason, p.Digest, input.Key, string(requestJSON), string(encoded), formatTime(now)); err != nil {
		return p, err
	}
	if err = tx.Commit(); err != nil {
		return p, fmt.Errorf("commit planning rearchive: %w", err)
	}
	return p, nil
}
