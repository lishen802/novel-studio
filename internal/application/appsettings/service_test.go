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
	mu         sync.Mutex
	saved      map[string]string
	deleted    []string
	getError   error
	saveError  error
	saveCalls  int
	failSaveOn int
}

func newFakeCredentials() *fakeCredentials {
	return &fakeCredentials{saved: make(map[string]string)}
}

func (f *fakeCredentials) Save(_ context.Context, ref, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveCalls++
	if f.failSaveOn > 0 && f.saveCalls == f.failSaveOn {
		return f.saveError
	}
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
	deleteError   error
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
	if r.deleteError != nil {
		return r.deleteError
	}
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

func (r *fakeRepo) ListLatestProfiles(_ context.Context) ([]appdomain.CreativeProfileVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	latestByID := make(map[string]appdomain.CreativeProfileVersion)
	for _, v := range r.profiles {
		if existing, ok := latestByID[v.ProfileID]; !ok || v.Version > existing.Version {
			latestByID[v.ProfileID] = v
		}
	}
	result := make([]appdomain.CreativeProfileVersion, 0, len(latestByID))
	for _, v := range latestByID {
		result = append(result, v)
	}
	return result, nil
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

// seedDefaultConnection 预置一个带凭据的 conn-1 连接：profileInput 默认引用它，
// 发布校验现在要求连接存在且凭据已配置，否则会直接返回 ErrProviderNotFound。
func seedDefaultConnection(t *testing.T, svc *Service, repo *fakeRepo, creds *fakeCredentials) {
	t.Helper()
	seedConn, err := appdomain.NewProviderConnection("conn-1", "OpenAI", agent.ProviderOpenAI, "https://api.openai.com/v1", "app-provider:conn-1", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertProvider(context.Background(), seedConn); err != nil {
		t.Fatal(err)
	}
	creds.saved["app-provider:conn-1"] = "secret"
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
	svc, repo, creds := newServiceHarness(t)
	seedDefaultConnection(t, svc, repo, creds)
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
	svc, repo, creds := newServiceHarness(t)
	seedDefaultConnection(t, svc, repo, creds)
	_, err := svc.PublishCreativeProfile(context.Background(), sameWriterReviewerInput())
	if !errors.Is(err, agent.ErrReviewerNotIndependent) {
		t.Fatalf("error = %v, want ErrReviewerNotIndependent", err)
	}
}

func TestPreferencesDefaultChangeDoesNotMutatePublishedVersions(t *testing.T) {
	svc, repo, creds := newServiceHarness(t)
	seedDefaultConnection(t, svc, repo, creds)
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
	svc, repo, creds := newServiceHarness(t)
	seedDefaultConnection(t, svc, repo, creds)
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

func TestDefaultProfileSelectionReturnsEmptySelectionWhenNoDefault(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	selection, err := svc.DefaultProfileSelection(context.Background())
	if err != nil {
		t.Fatalf("DefaultProfileSelection() error = %v, want no error", err)
	}
	if selection != (ports.ProjectProfileSelection{}) {
		t.Fatalf("selection = %#v, want empty selection", selection)
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
var _ ports.ProviderProber = (*Service)(nil)
var _ ports.CreativeProfileResolver = (*Service)(nil)

type fakeProvider struct {
	result ports.ModelGenerateResult
	err    error
}

func (f *fakeProvider) Generate(_ context.Context, _ string, _ ports.ModelGenerateRequest, _ func(string)) (ports.ModelGenerateResult, error) {
	return f.result, f.err
}

type fakeProviderFactory struct {
	provider ports.ModelProvider
	err      error
}

func (f *fakeProviderFactory) New(_ agent.ProviderConnection) (ports.ModelProvider, error) {
	return f.provider, f.err
}

func TestProbeProviderReturnsUsageAndLatency(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	svc.ConfigureProviders(&fakeProviderFactory{provider: &fakeProvider{
		result: ports.ModelGenerateResult{ProviderRequestID: "req-1", InputTokens: 5, OutputTokens: 2},
	}})
	result, err := svc.ProbeProvider(context.Background(), provider.ID, "gpt-test")
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderRequestID != "req-1" || result.InputTokens != 5 || result.OutputTokens != 2 {
		t.Fatalf("unexpected probe result: %#v", result)
	}
	if result.Model != "gpt-test" || result.ProviderID != provider.ID {
		t.Fatalf("unexpected probe identity: %#v", result)
	}
}

func TestProbeProviderMissingCredentialReturnsErrCredentialMissing(t *testing.T) {
	svc, _, creds := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	svc.ConfigureProviders(&fakeProviderFactory{provider: &fakeProvider{}})
	creds.getError = errors.New("not found")
	_, err := svc.ProbeProvider(context.Background(), provider.ID, "gpt-test")
	if !errors.Is(err, ErrCredentialMissing) {
		t.Fatalf("error = %v, want ErrCredentialMissing", err)
	}
}

func TestProbeProviderBlankModelRejected(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	svc.ConfigureProviders(&fakeProviderFactory{provider: &fakeProvider{}})
	_, err := svc.ProbeProvider(context.Background(), provider.ID, "  ")
	if !errors.Is(err, ErrInvalidProfileInput) {
		t.Fatalf("error = %v, want ErrInvalidProfileInput", err)
	}
}

func TestProbeProviderPropagatesProviderError(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	svc.ConfigureProviders(&fakeProviderFactory{provider: &fakeProvider{err: errors.New("provider failed")}})
	if _, err := svc.ProbeProvider(context.Background(), provider.ID, "gpt-test"); err == nil {
		t.Fatal("ProbeProvider succeeded")
	}
}

func TestUpdateProviderRestoresOldCredentialWhenMetadataFails(t *testing.T) {
	svc, repo, creds := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	repo.upsertError = errors.New("write failed")
	_, err := svc.UpdateProvider(context.Background(), provider.ID, ProviderInput{
		Name: "OpenAI", Kind: agent.ProviderOpenAI,
		BaseURL: "https://api.openai.com/v1", APIKey: "new-secret",
	})
	if err == nil {
		t.Fatal("UpdateProvider succeeded")
	}
	if got := creds.saved[provider.CredentialRef]; got != "secret" {
		t.Fatalf("credential after rollback = %q, want old secret", got)
	}
}

func TestUpdateProviderCredentialRollbackFailureReturnsRollbackError(t *testing.T) {
	svc, repo, creds := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	repo.upsertError = errors.New("write failed")
	creds.saveError = errors.New("vault unavailable")
	creds.failSaveOn = 3 // 首次写入新凭据成功，回滚写入旧凭据失败。
	_, err := svc.UpdateProvider(context.Background(), provider.ID, ProviderInput{
		Name: "OpenAI", Kind: agent.ProviderOpenAI,
		BaseURL: "https://api.openai.com/v1", APIKey: "new-secret",
	})
	if !errors.Is(err, ErrCredentialRollback) {
		t.Fatalf("error = %v, want ErrCredentialRollback", err)
	}
}

func TestDeleteProviderRemovesCredential(t *testing.T) {
	svc, _, creds := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	if err := svc.DeleteProvider(context.Background(), provider.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := creds.saved[provider.CredentialRef]; ok {
		t.Fatal("credential still present after delete")
	}
}

func TestDeleteProviderRestoresCredentialWhenMetadataFails(t *testing.T) {
	svc, repo, creds := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	repo.deleteError = errors.New("delete failed")
	if err := svc.DeleteProvider(context.Background(), provider.ID); err == nil {
		t.Fatal("DeleteProvider succeeded")
	}
	if got := creds.saved[provider.CredentialRef]; got != "secret" {
		t.Fatalf("credential after rollback = %q, want old secret", got)
	}
}

func TestResolveCreativeProfileReturnsConnectionMetadata(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	v := mustPublish(t, svc, profileUsing(provider.ID))
	resolved, err := svc.ResolveCreativeProfile(context.Background(), v.ProfileID, v.Version)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ProfileID != v.ProfileID || resolved.Version != v.Version {
		t.Fatalf("resolved identity = %s@%d", resolved.ProfileID, resolved.Version)
	}
	if resolved.MaxRunTokens != v.MaxRunTokens || resolved.MaxContextChars != v.MaxContextChars {
		t.Fatalf("resolved budgets = %d/%d", resolved.MaxRunTokens, resolved.MaxContextChars)
	}
	if len(resolved.Roles) != 3 {
		t.Fatalf("resolved roles = %d", len(resolved.Roles))
	}
	for _, role := range resolved.Roles {
		if role.ConnectionID != provider.ID || role.CredentialRef != provider.CredentialRef {
			t.Fatalf("role %s metadata = %#v", role.Role, role)
		}
		if role.ProviderName != provider.Name || role.Model == "" {
			t.Fatalf("role %s model/name = %q/%q", role.Role, role.ProviderName, role.Model)
		}
	}
}

func TestResolveCreativeProfileMissingCredential(t *testing.T) {
	svc, _, creds := newServiceHarness(t)
	provider := mustSaveProvider(t, svc)
	v := mustPublish(t, svc, profileUsing(provider.ID))
	creds.getError = errors.New("not found")
	if _, err := svc.ResolveCreativeProfile(context.Background(), v.ProfileID, v.Version); !errors.Is(err, ErrCredentialMissing) {
		t.Fatalf("error = %v, want ErrCredentialMissing", err)
	}
}

func TestResolveCreativeProfileNotFound(t *testing.T) {
	svc, _, _ := newServiceHarness(t)
	if _, err := svc.ResolveCreativeProfile(context.Background(), "missing", 1); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("error = %v, want ErrProfileNotFound", err)
	}
}

func TestLatestProfilesReturnsLatestVersionPerProfile(t *testing.T) {
	svc, repo, creds := newServiceHarness(t)
	seedDefaultConnection(t, svc, repo, creds)
	mustPublish(t, svc, profileInput("方案 A", "writer-a1"))
	mustPublish(t, svc, profileInput("方案 A", "writer-a2"))
	second := ProfileInput{
		ProfileID: "profile-2", Name: "方案 B",
		MaxRunTokens: 30000, MaxContextChars: 120000,
		Roles: []RoleInput{
			{Role: agent.RolePlanner, ConnectionID: "conn-1", Model: "planner-model", MaxOutputTokens: 1000, Temperature: 0.7, Enabled: true},
			{Role: agent.RoleWriter, ConnectionID: "conn-1", Model: "writer-b1", MaxOutputTokens: 2000, Temperature: 0.8, Enabled: true},
			{Role: agent.RoleContinuityReviewer, ConnectionID: "conn-1", Model: "reviewer-model", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
		},
	}
	mustPublish(t, svc, second)

	profiles, err := svc.LatestProfiles(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 2 {
		t.Fatalf("latest profiles = %d, want 2", len(profiles))
	}
	byID := map[string]appdomain.CreativeProfileVersion{}
	for _, p := range profiles {
		byID[p.ProfileID] = p
	}
	if byID["profile-1"].Version != 2 || byID["profile-2"].Version != 1 {
		t.Fatalf("latest versions = %d/%d", byID["profile-1"].Version, byID["profile-2"].Version)
	}
}
