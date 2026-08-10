package workspace

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

type ChapterChangeInput struct {
	ChapterID    string `json:"chapterId"`
	Title        string `json:"title"`
	Summary      string `json:"summary"`
	ProposedBody string `json:"proposedBody"`
	Reason       string `json:"reason"`
	CreatedBy    string `json:"createdBy"`
}

type OutlineContentInput struct {
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Goal     string `json:"goal"`
	Conflict string `json:"conflict"`
	Outcome  string `json:"outcome"`
	// Level and Position are only used for create_outline_node operations
	// produced by manuscript extraction. Update operations leave them empty.
	Level    string `json:"level,omitempty"`
	Position int    `json:"position,omitempty"`
	ParentID string `json:"parentId,omitempty"`
}

type StructuredChangeOperationInput struct {
	Kind              string                    `json:"kind"`
	TargetID          string                    `json:"targetId"`
	Reason            string                    `json:"reason"`
	Outline           *OutlineContentInput      `json:"outline,omitempty"`
	Character         *CharacterInput           `json:"character,omitempty"`
	Relationship      *RelationshipInput        `json:"relationship,omitempty"`
	WorldRule         *WorldRuleInput           `json:"worldRule,omitempty"`
	StoryEvent        *StoryEventInput          `json:"storyEvent,omitempty"`
	Fact              *FactInput                `json:"fact,omitempty"`
	ForeshadowingHook *ForeshadowingHookInput   `json:"foreshadowingHook,omitempty"`
	Evidence          []revisiondomain.Evidence `json:"evidence,omitempty"`
}

type StructuredChangeInput struct {
	Title      string                           `json:"title"`
	Summary    string                           `json:"summary"`
	CreatedBy  string                           `json:"createdBy"`
	Operations []StructuredChangeOperationInput `json:"operations"`
}

type ChangeOperationEditInput struct {
	ChangeSetID       string                  `json:"changeSetId"`
	OperationID       string                  `json:"operationId"`
	ChapterBody       string                  `json:"chapterBody"`
	Outline           *OutlineContentInput    `json:"outline,omitempty"`
	Character         *CharacterInput         `json:"character,omitempty"`
	Relationship      *RelationshipInput      `json:"relationship,omitempty"`
	WorldRule         *WorldRuleInput         `json:"worldRule,omitempty"`
	StoryEvent        *StoryEventInput        `json:"storyEvent,omitempty"`
	Fact              *FactInput              `json:"fact,omitempty"`
	ForeshadowingHook *ForeshadowingHookInput `json:"foreshadowingHook,omitempty"`
}

type ChangeOperationView struct {
	ID              string                    `json:"id"`
	Kind            string                    `json:"kind"`
	TargetID        string                    `json:"targetId"`
	BaseRevision    int64                     `json:"baseRevision"`
	BaseFingerprint string                    `json:"baseFingerprint"`
	BeforeValue     string                    `json:"beforeValue"`
	AfterValue      string                    `json:"afterValue"`
	Reason          string                    `json:"reason"`
	Status          string                    `json:"status"`
	Evidence        []revisiondomain.Evidence `json:"evidence,omitempty"`
}

type ChangeSetView struct {
	ID         string                `json:"id"`
	Title      string                `json:"title"`
	Summary    string                `json:"summary"`
	CreatedBy  string                `json:"createdBy"`
	Status     string                `json:"status"`
	Operations []ChangeOperationView `json:"operations"`
	CreatedAt  string                `json:"createdAt"`
	UpdatedAt  string                `json:"updatedAt"`
}

type ChangeSetApplyView struct {
	ChangeSet ChangeSetView `json:"changeSet"`
	Chapters  []ChapterView `json:"chapters"`
}

type factChangeValue struct {
	Text              string  `json:"text"`
	Category          string  `json:"category"`
	Status            string  `json:"status"`
	Confidence        float64 `json:"confidence"`
	ValidFromEventID  string  `json:"validFromEventId"`
	ValidUntilEventID string  `json:"validUntilEventId"`
}

func (s *Service) ChangeSets(ctx context.Context) ([]ChangeSetView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	changeSets, err := s.active.store.ListChangeSets(ctx)
	if err != nil {
		return nil, fmt.Errorf("list change sets: %w", err)
	}
	sort.Slice(changeSets, func(i, j int) bool {
		if changeSets[i].CreatedAt.Equal(changeSets[j].CreatedAt) {
			return changeSets[i].ID > changeSets[j].ID
		}
		return changeSets[i].CreatedAt.After(changeSets[j].CreatedAt)
	})
	views := make([]ChangeSetView, 0, len(changeSets))
	for _, changeSet := range changeSets {
		views = append(views, changeSetView(changeSet))
	}
	return views, nil
}

func (s *Service) ProposeChapterChange(ctx context.Context, input ChapterChangeInput) (ChangeSetView, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return ChangeSetView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return ChangeSetView{}, err
	}
	chapter, err := s.active.store.Chapter(ctx, strings.TrimSpace(input.ChapterID))
	if err != nil || chapter.ProjectID != project.ID {
		return ChangeSetView{}, fmt.Errorf("chapter change target is invalid")
	}
	changeSet, err := revisiondomain.NewChapterBodyChangeSet(
		s.ids.NewID(),
		s.ids.NewID(),
		project.ID,
		chapter.ID,
		input.Title,
		input.Summary,
		input.CreatedBy,
		chapter.Revision,
		chapter.Body,
		input.ProposedBody,
		input.Reason,
		s.clock.Now(),
	)
	if err != nil {
		return ChangeSetView{}, err
	}
	if err := s.active.store.CreateChangeSet(ctx, changeSet); err != nil {
		return ChangeSetView{}, fmt.Errorf("create change set: %w", err)
	}
	return changeSetView(changeSet), nil
}

func (s *Service) ProposeStructuredChange(ctx context.Context, input StructuredChangeInput) (ChangeSetView, error) {
	if err := s.license.Require(ctx, license.RunAI); err != nil {
		return ChangeSetView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return ChangeSetView{}, err
	}
	specs := make([]revisiondomain.PatchOperationSpec, 0, len(input.Operations))
	for _, operation := range input.Operations {
		spec, err := s.structuredOperationSpecLocked(ctx, project.ID, operation)
		if err != nil {
			return ChangeSetView{}, err
		}
		spec.ID = s.ids.NewID()
		specs = append(specs, spec)
	}
	changeSet, err := revisiondomain.NewChangeSet(
		s.ids.NewID(), project.ID, input.Title, input.Summary, input.CreatedBy, specs, s.clock.Now(),
	)
	if err != nil {
		return ChangeSetView{}, err
	}
	if err := s.active.store.CreateChangeSet(ctx, changeSet); err != nil {
		return ChangeSetView{}, fmt.Errorf("create structured change set: %w", err)
	}
	return changeSetView(changeSet), nil
}

func (s *Service) EditChangeOperation(ctx context.Context, input ChangeOperationEditInput) (ChangeSetView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return ChangeSetView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ChangeSetView{}, err
	}
	changeSet, err := s.active.store.ChangeSet(ctx, strings.TrimSpace(input.ChangeSetID))
	if err != nil {
		return ChangeSetView{}, fmt.Errorf("read change set: %w", err)
	}
	var selected *revisiondomain.PatchOperation
	for index := range changeSet.Operations {
		if changeSet.Operations[index].ID == strings.TrimSpace(input.OperationID) {
			selected = &changeSet.Operations[index]
			break
		}
	}
	if selected == nil {
		return ChangeSetView{}, revisiondomain.ErrOperationMissing
	}
	afterValue, err := s.editedOperationValueLocked(ctx, *selected, input)
	if err != nil {
		return ChangeSetView{}, err
	}
	edited, err := s.active.store.EditChangeOperation(
		ctx, changeSet.ID, selected.ID, afterValue, s.clock.Now(),
	)
	if err != nil {
		return ChangeSetView{}, fmt.Errorf("edit change operation: %w", err)
	}
	return changeSetView(edited), nil
}

func (s *Service) ApplyChangeSet(ctx context.Context, id string) (ChangeSetApplyView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return ChangeSetApplyView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ChangeSetApplyView{}, err
	}
	changeSet, err := s.active.store.ChangeSet(ctx, strings.TrimSpace(id))
	if err != nil {
		return ChangeSetApplyView{}, fmt.Errorf("read change set: %w", err)
	}
	pending := 0
	for _, operation := range changeSet.Operations {
		if operation.Status == revisiondomain.ChangeSetPending {
			pending++
		}
	}
	if pending > 1 {
		name := fmt.Sprintf("应用候选“%s”前", changeSet.Title)
		if _, err := s.createSnapshotLocked(ctx, name, revisiondomain.SnapshotRecovery); err != nil {
			return ChangeSetApplyView{}, fmt.Errorf("create change set recovery point: %w", err)
		}
	}
	result, err := s.active.store.ApplyChangeSet(ctx, changeSet.ID, s.clock.Now())
	if err != nil {
		return ChangeSetApplyView{}, fmt.Errorf("apply change set: %w", err)
	}
	return changeSetApplyView(result.ChangeSet, result.Chapters), nil
}

func (s *Service) ApplyChangeOperation(ctx context.Context, changeSetID, operationID string) (ChangeSetApplyView, error) {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return ChangeSetApplyView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ChangeSetApplyView{}, err
	}
	result, err := s.active.store.ApplyChangeOperation(
		ctx, strings.TrimSpace(changeSetID), strings.TrimSpace(operationID), s.clock.Now(),
	)
	if err != nil {
		return ChangeSetApplyView{}, fmt.Errorf("apply change operation: %w", err)
	}
	return changeSetApplyView(result.ChangeSet, result.Chapters), nil
}

func (s *Service) RejectChangeOperation(ctx context.Context, changeSetID, operationID string) error {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return err
	}
	if err := s.active.store.RejectChangeOperation(
		ctx, strings.TrimSpace(changeSetID), strings.TrimSpace(operationID), s.clock.Now(),
	); err != nil {
		return fmt.Errorf("reject change operation: %w", err)
	}
	return nil
}

func (s *Service) RejectChangeSet(ctx context.Context, id string) error {
	if err := s.license.Require(ctx, license.EditManuscript); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return err
	}
	if err := s.active.store.RejectChangeSet(ctx, strings.TrimSpace(id), s.clock.Now()); err != nil {
		return fmt.Errorf("reject change set: %w", err)
	}
	return nil
}

func changeSetView(changeSet revisiondomain.ChangeSet) ChangeSetView {
	operations := make([]ChangeOperationView, 0, len(changeSet.Operations))
	for _, operation := range changeSet.Operations {
		view := ChangeOperationView{
			ID:              operation.ID,
			Kind:            string(operation.Kind),
			TargetID:        operation.TargetID,
			BaseRevision:    operation.BaseRevision,
			BaseFingerprint: operation.BaseFingerprint,
			BeforeValue:     operation.BeforeValue,
			AfterValue:      operation.AfterValue,
			Reason:          operation.Reason,
			Status:          string(operation.Status),
		}
		if len(operation.Evidence) > 0 {
			view.Evidence = append([]revisiondomain.Evidence(nil), operation.Evidence...)
		}
		operations = append(operations, view)
	}
	return ChangeSetView{
		ID:         changeSet.ID,
		Title:      changeSet.Title,
		Summary:    changeSet.Summary,
		CreatedBy:  changeSet.CreatedBy,
		Status:     string(changeSet.Status),
		Operations: operations,
		CreatedAt:  changeSet.CreatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
		UpdatedAt:  changeSet.UpdatedAt.Format("2006-01-02T15:04:05.999999999Z07:00"),
	}
}

func changeSetApplyView(changeSet revisiondomain.ChangeSet, chapters []projectdomain.Chapter) ChangeSetApplyView {
	chapterViews := make([]ChapterView, 0, len(chapters))
	for _, chapter := range chapters {
		chapterViews = append(chapterViews, chapterView(chapter))
	}
	return ChangeSetApplyView{ChangeSet: changeSetView(changeSet), Chapters: chapterViews}
}

func (s *Service) structuredOperationSpecLocked(
	ctx context.Context,
	projectID string,
	input StructuredChangeOperationInput,
) (revisiondomain.PatchOperationSpec, error) {
	spec := revisiondomain.PatchOperationSpec{
		Kind:     revisiondomain.OperationKind(strings.TrimSpace(input.Kind)),
		TargetID: strings.TrimSpace(input.TargetID),
		Reason:   strings.TrimSpace(input.Reason),
	}
	switch spec.Kind {
	case revisiondomain.OperationUpdateOutlineNode:
		if input.Outline == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		current, err := s.outlineNodeLocked(ctx, spec.TargetID)
		if err != nil || current.ProjectID != projectID {
			return revisiondomain.PatchOperationSpec{}, fmt.Errorf("outline change target is invalid")
		}
		if _, err := current.UpdateContent(
			input.Outline.Title, input.Outline.Summary, input.Outline.Goal,
			input.Outline.Conflict, input.Outline.Outcome, s.clock.Now(),
		); err != nil {
			return revisiondomain.PatchOperationSpec{}, err
		}
		spec.BeforeValue = mustJSON(outlineContentFromDomain(current))
		spec.AfterValue = mustJSON(*input.Outline)
	case revisiondomain.OperationUpdateCharacter:
		if input.Character == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		current, err := s.active.store.Character(ctx, spec.TargetID)
		if err != nil || current.ProjectID != projectID {
			return revisiondomain.PatchOperationSpec{}, fmt.Errorf("character change target is invalid")
		}
		if _, err := updateCharacterFromInput(current, *input.Character, s.clock.Now()); err != nil {
			return revisiondomain.PatchOperationSpec{}, err
		}
		spec.BeforeValue = mustJSON(characterInputFromDomain(current))
		spec.AfterValue = mustJSON(*input.Character)
	case revisiondomain.OperationUpdateFact:
		if input.Fact == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		current, err := s.active.store.Fact(ctx, spec.TargetID)
		if err != nil || current.ProjectID != projectID {
			return revisiondomain.PatchOperationSpec{}, fmt.Errorf("fact change target is invalid")
		}
		if _, err := updateFactFromInput(current, *input.Fact, s.clock.Now()); err != nil {
			return revisiondomain.PatchOperationSpec{}, err
		}
		spec.BeforeValue = mustJSON(factInputFromDomain(current))
		spec.AfterValue = mustJSON(factMutableInput(*input.Fact))
	case revisiondomain.OperationCreateOutlineNode:
		if input.Outline == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		spec.BeforeValue = "{}"
		spec.AfterValue = mustJSON(*input.Outline)
		spec.Evidence = input.Evidence
	case revisiondomain.OperationCreateCharacter:
		if input.Character == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		spec.BeforeValue = "{}"
		spec.AfterValue = mustJSON(*input.Character)
		spec.Evidence = input.Evidence
	case revisiondomain.OperationCreateRelationship:
		if input.Relationship == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		spec.BeforeValue = "{}"
		spec.AfterValue = mustJSON(*input.Relationship)
		spec.Evidence = input.Evidence
	case revisiondomain.OperationCreateWorldRule:
		if input.WorldRule == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		spec.BeforeValue = "{}"
		spec.AfterValue = mustJSON(*input.WorldRule)
		spec.Evidence = input.Evidence
	case revisiondomain.OperationCreateStoryEvent:
		if input.StoryEvent == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		spec.BeforeValue = "{}"
		spec.AfterValue = mustJSON(*input.StoryEvent)
		spec.Evidence = input.Evidence
	case revisiondomain.OperationCreateFact:
		if input.Fact == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		spec.BeforeValue = "{}"
		spec.AfterValue = mustJSON(factMutableInput(*input.Fact))
		spec.Evidence = input.Evidence
	case revisiondomain.OperationCreateForeshadowingHook:
		if input.ForeshadowingHook == nil {
			return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
		}
		spec.BeforeValue = "{}"
		spec.AfterValue = mustJSON(*input.ForeshadowingHook)
		spec.Evidence = input.Evidence
	default:
		return revisiondomain.PatchOperationSpec{}, revisiondomain.ErrInvalidOperation
	}
	return spec, nil
}

func (s *Service) editedOperationValueLocked(
	ctx context.Context,
	operation revisiondomain.PatchOperation,
	input ChangeOperationEditInput,
) (string, error) {
	switch operation.Kind {
	case revisiondomain.OperationReplaceChapterBody:
		return input.ChapterBody, nil
	case revisiondomain.OperationUpdateOutlineNode:
		if input.Outline == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		current, err := s.outlineNodeLocked(ctx, operation.TargetID)
		if err != nil {
			return "", err
		}
		if _, err := current.UpdateContent(
			input.Outline.Title, input.Outline.Summary, input.Outline.Goal,
			input.Outline.Conflict, input.Outline.Outcome, s.clock.Now(),
		); err != nil {
			return "", err
		}
		return mustJSON(*input.Outline), nil
	case revisiondomain.OperationUpdateCharacter:
		if input.Character == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		current, err := s.active.store.Character(ctx, operation.TargetID)
		if err != nil {
			return "", err
		}
		if _, err := updateCharacterFromInput(current, *input.Character, s.clock.Now()); err != nil {
			return "", err
		}
		return mustJSON(*input.Character), nil
	case revisiondomain.OperationUpdateFact:
		if input.Fact == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		current, err := s.active.store.Fact(ctx, operation.TargetID)
		if err != nil {
			return "", err
		}
		if _, err := updateFactFromInput(current, *input.Fact, s.clock.Now()); err != nil {
			return "", err
		}
		return mustJSON(factMutableInput(*input.Fact)), nil
	case revisiondomain.OperationCreateOutlineNode:
		if input.Outline == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		return mustJSON(*input.Outline), nil
	case revisiondomain.OperationCreateCharacter:
		if input.Character == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		return mustJSON(*input.Character), nil
	case revisiondomain.OperationCreateRelationship:
		if input.Relationship == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		return mustJSON(*input.Relationship), nil
	case revisiondomain.OperationCreateWorldRule:
		if input.WorldRule == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		return mustJSON(*input.WorldRule), nil
	case revisiondomain.OperationCreateStoryEvent:
		if input.StoryEvent == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		return mustJSON(*input.StoryEvent), nil
	case revisiondomain.OperationCreateFact:
		if input.Fact == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		return mustJSON(factMutableInput(*input.Fact)), nil
	case revisiondomain.OperationCreateForeshadowingHook:
		if input.ForeshadowingHook == nil {
			return "", revisiondomain.ErrInvalidOperation
		}
		return mustJSON(*input.ForeshadowingHook), nil
	default:
		return "", revisiondomain.ErrInvalidOperation
	}
}

func (s *Service) outlineNodeLocked(ctx context.Context, id string) (storydomain.OutlineNode, error) {
	nodes, err := s.active.store.ListOutlineNodes(ctx)
	if err != nil {
		return storydomain.OutlineNode{}, err
	}
	for _, node := range nodes {
		if node.ID == id {
			return node, nil
		}
	}
	return storydomain.OutlineNode{}, fmt.Errorf("outline node does not exist: %s", id)
}

func updateCharacterFromInput(
	current worlddomain.Character,
	input CharacterInput,
	now time.Time,
) (worlddomain.Character, error) {
	return current.UpdateProfile(
		input.Name, input.Aliases, worlddomain.CharacterRole(input.Role), input.Summary,
		input.Motivation, input.Fear, input.Secret, input.Arc,
		worlddomain.CharacterStatus(input.Status), now,
	)
}

func updateFactFromInput(current canondomain.Fact, input FactInput, now time.Time) (canondomain.Fact, error) {
	return current.Update(
		input.Text, input.Category, canondomain.FactStatus(input.Status), input.Confidence,
		input.ValidFromEventID, input.ValidUntilEventID, now,
	)
}

func outlineContentFromDomain(node storydomain.OutlineNode) OutlineContentInput {
	return OutlineContentInput{
		Title: node.Title, Summary: node.Summary, Goal: node.Goal,
		Conflict: node.Conflict, Outcome: node.Outcome,
	}
}

func characterInputFromDomain(character worlddomain.Character) CharacterInput {
	aliases := append([]string(nil), character.Aliases...)
	if aliases == nil {
		aliases = []string{}
	}
	return CharacterInput{
		Name: character.Name, Aliases: aliases, Role: string(character.Role),
		Summary: character.Summary, Motivation: character.Motivation, Fear: character.Fear,
		Secret: character.Secret, Arc: character.Arc, Status: string(character.Status),
	}
}

func factInputFromDomain(fact canondomain.Fact) factChangeValue {
	return factChangeValue{
		Text: fact.Text, Category: fact.Category, Status: string(fact.Status),
		Confidence: fact.Confidence, ValidFromEventID: fact.ValidFromEventID,
		ValidUntilEventID: fact.ValidUntilEventID,
	}
}

func factMutableInput(input FactInput) factChangeValue {
	return factChangeValue{
		Text: input.Text, Category: input.Category, Status: input.Status,
		Confidence: input.Confidence, ValidFromEventID: input.ValidFromEventID,
		ValidUntilEventID: input.ValidUntilEventID,
	}
}

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}
