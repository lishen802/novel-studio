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
)

var (
	ErrUnsupportedImportFormat   = errors.New("unsupported import source format")
	ErrEmptyImportSource         = errors.New("import source is empty")
	ErrImportSourceTooLarge      = errors.New("import source is too large")
	ErrUnsupportedImportEncoding = errors.New("unsupported import source encoding")
	ErrUnsafeImportSource        = errors.New("unsafe import source")
	ErrImportArchiveExists       = errors.New("import source archive already exists")
	ErrInvalidImportArchivePath  = errors.New("invalid import archive path")
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
		return nil, fmt.Errorf("inspect project package: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return nil, fmt.Errorf("%w: project root is not a real directory", ErrInvalidImportArchivePath)
	}
	projectRoot, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open project package: %w", err)
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
			return nil, fmt.Errorf("create imports directory: %w", err)
		}
		info, err = projectRoot.Lstat("imports")
	}
	if err != nil {
		return nil, fmt.Errorf("inspect imports directory: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("%w: imports path is not a real directory", ErrInvalidImportArchivePath)
	}
	importsRoot, err := projectRoot.OpenRoot("imports")
	if err != nil {
		return nil, fmt.Errorf("open imports directory: %w", err)
	}
	openedImportsInfo, err := importsRoot.Stat(".")
	if err != nil || !os.SameFile(info, openedImportsInfo) {
		_ = importsRoot.Close()
		return nil, fmt.Errorf("%w: imports directory changed while opening", ErrInvalidImportArchivePath)
	}
	importsHandle, err := os.Open(importsDirectory)
	if err != nil {
		_ = importsRoot.Close()
		return nil, fmt.Errorf("open imports directory handle: %w", err)
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
		return nil, fmt.Errorf("close project package: %w", err)
	}
	closeProjectRoot = false
	return &importArchiveDirectory{root: importsRoot, handle: importsHandle}, nil
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
	reencoded, err := simplifiedchinese.GB18030.NewEncoder().Bytes(decoded)
	if err != nil || !bytes.Equal(reencoded, raw) {
		return "", "", ErrUnsupportedImportEncoding
	}
	if len(decoded) == 0 {
		return "", "", ErrEmptyImportSource
	}
	return string(decoded), importEncodingGB18030, nil
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
		return fmt.Errorf("create temporary import archive: %w", err)
	}
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = importsDirectory.root.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return fmt.Errorf("set import archive permissions: %w", err)
	}
	if _, err := io.Copy(temporary, bytes.NewReader(raw)); err != nil {
		return fmt.Errorf("write import archive: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync import archive: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close import archive: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := renameImportArchiveNoReplace(
		importsDirectory.handle,
		temporaryName,
		archiveName,
	); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return ErrImportArchiveExists
		}
		return fmt.Errorf("commit import archive: %w", err)
	}
	removeTemporary = false
	return nil
}

func createImportArchiveTemporary(importsDirectory *importArchiveDirectory) (*os.File, string, error) {
	for range 100 {
		var random [16]byte
		if _, err := cryptorand.Read(random[:]); err != nil {
			return nil, "", err
		}
		name := ".import-source-" + hex.EncodeToString(random[:])
		file, err := importsDirectory.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return file, name, err
	}
	return nil, "", errors.New("could not allocate a unique import archive temporary file")
}

func deleteImportArchive(importsDirectory *importArchiveDirectory, fileName string) error {
	if err := importsDirectory.root.Remove(fileName); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete import source archive: %w", err)
	}
	return nil
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
