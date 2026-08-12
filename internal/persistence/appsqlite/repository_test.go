package appsqlite

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/domain/appsettings"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewStore(db)
}

func mustPreferences(t *testing.T, store *Store, prefs appsettings.Preferences) {
	t.Helper()
	if err := store.SavePreferences(context.Background(), prefs); err != nil {
		t.Fatal(err)
	}
}

func mustProvider(t *testing.T, store *Store, p appsettings.ProviderConnection) {
	t.Helper()
	if err := store.UpsertProvider(context.Background(), p); err != nil {
		t.Fatal(err)
	}
}

func mustPublish(t *testing.T, store *Store, v appsettings.CreativeProfileVersion) {
	t.Helper()
	if err := store.PublishProfileVersion(context.Background(), v); err != nil {
		t.Fatal(err)
	}
}

func mustRecent(t *testing.T, store *Store, rp appsettings.RecentProject) {
	t.Helper()
	if err := store.RecordRecentProject(context.Background(), rp); err != nil {
		t.Fatal(err)
	}
}

func testRoles(connID, writerModel, reviewerModel string) []appsettings.RoleBinding {
	return []appsettings.RoleBinding{
		{Role: agent.RolePlanner, ConnectionID: connID, Model: "planner-model", MaxOutputTokens: 1000, Temperature: 0.7, Enabled: true},
		{Role: agent.RoleWriter, ConnectionID: connID, Model: writerModel, MaxOutputTokens: 2000, Temperature: 0.8, Enabled: true},
		{Role: agent.RoleContinuityReviewer, ConnectionID: connID, Model: reviewerModel, MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
	}
}

func sampleProfile(profileID string, version int64, writerModel string) appsettings.CreativeProfileVersion {
	v, _ := appsettings.NewCreativeProfileVersion(
		profileID, version, "默认方案",
		testRoles("conn-1", writerModel, "reviewer-model"),
		30000, 120000, time.Now(),
	)
	return v
}

func sampleProvider(id string) appsettings.ProviderConnection {
	p, _ := appsettings.NewProviderConnection(
		id, "OpenAI", agent.ProviderOpenAI,
		"https://api.openai.com/v1", "app-provider:"+id,
		time.Now(),
	)
	return p
}

func TestPreferencesRoundTrip(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)
	prefs := appsettings.Preferences{Theme: appsettings.ThemeDark, DefaultCreativeProfileID: "profile-1", UpdatedAt: now}
	mustPreferences(t, store, prefs)

	got, err := store.Preferences(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Theme != appsettings.ThemeDark || got.DefaultCreativeProfileID != "profile-1" {
		t.Fatalf("round trip = %#v", got)
	}
}

func TestPublishProfileVersionPersistsAllRolesInOneTransaction(t *testing.T) {
	store := newTestStore(t)
	v := sampleProfile("profile-1", 1, "writer-v1")
	mustPublish(t, store, v)

	got, err := store.ProfileVersion(context.Background(), "profile-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Roles) != 3 {
		t.Fatalf("roles = %d, want 3", len(got.Roles))
	}
	roleMap := make(map[agent.AgentRole]appsettings.RoleBinding, 3)
	for _, r := range got.Roles {
		roleMap[r.Role] = r
	}
	if roleMap[agent.RoleWriter].Model != "writer-v1" {
		t.Fatalf("writer model = %q", roleMap[agent.RoleWriter].Model)
	}
}

func TestProfileVersionIsImmutableAfterNewVersionPublished(t *testing.T) {
	store := newTestStore(t)
	v1 := sampleProfile("profile-1", 1, "writer-v1")
	mustPublish(t, store, v1)

	v2 := sampleProfile("profile-1", 2, "writer-v2")
	mustPublish(t, store, v2)

	gotV1, err := store.ProfileVersion(context.Background(), "profile-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	v1Roles := roleMap(gotV1.Roles)
	if v1Roles[agent.RoleWriter].Model != "writer-v1" {
		t.Fatalf("v1 was mutated: %q", v1Roles[agent.RoleWriter].Model)
	}

	gotV2, err := store.ProfileVersion(context.Background(), "profile-1", 2)
	if err != nil {
		t.Fatal(err)
	}
	v2Roles := roleMap(gotV2.Roles)
	if v2Roles[agent.RoleWriter].Model != "writer-v2" {
		t.Fatalf("v2 writer model = %q, want writer-v2", v2Roles[agent.RoleWriter].Model)
	}
}

func roleMap(roles []appsettings.RoleBinding) map[agent.AgentRole]appsettings.RoleBinding {
	m := make(map[agent.AgentRole]appsettings.RoleBinding, len(roles))
	for _, r := range roles {
		m[r.Role] = r
	}
	return m
}

func TestRecentProjectSamePathReplacesExisting(t *testing.T) {
	store := newTestStore(t)
	mustRecent(t, store, appsettings.RecentProject{ProjectID: "p1", Title: "旧标题", Path: "/books/a.zjproject", LastOpenedAt: time.Now()})
	mustRecent(t, store, appsettings.RecentProject{ProjectID: "p2", Title: "新标题", Path: "/books/a.zjproject", LastOpenedAt: time.Now()})

	projects, err := store.RecentProjects(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("recent count = %d, want 1", len(projects))
	}
	if projects[0].ProjectID != "p2" || projects[0].Title != "新标题" {
		t.Fatalf("recent project = %#v", projects[0])
	}
}

func TestRecentProjectsOrderedByLastOpenedAtDescending(t *testing.T) {
	store := newTestStore(t)
	mustRecent(t, store, appsettings.RecentProject{ProjectID: "p1", Title: "A", Path: "/a.zjproject", LastOpenedAt: time.Now().Add(-2 * time.Hour)})
	mustRecent(t, store, appsettings.RecentProject{ProjectID: "p2", Title: "B", Path: "/b.zjproject", LastOpenedAt: time.Now()})

	projects, err := store.RecentProjects(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 2 || projects[0].ProjectID != "p2" {
		t.Fatalf("order = %#v", projects)
	}
}

func TestMarkProjectMissing(t *testing.T) {
	store := newTestStore(t)
	mustRecent(t, store, appsettings.RecentProject{ProjectID: "p1", Title: "A", Path: "/a.zjproject", LastOpenedAt: time.Now()})
	if err := store.MarkProjectMissing(context.Background(), "p1", true); err != nil {
		t.Fatal(err)
	}
	projects, err := store.RecentProjects(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if !projects[0].Missing {
		t.Fatal("project not marked missing")
	}
}

func TestConcurrentUpsertProviderDoesNotDuplicate(t *testing.T) {
	store := newTestStore(t)
	p := sampleProvider("p1")
	var wg sync.WaitGroup
	const goroutines = 10
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_ = store.UpsertProvider(context.Background(), p)
		}()
	}
	wg.Wait()
	providers, err := store.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 {
		t.Fatalf("providers = %d, want 1", len(providers))
	}
}

func TestStoreSurvivesCloseAndReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.db")
	db1, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	store1 := NewStore(db1)
	p := sampleProvider("p1")
	mustProvider(t, store1, p)
	v := sampleProfile("profile-1", 1, "writer-v1")
	mustPublish(t, store1, v)
	if err := store1.Close(); err != nil {
		t.Fatal(err)
	}

	db2, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db2.Close()
	store2 := NewStore(db2)
	providers, err := store2.Providers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) != 1 || providers[0].ID != "p1" {
		t.Fatalf("providers after reopen = %#v", providers)
	}
	got, err := store2.ProfileVersion(context.Background(), "profile-1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Roles) != 3 {
		t.Fatalf("roles after reopen = %d", len(got.Roles))
	}
}

func TestProfileVersionMissingReturnsError(t *testing.T) {
	store := newTestStore(t)
	_, err := store.ProfileVersion(context.Background(), "nonexistent", 1)
	if !errors.Is(err, ErrSettingsMissing) {
		t.Fatalf("err = %v, want ErrSettingsMissing", err)
	}
}

func TestLegacyImportIsIdempotent(t *testing.T) {
	store := newTestStore(t)
	now := time.Now()
	if err := store.RecordLegacyImport(context.Background(), "project-1", "digest-1", "profile-1", 1, now); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordLegacyImport(context.Background(), "project-1", "digest-1", "profile-1", 1, now); err != nil {
		t.Fatal(err)
	}
	pid, version, err := store.LegacyImport(context.Background(), "project-1", "digest-1")
	if err != nil {
		t.Fatal(err)
	}
	if pid != "profile-1" || version != 1 {
		t.Fatalf("legacy import = %s/%d", pid, version)
	}
}

func TestLegacyImportMissingReturnsError(t *testing.T) {
	store := newTestStore(t)
	_, _, err := store.LegacyImport(context.Background(), "nonexistent", "no-digest")
	if !errors.Is(err, ErrSettingsMissing) {
		t.Fatalf("err = %v, want ErrSettingsMissing", err)
	}
}
