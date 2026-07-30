package projectfs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/importing"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

func TestImportSourceArchivePreparesSupportedEncodings(t *testing.T) {
	const text = "第一章 标题\r\n正文"

	utf16LE, err := unicode.UTF16(unicode.LittleEndian, unicode.UseBOM).NewEncoder().Bytes([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	utf16BE, err := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewEncoder().Bytes([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	gb18030, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(text))
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		raw      []byte
		encoding string
	}{
		{name: "UTF-8", raw: []byte(text), encoding: "utf-8"},
		{name: "UTF-8 BOM", raw: append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...), encoding: "utf-8"},
		{name: "UTF-16 LE BOM", raw: utf16LE, encoding: "utf-16le"},
		{name: "UTF-16 BE BOM", raw: utf16BE, encoding: "utf-16be"},
		{name: "GB18030", raw: gb18030, encoding: "gb18030"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectPackage := createTestPackage(t)
			sourcePath := filepath.Join(t.TempDir(), "用户书名.txt")
			writeTestFile(t, sourcePath, test.raw)

			prepared, err := (ImportSourceArchive{}).Prepare(
				context.Background(),
				projectPackage,
				"job-1",
				sourcePath,
			)
			if err != nil {
				t.Fatal(err)
			}

			sum := sha256.Sum256(test.raw)
			if prepared.Name != "用户书名.txt" {
				t.Fatalf("Name = %q", prepared.Name)
			}
			if prepared.Format != importing.SourceFormatTXT {
				t.Fatalf("Format = %q", prepared.Format)
			}
			if prepared.Encoding != test.encoding {
				t.Fatalf("Encoding = %q, want %q", prepared.Encoding, test.encoding)
			}
			if prepared.SHA256 != hex.EncodeToString(sum[:]) {
				t.Fatalf("SHA256 = %q", prepared.SHA256)
			}
			if prepared.Bytes != int64(len(test.raw)) {
				t.Fatalf("Bytes = %d, want %d", prepared.Bytes, len(test.raw))
			}
			if prepared.ArchivedRelativePath != "imports/job-1.txt" {
				t.Fatalf("ArchivedRelativePath = %q", prepared.ArchivedRelativePath)
			}
			if prepared.Text != text {
				t.Fatalf("Text = %q, want %q", prepared.Text, text)
			}

			archivedPath := filepath.Join(projectPackage.Root, filepath.FromSlash(prepared.ArchivedRelativePath))
			archived, err := os.ReadFile(archivedPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(archived, test.raw) {
				t.Fatal("archived source bytes changed")
			}
			info, err := os.Stat(archivedPath)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0o600 {
				t.Fatalf("archive mode = %o, want 600", got)
			}
		})
	}
}

func TestImportSourceArchivePreservesSupportedFormatExtension(t *testing.T) {
	tests := []struct {
		sourceName string
		format     importing.SourceFormat
		relative   string
	}{
		{sourceName: "novel.txt", format: importing.SourceFormatTXT, relative: "imports/job.txt"},
		{sourceName: "novel.md", format: importing.SourceFormatMarkdown, relative: "imports/job.md"},
		{sourceName: "novel.markdown", format: importing.SourceFormatMarkdown, relative: "imports/job.markdown"},
		{sourceName: "NOVEL.MD", format: importing.SourceFormatMarkdown, relative: "imports/job.md"},
	}

	for _, test := range tests {
		t.Run(test.sourceName, func(t *testing.T) {
			projectPackage := createTestPackage(t)
			sourcePath := filepath.Join(t.TempDir(), test.sourceName)
			writeTestFile(t, sourcePath, []byte("正文"))

			prepared, err := (ImportSourceArchive{}).Prepare(
				context.Background(),
				projectPackage,
				"job",
				sourcePath,
			)
			if err != nil {
				t.Fatal(err)
			}

			if prepared.Format != test.format {
				t.Fatalf("Format = %q, want %q", prepared.Format, test.format)
			}
			if prepared.ArchivedRelativePath != test.relative {
				t.Fatalf("ArchivedRelativePath = %q, want %q", prepared.ArchivedRelativePath, test.relative)
			}
		})
	}
}

func TestImportSourceArchiveDoesNotModifySource(t *testing.T) {
	projectPackage := createTestPackage(t)
	sourcePath := filepath.Join(t.TempDir(), "novel.txt")
	original := []byte("第一章\n正文")
	writeTestFile(t, sourcePath, original)
	if err := os.Chmod(sourcePath, 0o640); err != nil {
		t.Fatal(err)
	}

	if _, err := (ImportSourceArchive{}).Prepare(
		context.Background(),
		projectPackage,
		"job",
		sourcePath,
	); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("source contents changed")
	}
	info, err := os.Stat(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0o640 {
		t.Fatalf("source mode = %o, want 640", got)
	}
}

func TestImportSourceArchiveAcceptsMaximumSize(t *testing.T) {
	projectPackage := createTestPackage(t)
	sourcePath := filepath.Join(t.TempDir(), "maximum.txt")
	raw := bytes.Repeat([]byte("a"), importing.MaxManuscriptBytes)
	writeTestFile(t, sourcePath, raw)

	prepared, err := (ImportSourceArchive{}).Prepare(
		context.Background(),
		projectPackage,
		"maximum",
		sourcePath,
	)
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Bytes != int64(importing.MaxManuscriptBytes) {
		t.Fatalf("Bytes = %d", prepared.Bytes)
	}
}

func TestImportSourceArchiveRejectsUnsafeSources(t *testing.T) {
	projectPackage := createTestPackage(t)
	root := t.TempDir()
	emptyPath := filepath.Join(root, "empty.txt")
	writeTestFile(t, emptyPath, nil)
	tooLargePath := filepath.Join(root, "large.txt")
	writeTestFile(t, tooLargePath, nil)
	if err := os.Truncate(tooLargePath, int64(importing.MaxManuscriptBytes+1)); err != nil {
		t.Fatal(err)
	}
	unsupportedPath := filepath.Join(root, "novel.docx")
	writeTestFile(t, unsupportedPath, []byte("正文"))
	invalidEncodingPath := filepath.Join(root, "invalid.txt")
	writeTestFile(t, invalidEncodingPath, []byte{0xff, 0xff, 0xff})
	invalidUTF16Path := filepath.Join(root, "invalid-utf16.txt")
	writeTestFile(t, invalidUTF16Path, []byte{0xff, 0xfe, 0x00, 0xd8})
	bomlessUTF16Path := filepath.Join(root, "bomless-utf16.txt")
	bomlessUTF16, err := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder().Bytes(
		[]byte("novel"),
	)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, bomlessUTF16Path, bomlessUTF16)
	bomOnlyPath := filepath.Join(root, "bom-only.txt")
	writeTestFile(t, bomOnlyPath, []byte{0xef, 0xbb, 0xbf})
	directoryPath := filepath.Join(root, "directory.txt")
	if err := os.Mkdir(directoryPath, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkPath := filepath.Join(root, "symlink.txt")

	tests := []struct {
		name       string
		sourcePath string
		want       error
	}{
		{name: "unsupported format", sourcePath: unsupportedPath, want: ErrUnsupportedImportFormat},
		{name: "empty", sourcePath: emptyPath, want: ErrEmptyImportSource},
		{name: "too large", sourcePath: tooLargePath, want: ErrImportSourceTooLarge},
		{name: "unsupported encoding", sourcePath: invalidEncodingPath, want: ErrUnsupportedImportEncoding},
		{name: "invalid UTF-16", sourcePath: invalidUTF16Path, want: ErrUnsupportedImportEncoding},
		{name: "BOM-less UTF-16", sourcePath: bomlessUTF16Path, want: ErrUnsupportedImportEncoding},
		{name: "BOM only", sourcePath: bomOnlyPath, want: ErrEmptyImportSource},
		{name: "directory", sourcePath: directoryPath, want: ErrUnsafeImportSource},
	}
	if err := os.Symlink(filepath.Join(root, "real.txt"), symlinkPath); err == nil {
		tests = append(tests, struct {
			name       string
			sourcePath string
			want       error
		}{name: "symlink", sourcePath: symlinkPath, want: ErrUnsafeImportSource})
	} else if runtime.GOOS == "windows" {
		t.Logf("symlink rejection not exercised without symlink permission: %v", err)
	} else {
		t.Fatal(err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := (ImportSourceArchive{}).Prepare(
				context.Background(),
				projectPackage,
				"job-"+strings.ReplaceAll(test.name, " ", "-"),
				test.sourcePath,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}

	entries, err := os.ReadDir(projectPackage.Imports)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("unsafe sources left archives: %#v", entries)
	}
}

func TestImportSourceArchiveRejectsUnsafeJobID(t *testing.T) {
	projectPackage := createTestPackage(t)
	sourcePath := filepath.Join(t.TempDir(), "novel.txt")
	writeTestFile(t, sourcePath, []byte("正文"))

	_, err := (ImportSourceArchive{}).Prepare(
		context.Background(),
		projectPackage,
		"../outside",
		sourcePath,
	)

	if !errors.Is(err, ErrInvalidImportArchivePath) {
		t.Fatalf("error = %v, want ErrInvalidImportArchivePath", err)
	}
	if _, err := os.Stat(filepath.Join(projectPackage.Root, "outside.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("outside archive exists or Stat failed: %v", err)
	}
}

func TestImportSourceArchiveDoesNotOverwriteCollision(t *testing.T) {
	projectPackage := createTestPackage(t)
	sourcePath := filepath.Join(t.TempDir(), "novel.txt")
	writeTestFile(t, sourcePath, []byte("original"))
	archive := ImportSourceArchive{}
	prepared, err := archive.Prepare(context.Background(), projectPackage, "job", sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, sourcePath, []byte("replacement"))

	_, err = archive.Prepare(context.Background(), projectPackage, "job", sourcePath)
	if !errors.Is(err, ErrImportArchiveExists) {
		t.Fatalf("error = %v, want ErrImportArchiveExists", err)
	}

	archivedPath := filepath.Join(projectPackage.Root, filepath.FromSlash(prepared.ArchivedRelativePath))
	got, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("archived contents = %q", got)
	}
	entries, err := os.ReadDir(projectPackage.Imports)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "job.txt" {
		t.Fatalf("archive entries = %#v", entries)
	}
}

func TestRenameImportArchiveNoReplaceRejectsExistingTarget(t *testing.T) {
	projectPackage := createTestPackage(t)
	sourceName := ".import-source-test"
	targetName := "job.txt"
	writeTestFile(t, filepath.Join(projectPackage.Imports, sourceName), []byte("replacement"))
	writeTestFile(t, filepath.Join(projectPackage.Imports, targetName), []byte("original"))
	directory, err := os.Open(projectPackage.Imports)
	if err != nil {
		t.Fatal(err)
	}
	defer directory.Close()

	err = renameImportArchiveNoReplace(directory, sourceName, targetName)

	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("error = %v, want fs.ErrExist", err)
	}
	got, err := os.ReadFile(filepath.Join(projectPackage.Imports, targetName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("target contents = %q", got)
	}
	got, err = os.ReadFile(filepath.Join(projectPackage.Imports, sourceName))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "replacement" {
		t.Fatalf("source contents = %q", got)
	}
}

func TestImportSourceArchiveDeleteIsStrictAndIdempotent(t *testing.T) {
	projectPackage := createTestPackage(t)
	sourcePath := filepath.Join(t.TempDir(), "novel.md")
	writeTestFile(t, sourcePath, []byte("正文"))
	archive := ImportSourceArchive{}
	prepared, err := archive.Prepare(context.Background(), projectPackage, "job-1", sourcePath)
	if err != nil {
		t.Fatal(err)
	}

	if err := archive.Delete(context.Background(), projectPackage, prepared.ArchivedRelativePath); err != nil {
		t.Fatal(err)
	}
	if err := archive.Delete(context.Background(), projectPackage, prepared.ArchivedRelativePath); err != nil {
		t.Fatalf("idempotent delete: %v", err)
	}
}

func TestImportSourceArchiveDeleteRejectsPathsOutsideImports(t *testing.T) {
	projectPackage := createTestPackage(t)
	outsidePath := filepath.Join(projectPackage.Root, "outside.txt")
	writeTestFile(t, outsidePath, []byte("keep"))
	archive := ImportSourceArchive{}

	paths := []string{
		"../outside.txt",
		"imports/../outside.txt",
		"/imports/job.txt",
		"imports/job.exe",
		"imports/a/job.txt",
		"imports/job.name.txt",
		`imports\job.txt`,
		filepath.ToSlash(outsidePath),
	}
	for _, relativePath := range paths {
		t.Run(relativePath, func(t *testing.T) {
			err := archive.Delete(context.Background(), projectPackage, relativePath)
			if !errors.Is(err, ErrInvalidImportArchivePath) {
				t.Fatalf("error = %v, want ErrInvalidImportArchivePath", err)
			}
		})
	}

	got, err := os.ReadFile(outsidePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep" {
		t.Fatalf("outside contents = %q", got)
	}
}

func TestImportArchiveOperationsStayBoundToVerifiedImportsDirectory(t *testing.T) {
	projectPackage := createTestPackage(t)
	importsDirectory, err := openImportsDirectory(projectPackage)
	if err != nil {
		t.Fatal(err)
	}
	defer importsDirectory.Close()

	movedImports := filepath.Join(projectPackage.Root, "imports-moved")
	if err := os.Rename(projectPackage.Imports, movedImports); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, projectPackage.Imports); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("creating symlink requires permission: %v", err)
		}
		t.Fatal(err)
	}

	if err := archiveImportSource(
		context.Background(),
		importsDirectory,
		"job.txt",
		[]byte("archived"),
	); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(movedImports, "delete.txt"), []byte("inside"))
	writeTestFile(t, filepath.Join(outside, "delete.txt"), []byte("outside"))
	if err := deleteImportArchive(importsDirectory, "delete.txt"); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(movedImports, "job.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "archived" {
		t.Fatalf("archive contents = %q", got)
	}
	if _, err := os.Stat(filepath.Join(outside, "job.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("archive escaped verified directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(movedImports, "delete.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("verified archive was not deleted: %v", err)
	}
	got, err = os.ReadFile(filepath.Join(outside, "delete.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "outside" {
		t.Fatalf("outside contents = %q", got)
	}
}

func TestImportSourceArchiveHonorsCancelledContext(t *testing.T) {
	projectPackage := createTestPackage(t)
	sourcePath := filepath.Join(t.TempDir(), "novel.txt")
	writeTestFile(t, sourcePath, []byte("正文"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := (ImportSourceArchive{}).Prepare(ctx, projectPackage, "job", sourcePath)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func createTestPackage(t *testing.T) ports.ProjectPackage {
	t.Helper()
	projectPackage, err := (Manager{}).Create(t.TempDir(), "测试项目")
	if err != nil {
		t.Fatal(err)
	}
	return projectPackage
}

func writeTestFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

var _ ports.ImportSourceArchive = ImportSourceArchive{}
