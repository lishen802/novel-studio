package creation

import (
	"fmt"

	"github.com/lishen802/novel-studio/internal/agentassets"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type stageBinding struct {
	WorkflowID    string
	Version       int
	Kind          creationdomain.CandidateKind
	SchemaID      string
	SchemaVersion int
}

func bindingFor(stage creationdomain.Stage) (stageBinding, error) {
	switch stage {
	case creationdomain.StageDirections:
		return stageBinding{
			WorkflowID: agentassets.StoryDirectionsWorkflowID, Version: agentassets.StoryProductWorkflowVersion,
			Kind: creationdomain.KindDirectionSet, SchemaID: "story_directions.schema", SchemaVersion: 1,
		}, nil
	case creationdomain.StageProposal:
		return stageBinding{
			WorkflowID: agentassets.StoryProposalWorkflowID, Version: agentassets.StoryProductWorkflowVersion,
			Kind: creationdomain.KindProposal, SchemaID: "story_blueprint.schema", SchemaVersion: 2,
		}, nil
	case creationdomain.StagePeopleWorld:
		return stageBinding{
			WorkflowID: agentassets.StoryPeopleWorldWorkflowID, Version: agentassets.StoryProductWorkflowVersion,
			Kind: creationdomain.KindPeopleWorld, SchemaID: "story_cast.schema", SchemaVersion: 1,
		}, nil
	case creationdomain.StageRoadmap:
		return stageBinding{
			WorkflowID: agentassets.StoryRoadmapWorkflowID, Version: agentassets.StoryProductWorkflowVersion,
			Kind: creationdomain.KindRoadmap, SchemaID: "story_roadmap.schema", SchemaVersion: 1,
		}, nil
	default:
		return stageBinding{}, fmt.Errorf("%w: %s", ErrInvalidStage, stage)
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
