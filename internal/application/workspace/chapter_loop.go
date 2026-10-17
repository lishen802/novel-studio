package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	chapterapp "github.com/lishen802/novel-studio/internal/application/chapterloop"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	loopdomain "github.com/lishen802/novel-studio/internal/domain/chapterloop"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
)

// chapterLoopCompleter 是单章产品循环使用的模型完成器。
type chapterLoopCompleter = chapterapp.Completer

// ConfigureChapterCompleter 注入测试或本地完成器；为空时改走项目已固定的创作方案。
func (s *Service) ConfigureChapterCompleter(completer chapterLoopCompleter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chapterCompleter = completer
}

type StartChapterProductionInput struct {
	RoadmapNodeID string `json:"roadmapNodeId"`
	ChapterID     string `json:"chapterId"`
}

type SelectionRewriteInput struct {
	ChapterID string `json:"chapterId"`
	Revision  int64  `json:"revision"`
	Body      string `json:"body"`
	StartRune int    `json:"startRune"`
	EndRune   int    `json:"endRune"`
	Mode      string `json:"mode"`
}

type AcceptSelectionInput struct {
	ProposalID  string `json:"proposalId"`
	ChapterID   string `json:"chapterId"`
	Revision    int64  `json:"revision"`
	Body        string `json:"body"`
	Partial     bool   `json:"partial"`
	Replacement string `json:"replacement"`
}

// StartChapterProduction 从已写入项目的路线图节点生成章节候选。
func (s *Service) StartChapterProduction(ctx context.Context, input StartChapterProductionInput) (chapterapp.Production, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return chapterapp.Production{}, err
	}
	s.mu.Lock()
	if err := s.requireEditableLocked(); err != nil {
		s.mu.Unlock()
		return chapterapp.Production{}, err
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		s.mu.Unlock()
		return chapterapp.Production{}, err
	}
	node, err := confirmedOutlineNode(ctx, s.active.store, input.RoadmapNodeID)
	if err != nil {
		s.mu.Unlock()
		return chapterapp.Production{}, err
	}
	if _, err := s.active.store.Chapter(ctx, input.ChapterID); err != nil {
		s.mu.Unlock()
		return chapterapp.Production{}, err
	}
	s.mu.Unlock()
	loop, _, budget, err := s.loopService(ctx)
	if err != nil {
		return chapterapp.Production{}, err
	}
	constraint := "保持路线图目标"
	if strings.TrimSpace(node.Goal) != "" {
		constraint = "保持路线图目标：" + strings.TrimSpace(node.Goal)
	}
	return loop.StartProduction(ctx, chapterapp.ProductionRequest{
		ProjectID: project.ID, RoadmapNodeID: node.ID, ChapterID: input.ChapterID, Confirmed: true,
		Manifest: map[string]string{
			"title": node.Title, "summary": node.Summary, "goal": node.Goal, "conflict": node.Conflict, "outcome": node.Outcome,
		},
		CharBudget: budget, Constraints: []string{constraint},
	})
}

// FinalizeChapterProduction 把已选候选写成新的 canonical revision。
func (s *Service) FinalizeChapterProduction(ctx context.Context, chapterID string) (ChapterView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return ChapterView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ChapterView{}, err
	}
	backend, projectID, err := s.loopBackendLocked(ctx)
	if err != nil {
		return ChapterView{}, err
	}
	record, err := backend.LatestChapterLoop(ctx, projectID, "production", chapterID)
	if err != nil {
		return ChapterView{}, err
	}
	var production chapterapp.Production
	if err := json.Unmarshal([]byte(record.Payload), &production); err != nil {
		return ChapterView{}, err
	}
	var body string
	for _, candidate := range production.Candidates {
		if candidate.ID == production.SelectedID {
			body = candidate.Body
		}
	}
	if strings.TrimSpace(body) == "" || production.SelectedID == "" {
		return ChapterView{}, loopdomain.ErrCandidateNotSelectable
	}
	previous, err := s.active.store.Chapter(ctx, chapterID)
	if err != nil {
		return ChapterView{}, err
	}
	updated, err := previous.Commit(body, previous.Revision, s.clock.Now())
	if err != nil {
		return ChapterView{}, err
	}
	if err := s.active.store.CommitChapter(ctx, updated, previous); err != nil {
		return ChapterView{}, fmt.Errorf("finalize chapter: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	return chapterView(updated), nil
}

// PrepareSelectionRewrite 生成绑定当前正文范围的改写候选。
func (s *Service) PrepareSelectionRewrite(ctx context.Context, input SelectionRewriteInput) (loopdomain.SelectionProposal, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return loopdomain.SelectionProposal{}, err
	}
	loop, projectID, err := s.editableLoop(ctx)
	if err != nil {
		return loopdomain.SelectionProposal{}, err
	}
	return loop.PrepareSelection(ctx, chapterapp.SelectionRequest{
		ProjectID: projectID, ChapterID: input.ChapterID, Revision: input.Revision, Body: input.Body,
		StartRune: input.StartRune, EndRune: input.EndRune, Mode: input.Mode,
	})
}

// ExplainSelection 解释选区，不发起整章生成，也不改正文。
func (s *Service) ExplainSelection(ctx context.Context, input SelectionRewriteInput) (string, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return "", err
	}
	loop, _, err := s.editableLoop(ctx)
	if err != nil {
		return "", err
	}
	return loop.ExplainSelection(ctx, chapterapp.SelectionRequest{
		ProjectID: "", ChapterID: input.ChapterID, Revision: input.Revision, Body: input.Body,
		StartRune: input.StartRune, EndRune: input.EndRune, Mode: "explain",
	})
}

// AcceptSelectionRewrite 在选区仍对应当前正文时创建新 revision。
func (s *Service) AcceptSelectionRewrite(ctx context.Context, input AcceptSelectionInput) (ChapterView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return ChapterView{}, err
	}
	loop, projectID, err := s.editableLoop(ctx)
	if err != nil {
		return ChapterView{}, err
	}
	body, _, err := loop.AcceptSelection(ctx, chapterapp.AcceptSelectionRequest{
		ProjectID: projectID, ProposalID: input.ProposalID, Body: input.Body, Revision: input.Revision,
		Partial: input.Partial, Replacement: input.Replacement,
	})
	if err != nil {
		return ChapterView{}, err
	}
	return s.CommitChapter(ctx, input.ChapterID, body, input.Revision)
}

// CommitChapterPosthoc 从已定稿正文提取最小记忆；失败不回滚正文。
func (s *Service) CommitChapterPosthoc(ctx context.Context, chapterID, idempotencyKey string) (loopdomain.Posthoc, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return loopdomain.Posthoc{}, err
	}
	s.mu.Lock()
	if err := s.requireEditableLocked(); err != nil {
		s.mu.Unlock()
		return loopdomain.Posthoc{}, err
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		s.mu.Unlock()
		return loopdomain.Posthoc{}, err
	}
	chapter, err := s.active.store.Chapter(ctx, chapterID)
	if err != nil {
		s.mu.Unlock()
		return loopdomain.Posthoc{}, err
	}
	s.mu.Unlock()
	loop, _, _, err := s.loopService(ctx)
	if err != nil {
		return loopdomain.Posthoc{}, err
	}
	return loop.CommitPosthoc(ctx, chapterapp.PosthocRequest{
		ProjectID: project.ID, ChapterID: chapter.ID, Revision: chapter.Revision, Body: chapter.Body, IdempotencyKey: idempotencyKey,
	})
}

// RecoverChapterPosthoc 在定稿来源未变时清除待重建标记。
func (s *Service) RecoverChapterPosthoc(ctx context.Context, chapterID string) (loopdomain.Posthoc, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return loopdomain.Posthoc{}, err
	}
	s.mu.Lock()
	if err := s.requireEditableLocked(); err != nil {
		s.mu.Unlock()
		return loopdomain.Posthoc{}, err
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		s.mu.Unlock()
		return loopdomain.Posthoc{}, err
	}
	chapter, err := s.active.store.Chapter(ctx, chapterID)
	if err != nil {
		s.mu.Unlock()
		return loopdomain.Posthoc{}, err
	}
	s.mu.Unlock()
	loop, _, _, err := s.loopService(ctx)
	if err != nil {
		return loopdomain.Posthoc{}, err
	}
	return loop.RecoverPosthoc(ctx, project.ID, chapter.ID, chapter.Body, chapter.Revision)
}

// GenerateOpeningSample 按已保存交接生成非正式试写。
func (s *Service) GenerateOpeningSample(ctx context.Context) (loopdomain.OpeningSample, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return loopdomain.OpeningSample{}, err
	}
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	view, err := runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	if view.SampleHandoff == nil || view.SampleHandoff.Status != "prepared" {
		return loopdomain.OpeningSample{}, loopdomain.ErrSampleNotPromotable
	}
	loop, _, _, err := s.loopService(ctx)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	handoff := view.SampleHandoff
	return loop.GenerateOpeningSample(ctx, chapterapp.OpeningSampleRequest{
		ProjectID: runtime.projectID, HandoffID: handoff.ID, DirectionCandidateID: handoff.DirectionCandidateID,
		DirectionDigest: handoff.DirectionContentSHA256, CurrentDigest: handoff.DirectionContentSHA256,
		PointOfView: handoff.PointOfView, Tone: handoff.Tone, Constraints: append([]string(nil), handoff.Constraints...), TargetRunes: handoff.TargetRunes,
	})
}

// RecordOpeningSampleFeedback 把试写意见退回策划。
func (s *Service) RecordOpeningSampleFeedback(ctx context.Context, sampleID, kind string) (loopdomain.OpeningSample, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return loopdomain.OpeningSample{}, err
	}
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	loop, _, _, err := s.loopService(ctx)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	return loop.RecordOpeningSampleFeedback(ctx, runtime.projectID, sampleID, loopdomain.SampleFeedbackKind(kind))
}

// PromoteOpeningSample 仅在档案已确认时接受显式转正。
func (s *Service) PromoteOpeningSample(ctx context.Context, sampleID string) (loopdomain.OpeningSample, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return loopdomain.OpeningSample{}, err
	}
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	view, err := runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	loop, _, _, err := s.loopService(ctx)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	digest := ""
	if view.SampleHandoff != nil {
		digest = view.SampleHandoff.DirectionContentSHA256
		if view.SampleHandoff.Status == "stale" {
			digest = ""
		}
	}
	return loop.PromoteOpeningSample(ctx, chapterapp.PromoteSampleRequest{
		ProjectID: runtime.projectID, SampleID: sampleID, ArchiveConfirmed: view.ArchiveConfirmed, ExplicitAccept: true, CurrentDigest: digest,
	})
}

// CurrentOpeningSample 恢复最近一次非正式试写。
func (s *Service) CurrentOpeningSample(ctx context.Context) (loopdomain.OpeningSample, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	view, err := runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	if view.SampleHandoff == nil {
		return loopdomain.OpeningSample{}, loopdomain.ErrInvalidOpeningSample
	}
	loop, _, _, err := s.loopService(ctx)
	if err != nil {
		return loopdomain.OpeningSample{}, err
	}
	return loop.LatestOpeningSample(ctx, runtime.projectID, view.SampleHandoff.ID)
}

func (s *Service) editableLoop(ctx context.Context) (*chapterapp.Service, string, error) {
	if err := s.requireEditable(); err != nil {
		return nil, "", err
	}
	loop, projectID, _, err := s.loopService(ctx)
	return loop, projectID, err
}

func (s *Service) loopService(ctx context.Context) (*chapterapp.Service, string, int, error) {
	s.mu.Lock()
	if s.active == nil {
		s.mu.Unlock()
		return nil, "", 0, ErrNoActiveProject
	}
	backend, projectID, err := s.loopBackendLocked(ctx)
	generation := s.active.generation
	completer := s.chapterCompleter
	s.mu.Unlock()
	if err != nil {
		return nil, "", 0, err
	}
	budget := 8000
	if completer == nil {
		if s.credentials == nil || s.providers == nil {
			return nil, "", 0, ErrAgentRuntimeUnavailable
		}
		profile, profileErr := s.creationProfile(ctx)
		if profileErr != nil {
			return nil, "", 0, profileErr
		}
		if !profile.ContextBudgetUnlimited && profile.MaxContextChars > 0 {
			budget = profile.MaxContextChars
		}
		completer = profileCompleter{credentials: s.credentials, providers: s.providers, profile: profile}
	}
	guarded := guardedLoopBackend{inner: backend, service: s, generation: generation}
	return chapterapp.NewService(chapterapp.NewBackendStore(guarded), completer, s.clock, s.ids), projectID, budget, nil
}

func (s *Service) loopBackendLocked(ctx context.Context) (chapterapp.Backend, string, error) {
	backend, ok := s.active.store.(chapterapp.Backend)
	if !ok {
		return nil, "", ErrCreationUnavailable
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return nil, "", err
	}
	return backend, project.ID, nil
}

func confirmedOutlineNode(ctx context.Context, store ports.ProjectStore, id string) (storydomain.OutlineNode, error) {
	nodes, err := store.ListOutlineNodes(ctx)
	if err != nil {
		return storydomain.OutlineNode{}, err
	}
	for _, node := range nodes {
		if node.ID == strings.TrimSpace(id) && node.DeletedAt == nil {
			return node, nil
		}
	}
	return storydomain.OutlineNode{}, loopdomain.ErrInvalidProduction
}

type guardedLoopBackend struct {
	inner      chapterapp.Backend
	service    *Service
	generation uint64
}

func (guard guardedLoopBackend) guard() error {
	if guard.service.currentGeneration() != guard.generation {
		return creationapp.ErrStaleStore
	}
	return nil
}

func (guard guardedLoopBackend) AppendChapterLoop(ctx context.Context, record loopdomain.StoredRecord) error {
	if err := guard.guard(); err != nil {
		return err
	}
	return guard.inner.AppendChapterLoop(ctx, record)
}

func (guard guardedLoopBackend) LatestChapterLoop(ctx context.Context, projectID, kind, subjectID string) (loopdomain.StoredRecord, error) {
	if err := guard.guard(); err != nil {
		return loopdomain.StoredRecord{}, err
	}
	return guard.inner.LatestChapterLoop(ctx, projectID, kind, subjectID)
}

func (guard guardedLoopBackend) GetChapterLoop(ctx context.Context, projectID, id string) (loopdomain.StoredRecord, error) {
	if err := guard.guard(); err != nil {
		return loopdomain.StoredRecord{}, err
	}
	return guard.inner.GetChapterLoop(ctx, projectID, id)
}

func (guard guardedLoopBackend) ListChapterLoop(ctx context.Context, projectID, kind string) ([]loopdomain.StoredRecord, error) {
	if err := guard.guard(); err != nil {
		return nil, err
	}
	return guard.inner.ListChapterLoop(ctx, projectID, kind)
}

type profileCompleter struct {
	credentials ports.CredentialStore
	providers   ports.ModelProviderFactory
	profile     agentdomain.ResolvedCreativeProfile
}

// Complete 按阶段角色调用项目已固定的供应商，不另建运行时。
func (completer profileCompleter) Complete(ctx context.Context, call chapterapp.Call) (chapterapp.Completion, error) {
	role := agentdomain.RoleWriter
	switch call.Role {
	case "planner":
		role = agentdomain.RolePlanner
	case "reviewer":
		role = agentdomain.RoleContinuityReviewer
	case "writer":
		role = agentdomain.RoleWriter
	default:
		return chapterapp.Completion{}, loopdomain.ErrInvalidProduction
	}
	var resolved agentdomain.ResolvedRole
	found := false
	for _, item := range completer.profile.Roles {
		if item.Role == role {
			resolved = item
			found = true
		}
	}
	if !found {
		return chapterapp.Completion{}, ErrProjectProfileNotConfigured
	}
	apiKey, err := completer.credentials.Get(ctx, resolved.CredentialRef)
	if err != nil {
		return chapterapp.Completion{}, err
	}
	provider, err := completer.providers.New(agentdomain.ProviderConnection{
		ID: resolved.ConnectionID, Name: resolved.ProviderName, Kind: resolved.ProviderKind,
		BaseURL: resolved.BaseURL, CredentialRef: resolved.CredentialRef,
	})
	if err != nil {
		return chapterapp.Completion{}, err
	}
	tokens := resolved.MaxOutputTokens
	if tokens < 1 {
		tokens = 1024
	}
	generated, err := provider.Generate(ctx, apiKey, ports.ModelGenerateRequest{
		Model: resolved.Model, System: "你是长篇小说创作助手。只输出要求的正文或 JSON。", Input: call.Input,
		MaxOutputTokens: tokens, ReasoningEffort: resolved.ReasoningEffort, Temperature: resolved.Temperature,
	}, nil)
	if err != nil {
		return chapterapp.Completion{}, err
	}
	return chapterapp.Completion{Text: generated.Text, InputTokens: generated.InputTokens, OutputTokens: generated.OutputTokens}, nil
}
