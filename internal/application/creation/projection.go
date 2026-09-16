package creation

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type WorkbenchView struct {
	Revisions         []RevisionView                `json:"revisions"`
	DirectionActions  []DirectionActionView         `json:"directionActions"`
	FieldEditor       FieldEditorView               `json:"fieldEditor"`
	Review            ReviewView                    `json:"review"`
	ReviewCursor      ReviewCursorView              `json:"reviewCursor"`
	CandidateVersions []CandidateVersionSummaryView `json:"candidateVersions"`
	SampleHandoff     *SampleView                   `json:"sampleHandoff"`
	CanPrepareSample  bool                          `json:"canPrepareSample"`
	Material          MaterialView                  `json:"material"`
	ContentVersion    int64                         `json:"contentVersion"`
	RunSlotVersion    int64                         `json:"runSlotVersion"`
	Entry             EntryView                     `json:"entry"`
	ReadOnly          bool                          `json:"readOnly"`
	Applicability     string                        `json:"applicability"`
	SessionID         string                        `json:"sessionId"`
	ProjectID         string                        `json:"projectId"`
	BranchID          string                        `json:"branchId"`
	Stage             creationdomain.Stage          `json:"stage"`
	AdvanceMode       creationdomain.AdvanceMode    `json:"advanceMode"`
	BlockingReason    creationdomain.BlockingReason `json:"blockingReason"`
	Recoverable       bool                          `json:"recoverable"`
	ArchiveConfirmed  bool                          `json:"archiveConfirmed"`
	IntentText        string                        `json:"intentText"`
	StageRail         []RailStopView                `json:"stageRail"`
	DirectionSet      DirectionSetView              `json:"directionSet"`
	Proposal          ProposalView                  `json:"proposal"`
	PeopleWorld       PeopleWorldView               `json:"peopleWorld"`
	Roadmap           RoadmapView                   `json:"roadmap"`
	CurrentRun        RunView                       `json:"currentRun"`
	Assistant         AssistantSummaryView          `json:"assistant"`
	Impact            ImpactProjection              `json:"impact"`
	Capabilities      []string                      `json:"capabilities"`
}

type RailStopView struct {
	ID     creationdomain.Stage `json:"id"`
	Label  string               `json:"label"`
	Status string               `json:"status"`
}

type DirectionSetView struct {
	CandidateID         string                  `json:"candidateId"`
	SelectedCandidateID string                  `json:"selectedCandidateId"`
	Version             int                     `json:"version"`
	SelectedID          string                  `json:"selectedId"`
	KeptIDs             []string                `json:"keptIds"`
	Status              string                  `json:"status"`
	Cards               []DirectionCardView     `json:"cards"`
	Combined            []CombinedDirectionView `json:"combined"`
}

type CombinedDirectionView struct {
	CandidateID string            `json:"candidateId"`
	Version     int               `json:"version"`
	Card        DirectionCardView `json:"card"`
}

type DirectionCardView struct {
	ID           string `json:"id"`
	Hook         string `json:"hook"`
	Genre        string `json:"genre"`
	Protagonist  string `json:"protagonist"`
	CoreConflict string `json:"coreConflict"`
	GrowthCost   string `json:"growthCost"`
	Pacing       string `json:"pacing"`
	Ending       string `json:"ending"`
	Selected     bool   `json:"selected"`
}

type ProposalView struct {
	CandidateID string                `json:"candidateId"`
	Version     int                   `json:"version"`
	Status      string                `json:"status"`
	Title       string                `json:"title"`
	Sections    []ProposalSectionView `json:"sections"`
}

type ProposalSectionView struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	LockPath string `json:"lockPath"`
	Locked   bool   `json:"locked"`
	Accepted bool   `json:"accepted"`
	Rejected bool   `json:"rejected"`
}

type PeopleWorldView struct {
	CandidateID string                `json:"candidateId"`
	Version     int                   `json:"version"`
	Status      string                `json:"status"`
	Entities    []PeopleWorldNodeView `json:"entities"`
}

type PeopleWorldNodeView struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Complete bool   `json:"complete"`
	Accepted bool   `json:"accepted"`
	Rejected bool   `json:"rejected"`
}

type RoadmapView struct {
	CandidateID string            `json:"candidateId"`
	Version     int               `json:"version"`
	Status      string            `json:"status"`
	Book        RoadmapNodeView   `json:"book"`
	Volumes     []RoadmapNodeView `json:"volumes"`
	Arcs        []RoadmapNodeView `json:"arcs"`
	Chapters    []RoadmapNodeView `json:"chapters"`
}

type RoadmapGoalItemView struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
	Detail    string `json:"detail"`
}

type RoadmapNodeView struct {
	ParentID           string                `json:"parentId"`
	ID                 string                `json:"id"`
	Title              string                `json:"title"`
	Summary            string                `json:"summary"`
	Position           int                   `json:"position"`
	Accepted           bool                  `json:"accepted"`
	Rejected           bool                  `json:"rejected"`
	ReaderExpectation  string                `json:"readerExpectation,omitempty"`
	StagePayoff        string                `json:"stagePayoff,omitempty"`
	IrreversibleChange string                `json:"irreversibleChange,omitempty"`
	EndingContribution string                `json:"endingContribution,omitempty"`
	Commitments        []RoadmapGoalItemView `json:"commitments,omitempty"`
	RetainedQuestions  []RoadmapGoalItemView `json:"retainedQuestions,omitempty"`
}

type RunProgressStep struct {
	Stage  string `json:"stage"`
	Status string `json:"status"`
}
type RunView struct {
	Steps       []RunProgressStep `json:"steps"`
	CreatedAt   string            `json:"createdAt"`
	UpdatedAt   string            `json:"updatedAt"`
	RunID       string            `json:"runId"`
	Status      string            `json:"status"`
	Cancellable bool              `json:"cancellable"`
	PublicStage string            `json:"publicStage"`
}

type AssistantSummaryView struct {
	PendingActionCount int                           `json:"pendingActionCount"`
	Messages           []AssistantMessageView        `json:"messages"`
	Authorization      *DiscussionAuthorizationView  `json:"authorization,omitempty"`
	Authorizations     []DiscussionAuthorizationView `json:"authorizations,omitempty"`
	PendingDiscussion  *DiscussionTurnView           `json:"pendingDiscussion,omitempty"`
}

type DiscussionSuggestionView struct {
	ID            string   `json:"id"`
	Paths         []string `json:"paths"`
	Instruction   string   `json:"instruction"`
	PreservePaths []string `json:"preservePaths"`
	Rationale     string   `json:"rationale"`
}

type AssistantMessageView struct {
	ID              string                     `json:"id"`
	Role            string                     `json:"role"`
	Kind            string                     `json:"kind,omitempty"`
	Status          string                     `json:"status,omitempty"`
	Text            string                     `json:"text"`
	Stage           string                     `json:"stage"`
	CandidateID     string                     `json:"candidateId"`
	EntityRef       string                     `json:"entityRef"`
	FieldPath       string                     `json:"fieldPath"`
	ActionID        string                     `json:"actionId"`
	ActionState     string                     `json:"actionState"`
	AuthorizationID string                     `json:"authorizationId,omitempty"`
	RunID           string                     `json:"runId,omitempty"`
	ReplyToID       string                     `json:"replyToId,omitempty"`
	ErrorCode       string                     `json:"errorCode,omitempty"`
	Suggestions     []DiscussionSuggestionView `json:"suggestions,omitempty"`
	CreatedAt       string                     `json:"createdAt"`
}

type CandidateView struct {
	ID          string               `json:"id"`
	Stage       creationdomain.Stage `json:"stage"`
	Kind        string               `json:"kind"`
	Version     int                  `json:"version"`
	Status      string               `json:"status"`
	PayloadJSON string               `json:"payloadJson"`
	LockMap     map[string]bool      `json:"lockMap"`
}

// Workbench 组装驾驶舱权威投影，不含 Prompt、密钥或隐藏推理。
func (service *Service) Workbench(ctx context.Context, projectID, branchID string) (WorkbenchView, error) {
	session, err := service.loadSession(ctx, projectID)
	if isMissingSession(err) {
		view := assembleWorkbench(creationdomain.Session{ProjectID: projectID, BranchID: branchID,
			Stage: creationdomain.StageIntent, AdvanceMode: creationdomain.ModeStepReview}, nil,
			ImpactProjection{Items: []ImpactItem{}, RegenerateStages: []creationdomain.Stage{}}, 0,
			creationdomain.CandidateReview{}, creationdomain.ReviewCursor{})
		view.Applicability = "not_started"
		view.Capabilities = []string{}
		return view, nil
	}
	if err != nil {
		return WorkbenchView{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return WorkbenchView{}, err
	}
	impact, err := service.projectImpact(ctx, session)
	if err != nil {
		return WorkbenchView{}, err
	}
	pending, err := service.store.PendingActionCount(ctx, session.ProjectID)
	if err != nil {
		return WorkbenchView{}, err
	}
	review := creationdomain.CandidateReview{}
	if current, ok := currentCandidate(candidates, session.Stage); ok {
		stored, err := service.store.Review(ctx, current.ID)
		if err != nil {
			return WorkbenchView{}, err
		}
		if stored.SessionID == "" {
			stored, err = creationdomain.NewCandidateReview(current, service.clock.Now())
			if err != nil {
				return WorkbenchView{}, err
			}
		}
		review = stored
	}
	cursor, err := service.store.ReviewCursor(ctx, session.ID)
	if errors.Is(err, ErrReviewCursorNotFound) {
		cursor = creationdomain.ReviewCursor{}
	} else if err != nil {
		return WorkbenchView{}, err
	}
	view := assembleWorkbench(session, candidates, impact, pending, review, cursor)
	actionStates := map[string]string{}
	messages := []assistantdomain.Message{}
	assistantSession, assistantErr := service.store.AssistantSession(ctx, session.ProjectID, session.BranchID)
	if assistantErr == nil {
		actions, listErr := service.store.ListActions(ctx, assistantSession.ID)
		if listErr != nil {
			return WorkbenchView{}, listErr
		}
		for _, action := range actions {
			actionStates[action.ID] = string(action.State)
			if action.Operation == assistantdomain.OpAuthorizeDiscussion && action.State == assistantdomain.StatePendingConfirmation {
				if input, decodeErr := decodeDiscussionInput(action); decodeErr == nil {
					pendingView, viewErr := service.discussionPreviewView(ctx, session, action, input)
					if viewErr == nil {
						view.Assistant.PendingDiscussion = &pendingView
					}
				}
			}
		}
		messages, listErr = service.store.ListAssistantMessages(ctx, assistantSession.ID)
		if listErr != nil {
			return WorkbenchView{}, listErr
		}
	} else if !isMissingSession(assistantErr) {
		return WorkbenchView{}, assistantErr
	}
	view.Assistant.Messages = make([]AssistantMessageView, 0, len(messages))
	for _, message := range messages {
		view.Assistant.Messages = append(view.Assistant.Messages, projectAssistantMessage(message, actionStates[message.ActionID]))
	}
	if current, ok := currentCandidate(candidates, session.Stage); ok && assistantErr == nil {
		auths, listErr := service.store.ListAuthorizations(ctx, assistantSession.ID)
		if listErr != nil {
			return WorkbenchView{}, listErr
		}
		var matched assistantdomain.Authorization
		view.Assistant.Authorizations = make([]DiscussionAuthorizationView, 0, len(auths))
		for _, auth := range auths {
			if auth.CandidateID != current.ID {
				continue
			}
			action, actionErr := service.store.Action(ctx, auth.ActionID)
			cost := CostPreview{}
			stale := auth.State == assistantdomain.AuthStale
			if actionErr == nil {
				if input, decodeErr := decodeDiscussionInput(action); decodeErr == nil {
					cost = input.Cost
					stale = stale || service.discussionCurrent(ctx, session, input) != nil
				}
			}
			view.Assistant.Authorizations = append(view.Assistant.Authorizations, projectAuthorization(auth, cost, stale, service.clock.Now()))
			authActive := auth.State == assistantdomain.AuthActive && (auth.ExpiresAt.IsZero() || service.clock.Now().Before(auth.ExpiresAt))
			matchedActive := matched.State == assistantdomain.AuthActive && (matched.ExpiresAt.IsZero() || service.clock.Now().Before(matched.ExpiresAt))
			if matched.ID == "" || (authActive && !matchedActive) || (authActive == matchedActive && auth.CreatedAt.After(matched.CreatedAt)) {
				matched = auth
			}
		}
		if matched.ID != "" {
			action, actionErr := service.store.Action(ctx, matched.ActionID)
			cost := CostPreview{}
			stale := matched.State == assistantdomain.AuthStale
			if actionErr == nil {
				if input, decodeErr := decodeDiscussionInput(action); decodeErr == nil {
					cost = input.Cost
					stale = stale || service.discussionCurrent(ctx, session, input) != nil
				}
			}
			viewAuth := projectAuthorization(matched, cost, stale, service.clock.Now())
			view.Assistant.Authorization = &viewAuth
		}
	}
	if session.Stage != creationdomain.StageIntent {
		if current, ok := currentCandidate(candidates, session.Stage); ok {
			view.FieldEditor = candidateTextFields(current)
		}
	}
	if err := service.projectSample(ctx, session, candidates, &view); err != nil {
		return WorkbenchView{}, err
	}
	runID := session.ActiveRunID
	if runID == "" {
		// 已物化候选仍保留运行入口，不能因槽位清空而丢失完成结果。
		var latest creationdomain.Candidate
		for _, candidate := range candidates {
			if candidate.RunID != "" && (latest.ID == "" || candidate.CreatedAt.After(latest.CreatedAt) ||
				(candidate.CreatedAt.Equal(latest.CreatedAt) && candidate.Version > latest.Version)) {
				latest = candidate
			}
		}
		runID = latest.RunID
	}
	if runID != "" {
		run, readErr := service.kernel.Run(ctx, runID)
		if readErr != nil {
			view.CurrentRun = RunView{RunID: runID, Status: "outcome_unknown"}
			view.Recoverable = session.ActiveRunID != ""
		} else {
			view.CurrentRun = projectCreationRun(run, service.coordinator.Live(runID))
		}
	}
	return view, nil
}

// projectCreationRun 从持久化步骤投影公开阶段，不使用工作流内部节点名猜测状态。
func projectCreationRun(run agentdomain.Run, live bool) RunView {
	view := RunView{RunID: run.ID, Status: string(run.Status), Cancellable: live, PublicStage: string(agentdomain.PublicStageContextPreparing), Steps: []RunProgressStep{}}
	if !run.CreatedAt.IsZero() {
		view.CreatedAt = run.CreatedAt.Format(time.RFC3339Nano)
	}
	if !run.UpdatedAt.IsZero() {
		view.UpdatedAt = run.UpdatedAt.Format(time.RFC3339Nano)
	}
	if run.CompletedAt != nil {
		view.UpdatedAt = run.CompletedAt.Format(time.RFC3339Nano)
	}
	// 同一节点只投影最新尝试，历史失败不能遮挡正在重试的步骤。
	positions := map[string]int{}
	latest := []agentdomain.RunStep{}
	for index, step := range run.Steps {
		key := step.NodeID
		if key == "" {
			key = step.ID
		}
		if key == "" {
			key = strconv.Itoa(index)
		}
		if position, ok := positions[key]; ok {
			if step.Attempt >= latest[position].Attempt {
				latest[position] = step
			}
			continue
		}
		positions[key] = len(latest)
		latest = append(latest, step)
	}
	running := ""
	for _, step := range latest {
		stage := string(step.PublicStageID)
		view.Steps = append(view.Steps, RunProgressStep{Stage: stage, Status: string(step.Status)})
		if stage != "" && (step.Status == agentdomain.StepCompleted || step.Status == agentdomain.StepFailed) {
			view.PublicStage = stage
		}
		if stage != "" && step.Status == agentdomain.StepRunning && running == "" {
			running = stage
		}
	}
	if running != "" {
		view.PublicStage = running
	}
	if run.Status == agentdomain.RunCompleted {
		view.PublicStage = string(agentdomain.PublicStageCandidateReady)
	}
	return view
}

func assembleWorkbench(
	session creationdomain.Session,
	candidates []creationdomain.Candidate,
	impact ImpactProjection,
	pending int,
	review creationdomain.CandidateReview,
	cursor creationdomain.ReviewCursor,
) WorkbenchView {
	view := WorkbenchView{
		Applicability:  "active",
		ContentVersion: session.ContentVersion, RunSlotVersion: session.RunSlotVersion, Entry: EntryView{Kind: session.EntryKind()},
		SessionID:         session.ID,
		ProjectID:         session.ProjectID,
		BranchID:          session.BranchID,
		Stage:             session.Stage,
		AdvanceMode:       session.AdvanceMode,
		BlockingReason:    session.BlockingReason,
		Recoverable:       session.BlockingReason == creationdomain.BlockCostAuthorization && strings.TrimSpace(session.ActiveRunID) != "",
		ArchiveConfirmed:  strings.TrimSpace(session.ArchiveChangeSetID) != "",
		IntentText:        session.IntentText,
		StageRail:         buildStageRail(session),
		Impact:            impact,
		Assistant:         AssistantSummaryView{PendingActionCount: pending, Messages: []AssistantMessageView{}},
		DirectionSet:      DirectionSetView{Cards: []DirectionCardView{}, KeptIDs: append([]string{}, session.KeptDirectionIDs...), Combined: []CombinedDirectionView{}},
		Proposal:          ProposalView{Sections: []ProposalSectionView{}},
		PeopleWorld:       PeopleWorldView{Entities: []PeopleWorldNodeView{}},
		Roadmap:           RoadmapView{Volumes: []RoadmapNodeView{}, Arcs: []RoadmapNodeView{}, Chapters: []RoadmapNodeView{}},
		FieldEditor:       FieldEditorView{Fields: []TextFieldView{}},
		Revisions:         []RevisionView{},
		Review:            projectReview(review, reviewRefsOfStage(candidates, session.Stage)),
		ReviewCursor:      projectReviewCursor(cursor),
		CandidateVersions: projectVersionSummaries(candidates, session.Stage),
	}
	if current, ok := currentCandidateOfKind(candidates, creationdomain.StageDirections, creationdomain.KindDirectionSet); ok {
		view.DirectionSet = projectDirectionSet(current, session.SelectedDirectionCardID)
		view.DirectionSet.SelectedCandidateID = session.SelectedDirectionCandidateID
		view.DirectionSet.KeptIDs = append([]string{}, session.KeptDirectionIDs...)
	}
	for _, candidate := range candidates {
		if candidate.Stage != creationdomain.StageDirections || candidate.Kind != creationdomain.KindDirection || candidate.Status == creationdomain.StatusDiscarded {
			continue
		}
		var card agentdomain.DirectionCard
		if json.Unmarshal([]byte(candidate.PayloadJSON), &card) != nil {
			continue
		}
		view.DirectionSet.Combined = append(view.DirectionSet.Combined, CombinedDirectionView{
			CandidateID: candidate.ID, Version: candidate.Version,
			Card: DirectionCardView{ID: card.ID, Hook: card.Hook, Genre: card.Genre, Protagonist: card.Protagonist, CoreConflict: card.CoreConflict, GrowthCost: card.GrowthCost, Pacing: card.Pacing, Ending: card.Ending, Selected: candidate.ID == session.SelectedDirectionCandidateID && card.ID == session.SelectedDirectionCardID},
		})
	}
	sort.Slice(view.DirectionSet.Combined, func(i, j int) bool {
		return view.DirectionSet.Combined[i].Version > view.DirectionSet.Combined[j].Version
	})
	if current, ok := currentCandidate(candidates, creationdomain.StageProposal); ok {
		view.Proposal = projectProposal(current, review)
	}
	if current, ok := currentCandidate(candidates, creationdomain.StagePeopleWorld); ok {
		view.PeopleWorld = projectPeopleWorld(current, review)
	}
	if current, ok := currentCandidate(candidates, creationdomain.StageRoadmap); ok {
		view.Roadmap = projectRoadmap(current, review)
	}
	if session.Entry != nil {
		view.Entry = EntryView{ID: session.Entry.ID, Version: session.Entry.Version, Kind: session.Entry.Kind, Material: session.Entry.Material}
	}
	view.Material = projectMaterial(session, candidates)
	view.Capabilities = workbenchCapabilities(session, view)
	return view
}

func buildStageRail(session creationdomain.Session) []RailStopView {
	labels := map[creationdomain.Stage]string{
		creationdomain.StageIntent:         "灵感",
		creationdomain.StageDirections:     "故事方向",
		creationdomain.StageProposal:       "作品方案",
		creationdomain.StagePeopleWorld:    "人物世界",
		creationdomain.StageRoadmap:        "全书路线图",
		creationdomain.StageArchiveConfirm: "确认档案",
	}
	if session.EntryKind() == creationdomain.EntryAuthorPlan {
		labels[creationdomain.StageIntent] = "已有方案"
	}
	current := stageIndex(session.Stage)
	rail := make([]RailStopView, 0, len(creationdomain.Rail()))
	for _, stage := range creationdomain.Rail() {
		status := "upcoming"
		switch {
		case stageIndex(stage) < current:
			status = "confirmed"
		case stage == session.Stage:
			if session.BlockingReason != creationdomain.BlockNone {
				status = "blocked"
			} else {
				status = "current"
			}
		}
		if stage == creationdomain.StageDirections && session.EntryKind() == creationdomain.EntryAuthorPlan {
			status = "not_applicable"
		}
		rail = append(rail, RailStopView{ID: stage, Label: labels[stage], Status: status})
	}
	return rail
}

// applyReviewFlags 把逐项审阅决定投影为界面可见的接受/排除标记。
func applyReviewFlags(accepted, rejected *bool, review creationdomain.CandidateReview, scope creationdomain.ReviewScope, id string) {
	ref, err := creationdomain.NewReviewRef(scope, id)
	if err != nil {
		return
	}
	switch review.State(ref) {
	case creationdomain.DecisionAccept:
		*accepted = true
	case creationdomain.DecisionReject:
		*rejected = true
	}
}

func projectDirectionSet(candidate creationdomain.Candidate, selectedID string) DirectionSetView {
	view := DirectionSetView{
		CandidateID: candidate.ID, SelectedCandidateID: candidate.ID, Version: candidate.Version, SelectedID: selectedID,
		Status: string(candidate.Status), Cards: []DirectionCardView{}, Combined: []CombinedDirectionView{},
	}
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryDirections, []byte(candidate.PayloadJSON))
	if err != nil {
		return view
	}
	output := decoded.(*agentdomain.StoryDirectionsOutput)
	for _, card := range output.Directions {
		view.Cards = append(view.Cards, DirectionCardView{
			ID: card.ID, Hook: card.Hook, Genre: card.Genre, Protagonist: card.Protagonist,
			CoreConflict: card.CoreConflict, GrowthCost: card.GrowthCost, Pacing: card.Pacing, Ending: card.Ending,
			Selected: selectedID == card.ID,
		})
	}
	return view
}

func projectProposal(candidate creationdomain.Candidate, review creationdomain.CandidateReview) ProposalView {
	view := ProposalView{
		CandidateID: candidate.ID, Version: candidate.Version, Status: string(candidate.Status),
		Sections: []ProposalSectionView{},
	}
	blueprint, err := decodeBlueprint(candidate.PayloadJSON)
	if err != nil {
		return view
	}
	view.Title = blueprint.Title
	sections := []ProposalSectionView{
		{ID: "position", Title: "故事定位", Content: joinLines(blueprint.Positioning, blueprint.SellPoint, blueprint.Genre), LockPath: "/positioning"},
		{ID: "character", Title: "人物弧", Content: blueprint.ProtagonistArc, LockPath: "/protagonist_arc"},
		{ID: "world", Title: "世界规则", Content: strings.Join(blueprint.WorldRules, "\n"), LockPath: ""},
		{ID: "mainplot", Title: "主线", Content: joinLines(blueprint.CoreConflict, strings.Join(blueprint.MajorMilestones, " → ")), LockPath: "/core_conflict"},
		{ID: "subplot", Title: "支线", Content: strings.Join(blueprint.Subplots, "\n"), LockPath: ""},
		{ID: "ending", Title: "结局", Content: blueprint.Ending, LockPath: "/ending"},
		{ID: "forbidden", Title: "禁写项", Content: strings.Join(blueprint.Forbidden, "\n"), LockPath: ""},
	}
	for i := range sections {
		if sections[i].LockPath != "" {
			sections[i].Locked = candidate.Locked(sections[i].LockPath)
		}
		applyReviewFlags(&sections[i].Accepted, &sections[i].Rejected, review, creationdomain.ReviewScopeSection, sections[i].ID)
	}
	view.Sections = sections
	return view
}

func projectPeopleWorld(candidate creationdomain.Candidate, review creationdomain.CandidateReview) PeopleWorldView {
	view := PeopleWorldView{
		CandidateID: candidate.ID, Version: candidate.Version, Status: string(candidate.Status),
		Entities: []PeopleWorldNodeView{},
	}
	cast, err := decodeCast(candidate.PayloadJSON)
	if err != nil {
		return view
	}
	for _, item := range cast.Characters {
		entityID := strings.TrimSpace(item.ID)
		if entityID == "" {
			entityID = item.Name
		}
		node := PeopleWorldNodeView{
			ID: "character:" + entityID, Kind: "character", Title: item.Name, Summary: item.Summary,
			Complete: strings.TrimSpace(item.Summary) != "",
		}
		applyReviewFlags(&node.Accepted, &node.Rejected, review, creationdomain.ReviewScopeEntity, node.ID)
		view.Entities = append(view.Entities, node)
	}
	for _, item := range cast.Factions {
		node := PeopleWorldNodeView{
			ID: "faction:" + item.Name, Kind: "faction", Title: item.Name, Summary: item.Summary, Complete: true,
		}
		applyReviewFlags(&node.Accepted, &node.Rejected, review, creationdomain.ReviewScopeEntity, node.ID)
		view.Entities = append(view.Entities, node)
	}
	for _, item := range cast.Locations {
		node := PeopleWorldNodeView{
			ID: "location:" + item.Name, Kind: "location", Title: item.Name, Summary: item.Summary, Complete: true,
		}
		applyReviewFlags(&node.Accepted, &node.Rejected, review, creationdomain.ReviewScopeEntity, node.ID)
		view.Entities = append(view.Entities, node)
	}
	for _, item := range cast.WorldRules {
		node := PeopleWorldNodeView{
			ID: "rule:" + item.Name, Kind: "world_rule", Title: item.Name, Summary: item.Description, Complete: true,
		}
		applyReviewFlags(&node.Accepted, &node.Rejected, review, creationdomain.ReviewScopeEntity, node.ID)
		view.Entities = append(view.Entities, node)
	}
	return view
}

func projectRoadmap(candidate creationdomain.Candidate, review creationdomain.CandidateReview) RoadmapView {
	view := RoadmapView{
		CandidateID: candidate.ID, Version: candidate.Version, Status: string(candidate.Status),
		Volumes: []RoadmapNodeView{}, Arcs: []RoadmapNodeView{}, Chapters: []RoadmapNodeView{},
	}
	road, err := decodeRoadmap(candidate.PayloadJSON)
	if err != nil {
		return view
	}
	view.Book = RoadmapNodeView{ID: "book:" + road.Book.ID, Title: road.Book.Title, Summary: road.Book.Proposition}
	applyReviewFlags(&view.Book.Accepted, &view.Book.Rejected, review, creationdomain.ReviewScopeEntity, view.Book.ID)
	for _, volume := range road.Volumes {
		node := RoadmapNodeView{
			ID: "volume:" + volume.ID, ParentID: "book:" + road.Book.ID, Title: volume.Title, Summary: volume.Goal, Position: volume.Position,
		}
		applyReviewFlags(&node.Accepted, &node.Rejected, review, creationdomain.ReviewScopeEntity, node.ID)
		view.Volumes = append(view.Volumes, node)
	}
	for _, arc := range road.Arcs {
		commitments := make([]RoadmapGoalItemView, 0, len(arc.Commitments))
		for _, item := range arc.Commitments {
			commitments = append(commitments, RoadmapGoalItemView{ID: item.ID, Statement: item.Statement, Detail: item.FulfillmentRange})
		}
		questions := make([]RoadmapGoalItemView, 0, len(arc.RetainedQuestions))
		for _, item := range arc.RetainedQuestions {
			questions = append(questions, RoadmapGoalItemView{ID: item.ID, Statement: item.Statement, Detail: item.Reason})
		}
		arcNode := RoadmapNodeView{
			ID: "arc:" + strings.TrimSpace(arc.ID), ParentID: "volume:" + arc.VolumeRef, Title: arc.Title, Summary: arc.CausalChain, Position: arc.Position,
			ReaderExpectation: arc.ReaderExpectation, StagePayoff: arc.StagePayoff,
			IrreversibleChange: arc.IrreversibleChange, EndingContribution: arc.EndingContribution,
			Commitments: commitments, RetainedQuestions: questions,
		}
		applyReviewFlags(&arcNode.Accepted, &arcNode.Rejected, review, creationdomain.ReviewScopeEntity, arcNode.ID)
		view.Arcs = append(view.Arcs, arcNode)
	}
	for _, chapter := range road.NearTermChapters {
		node := RoadmapNodeView{
			ID: "chapter:" + chapter.ID, ParentID: "arc:" + chapter.ArcRef, Title: chapter.Title, Summary: chapter.Goal, Position: chapter.Position,
		}
		applyReviewFlags(&node.Accepted, &node.Rejected, review, creationdomain.ReviewScopeEntity, node.ID)
		view.Chapters = append(view.Chapters, node)
	}
	return view
}

func workbenchCapabilities(session creationdomain.Session, view WorkbenchView) []string {
	caps := []string{"SaveIntent", "SetAdvanceMode", "SaveCreationEntry"}
	if strings.TrimSpace(session.ActiveRunID) != "" {
		return append(caps, "CancelCreationRun")
	}
	switch session.Stage {
	case creationdomain.StageIntent, creationdomain.StageDirections:
		if session.EntryKind() == creationdomain.EntryAuthorPlan {
			return append(caps, "PreviewGenerateStage", "ConfirmGenerateStage", "AdvanceStage")
		}
		caps = append(caps, "PreviewGenerateDirections", "ConfirmGenerateDirections")
		if len(view.DirectionSet.Cards) > 0 {
			caps = append(caps, "SelectDirection", "SaveCandidateDraft", "RegenerateDirection")
			if len(view.DirectionSet.KeptIDs) >= 2 {
				caps = append(caps, "CombineDirections")
			}
		}
	case creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap:
		caps = append(caps, "PreviewGenerateStage", "ConfirmGenerateStage", "SaveCandidateDraft", "AcceptCandidate", "AdvanceStage",
			"SetReview", "AcceptAllReview", "UseCandidateVersion", "CompareCandidateVersions", "SaveReviewCursor")
		if session.EntryKind() == creationdomain.EntryAuthorPlan {
			caps = append(caps, "UseCreationMaterial")
		}
		if view.Proposal.CandidateID != "" {
			caps = append(caps, "LockField", "UnlockField")
		}
	case creationdomain.StageArchiveConfirm:
		caps = append(caps, "PreviewConfirmArchive", "ConfirmArchive")
	}
	if session.Stage != creationdomain.StageIntent {
		caps = append(caps, "LookBackStage")
	}
	if len(view.Impact.Items) > 0 {
		caps = append(caps, "ResolveImpact")
	}
	return caps
}

func joinLines(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kept = append(kept, part)
	}
	return strings.Join(kept, "\n")
}

func stageIndex(stage creationdomain.Stage) int {
	for i, item := range creationdomain.Rail() {
		if item == stage {
			return i
		}
	}
	return -1
}
