package workspace

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	appsettingsapp "github.com/lishen802/novel-studio/internal/application/appsettings"
	"github.com/lishen802/novel-studio/internal/application/license"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/persistence/appsqlite"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
)

// TestCreateAgentRunWithRealSettingsPipeline 验证从真实 SQLite 设置库发布方案、
// 经 AppSettings Resolver 解析、到 workspace.CreateChapterAgentRun 创建运行的
// 端到端链路。回归点：持久化层按 role ASC 排序得到
// continuity_reviewer → planner → writer，但 AgentRun 严格按位置校验
// planner → writer → continuity_reviewer。若任一环节未规范化顺序，会返回
// invalid agent run，运行根本到不了模型调用。
func TestCreateAgentRunWithRealSettingsPipeline(t *testing.T) {
	ctx := context.Background()

	// 真实应用设置 SQLite 库 + 真实 AppSettings.Service。
	settingsDB, err := appsqlite.Open(ctx, filepath.Join(t.TempDir(), "settings.db"))
	if err != nil {
		t.Fatalf("open settings db: %v", err)
	}
	defer settingsDB.Close()
	settingsStore := appsqlite.NewStore(settingsDB)

	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	clock := fixedClock{now: time.Unix(200, 0)}
	settingsSvc := appsettingsapp.NewService(settingsStore, credentials, clock, ids.UUIDGenerator{})

	// 保存两个 Provider（writer 与 reviewer 必须使用不同连接或模型，否则领域校验拒绝）。
	plannerConn, err := settingsSvc.SaveProvider(ctx, appsettingsapp.ProviderInput{
		Name: "OpenAI 主连接", Kind: agentdomain.ProviderOpenAI,
		BaseURL: "https://api.openai.com/v1", APIKey: "key-a",
	})
	if err != nil {
		t.Fatalf("save planner provider: %v", err)
	}
	reviewerConn, err := settingsSvc.SaveProvider(ctx, appsettingsapp.ProviderInput{
		Name: "Anthropic 审阅连接", Kind: agentdomain.ProviderAnthropic,
		BaseURL: "https://api.anthropic.com", APIKey: "key-b",
	})
	if err != nil {
		t.Fatalf("save reviewer provider: %v", err)
	}

	// 故意按字母逆序传入角色，确保 CreateAgentRun 不依赖调用方顺序。
	profile, err := settingsSvc.PublishCreativeProfile(ctx, appsettingsapp.ProfileInput{
		ProfileID: "profile-integration", Name: "集成方案",
		MaxRunTokens: 30000, MaxContextChars: 120000,
		Roles: []appsettingsapp.RoleInput{
			{Role: agentdomain.RoleContinuityReviewer, ConnectionID: reviewerConn.ID, Model: "reviewer-model", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
			{Role: agentdomain.RoleWriter, ConnectionID: plannerConn.ID, Model: "writer-model", MaxOutputTokens: 3000, Temperature: 0.7, Enabled: true},
			{Role: agentdomain.RolePlanner, ConnectionID: plannerConn.ID, Model: "planner-model", MaxOutputTokens: 1000, Temperature: 0.7, Enabled: true},
		},
	})
	if err != nil {
		t.Fatalf("publish profile: %v", err)
	}
	if err := settingsSvc.SetDefaultProfile(ctx, profile.ProfileID); err != nil {
		t.Fatalf("set default profile: %v", err)
	}

	// 真实模型 Provider 工厂：每个角色返回固定输出。
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"planner-model":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
			"writer-model":   "夜色压在山门之上，墨老沿着石阶无声而行。",
			"reviewer-model": `{"approved":true,"summary":"未发现连续性冲突","issues":[]}`,
		},
	}

	// workspace 服务：注入真实 AppSettings.Service 作为 resolver 与 defaults。
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		clock,
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	service.ConfigureCreativeProfileResolver(settingsSvc)
	service.ConfigureProjectDefaults(settingsSvc)
	defer service.Close()

	if _, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "墨老企图夺舍主角"); err != nil {
		t.Fatalf("create project: %v", err)
	}
	chapter, err := service.CreateChapter(ctx, "第一章")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}

	// 关键回归断言：真实 DB 路径下不应返回 invalid agent run。
	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章，保持克制的第三人称。")
	if err != nil {
		t.Fatalf("CreateChapterAgentRun returned error (regression: role order): %v", err)
	}
	if run.Status != "queued" {
		t.Fatalf("unexpected initial run status: %q", run.Status)
	}
	if len(run.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(run.Steps))
	}
	expectedOrder := []string{
		string(agentdomain.RolePlanner),
		string(agentdomain.RoleWriter),
		string(agentdomain.RoleContinuityReviewer),
	}
	for i, want := range expectedOrder {
		if run.Steps[i].Role != want {
			t.Fatalf("step %d role = %q, want %q (regression: role ordering)", i, run.Steps[i].Role, want)
		}
	}

	// 执行运行并等待终态，确认整条链路真的能完成。
	run, err = service.ExecuteAgentRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ExecuteAgentRun: %v", err)
	}
	waitForAgentRunStatus(t, service, run.ID, "completed", 5*time.Second)
	run, err = service.AgentRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("AgentRun: %v", err)
	}
	if run.Status != "completed" {
		t.Fatalf("run did not complete, status = %q, error = %q", run.Status, run.ErrorMessage)
	}
	if len(providerFactory.requests) != 3 {
		t.Fatalf("expected 3 provider calls, got %d", len(providerFactory.requests))
	}
}
