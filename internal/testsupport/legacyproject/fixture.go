// Package legacyproject builds deterministic v9 .zjproject fixtures for
// migration and recovery tests. The fixture is generated in a temporary
// directory from the canonical Goose migrations and a fixed synthetic seed;
// no binary databases or real manuscripts are committed to the repository.
package legacyproject

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

const (
	packageName  = "legacy-v09.zjproject"
	databaseFile = "project.db"

	// fixtureSchemaVersion is the Goose version the fixture is built at.
	// The v9 -> v14 migration path is what this fixture exercises.
	fixtureSchemaVersion = 9
)

// Fixture describes a generated legacy v9 .zjproject package.
type Fixture struct {
	// PackagePath is the absolute path to the .zjproject directory.
	PackagePath string
	// DatabasePath is the absolute path to project.db inside the package.
	DatabasePath string
	// Expected is the authoritative set of business sentinels loaded from
	// expected.json. Tests compare live database rows against this value.
	Expected Expected
}

// Expected is the authoritative set of business sentinels that the v9 fixture
// must contain. It is loaded from fixtures/migrations/v09/expected.json at
// build time and compared against the live database rows in tests. All fields
// are comparable with == so tests can detect any drift with a single check.
type Expected struct {
	Project         ExpectedProject         `json:"project"`
	Chapter         ExpectedChapter         `json:"chapter"`
	ChapterRevision ExpectedChapterRevision `json:"chapter_revision"`
	Draft           ExpectedDraft           `json:"draft"`
	Character       ExpectedCharacter       `json:"character"`
	WorldRule       ExpectedWorldRule       `json:"world_rule"`
	CanonFact       ExpectedCanonFact       `json:"canon_fact"`
	ChangeSet       ExpectedChangeSet       `json:"change_set"`
	ChangeOperation ExpectedChangeOperation `json:"change_operation"`
}

type ExpectedProject struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Genre   string `json:"genre"`
	Summary string `json:"summary"`
}

type ExpectedChapter struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Position int    `json:"position"`
	Body     string `json:"body"`
	Revision int    `json:"revision"`
}

type ExpectedChapterRevision struct {
	ID        int64  `json:"id"`
	ChapterID string `json:"chapter_id"`
	Revision  int    `json:"revision"`
	Title     string `json:"title"`
	Body      string `json:"body"`
}

type ExpectedDraft struct {
	ChapterID    string `json:"chapter_id"`
	Body         string `json:"body"`
	BaseRevision int    `json:"base_revision"`
}

type ExpectedCharacter struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Summary string `json:"summary"`
}

type ExpectedWorldRule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ExpectedCanonFact struct {
	ID       string `json:"id"`
	FactText string `json:"fact_text"`
	Category string `json:"category"`
	Origin   string `json:"origin"`
	Status   string `json:"status"`
}

type ExpectedChangeSet struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type ExpectedChangeOperation struct {
	ID           string `json:"id"`
	ChangeSetID  string `json:"change_set_id"`
	Kind         string `json:"kind"`
	TargetID     string `json:"target_id"`
	BaseRevision int    `json:"base_revision"`
	BeforeValue  string `json:"before_value"`
	AfterValue   string `json:"after_value"`
}

// Build creates a deterministic v9 legacy .zjproject package in parentDir.
// The package contains only project.db; no imports/, recovery/ or other
// layout directories are pre-created, so the fixture genuinely exercises the
// legacy package compatibility path in projectfs.OpenLayout.
//
// Build fails the test immediately on any I/O, migration, or seed error so
// that downstream tests never receive a partial fixture.
func Build(t testing.TB, parentDir string) Fixture {
	t.Helper()

	packagePath := filepath.Join(parentDir, packageName)
	if err := os.Mkdir(packagePath, 0o755); err != nil {
		t.Fatalf("create legacy package: %v", err)
	}
	databasePath := filepath.Join(packagePath, databaseFile)

	migrationFS := sqlite.MigrationsFS()
	seedSQL := readFixtureFile(t, "seed.sql")
	expected := readExpected(t)

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() {
		if db != nil {
			_ = db.Close()
		}
	}()

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		t.Fatalf("create goose provider: %v", err)
	}
	if _, err := provider.UpTo(context.Background(), fixtureSchemaVersion); err != nil {
		t.Fatalf("migrate to v9: %v", err)
	}

	if _, err := db.ExecContext(context.Background(), string(seedSQL)); err != nil {
		t.Fatalf("execute seed.sql: %v", err)
	}

	// Close the connection to flush WAL pages, then checkpoint and remove
	// -wal/-shm so the package is self-contained and byte-stable for copying.
	if err := db.Close(); err != nil {
		t.Fatalf("close seeded database: %v", err)
	}
	db = nil

	checkpointDB, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("reopen sqlite for checkpoint: %v", err)
	}
	if _, err := checkpointDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatalf("wal_checkpoint: %v", err)
	}
	if err := checkpointDB.Close(); err != nil {
		t.Fatalf("close checkpointed database: %v", err)
	}

	// Remove any residual WAL/SHM files; after TRUNCATE checkpoint they are
	// empty, but removing them keeps the package self-contained across
	// platforms and avoids surprising the migration recovery code.
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(databasePath + suffix); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove %s: %v", suffix, err)
		}
	}

	return Fixture{
		PackagePath:  packagePath,
		DatabasePath: databasePath,
		Expected:     expected,
	}
}

// readFixtureFile reads a file from fixtures/migrations/v09/ relative to the
// repository root. The repository root is located via runtime.Caller so the
// lookup does not depend on the working directory. This is a test-support
// package and is only compiled into test binaries, so source-tree presence
// is guaranteed.
func readFixtureFile(t testing.TB, name string) []byte {
	t.Helper()
	path := filepath.Join(repoRoot(t), "fixtures", "migrations", "v09", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture file %s: %v", name, err)
	}
	return data
}

func readExpected(t testing.TB) Expected {
	t.Helper()
	data := readFixtureFile(t, "expected.json")
	var expected Expected
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatalf("parse expected.json: %v", err)
	}
	return expected
}

// repoRoot returns the repository root directory derived from the source
// file path recorded at compile time. fixture.go lives at
//   <root>/internal/testsupport/legacyproject/fixture.go
// so the root is three directories up from this file's directory.
func repoRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed to locate fixture.go")
	}
	dir := filepath.Dir(file)
	root := filepath.Join(dir, "..", "..", "..")
	cleaned, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return cleaned
}

