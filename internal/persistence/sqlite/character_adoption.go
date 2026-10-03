package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/lishen802/novel-studio/internal/agentassets"
	agent "github.com/lishen802/novel-studio/internal/domain/agent"
	creation "github.com/lishen802/novel-studio/internal/domain/creation"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
	world "github.com/lishen802/novel-studio/internal/domain/world"
)

func adoptionHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

type characterAdoptionSource struct {
	asset.CharacterSuggestion
	RunID        string `json:"runId"`
	ArtifactID   string `json:"artifactId"`
	SourceDigest string `json:"sourceDigest"`
}

// characterAdoption 核对已审阅的完整候选及冻结授权，以唯一完整档案定位稳定人物身份。
func characterAdoption(ctx context.Context, q constraintReader, row constraintRow, candidateID, projectID, branchID string) (characterAdoptionSource, error) {
	var source characterAdoptionSource
	var parent, payload, candidateHash, runID, artifactID, arguments, argumentsHash, artifact, artifactHash, basePayload, sourceID string
	err := q.QueryRowContext(ctx, `SELECT c.parent_candidate_id,c.payload_json,c.content_sha256,c.run_id,c.artifact_id,x.id,x.arguments,x.arguments_hash,f.content,f.sha256,b.payload_json,a.source_candidate_id
 FROM story_assets a JOIN creation_candidates c ON c.id=?
 JOIN creation_candidates b ON b.id=c.parent_candidate_id AND b.session_id=c.session_id AND b.kind=c.kind
 JOIN creation_sessions s ON s.id=c.session_id
 JOIN agent_runs r ON r.id=c.run_id
 JOIN agent_artifacts f ON f.id=c.artifact_id AND f.run_id=r.id
 JOIN assistant_actions x ON CASE WHEN json_valid(x.arguments) THEN json_extract(x.arguments,'$.resultId') END=c.id AND CASE WHEN json_valid(x.arguments) THEN json_extract(x.arguments,'$.runId') END=r.id
 WHERE a.id=? AND a.project_id=? AND s.project_id=a.project_id AND s.branch_id=?
 AND x.project_id=a.project_id AND x.branch_id=s.branch_id AND x.operation='revise_fields' AND x.state='succeeded'
 AND c.kind='people_world' AND c.origin='model' AND c.quality_status='passed' AND c.status IN ('draft','selected','confirmed')
 AND c.payload_schema_id='story_cast.schema' AND c.payload_schema_version=2
 AND r.project_id=a.project_id AND r.status='completed' AND r.quality_status='passed' AND r.task_kind='revise_story_candidate' AND r.workflow_id=? AND r.workflow_version=?`, candidateID, row.AssetID, projectID, branchID, agentassets.StoryCandidateRevisionWorkflowID, agentassets.StoryCandidateRevisionWorkflowVersion).Scan(&parent, &payload, &candidateHash, &runID, &artifactID, &source.ActionID, &arguments, &argumentsHash, &artifact, &artifactHash, &basePayload, &sourceID)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return source, fmt.Errorf("read character candidate source: %w", err)
		}
		return source, fmt.Errorf("%w: 候选来源、任务状态或分支不再有效", asset.ErrChangeConflict)
	}
	if parent != sourceID || adoptionHash(payload) != candidateHash || adoptionHash(artifact) != artifactHash || adoptionHash(arguments) != argumentsHash {
		return source, fmt.Errorf("%w: 候选与来源文件内容校验失败", asset.ErrChangeConflict)
	}
	var input struct {
		Contract string             `json:"contract"`
		Base     creation.Candidate `json:"base"`
		BaseID   string             `json:"baseId"`
		RunID    string             `json:"runId"`
		ResultID string             `json:"resultId"`
		Request  struct {
			CandidateID string   `json:"candidateId"`
			Paths       []string `json:"paths"`
			AllowAdd    []string `json:"allowAdd"`
			AllowRemove []string `json:"allowRemove"`
		} `json:"request"`
	}
	if json.Unmarshal([]byte(arguments), &input) != nil || input.Contract != "story_revision@1" || input.Base.ID != parent || input.BaseID == "" || input.Request.CandidateID != parent || input.RunID != runID || input.ResultID != candidateID || input.Base.PayloadJSON != basePayload || input.Base.ContentSHA256 != adoptionHash(basePayload) {
		return source, fmt.Errorf("%w: 候选的冻结授权与来源基线不一致", asset.ErrChangeConflict)
	}
	var output agent.StoryCandidateRevisionOutput
	if json.Unmarshal([]byte(artifact), &output) != nil || output.Validate() != nil || output.CandidateKind != "people_world" || output.PayloadSchemaRef != "story_cast.schema@2" {
		return source, fmt.Errorf("%w: 候选结果合同无效", asset.ErrChangeConflict)
	}
	var actual, expected any
	if json.Unmarshal(output.Payload, &actual) != nil || json.Unmarshal([]byte(payload), &expected) != nil || !reflect.DeepEqual(actual, expected) {
		return source, fmt.Errorf("%w: 候选与审阅结果不一致", asset.ErrChangeConflict)
	}
	if _, err = input.Base.ReviewScopedRevision(payload, creation.RevisionScope{Paths: input.Request.Paths, AllowAdd: input.Request.AllowAdd, AllowRemove: input.Request.AllowRemove}); err != nil {
		return source, fmt.Errorf("%w: 候选超出授权修改范围", asset.ErrChangeConflict)
	}
	var base, next agent.StoryCastOutput
	if json.Unmarshal([]byte(basePayload), &base) != nil || json.Unmarshal([]byte(payload), &next) != nil {
		return source, fmt.Errorf("%w: 候选人物资料无法读取", asset.ErrChangeConflict)
	}
	matches := 0
	for _, character := range base.Characters {
		if castProfile(character) == row.Current {
			matches++
			source.EntityID = character.ID
		}
	}
	if matches != 1 || source.EntityID == "" {
		return source, fmt.Errorf("%w: 当前正式档案与 AI 修改基线不一致，或人物身份不唯一", asset.ErrChangeConflict)
	}
	matches = 0
	for _, character := range next.Characters {
		if character.ID == source.EntityID {
			matches++
			source.Text = castProfile(character)
		}
	}
	if matches != 1 || source.Text == "" || source.Text == row.Current {
		return source, fmt.Errorf("%w: AI 候选没有该人物的有效差异", asset.ErrChangeConflict)
	}
	source.SourceDigest = adoptionHash(argumentsHash + artifactHash + candidateHash)
	source.BranchID = branchID
	source.CandidateID = candidateID
	source.RunID = runID
	source.ArtifactID = artifactID
	return source, nil
}

func castProfile(c agent.CastCharacter) string {
	encoded, _ := json.Marshal(asset.CharacterProfile{Name: c.Name, Aliases: c.Aliases, Role: world.CharacterRole(c.Role), Summary: c.Summary, Motivation: c.Motivation, Fear: c.Fear, Secret: c.Secret, Arc: c.Arc})
	return constraintText("character", string(encoded))
}

// characterSuggestions 只列出可证明基线和来源的人物修改，未接通的候选不冒充正式修订。
func characterSuggestions(ctx context.Context, q constraintReader, row constraintRow) ([]asset.CharacterSuggestion, error) {
	rows, err := q.QueryContext(ctx, `SELECT c.id,s.project_id,s.branch_id FROM creation_candidates c JOIN creation_sessions s ON s.id=c.session_id JOIN story_assets a ON a.id=? AND a.source_candidate_id=c.parent_candidate_id WHERE c.kind='people_world' AND c.status IN ('draft','selected','confirmed') ORDER BY c.version DESC,c.id LIMIT 20`, row.AssetID)
	if err != nil {
		return nil, err
	}
	type ref struct{ id, project, branch string }
	refs := []ref{}
	for rows.Next() {
		var r ref
		if err = rows.Scan(&r.id, &r.project, &r.branch); err != nil {
			rows.Close()
			return nil, err
		}
		refs = append(refs, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []asset.CharacterSuggestion{}
	if row.Locked || row.stale {
		return out, nil
	}
	for _, r := range refs {
		source, e := characterAdoption(ctx, q, row, r.id, r.project, r.branch)
		if e == nil {
			out = append(out, source.CharacterSuggestion)
		} else if !errors.Is(e, asset.ErrChangeConflict) {
			return nil, e
		}
	}
	return out, nil
}
