package creation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// planningRearchiveStore 将正式快照和原子追加限定在持久化边界。
type planningRearchiveStore interface {
	PlanningArchiveAssets(context.Context, string) ([]asset.PlanningArchiveAsset, error)
	PlanningArchivedPeopleCandidate(context.Context, string) (string, error)
	PreviewConstraintChange(context.Context, asset.ConstraintRequest) (asset.ConstraintPreview, error)
	ApplyPlanningRearchive(context.Context, asset.PlanningRearchiveApply, time.Time) (asset.PlanningRearchivePreview, error)
	PlanningRearchiveReceipt(context.Context, asset.PlanningRearchiveRequest, string, string) (asset.PlanningRearchivePreview, bool, error)
}

func hashPlanningPreview(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// PreviewPlanningRearchive 将已确认候选的可映射文字与正式版本逐项比对。
func (service *Service) PreviewPlanningRearchive(ctx context.Context, request asset.PlanningRearchiveRequest) (asset.PlanningRearchivePreview, error) {
	reader, ok := service.store.(planningRearchiveStore)
	if !ok {
		return asset.PlanningRearchivePreview{}, ErrUnavailable
	}
	session, candidates, err := service.loadArchiveInputs(ctx, request.ProjectID)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	if session.ArchiveChangeSetID == "" || session.ActiveRunID != "" || request.BranchID == "" || strings.TrimSpace(request.Reason) == "" || len(request.Reason) > 2000 || request.ChapterID == "" {
		return asset.PlanningRearchivePreview{}, asset.ErrChangeUnsupported
	}
	proposal, people, roadmap, err := requiredArchiveCandidates(session, candidates)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	guards := []asset.PlanningRearchiveGuard{}
	for _, candidate := range []creationdomain.Candidate{proposal, roadmap} {
		if err = validateCandidateSchema(candidate.Stage, candidate.PayloadJSON, candidate.PayloadSchemaVersion); err != nil {
			return asset.PlanningRearchivePreview{}, err
		}
		if err = validateCandidateDependency(session, candidates, candidate); err != nil {
			return asset.PlanningRearchivePreview{}, err
		}
		guard, e := checkedRearchiveCandidateReview(ctx, service, candidate)
		if e != nil {
			return asset.PlanningRearchivePreview{}, e
		}
		guards = append(guards, guard)
	}
	peopleSource, err := reader.PlanningArchivedPeopleCandidate(ctx, request.ProjectID)
	if err != nil || people.ID != peopleSource {
		return asset.PlanningRearchivePreview{}, errRearchiveIdentity
	}
	peopleReview, err := service.store.Review(ctx, people.ID)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	peopleGuard := asset.PlanningRearchiveGuard{CandidateID: people.ID, ContentSHA256: people.ContentSHA256, ReviewVersion: peopleReview.ReviewVersion}
	guards = []asset.PlanningRearchiveGuard{guards[0], peopleGuard, guards[1]}
	assets, err := reader.PlanningArchiveAssets(ctx, request.ProjectID)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	var originalProposalID, originalRoadmapID string
	roadAssets := []asset.PlanningArchiveAsset{}
	for _, row := range assets {
		switch row.Kind {
		case "proposal":
			if originalProposalID != "" {
				return asset.PlanningRearchivePreview{}, errRearchiveIdentity
			}
			originalProposalID = row.SourceCandidateID
		case "outline_node":
			if originalRoadmapID != "" && originalRoadmapID != row.SourceCandidateID {
				return asset.PlanningRearchivePreview{}, errRearchiveIdentity
			}
			originalRoadmapID = row.SourceCandidateID
			roadAssets = append(roadAssets, row)
		}
	}
	if originalProposalID == "" || originalRoadmapID == "" || len(roadAssets) == 0 {
		return asset.PlanningRearchivePreview{}, errRearchiveIdentity
	}
	if !rearchiveDescends(ctx, service.store, proposal, originalProposalID) || !rearchiveDescends(ctx, service.store, roadmap, originalRoadmapID) {
		return asset.PlanningRearchivePreview{}, errRearchiveIdentity
	}
	oldProposal, err := service.store.Candidate(ctx, originalProposalID)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	oldRoadmap, err := service.store.Candidate(ctx, originalRoadmapID)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	if err = rearchiveScope(oldProposal, proposal, "proposal"); err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	if err = rearchiveScope(oldRoadmap, roadmap, "roadmap"); err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	oldRoad, err := decodeArchivableRoadmap(oldRoadmap)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	newRoad, err := decodeArchivableRoadmap(roadmap)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	roadFields, err := mapRearchiveRoadmap(oldRoad, newRoad, roadAssets)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	if _, err = decodeBlueprint(proposal.PayloadJSON); err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	preview := asset.PlanningRearchivePreview{Request: request, Changes: []asset.PlanningRearchiveChange{}}
	for _, row := range assets {
		var desired string
		if row.Kind == "proposal" {
			desired, err = planningTextMap("proposal", proposal.PayloadJSON)
		} else {
			encoded, _ := json.Marshal(roadFields[row.AssetID])
			desired, err = planningTextMap("outline_node", string(encoded))
		}
		if err != nil {
			return preview, err
		}
		current, e := planningTextMap(row.Kind, row.Current)
		if e != nil {
			return preview, e
		}
		if current == desired {
			continue
		}
		changeRequest := asset.ConstraintRequest{ProjectID: request.ProjectID, BranchID: request.BranchID, AssetID: row.AssetID, Intent: "future_only", ChapterID: request.ChapterID, Text: desired, Reason: request.Reason}
		cp, e := reader.PreviewConstraintChange(ctx, changeRequest)
		if e != nil {
			return preview, e
		}
		if !cp.CanApply {
			return preview, fmt.Errorf("%w: %s", asset.ErrChangeConflict, cp.BlockedReason)
		}
		if len(preview.Changes) == 0 {
			preview.Position = cp.Position
			preview.ProtectedChapters = cp.ProtectedChapters
			preview.FutureChapters = cp.FutureChapters
		}
		preview.Changes = append(preview.Changes, asset.PlanningRearchiveChange{AssetID: row.AssetID, Kind: row.Kind, Before: cp.Before, After: cp.Request.Text, Request: cp.Request, Digest: cp.Digest})
	}
	if len(preview.Changes) == 0 {
		return preview, asset.ErrChangeUnsupported
	}
	sortedRearchiveChanges(preview.Changes)
	preview.Digest = hashPlanningPreview(struct {
		Request asset.PlanningRearchiveRequest
		Assets  []asset.PlanningArchiveAsset
		Changes []asset.PlanningRearchiveChange
		Guards  []asset.PlanningRearchiveGuard
	}{request, assets, preview.Changes, guards})
	return preview, nil
}

// ConfirmPlanningRearchive 以同一组预览参数重算确认，再由仓储原子追加版本。
func (service *Service) ConfirmPlanningRearchive(ctx context.Context, command asset.PlanningRearchiveCommand) (asset.PlanningRearchivePreview, error) {
	reader, ok := service.store.(planningRearchiveStore)
	if !ok {
		return asset.PlanningRearchivePreview{}, ErrUnavailable
	}
	if command.IdempotencyKey == "" || command.Digest == "" {
		return asset.PlanningRearchivePreview{}, asset.ErrChangeConflict
	}
	if receipt, found, err := reader.PlanningRearchiveReceipt(ctx, command.Request, command.Digest, command.IdempotencyKey); err != nil || found {
		return receipt, err
	}
	preview, err := service.PreviewPlanningRearchive(ctx, command.Request)
	if err != nil {
		return preview, err
	}
	if preview.Digest != command.Digest {
		return preview, asset.ErrChangeConflict
	}
	session, candidates, err := service.loadArchiveInputs(ctx, command.Request.ProjectID)
	if err != nil {
		return preview, err
	}
	proposal, people, roadmap, err := requiredArchiveCandidates(session, candidates)
	if err != nil {
		return preview, err
	}
	first, err := checkedRearchiveCandidateReview(ctx, service, proposal)
	if err != nil {
		return preview, err
	}
	peopleSource, err := reader.PlanningArchivedPeopleCandidate(ctx, command.Request.ProjectID)
	if err != nil || people.ID != peopleSource {
		return preview, errRearchiveIdentity
	}
	peopleReview, err := service.store.Review(ctx, people.ID)
	if err != nil {
		return preview, err
	}
	second := asset.PlanningRearchiveGuard{CandidateID: people.ID, ContentSHA256: people.ContentSHA256, ReviewVersion: peopleReview.ReviewVersion}
	third, err := checkedRearchiveCandidateReview(ctx, service, roadmap)
	if err != nil {
		return preview, err
	}
	assets, err := reader.PlanningArchiveAssets(ctx, command.Request.ProjectID)
	if err != nil {
		return preview, err
	}
	return reader.ApplyPlanningRearchive(ctx, asset.PlanningRearchiveApply{Preview: preview, Assets: assets, Guards: []asset.PlanningRearchiveGuard{first, second, third}, Key: command.IdempotencyKey}, service.clock.Now())
}

func checkedRearchiveCandidateReview(ctx context.Context, service *Service, candidate creationdomain.Candidate) (asset.PlanningRearchiveGuard, error) {
	if candidate.Status != creationdomain.StatusConfirmed || candidate.Blocked() {
		return asset.PlanningRearchiveGuard{}, creationdomain.ErrCandidateAcceptanceRequired
	}
	review, err := service.store.Review(ctx, candidate.ID)
	if err != nil {
		return asset.PlanningRearchiveGuard{}, err
	}
	for _, refs := range reviewRefs(candidate) {
		for _, ref := range refs {
			if review.State(ref) != creationdomain.DecisionAccept {
				return asset.PlanningRearchiveGuard{}, ErrReviewIncomplete
			}
		}
	}
	return asset.PlanningRearchiveGuard{CandidateID: candidate.ID, ContentSHA256: candidate.ContentSHA256, ReviewVersion: review.ReviewVersion}, nil
}

func rearchiveDescends(ctx context.Context, store Store, candidate creationdomain.Candidate, sourceID string) bool {
	seen := map[string]bool{}
	for candidate.ID != "" && !seen[candidate.ID] {
		if candidate.ID == sourceID {
			return true
		}
		seen[candidate.ID] = true
		if candidate.ParentCandidateID == "" {
			return false
		}
		parent, err := store.Candidate(ctx, candidate.ParentCandidateID)
		if err != nil || parent.SessionID != candidate.SessionID {
			return false
		}
		candidate = parent
	}
	return false
}

func rearchiveScope(source, current creationdomain.Candidate, kind string) error {
	if !reflect.DeepEqual(source.LockMap, current.LockMap) {
		return asset.ErrChangeConflict
	}
	paths := []string{}
	if kind == "proposal" {
		fields, err := asset.PlanningFields("proposal", source.PayloadJSON)
		if err != nil {
			return err
		}
		for _, field := range fields {
			if !source.Locked(field.Path) && !strings.HasPrefix(field.Path, "/major_milestones/") && !strings.HasPrefix(field.Path, "/subplots/") {
				paths = append(paths, field.Path)
			}
		}
	} else {
		for _, field := range candidateTextFields(source).Fields {
			if rearchiveRoadmapPath(field.Path) && !source.Locked(field.Path) {
				paths = append(paths, field.Path)
			}
		}
	}
	if len(paths) == 0 {
		return asset.ErrChangeUnsupported
	}
	if _, err := source.ReviewScopedRevision(current.PayloadJSON, creationdomain.RevisionScope{Paths: paths}); err != nil {
		return fmt.Errorf("%w: %v", asset.ErrChangeUnsupported, err)
	}
	return nil
}

// rearchiveRoadmapPath 只准许已有正式字段可完整表达的文字；结构与锚点另行处理。
func rearchiveRoadmapPath(path string) bool {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) == 2 && parts[0] == "book" {
		return parts[1] == "title" || parts[1] == "proposition" || parts[1] == "protagonist_arc" || parts[1] == "ending"
	}
	if len(parts) == 3 && strings.HasPrefix(parts[1], "@") {
		switch parts[0] {
		case "volumes":
			return parts[2] == "title" || parts[2] == "goal" || parts[2] == "conflict" || parts[2] == "payoff"
		case "arcs":
			switch parts[2] {
			case "title", "causal_chain", "character_change", "reader_expectation", "stage_payoff", "irreversible_change", "ending_contribution":
				return true
			}
		case "near_term_chapters":
			return parts[2] == "title" || parts[2] == "goal" || parts[2] == "conflict" || parts[2] == "outcome"
		}
	}
	if len(parts) == 5 && parts[0] == "arcs" && strings.HasPrefix(parts[1], "@") && (parts[2] == "commitments" || parts[2] == "retained_questions") && strings.HasPrefix(parts[3], "@") {
		return parts[4] == "statement" || parts[4] == "fulfillment_range" || parts[4] == "reason"
	}
	return false
}

func planningTextMap(kind, payload string) (string, error) {
	fields, err := asset.PlanningFields(kind, payload)
	if err != nil {
		return "", err
	}
	values := map[string]string{}
	for _, field := range fields {
		values[field.Path] = field.Value
	}
	encoded, _ := json.Marshal(values)
	return string(encoded), nil
}

func sortedRearchiveChanges(changes []asset.PlanningRearchiveChange) {
	sort.Slice(changes, func(i, j int) bool { return changes[i].AssetID < changes[j].AssetID })
}

var errRearchiveIdentity = errors.New("正式规划来源身份无法唯一核验")
