package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// SaveCreationReview 以 review_version 为前置条件写入逐项审阅关系；
// expected.ReviewVersion 为 0 表示首次建立审阅记录。
func (s *Store) SaveCreationReview(ctx context.Context, expected, updated creationdomain.CandidateReview) error {
	accepted, rejected, err := encodeReviewRefs(updated)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return databaseOperationError("begin creation review", err)
	}
	defer tx.Rollback()
	var existing int64
	queryErr := tx.QueryRowContext(ctx,
		`SELECT review_version FROM creation_candidate_reviews WHERE candidate_id = ?`,
		updated.CandidateID,
	).Scan(&existing)
	if queryErr != nil && !errors.Is(queryErr, sql.ErrNoRows) {
		return databaseOperationError("read creation review", queryErr)
	}
	if errors.Is(queryErr, sql.ErrNoRows) {
		if expected.ReviewVersion != 0 {
			return creationdomain.ErrReviewVersionConflict
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO creation_candidate_reviews (
				candidate_id, session_id, stage, review_version, accepted_json, rejected_json, created_at, updated_at
			) VALUES (?, ?, ?, 1, ?, ?, ?, ?)
		`,
			updated.CandidateID, updated.SessionID, string(updated.Stage), string(accepted), string(rejected),
			formatTime(updated.UpdatedAt), formatTime(updated.UpdatedAt),
		); err != nil {
			return databaseOperationError("insert creation review", err)
		}
	} else {
		result, err := tx.ExecContext(ctx, `
			UPDATE creation_candidate_reviews SET
				accepted_json = ?, rejected_json = ?, review_version = review_version + 1, updated_at = ?
			WHERE candidate_id = ? AND review_version = ? AND session_id = ?
		`,
			string(accepted), string(rejected), formatTime(updated.UpdatedAt),
			updated.CandidateID, expected.ReviewVersion, updated.SessionID)
		if err != nil {
			return databaseOperationError("update creation review", err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return databaseOperationError("read creation review update result", err)
		}
		if count != 1 {
			return creationdomain.ErrReviewVersionConflict
		}
	}
	if err := tx.Commit(); err != nil {
		return databaseOperationError("commit creation review", err)
	}
	return nil
}

// CreationReview 读取候选的逐项审阅关系；未记录时返回零值，调用方按未审阅处理。
func (s *Store) CreationReview(ctx context.Context, candidateID string) (creationdomain.CandidateReview, error) {
	var review creationdomain.CandidateReview
	var acceptedJSON, rejectedJSON, createdAt, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT candidate_id, session_id, stage, review_version, accepted_json, rejected_json, created_at, updated_at
		FROM creation_candidate_reviews
		WHERE candidate_id = ?
	`, candidateID).Scan(
		&review.CandidateID, &review.SessionID, &review.Stage, &review.ReviewVersion,
		&acceptedJSON, &rejectedJSON, &createdAt, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return creationdomain.CandidateReview{}, nil
		}
		return creationdomain.CandidateReview{}, fmt.Errorf("read creation review: %w", err)
	}
	if err := decodeReviewRefs(acceptedJSON, &review.Accepted); err != nil {
		return creationdomain.CandidateReview{}, err
	}
	if err := decodeReviewRefs(rejectedJSON, &review.Rejected); err != nil {
		return creationdomain.CandidateReview{}, err
	}
	if review.CreatedAt, err = parseTime(createdAt); err != nil {
		return creationdomain.CandidateReview{}, fmt.Errorf("parse creation review created_at: %w", err)
	}
	if review.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return creationdomain.CandidateReview{}, fmt.Errorf("parse creation review updated_at: %w", err)
	}
	return review, nil
}

// SaveCreationReviewCursor 以调用方读到的 view_version 为前置条件保存审阅位置；
// 行不存在时建立初始位置，不覆盖其他会话。
func (s *Store) SaveCreationReviewCursor(ctx context.Context, cursor creationdomain.ReviewCursor) error {
	_, err := s.CreationReviewCursor(ctx, cursor.SessionID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err := s.db.ExecContext(ctx, `
			INSERT INTO creation_review_cursors (
				session_id, view_version, stage, candidate_id, entity_id, section_id, comparison_ref, updated_at
			) VALUES (?, 1, ?, ?, ?, ?, ?, ?)
		`,
			cursor.SessionID, string(cursor.Stage), cursor.CandidateID, cursor.EntityID, cursor.SectionID,
			cursor.ComparisonRef, formatTime(cursor.UpdatedAt),
		); err != nil {
			return databaseOperationError("insert creation review cursor", err)
		}
		return nil
	case err != nil:
		return err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE creation_review_cursors SET
			stage = ?, candidate_id = ?, entity_id = ?, section_id = ?, comparison_ref = ?,
			view_version = view_version + 1, updated_at = ?
		WHERE session_id = ? AND view_version = ?
	`,
		string(cursor.Stage), cursor.CandidateID, cursor.EntityID, cursor.SectionID, cursor.ComparisonRef,
		formatTime(cursor.UpdatedAt), cursor.SessionID, cursor.ViewVersion)
	if err != nil {
		return databaseOperationError("update creation review cursor", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return databaseOperationError("read creation review cursor result", err)
	}
	if count != 1 {
		return creationdomain.ErrReviewVersionConflict
	}
	return nil
}

// CreationReviewCursor 读取会话审阅位置；未记录时返回零值。
func (s *Store) CreationReviewCursor(ctx context.Context, sessionID string) (creationdomain.ReviewCursor, error) {
	var cursor creationdomain.ReviewCursor
	var stage, updatedAt string
	err := s.db.QueryRowContext(ctx, `
		SELECT session_id, view_version, stage, candidate_id, entity_id, section_id, comparison_ref, updated_at
		FROM creation_review_cursors
		WHERE session_id = ?
	`, sessionID).Scan(
		&cursor.SessionID, &cursor.ViewVersion, &stage, &cursor.CandidateID,
		&cursor.EntityID, &cursor.SectionID, &cursor.ComparisonRef, &updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return creationdomain.ReviewCursor{}, sql.ErrNoRows
		}
		return creationdomain.ReviewCursor{}, fmt.Errorf("read creation review cursor: %w", err)
	}
	cursor.Stage = creationdomain.Stage(stage)
	if cursor.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return creationdomain.ReviewCursor{}, fmt.Errorf("parse creation review cursor updated_at: %w", err)
	}
	return cursor, nil
}

func encodeReviewRefs(review creationdomain.CandidateReview) ([]byte, []byte, error) {
	accepted, err := json.Marshal(review.Accepted)
	if err != nil {
		return nil, nil, fmt.Errorf("encode creation review accepted refs: %w", err)
	}
	if string(accepted) == "null" {
		accepted = []byte("[]")
	}
	rejected, err := json.Marshal(review.Rejected)
	if err != nil {
		return nil, nil, fmt.Errorf("encode creation review rejected refs: %w", err)
	}
	if string(rejected) == "null" {
		rejected = []byte("[]")
	}
	return accepted, rejected, nil
}

func decodeReviewRefs(encoded string, target *[]creationdomain.ReviewRef) error {
	refs := []creationdomain.ReviewRef{}
	if err := json.Unmarshal([]byte(encoded), &refs); err != nil {
		return fmt.Errorf("decode creation review refs: %w", err)
	}
	*target = refs
	return nil
}
