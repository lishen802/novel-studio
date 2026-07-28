package revision

import (
	"errors"
	"testing"
	"time"
)

func TestNewNamedSnapshotNormalizesNameAndKeepsIdentity(t *testing.T) {
	now := time.Unix(200, 0)

	snapshot, err := NewSnapshot(
		"snapshot-1",
		"project-1",
		"  第一幕定稿  ",
		SnapshotNamed,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ID != "snapshot-1" || snapshot.ProjectID != "project-1" {
		t.Fatalf("snapshot identity = %#v", snapshot)
	}
	if snapshot.Name != "第一幕定稿" || snapshot.Kind != SnapshotNamed {
		t.Fatalf("snapshot metadata = %#v", snapshot)
	}
	if !snapshot.CreatedAt.Equal(now.UTC()) {
		t.Fatalf("created at = %v", snapshot.CreatedAt)
	}
}

func TestNewSnapshotRejectsBlankNameAndUnknownKind(t *testing.T) {
	_, err := NewSnapshot("snapshot-1", "project-1", " ", SnapshotNamed, time.Now())
	if !errors.Is(err, ErrBlankValue) {
		t.Fatalf("blank name error = %v", err)
	}

	_, err = NewSnapshot("snapshot-1", "project-1", "版本", SnapshotKind("other"), time.Now())
	if !errors.Is(err, ErrInvalidSnapshotKind) {
		t.Fatalf("invalid kind error = %v", err)
	}
}
