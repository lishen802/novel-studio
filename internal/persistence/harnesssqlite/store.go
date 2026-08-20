package harnesssqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
			binding_json, case_ids_json, status, started_at, completed_at, failure
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, '')`,
		run.ID, run.DatasetID, run.DatasetVersion, run.DatasetSHA256, run.Mode, run.Seed,
		string(binding), string(caseIDs), run.Status, formatTime(run.StartedAt),
	)
	if err != nil {
		return fmt.Errorf("insert harness run: %w", err)
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
	if _, err := transaction.ExecContext(ctx, `UPDATE harness_runs SET status = ?, completed_at = ?, failure = ? WHERE id = ?`, run.Status, completed, run.Failure, run.ID); err != nil {
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
		result = append(result, quality.AnonymousHarnessCandidate{Slot: fmt.Sprintf("候选-%d", index+1), Content: candidate.Content})
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
		       binding_json, case_ids_json, status, started_at, completed_at, failure
		FROM harness_runs WHERE id = ?`, runID).Scan(
		&run.ID, &run.DatasetID, &run.DatasetVersion, &run.DatasetSHA256, &run.Mode, &run.Seed,
		&bindingJSON, &caseIDsJSON, &run.Status, &startedAt, &completedAt, &run.Failure,
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

// DeleteAll 仅清除独立 Harness 库内的实验数据。
func (store *Store) DeleteAll(ctx context.Context) error {
	transaction, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin deleting harness data: %w", err)
	}
	defer transaction.Rollback()
	for _, table := range []string{"harness_ratings", "harness_reports", "harness_case_results", "harness_runs"} {
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
