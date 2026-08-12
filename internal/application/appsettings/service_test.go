package appsettings

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	"github.com/lishen802/novel-studio/internal/domain/agent"
	appdomain "github.com/lishen802/novel-studio/internal/domain/appsettings"
)

type fakeClock struct{ t time.Time }

func (f *fakeClock) Now() time.Time { return f.t }

type fakeIDGenerator struct{ counter int }

func (f *fakeIDGenerator) NewID() string {
	f.counter++
	return fmt.Sprintf("id-%d", f.counter)
}

type fakeCredentials struct {
	mu       sync.Mutex
	saved    map[string]string
	deleted  []string
	getError error
}

func newFakeCredentials() *fakeCredentials {
	return &fakeCredentials{saved: make(map[string]string)}
}

func (f *fakeCredentials) Save(_ context.Context, ref, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saved[ref] = secret
	return nil
}

func (f *fakeCredentials) Get(_ context.Context, ref string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getError != nil {
		return "", f.getError
	}
	secret, ok := f.saved[ref]
	if !ok {
		return "", errors.New("not found")
	}
	return secret, nil
}

func (f *fakeCredentials) Delete(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.saved, ref)
	f.deleted = append(f.deleted, ref)
	return nil
}

type fakeRepo struct {
	mu            sync.Mutex
	prefs         appdomain.Preferences
	providers     []appdomain.ProviderConnection
	profiles      []appdomain.CreativeProfileVersion
	recent        []appdomain.RecentProject
	legacyImports map[string]string
	upsertError   error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		prefs:         appdomain.Preferences{Theme: appdomain.ThemeSystem, UpdatedAt: time.Now()},
		legacyImports: make(map[string]string),
	}
}

func (r *fakeRepo) Preferences(_ context.Context) (appdomain.Preferences, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.prefs, nil
}

func (r *fakeRepo) SavePreferences(_ context.Context, prefs appdomain.Preferences) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefs = prefs
	return nil
}

func (r *fakeRepo) UpsertProvider(_ context.Context, p appdomain.ProviderConnection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.upsertError != nil {
		return r.upsertError
	}
	for i, existing := range r.providers {
		if existing.ID == p.ID {
			r.providers[i] = p
			return nil
		}
	}
	r.providers = append(r.providers, p)
	return nil
}

func (r *fakeRepo) Provider(_ context.Context, id string) (appdomain.ProviderConnection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return appdomain.ProviderConnection{}, errTestMissing
}

func (r *fakeRepo) Providers(_ context.Context) ([]appdomain.ProviderConnection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]appdomain.ProviderConnection(nil), r.providers...), nil
}

func (r *fakeRepo) DeleteProvider(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, p := range r.providers {
		if p.ID == id {
			r.providers = append(r.providers[:i], r.providers[i+1:]...)
			return nil
		}
	}
	return nil
}

func (r *fakeRepo) ProviderInUse(_ context.Context, providerID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.profiles {
		for _, role := range v.Roles {
			if role.ConnectionID == providerID {
				return true, nil
			}
		}
	}
	return false, nil
}

func (r *fakeRepo) PublishProfileVersion(_ context.Context, v appdomain.CreativeProfileVersion) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, existing := range r.profiles {
		if existing.ProfileID == v.ProfileID && existing.Version == v.Version {
			r.profiles[i] = v
			return nil
		}
	}
	r.profiles = append(r.profiles, v)
	return nil
}

func (r *fakeRepo) ProfileVersion(_ context.Context, profileID string, version int64) (appdomain.CreativeProfileVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range r.profiles {
		if v.ProfileID == profileID && v.Version == version {
			return v, nil
		}
	}
	return appdomain.CreativeProfileVersion{}, errTestMissing
}

func (r *fakeRepo) LatestProfileVersion(_ context.Context, profileID string) (appdomain.CreativeProfileVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var latest appdomain.CreativeProfileVersion
	found := false
	for _, v := range r.profiles {
		if v.ProfileID == profileID && (!found || v.Version > latest.Version) {
			latest = v
			found = true
		}
	}
	if !found {
		return appdomain.CreativeProfileVersion{}, errTestMissing
	}
	return latest, nil
}

func (r *fakeRepo) RecordRecentProject(_ context.Context, rp appdomain.RecentProject) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, existing := range r.recent {
		if existing.Path == rp.Path {
			r.recent[i] = rp
			return nil
		}
	}
	r.recent = append(r.recent, rp)
	return nil
}

func (r *fakeRepo) RecentProjects(_ context.Context, limit int) ([]appdomain.RecentProject, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit > len(r.recent) {
		limit = len(r.recent)
	}
	return append([]appdomain.RecentProject(nil), r.recent[:limit]...), nil
}

func (r *fakeRepo) MarkProjectMissing(_ context.Context, projectID string, missing bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, rp := range r.recent {
		if rp.ProjectID == projectID {
			r.recent[i].Missing = missing
			return nil
		}
	}
	return nil
}

func (r *fakeRepo) RecordLegacyImport(_ context.Context, projectID, sourceDigest, profileID string, profileVersion int64, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.legacyImports[projectID+"|"+sourceDigest] = fmt.Sprintf("%s\x00%d", profileID, profileVersion)
	return nil
}

func (r *fakeRepo) LegacyImport(_ context.Context, projectID, sourceDigest string) (string, int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	val, ok := r.legacyImports[projectID+"|"+sourceDigest]
	if !ok {
		return "", 0, errTestMissing
	}
	parts := strings.SplitN(val, "\x00", 2)
	if len(parts) != 2 {
		return "", 0, errTestMissing
	}
	var ver int64
	fmt.Sscanf(parts[1], "%d", &ver)
	return parts[0], ver, nil
}

func newServiceHarness(t *testing.T) (*Service, *fakeRepo, *fakeCredentials) {
	t.Helper()
	repo := newFakeRepo()
	creds := newFakeCredentials()
	clock := &fakeClock{t: time.Now()}
	ids := &fakeIDGenerator{}
	svc := NewService(repo, creds, clock, ids)
	return svc, repo, creds
}

func mustPublish(t *testing.T, svc *Service, input ProfileInput) appdomain.CreativeProfileVersion {
	t.Helper()
	v, err := svc.PublishCreativeProfile(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func mustSaveProvider(t *testing.T, svc *Service) appdomain.ProviderConnection {
	t.Helper()
	p, err := svc.SaveProvider(context.Background(), ProviderInput{
		Name: "OpenAI", Kind: agent.ProviderOpenAI,
		BaseURL: "https://api.openai.com/v1", APIKey: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func profileInput(name, writerModel string) ProfileInput {
	return ProfileInput{
		ProfileID: "profile-1", Name: name,
		MaxRunTokens: 30000, MaxContextChars: 120000,
		Roles: []RoleInput{
			{Role: agent.RolePlanner, ConnectionID: "conn-1", Model: "planner-model", MaxOutputTokens: 1000, Temperature: 0.7, Enabled: true},
			{Role: agent.RoleWriter, ConnectionID: "conn-1", Model: writerModel, MaxOutputTokens: 2000, Temperature: 0.8, Enabled: true},
			{Role: agent.RoleContinuityReviewer, ConnectionID: "conn-1", Model: "reviewer-model", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
		},
	}
}

func profileUsing(providerID string) ProfileInput {
	input := profileInput("默认方案", "writer-model")
	for i := range input.Roles {
		input.Roles[i].ConnectionID = providerID
	}
	return input
}

func sameWriterReviewerInput() ProfileInput {
	return ProfileInput{
		ProfileID: "profile-1", Name: "同模型",
		MaxRunTokens: 30000, MaxContextChars: 120000,
		Roles: []RoleInput{
			{Role: agent.RolePlanner, ConnectionID: "conn-1", Model: "planner-model", MaxOutputTokens: 1000, Temperature: 0.7, Enabled: true},
			{Role: agent.RoleWriter, ConnectionID: "conn-1", Model: "same-model", MaxOutputTokens: 2000, Temperature: 0.8, Enabled: true},
			{Role: agent.RoleContinuityReviewer, ConnectionID: "conn-1", Model: "same-model", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
		},
	}
}

func TestPublishDefaultProfileCreatesNewVersionWithoutChangingOldVersion(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	v1 := mustPublish(t, svc, profileInput("默认方案", "writer-model-v1"))
	v2 := mustPublish(t, svc, profileInput("默认方案", "writer-model-v2"))
	if v1.Version != 1 || v2.Version != 2 {
		t.Fatalf("versions = %d, %d", v1.Version, v2.Version)
	}
	got, err := svc.ProfileVersion(context.Background(), v1.ProfileID, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got.Roles {
		if r.Role == agent.RoleWriter && r.Model != "writer-model-v1" {
			t.Fatalf("v1 was mutated: %q", r.Model)
		}
	}
}

func TestSaveProviderStoresOnlyCredentialReferenceInDatabase(t *testing.T) {
	svc, repo, creds := newServiceHarness(t)
	provider, err := svc.SaveProvider(context.Background(), ProviderInput{
		Name: "OpenAI", Kind: agent.ProviderOpenAI,
		BaseURL: "https://api.openai.com/v1", APIKey: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	providers, _ := repo.Providers(context.Background())
	if len(providers) != 1 {
		t.Fatalf("providers = %d", len(providers))
	}
	for _, p := range providers {
		if strings.Contains(fmt.Sprint(p), "secret") {
			t.Fatal("secret reached repository")
		}
	}
	if creds.saved[provider.CredentialRef] != "secret" {
		t.Fatal("credential not saved to vault")
	}
}

func TestSaveProviderRollsBackNewCredentialWhenMetadataWriteFails(t *testing.T) {
	svc, repo, creds := newServiceHarness(t)
	repo.upsertError = errors.New("write failed")
	_, err := svc.SaveProvider(context.Background(), ProviderInput{
		Name: "OpenAI", Kind: agent.ProviderOpenAI,
		BaseURL: "https://api.openai.com/v1", APIKey: "secret",
	})
	if err == nil {
		t.Fatal("SaveProvider succeeded")
	}
	if len(creds.deleted) != 1 {
		t.Fatalf("credential rollback = %v", creds.deleted)
	}
	if len(creds.saved) != 0 {
		t.Fatalf("saved credentials = %v", creds.saved)
	}
}

func TestDeleteProviderRejectsProfileReferences(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	mustPublish(t, svc, profileUsing(provider.ID))
	if err := svc.DeleteProvider(context.Background(), provider.ID); !errors.Is(err, ErrProviderInUse) {
		t.Fatalf("DeleteProvider() error = %v", err)
	}
}

func TestPublishProfileRejectsSameWriterAndReviewerModel(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	_, err := svc.PublishCreativeProfile(context.Background(), sameWriterReviewerInput())
	if !errors.Is(err, agent.ErrReviewerNotIndependent) {
		t.Fatalf("error = %v, want ErrReviewerNotIndependent", err)
	}
}

func TestPreferencesDefaultChangeDoesNotMutatePublishedVersions(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	v1 := mustPublish(t, svc, profileInput("方案 A", "writer-a"))
	v2 := mustPublish(t, svc, profileInput("方案 B", "writer-b"))
	if err := svc.SetDefaultProfile(context.Background(), v2.ProfileID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ProfileVersion(context.Background(), v1.ProfileID, v1.Version)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got.Roles {
		if r.Role == agent.RoleWriter && r.Model != "writer-a" {
			t.Fatalf("old version model = %q", r.Model)
		}
	}
}

func TestDefaultProfileSelectionReturnsLatestVersion(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	v1 := mustPublish(t, svc, profileInput("默认方案", "writer-v1"))
	mustPublish(t, svc, profileInput("默认方案", "writer-v2"))
	if err := svc.SetDefaultProfile(context.Background(), v1.ProfileID); err != nil {
		t.Fatal(err)
	}
	selection, err := svc.DefaultProfileSelection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if selection.Version != 2 {
		t.Fatalf("selection version = %d, want 2", selection.Version)
	}
}

func TestDefaultProfileSelectionReturnsErrorWhenNoDefault(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	_, err := svc.DefaultProfileSelection(context.Background())
	if !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("error = %v, want ErrProfileNotFound", err)
	}
}

func TestDeleteProviderSucceedsWhenNotInUse(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	if err := svc.DeleteProvider(context.Background(), provider.ID); err != nil {
		t.Fatalf("DeleteProvider() error = %v", err)
	}
}

func TestDeleteProviderNotFound(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	if err := svc.DeleteProvider(context.Background(), "nonexistent"); !errors.Is(err, ErrProviderNotFound) {
		t.Fatalf("error = %v, want ErrProviderNotFound", err)
	}
}

func TestHasCredentialReturnsTrueWhenVaultHasSecret(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	if !svc.HasCredential(context.Background(), provider) {
		t.Fatal("HasCredential = false, want true")
	}
}

func TestHasCredentialReturnsFalseWhenVaultMissingSecret(t *testing.T) {
	svc, _, creds := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	creds.getError = errors.New("not found")
	if svc.HasCredential(context.Background(), provider) {
		t.Fatal("HasCredential = true, want false")
	}
}

// Compile-time check that Service implements ports.ProjectDefaults.
var errTestMissing = errors.New("record not found")

var _ ports.ProjectDefaults = (*Service)(nil)
