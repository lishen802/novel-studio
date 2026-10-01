package projectfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManuscriptExportIsUniqueAndPreservesExistingDelivery(t *testing.T) {
	manager := Manager{}
	root := t.TempDir()
	first, err := manager.WriteManuscript(root, "txt", "第一份稿件")
	if err != nil {
		t.Fatal(err)
	}
	second, err := manager.WriteManuscript(root, "txt", "第二份稿件")
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(first)
	if err != nil || first == second || string(body) != "第一份稿件" {
		t.Fatalf("existing export changed: %s %v", body, err)
	}
	if path, err := manager.WriteManuscript(filepath.Join(root, "missing"), "txt", "失败稿"); err == nil || path != "" {
		t.Fatalf("failed export reported success: %q %v", path, err)
	}
	if _, err := manager.WriteManuscript(root, "../bad", "越界"); err == nil {
		t.Fatal("invalid extension accepted")
	}
}
