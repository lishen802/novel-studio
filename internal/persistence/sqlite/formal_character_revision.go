package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/lishen802/novel-studio/internal/agentassets"
	agent "github.com/lishen802/novel-studio/internal/domain/agent"
	creation "github.com/lishen802/novel-studio/internal/domain/creation"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// FormalCharacterRevisionBase 在同一读取快照冻结 head 和保护输入，拒绝缺来源、锁定及身份歧义。
func (s *Store) FormalCharacterRevisionBase(ctx context.Context, projectID, branchID, assetID string) (asset.CharacterRevisionBase, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return asset.CharacterRevisionBase{}, err
	}
	defer tx.Rollback()
	return formalCharacterBase(ctx, tx, projectID, branchID, assetID)
}
func formalCharacterBase(ctx context.Context, q constraintReader, projectID, branchID, assetID string) (asset.CharacterRevisionBase, error) {
	base := asset.CharacterRevisionBase{ProjectID: projectID, BranchID: branchID, AssetID: assetID}
	rows, err := characterConstraintRows(ctx, q)
	if err != nil {
		return base, err
	}
	var selected *constraintRow
	for i := range rows {
		if rows[i].AssetID == assetID {
			selected = &rows[i]
			break
		}
	}
	if selected == nil || selected.Locked || selected.stale {
		return base, asset.ErrChangeConflict
	}
	var sourceID, sourcePayload, sourceHash string
	err = q.QueryRowContext(ctx, `SELECT c.id,c.payload_json,c.content_sha256 FROM story_assets a JOIN creation_candidates c ON c.id=a.source_candidate_id WHERE a.id=? AND a.project_id=? AND c.kind='people_world' AND c.payload_schema_id='story_cast.schema' AND c.payload_schema_version=2`, assetID, projectID).Scan(&sourceID, &sourcePayload, &sourceHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return base, asset.ErrChangeUnsupported
		}
		return base, err
	}
	if adoptionHash(sourcePayload) != sourceHash {
		return base, asset.ErrChangeConflict
	}
	var cast agent.StoryCastOutput
	if json.Unmarshal([]byte(sourcePayload), &cast) != nil || cast.Validate() != nil {
		return base, asset.ErrChangeUnsupported
	}
	// 唯一完整原档案证明来源实体，随后以最新正式档案替换；不通过同名推断身份。
	originalCharacters := append([]agent.CastCharacter(nil), cast.Characters...)
	for _, row := range rows {
		var origin string
		if err = q.QueryRowContext(ctx, `SELECT COALESCE(source_candidate_id,'') FROM story_assets WHERE id=?`, row.AssetID).Scan(&origin); err != nil {
			return base, err
		}
		if origin != sourceID {
			continue
		}
		matches, index := 0, -1
		for i, c := range originalCharacters {
			if castProfile(c) == row.Original {
				matches++
				index = i
			}
		}
		if matches != 1 {
			return base, asset.ErrChangeConflict
		}
		var p asset.CharacterProfile
		if json.Unmarshal([]byte(row.Current), &p) != nil {
			return base, asset.ErrChangeUnsupported
		}
		id := cast.Characters[index].ID
		cast.Characters[index] = agent.CastCharacter{ID: id, Name: p.Name, Aliases: p.Aliases, Role: string(p.Role), Summary: p.Summary, Motivation: p.Motivation, Fear: p.Fear, Secret: p.Secret, Arc: p.Arc}
		if row.AssetID == assetID {
			base.EntityID = id
		}
	}
	if base.EntityID == "" || cast.Validate() != nil {
		return base, asset.ErrChangeUnsupported
	}
	payload, err := json.Marshal(cast)
	if err != nil {
		return base, err
	}
	base.CastJSON = string(payload)
	base.TypedID = selected.typedID
	base.Revision = selected.Revision
	base.Profile = selected.Current
	base.PayloadSHA256 = adoptionHash(selected.payload)
	// 最新正式载荷单独作为必要来源；未来规划与设定不能解释为历史事实。
	records, err := q.QueryContext(ctx, `SELECT a.id,a.kind,a.current_revision,r.payload_json FROM story_assets a JOIN story_asset_revisions r ON r.asset_id=a.id AND r.revision=a.current_revision WHERE a.project_id=? AND a.status='canonical' ORDER BY a.id`, projectID)
	if err != nil {
		return base, err
	}
	contextRows := []map[string]any{}
	for records.Next() {
		var id, kind, text string
		var revision int64
		if err = records.Scan(&id, &kind, &revision, &text); err != nil {
			records.Close()
			return base, err
		}
		contextRows = append(contextRows, map[string]any{"assetId": id, "kind": kind, "revision": revision, "payload": json.RawMessage(text)})
	}
	err = records.Err()
	records.Close()
	if err != nil {
		return base, err
	}
	var title, genre, summary, updated string
	if err = q.QueryRowContext(ctx, `SELECT title,genre,summary,updated_at FROM projects WHERE id=?`, projectID).Scan(&title, &genre, &summary, &updated); err != nil {
		return base, err
	}
	encoded, err := json.Marshal(map[string]any{"project": map[string]string{"id": projectID, "title": title, "genre": genre, "summary": summary, "updatedAt": updated}, "latestFormalAssets": contextRows, "referenceCastCollectionsAreOriginalAndReadOnly": true, "futureSettingsAreNotHistoricalFacts": true, "planningGoalsAreNotHistoricalFacts": true})
	if err != nil {
		return base, err
	}
	base.ContextJSON = string(encoded)
	base.Digest, err = constraintFingerprint(ctx, q, asset.ConstraintRequest{ProjectID: projectID, BranchID: branchID, AssetID: assetID})
	return base, err
}

// formalCharacterAdoption 只从已完成且通过独立审阅的指定工件提取人物差异，正式确认仍复验全部基线。
func formalCharacterAdoption(ctx context.Context, q constraintReader, row constraintRow, candidateID, projectID, branchID string) (characterAdoptionSource, error) {
	var source characterAdoptionSource
	actionID := strings.TrimPrefix(candidateID, asset.FormalCharacterCandidatePrefix)
	var arguments, argumentsHash, artifact, artifactHash string
	err := q.QueryRowContext(ctx, `SELECT x.arguments,x.arguments_hash,r.id,f.id,f.content,f.sha256 FROM assistant_actions x JOIN agent_runs r ON r.id=CASE WHEN json_valid(x.arguments) THEN json_extract(x.arguments,'$.runId') END JOIN agent_run_steps step ON step.run_id=r.id AND step.node_id='revise' AND step.status='completed' JOIN agent_artifacts f ON f.id=step.primary_artifact_id AND f.run_id=r.id WHERE x.id=? AND x.project_id=? AND x.branch_id=? AND x.operation='revise_fields' AND x.state='succeeded' AND r.project_id=x.project_id AND r.status='completed' AND r.quality_status='passed' AND r.task_kind='revise_story_candidate' AND r.workflow_id=? AND r.workflow_version=?`, actionID, projectID, branchID, agentassets.StoryCandidateRevisionWorkflowID, agentassets.StoryCandidateRevisionWorkflowVersion).Scan(&arguments, &argumentsHash, &source.RunID, &source.ArtifactID, &artifact, &artifactHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return source, asset.ErrChangeConflict
		}
		return source, err
	}
	var input struct {
		Contract string                       `json:"contract"`
		Formal   *asset.CharacterRevisionBase `json:"formal"`
		Base     creation.Candidate           `json:"base"`
		Request  struct {
			FormalAssetID    string   `json:"formalAssetId"`
			ExpectedRevision int64    `json:"expectedRevision"`
			Paths            []string `json:"paths"`
			AllowAdd         []string `json:"allowAdd"`
			AllowRemove      []string `json:"allowRemove"`
		} `json:"request"`
	}
	if json.Unmarshal([]byte(arguments), &input) != nil || input.Contract != "story_revision@1" || input.Formal == nil || adoptionHash(arguments) != argumentsHash || adoptionHash(artifact) != artifactHash {
		return source, asset.ErrChangeConflict
	}
	latest, err := formalCharacterBase(ctx, q, projectID, branchID, row.AssetID)
	if err != nil {
		if errors.Is(err, asset.ErrChangeConflict) || errors.Is(err, asset.ErrChangeUnsupported) {
			return source, asset.ErrChangeConflict
		}
		return source, err
	}
	if !reflect.DeepEqual(latest, *input.Formal) || input.Request.FormalAssetID != row.AssetID || input.Request.ExpectedRevision != row.Revision || input.Base.PayloadJSON != latest.CastJSON || input.Base.ContentSHA256 != adoptionHash(latest.CastJSON) || len(input.Request.Paths) != 1 || len(input.Request.AllowAdd) > 0 || len(input.Request.AllowRemove) > 0 {
		return source, asset.ErrChangeConflict
	}
	prefix := "/characters/@" + strings.ReplaceAll(strings.ReplaceAll(latest.EntityID, "~", "~0"), "/", "~1") + "/"
	field := strings.TrimPrefix(input.Request.Paths[0], prefix)
	if input.Request.Paths[0] == field || !formalCharacterField(field) {
		return source, asset.ErrChangeConflict
	}
	var output agent.StoryCandidateRevisionOutput
	if json.Unmarshal([]byte(artifact), &output) != nil || output.Validate() != nil || output.CandidateKind != "people_world" || output.PayloadSchemaRef != "story_cast.schema@2" {
		return source, asset.ErrChangeConflict
	}
	if _, err = input.Base.ReviewScopedRevision(string(output.Payload), creation.RevisionScope{Paths: input.Request.Paths}); err != nil {
		return source, fmt.Errorf("%w: %v", asset.ErrChangeConflict, err)
	}
	var next agent.StoryCastOutput
	if json.Unmarshal(output.Payload, &next) != nil {
		return source, asset.ErrChangeConflict
	}
	for _, c := range next.Characters {
		if c.ID == latest.EntityID {
			source.Text = castProfile(c)
		}
	}
	if source.Text == "" || source.Text == row.Current {
		return source, asset.ErrChangeConflict
	}
	source.CandidateID = candidateID
	source.EntityID = latest.EntityID
	source.BranchID = branchID
	source.ActionID = actionID
	source.SourceDigest = adoptionHash(argumentsHash + artifactHash)
	return source, nil
}
func formalCharacterField(field string) bool {
	switch field {
	case "name", "summary", "motivation", "fear", "secret", "arc":
		return true
	}
	return false
}
