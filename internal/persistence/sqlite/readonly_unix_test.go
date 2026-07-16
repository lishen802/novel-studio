//go:build !windows

package sqlite

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

func TestReadOnlyProjectReturnsPermissionErrorWithoutChangingData(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "project.db")
	project, err := projectdomain.NewProject(
		"project-1",
		"长夜织境",
		"东方奇幻",
		"",
		time.Unix(100, 0),
	)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := (Factory{}).Create(context.Background(), path, project)
	if err != nil {
		t.Fatal(err)
	}
	store := opened.(*Store)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(context.Background(), chapter); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(path, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(directory, 0o700)
	defer os.Chmod(path, 0o600)

	reopened, err := (Factory{}).Open(context.Background(), path)
	if err != nil {
		if !errors.Is(err, ErrDatabasePermission) {
			t.Fatalf("open error = %v, want ErrDatabasePermission", err)
		}
		return
	}
	readOnlyStore := reopened.(*Store)
	defer readOnlyStore.Close()

	err = readOnlyStore.SaveDraft(context.Background(), chapter.ID, "无法写入", chapter.Revision)
	if !errors.Is(err, ErrDatabasePermission) {
		t.Fatalf("SaveDraft error = %v, want ErrDatabasePermission", err)
	}
	committed, err := readOnlyStore.Chapter(context.Background(), chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if committed != chapter {
		t.Fatalf("committed chapter changed: %#v", committed)
	}
}
