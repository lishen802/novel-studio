package projectfs

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/importing"
	"golang.org/x/text/encoding/simplifiedchinese"
	textunicode "golang.org/x/text/encoding/unicode"
)

const (
	importEncodingUTF8    = "utf-8"
	importEncodingUTF16LE = "utf-16le"
	importEncodingUTF16BE = "utf-16be"
	importEncodingGB18030 = "gb18030"

	importTemporaryOrphanAge = 24 * time.Hour
)

var (
	ErrUnsupportedImportFormat   = ports.ErrUnsupportedImportFormat
	ErrEmptyImportSource         = ports.ErrEmptyImportSource
	ErrImportSourceTooLarge      = ports.ErrImportSourceTooLarge
	ErrUnsupportedImportEncoding = ports.ErrUnsupportedImportEncoding
	ErrUnsafeImportSource        = ports.ErrUnsafeImportSource
	ErrImportArchiveExists       = ports.ErrImportArchiveExists
	ErrInvalidImportArchivePath  = ports.ErrInvalidImportArchivePath
)

type ImportSourceArchive struct{}

type importArchiveDirectory struct {
	root   *os.Root
	handle *os.File
}

func (directory *importArchiveDirectory) Close() error {
	rootErr := directory.root.Close()
	handleErr := directory.handle.Close()
	if rootErr != nil {
		return rootErr
	}
	return handleErr
}

func (ImportSourceArchive) Prepare(
	ctx context.Context,
	projectPackage ports.ProjectPackage,
	jobID string,
	sourcePath string,
) (ports.PreparedImportSource, error) {
	if err := ctx.Err(); err != nil {
		return ports.PreparedImportSource{}, err
	}
	format, extension, err := importFormat(sourcePath)
	if err != nil {
		return ports.PreparedImportSource{}, err
	}
	if err := validateImportJobID(jobID); err != nil {
		return ports.PreparedImportSource{}, err
	}
	importsDirectory, err := openImportsDirectory(projectPackage)
	if err != nil {
		return ports.PreparedImportSource{}, err
	}
	defer importsDirectory.Close()
	if err := ctx.Err(); err != nil {
		return ports.PreparedImportSource{}, err
	}
	if err := cleanupOrphanImportTemporaryFiles(importsDirectory); err != nil {
		return ports.PreparedImportSource{}, err
	}

	raw, err := readImportSource(ctx, sourcePath)
	if err != nil {
		return ports.PreparedImportSource{}, err
	}
	if err := ctx.Err(); err != nil {
		return ports.PreparedImportSource{}, err
	}
	text, encoding, err := decodeImportSource(raw)
	if err != nil {
		return ports.PreparedImportSource{}, err
	}
	if err := ctx.Err(); err != nil {
		return ports.PreparedImportSource{}, err
	}
	sum := sha256.Sum256(raw)
	if err := ctx.Err(); err != nil {
		return ports.PreparedImportSource{}, err
	}

	relativePath := path.Join("imports", jobID+extension)
	if err := archiveImportSource(ctx, importsDirectory, jobID+extension, raw); err != nil {
		return ports.PreparedImportSource{}, err
	}

	return ports.PreparedImportSource{
		Name:                 filepath.Base(sourcePath),
		Format:               format,
		Encoding:             encoding,
		SHA256:               hex.EncodeToString(sum[:]),
		Bytes:                int64(len(raw)),
		ArchivedRelativePath: relativePath,
		Text:                 text,
	}, nil
}

func (ImportSourceArchive) Delete(
	ctx context.Context,
	projectPackage ports.ProjectPackage,
	relativePath string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	importsDirectory, err := openImportsDirectory(projectPackage)
	if err != nil {
		return err
	}
	defer importsDirectory.Close()
	fileName, err := validateImportArchiveRelativePath(relativePath)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return deleteImportArchive(importsDirectory, fileName)
}

func importFormat(sourcePath string) (importing.SourceFormat, string, error) {
	if strings.TrimSpace(sourcePath) == "" {
		return "", "", fmt.Errorf("%w: source path is blank", ErrUnsafeImportSource)
	}
	switch strings.ToLower(filepath.Ext(sourcePath)) {
	case ".txt":
		return importing.SourceFormatTXT, ".txt", nil
	case ".md":
		return importing.SourceFormatMarkdown, ".md", nil
	case ".markdown":
		return importing.SourceFormatMarkdown, ".markdown", nil
	default:
		return "", "", fmt.Errorf("%w: allowed extensions are .txt, .md, and .markdown", ErrUnsupportedImportFormat)
	}
}

func validateImportJobID(jobID string) error {
	if len(jobID) == 0 || len(jobID) > 128 {
		return fmt.Errorf("%w: invalid job id length", ErrInvalidImportArchivePath)
	}
	for index := range len(jobID) {
		value := jobID[index]
		if value >= 'a' && value <= 'z' ||
			value >= 'A' && value <= 'Z' ||
			value >= '0' && value <= '9' ||
			value == '-' ||
			value == '_' {
			continue
		}
		return fmt.Errorf("%w: job id contains an unsafe character", ErrInvalidImportArchivePath)
	}
	return nil
}

func openImportsDirectory(projectPackage ports.ProjectPackage) (*importArchiveDirectory, error) {
	root, err := filepath.Abs(filepath.Clean(projectPackage.Root))
	if err != nil || strings.TrimSpace(projectPackage.Root) == "" {
		return nil, fmt.Errorf("%w: invalid project root", ErrInvalidImportArchivePath)
	}
	importsDirectory, err := filepath.Abs(filepath.Clean(projectPackage.Imports))
	if err != nil || strings.TrimSpace(projectPackage.Imports) == "" {
		return nil, fmt.Errorf("%w: invalid imports directory", ErrInvalidImportArchivePath)
	}
	if importsDirectory != filepath.Join(root, "imports") {
		return nil, fmt.Errorf("%w: imports directory is outside the project package", ErrInvalidImportArchivePath)
	}

	rootInfo, err := os.Lstat(root)
	if err != nil {
		return nil, importArchiveFilesystemError("inspect project package", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, fmt.Errorf("%w: project root is not a real directory", ErrInvalidImportArchivePath)
	}
	projectRoot, err := os.OpenRoot(root)
	if err != nil {
		return nil, importArchiveFilesystemError("open project package", err)
	}
	closeProjectRoot := true
	defer func() {
		if closeProjectRoot {
			_ = projectRoot.Close()
		}
	}()
	openedRootInfo, err := projectRoot.Stat(".")
	if err != nil || !os.SameFile(rootInfo, openedRootInfo) {
		return nil, fmt.Errorf("%w: project root changed while opening", ErrInvalidImportArchivePath)
	}

	info, err := projectRoot.Lstat("imports")
	if errors.Is(err, fs.ErrNotExist) {
		if err := projectRoot.Mkdir("imports", 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, importArchiveFilesystemError("create imports directory", err)
		}
		info, err = projectRoot.Lstat("imports")
	}
	if err != nil {
		return nil, importArchiveFilesystemError("inspect imports directory", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("%w: imports path is not a real directory", ErrInvalidImportArchivePath)
	}
	importsRoot, err := projectRoot.OpenRoot("imports")
	if err != nil {
		return nil, importArchiveFilesystemError("open imports directory", err)
	}
	openedImportsInfo, err := importsRoot.Stat(".")
	if err != nil || !os.SameFile(info, openedImportsInfo) {
		_ = importsRoot.Close()
		return nil, fmt.Errorf("%w: imports directory changed while opening", ErrInvalidImportArchivePath)
	}
	importsHandle, err := os.Open(importsDirectory)
	if err != nil {
		_ = importsRoot.Close()
		return nil, importArchiveFilesystemError("open imports directory handle", err)
	}
	handleInfo, err := importsHandle.Stat()
	if err != nil || !os.SameFile(info, handleInfo) {
		_ = importsHandle.Close()
		_ = importsRoot.Close()
		return nil, fmt.Errorf("%w: imports directory changed while opening handle", ErrInvalidImportArchivePath)
	}
	if err := projectRoot.Close(); err != nil {
		_ = importsHandle.Close()
		_ = importsRoot.Close()
		return nil, importArchiveFilesystemError("close project package", err)
	}
	closeProjectRoot = false
	return &importArchiveDirectory{root: importsRoot, handle: importsHandle}, nil
}

func importArchiveFilesystemError(operation string, err error) error {
	return fmt.Errorf("%w: %s: %w", ErrInvalidImportArchivePath, operation, err)
}

func readImportSource(ctx context.Context, sourcePath string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	before, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("%w: inspect source: %v", ErrUnsafeImportSource, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: source must be a regular file", ErrUnsafeImportSource)
	}
	if before.Size() == 0 {
		return nil, ErrEmptyImportSource
	}
	if before.Size() > int64(importing.MaxManuscriptBytes) {
		return nil, ErrImportSourceTooLarge
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("%w: open source: %v", ErrUnsafeImportSource, err)
	}
	defer source.Close()
	opened, err := source.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("%w: source changed before reading", ErrUnsafeImportSource)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(io.LimitReader(source, int64(importing.MaxManuscriptBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read source: %v", ErrUnsafeImportSource, err)
	}
	if len(raw) > importing.MaxManuscriptBytes {
		return nil, ErrImportSourceTooLarge
	}
	after, err := os.Lstat(sourcePath)
	if err != nil ||
		after.Mode()&os.ModeSymlink != 0 ||
		!after.Mode().IsRegular() ||
		!os.SameFile(before, after) ||
		before.Size() != opened.Size() ||
		opened.Size() != after.Size() ||
		after.Size() != int64(len(raw)) {
		return nil, fmt.Errorf("%w: source changed while reading", ErrUnsafeImportSource)
	}
	if len(raw) == 0 {
		return nil, ErrEmptyImportSource
	}
	return raw, nil
}

func decodeImportSource(raw []byte) (string, string, error) {
	if len(raw) == 0 {
		return "", "", ErrEmptyImportSource
	}
	if bytes.HasPrefix(raw, []byte{0xff, 0xfe, 0x00, 0x00}) ||
		bytes.HasPrefix(raw, []byte{0x00, 0x00, 0xfe, 0xff}) {
		return "", "", ErrUnsupportedImportEncoding
	}
	if bytes.HasPrefix(raw, []byte{0xef, 0xbb, 0xbf}) {
		body := raw[3:]
		if len(body) == 0 {
			return "", "", ErrEmptyImportSource
		}
		if !utf8.Valid(body) {
			return "", "", ErrUnsupportedImportEncoding
		}
		return string(body), importEncodingUTF8, nil
	}
	if bytes.HasPrefix(raw, []byte{0xff, 0xfe}) {
		return decodeUTF16(raw[2:], textunicode.LittleEndian, importEncodingUTF16LE)
	}
	if bytes.HasPrefix(raw, []byte{0xfe, 0xff}) {
		return decodeUTF16(raw[2:], textunicode.BigEndian, importEncodingUTF16BE)
	}
	if bytes.IndexByte(raw, 0) >= 0 {
		return "", "", ErrUnsupportedImportEncoding
	}
	if utf8.Valid(raw) {
		return string(raw), importEncodingUTF8, nil
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(raw)
	if err != nil || !utf8.Valid(decoded) {
		return "", "", ErrUnsupportedImportEncoding
	}
	if !validGB18030Bytes(raw) ||
		bytes.Contains(decoded, []byte(string(utf8.RuneError))) {
		return "", "", ErrUnsupportedImportEncoding
	}
	if len(decoded) == 0 {
		return "", "", ErrEmptyImportSource
	}
	return string(decoded), importEncodingGB18030, nil
}

func validGB18030Bytes(raw []byte) bool {
	for index := 0; index < len(raw); {
		value := raw[index]
		switch {
		case value <= 0x7f || value == 0x80:
			index++
		case value >= 0x81 && value <= 0xfe:
			if index+1 >= len(raw) {
				return false
			}
			second := raw[index+1]
			if second >= 0x40 && second <= 0xfe && second != 0x7f {
				index += 2
				continue
			}
			if second < 0x30 || second > 0x39 ||
				index+3 >= len(raw) ||
				raw[index+2] < 0x81 || raw[index+2] > 0xfe ||
				raw[index+3] < 0x30 || raw[index+3] > 0x39 {
				return false
			}
			index += 4
		default:
			return false
		}
	}
	return true
}

func decodeUTF16(
	raw []byte,
	endianness textunicode.Endianness,
	encodingName string,
) (string, string, error) {
	if len(raw) == 0 {
		return "", "", ErrEmptyImportSource
	}
	if len(raw)%2 != 0 {
		return "", "", ErrUnsupportedImportEncoding
	}
	decoded, err := textunicode.UTF16(endianness, textunicode.IgnoreBOM).NewDecoder().Bytes(raw)
	if err != nil || !utf8.Valid(decoded) {
		return "", "", ErrUnsupportedImportEncoding
	}
	reencoded, err := textunicode.UTF16(endianness, textunicode.IgnoreBOM).NewEncoder().Bytes(decoded)
	if err != nil || !bytes.Equal(reencoded, raw) {
		return "", "", ErrUnsupportedImportEncoding
	}
	if len(decoded) == 0 {
		return "", "", ErrEmptyImportSource
	}
	return string(decoded), encodingName, nil
}

func archiveImportSource(
	ctx context.Context,
	importsDirectory *importArchiveDirectory,
	archiveName string,
	raw []byte,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	temporary, temporaryName, err := createImportArchiveTemporary(importsDirectory)
	if err != nil {
		return importArchiveFilesystemError("create temporary import archive", err)
	}
	defer func() {
		_ = removeImportTemporaryNameIfSame(
			importsDirectory,
			temporary,
			temporaryName,
		)
		_ = temporary.Close()
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return importArchiveFilesystemError("set import archive permissions", err)
	}
	if _, err := io.Copy(temporary, bytes.NewReader(raw)); err != nil {
		return importArchiveFilesystemError("write import archive", err)
	}
	if err := temporary.Sync(); err != nil {
		return importArchiveFilesystemError("sync import archive", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := commitImportArchive(
		importsDirectory,
		temporary,
		temporaryName,
		archiveName,
	); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return ErrImportArchiveExists
		}
		return importArchiveFilesystemError("commit import archive", err)
	}
	return nil
}

func commitImportArchive(
	importsDirectory *importArchiveDirectory,
	temporary *os.File,
	temporaryName string,
	archiveName string,
) error {
	err := commitOpenImportArchiveNoReplace(
		importsDirectory.handle,
		temporary,
		temporaryName,
		archiveName,
	)
	if err == nil || errors.Is(err, fs.ErrExist) {
		return err
	}
	return copyOpenImportArchiveNoReplace(importsDirectory, temporary, archiveName)
}

func copyOpenImportArchiveNoReplace(
	importsDirectory *importArchiveDirectory,
	temporary *os.File,
	archiveName string,
) error {
	target, err := openImportArchiveTemporary(importsDirectory, archiveName)
	if err != nil {
		return err
	}
	removeTarget := true
	defer func() {
		if removeTarget {
			_ = removeImportTemporaryNameIfSame(importsDirectory, target, archiveName)
		}
		_ = target.Close()
	}()

	if _, err := temporary.Seek(0, io.SeekStart); err != nil {
		return err
	}
	if _, err := io.Copy(target, temporary); err != nil {
		return err
	}
	if err := target.Sync(); err != nil {
		return err
	}
	removeTarget = false
	return nil
}

func createImportArchiveTemporary(importsDirectory *importArchiveDirectory) (*os.File, string, error) {
	for range 100 {
		var random [16]byte
		if _, err := cryptorand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := ".import-source-" + hex.EncodeToString(random[:])
		file, err := openImportArchiveTemporary(importsDirectory, name)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return file, name, err
	}
	return nil, "", errors.New("could not allocate a unique import archive temporary file")
}

func removeImportTemporaryNameIfSame(
	importsDirectory *importArchiveDirectory,
	temporary *os.File,
	temporaryName string,
) error {
	expected, err := temporary.Stat()
	if err != nil {
		return fmt.Errorf("%w: inspect open import temporary: %v", ErrInvalidImportArchivePath, err)
	}
	actual, err := importsDirectory.root.Lstat(temporaryName)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: inspect import temporary name: %v", ErrInvalidImportArchivePath, err)
	}
	if !actual.Mode().IsRegular() || !os.SameFile(expected, actual) {
		return nil
	}
	if err := importsDirectory.root.Remove(temporaryName); err != nil &&
		!errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: remove import temporary: %v", ErrInvalidImportArchivePath, err)
	}
	return nil
}

func deleteImportArchive(importsDirectory *importArchiveDirectory, fileName string) error {
	if err := importsDirectory.root.Remove(fileName); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: delete import source archive: %v", ErrInvalidImportArchivePath, err)
	}
	return nil
}

func cleanupOrphanImportTemporaryFiles(importsDirectory *importArchiveDirectory) error {
	entries, err := fs.ReadDir(importsDirectory.root.FS(), ".")
	if err != nil {
		return fmt.Errorf("%w: list import archives: %v", ErrInvalidImportArchivePath, err)
	}
	orphanCutoff := time.Now().Add(-importTemporaryOrphanAge)
	for _, entry := range entries {
		if !isOwnedImportTemporaryName(entry.Name()) {
			continue
		}
		info, err := importsDirectory.root.Lstat(entry.Name())
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("%w: inspect orphan import archive: %v", ErrInvalidImportArchivePath, err)
		}
		if info.IsDir() {
			continue
		}
		if info.ModTime().After(orphanCutoff) {
			continue
		}
		if err := importsDirectory.root.Remove(entry.Name()); err != nil &&
			!errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: remove orphan import archive: %v", ErrInvalidImportArchivePath, err)
		}
	}
	return nil
}

func cleanupOrphanImportTemporaryFilesForLayout(layout Layout) error {
	directory, err := openImportsDirectory(ports.ProjectPackage{
		Root:    layout.Root,
		Imports: layout.Imports,
	})
	if err != nil {
		return err
	}
	defer directory.Close()
	return cleanupOrphanImportTemporaryFiles(directory)
}

func isOwnedImportTemporaryName(name string) bool {
	const prefix = ".import-source-"
	if len(name) != len(prefix)+32 || !strings.HasPrefix(name, prefix) {
		return false
	}
	for _, value := range name[len(prefix):] {
		if value >= '0' && value <= '9' || value >= 'a' && value <= 'f' {
			continue
		}
		return false
	}
	return true
}

func validateImportArchiveRelativePath(relativePath string) (string, error) {
	if relativePath == "" ||
		strings.ContainsRune(relativePath, '\\') ||
		path.IsAbs(relativePath) ||
		filepath.IsAbs(relativePath) ||
		filepath.VolumeName(relativePath) != "" ||
		path.Clean(relativePath) != relativePath {
		return "", ErrInvalidImportArchivePath
	}
	parts := strings.Split(relativePath, "/")
	if len(parts) != 2 || parts[0] != "imports" {
		return "", ErrInvalidImportArchivePath
	}
	extension := path.Ext(parts[1])
	if extension != ".txt" && extension != ".md" && extension != ".markdown" {
		return "", ErrInvalidImportArchivePath
	}
	jobID := strings.TrimSuffix(parts[1], extension)
	if err := validateImportJobID(jobID); err != nil {
		return "", ErrInvalidImportArchivePath
	}
	return parts[1], nil
}

var _ ports.ImportSourceArchive = ImportSourceArchive{}
