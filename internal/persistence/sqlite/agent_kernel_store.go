package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

func validateAgentRunBundle(bundle agentdomain.RunBundle) error {
	run := bundle.Run
	if strings.TrimSpace(run.ID) == "" {
		return agentdomain.ErrInvalidAgentPersistence
	}
	if !run.IsLegacy() {
		if !agentdomain.ValidTaskKind(run.TaskKind) || run.WorkflowBinding.Validate() != nil ||
			!agentdomain.ValidSHA256(run.InputDigest) || run.BudgetPolicyVersion < 1 ||
			run.QualityPolicyVersion < 1 || run.MaxRevisionAttempts < 0 {
			return agentdomain.ErrInvalidAgentPersistence
		}
	}
	steps := make(map[string]int, len(run.Steps))
	for _, step := range run.Steps {
		if step.RunID != run.ID || strings.TrimSpace(step.ID) == "" || strings.TrimSpace(step.PrimaryArtifactID) != "" {
			return agentdomain.ErrInvalidAgentPersistence
		}
		if !run.IsLegacy() && validateKernelStep(step) != nil {
			return agentdomain.ErrInvalidAgentPersistence
		}
		steps[step.ID] = normalizedAttempt(step)
	}
	return validateBundleChildren(run.ID, steps, bundle.Artifacts, bundle.Relations, bundle.Manifests, bundle.Findings)
}

func validateAgentStepBundle(bundle agentdomain.StepBundle) error {
	step := bundle.Step
	if strings.TrimSpace(step.ID) == "" || strings.TrimSpace(step.RunID) == "" {
		return agentdomain.ErrInvalidAgentPersistence
	}
	if step.Capability != "" && validateKernelStep(step) != nil {
		return agentdomain.ErrInvalidAgentPersistence
	}
	steps := map[string]int{step.ID: normalizedAttempt(step)}
	return validateBundleChildren(step.RunID, steps, bundle.Artifacts, bundle.Relations, bundle.Manifests, bundle.Findings)
}

func validateBundleChildren(
	runID string,
	steps map[string]int,
	artifacts []agentdomain.Artifact,
	relations []agentdomain.ArtifactRelationRecord,
	manifests []agentdomain.ContextManifest,
	findings []agentdomain.ReviewFindingRecord,
) error {
	for _, artifact := range artifacts {
		if artifact.RunID != runID || artifact.Validate() != nil {
			return agentdomain.ErrInvalidAgentPersistence
		}
		if artifact.StepID != "" {
			if _, ok := steps[artifact.StepID]; !ok {
				return agentdomain.ErrInvalidAgentPersistence
			}
		}
	}
	created := make(map[string]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		created[artifact.ID] = struct{}{}
	}
	for _, relation := range relations {
		if relation.Validate() != nil {
			return agentdomain.ErrInvalidAgentPersistence
		}
		if _, exists := created[relation.ArtifactID]; !exists {
			return agentdomain.ErrInvalidAgentPersistence
		}
	}
	for _, manifest := range manifests {
		if manifest.RunID != runID || manifest.Validate() != nil {
			return agentdomain.ErrInvalidAgentPersistence
		}
		attempt, ok := steps[manifest.StepID]
		if !ok || manifest.Attempt != attempt {
			return agentdomain.ErrInvalidAgentPersistence
		}
	}
	for _, finding := range findings {
		if finding.RunID != runID || finding.Validate() != nil {
			return agentdomain.ErrInvalidAgentPersistence
		}
		attempt, ok := steps[finding.StepID]
		if !ok || finding.Attempt != attempt {
			return agentdomain.ErrInvalidAgentPersistence
		}
	}
	return nil
}

func validateKernelStep(step agentdomain.RunStep) error {
	if !agentdomain.ValidCapability(step.Capability) || step.Attempt < 1 ||
		step.PromptBinding.Validate() != nil || step.SchemaBinding.Validate() != nil ||
		strings.TrimSpace(step.NodeID) == "" || !agentdomain.ValidPublicStageID(step.PublicStageID) || strings.TrimSpace(step.AssemblerID) == "" ||
		step.AssemblerVersion < 1 || !agentdomain.ValidSHA256(step.InputDigest) ||
		(step.OutputDigest != "" && !agentdomain.ValidSHA256(step.OutputDigest)) {
		return agentdomain.ErrInvalidAgentPersistence
	}
	if step.Status == agentdomain.StepCompleted &&
		(strings.TrimSpace(step.PrimaryArtifactID) == "" || !agentdomain.ValidSHA256(step.OutputDigest) ||
			!agentdomain.ValidQualityStatus(step.QualityStatus)) {
		return agentdomain.ErrInvalidAgentPersistence
	}
	return nil
}

func normalizedNodeID(step agentdomain.RunStep) string {
	if strings.TrimSpace(step.NodeID) != "" {
		return strings.TrimSpace(step.NodeID)
	}
	return string(step.Role)
}

func normalizedAttempt(step agentdomain.RunStep) int {
	if step.Attempt > 0 {
		return step.Attempt
	}
	return 1
}

func insertAgentArtifact(ctx context.Context, exec txExecutor, artifact agentdomain.Artifact) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO agent_artifacts(id, run_id, step_id, kind, content, sha256, created_at)
		VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?)
	`, artifact.ID, artifact.RunID, artifact.StepID, artifact.Kind, artifact.Content, artifact.SHA256, formatTime(artifact.CreatedAt))
	if err != nil {
		return databaseOperationError("create agent artifact", err)
	}
	return nil
}

func insertAgentArtifactRelation(ctx context.Context, exec txExecutor, relation agentdomain.ArtifactRelationRecord) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO agent_artifact_relations(artifact_id, relation, related_artifact_id, base_digest, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, relation.ArtifactID, relation.Relation, relation.RelatedArtifactID, relation.BaseDigest, formatTime(relation.CreatedAt))
	if err != nil {
		return databaseOperationError("create agent artifact relation", err)
	}
	return nil
}

func insertContextManifest(ctx context.Context, exec txExecutor, manifest agentdomain.ContextManifest) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO context_manifests(
			id, version, run_id, step_id, attempt, canonical_head, assembler_id,
			assembler_version, character_budget, estimated_token_budget, used_chars,
			truncated, assembled_content_hash, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, manifest.ID, manifest.Version, manifest.RunID, manifest.StepID, manifest.Attempt,
		manifest.CanonicalHead, manifest.AssemblerID, manifest.AssemblerVersion,
		manifest.CharacterBudget, manifest.EstimatedTokenBudget, manifest.UsedChars,
		manifest.Truncated, manifest.AssembledContentHash, formatTime(manifest.CreatedAt))
	if err != nil {
		return databaseOperationError("create context manifest", err)
	}
	for _, item := range manifest.Items {
		_, err = exec.ExecContext(ctx, `
			INSERT INTO context_manifest_items(
				manifest_id, source_kind, source_id, source_version, range_start, range_end,
				content_hash, position, priority, selection_reason, provisional, included,
				omission_reason
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, item.ManifestID, item.SourceKind, item.SourceID, item.SourceVersion,
			nullableInt(item.RangeStart), nullableInt(item.RangeEnd), item.ContentHash,
			item.Position, item.Priority, item.SelectionReason, item.Provisional,
			item.Included, item.OmissionReason)
		if err != nil {
			return databaseOperationError("create context manifest item", err)
		}
	}
	return nil
}

func insertAgentReviewFinding(ctx context.Context, exec txExecutor, finding agentdomain.ReviewFindingRecord) error {
	_, err := exec.ExecContext(ctx, `
		INSERT INTO agent_review_findings(
			id, run_id, step_id, attempt, category, severity, status, source, message,
			suggested_action, quoted_evidence, scope_start, scope_end, constraint_ref,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, storedFindingID(finding), finding.RunID, finding.StepID, finding.Attempt, finding.Category,
		finding.Severity, finding.Status, finding.Source, finding.Message, finding.Suggestion,
		finding.Quote, nullableInt(finding.ScopeStart), nullableInt(finding.ScopeEnd),
		finding.ConstraintRef, formatTime(finding.CreatedAt), formatTime(finding.UpdatedAt))
	if err != nil {
		return databaseOperationError("create agent review finding", err)
	}
	return nil
}

func (s *Store) ContextManifest(ctx context.Context, id string) (agentdomain.ContextManifest, error) {
	manifest, err := scanContextManifest(s.db.QueryRowContext(ctx, `
		SELECT id, version, run_id, step_id, attempt, canonical_head, assembler_id,
			assembler_version, character_budget, estimated_token_budget, used_chars,
			truncated, assembled_content_hash, created_at
		FROM context_manifests WHERE id = ?
	`, id))
	if err == sql.ErrNoRows {
		return agentdomain.ContextManifest{}, fmt.Errorf("%w: context manifest %s", ErrAgentRecordMissing, id)
	}
	if err != nil {
		return agentdomain.ContextManifest{}, err
	}
	manifest.Items, err = s.listContextManifestItems(ctx, id)
	return manifest, err
}

func (s *Store) ListContextManifests(ctx context.Context, runID string) ([]agentdomain.ContextManifest, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, version, run_id, step_id, attempt, canonical_head, assembler_id,
			assembler_version, character_budget, estimated_token_budget, used_chars,
			truncated, assembled_content_hash, created_at
		FROM context_manifests WHERE run_id = ? ORDER BY created_at, id
	`, runID)
	if err != nil {
		return nil, databaseOperationError("list context manifests", err)
	}
	defer rows.Close()
	result := make([]agentdomain.ContextManifest, 0)
	for rows.Next() {
		manifest, scanErr := scanContextManifest(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, manifest)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Items, err = s.listContextManifestItems(ctx, result[index].ID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func scanContextManifest(scanner rowScanner) (agentdomain.ContextManifest, error) {
	var manifest agentdomain.ContextManifest
	var createdAt string
	err := scanner.Scan(
		&manifest.ID, &manifest.Version, &manifest.RunID, &manifest.StepID, &manifest.Attempt,
		&manifest.CanonicalHead, &manifest.AssemblerID, &manifest.AssemblerVersion,
		&manifest.CharacterBudget, &manifest.EstimatedTokenBudget, &manifest.UsedChars,
		&manifest.Truncated, &manifest.AssembledContentHash, &createdAt,
	)
	if err != nil {
		return manifest, err
	}
	manifest.CreatedAt, err = parseTime(createdAt)
	return manifest, err
}

func (s *Store) listContextManifestItems(ctx context.Context, manifestID string) ([]agentdomain.ContextManifestItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT manifest_id, source_kind, source_id, source_version, range_start, range_end,
			content_hash, position, priority, selection_reason, provisional, included,
			omission_reason
		FROM context_manifest_items WHERE manifest_id = ? ORDER BY position
	`, manifestID)
	if err != nil {
		return nil, databaseOperationError("list context manifest items", err)
	}
	defer rows.Close()
	result := make([]agentdomain.ContextManifestItem, 0)
	for rows.Next() {
		var item agentdomain.ContextManifestItem
		var start, end sql.NullInt64
		if err := rows.Scan(&item.ManifestID, &item.SourceKind, &item.SourceID, &item.SourceVersion,
			&start, &end, &item.ContentHash, &item.Position, &item.Priority, &item.SelectionReason,
			&item.Provisional, &item.Included, &item.OmissionReason); err != nil {
			return nil, err
		}
		item.RangeStart = intPointer(start)
		item.RangeEnd = intPointer(end)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListAgentReviewFindings(ctx context.Context, runID string) ([]agentdomain.ReviewFindingRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, run_id, step_id, attempt, category, severity, status, source, message,
			suggested_action, quoted_evidence, scope_start, scope_end, constraint_ref,
			created_at, updated_at
		FROM agent_review_findings WHERE run_id = ? ORDER BY created_at, id
	`, runID)
	if err != nil {
		return nil, databaseOperationError("list agent review findings", err)
	}
	defer rows.Close()
	result := make([]agentdomain.ReviewFindingRecord, 0)
	for rows.Next() {
		var finding agentdomain.ReviewFindingRecord
		var start, end sql.NullInt64
		var createdAt, updatedAt string
		if err := rows.Scan(&finding.ID, &finding.RunID, &finding.StepID, &finding.Attempt,
			&finding.Category, &finding.Severity, &finding.Status, &finding.Source,
			&finding.Message, &finding.Suggestion, &finding.Quote, &start, &end,
			&finding.ConstraintRef, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		// 数据库主键跨运行唯一，读取时恢复模型在本次步骤内使用的局部引用。
		finding.ID = strings.TrimPrefix(finding.ID, findingStoragePrefix(finding))
		finding.ScopeStart = intPointer(start)
		finding.ScopeEnd = intPointer(end)
		finding.CreatedAt, err = parseTime(createdAt)
		if err == nil {
			finding.UpdatedAt, err = parseTime(updatedAt)
		}
		if err != nil {
			return nil, err
		}
		result = append(result, finding)
	}
	return result, rows.Err()
}

// storedFindingID 为模型局部编号加上运行、步骤和重试范围，避免跨次调用撞主键。
func storedFindingID(finding agentdomain.ReviewFindingRecord) string {
	return findingStoragePrefix(finding) + finding.ID
}

func findingStoragePrefix(finding agentdomain.ReviewFindingRecord) string {
	return finding.RunID + "/" + finding.StepID + "/" + strconv.Itoa(finding.Attempt) + "/"
}

func nullableInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
}

func intPointer(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}
