package creation

import (
	"context"
	"database/sql"
	"errors"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type SaveEntryRequest struct {
	ProjectID              string                   `json:"projectId"`
	SessionID              string                   `json:"sessionId"`
	ExpectedVersion        int64                    `json:"expectedVersion"`
	ExpectedRunSlotVersion int64                    `json:"expectedRunSlotVersion"`
	Kind                   creationdomain.EntryKind `json:"kind"`
	Material               string                   `json:"material"`
}

type EntryView struct {
	ID       string                   `json:"id"`
	Version  int64                    `json:"version"`
	Kind     creationdomain.EntryKind `json:"kind"`
	Material string                   `json:"material"`
}

// loadSession 以内容版本检查来源头与会话快照一致，防止读到切换前后的混合状态。
func (service *Service) loadSession(ctx context.Context, projectID string) (creationdomain.Session, error) {
	session, err := service.store.Session(ctx, projectID)
	if err != nil {
		return session, err
	}
	session.Entry = nil
	entry, err := service.store.CurrentEntry(ctx, session.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return session, err
	}
	if err == nil {
		session.Entry = &entry
	}
	latest, err := service.store.Session(ctx, projectID)
	if err != nil {
		return session, err
	}
	if latest.ContentVersion != session.ContentVersion {
		return session, creationdomain.ErrSessionVersionConflict
	}
	return session, nil
}

// SaveEntry 保存来源并暂停为分步审阅；活动运行必须先取消与对账，不自动重放费用。
func (service *Service) SaveEntry(ctx context.Context, request SaveEntryRequest) (creationdomain.EntryRevision, error) {
	session, err := service.loadSession(ctx, request.ProjectID)
	if err != nil {
		return creationdomain.EntryRevision{}, err
	}
	if session.ID != request.SessionID || session.ContentVersion != request.ExpectedVersion || session.RunSlotVersion != request.ExpectedRunSlotVersion {
		return creationdomain.EntryRevision{}, creationdomain.ErrSessionVersionConflict
	}
	if session.ActiveRunID != "" || session.ActiveOperationID != "" {
		return creationdomain.EntryRevision{}, ErrRunActive
	}
	var entry creationdomain.EntryRevision
	if session.Entry == nil {
		entry, err = creationdomain.NewEntryRevision(service.ids.NewID(), session.ID, "", 1, request.Kind, request.Material, service.clock.Now())
	} else {
		entry, err = session.Entry.Revise(service.ids.NewID(), request.Kind, request.Material, service.clock.Now())
	}
	if err != nil {
		return entry, err
	}
	if err := service.store.AppendEntry(ctx, session, entry); err != nil {
		return creationdomain.EntryRevision{}, err
	}
	latest, err := service.loadSession(ctx, request.ProjectID)
	if err != nil {
		return entry, err
	}
	if err := service.refreshImpact(ctx, latest); err != nil {
		return entry, err
	}
	return entry, nil
}
