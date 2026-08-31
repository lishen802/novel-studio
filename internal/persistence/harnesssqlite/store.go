package harnesssqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/quality"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (store *Store) CreateRun(ctx context.Context, run quality.HarnessRun) error {
	binding, err := json.Marshal(run.Binding)
	if err != nil {
		return fmt.Errorf("encode harness binding: %w", err)
	}
	caseIDs, err := json.Marshal(run.CaseIDs)
	if err != nil {
		return fmt.Errorf("encode harness case ids: %w", err)
	}
	_, err = store.db.ExecContext(ctx, `
		INSERT INTO harness_runs (
			id, dataset_id, dataset_version, dataset_sha256, mode, random_seed,
			binding_json, case_ids_json, status, lifecycle_state, retry_of, current_case_id, public_stage_id,
			input_tokens, output_tokens, calls, started_at, completed_at, failure
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, '')`,
		run.ID, run.DatasetID, run.DatasetVersion, run.DatasetSHA256, run.Mode, run.Seed,
		string(binding), string(caseIDs), legacyStatus(run.Status), run.Status, run.RetryOf, run.CurrentCaseID, run.PublicStageID,
		run.InputTokens, run.OutputTokens, run.Calls, formatTime(run.StartedAt),
	)
	if err != nil {
		return fmt.Errorf("insert harness run: %w", err)
	}
	return nil
}

// UpdateRun 保存可恢复生命周期；旧 status 列仅用于兼容已发布的首版表约束。
func (store *Store) UpdateRun(ctx context.Context, run quality.HarnessRun) error {
	var completed any
	if run.CompletedAt != nil {
		completed = formatTime(*run.CompletedAt)
	}
	result, err := store.db.ExecContext(ctx, `
		UPDATE harness_runs SET status = ?, lifecycle_state = ?, retry_of = ?, current_case_id = ?, public_stage_id = ?,
		input_tokens = ?, output_tokens = ?, calls = ?, completed_at = ?, failure = ? WHERE id = ?`,
		legacyStatus(run.Status), run.Status, run.RetryOf, run.CurrentCaseID, run.PublicStageID,
		run.InputTokens, run.OutputTokens, run.Calls, completed, run.Failure, run.ID)
	if err != nil {
		return fmt.Errorf("update harness run: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("update harness run: %w", sql.ErrNoRows)
	}
	return nil
}

func (store *Store) SaveCaseResult(ctx context.Context, result quality.HarnessCaseResult) error {
	candidates, err := json.Marshal(result.Candidates)
	if err != nil {
		return fmt.Errorf("encode harness candidates: %w", err)
	}
	order, err := json.Marshal(result.AnonymousOrder)
	if err != nil {
		return fmt.Errorf("encode harness anonymous order: %w", err)
	}
	_, err = store.db.ExecContext(ctx, `
		INSERT INTO harness_case_results (run_id, case_id, genre, task_kind, candidates_json, anonymous_order_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id, case_id) DO UPDATE SET
			genre = excluded.genre, task_kind = excluded.task_kind,
			candidates_json = excluded.candidates_json, anonymous_order_json = excluded.anonymous_order_json`,
		result.RunID, result.CaseID, result.Genre, result.TaskKind, string(candidates), string(order),
	)
	if err != nil {
		return fmt.Errorf("save harness case result: %w", err)
	}
	return nil
}

// CompleteRun 原子更新运行终态并保存对应自动报告。
func (store *Store) CompleteRun(ctx context.Context, run quality.HarnessRun, report quality.HarnessReport) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("encode harness report: %w", err)
	}
	transaction, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin harness completion: %w", err)
	}
	defer transaction.Rollback()
	var completed any
	if run.CompletedAt != nil {
		completed = formatTime(*run.CompletedAt)
	}
	if _, err := transaction.ExecContext(ctx, `UPDATE harness_runs SET status = ?, lifecycle_state = ?, retry_of = ?, current_case_id = ?, public_stage_id = ?, input_tokens = ?, output_tokens = ?, calls = ?, completed_at = ?, failure = ? WHERE id = ?`, legacyStatus(run.Status), run.Status, run.RetryOf, run.CurrentCaseID, run.PublicStageID, run.InputTokens, run.OutputTokens, run.Calls, completed, run.Failure, run.ID); err != nil {
		return fmt.Errorf("update harness run: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `
		INSERT INTO harness_reports (run_id, report_json, generated_at) VALUES (?, ?, ?)
		ON CONFLICT(run_id) DO UPDATE SET report_json = excluded.report_json, generated_at = excluded.generated_at`,
		report.RunID, string(payload), formatTime(report.GeneratedAt)); err != nil {
		return fmt.Errorf("save harness report: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit harness completion: %w", err)
	}
	return nil
}

// GetBlindCaseResult 只返回匿名槽位和正文，不暴露候选、模型或工作流身份。
func (store *Store) GetBlindCaseResult(ctx context.Context, runID, caseID string) ([]quality.AnonymousHarnessCandidate, error) {
	var candidatesJSON, orderJSON string
	if err := store.db.QueryRowContext(ctx, `SELECT candidates_json, anonymous_order_json FROM harness_case_results WHERE run_id = ? AND case_id = ?`, runID, caseID).Scan(&candidatesJSON, &orderJSON); err != nil {
		return nil, fmt.Errorf("load blind harness result: %w", err)
	}
	var candidates []quality.HarnessCandidate
	var order []string
	if err := json.Unmarshal([]byte(candidatesJSON), &candidates); err != nil {
		return nil, fmt.Errorf("decode harness candidates: %w", err)
	}
	if err := json.Unmarshal([]byte(orderJSON), &order); err != nil {
		return nil, fmt.Errorf("decode harness anonymous order: %w", err)
	}
	byKind := make(map[string]quality.HarnessCandidate, len(candidates))
	for _, candidate := range candidates {
		byKind[string(candidate.Kind)] = candidate
	}
	result := make([]quality.AnonymousHarnessCandidate, 0, len(order))
	for index, kind := range order {
		candidate, exists := byKind[kind]
		if !exists {
			return nil, fmt.Errorf("load blind harness result: unknown candidate mapping")
		}
		result = append(result, quality.AnonymousHarnessCandidate{Slot: fmt.Sprintf("候选-%d", index+1), Content: candidate.Content, QualityReportJSON: candidate.QualityReportJSON})
	}
	return result, nil
}

func (store *Store) GetReport(ctx context.Context, runID string) (quality.HarnessReport, error) {
	var payload string
	if err := store.db.QueryRowContext(ctx, `SELECT report_json FROM harness_reports WHERE run_id = ?`, runID).Scan(&payload); err != nil {
		return quality.HarnessReport{}, fmt.Errorf("load harness report: %w", err)
	}
	var report quality.HarnessReport
	if err := json.Unmarshal([]byte(payload), &report); err != nil {
		return quality.HarnessReport{}, fmt.Errorf("decode harness report: %w", err)
	}
	return report, nil
}

func (store *Store) GetRun(ctx context.Context, runID string) (quality.HarnessRun, error) {
	var run quality.HarnessRun
	var bindingJSON, caseIDsJSON, startedAt string
	var completedAt sql.NullString
	err := store.db.QueryRowContext(ctx, `
		SELECT id, dataset_id, dataset_version, dataset_sha256, mode, random_seed,
		       binding_json, case_ids_json, lifecycle_state, retry_of, current_case_id, public_stage_id,
		       input_tokens, output_tokens, calls, started_at, completed_at, failure
		FROM harness_runs WHERE id = ?`, runID).Scan(
		&run.ID, &run.DatasetID, &run.DatasetVersion, &run.DatasetSHA256, &run.Mode, &run.Seed,
		&bindingJSON, &caseIDsJSON, &run.Status, &run.RetryOf, &run.CurrentCaseID, &run.PublicStageID,
		&run.InputTokens, &run.OutputTokens, &run.Calls, &startedAt, &completedAt, &run.Failure,
	)
	if err != nil {
		return quality.HarnessRun{}, fmt.Errorf("load harness run: %w", err)
	}
	if err := json.Unmarshal([]byte(bindingJSON), &run.Binding); err != nil {
		return quality.HarnessRun{}, fmt.Errorf("decode harness binding: %w", err)
	}
	if err := json.Unmarshal([]byte(caseIDsJSON), &run.CaseIDs); err != nil {
		return quality.HarnessRun{}, fmt.Errorf("decode harness case ids: %w", err)
	}
	run.StartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return quality.HarnessRun{}, fmt.Errorf("decode harness start time: %w", err)
	}
	if completedAt.Valid {
		completed, err := time.Parse(time.RFC3339Nano, completedAt.String)
		if err != nil {
			return quality.HarnessRun{}, fmt.Errorf("decode harness completion time: %w", err)
		}
		run.CompletedAt = &completed
	}
	return run, nil
}

// ListRuns 返回最新运行，供桌面端重挂载后从持久化事实恢复。
func (store *Store) ListRuns(ctx context.Context) ([]quality.HarnessRun, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT id FROM harness_runs ORDER BY started_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list harness runs: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan harness run: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate harness runs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close harness runs: %w", err)
	}
	var runs []quality.HarnessRun
	for _, id := range ids {
		run, err := store.GetRun(ctx, id)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// DeleteRun 删除一条隔离实验及其级联结果、评分与报告。
func (store *Store) DeleteRun(ctx context.Context, runID string) error {
	result, err := store.db.ExecContext(ctx, `DELETE FROM harness_runs WHERE id = ?`, strings.TrimSpace(runID))
	if err != nil {
		return fmt.Errorf("delete harness run: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected != 1 {
		return fmt.Errorf("delete harness run: %w", sql.ErrNoRows)
	}
	return nil
}

func (store *Store) ListCaseResults(ctx context.Context, runID string) ([]quality.HarnessCaseResult, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT case_id, genre, task_kind, candidates_json, anonymous_order_json FROM harness_case_results WHERE run_id = ? ORDER BY case_id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list harness case results: %w", err)
	}
	defer rows.Close()
	var results []quality.HarnessCaseResult
	for rows.Next() {
		var result quality.HarnessCaseResult
		var candidatesJSON, orderJSON string
		result.RunID = runID
		if err := rows.Scan(&result.CaseID, &result.Genre, &result.TaskKind, &candidatesJSON, &orderJSON); err != nil {
			return nil, fmt.Errorf("scan harness case result: %w", err)
		}
		if err := json.Unmarshal([]byte(candidatesJSON), &result.Candidates); err != nil {
			return nil, fmt.Errorf("decode harness candidates: %w", err)
		}
		if err := json.Unmarshal([]byte(orderJSON), &result.AnonymousOrder); err != nil {
			return nil, fmt.Errorf("decode harness anonymous order: %w", err)
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

func (store *Store) GetCaseResult(ctx context.Context, runID, caseID string) (quality.HarnessCaseResult, error) {
	results, err := store.ListCaseResults(ctx, runID)
	if err != nil {
		return quality.HarnessCaseResult{}, err
	}
	for _, result := range results {
		if result.CaseID == caseID {
			return result, nil
		}
	}
	return quality.HarnessCaseResult{}, fmt.Errorf("load harness case result: %w", sql.ErrNoRows)
}

// SaveRatingSubmission 原子保存整组候选评分，重复提交明确返回领域错误。
func (store *Store) SaveRatingSubmission(ctx context.Context, submission quality.HarnessRatingSubmission) error {
	payload, err := json.Marshal(submission.Ratings)
	if err != nil {
		return fmt.Errorf("encode harness ratings: %w", err)
	}
	_, err = store.db.ExecContext(ctx, `INSERT INTO harness_rating_submissions (id, run_id, case_id, local_rater_id, ratings_json, created_at) VALUES (?, ?, ?, ?, ?, ?)`, submission.ID, submission.RunID, submission.CaseID, submission.LocalRaterID, string(payload), formatTime(submission.CreatedAt))
	if err != nil {
		if isUniqueConstraint(err) {
			return quality.ErrHarnessRatingExists
		}
		return fmt.Errorf("save harness rating submission: %w", err)
	}
	return nil
}

func (store *Store) GetRatingSubmission(ctx context.Context, runID, caseID, raterID string) (quality.HarnessRatingSubmission, error) {
	var submission quality.HarnessRatingSubmission
	var payload, createdAt string
	err := store.db.QueryRowContext(ctx, `SELECT id, ratings_json, created_at FROM harness_rating_submissions WHERE run_id = ? AND case_id = ? AND local_rater_id = ?`, runID, caseID, raterID).Scan(&submission.ID, &payload, &createdAt)
	if err != nil {
		return quality.HarnessRatingSubmission{}, fmt.Errorf("load harness rating submission: %w", err)
	}
	submission.RunID, submission.CaseID, submission.LocalRaterID = runID, caseID, raterID
	if err := json.Unmarshal([]byte(payload), &submission.Ratings); err != nil {
		return quality.HarnessRatingSubmission{}, fmt.Errorf("decode harness ratings: %w", err)
	}
	submission.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	return submission, err
}

func (store *Store) ListRatingSubmissions(ctx context.Context, runID string) ([]quality.HarnessRatingSubmission, error) {
	rows, err := store.db.QueryContext(ctx, `SELECT id, case_id, local_rater_id, ratings_json, created_at FROM harness_rating_submissions WHERE run_id = ? ORDER BY created_at, id`, runID)
	if err != nil {
		return nil, fmt.Errorf("list harness rating submissions: %w", err)
	}
	defer rows.Close()
	var result []quality.HarnessRatingSubmission
	for rows.Next() {
		var submission quality.HarnessRatingSubmission
		var payload, createdAt string
		submission.RunID = runID
		if err := rows.Scan(&submission.ID, &submission.CaseID, &submission.LocalRaterID, &payload, &createdAt); err != nil {
			return nil, fmt.Errorf("scan harness rating submission: %w", err)
		}
		if err := json.Unmarshal([]byte(payload), &submission.Ratings); err != nil {
			return nil, fmt.Errorf("decode harness ratings: %w", err)
		}
		submission.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, fmt.Errorf("decode harness rating time: %w", err)
		}
		result = append(result, submission)
	}
	return result, rows.Err()
}

// DeleteAll 仅清除独立 Harness 库内的实验数据。
func (store *Store) DeleteAll(ctx context.Context) error {
	transaction, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin deleting harness data: %w", err)
	}
	defer transaction.Rollback()
	for _, table := range []string{"harness_rating_submissions", "harness_ratings", "harness_reports", "harness_case_results", "harness_runs"} {
		if _, err := transaction.ExecContext(ctx, "DELETE FROM "+table); err != nil {
			return fmt.Errorf("delete %s: %w", table, err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit deleting harness data: %w", err)
	}
	return nil
}

func formatTime(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func legacyStatus(status quality.HarnessRunStatus) quality.HarnessRunStatus {
	switch status {
	case quality.HarnessRunCompleted:
		return quality.HarnessRunCompleted
	case quality.HarnessRunFailed, quality.HarnessRunCancelled:
		return quality.HarnessRunFailed
	default:
		return quality.HarnessRunRunning
	}
}

func isUniqueConstraint(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}
