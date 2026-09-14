package creation

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// decodeMaterialForEntry 将模型引用与实际原文核对；模型不能伪造来源或吞掉缺口。
func decodeMaterialForEntry(entry *creationdomain.EntryRevision, payload string) (agentdomain.MaterialAnalysisOutput, error) {
	var output agentdomain.MaterialAnalysisOutput
	if entry == nil || entry.Kind != creationdomain.EntryAuthorPlan || entry.Validate() != nil {
		return output, creationdomain.ErrInvalidEntry
	}
	if err := agentdomain.UnmarshalModelJSON([]byte(payload), &output); err != nil {
		return output, err
	}
	if err := output.Validate(); err != nil {
		return output, err
	}
	if output.EntryID != entry.ID || output.SourceHash != entry.MaterialSHA256 {
		return output, creationdomain.ErrInvalidSourceSpan
	}
	for _, source := range output.Sources {
		if _, ok := materialPayload(output, creationdomain.Stage(source.Stage)); !ok {
			return output, creationdomain.ErrInvalidSourceSpan
		}
		if _, err := materialQuote(*entry, source.Quote); err != nil {
			return output, err
		}
	}
	for _, gap := range output.Gaps {
		if gap.Quote != "" {
			if _, err := materialQuote(*entry, gap.Quote); err != nil {
				return output, err
			}
		}
	}
	for _, stage := range []creationdomain.Stage{creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap} {
		data, ok := materialPayload(output, stage)
		if !ok {
			continue
		}
		if err := validateCandidatePayload(stage, data); err != nil {
			return output, err
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(data), &fields); err != nil {
			return output, err
		}
		covered := map[string]bool{}
		for _, source := range output.Sources {
			if source.Stage != string(stage) {
				continue
			}
			key := strings.TrimPrefix(source.FieldPath, "/")
			if _, exists := fields[key]; !exists || covered[key] {
				return output, creationdomain.ErrInvalidSourceSpan
			}
			if _, err := materialQuote(*entry, source.Quote); err != nil {
				return output, err
			}
			covered[key] = true
		}
		for key := range fields {
			if !covered[key] {
				return output, creationdomain.ErrInvalidSourceSpan
			}
		}
	}
	return output, nil
}

// materialQuote 用唯一引文生成可靠的 Unicode 来源范围，不相信模型计算的位置。
func materialQuote(entry creationdomain.EntryRevision, quote string) (creationdomain.MaterialSpan, error) {
	if quote == "" || strings.Count(entry.Material, quote) != 1 {
		return creationdomain.MaterialSpan{}, creationdomain.ErrInvalidSourceSpan
	}
	offset := strings.Index(entry.Material, quote)
	start := utf8.RuneCountInString(entry.Material[:offset])
	return creationdomain.MaterialSpan{EntryID: entry.ID, MaterialSHA256: entry.MaterialSHA256, Start: start, End: start + utf8.RuneCountInString(quote)}, nil
}

func materialPayload(output agentdomain.MaterialAnalysisOutput, stage creationdomain.Stage) (string, bool) {
	var value any
	switch stage {
	case creationdomain.StageProposal:
		if output.Proposal == nil {
			return "", false
		}
		value = output.Proposal
	case creationdomain.StagePeopleWorld:
		if output.PeopleWorld == nil {
			return "", false
		}
		value = output.PeopleWorld
	case creationdomain.StageRoadmap:
		if output.Roadmap == nil {
			return "", false
		}
		value = output.Roadmap
	default:
		return "", false
	}
	raw, err := json.Marshal(value)
	return string(raw), err == nil
}

func validatedMaterial(session creationdomain.Session, candidates []creationdomain.Candidate) (creationdomain.Candidate, agentdomain.MaterialAnalysisOutput, error) {
	analysis, ok := currentCandidate(candidates, creationdomain.StageIntent)
	if !ok || analysis.PayloadSchemaID != "author_material.schema" || analysis.PayloadSchemaVersion != 1 {
		return analysis, agentdomain.MaterialAnalysisOutput{}, creationdomain.ErrStageIncomplete
	}
	if analysis.Blocked() {
		return analysis, agentdomain.MaterialAnalysisOutput{}, creationdomain.ErrQualityBlocked
	}
	if err := validateCandidateDependency(session, candidates, analysis); err != nil {
		return analysis, agentdomain.MaterialAnalysisOutput{}, err
	}
	output, err := decodeMaterialForEntry(session.Entry, analysis.PayloadJSON)
	return analysis, output, err
}

// validateMaterialGapsForArchive 每次确认重新核对报告与解决候选，冲突不可被普通阶段接受绕过。
func validateMaterialGapsForArchive(session creationdomain.Session, candidates []creationdomain.Candidate) error {
	analysis, output, err := validatedMaterial(session, candidates)
	if err != nil {
		return err
	}
	report := creationdomain.PlanGapReport{ID: analysis.ID, SchemaVersion: 1, EntryID: output.EntryID, MaterialSHA256: output.SourceHash}
	for _, gap := range output.Gaps {
		item := creationdomain.PlanGap{ID: gap.ID, FieldPath: gap.FieldPath, Kind: creationdomain.GapKind(gap.Kind), Reason: gap.Reason, SuggestedAction: gap.SuggestedAction, Resolution: creationdomain.GapOpen}
		if gap.Quote != "" {
			span, err := materialQuote(*session.Entry, gap.Quote)
			if err != nil {
				return err
			}
			item.Evidence = []creationdomain.MaterialSpan{span}
		}
		candidate, ok := currentCandidate(candidates, creationdomain.Stage(gap.Stage))
		if gap.Kind != "conflict" && ok && candidate.Status == creationdomain.StatusConfirmed && validateCandidateDependency(session, candidates, candidate) == nil && !candidate.Blocked() {
			item.Resolution = creationdomain.GapResolved
			item.ResolutionCandidateID = candidate.ID
			item.ResolutionCandidateSHA256 = candidate.ContentSHA256
		}
		report.Items = append(report.Items, item)
	}
	if !report.Ready(*session.Entry) {
		return creationdomain.ErrInvalidGapReport
	}
	return nil
}

type UseMaterialRequest struct {
	ProjectID       string               `json:"projectId"`
	SessionID       string               `json:"sessionId"`
	ExpectedVersion int64                `json:"expectedVersion"`
	AnalysisID      string               `json:"analysisId"`
	Stage           creationdomain.Stage `json:"stage"`
}

// UseMaterialCandidate 显式复用原文已提供的阶段载荷，仍保持待接受状态，不调用模型。
func (service *Service) UseMaterialCandidate(ctx context.Context, request UseMaterialRequest) (creationdomain.Candidate, error) {
	session, err := service.loadSession(ctx, request.ProjectID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if session.ID != request.SessionID || session.ContentVersion != request.ExpectedVersion {
		return creationdomain.Candidate{}, creationdomain.ErrSessionVersionConflict
	}
	if session.ActiveRunID != "" || session.ActiveOperationID != "" {
		return creationdomain.Candidate{}, ErrRunActive
	}
	if session.EntryKind() != creationdomain.EntryAuthorPlan || session.Stage != request.Stage {
		return creationdomain.Candidate{}, ErrInvalidStage
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	analysis, output, err := validatedMaterial(session, candidates)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if analysis.ID != request.AnalysisID {
		return creationdomain.Candidate{}, creationdomain.ErrCandidateVersionConflict
	}
	payload, ok := materialPayload(output, request.Stage)
	if !ok {
		return creationdomain.Candidate{}, creationdomain.ErrStageIncomplete
	}
	if current, exists := currentCandidate(candidates, request.Stage); exists {
		if current.ParentCandidateID == analysis.ID && current.PayloadJSON == payload && validateCandidateDependency(session, candidates, current) == nil {
			return current, nil
		}
		return creationdomain.Candidate{}, creationdomain.ErrCandidateVersionConflict
	}
	binding, err := bindingForEntry(session.EntryKind(), request.Stage)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if request.Stage == creationdomain.StageRoadmap {
		if road, decodeErr := decodeRoadmap(payload); decodeErr == nil && road.ValidatePlanningGoals() != nil {
			binding.WorkflowID = "story.author_roadmap"
			binding.Version = 1
			binding.SchemaVersion = 2
		}
	}
	version := 1
	for _, old := range candidates {
		if old.Stage == request.Stage && old.Version >= version {
			version = old.Version + 1
		}
	}
	candidate, err := creationdomain.NewCandidate(service.ids.NewID(), session.ID, request.Stage, binding.Kind, version, analysis.ID, "", analysis.ArtifactID, payload, binding.SchemaID, binding.SchemaVersion, service.clock.Now())
	if err != nil {
		return candidate, err
	}
	candidate = candidate.WithUpstream(liveUpstreamDigest(session, candidates, request.Stage)).WithQuality(analysis.QualityStatus)
	if err := service.store.SaveCandidate(ctx, candidate); err != nil {
		return creationdomain.Candidate{}, err
	}
	return candidate, nil
}

type MaterialView struct {
	AnalysisID      string                    `json:"analysisId"`
	Status          string                    `json:"status"`
	AvailableStages []creationdomain.Stage    `json:"availableStages"`
	Gaps            []agentdomain.MaterialGap `json:"gaps"`
}

func projectMaterial(session creationdomain.Session, candidates []creationdomain.Candidate) MaterialView {
	view := MaterialView{Status: "not_analyzed", AvailableStages: []creationdomain.Stage{}, Gaps: []agentdomain.MaterialGap{}}
	if session.EntryKind() != creationdomain.EntryAuthorPlan {
		return view
	}
	analysis, output, err := validatedMaterial(session, candidates)
	if err != nil {
		if analysis.ID != "" {
			view.AnalysisID = analysis.ID
			view.Status = "needs_review"
		}
		return view
	}
	view.AnalysisID = analysis.ID
	view.Status = "ready"
	view.Gaps = output.Gaps
	for _, stage := range []creationdomain.Stage{creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap} {
		if _, ok := materialPayload(output, stage); ok {
			view.AvailableStages = append(view.AvailableStages, stage)
		}
	}
	return view
}
