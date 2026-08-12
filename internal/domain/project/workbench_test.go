package project

import (
	"testing"
	"time"
)

func TestWorkbenchStageConstants(t *testing.T) {
	stages := map[WorkbenchStage]string{
		StageIdea:         "idea",
		StageProposal:     "proposal",
		StageRoadmap:      "roadmap",
		StageBatchRunning: "batch_running",
		StageReview:       "review",
		StageWriting:      "writing",
	}
	for stage, want := range stages {
		if string(stage) != want {
			t.Fatalf("stage %q = %q, want %q", stage, string(stage), want)
		}
	}
}

func TestDefaultWorkbenchProjectionHasIdeaStageAndHiddenPanel(t *testing.T) {
	projection := WorkbenchProjection{
		Stage:         StageIdea,
		Canvas:        "",
		AIPanel:       "hidden",
		ArchiveStatus: "not_generated",
	}
	if projection.Stage != StageIdea {
		t.Fatalf("Stage = %q, want %q", projection.Stage, StageIdea)
	}
	if projection.Canvas != "" {
		t.Fatalf("Canvas = %q, want empty", projection.Canvas)
	}
	if projection.AIPanel != "hidden" {
		t.Fatalf("AIPanel = %q, want hidden", projection.AIPanel)
	}
	if projection.ArchiveStatus != "not_generated" {
		t.Fatalf("ArchiveStatus = %q, want not_generated", projection.ArchiveStatus)
	}
}

func TestProfileSelectionCarriesProfileIDAndVersion(t *testing.T) {
	selection := ProfileSelection{ProfileID: "profile-1", Version: 3}
	if selection.ProfileID != "profile-1" {
		t.Fatalf("ProfileID = %q", selection.ProfileID)
	}
	if selection.Version != 3 {
		t.Fatalf("Version = %d", selection.Version)
	}
}

func TestWorkbenchStateRecordsUpdatedAt(t *testing.T) {
	now := time.Unix(200, 0).UTC()
	state := WorkbenchState{
		Stage:         StageWriting,
		Canvas:        "manuscript",
		AIPanel:       "hidden",
		ArchiveStatus: "not_generated",
		UpdatedAt:     now,
	}
	if !state.UpdatedAt.Equal(now) {
		t.Fatalf("UpdatedAt = %v, want %v", state.UpdatedAt, now)
	}
}
