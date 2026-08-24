package creativeharness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/persistence/harnesssqlite"
	"github.com/lishen802/novel-studio/internal/quality"
)

func TestPreviewFreezesCostDigestAndCreateRequiresExactConfirmation(t *testing.T) {
	db, err := harnesssqlite.Open(context.Background(), filepath.Join(t.TempDir(), "harness.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ids := &testIDs{}
	service, err := New(ServiceConfig{Store: harnesssqlite.NewStore(db), Profiles: testProfiles{}, Credentials: testCredentials{}, Providers: testProviders{}, Clock: testClock{}, IDs: ids})
	if err != nil {
		t.Fatal(err)
	}
	input := PreviewInput{ProfileID: "profile", ProfileVersion: 1, VariantID: "minimal", Mode: quality.HarnessModeReduced, Seed: 9}
	preview, err := service.Preview(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if preview.ConfirmationDigest == "" || preview.CaseCount < 3 || preview.MaximumOutputTokens == 0 {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := service.Create(context.Background(), RunInput{PreviewInput: input, CostConfirmed: true, ConfirmationDigest: "stale"}); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("stale confirmation error = %v", err)
	}
	created, err := service.Create(context.Background(), RunInput{PreviewInput: input, CostConfirmed: true, ConfirmationDigest: preview.ConfirmationDigest})
	if err != nil || created.Run.Status != quality.HarnessRunQueued || created.CompletedCases != 0 {
		t.Fatalf("created = %+v, err = %v", created, err)
	}
}

func TestArtifactProjectionUsesDraftThenRevisionInsteadOfPlannerOutput(t *testing.T) {
	run := agentdomain.Run{Steps: []agentdomain.RunStep{{PrimaryArtifactID: "plan"}, {PrimaryArtifactID: "draft"}, {PrimaryArtifactID: "review"}, {PrimaryArtifactID: "revision"}}}
	artifacts := []agentdomain.Artifact{
		{ID: "plan", Kind: agentdomain.ArtifactPlan, Content: "规划"},
		{ID: "draft", Kind: agentdomain.ArtifactDraft, Content: "初稿"},
		{ID: "review", Kind: agentdomain.ArtifactReview, Content: "审阅"},
		{ID: "revision", Kind: agentdomain.ArtifactRevision, Content: "修订稿"},
	}
	if got := initialArtifactContent(run, artifacts, quality.HarnessTaskChapter); got != "初稿" {
		t.Fatalf("initial = %q", got)
	}
	if got := finalArtifactContent(run, artifacts, quality.HarnessTaskChapter); got != "修订稿" {
		t.Fatalf("final = %q", got)
	}
}

func TestQualityReportsExposeOnlyStructuredPublicEvidence(t *testing.T) {
	run := agentdomain.Run{Steps: []agentdomain.RunStep{{ID: "draft-step", Capability: agentdomain.CapabilityProseWriter}}}
	artifacts := []agentdomain.Artifact{{StepID: "draft-step", Kind: agentdomain.ArtifactQualityReport, Content: `{"status":"passed","findings":[{"location":"第2段"}]}`}, {StepID: "draft-step", Kind: agentdomain.ArtifactContext, Content: "私有上下文"}}
	candidate := quality.HarnessCandidate{Kind: quality.CandidateInitial}
	applyCandidateEvidence(&candidate, run, artifacts)
	if !json.Valid([]byte(candidate.QualityReportJSON)) || strings.Contains(candidate.QualityReportJSON, "私有上下文") {
		t.Fatalf("quality projection = %q", candidate.QualityReportJSON)
	}
}

func TestSummarizeRatingsRefreshesAcceptanceAndDimensions(t *testing.T) {
	summary := summarizeRatings([]quality.HarnessRatingSubmission{{CreatedAt: time.Now(), Ratings: []quality.HarnessCandidateRating{
		{Acceptance: quality.HarnessAcceptDirect, Scores: map[string]int{"人物动机": 5}},
		{Acceptance: quality.HarnessAcceptRewrite, Scores: map[string]int{"人物动机": 1}},
	}}})
	if summary.Submissions != 1 || summary.CandidateRatings != 2 || summary.Acceptable != 1 || summary.Rewrite != 1 || summary.DimensionAverage["人物动机"] != 3 {
		t.Fatalf("summary = %+v", summary)
	}
}

type testProfiles struct{}

func (testProfiles) ResolveCreativeProfile(context.Context, string, int64) (agentdomain.ResolvedCreativeProfile, error) {
	roles := []agentdomain.ResolvedRole{
		{Role: agentdomain.RolePlanner, ConnectionID: "provider", ProviderName: "Provider", ProviderKind: agentdomain.ProviderOpenAI, CredentialRef: "planner", Model: "model", MaxOutputTokens: 6000},
		{Role: agentdomain.RoleWriter, ConnectionID: "provider", ProviderName: "Provider", ProviderKind: agentdomain.ProviderOpenAI, CredentialRef: "writer", Model: "model", MaxOutputTokens: 6000},
		{Role: agentdomain.RoleContinuityReviewer, ConnectionID: "provider", ProviderName: "Provider", ProviderKind: agentdomain.ProviderOpenAI, CredentialRef: "review", Model: "model", MaxOutputTokens: 6000},
	}
	return agentdomain.ResolvedCreativeProfile{ProfileID: "profile", Version: 1, MaxRunTokens: 50_000, MaxContextChars: 100_000, Roles: roles}, nil
}

type testCredentials struct{}

func (testCredentials) Save(context.Context, string, string) error  { return nil }
func (testCredentials) Get(context.Context, string) (string, error) { return "secret", nil }
func (testCredentials) Delete(context.Context, string) error        { return nil }

type testProviders struct{}

func (testProviders) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return nil, errors.New("测试不得调用 Provider")
}

type testClock struct{}

func (testClock) Now() time.Time { return time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC) }

type testIDs struct {
	mu   sync.Mutex
	next int
}

func (ids *testIDs) NewID() string {
	ids.mu.Lock()
	defer ids.mu.Unlock()
	ids.next++
	return fmt.Sprintf("id-%d", ids.next)
}
