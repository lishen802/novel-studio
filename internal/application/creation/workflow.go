package creation

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type stageBinding struct {
	WorkflowID    string
	Version       int
	Kind          creationdomain.CandidateKind
	SchemaID      string
	SchemaVersion int
	Stage         creationdomain.Stage
}

func bindingFor(stage creationdomain.Stage) (stageBinding, error) {
	switch stage {
	case creationdomain.StageIntent:
		return stageBinding{WorkflowID: "story.author_material", Version: 2, Kind: creationdomain.KindProposal, SchemaID: "author_material.schema", SchemaVersion: 2, Stage: stage}, nil
	case creationdomain.StageDirections:
		return stageBinding{
			WorkflowID: agentassets.StoryDirectionsWorkflowID, Version: agentassets.StoryProductWorkflowVersion,
			Kind: creationdomain.KindDirectionSet, SchemaID: "story_directions.schema", SchemaVersion: 1,
			Stage: stage,
		}, nil
	case creationdomain.StageProposal:
		return stageBinding{
			WorkflowID: agentassets.StoryProposalWorkflowID, Version: agentassets.StoryProductWorkflowVersion,
			Kind: creationdomain.KindProposal, SchemaID: "story_blueprint.schema", SchemaVersion: 2,
			Stage: stage,
		}, nil
	case creationdomain.StagePeopleWorld:
		return stageBinding{
			WorkflowID: agentassets.StoryPeopleWorldWorkflowID, Version: agentassets.StoryPeopleWorldWorkflowVersion,
			Kind: creationdomain.KindPeopleWorld, SchemaID: "story_cast.schema", SchemaVersion: 2,
			Stage: stage,
		}, nil
	case creationdomain.StageRoadmap:
		return stageBinding{
			WorkflowID: agentassets.StoryRoadmapWorkflowID, Version: agentassets.StoryRoadmapWorkflowVersion,
			Kind: creationdomain.KindRoadmap, SchemaID: "story_roadmap.schema", SchemaVersion: 3,
			Stage: stage,
		}, nil
	default:
		return stageBinding{}, fmt.Errorf("%w: %s", ErrInvalidStage, stage)
	}
}

// bindingByWorkflow 按当前 WorkflowBinding 反查阶段绑定，供恢复对账物化结果。
func bindingByWorkflow(workflowID string, version int) (stageBinding, error) {
	for _, stage := range []creationdomain.Stage{creationdomain.StageIntent, creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap} {
		binding, err := bindingForEntry(creationdomain.EntryAuthorPlan, stage)
		if err == nil && binding.WorkflowID == workflowID && binding.Version == version {
			return binding, nil
		}
	}
	for _, stage := range generateStages() {
		binding, err := bindingFor(stage)
		if err != nil {
			continue
		}
		if binding.WorkflowID == workflowID && binding.Version == version {
			return binding, nil
		}
	}
	if workflowID == agentassets.StoryCandidateDiscussionWorkflowID && version == agentassets.StoryCandidateDiscussionWorkflowVersion {
		return discussionBinding(), nil
	}
	return stageBinding{}, fmt.Errorf("%w: %s@%d", ErrInvalidStage, workflowID, version)
}

// authorReviseBinding 返回作者定向修订工作流绑定（单节点结构化输出）。
func authorReviseBinding() stageBinding {
	return stageBinding{
		WorkflowID: agentassets.StoryAuthorReviseWorkflowID, Version: agentassets.StoryAuthorReviseWorkflowVersion,
		Kind: creationdomain.KindProposal, SchemaID: "author_revise_blueprint.schema", SchemaVersion: 1,
		Stage: creationdomain.StageProposal,
	}
}

func discussionBinding() stageBinding {
	return stageBinding{
		WorkflowID: agentassets.StoryCandidateDiscussionWorkflowID, Version: agentassets.StoryCandidateDiscussionWorkflowVersion,
		SchemaID: "story_candidate_discussion.schema", SchemaVersion: 1,
	}
}

func generateStages() []creationdomain.Stage {
	return []creationdomain.Stage{
		creationdomain.StageDirections, creationdomain.StageProposal,
		creationdomain.StagePeopleWorld, creationdomain.StageRoadmap,
	}
}

func remainingGenerateStages(stage creationdomain.Stage) []creationdomain.Stage {
	remaining := make([]creationdomain.Stage, 0)
	seen := false
	for _, item := range generateStages() {
		if item == stage {
			seen = true
			continue
		}
		if seen {
			remaining = append(remaining, item)
		}
	}
	return remaining
}

// directionCardByID 从方向候选载荷中按稳定卡 ID 取一张卡；卡不存在返回领域错误。
func directionCardByID(candidate creationdomain.Candidate, cardID string) (agentdomain.DirectionCard, error) {
	decoded, err := agentdomain.DecodeTaskOutput(agentdomain.TaskStoryDirections, []byte(candidate.PayloadJSON))
	if err != nil {
		return agentdomain.DirectionCard{}, err
	}
	output := decoded.(*agentdomain.StoryDirectionsOutput)
	for _, card := range output.Directions {
		if strings.TrimSpace(card.ID) == strings.TrimSpace(cardID) {
			return card, nil
		}
	}
	return agentdomain.DirectionCard{}, creationdomain.ErrDirectionCard
}

// selectedDirectionCard 取作者实际选中的方向卡及其候选；选择不完整时返回错误。
// 计算工作输入时若被选候选已被新版本替代，则跟随当前候选中的同 ID 卡，
// 使上游编辑能正确传播为下游 stale；当前版本缺失该卡视为上游已变。
func selectedDirectionCard(
	session creationdomain.Session, candidates []creationdomain.Candidate,
) (agentdomain.DirectionCard, creationdomain.Candidate, error) {
	if strings.TrimSpace(session.SelectedDirectionCandidateID) == "" || strings.TrimSpace(session.SelectedDirectionCardID) == "" {
		return agentdomain.DirectionCard{}, creationdomain.Candidate{}, ErrDirectionRequired
	}
	candidate, ok := candidateByID(candidates, session.SelectedDirectionCandidateID)
	if !ok {
		return agentdomain.DirectionCard{}, creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	if candidate.Status == creationdomain.StatusSuperseded || candidate.Status == creationdomain.StatusDiscarded {
		current, found := currentCandidate(candidates, creationdomain.StageDirections)
		if !found {
			return agentdomain.DirectionCard{}, creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
		}
		candidate = current
	}
	card, err := directionCardByID(candidate, session.SelectedDirectionCardID)
	if err != nil {
		return agentdomain.DirectionCard{}, creationdomain.Candidate{}, err
	}
	return card, candidate, nil
}

// lockedConstraints 提取候选锁定字段的路径与当前值，作为生成输入中的作者约束。
func lockedConstraints(candidate creationdomain.Candidate) (string, bool) {
	if !candidate.HasLocks() {
		return "", false
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(candidate.PayloadJSON), &payload); err != nil {
		return "", false
	}
	constraints := make(map[string]any, len(candidate.LockMap))
	for path, locked := range candidate.LockMap {
		if !locked {
			continue
		}
		key := strings.TrimPrefix(path, "/")
		if value, ok := payload[key]; ok {
			constraints[key] = value
		}
	}
	if len(constraints) == 0 {
		return "", false
	}
	encoded, err := json.Marshal(constraints)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}

// generateSources 按阶段装配实际选中的上游输入：后续阶段必须收到选中方向卡、
// 作品方案与人物世界等完整上游（P1-3），锁定字段作为作者约束一并冻结。
func generateSources(session creationdomain.Session, candidates []creationdomain.Candidate, stage creationdomain.Stage) ([]agentkernel.Source, error) {
	sources := []agentkernel.Source{{
		Kind: agentdomain.ManifestAuthorInstruction, ID: "intent", Version: "1",
		Content: creationInstruction(session), Priority: 1000, SelectionReason: "作者创作意图", Required: true,
	}}
	if stage == creationdomain.StageDirections {
		return sources, nil
	}
	if session.EntryKind() == creationdomain.EntryAuthorPlan {
		if session.Entry == nil || session.Entry.Validate() != nil {
			return nil, creationdomain.ErrInvalidEntry
		}
		raw, _ := json.Marshal(session.Entry)
		sources = append(sources, agentkernel.Source{Kind: agentdomain.ManifestAuthorInstruction, ID: "author_material", Version: fmt.Sprint(session.Entry.Version), Content: string(raw), Priority: 1100, Required: true, SelectionReason: "作者原始方案及不可变来源"})
		if stage == creationdomain.StageIntent {
			return sources, nil
		}
	} else {
		card, cardCandidate, err := selectedDirectionCard(session, candidates)
		if err != nil {
			return nil, err
		}
		cardJSON, err := json.Marshal(card)
		if err != nil {
			return nil, err
		}
		sources = append(sources, agentkernel.Source{
			Kind: agentdomain.ManifestStoryAsset, ID: "direction:" + card.ID,
			Version: fmt.Sprintf("%d", cardCandidate.Version), Content: string(cardJSON),
			Priority: 900, SelectionReason: "作者选中的故事方向", Provisional: true, Required: true,
		})
	}
	if stage == creationdomain.StageProposal {
		return sources, nil
	}
	proposal, ok := currentCandidate(candidates, creationdomain.StageProposal)
	if !ok {
		return nil, creationdomain.ErrStageIncomplete
	}
	sources = append(sources, agentkernel.Source{
		Kind: agentdomain.ManifestStoryAsset, ID: "proposal", Version: fmt.Sprintf("%d", proposal.Version),
		Content: proposal.PayloadJSON, Priority: 800, SelectionReason: "作品方案", Provisional: true, Required: true,
	})
	if constraints, has := lockedConstraints(proposal); has {
		sources = append(sources, agentkernel.Source{
			Kind: agentdomain.ManifestAnchor, ID: "proposal-locks", Version: fmt.Sprintf("%d", proposal.Version),
			Content: constraints, Priority: 850, SelectionReason: "作者锁定的方案字段", Required: true,
		})
	}
	if stage == creationdomain.StagePeopleWorld {
		return sources, nil
	}
	people, ok := currentCandidate(candidates, creationdomain.StagePeopleWorld)
	if !ok {
		return nil, creationdomain.ErrStageIncomplete
	}
	sources = append(sources, agentkernel.Source{
		Kind: agentdomain.ManifestStoryAsset, ID: "people_world", Version: fmt.Sprintf("%d", people.Version),
		Content: people.PayloadJSON, Priority: 700, SelectionReason: "人物与世界", Provisional: true, Required: true,
	})
	if constraints, has := lockedConstraints(people); has {
		sources = append(sources, agentkernel.Source{
			Kind: agentdomain.ManifestAnchor, ID: "people-world-locks", Version: fmt.Sprintf("%d", people.Version),
			Content: constraints, Priority: 750, SelectionReason: "作者锁定的人物世界字段", Required: true,
		})
	}
	return sources, nil
}

// bindingForEntry 为已有方案选择当前专用工作流。
func bindingForEntry(kind creationdomain.EntryKind, stage creationdomain.Stage) (stageBinding, error) {
	binding, err := bindingFor(stage)
	if err != nil {
		return binding, err
	}
	if kind != creationdomain.EntryAuthorPlan {
		return binding, nil
	}
	switch stage {
	case creationdomain.StageProposal:
		binding.WorkflowID = "story.author_proposal"
		binding.Version = 1
	case creationdomain.StagePeopleWorld:
		binding.WorkflowID = "story.author_people_world"
		binding.Version = 1
	case creationdomain.StageRoadmap:
		binding.WorkflowID = "story.author_roadmap"
		binding.Version = agentassets.StoryAuthorRoadmapWorkflowVersion
	}
	return binding, nil
}
func creationInstruction(session creationdomain.Session) string {
	if session.EntryKind() == creationdomain.EntryAuthorPlan {
		return "整理并保留作者已有方案；仅补齐当前阶段必要缺口，所有新内容均待作者审阅。"
	}
	return session.IntentText
}
