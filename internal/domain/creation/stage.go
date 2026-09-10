package creation

import "strings"

var (
	ErrBlankValue                  = err("creation value must not be blank")
	ErrInvalidTransition           = err("invalid creation stage transition")
	ErrStageIncomplete             = err("creation stage is incomplete")
	ErrFieldLocked                 = err("creation field is locked")
	ErrInvalidJSON                 = err("creation payload is not valid json")
	ErrFieldMissing                = err("creation field path is missing")
	ErrImpactUnresolved            = err("creation impact must be resolved")
	ErrDirectionCard               = err("creation direction card not found in candidate")
	ErrQualityBlocked              = err("creation candidate quality is blocked")
	ErrCandidateAcceptanceRequired = err("creation candidate must be accepted before advancing or archiving")
	// ErrConfirmationStale 表示入档事务内发现确认前置条件在预览后已变化（改锁、改选、候选修改或质量变化）。
	ErrConfirmationStale = err("creation confirmation prerequisites changed")
)

// MaxKeptDirections 限制作者保留的方向卡数量，与初始三卡规模一致。
const MaxKeptDirections = 3

type errorString string

func (e errorString) Error() string { return string(e) }

func err(text string) errorString { return errorString(text) }

type Stage string

const (
	StageIntent         Stage = "intent"
	StageDirections     Stage = "directions"
	StageProposal       Stage = "proposal"
	StagePeopleWorld    Stage = "people_world"
	StageRoadmap        Stage = "roadmap"
	StageArchiveConfirm Stage = "archive_confirm"
)

type AdvanceMode string

const (
	ModeStepReview  AdvanceMode = "step_review"
	ModeAutoAdvance AdvanceMode = "auto_advance"
)

type BlockingReason string

const (
	BlockNone              BlockingReason = ""
	BlockCostAuthorization BlockingReason = "cost_authorization"
	BlockLockedFields      BlockingReason = "locked_fields"
	BlockConflict          BlockingReason = "conflict"
	BlockFormalWrite       BlockingReason = "formal_write"
	BlockPendingAction     BlockingReason = "pending_action"
	BlockQualityReview     BlockingReason = "quality_review"
)

// Rail 返回作者可见的原创构建阶段顺序。
func Rail() []Stage {
	return []Stage{StageIntent, StageDirections, StageProposal, StagePeopleWorld, StageRoadmap, StageArchiveConfirm}
}

func (stage Stage) index() int {
	for i, item := range Rail() {
		if item == stage {
			return i
		}
	}
	return -1
}

func (stage Stage) Valid() bool {
	return stage.index() >= 0
}

func NextStage(stage Stage) (Stage, bool) {
	index := stage.index()
	rail := Rail()
	if index < 0 || index+1 >= len(rail) {
		return "", false
	}
	return rail[index+1], true
}

// WorkbenchStage 把细粒度创作阶段投影到既有工作台粗阶段。
func (stage Stage) WorkbenchStage() string {
	switch stage {
	case StageIntent, StageDirections:
		return "idea"
	case StageProposal, StagePeopleWorld:
		return "proposal"
	case StageRoadmap, StageArchiveConfirm:
		return "roadmap"
	default:
		return ""
	}
}

// CanAdvance 只允许前进到轨道上的下一阶段。
func CanAdvance(from, to Stage) error {
	next, ok := NextStage(from)
	if !ok || next != to {
		return ErrInvalidTransition
	}
	return nil
}

// CanLookBack 允许回到已经到达过的更早阶段，不能借此前进。
func CanLookBack(current, target Stage) error {
	from := current.index()
	to := target.index()
	if from < 0 || to < 0 || to >= from {
		return ErrInvalidTransition
	}
	return nil
}

// NormalizeLegacyWorkbenchStage 把旧项目粗阶段映射为创作阶段。
func NormalizeLegacyWorkbenchStage(stage string) Stage {
	switch strings.TrimSpace(stage) {
	case "idea":
		return StageIntent
	case string(StageDirections):
		return StageDirections
	case "proposal":
		return StageProposal
	case string(StagePeopleWorld):
		return StagePeopleWorld
	case "roadmap":
		return StageRoadmap
	case string(StageArchiveConfirm):
		return StageArchiveConfirm
	default:
		return ""
	}
}
