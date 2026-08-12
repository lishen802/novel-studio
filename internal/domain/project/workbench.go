package project

import "time"

type WorkbenchStage string

const (
	StageIdea         WorkbenchStage = "idea"
	StageProposal     WorkbenchStage = "proposal"
	StageRoadmap      WorkbenchStage = "roadmap"
	StageBatchRunning WorkbenchStage = "batch_running"
	StageReview       WorkbenchStage = "review"
	StageWriting      WorkbenchStage = "writing"
)

type WorkbenchProjection struct {
	Stage               WorkbenchStage `json:"stage"`
	Canvas              string         `json:"canvas"`
	AIPanel             string         `json:"aiPanel"`
	PendingReviewCount  int            `json:"pendingReviewCount"`
	RecoverableJobCount int            `json:"recoverableJobCount"`
	ArchiveStatus       string         `json:"archiveStatus"`
}

type ProfileSelection struct {
	ProfileID string
	Version   int64
}

type WorkbenchState struct {
	Stage               WorkbenchStage
	Canvas              string
	AIPanel             string
	PendingReviewCount  int
	RecoverableJobCount int
	ArchiveStatus       string
	UpdatedAt           time.Time
}
