package chapterloop

import (
	"context"
	"encoding/json"

	loopdomain "github.com/lishen802/novel-studio/internal/domain/chapterloop"
)

// Backend 是 SQLite 项目库上的单章记录端口。
type Backend interface {
	AppendChapterLoop(context.Context, loopdomain.StoredRecord) error
	LatestChapterLoop(context.Context, string, string, string) (loopdomain.StoredRecord, error)
	GetChapterLoop(context.Context, string, string) (loopdomain.StoredRecord, error)
	ListChapterLoop(context.Context, string, string) ([]loopdomain.StoredRecord, error)
}

// NewBackendStore 把项目库端口适配成应用记录存储。
func NewBackendStore(backend Backend) Store {
	return backendStore{backend: backend}
}

type backendStore struct {
	backend Backend
}

func (store backendStore) Append(ctx context.Context, record Record) error {
	return store.backend.AppendChapterLoop(ctx, loopdomain.StoredRecord{
		ID: record.ID, ProjectID: record.ProjectID, Kind: record.Kind, SubjectID: record.SubjectID,
		IdempotencyKey: record.IdempotencyKey, Payload: string(record.Payload),
	})
}

func (store backendStore) Latest(ctx context.Context, projectID, kind, subjectID string) (Record, error) {
	record, err := store.backend.LatestChapterLoop(ctx, projectID, kind, subjectID)
	return storedToRecord(record), err
}

func (store backendStore) Get(ctx context.Context, projectID, id string) (Record, error) {
	record, err := store.backend.GetChapterLoop(ctx, projectID, id)
	return storedToRecord(record), err
}

func (store backendStore) List(ctx context.Context, projectID, kind string) ([]Record, error) {
	records, err := store.backend.ListChapterLoop(ctx, projectID, kind)
	if err != nil {
		return nil, err
	}
	result := make([]Record, 0, len(records))
	for _, record := range records {
		result = append(result, storedToRecord(record))
	}
	return result, nil
}

func storedToRecord(record loopdomain.StoredRecord) Record {
	return Record{
		ID: record.ID, ProjectID: record.ProjectID, Kind: record.Kind, SubjectID: record.SubjectID,
		IdempotencyKey: record.IdempotencyKey, Payload: json.RawMessage(record.Payload),
	}
}
