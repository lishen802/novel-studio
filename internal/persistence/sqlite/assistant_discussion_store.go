package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
)

const assistantAuthorizationSelect = `
		SELECT id, session_id, project_id, branch_id, action_id, candidate_id, candidate_version,
			candidate_sha256, stage, entity_ref, field_paths_json, profile_version_id, provider_id,
			provider_name, model, reasoning_effort, max_calls, max_total_tokens, max_turn_tokens,
			reserved_calls, reserved_tokens, used_calls, used_tokens, state, expires_at, created_at, updated_at
		FROM assistant_authorizations
`

// SaveAssistantAuthorization 持久化讨论授权快照，不把额度放在内存里。
func (s *Store) SaveAssistantAuthorization(ctx context.Context, auth assistantdomain.Authorization) error {
	paths, err := json.Marshal(auth.FieldPaths)
	if err != nil {
		return fmt.Errorf("encode authorization field paths: %w", err)
	}
	if string(paths) == "null" {
		paths = []byte("[]")
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO assistant_authorizations (
			id, session_id, project_id, branch_id, action_id, candidate_id, candidate_version,
			candidate_sha256, stage, entity_ref, field_paths_json, profile_version_id, provider_id,
			provider_name, model, reasoning_effort, max_calls, max_total_tokens, max_turn_tokens,
			reserved_calls, reserved_tokens, used_calls, used_tokens, state, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			reserved_calls = excluded.reserved_calls,
			reserved_tokens = excluded.reserved_tokens,
			used_calls = excluded.used_calls,
			used_tokens = excluded.used_tokens,
			state = excluded.state,
			updated_at = excluded.updated_at
	`, auth.ID, auth.SessionID, auth.ProjectID, auth.BranchID, auth.ActionID, auth.CandidateID,
		auth.CandidateVersion, auth.CandidateSHA256, auth.Stage, auth.EntityRef, string(paths),
		auth.ProfileVersionID, auth.ProviderID, auth.ProviderName, auth.Model, auth.ReasoningEffort,
		auth.MaxCalls, auth.MaxTotalTokens, auth.MaxTurnTokens, auth.ReservedCalls, auth.ReservedTokens,
		auth.UsedCalls, auth.UsedTokens, string(auth.State), formatTime(auth.ExpiresAt),
		formatTime(auth.CreatedAt), formatTime(auth.UpdatedAt))
	if err != nil {
		return databaseOperationError("save assistant authorization", err)
	}
	return nil
}

// AssistantAuthorization 读取一条讨论授权。
func (s *Store) AssistantAuthorization(ctx context.Context, id string) (assistantdomain.Authorization, error) {
	return s.scanAssistantAuthorization(s.db.QueryRowContext(ctx, assistantAuthorizationSelect+` WHERE id = ?`, strings.TrimSpace(id)))
}

// AssistantAuthorizationByAction 按确认 Action 恢复授权。
func (s *Store) AssistantAuthorizationByAction(ctx context.Context, actionID string) (assistantdomain.Authorization, error) {
	return s.scanAssistantAuthorization(s.db.QueryRowContext(ctx, assistantAuthorizationSelect+` WHERE action_id = ?`, strings.TrimSpace(actionID)))
}

// ListAssistantAuthorizations 列出会话内全部讨论授权。
func (s *Store) ListAssistantAuthorizations(ctx context.Context, sessionID string) ([]assistantdomain.Authorization, error) {
	rows, err := s.db.QueryContext(ctx, assistantAuthorizationSelect+`
		WHERE session_id = ?
		ORDER BY created_at, id
	`, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, databaseOperationError("list assistant authorizations", err)
	}
	defer rows.Close()
	items := make([]assistantdomain.Authorization, 0)
	for rows.Next() {
		auth, err := scanAssistantAuthorizationRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, auth)
	}
	if err := rows.Err(); err != nil {
		return nil, databaseOperationError("iterate assistant authorizations", err)
	}
	return items, nil
}

func (s *Store) scanAssistantAuthorization(row *sql.Row) (assistantdomain.Authorization, error) {
	auth, err := scanAssistantAuthorizationRow(row)
	if err != nil {
		return assistantdomain.Authorization{}, fmt.Errorf("read assistant authorization: %w", err)
	}
	return auth, nil
}

type authorizationRow interface {
	Scan(dest ...any) error
}

func scanAssistantAuthorizationRow(row authorizationRow) (assistantdomain.Authorization, error) {
	var auth assistantdomain.Authorization
	var paths, expiresAt, createdAt, updatedAt string
	err := row.Scan(
		&auth.ID, &auth.SessionID, &auth.ProjectID, &auth.BranchID, &auth.ActionID, &auth.CandidateID,
		&auth.CandidateVersion, &auth.CandidateSHA256, &auth.Stage, &auth.EntityRef, &paths,
		&auth.ProfileVersionID, &auth.ProviderID, &auth.ProviderName, &auth.Model, &auth.ReasoningEffort,
		&auth.MaxCalls, &auth.MaxTotalTokens, &auth.MaxTurnTokens, &auth.ReservedCalls, &auth.ReservedTokens,
		&auth.UsedCalls, &auth.UsedTokens, &auth.State, &expiresAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return assistantdomain.Authorization{}, err
	}
	if strings.TrimSpace(paths) == "" {
		paths = "[]"
	}
	if err := json.Unmarshal([]byte(paths), &auth.FieldPaths); err != nil {
		return assistantdomain.Authorization{}, fmt.Errorf("decode authorization field paths: %w", err)
	}
	auth.ExpiresAt, err = parseTime(expiresAt)
	if err != nil {
		return assistantdomain.Authorization{}, fmt.Errorf("parse authorization expires_at: %w", err)
	}
	auth.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return assistantdomain.Authorization{}, fmt.Errorf("parse authorization created_at: %w", err)
	}
	auth.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return assistantdomain.Authorization{}, fmt.Errorf("parse authorization updated_at: %w", err)
	}
	return auth, nil
}

type assistantMessageRow interface {
	Scan(dest ...any) error
}

func scanAssistantMessage(row assistantMessageRow) (assistantdomain.Message, error) {
	var message assistantdomain.Message
	var createdAt, updatedAt, suggestions string
	err := row.Scan(&message.ID, &message.SessionID, &message.Role, &message.Text,
		&message.Stage, &message.CandidateID, &message.EntityRef, &message.FieldPath,
		&message.ActionID, &message.IdempotencyKey, &createdAt, &message.Kind, &message.Status,
		&message.ReplyToID, &message.AuthorizationID, &message.RunID, &message.OperationID,
		&message.ErrorCode, &suggestions, &updatedAt)
	if err != nil {
		return assistantdomain.Message{}, databaseOperationError("scan assistant message", err)
	}
	message.Kind = messageKindOrDefault(message.Kind)
	message.Status = messageStatusOrDefault(message.Status)
	if strings.TrimSpace(suggestions) == "" {
		suggestions = "[]"
	}
	if err := json.Unmarshal([]byte(suggestions), &message.Suggestions); err != nil {
		return assistantdomain.Message{}, fmt.Errorf("decode assistant suggestions: %w", err)
	}
	message.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return assistantdomain.Message{}, fmt.Errorf("parse assistant message created_at: %w", err)
	}
	if strings.TrimSpace(updatedAt) == "" {
		message.UpdatedAt = message.CreatedAt
	} else {
		message.UpdatedAt, err = parseTime(updatedAt)
		if err != nil {
			return assistantdomain.Message{}, fmt.Errorf("parse assistant message updated_at: %w", err)
		}
	}
	return message, nil
}

func encodeSuggestions(items []assistantdomain.Suggestion) ([]byte, error) {
	if items == nil {
		return []byte("[]"), nil
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("encode assistant suggestions: %w", err)
	}
	if string(encoded) == "null" {
		return []byte("[]"), nil
	}
	return encoded, nil
}

func messageKindOrDefault(kind assistantdomain.MessageKind) assistantdomain.MessageKind {
	if kind == "" {
		return assistantdomain.KindRevisionRequest
	}
	return kind
}

func messageStatusOrDefault(status assistantdomain.MessageStatus) assistantdomain.MessageStatus {
	if status == "" {
		return assistantdomain.MessageAnswered
	}
	return status
}
