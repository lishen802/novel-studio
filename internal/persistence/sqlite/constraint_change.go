package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
	"strings"
	"time"
	"unicode/utf8"
)

type constraintReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type constraintRow struct {
	asset.ConstraintView
	payload   string
	typedID   string
	scopeKind string
	scopeID   string
	stale     bool
}

// constraintRows 使用版本载荷作为正式基线，原始 typed 行只用于校验身份和作用域。
func constraintRows(ctx context.Context, q constraintReader) ([]constraintRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT a.id,a.kind,a.current_revision,a.locked,a.stale,a.typed_id,
 COALESCE(w.name,h.statement,a.kind),COALESCE(w.description,h.statement,''),r.payload_json,
 COALESCE(h.scope_kind,w.scope_type,''),COALESCE(h.scope_id,w.scope_id,'')
 FROM story_assets a JOIN story_asset_revisions r ON r.asset_id=a.id AND r.revision=a.current_revision
 LEFT JOIN world_rules w ON a.kind='world_rule' AND a.typed_table='world_rules' AND w.id=a.typed_id AND w.project_id=a.project_id
 LEFT JOIN story_anchors h ON h.asset_id=a.id AND h.project_id=a.project_id
 WHERE a.status='canonical' AND ((a.kind='world_rule' AND w.id IS NOT NULL) OR
 (a.kind IN ('hard_anchor','soft_anchor','forbidden') AND h.id IS NOT NULL)) ORDER BY a.kind,a.id`)
	if err != nil {
		return nil, fmt.Errorf("list formal constraints: %w", err)
	}
	defer rows.Close()
	result := []constraintRow{}
	for rows.Next() {
		var r constraintRow
		var locked, stale int
		if err = rows.Scan(&r.AssetID, &r.Kind, &r.Revision, &locked, &stale, &r.typedID, &r.Name, &r.Original, &r.payload, &r.scopeKind, &r.scopeID); err != nil {
			return nil, err
		}
		r.Locked = locked == 1
		r.stale = stale == 1
		r.Current = constraintText(r.Kind, r.payload)
		r.Changes = []asset.ConstraintChange{}
		if r.Current == "" {
			return nil, asset.ErrChangeUnsupported
		}
		result = append(result, r)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	characters, err := characterConstraintRows(ctx, q)
	if err != nil {
		return nil, err
	}
	planning, err := planningConstraintRows(ctx, q)
	if err != nil {
		return nil, err
	}
	return append(append(result, characters...), planning...), nil
}
func constraintText(kind, payload string) string {
	if kind == "proposal" || kind == "outline_node" {
		fields, err := asset.PlanningFields(kind, payload)
		if err != nil {
			return ""
		}
		values := map[string]string{}
		for _, field := range fields {
			values[field.Path] = field.Value
		}
		encoded, _ := json.Marshal(values)
		return string(encoded)
	}
	if kind == "character" {
		var profile asset.CharacterProfile
		if json.Unmarshal([]byte(payload), &profile) != nil {
			return ""
		}
		normalized, err := json.Marshal(profile)
		if err != nil {
			return ""
		}
		canonical, e := asset.ParseCharacterProfile(string(normalized))
		if e != nil {
			return ""
		}
		normalized, _ = json.Marshal(canonical)
		return string(normalized)
	}
	var v map[string]json.RawMessage
	if json.Unmarshal([]byte(payload), &v) != nil {
		return ""
	}
	key := "statement"
	if kind == "world_rule" {
		key = "description"
	}
	var text string
	_ = json.Unmarshal(v[key], &text)
	return text
}

// Constraints 同时返回原设定与修订序列，不将未来版本回写到原档案。
func (s *Store) Constraints(ctx context.Context) ([]asset.ConstraintView, error) {
	return s.constraintHistory(ctx, "constraint")
}

// CharacterRevisions 提供原始人物与按章修订历史，不把未来档案回写为全书事实。
func (s *Store) CharacterRevisions(ctx context.Context) ([]asset.ConstraintView, error) {
	return s.constraintHistory(ctx, "character")
}

func (s *Store) constraintHistory(ctx context.Context, kind string) ([]asset.ConstraintView, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := constraintRows(ctx, tx)
	if err != nil {
		return nil, err
	}
	changes, err := tx.QueryContext(ctx, `SELECT c.asset_id,c.revision,c.chapter_id,c.position,c.reason,r.payload_json,a.kind FROM story_constraint_changes c JOIN story_asset_revisions r ON r.asset_id=c.asset_id AND r.revision=c.revision JOIN story_assets a ON a.id=c.asset_id ORDER BY c.asset_id,c.position`)
	if err != nil {
		return nil, err
	}
	defer changes.Close()
	index := map[string]int{}
	for i, r := range rows {
		index[r.AssetID] = i
	}
	for changes.Next() {
		var c asset.ConstraintChange
		var payload, kind string
		if err = changes.Scan(&c.AssetID, &c.Revision, &c.ChapterID, &c.Position, &c.Reason, &payload, &kind); err != nil {
			return nil, err
		}
		c.Text = constraintText(kind, payload)
		var metadata struct {
			Source characterAdoptionSource `json:"_candidateAdoption"`
		}
		if json.Unmarshal([]byte(payload), &metadata) == nil {
			c.CandidateID = metadata.Source.CandidateID
		}
		if i, ok := index[c.AssetID]; ok {
			rows[i].Changes = append(rows[i].Changes, c)
		}
	}
	if err = changes.Err(); err != nil {
		return nil, err
	}
	out := make([]asset.ConstraintView, 0, len(rows))
	for _, r := range rows {
		if (kind == "character" && r.Kind == "character") || (kind == "planning" && (r.Kind == "proposal" || r.Kind == "outline_node")) || (kind == "constraint" && r.Kind != "character" && r.Kind != "proposal" && r.Kind != "outline_node") {
			r.TypedID = r.typedID
			if kind == "character" {
				r.CandidateSuggestions, err = characterSuggestions(ctx, tx, r)
				if err != nil {
					return nil, err
				}
			}
			out = append(out, r.ConstraintView)
		}
	}
	return out, nil
}

// EffectiveConstraints 按目标章节选择最后生效的版本，旧章永远读取旧载荷。
func (s *Store) EffectiveConstraints(ctx context.Context, position int) ([]asset.EffectiveConstraint, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT a.id,a.kind,a.typed_id,r.revision,r.payload_json,
 COALESCE(h.scope_kind,w.scope_type,''),COALESCE(h.scope_id,w.scope_id,'')
 FROM story_assets a JOIN story_asset_revisions r ON r.asset_id=a.id AND r.revision=COALESCE(
 (SELECT c.revision FROM story_constraint_changes c WHERE c.asset_id=a.id AND c.position<=? ORDER BY c.position DESC LIMIT 1),1)
 LEFT JOIN world_rules w ON a.kind='world_rule' AND w.id=a.typed_id
 LEFT JOIN story_anchors h ON h.asset_id=a.id
 LEFT JOIN formal_outline_introductions intro ON intro.asset_id=a.id
 WHERE a.status='canonical' AND (intro.asset_id IS NULL OR intro.position<=?) AND a.kind IN ('world_rule','hard_anchor','soft_anchor','forbidden','character','proposal','outline_node') ORDER BY a.id`, position, position)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []asset.EffectiveConstraint{}
	for rows.Next() {
		var v asset.EffectiveConstraint
		var payload string
		if err = rows.Scan(&v.AssetID, &v.Kind, &v.TypedID, &v.Revision, &payload, &v.ScopeKind, &v.ScopeID); err != nil {
			return nil, err
		}
		v.Text = constraintText(v.Kind, payload)
		out = append(out, v)
	}
	return out, rows.Err()
}

func validateConstraintRequest(r asset.ConstraintRequest) error {
	if r.Intent != "future_only" {
		return asset.ErrChangeUnsupported
	}
	if r.ProjectID == "" || r.AssetID == "" || r.ChapterID == "" || strings.TrimSpace(r.Text) == "" || strings.TrimSpace(r.Reason) == "" || utf8.RuneCountInString(r.Text) > 48000 || utf8.RuneCountInString(r.Reason) > 2000 {
		return asset.ErrChangeUnsupported
	}
	return nil
}

// constraintFingerprint 冻结章节、恢复输入、所有设定版本和待处理候选，确认时不能偷换基线。
func constraintFingerprint(ctx context.Context, q constraintReader, r asset.ConstraintRequest) (string, error) {
	queries := []string{
		`SELECT id,title,position,body,revision FROM chapters ORDER BY id`,
		`SELECT chapter_id,revision,title,body FROM chapter_revisions ORDER BY chapter_id,revision`,
		`SELECT chapter_id,body,base_revision FROM drafts ORDER BY chapter_id`,
		`SELECT id,current_revision,locked,stale,status FROM story_assets ORDER BY id`,
		`SELECT asset_id,revision,payload_json FROM story_asset_revisions ORDER BY asset_id,revision`,
		`SELECT id,name,aliases_json,role,summary,motivation,fear,secret,arc,status,deleted_at FROM characters ORDER BY id`,
		`SELECT id,lock_map_json FROM creation_candidates ORDER BY id`,
		`SELECT id,version,status,content_sha256 FROM creation_candidates ORDER BY id`,
		`SELECT id,status,after_value FROM change_operations ORDER BY id`,
		`SELECT id,parent_id,level,title,summary,goal,conflict,outcome,position,chapter_id,planning_json,deleted_at FROM outline_nodes ORDER BY id`,
	}
	h := sha256.New()
	encoded, _ := json.Marshal(r)
	h.Write(encoded)
	for _, query := range queries {
		rows, err := q.QueryContext(ctx, query)
		if err != nil {
			return "", err
		}
		cols, err := rows.Columns()
		if err != nil {
			rows.Close()
			return "", err
		}
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		for rows.Next() {
			if err = rows.Scan(pointers...); err != nil {
				rows.Close()
				return "", err
			}
			encoded, err = json.Marshal(values)
			if err != nil {
				rows.Close()
				return "", err
			}
			h.Write(encoded)
			h.Write([]byte{'\n'})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// previewConstraint 核对可执行边界并生成不可换绑的确认摘要。
func previewConstraint(ctx context.Context, q constraintReader, r asset.ConstraintRequest) (asset.ConstraintPreview, constraintRow, error) {
	p := asset.ConstraintPreview{Request: r}
	var adoptionSource characterAdoptionSource
	if err := validateConstraintRequest(r); err != nil {
		return p, constraintRow{}, err
	}
	var projectID string
	if err := q.QueryRowContext(ctx, `SELECT project_id FROM chapters WHERE id=?`, r.ChapterID).Scan(&projectID); err != nil || projectID != r.ProjectID {
		return p, constraintRow{}, asset.ErrChangeUnsupported
	}
	if err := q.QueryRowContext(ctx, `SELECT position FROM chapters WHERE id=?`, r.ChapterID).Scan(&p.Position); err != nil {
		return p, constraintRow{}, err
	}
	rows, err := constraintRows(ctx, q)
	if err != nil {
		return p, constraintRow{}, err
	}
	var selected constraintRow
	for _, row := range rows {
		if row.AssetID == r.AssetID {
			selected = row
		}
	}
	var owner string
	err = q.QueryRowContext(ctx, `SELECT project_id FROM story_assets WHERE id=?`, r.AssetID).Scan(&owner)
	if err != nil || owner != r.ProjectID || selected.AssetID == "" {
		return p, selected, asset.ErrChangeUnsupported
	}
	if r.CandidateID != "" {
		if selected.Kind != "character" {
			return p, selected, asset.ErrChangeUnsupported
		}
		source, e := characterAdoption(ctx, q, selected, r.CandidateID, r.ProjectID, r.BranchID)
		adoptionSource = source
		if e != nil {
			return p, selected, e
		}
		profile, e := asset.ParseCharacterProfile(r.Text)
		if e != nil {
			return p, selected, asset.ErrChangeConflict
		}
		encoded, _ := json.Marshal(profile)
		if string(encoded) != source.Text {
			return p, selected, asset.ErrChangeConflict
		}
	}
	if selected.Kind == "character" {
		profile, e := asset.ParseCharacterProfile(r.Text)
		if e != nil {
			return p, selected, e
		}
		encoded, _ := json.Marshal(profile)
		p.Request.Text = string(encoded)
	} else if selected.Kind == "proposal" || selected.Kind == "outline_node" {
		_, normalized, e := asset.EditPlanning(selected.Kind, selected.payload, r.Text)
		if e != nil {
			return p, selected, e
		}
		p.Request.Text = normalized
	} else if utf8.RuneCountInString(r.Text) > 12000 {
		return p, selected, asset.ErrChangeUnsupported
	}
	p.Before = selected.Current
	p.Revision = selected.Revision + 1
	p.CanApply = true
	if selected.Locked || selected.stale {
		p.CanApply = false
		p.BlockedReason = "该资产或来源人物字段已锁定，或资产已失效，不能修订"
	}
	if strings.TrimSpace(p.Request.Text) == strings.TrimSpace(p.Before) {
		p.CanApply = false
		p.BlockedReason = "新旧语句相同"
	}
	var last int
	err = q.QueryRowContext(ctx, `SELECT COALESCE(MAX(position),0) FROM story_constraint_changes WHERE asset_id=?`, r.AssetID).Scan(&last)
	if err != nil {
		return p, selected, err
	}
	if p.Position <= last {
		p.CanApply = false
		p.BlockedReason = "生效章节必须晚于该约束已有的生效边界"
	}
	if selected.Kind == "outline_node" {
		var introducedAt int
		err = q.QueryRowContext(ctx, `SELECT COALESCE((SELECT position FROM formal_outline_introductions WHERE asset_id=?),0)`, r.AssetID).Scan(&introducedAt)
		if err != nil {
			return p, selected, err
		}
		if introducedAt > 0 && p.Position < introducedAt {
			p.CanApply = false
			p.BlockedReason = "生效章节不能早于该节点的新增边界"
		}
	}
	var count int
	err = q.QueryRowContext(ctx, `SELECT count(*) FROM chapters c WHERE c.position>=? AND (c.body<>'' OR EXISTS(SELECT 1 FROM chapter_revisions h WHERE h.chapter_id=c.id AND h.body<>'') OR EXISTS(SELECT 1 FROM drafts d WHERE d.chapter_id=c.id AND d.body<>''))`, p.Position).Scan(&count)
	if err != nil {
		return p, selected, err
	}
	if count > 0 {
		p.CanApply = false
		p.BlockedReason = "生效范围已有正文、历史正文或恢复草稿，请选择更晚的空章节"
	}
	err = q.QueryRowContext(ctx, `SELECT count(*) FROM chapters WHERE position<?`, p.Position).Scan(&p.ProtectedChapters)
	if err != nil {
		return p, selected, err
	}
	err = q.QueryRowContext(ctx, `SELECT count(*) FROM chapters WHERE position>=?`, p.Position).Scan(&p.FutureChapters)
	if err != nil {
		return p, selected, err
	}
	var running int
	err = q.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM agent_runs WHERE status IN ('queued','running'))+(SELECT count(*) FROM extraction_tasks WHERE status IN ('queued','running','recoverable'))+(SELECT count(*) FROM creation_sessions WHERE active_run_id<>'')`).Scan(&running)
	if err != nil {
		return p, selected, err
	}
	if running > 0 {
		p.CanApply = false
		p.BlockedReason = "有模型任务正在运行，请等待结束后重新预览"
	}
	p.Digest, err = constraintFingerprint(ctx, q, p.Request)
	if r.CandidateID != "" {
		sourceJSON, _ := json.Marshal(adoptionSource)
		p.Digest = adoptionHash(p.Digest + string(sourceJSON))
	}
	return p, selected, err
}

// PreviewConstraintChange 只读取数据库，不调用模型、不修改正式资产。
func (s *Store) PreviewConstraintChange(ctx context.Context, r asset.ConstraintRequest) (asset.ConstraintPreview, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return asset.ConstraintPreview{}, err
	}
	defer tx.Rollback()
	p, _, err := previewConstraint(ctx, tx, r)
	return p, err
}

// ApplyConstraintChange 在同一事务核对预览、追加版本、标记旧候选失效并保存幂等回执。
func (s *Store) ApplyConstraintChange(ctx context.Context, c asset.ConstraintCommand, now time.Time) (asset.ConstraintPreview, error) {
	if c.IdempotencyKey == "" || len(c.IdempotencyKey) > 200 || c.Digest == "" {
		return asset.ConstraintPreview{}, asset.ErrChangeConflict
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return asset.ConstraintPreview{}, err
	}
	defer tx.Rollback()
	requestJSON, _ := json.Marshal(c.Request)
	var oldRequest, oldDigest, receipt string
	err = tx.QueryRowContext(ctx, `SELECT request_json,digest,receipt_json FROM story_constraint_changes WHERE project_id=? AND idempotency_key=?`, c.Request.ProjectID, c.IdempotencyKey).Scan(&oldRequest, &oldDigest, &receipt)
	if err == nil {
		var p asset.ConstraintPreview
		if oldRequest != string(requestJSON) || oldDigest != c.Digest || json.Unmarshal([]byte(receipt), &p) != nil {
			return p, asset.ErrChangeConflict
		}
		return p, nil
	}
	if err != sql.ErrNoRows {
		return asset.ConstraintPreview{}, err
	}
	p, row, err := previewConstraint(ctx, tx, c.Request)
	if err != nil {
		return p, err
	}
	if !p.CanApply || p.Digest != c.Digest {
		return p, asset.ErrChangeConflict
	}
	var payload map[string]json.RawMessage
	if err = json.Unmarshal([]byte(row.payload), &payload); err != nil {
		return p, err
	}
	if row.Kind == "character" {
		profile, e := asset.ParseCharacterProfile(c.Request.Text)
		if e != nil {
			return p, e
		}
		fields, _ := json.Marshal(profile)
		var updates map[string]json.RawMessage
		_ = json.Unmarshal(fields, &updates)
		for key, value := range updates {
			payload[key] = value
		}
	} else if row.Kind == "proposal" || row.Kind == "outline_node" {
		updated, _, e := asset.EditPlanning(row.Kind, row.payload, c.Request.Text)
		if e != nil {
			return p, e
		}
		if json.Unmarshal([]byte(updated), &payload) != nil {
			return p, asset.ErrChangeUnsupported
		}
	} else {
		key := "statement"
		if row.Kind == "world_rule" {
			key = "description"
		}
		payload[key], _ = json.Marshal(c.Request.Text)
	}
	createdBy := "author-future-only"
	delete(payload, "_candidateAdoption")
	// 来源必须在写入版本的同一事务重新验证，客户端不能自报 AI 审计信息。
	if c.Request.CandidateID != "" {
		source, e := characterAdoption(ctx, tx, row, c.Request.CandidateID, c.Request.ProjectID, c.Request.BranchID)
		if e != nil {
			return p, e
		}
		source.Text = ""
		payload["_candidateAdoption"], _ = json.Marshal(source)
		createdBy = "ai-candidate-future-only"
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	_, err = tx.ExecContext(ctx, `INSERT INTO story_asset_revisions(asset_id,revision,payload_json,content_sha256,created_by,created_at) VALUES(?,?,?,?,?,?)`, row.AssetID, p.Revision, string(encoded), hex.EncodeToString(sum[:]), createdBy, formatTime(now))
	if err != nil {
		return p, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE story_assets SET current_revision=?,updated_at=? WHERE id=? AND current_revision=? AND locked=0 AND stale=0 AND status='canonical'`, p.Revision, formatTime(now), row.AssetID, row.Revision)
	if err != nil {
		return p, err
	}
	if err = requireOneRow(result, "constraint", row.AssetID); err != nil {
		return p, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE creation_candidates SET status='stale',updated_at=? WHERE status IN ('draft','selected','confirmed')`, formatTime(now))
	if err != nil {
		return p, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE creation_sessions SET content_version=content_version+1,updated_at=?`, formatTime(now))
	if err != nil {
		return p, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE assistant_authorizations SET state='stale',updated_at=? WHERE state IN ('pending_confirmation','active')`, formatTime(now))
	if err != nil {
		return p, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE change_operations SET status='conflicted',updated_at=? WHERE status='pending' AND (kind<>'replace_chapter_body' OR target_id IN (SELECT id FROM chapters WHERE position>=?))`, formatTime(now), p.Position)
	if err != nil {
		return p, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE change_sets SET status='conflicted',updated_at=? WHERE status IN ('pending','partially_applied') AND id IN (SELECT change_set_id FROM change_operations WHERE status='conflicted')`, formatTime(now))
	if err != nil {
		return p, err
	}
	encoded, _ = json.Marshal(p)
	_, err = tx.ExecContext(ctx, `INSERT INTO story_constraint_changes(project_id,asset_id,revision,chapter_id,position,reason,digest,idempotency_key,request_json,receipt_json,created_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, c.Request.ProjectID, row.AssetID, p.Revision, c.Request.ChapterID, p.Position, c.Request.Reason, c.Digest, c.IdempotencyKey, string(requestJSON), string(encoded), formatTime(now))
	if err != nil {
		return p, err
	}
	if err = tx.Commit(); err != nil {
		return p, fmt.Errorf("commit future constraint: %w", err)
	}
	return p, nil
}
