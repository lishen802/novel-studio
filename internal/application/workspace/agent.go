package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

type ProviderConnectionInput struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
	BaseURL string `json:"baseUrl"`
	APIKey  string `json:"apiKey"`
}

type ProviderConnectionView struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	BaseURL       string `json:"baseUrl"`
	HasCredential bool   `json:"hasCredential"`
	UpdatedAt     string `json:"updatedAt"`
}

type AgentProfileInput struct {
	Role            string  `json:"role"`
	ConnectionID    string  `json:"connectionId"`
	Model           string  `json:"model"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Temperature     float64 `json:"temperature"`
	Enabled         bool    `json:"enabled"`
}

type AgentProfileView struct {
	ID              string  `json:"id"`
	Role            string  `json:"role"`
	ConnectionID    string  `json:"connectionId"`
	Model           string  `json:"model"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	Temperature     float64 `json:"temperature"`
	Enabled         bool    `json:"enabled"`
	UpdatedAt       string  `json:"updatedAt"`
}

type AgentRunStepView struct {
	ID           string `json:"id"`
	Role         string `json:"role"`
	Position     int    `json:"position"`
	Status       string `json:"status"`
	Output       string `json:"output"`
	ErrorCode    string `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

type AgentRunView struct {
	ID           string             `json:"id"`
	ChapterID    string             `json:"chapterId"`
	Prompt       string             `json:"prompt"`
	Status       string             `json:"status"`
	ErrorCode    string             `json:"errorCode"`
	ErrorMessage string             `json:"errorMessage"`
	ChangeSetID  string             `json:"changeSetId"`
	RetryOf      string             `json:"retryOf"`
	Steps        []AgentRunStepView `json:"steps"`
	CreatedAt    string             `json:"createdAt"`
	UpdatedAt    string             `json:"updatedAt"`
}

func (s *Service) SaveProviderConnection(
	ctx context.Context,
	input ProviderConnectionInput,
) (ProviderConnectionView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return ProviderConnectionView{}, err
	}
	if s.credentials == nil {
		return ProviderConnectionView{}, ErrAgentRuntimeUnavailable
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return ProviderConnectionView{}, err
	}
	id := strings.TrimSpace(input.ID)
	createdAt := s.clock.Now()
	credentialRef := ""
	if id == "" {
		id = s.ids.NewID()
		credentialRef = "provider:" + id
	} else {
		existing, err := store.ProviderConnection(ctx, id)
		if err != nil {
			return ProviderConnectionView{}, err
		}
		createdAt = existing.CreatedAt
		credentialRef = existing.CredentialRef
	}
	connection, err := agentdomain.NewProviderConnection(
		id, project.ID, input.Name, agentdomain.ProviderKind(input.Kind),
		input.BaseURL, credentialRef, s.clock.Now(),
	)
	if err != nil {
		return ProviderConnectionView{}, err
	}
	connection.CreatedAt = createdAt
	if strings.TrimSpace(input.APIKey) != "" {
		if err := s.credentials.Save(ctx, credentialRef, input.APIKey); err != nil {
			return ProviderConnectionView{}, fmt.Errorf("save provider credential: %w", err)
		}
	}
	if _, err := s.credentials.Get(ctx, credentialRef); err != nil {
		return ProviderConnectionView{}, fmt.Errorf("provider API key is required: %w", err)
	}
	if err := store.UpsertProviderConnection(ctx, connection); err != nil {
		return ProviderConnectionView{}, err
	}
	return providerConnectionView(connection, true), nil
}

func (s *Service) ProviderConnections(ctx context.Context) ([]ProviderConnectionView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return nil, err
	}
	connections, err := store.ListProviderConnections(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]ProviderConnectionView, 0, len(connections))
	for _, connection := range connections {
		hasCredential := false
		if s.credentials != nil {
			_, credentialErr := s.credentials.Get(ctx, connection.CredentialRef)
			hasCredential = credentialErr == nil
		}
		result = append(result, providerConnectionView(connection, hasCredential))
	}
	return result, nil
}

func (s *Service) DeleteProviderConnection(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return err
	}
	connection, err := store.ProviderConnection(ctx, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if err := store.DeleteProviderConnection(ctx, connection.ID); err != nil {
		return err
	}
	if s.credentials != nil {
		if err := s.credentials.Delete(ctx, connection.CredentialRef); err != nil {
			return fmt.Errorf("delete provider credential: %w", err)
		}
	}
	return nil
}

func (s *Service) TestProviderConnection(ctx context.Context, id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return err
	}
	if s.credentials == nil || s.providers == nil {
		return ErrAgentRuntimeUnavailable
	}
	connection, err := store.ProviderConnection(ctx, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	profiles, err := store.ListAgentProfiles(ctx)
	if err != nil {
		return err
	}
	model := ""
	for _, profile := range profiles {
		if profile.ConnectionID == connection.ID {
			model = profile.Model
			break
		}
	}
	if model == "" {
		return errors.New("请先为此连接配置一个智能体模型")
	}
	apiKey, err := s.credentials.Get(ctx, connection.CredentialRef)
	if err != nil {
		return err
	}
	provider, err := s.providers.New(connection)
	if err != nil {
		return err
	}
	_, err = provider.Generate(ctx, apiKey, ports.ModelGenerateRequest{
		Model: model, System: "Return a concise acknowledgement.",
		Input: "Connection test.", MaxOutputTokens: 16,
	}, nil)
	return err
}

func (s *Service) SaveAgentProfile(
	ctx context.Context,
	input AgentProfileInput,
) (AgentProfileView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return AgentProfileView{}, err
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return AgentProfileView{}, err
	}
	id := s.ids.NewID()
	createdAt := s.clock.Now()
	profiles, err := store.ListAgentProfiles(ctx)
	if err != nil {
		return AgentProfileView{}, err
	}
	for _, existing := range profiles {
		if existing.Role == agentdomain.AgentRole(input.Role) {
			id = existing.ID
			createdAt = existing.CreatedAt
			break
		}
	}
	if _, err := store.ProviderConnection(ctx, input.ConnectionID); err != nil {
		return AgentProfileView{}, err
	}
	profile, err := agentdomain.NewAgentProfile(
		id, project.ID, agentdomain.AgentRole(input.Role), input.ConnectionID, input.Model,
		input.MaxOutputTokens, input.Temperature, input.Enabled, s.clock.Now(),
	)
	if err != nil {
		return AgentProfileView{}, err
	}
	profile.CreatedAt = createdAt
	if err := store.UpsertAgentProfile(ctx, profile); err != nil {
		return AgentProfileView{}, err
	}
	return agentProfileView(profile), nil
}

func (s *Service) AgentProfiles(ctx context.Context) ([]AgentProfileView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return nil, err
	}
	profiles, err := store.ListAgentProfiles(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]AgentProfileView, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, agentProfileView(profile))
	}
	return result, nil
}

func (s *Service) CreateChapterAgentRun(
	ctx context.Context,
	chapterID, prompt string,
) (AgentRunView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return AgentRunView{}, err
	}
	if s.credentials == nil || s.providers == nil {
		return AgentRunView{}, ErrAgentRuntimeUnavailable
	}
	chapter, err := s.active.store.Chapter(ctx, strings.TrimSpace(chapterID))
	if err != nil {
		return AgentRunView{}, err
	}
	profiles, err := store.ListAgentProfiles(ctx)
	if err != nil {
		return AgentRunView{}, err
	}
	if err := agentdomain.ValidateAuthoringProfiles(profiles); err != nil {
		return AgentRunView{}, err
	}
	byRole := make(map[agentdomain.AgentRole]agentdomain.AgentProfile, len(profiles))
	for _, profile := range profiles {
		byRole[profile.Role] = profile
		connection, err := store.ProviderConnection(ctx, profile.ConnectionID)
		if err != nil {
			return AgentRunView{}, err
		}
		if _, err := s.credentials.Get(ctx, connection.CredentialRef); err != nil {
			return AgentRunView{}, fmt.Errorf("missing credential for %s: %w", profile.Role, err)
		}
	}
	run, err := agentdomain.NewChapterDraftRun(
		s.ids.NewID(), chapter.ProjectID, chapter.ID, chapter.Revision, prompt,
		[]agentdomain.RunStepSpec{
			{ID: s.ids.NewID(), Role: agentdomain.RolePlanner, ProfileID: byRole[agentdomain.RolePlanner].ID},
			{ID: s.ids.NewID(), Role: agentdomain.RoleWriter, ProfileID: byRole[agentdomain.RoleWriter].ID},
			{ID: s.ids.NewID(), Role: agentdomain.RoleContinuityReviewer, ProfileID: byRole[agentdomain.RoleContinuityReviewer].ID},
		},
		s.clock.Now(),
	)
	if err != nil {
		return AgentRunView{}, err
	}
	if err := store.CreateAgentRun(ctx, run); err != nil {
		return AgentRunView{}, err
	}
	return agentRunView(run), nil
}

func (s *Service) ExecuteAgentRun(ctx context.Context, id string) (AgentRunView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return AgentRunView{}, err
	}
	if s.credentials == nil || s.providers == nil {
		return AgentRunView{}, ErrAgentRuntimeUnavailable
	}
	run, err := store.AgentRun(ctx, strings.TrimSpace(id))
	if err != nil {
		return AgentRunView{}, err
	}
	if run.Status != agentdomain.RunQueued {
		return AgentRunView{}, fmt.Errorf("agent run is not queued: %s", run.Status)
	}
	run, _ = run.Transition(agentdomain.RunRunning, "", s.clock.Now())
	if err := store.UpdateAgentRun(ctx, run); err != nil {
		return AgentRunView{}, err
	}
	failed := func(cause error) (AgentRunView, error) {
		run.ErrorCode = "provider_error"
		run, _ = run.Transition(agentdomain.RunFailed, cause.Error(), s.clock.Now())
		_ = store.UpdateAgentRun(ctx, run)
		return agentRunView(run), cause
	}

	chapter, err := s.active.store.Chapter(ctx, run.ChapterID)
	if err != nil {
		return failed(err)
	}
	contextJSON, err := s.assembleAuthoringContext(ctx, chapter.ID)
	if err != nil {
		return failed(err)
	}
	if err := store.CreateAgentArtifact(ctx, newArtifact(
		s.ids.NewID(), run.ID, "", agentdomain.ArtifactContext, contextJSON, s.clock.Now(),
	)); err != nil {
		return failed(err)
	}

	profiles, err := store.ListAgentProfiles(ctx)
	if err != nil {
		return failed(err)
	}
	profileByID := make(map[string]agentdomain.AgentProfile, len(profiles))
	for _, profile := range profiles {
		profileByID[profile.ID] = profile
	}
	var plan, draft, review string
	for index := range run.Steps {
		step := run.Steps[index]
		profile := profileByID[step.ProfileID]
		connection, err := store.ProviderConnection(ctx, profile.ConnectionID)
		if err != nil {
			return failed(err)
		}
		apiKey, err := s.credentials.Get(ctx, connection.CredentialRef)
		if err != nil {
			return failed(err)
		}
		provider, err := s.providers.New(connection)
		if err != nil {
			return failed(err)
		}
		now := s.clock.Now().UTC()
		step.Status = agentdomain.StepRunning
		step.StartedAt = &now
		step.UpdatedAt = now
		if err := store.UpdateAgentRunStep(ctx, step); err != nil {
			return failed(err)
		}
		request := agentRequest(step.Role, profile, run.Prompt, contextJSON, plan, draft)
		generated, err := provider.Generate(ctx, apiKey, request, func(delta string) {
			step.Output += delta
		})
		if err != nil {
			step.Status = agentdomain.StepFailed
			step.ErrorCode = "provider_error"
			step.ErrorMessage = err.Error()
			completedAt := s.clock.Now().UTC()
			step.CompletedAt = &completedAt
			step.UpdatedAt = completedAt
			_ = store.UpdateAgentRunStep(ctx, step)
			run.Steps[index] = step
			return failed(err)
		}
		step.Output = generated.Text
		if err := validateStructuredOutput(step.Role, generated.Text); err != nil {
			step.Status = agentdomain.StepFailed
			step.ErrorCode = "invalid_structured_output"
			step.ErrorMessage = err.Error()
			completedAt := s.clock.Now().UTC()
			step.CompletedAt = &completedAt
			step.UpdatedAt = completedAt
			_ = store.UpdateAgentRunStep(ctx, step)
			run.Steps[index] = step
			return failed(err)
		}
		step.Status = agentdomain.StepCompleted
		completedAt := s.clock.Now().UTC()
		step.CompletedAt = &completedAt
		step.UpdatedAt = completedAt
		if err := store.UpdateAgentRunStep(ctx, step); err != nil {
			return failed(err)
		}
		run.Steps[index] = step
		kind := agentdomain.ArtifactPlan
		switch step.Role {
		case agentdomain.RolePlanner:
			plan = generated.Text
		case agentdomain.RoleWriter:
			draft = generated.Text
			kind = agentdomain.ArtifactDraft
		case agentdomain.RoleContinuityReviewer:
			review = generated.Text
			kind = agentdomain.ArtifactReview
		}
		if err := store.CreateAgentArtifact(ctx, newArtifact(
			s.ids.NewID(), run.ID, step.ID, kind, generated.Text, s.clock.Now(),
		)); err != nil {
			return failed(err)
		}
	}

	var reviewOutput struct {
		Summary string `json:"summary"`
	}
	_ = json.Unmarshal([]byte(review), &reviewOutput)
	changeSet, err := revisiondomain.NewChapterBodyChangeSet(
		s.ids.NewID(), s.ids.NewID(), run.ProjectID, run.ChapterID,
		"智能体草稿："+chapter.Title,
		strings.TrimSpace(reviewOutput.Summary),
		"agent-run:"+run.ID,
		run.ChapterBaseRevision, chapter.Body, draft,
		"由规划、写作和连续性审阅智能体生成，等待作者确认。",
		s.clock.Now(),
	)
	if err != nil {
		return failed(err)
	}
	if err := s.active.store.CreateChangeSet(ctx, changeSet); err != nil {
		return failed(err)
	}
	run.ChangeSetID = changeSet.ID
	run, err = run.Transition(agentdomain.RunCompleted, "", s.clock.Now())
	if err != nil {
		return failed(err)
	}
	if err := store.UpdateAgentRun(ctx, run); err != nil {
		return AgentRunView{}, err
	}
	return agentRunView(run), nil
}

func (s *Service) AgentRuns(ctx context.Context) ([]AgentRunView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return nil, err
	}
	runs, err := store.ListAgentRuns(ctx, 50)
	if err != nil {
		return nil, err
	}
	result := make([]AgentRunView, 0, len(runs))
	for _, run := range runs {
		result = append(result, agentRunView(run))
	}
	return result, nil
}

func (s *Service) AgentRun(ctx context.Context, id string) (AgentRunView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	store, err := s.agentStoreLocked()
	if err != nil {
		return AgentRunView{}, err
	}
	run, err := store.AgentRun(ctx, strings.TrimSpace(id))
	if err != nil {
		return AgentRunView{}, err
	}
	return agentRunView(run), nil
}

func (s *Service) agentStoreLocked() (ports.AgentStore, error) {
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	store, ok := s.active.store.(ports.AgentStore)
	if !ok {
		return nil, ErrAgentStoreUnavailable
	}
	return store, nil
}

func (s *Service) assembleAuthoringContext(ctx context.Context, chapterID string) (string, error) {
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return "", err
	}
	chapter, err := s.active.store.Chapter(ctx, chapterID)
	if err != nil {
		return "", err
	}
	chapters, err := s.active.store.ListChapters(ctx)
	if err != nil {
		return "", err
	}
	outline, err := s.active.store.ListOutlineNodes(ctx)
	if err != nil {
		return "", err
	}
	scenes, err := s.active.store.ListSceneCards(ctx, chapterID)
	if err != nil {
		return "", err
	}
	characters, err := s.active.store.ListCharacters(ctx)
	if err != nil {
		return "", err
	}
	factions, err := s.active.store.ListFactions(ctx)
	if err != nil {
		return "", err
	}
	relationships, err := s.active.store.ListRelationships(ctx)
	if err != nil {
		return "", err
	}
	regions, err := s.active.store.ListWorldRegions(ctx)
	if err != nil {
		return "", err
	}
	locations, err := s.active.store.ListLocations(ctx)
	if err != nil {
		return "", err
	}
	rules, err := s.active.store.ListWorldRules(ctx)
	if err != nil {
		return "", err
	}
	hooks, err := s.active.store.ListForeshadowingHooks(ctx)
	if err != nil {
		return "", err
	}
	events, err := s.active.store.ListStoryEvents(ctx)
	if err != nil {
		return "", err
	}
	facts, err := s.active.store.ListFacts(ctx)
	if err != nil {
		return "", err
	}
	knowledge, err := s.active.store.ListCharacterKnowledge(ctx)
	if err != nil {
		return "", err
	}
	payload := map[string]any{
		"contract": map[string]any{
			"confirmedFactsAreHardConstraints": true,
			"candidateFactsRequireCaution":     true,
			"chapterBodyIsReferenceOnly":       true,
		},
		"project": project, "targetChapter": chapter, "chapters": chapters,
		"outline": outline, "sceneCards": scenes,
		"characters": characters, "factions": factions, "relationships": relationships,
		"regions": regions, "locations": locations, "worldRules": rules,
		"foreshadowingHooks": hooks, "storyEvents": events,
		"canonFacts": facts, "characterKnowledge": knowledge,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode agent context: %w", err)
	}
	return string(encoded), nil
}

func agentRequest(
	role agentdomain.AgentRole,
	profile agentdomain.AgentProfile,
	goal, contextJSON, plan, draft string,
) ports.ModelGenerateRequest {
	request := ports.ModelGenerateRequest{
		Model: profile.Model, MaxOutputTokens: profile.MaxOutputTokens,
		Temperature: profile.Temperature,
	}
	switch role {
	case agentdomain.RolePlanner:
		request.System = "你是长篇小说规划智能体。基于给定事实设计可执行的章节场景，不得改写已确认事实。只返回符合 JSON Schema 的结果。"
		request.Input = "作者目标：\n" + goal + "\n\n项目上下文：\n" + contextJSON
		request.SchemaName = "chapter_plan"
		request.JSONSchema = json.RawMessage(planSchema)
	case agentdomain.RoleWriter:
		request.System = "你是长篇小说正文写作智能体。严格遵守项目上下文和规划，直接输出完整章节正文，不要解释。"
		request.Input = "作者目标：\n" + goal + "\n\n项目上下文：\n" + contextJSON + "\n\n章节规划：\n" + plan
	case agentdomain.RoleContinuityReviewer:
		request.System = "你是独立的小说连续性审阅智能体。检查草稿与事实、人物、时间线和伏笔是否冲突。只返回符合 JSON Schema 的结果。"
		request.Input = "项目上下文：\n" + contextJSON + "\n\n待审草稿：\n" + draft
		request.SchemaName = "continuity_review"
		request.JSONSchema = json.RawMessage(reviewSchema)
	}
	return request
}

func validateStructuredOutput(role agentdomain.AgentRole, output string) error {
	switch role {
	case agentdomain.RolePlanner:
		var value struct {
			Summary string `json:"summary"`
			Scenes  []struct {
				Title    string `json:"title"`
				Goal     string `json:"goal"`
				Conflict string `json:"conflict"`
				Outcome  string `json:"outcome"`
			} `json:"scenes"`
		}
		if err := json.Unmarshal([]byte(output), &value); err != nil || strings.TrimSpace(value.Summary) == "" || len(value.Scenes) == 0 {
			return errors.New("规划智能体返回了无效结构")
		}
	case agentdomain.RoleContinuityReviewer:
		var value struct {
			Approved *bool  `json:"approved"`
			Summary  string `json:"summary"`
			Issues   []any  `json:"issues"`
		}
		if err := json.Unmarshal([]byte(output), &value); err != nil || value.Approved == nil || strings.TrimSpace(value.Summary) == "" {
			return errors.New("连续性审阅智能体返回了无效结构")
		}
	}
	return nil
}

func newArtifact(
	id, runID, stepID string,
	kind agentdomain.ArtifactKind,
	content string,
	now time.Time,
) agentdomain.AgentArtifact {
	sum := sha256.Sum256([]byte(content))
	return agentdomain.AgentArtifact{
		ID: id, RunID: runID, StepID: stepID, Kind: kind, Content: content,
		SHA256: hex.EncodeToString(sum[:]), CreatedAt: now.UTC(),
	}
}

func providerConnectionView(connection agentdomain.ProviderConnection, hasCredential bool) ProviderConnectionView {
	return ProviderConnectionView{
		ID: connection.ID, Name: connection.Name, Kind: string(connection.Kind),
		BaseURL: connection.BaseURL, HasCredential: hasCredential,
		UpdatedAt: connection.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func agentProfileView(profile agentdomain.AgentProfile) AgentProfileView {
	return AgentProfileView{
		ID: profile.ID, Role: string(profile.Role), ConnectionID: profile.ConnectionID,
		Model: profile.Model, MaxOutputTokens: profile.MaxOutputTokens,
		Temperature: profile.Temperature, Enabled: profile.Enabled,
		UpdatedAt: profile.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func agentRunView(run agentdomain.AgentRun) AgentRunView {
	steps := make([]AgentRunStepView, 0, len(run.Steps))
	for _, step := range run.Steps {
		steps = append(steps, AgentRunStepView{
			ID: step.ID, Role: string(step.Role), Position: step.Position,
			Status: string(step.Status), Output: step.Output,
			ErrorCode: step.ErrorCode, ErrorMessage: step.ErrorMessage,
		})
	}
	return AgentRunView{
		ID: run.ID, ChapterID: run.ChapterID, Prompt: run.Prompt,
		Status: string(run.Status), ErrorCode: run.ErrorCode, ErrorMessage: run.ErrorMessage,
		ChangeSetID: run.ChangeSetID, RetryOf: run.RetryOf, Steps: steps,
		CreatedAt: run.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt: run.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}

const planSchema = `{
	"type":"object",
	"additionalProperties":false,
	"required":["summary","scenes"],
	"properties":{
		"summary":{"type":"string"},
		"scenes":{"type":"array","minItems":1,"items":{
			"type":"object","additionalProperties":false,
			"required":["title","goal","conflict","outcome"],
			"properties":{
				"title":{"type":"string"},"goal":{"type":"string"},
				"conflict":{"type":"string"},"outcome":{"type":"string"}
			}
		}}
	}
}`

const reviewSchema = `{
	"type":"object",
	"additionalProperties":false,
	"required":["approved","summary","issues"],
	"properties":{
		"approved":{"type":"boolean"},
		"summary":{"type":"string"},
		"issues":{"type":"array","items":{
			"type":"object","additionalProperties":false,
			"required":["severity","category","description","suggestion"],
			"properties":{
				"severity":{"type":"string"},"category":{"type":"string"},
				"description":{"type":"string"},"suggestion":{"type":"string"}
			}
		}}
	}
}`
