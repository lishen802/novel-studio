package workspace

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/license"
	canondomain "github.com/lishen802/novel-studio/internal/domain/canon"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	timelinedomain "github.com/lishen802/novel-studio/internal/domain/timeline"
)

type StoryEventInput struct {
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Cause          string `json:"cause"`
	Result         string `json:"result"`
	StoryTime      string `json:"storyTime"`
	StoryOrder     int64  `json:"storyOrder"`
	NarrativeOrder int    `json:"narrativeOrder"`
	Status         string `json:"status"`
}

type StoryEventLinkInput struct {
	EventID    string `json:"eventId"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Role       string `json:"role"`
}

type FactInput struct {
	Text              string  `json:"text"`
	Category          string  `json:"category"`
	Origin            string  `json:"origin"`
	Status            string  `json:"status"`
	Confidence        float64 `json:"confidence"`
	ValidFromEventID  string  `json:"validFromEventId"`
	ValidUntilEventID string  `json:"validUntilEventId"`
}

type FactEvidenceInput struct {
	FactID          string `json:"factId"`
	ChapterID       string `json:"chapterId"`
	Quote           string `json:"quote"`
	StartOffset     int    `json:"startOffset"`
	EndOffset       int    `json:"endOffset"`
	ChapterRevision int64  `json:"chapterRevision"`
}

type FactScopeInput struct {
	FactID     string `json:"factId"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
}

type FactConflictInput struct {
	FactID      string `json:"factId"`
	OtherFactID string `json:"otherFactId"`
	Reason      string `json:"reason"`
}

type CharacterKnowledgeInput struct {
	CharacterID            string `json:"characterId"`
	FactID                 string `json:"factId"`
	State                  string `json:"state"`
	BeliefText             string `json:"beliefText"`
	AvailableFromChapterID string `json:"availableFromChapterId"`
	Notes                  string `json:"notes"`
}

type StoryEventView struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Cause          string `json:"cause"`
	Result         string `json:"result"`
	StoryTime      string `json:"storyTime"`
	StoryOrder     int64  `json:"storyOrder"`
	NarrativeOrder int    `json:"narrativeOrder"`
	Status         string `json:"status"`
}

type StoryEventLinkView struct {
	EventID    string `json:"eventId"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Role       string `json:"role"`
}

type FactView struct {
	ID                string  `json:"id"`
	Text              string  `json:"text"`
	Category          string  `json:"category"`
	Origin            string  `json:"origin"`
	Status            string  `json:"status"`
	Confidence        float64 `json:"confidence"`
	ValidFromEventID  string  `json:"validFromEventId"`
	ValidUntilEventID string  `json:"validUntilEventId"`
	CreatedBy         string  `json:"createdBy"`
	ConfirmedBy       string  `json:"confirmedBy"`
}

type FactEvidenceView struct {
	ID              string `json:"id"`
	FactID          string `json:"factId"`
	ChapterID       string `json:"chapterId"`
	Quote           string `json:"quote"`
	StartOffset     int    `json:"startOffset"`
	EndOffset       int    `json:"endOffset"`
	ChapterRevision int64  `json:"chapterRevision"`
}

type FactScopeView struct {
	ID         string `json:"id"`
	FactID     string `json:"factId"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
}

type FactConflictView struct {
	ID          string `json:"id"`
	FactID      string `json:"factId"`
	OtherFactID string `json:"otherFactId"`
	Reason      string `json:"reason"`
	Status      string `json:"status"`
}

type CharacterKnowledgeView struct {
	ID                     string `json:"id"`
	CharacterID            string `json:"characterId"`
	FactID                 string `json:"factId"`
	State                  string `json:"state"`
	BeliefText             string `json:"beliefText"`
	AvailableFromChapterID string `json:"availableFromChapterId"`
	Notes                  string `json:"notes"`
}

type ContinuityView struct {
	Events            []StoryEventView       `json:"events"`
	EventLinks        []StoryEventLinkView   `json:"eventLinks"`
	Facts             []FactView             `json:"facts"`
	Evidence          []FactEvidenceView     `json:"evidence"`
	Scopes            []FactScopeView        `json:"scopes"`
	Conflicts         []FactConflictView     `json:"conflicts"`
	ConsistencyIssues []ConsistencyIssueView `json:"consistencyIssues"`
}

func (s *Service) Continuity(ctx context.Context) (ContinuityView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return ContinuityView{}, ErrNoActiveProject
	}
	events, err := s.active.store.ListStoryEvents(ctx)
	if err != nil {
		return ContinuityView{}, fmt.Errorf("list story events: %w", err)
	}
	links, err := s.active.store.ListStoryEventLinks(ctx)
	if err != nil {
		return ContinuityView{}, fmt.Errorf("list story event links: %w", err)
	}
	facts, err := s.active.store.ListFacts(ctx)
	if err != nil {
		return ContinuityView{}, fmt.Errorf("list facts: %w", err)
	}
	evidence, err := s.active.store.ListFactEvidence(ctx)
	if err != nil {
		return ContinuityView{}, fmt.Errorf("list fact evidence: %w", err)
	}
	scopes, err := s.active.store.ListFactScopes(ctx)
	if err != nil {
		return ContinuityView{}, fmt.Errorf("list fact scopes: %w", err)
	}
	conflicts, err := s.active.store.ListFactConflicts(ctx)
	if err != nil {
		return ContinuityView{}, fmt.Errorf("list fact conflicts: %w", err)
	}
	chapters, err := s.active.store.ListChapters(ctx)
	if err != nil {
		return ContinuityView{}, fmt.Errorf("list chapters for continuity: %w", err)
	}
	return continuityView(events, links, facts, evidence, scopes, conflicts, chapters), nil
}

func (s *Service) CreateStoryEvent(ctx context.Context, input StoryEventInput) (StoryEventView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return StoryEventView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return StoryEventView{}, err
	}
	status := timelinedomain.EventStatus(input.Status)
	if status == "" {
		status = timelinedomain.EventPlanned
	}
	event, err := timelinedomain.NewStoryEvent(s.ids.NewID(), project.ID, input.Title, input.StoryTime, input.StoryOrder, input.NarrativeOrder, status, s.clock.Now())
	if err != nil {
		return StoryEventView{}, err
	}
	event, err = event.Update(input.Title, input.Summary, input.Cause, input.Result, input.StoryTime, input.StoryOrder, input.NarrativeOrder, status, s.clock.Now())
	if err != nil {
		return StoryEventView{}, err
	}
	if err := s.active.store.CreateStoryEvent(ctx, event); err != nil {
		return StoryEventView{}, fmt.Errorf("create story event: %w", err)
	}
	return storyEventView(event), nil
}

func (s *Service) UpdateStoryEvent(ctx context.Context, id string, input StoryEventInput) (StoryEventView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return StoryEventView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return StoryEventView{}, err
	}
	event, err := s.active.store.StoryEvent(ctx, strings.TrimSpace(id))
	if err != nil {
		return StoryEventView{}, timelinedomain.ErrInvalidReference
	}
	updated, err := event.Update(input.Title, input.Summary, input.Cause, input.Result, input.StoryTime, input.StoryOrder, input.NarrativeOrder, timelinedomain.EventStatus(input.Status), s.clock.Now())
	if err != nil {
		return StoryEventView{}, err
	}
	if err := s.active.store.UpdateStoryEvent(ctx, updated); err != nil {
		return StoryEventView{}, fmt.Errorf("update story event: %w", err)
	}
	return storyEventView(updated), nil
}

func (s *Service) ArchiveStoryEvent(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error { return s.active.store.ArchiveStoryEvent(ctx, strings.TrimSpace(id), s.clock.Now()) })
}

func (s *Service) LinkStoryEvent(ctx context.Context, input StoryEventLinkInput) (StoryEventLinkView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return StoryEventLinkView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return StoryEventLinkView{}, err
	}
	event, err := s.active.store.StoryEvent(ctx, strings.TrimSpace(input.EventID))
	if err != nil {
		return StoryEventLinkView{}, timelinedomain.ErrInvalidReference
	}
	targetType := timelinedomain.EventTargetType(input.TargetType)
	if err := s.validateContinuityTargetLocked(ctx, event.ProjectID, string(targetType), input.TargetID); err != nil {
		return StoryEventLinkView{}, err
	}
	link, err := timelinedomain.NewEventLink(event, targetType, input.TargetID, input.Role, s.clock.Now())
	if err != nil {
		return StoryEventLinkView{}, err
	}
	if err := s.active.store.UpsertStoryEventLink(ctx, link); err != nil {
		return StoryEventLinkView{}, fmt.Errorf("link story event: %w", err)
	}
	return storyEventLinkView(link), nil
}

func (s *Service) UnlinkStoryEvent(ctx context.Context, eventID, targetType, targetID string) error {
	return s.withStoryWrite(ctx, func() error {
		return s.active.store.DeleteStoryEventLink(ctx, strings.TrimSpace(eventID), timelinedomain.EventTargetType(targetType), strings.TrimSpace(targetID))
	})
}

func (s *Service) CreateFact(ctx context.Context, input FactInput) (FactView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return FactView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return FactView{}, err
	}
	if err := s.validateFactIntervalLocked(ctx, project.ID, input.ValidFromEventID, input.ValidUntilEventID); err != nil {
		return FactView{}, err
	}
	fact, err := canondomain.NewFact(s.ids.NewID(), project.ID, input.Text, input.Category, canondomain.FactOrigin(input.Origin), canondomain.FactStatus(input.Status), input.Confidence, input.ValidFromEventID, input.ValidUntilEventID, s.clock.Now())
	if err != nil {
		return FactView{}, err
	}
	if err := s.active.store.CreateFact(ctx, fact); err != nil {
		return FactView{}, fmt.Errorf("create fact: %w", err)
	}
	return factView(fact), nil
}

func (s *Service) UpdateFact(ctx context.Context, id string, input FactInput) (FactView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return FactView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return FactView{}, err
	}
	fact, err := s.active.store.Fact(ctx, strings.TrimSpace(id))
	if err != nil {
		return FactView{}, canondomain.ErrInvalidReference
	}
	if err := s.validateFactIntervalLocked(ctx, fact.ProjectID, input.ValidFromEventID, input.ValidUntilEventID); err != nil {
		return FactView{}, err
	}
	updated, err := fact.Update(input.Text, input.Category, canondomain.FactStatus(input.Status), input.Confidence, input.ValidFromEventID, input.ValidUntilEventID, s.clock.Now())
	if err != nil {
		return FactView{}, err
	}
	if err := s.active.store.UpdateFact(ctx, updated); err != nil {
		return FactView{}, fmt.Errorf("update fact: %w", err)
	}
	return factView(updated), nil
}

func (s *Service) ConfirmFact(ctx context.Context, id string) (FactView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return FactView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return FactView{}, err
	}
	fact, err := s.active.store.Fact(ctx, strings.TrimSpace(id))
	if err != nil {
		return FactView{}, canondomain.ErrInvalidReference
	}
	confirmed, err := fact.Confirm("author", s.clock.Now())
	if err != nil {
		return FactView{}, err
	}
	if err := s.active.store.UpdateFact(ctx, confirmed); err != nil {
		return FactView{}, fmt.Errorf("confirm fact: %w", err)
	}
	return factView(confirmed), nil
}

func (s *Service) ArchiveFact(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error { return s.active.store.ArchiveFact(ctx, strings.TrimSpace(id), s.clock.Now()) })
}

func (s *Service) CharacterKnowledge(ctx context.Context) ([]CharacterKnowledgeView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return nil, ErrNoActiveProject
	}
	knowledge, err := s.active.store.ListCharacterKnowledge(ctx)
	if err != nil {
		return nil, fmt.Errorf("list character knowledge: %w", err)
	}
	views := make([]CharacterKnowledgeView, 0, len(knowledge))
	for _, value := range knowledge {
		views = append(views, characterKnowledgeView(value))
	}
	sort.Slice(views, func(i, j int) bool { return views[i].ID < views[j].ID })
	return views, nil
}

func (s *Service) CreateCharacterKnowledge(ctx context.Context, input CharacterKnowledgeInput) (CharacterKnowledgeView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return CharacterKnowledgeView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return CharacterKnowledgeView{}, err
	}
	character, err := s.active.store.Character(ctx, strings.TrimSpace(input.CharacterID))
	if err != nil || character.ProjectID != project.ID {
		return CharacterKnowledgeView{}, canondomain.ErrInvalidReference
	}
	state := canondomain.KnowledgeState(input.State)
	factID := strings.TrimSpace(input.FactID)
	if factID != "" {
		fact, err := s.active.store.Fact(ctx, factID)
		if err != nil || fact.ProjectID != project.ID {
			return CharacterKnowledgeView{}, canondomain.ErrInvalidReference
		}
		if state != canondomain.KnowledgeMisinformed && fact.Status != canondomain.StatusAuthorConfirmed && fact.Status != canondomain.StatusTextEvidenced {
			return CharacterKnowledgeView{}, canondomain.ErrInvalidReference
		}
	}
	chapterID := strings.TrimSpace(input.AvailableFromChapterID)
	if chapterID != "" {
		chapter, err := s.active.store.Chapter(ctx, chapterID)
		if err != nil || chapter.ProjectID != project.ID {
			return CharacterKnowledgeView{}, canondomain.ErrInvalidReference
		}
	}
	knowledge, err := canondomain.NewCharacterKnowledge(s.ids.NewID(), project.ID, character.ID, factID, state, input.BeliefText, chapterID, input.Notes, s.clock.Now())
	if err != nil {
		return CharacterKnowledgeView{}, err
	}
	if err := s.active.store.CreateCharacterKnowledge(ctx, knowledge); err != nil {
		return CharacterKnowledgeView{}, fmt.Errorf("create character knowledge: %w", err)
	}
	return characterKnowledgeView(knowledge), nil
}

func (s *Service) ArchiveCharacterKnowledge(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error {
		return s.active.store.ArchiveCharacterKnowledge(ctx, strings.TrimSpace(id), s.clock.Now())
	})
}

func (s *Service) AddFactEvidence(ctx context.Context, input FactEvidenceInput) (FactEvidenceView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return FactEvidenceView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return FactEvidenceView{}, err
	}
	fact, err := s.active.store.Fact(ctx, strings.TrimSpace(input.FactID))
	if err != nil {
		return FactEvidenceView{}, canondomain.ErrInvalidReference
	}
	chapter, err := s.active.store.Chapter(ctx, strings.TrimSpace(input.ChapterID))
	if err != nil || chapter.ProjectID != fact.ProjectID {
		return FactEvidenceView{}, canondomain.ErrInvalidReference
	}
	evidence, err := canondomain.NewEvidence(s.ids.NewID(), fact.ProjectID, fact.ID, chapter.ID, input.Quote, input.StartOffset, input.EndOffset, input.ChapterRevision, s.clock.Now())
	if err != nil {
		return FactEvidenceView{}, err
	}
	runes := []rune(chapter.Body)
	if input.ChapterRevision != chapter.Revision || input.EndOffset > len(runes) || string(runes[input.StartOffset:input.EndOffset]) != input.Quote {
		return FactEvidenceView{}, canondomain.ErrInvalidEvidence
	}
	if err := s.active.store.CreateFactEvidence(ctx, evidence); err != nil {
		return FactEvidenceView{}, fmt.Errorf("add fact evidence: %w", err)
	}
	return factEvidenceView(evidence), nil
}

func (s *Service) RemoveFactEvidence(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error { return s.active.store.DeleteFactEvidence(ctx, strings.TrimSpace(id)) })
}

func (s *Service) AddFactScope(ctx context.Context, input FactScopeInput) (FactScopeView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return FactScopeView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return FactScopeView{}, err
	}
	fact, err := s.active.store.Fact(ctx, strings.TrimSpace(input.FactID))
	if err != nil {
		return FactScopeView{}, canondomain.ErrInvalidReference
	}
	if err := s.validateContinuityTargetLocked(ctx, fact.ProjectID, input.TargetType, input.TargetID); err != nil {
		return FactScopeView{}, err
	}
	scope, err := canondomain.NewScope(s.ids.NewID(), fact.ProjectID, fact.ID, canondomain.ScopeType(input.TargetType), input.TargetID, s.clock.Now())
	if err != nil {
		return FactScopeView{}, err
	}
	if err := s.active.store.CreateFactScope(ctx, scope); err != nil {
		return FactScopeView{}, fmt.Errorf("add fact scope: %w", err)
	}
	return factScopeView(scope), nil
}

func (s *Service) RemoveFactScope(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error { return s.active.store.DeleteFactScope(ctx, strings.TrimSpace(id)) })
}

func (s *Service) CreateFactConflict(ctx context.Context, input FactConflictInput) (FactConflictView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return FactConflictView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return FactConflictView{}, err
	}
	fact, err := s.active.store.Fact(ctx, strings.TrimSpace(input.FactID))
	if err != nil {
		return FactConflictView{}, canondomain.ErrInvalidReference
	}
	other, err := s.active.store.Fact(ctx, strings.TrimSpace(input.OtherFactID))
	if err != nil || other.ProjectID != fact.ProjectID {
		return FactConflictView{}, canondomain.ErrInvalidReference
	}
	conflict, err := canondomain.NewConflict(s.ids.NewID(), fact.ProjectID, fact.ID, other.ID, input.Reason, s.clock.Now())
	if err != nil {
		return FactConflictView{}, err
	}
	if err := s.active.store.CreateFactConflict(ctx, conflict); err != nil {
		return FactConflictView{}, fmt.Errorf("create fact conflict: %w", err)
	}
	return factConflictView(conflict), nil
}

func (s *Service) ResolveFactConflict(ctx context.Context, id string) (FactConflictView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return FactConflictView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return FactConflictView{}, err
	}
	conflicts, err := s.active.store.ListFactConflicts(ctx)
	if err != nil {
		return FactConflictView{}, err
	}
	for _, conflict := range conflicts {
		if conflict.ID == strings.TrimSpace(id) {
			resolved, err := conflict.Resolve(s.clock.Now())
			if err != nil {
				return FactConflictView{}, err
			}
			if err := s.active.store.UpdateFactConflict(ctx, resolved); err != nil {
				return FactConflictView{}, err
			}
			return factConflictView(resolved), nil
		}
	}
	return FactConflictView{}, canondomain.ErrInvalidReference
}

func (s *Service) DeleteFactConflict(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error { return s.active.store.DeleteFactConflict(ctx, strings.TrimSpace(id)) })
}

func (s *Service) validateFactIntervalLocked(ctx context.Context, projectID, fromID, untilID string) error {
	fromID, untilID = strings.TrimSpace(fromID), strings.TrimSpace(untilID)
	var from, until *timelinedomain.StoryEvent
	if fromID != "" {
		value, err := s.active.store.StoryEvent(ctx, fromID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
		from = &value
	}
	if untilID != "" {
		value, err := s.active.store.StoryEvent(ctx, untilID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
		until = &value
	}
	if from != nil && until != nil && from.StoryOrder > until.StoryOrder {
		return canondomain.ErrInvalidReference
	}
	return nil
}

func (s *Service) validateContinuityTargetLocked(ctx context.Context, projectID, targetType, targetID string) error {
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return canondomain.ErrInvalidReference
	}
	switch targetType {
	case "chapter":
		value, err := s.active.store.Chapter(ctx, targetID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
	case "character":
		value, err := s.active.store.Character(ctx, targetID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
	case "faction":
		value, err := s.active.store.Faction(ctx, targetID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
	case "location":
		value, err := s.active.store.Location(ctx, targetID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
	case "item":
		value, err := s.active.store.Item(ctx, targetID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
	case "region":
		value, err := s.active.store.WorldRegion(ctx, targetID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
	case "rule":
		value, err := s.active.store.WorldRule(ctx, targetID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
	case "event":
		value, err := s.active.store.StoryEvent(ctx, targetID)
		if err != nil || value.ProjectID != projectID {
			return canondomain.ErrInvalidReference
		}
	default:
		return canondomain.ErrInvalidReference
	}
	return nil
}

func continuityView(events []timelinedomain.StoryEvent, links []timelinedomain.EventLink, facts []canondomain.Fact, evidence []canondomain.Evidence, scopes []canondomain.Scope, conflicts []canondomain.Conflict, chapters []projectdomain.Chapter) ContinuityView {
	view := ContinuityView{Events: make([]StoryEventView, 0, len(events)), EventLinks: make([]StoryEventLinkView, 0, len(links)), Facts: make([]FactView, 0, len(facts)), Evidence: make([]FactEvidenceView, 0, len(evidence)), Scopes: make([]FactScopeView, 0, len(scopes)), Conflicts: make([]FactConflictView, 0, len(conflicts))}
	for _, value := range events {
		view.Events = append(view.Events, storyEventView(value))
	}
	for _, value := range links {
		view.EventLinks = append(view.EventLinks, storyEventLinkView(value))
	}
	for _, value := range facts {
		view.Facts = append(view.Facts, factView(value))
	}
	for _, value := range evidence {
		view.Evidence = append(view.Evidence, factEvidenceView(value))
	}
	for _, value := range scopes {
		view.Scopes = append(view.Scopes, factScopeView(value))
	}
	for _, value := range conflicts {
		view.Conflicts = append(view.Conflicts, factConflictView(value))
	}
	view.ConsistencyIssues = continuityIssues(events, facts, evidence, conflicts, chapters)
	sort.Slice(view.Events, func(i, j int) bool {
		if view.Events[i].StoryOrder == view.Events[j].StoryOrder {
			return view.Events[i].NarrativeOrder < view.Events[j].NarrativeOrder
		}
		return view.Events[i].StoryOrder < view.Events[j].StoryOrder
	})
	return view
}

func continuityIssues(events []timelinedomain.StoryEvent, facts []canondomain.Fact, evidence []canondomain.Evidence, conflicts []canondomain.Conflict, chapters []projectdomain.Chapter) []ConsistencyIssueView {
	issues := make([]ConsistencyIssueView, 0)
	orders := make(map[int][]string)
	for _, event := range events {
		orders[event.NarrativeOrder] = append(orders[event.NarrativeOrder], event.ID)
	}
	for order, ids := range orders {
		if len(ids) > 1 {
			issues = append(issues, ConsistencyIssueView{Code: "duplicate_narrative_order", Severity: "warning", Message: fmt.Sprintf("叙述顺序 #%d 被多个事件占用。", order), EntityIDs: ids})
		}
	}
	evidenceByFact := make(map[string][]canondomain.Evidence)
	for _, value := range evidence {
		evidenceByFact[value.FactID] = append(evidenceByFact[value.FactID], value)
	}
	openConflict := make(map[string]bool)
	for _, value := range conflicts {
		if value.Status == canondomain.ConflictOpen {
			openConflict[value.FactID], openConflict[value.OtherFactID] = true, true
		}
	}
	chapterByID := make(map[string]projectdomain.Chapter)
	for _, chapter := range chapters {
		chapterByID[chapter.ID] = chapter
	}
	for _, fact := range facts {
		if fact.Status == canondomain.StatusTextEvidenced && len(evidenceByFact[fact.ID]) == 0 {
			issues = append(issues, ConsistencyIssueView{Code: "missing_fact_evidence", Severity: "warning", Message: fmt.Sprintf("文本事实“%s”尚未关联原文证据。", fact.Text), EntityIDs: []string{fact.ID}})
		}
		if fact.Status == canondomain.StatusConflicted && !openConflict[fact.ID] {
			issues = append(issues, ConsistencyIssueView{Code: "missing_fact_conflict", Severity: "warning", Message: fmt.Sprintf("冲突事实“%s”尚未指定相冲突的事实。", fact.Text), EntityIDs: []string{fact.ID}})
		}
	}
	for _, value := range evidence {
		chapter, ok := chapterByID[value.ChapterID]
		runes := []rune(chapter.Body)
		validAnchor := ok && value.StartOffset >= 0 && value.EndOffset <= len(runes) && value.StartOffset < value.EndOffset && string(runes[value.StartOffset:value.EndOffset]) == value.Quote
		if !ok || chapter.Revision != value.ChapterRevision || !validAnchor {
			issues = append(issues, ConsistencyIssueView{Code: "stale_fact_evidence", Severity: "warning", Message: "事实证据对应的章节正文已经变化，请重新定位。", EntityIDs: []string{value.FactID, value.ID, value.ChapterID}})
		}
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Code+issues[i].Message < issues[j].Code+issues[j].Message })
	return issues
}

func storyEventView(value timelinedomain.StoryEvent) StoryEventView {
	return StoryEventView{ID: value.ID, Title: value.Title, Summary: value.Summary, Cause: value.Cause, Result: value.Result, StoryTime: value.StoryTime, StoryOrder: value.StoryOrder, NarrativeOrder: value.NarrativeOrder, Status: string(value.Status)}
}
func storyEventLinkView(value timelinedomain.EventLink) StoryEventLinkView {
	return StoryEventLinkView{EventID: value.EventID, TargetType: string(value.TargetType), TargetID: value.TargetID, Role: value.Role}
}
func factView(value canondomain.Fact) FactView {
	return FactView{ID: value.ID, Text: value.Text, Category: value.Category, Origin: string(value.Origin), Status: string(value.Status), Confidence: value.Confidence, ValidFromEventID: value.ValidFromEventID, ValidUntilEventID: value.ValidUntilEventID, CreatedBy: value.CreatedBy, ConfirmedBy: value.ConfirmedBy}
}
func factEvidenceView(value canondomain.Evidence) FactEvidenceView {
	return FactEvidenceView{ID: value.ID, FactID: value.FactID, ChapterID: value.ChapterID, Quote: value.Quote, StartOffset: value.StartOffset, EndOffset: value.EndOffset, ChapterRevision: value.ChapterRevision}
}
func factScopeView(value canondomain.Scope) FactScopeView {
	return FactScopeView{ID: value.ID, FactID: value.FactID, TargetType: string(value.TargetType), TargetID: value.TargetID}
}
func factConflictView(value canondomain.Conflict) FactConflictView {
	return FactConflictView{ID: value.ID, FactID: value.FactID, OtherFactID: value.OtherFactID, Reason: value.Reason, Status: string(value.Status)}
}
func characterKnowledgeView(value canondomain.CharacterKnowledge) CharacterKnowledgeView {
	return CharacterKnowledgeView{ID: value.ID, CharacterID: value.CharacterID, FactID: value.FactID, State: string(value.State), BeliefText: value.BeliefText, AvailableFromChapterID: value.AvailableFromChapterID, Notes: value.Notes}
}
