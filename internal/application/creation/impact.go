package creation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type ImpactItem struct {
	CandidateID string                        `json:"candidateId"`
	Stage       creationdomain.Stage          `json:"stage"`
	Severity    creationdomain.ImpactSeverity `json:"severity"`
	Reason      string                        `json:"reason"`
	Locked      bool                          `json:"locked"`
	Confirmed   bool                          `json:"confirmed"`
	Excepted    bool                          `json:"excepted"`
}

type ImpactProjection struct {
	Items            []ImpactItem           `json:"items"`
	TouchesLocked    bool                   `json:"touchesLocked"`
	TouchesConfirmed bool                   `json:"touchesConfirmed"`
	CanAdvance       bool                   `json:"canAdvance"`
	RegenerateStages []creationdomain.Stage `json:"regenerateStages"`
	EstimatedCalls   int                    `json:"estimatedCalls"`
	Evidence         string                 `json:"evidence"`
}

type ResolveImpactRequest struct {
	ProjectID string                      `json:"projectId"`
	Choice    creationdomain.ImpactChoice `json:"choice"`
}

type ResolveImpactResult struct {
	Session          creationdomain.Session `json:"session"`
	Projection       ImpactProjection       `json:"projection"`
	RegenerateStages []creationdomain.Stage `json:"regenerateStages"`
}

// SaveCandidateDraft 把作者直接编辑存为新候选版本，并重算下游 stale，不调用模型。
func (service *Service) SaveCandidateDraft(ctx context.Context, projectID, candidateID, payloadJSON string) (creationdomain.Candidate, error) {
	session, err := service.store.Session(ctx, projectID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	parent, err := service.store.Candidate(ctx, candidateID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if parent.SessionID != session.ID {
		return creationdomain.Candidate{}, ErrSessionNotFound
	}
	if err := validateCandidatePayload(parent.Stage, payloadJSON); err != nil {
		return creationdomain.Candidate{}, err
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	draft, err := parent.SaveDraft(service.ids.NewID(), payloadJSON, service.clock.Now())
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	draft = draft.WithUpstream(liveUpstreamDigest(session, candidates, parent.Stage))
	superseded, err := parent.MarkSuperseded(service.clock.Now())
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if err := service.store.UpdateCandidate(ctx, superseded); err != nil {
		return creationdomain.Candidate{}, err
	}
	if err := service.store.SaveCandidate(ctx, draft); err != nil {
		return creationdomain.Candidate{}, err
	}
	if err := service.refreshImpact(ctx, session); err != nil {
		return creationdomain.Candidate{}, err
	}
	return draft, nil
}

// ProjectImpact 计算当前会话的下游过期范围，不创建运行、不改写正式资产。
func (service *Service) ProjectImpact(ctx context.Context, projectID string) (ImpactProjection, error) {
	session, err := service.store.Session(ctx, projectID)
	if err != nil {
		return ImpactProjection{}, err
	}
	if err := service.refreshImpact(ctx, session); err != nil {
		return ImpactProjection{}, err
	}
	return service.projectImpact(ctx, session)
}

// ResolveImpact 执行作者对 stale 的四选一，局部重算只返回待重跑阶段，不开付费运行。
func (service *Service) ResolveImpact(ctx context.Context, request ResolveImpactRequest) (ResolveImpactResult, error) {
	if !request.Choice.Valid() {
		return ResolveImpactResult{}, creationdomain.ErrBlankValue
	}
	session, err := service.store.Session(ctx, request.ProjectID)
	if err != nil {
		return ResolveImpactResult{}, err
	}
	if err := service.refreshImpact(ctx, session); err != nil {
		return ResolveImpactResult{}, err
	}
	switch request.Choice {
	case creationdomain.ChoiceContinueTalk:
	case creationdomain.ChoiceKeepException:
		if err := service.keepImpactExceptions(ctx, session); err != nil {
			return ResolveImpactResult{}, err
		}
	case creationdomain.ChoiceAbandonUpstream:
		if err := service.abandonUpstream(ctx, session); err != nil {
			return ResolveImpactResult{}, err
		}
		if err := service.refreshImpact(ctx, session); err != nil {
			return ResolveImpactResult{}, err
		}
	case creationdomain.ChoiceRegenerateLocal:
	}
	projection, err := service.projectImpact(ctx, session)
	if err != nil {
		return ResolveImpactResult{}, err
	}
	result := ResolveImpactResult{Session: session, Projection: projection}
	if request.Choice == creationdomain.ChoiceRegenerateLocal {
		result.RegenerateStages = projection.RegenerateStages
	}
	return result, nil
}

func (service *Service) refreshImpact(ctx context.Context, session creationdomain.Session) error {
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return err
	}
	now := service.clock.Now()
	index := map[string]int{}
	for i := range candidates {
		index[candidates[i].ID] = i
	}
	replace := func(updated creationdomain.Candidate) {
		if i, ok := index[updated.ID]; ok {
			candidates[i] = updated
		}
	}
	for _, stage := range generateStages() {
		current, ok := currentCandidate(candidates, stage)
		if !ok {
			continue
		}
		live := liveUpstreamDigest(session, candidates, stage)
		if strings.TrimSpace(live) == "" || strings.TrimSpace(current.UpstreamDigest) == "" {
			continue
		}
		if current.UpstreamDigest == live {
			if current.Status == creationdomain.StatusStale {
				cleared, clearErr := current.ClearStale(now)
				if clearErr != nil {
					return fmt.Errorf("clear creation stale: %w", clearErr)
				}
				if err := service.store.UpdateCandidate(ctx, cleared); err != nil {
					return err
				}
				replace(cleared)
			}
			continue
		}
		if current.ExceptionMatches(live) {
			continue
		}
		severity := creationdomain.StaleSeverity(current)
		refs := impactRefsFor(candidates, stage)
		reason := staleReasonFor(stage)
		if current.Status == creationdomain.StatusStale && current.StaleReason == reason &&
			current.ImpactSeverity == severity && sameStrings(current.ImpactRefs, refs) {
			continue
		}
		marked, markErr := current.MarkStale(reason, refs, severity, now)
		if markErr != nil {
			return fmt.Errorf("mark creation stale: %w", markErr)
		}
		if err := service.store.UpdateCandidate(ctx, marked); err != nil {
			return err
		}
		replace(marked)
	}
	return nil
}

func (service *Service) keepImpactExceptions(ctx context.Context, session creationdomain.Session) error {
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return err
	}
	now := service.clock.Now()
	for _, stage := range generateStages() {
		current, ok := currentCandidate(candidates, stage)
		if !ok || current.Status != creationdomain.StatusStale {
			continue
		}
		live := liveUpstreamDigest(session, candidates, stage)
		if current.ExceptionMatches(live) {
			continue
		}
		kept, err := current.KeepException(live, now)
		if err != nil {
			return err
		}
		if err := service.store.UpdateCandidate(ctx, kept); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) abandonUpstream(ctx context.Context, session creationdomain.Session) error {
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return err
	}
	now := service.clock.Now()
	seen := map[string]struct{}{}
	for _, stage := range generateStages() {
		current, ok := currentCandidate(candidates, stage)
		if !ok || current.Status != creationdomain.StatusStale {
			continue
		}
		live := liveUpstreamDigest(session, candidates, stage)
		if current.ExceptionMatches(live) {
			continue
		}
		for _, ref := range current.ImpactRefs {
			if ref == "" || ref == "intent" {
				continue
			}
			seen[ref] = struct{}{}
		}
	}
	for id := range seen {
		upstream, err := service.store.Candidate(ctx, id)
		if err != nil {
			return err
		}
		if upstream.Status == creationdomain.StatusDiscarded {
			continue
		}
		discarded, err := upstream.Discard(now)
		if err != nil {
			return err
		}
		if err := service.store.UpdateCandidate(ctx, discarded); err != nil {
			return err
		}
		if strings.TrimSpace(upstream.SupersedesCandidateID) == "" {
			continue
		}
		previous, err := service.store.Candidate(ctx, upstream.SupersedesCandidateID)
		if err != nil {
			return err
		}
		restored, err := previous.Restore(now)
		if err != nil {
			return err
		}
		if err := service.store.UpdateCandidate(ctx, restored); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) projectImpact(ctx context.Context, session creationdomain.Session) (ImpactProjection, error) {
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return ImpactProjection{}, err
	}
	projection := ImpactProjection{Items: []ImpactItem{}, RegenerateStages: []creationdomain.Stage{}}
	reasons := make([]string, 0)
	seenStage := map[creationdomain.Stage]bool{}
	for _, stage := range generateStages() {
		current, ok := currentCandidate(candidates, stage)
		if !ok || current.Status != creationdomain.StatusStale {
			continue
		}
		live := liveUpstreamDigest(session, candidates, stage)
		item := ImpactItem{
			CandidateID: current.ID,
			Stage:       current.Stage,
			Severity:    current.ImpactSeverity,
			Reason:      current.StaleReason,
			Locked:      current.HasLocks(),
			Confirmed:   current.ImpactSeverity == creationdomain.ImpactMust && !current.HasLocks(),
			Excepted:    current.ExceptionMatches(live),
		}
		projection.Items = append(projection.Items, item)
		if item.Locked {
			projection.TouchesLocked = true
		}
		if item.Confirmed {
			projection.TouchesConfirmed = true
		}
		if !item.Excepted {
			if current.StaleReason != "" {
				reasons = append(reasons, current.StaleReason)
			}
			if !seenStage[current.Stage] {
				projection.RegenerateStages = append(projection.RegenerateStages, current.Stage)
				seenStage[current.Stage] = true
			}
		}
	}
	projection.Evidence = strings.Join(reasons, "；")
	projection.EstimatedCalls = service.estimateRegenerateCalls(ctx, projection.RegenerateStages)
	projection.CanAdvance = stageReady(session, candidates) == nil
	return projection, nil
}

func (service *Service) estimateRegenerateCalls(ctx context.Context, stages []creationdomain.Stage) int {
	total := 0
	for _, stage := range stages {
		binding, err := bindingFor(stage)
		if err != nil {
			continue
		}
		compiled, err := agentkernel.Compile(ctx, agentkernel.Registries{
			Workflows: service.registry, Prompts: service.registry, Schemas: service.registry, Assemblers: service.registry,
		}, binding.WorkflowID, binding.Version)
		if err != nil {
			continue
		}
		total += len(compiled.Nodes)
	}
	return total
}

// liveUpstreamDigest 计算某阶段当前实际输入的级联摘要：任一上游输入变化都会改变下游摘要。
func liveUpstreamDigest(session creationdomain.Session, candidates []creationdomain.Candidate, stage creationdomain.Stage) string {
	switch stage {
	case creationdomain.StageDirections:
		if strings.TrimSpace(session.IntentText) == "" {
			return ""
		}
		return hashParts("intent", strings.TrimSpace(session.IntentText))
	case creationdomain.StageProposal:
		if strings.TrimSpace(session.IntentText) == "" {
			return ""
		}
		card, _, err := selectedDirectionCard(session, candidates)
		if err != nil {
			return ""
		}
		encoded, err := json.Marshal(card)
		if err != nil {
			return ""
		}
		return hashParts("intent", strings.TrimSpace(session.IntentText), "direction", string(encoded))
	case creationdomain.StagePeopleWorld:
		base := liveUpstreamDigest(session, candidates, creationdomain.StageProposal)
		if base == "" {
			return ""
		}
		current, ok := currentCandidate(candidates, creationdomain.StageProposal)
		if !ok {
			return ""
		}
		return hashParts(base, "payload", current.ContentSHA256)
	case creationdomain.StageRoadmap:
		base := liveUpstreamDigest(session, candidates, creationdomain.StagePeopleWorld)
		if base == "" {
			return ""
		}
		current, ok := currentCandidate(candidates, creationdomain.StagePeopleWorld)
		if !ok {
			return ""
		}
		return hashParts(base, "payload", current.ContentSHA256)
	}
	return ""
}

func impactRefsFor(candidates []creationdomain.Candidate, stage creationdomain.Stage) []string {
	var upstream creationdomain.Stage
	switch stage {
	case creationdomain.StageDirections:
		return []string{"intent"}
	case creationdomain.StageProposal:
		upstream = creationdomain.StageDirections
	case creationdomain.StagePeopleWorld:
		upstream = creationdomain.StageProposal
	case creationdomain.StageRoadmap:
		upstream = creationdomain.StagePeopleWorld
	default:
		return []string{}
	}
	if current, ok := currentCandidate(candidates, upstream); ok {
		return []string{current.ID}
	}
	return []string{}
}

func staleReasonFor(stage creationdomain.Stage) string {
	switch stage {
	case creationdomain.StageDirections:
		return "创作意图已改"
	case creationdomain.StageProposal:
		return "故事方向已改"
	case creationdomain.StagePeopleWorld:
		return "作品方案已改"
	case creationdomain.StageRoadmap:
		return "人物世界已改"
	default:
		return "上游内容已改"
	}
}

func unresolvedMustFix(session creationdomain.Session, candidates []creationdomain.Candidate) bool {
	for _, stage := range generateStages() {
		current, ok := currentCandidate(candidates, stage)
		if !ok || current.Status != creationdomain.StatusStale || current.ImpactSeverity != creationdomain.ImpactMust {
			continue
		}
		if current.ExceptionMatches(liveUpstreamDigest(session, candidates, stage)) {
			continue
		}
		return true
	}
	return false
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

// currentCandidate 返回某阶段最新且未被替代/放弃的候选。
func currentCandidate(candidates []creationdomain.Candidate, stage creationdomain.Stage) (creationdomain.Candidate, bool) {
	var current creationdomain.Candidate
	found := false
	for _, candidate := range candidates {
		if candidate.Stage != stage || candidate.Status == creationdomain.StatusSuperseded || candidate.Status == creationdomain.StatusDiscarded {
			continue
		}
		if !found || candidate.Version > current.Version {
			current = candidate
			found = true
		}
	}
	return current, found
}

func candidateByID(candidates []creationdomain.Candidate, id string) (creationdomain.Candidate, bool) {
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return creationdomain.Candidate{}, false
}

// stageReady 判断当前阶段是否满足推进条件：候选完整、质量未阻断、无未解决 stale。
func stageReady(session creationdomain.Session, candidates []creationdomain.Candidate) error {
	switch session.Stage {
	case creationdomain.StageIntent:
		if strings.TrimSpace(session.IntentText) == "" {
			return creationdomain.ErrStageIncomplete
		}
	case creationdomain.StageDirections:
		if strings.TrimSpace(session.SelectedDirectionCandidateID) == "" ||
			strings.TrimSpace(session.SelectedDirectionCardID) == "" {
			return creationdomain.ErrStageIncomplete
		}
	case creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap:
		current, ok := currentCandidate(candidates, session.Stage)
		if !ok {
			return creationdomain.ErrStageIncomplete
		}
		if current.Blocked() {
			return creationdomain.ErrQualityBlocked
		}
		if err := validateCandidatePayload(session.Stage, current.PayloadJSON); err != nil {
			return err
		}
	}
	if current, ok := currentCandidate(candidates, session.Stage); ok {
		live := liveUpstreamDigest(session, candidates, session.Stage)
		if current.Status == creationdomain.StatusStale && !current.ExceptionMatches(live) {
			return creationdomain.ErrImpactUnresolved
		}
	}
	return nil
}

func validateCandidatePayload(stage creationdomain.Stage, payload string) error {
	kind := agentdomain.TaskStoryBlueprint
	switch stage {
	case creationdomain.StageDirections:
		kind = agentdomain.TaskStoryDirections
	case creationdomain.StagePeopleWorld:
		kind = agentdomain.TaskStoryCast
	case creationdomain.StageRoadmap:
		kind = agentdomain.TaskStoryRoadmap
	}
	decoded, err := agentdomain.DecodeTaskOutput(kind, []byte(payload))
	if err != nil {
		return err
	}
	if stage == creationdomain.StageProposal {
		return decoded.(*agentdomain.StoryBlueprintOutput).ValidateProduct()
	}
	return nil
}
