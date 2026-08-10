package workspace

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/license"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

type CharacterInput struct {
	Name       string   `json:"name"`
	Aliases    []string `json:"aliases"`
	Role       string   `json:"role"`
	Summary    string   `json:"summary"`
	Motivation string   `json:"motivation"`
	Fear       string   `json:"fear"`
	Secret     string   `json:"secret"`
	Arc        string   `json:"arc"`
	Status     string   `json:"status"`
}

type FactionInput struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Goal    string `json:"goal"`
	Status  string `json:"status"`
}

type MembershipInput struct {
	CharacterID string `json:"characterId"`
	FactionID   string `json:"factionId"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

type RelationshipInput struct {
	SourceCharacterID string `json:"sourceCharacterId"`
	TargetCharacterID string `json:"targetCharacterId"`
	Kind              string `json:"kind"`
	Label             string `json:"label"`
	Strength          int    `json:"strength"`
	PublicState       string `json:"publicState"`
	TrueState         string `json:"trueState"`
}

type AppearanceInput struct {
	CharacterID string `json:"characterId"`
	TargetID    string `json:"targetId"`
	Role        string `json:"role"`
}

type CharacterView struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Aliases    []string `json:"aliases"`
	Role       string   `json:"role"`
	Summary    string   `json:"summary"`
	Motivation string   `json:"motivation"`
	Fear       string   `json:"fear"`
	Secret     string   `json:"secret"`
	Arc        string   `json:"arc"`
	Status     string   `json:"status"`
}

type FactionView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Goal    string `json:"goal"`
	Status  string `json:"status"`
}

type MembershipView struct {
	CharacterID string `json:"characterId"`
	FactionID   string `json:"factionId"`
	Role        string `json:"role"`
	Status      string `json:"status"`
}

type RelationshipView struct {
	ID                string `json:"id"`
	SourceCharacterID string `json:"sourceCharacterId"`
	TargetCharacterID string `json:"targetCharacterId"`
	Kind              string `json:"kind"`
	Label             string `json:"label"`
	Strength          int    `json:"strength"`
	PublicState       string `json:"publicState"`
	TrueState         string `json:"trueState"`
}

type ChapterAppearanceView struct {
	CharacterID string `json:"characterId"`
	ChapterID   string `json:"chapterId"`
	Role        string `json:"role"`
}

type SceneAppearanceView struct {
	CharacterID string `json:"characterId"`
	SceneCardID string `json:"sceneCardId"`
	Role        string `json:"role"`
}

type ConsistencyIssueView struct {
	Code      string   `json:"code"`
	Severity  string   `json:"severity"`
	Message   string   `json:"message"`
	EntityIDs []string `json:"entityIds"`
}

type StoryWorldView struct {
	Characters         []CharacterView         `json:"characters"`
	Factions           []FactionView           `json:"factions"`
	Memberships        []MembershipView        `json:"memberships"`
	Relationships      []RelationshipView      `json:"relationships"`
	ChapterAppearances []ChapterAppearanceView `json:"chapterAppearances"`
	SceneAppearances   []SceneAppearanceView   `json:"sceneAppearances"`
	ConsistencyIssues  []ConsistencyIssueView  `json:"consistencyIssues"`
}

func (s *Service) StoryWorld(ctx context.Context) (StoryWorldView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return StoryWorldView{}, ErrNoActiveProject
	}
	characters, err := s.active.store.ListCharacters(ctx)
	if err != nil {
		return StoryWorldView{}, fmt.Errorf("list characters: %w", err)
	}
	factions, err := s.active.store.ListFactions(ctx)
	if err != nil {
		return StoryWorldView{}, fmt.Errorf("list factions: %w", err)
	}
	memberships, err := s.active.store.ListMemberships(ctx)
	if err != nil {
		return StoryWorldView{}, fmt.Errorf("list memberships: %w", err)
	}
	relationships, err := s.active.store.ListRelationships(ctx)
	if err != nil {
		return StoryWorldView{}, fmt.Errorf("list relationships: %w", err)
	}
	chapterAppearances, err := s.active.store.ListChapterAppearances(ctx)
	if err != nil {
		return StoryWorldView{}, fmt.Errorf("list chapter appearances: %w", err)
	}
	sceneAppearances, err := s.active.store.ListSceneAppearances(ctx)
	if err != nil {
		return StoryWorldView{}, fmt.Errorf("list scene appearances: %w", err)
	}
	return storyWorldView(characters, factions, memberships, relationships, chapterAppearances, sceneAppearances), nil
}

func (s *Service) CreateCharacter(ctx context.Context, input CharacterInput) (CharacterView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return CharacterView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return CharacterView{}, err
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return CharacterView{}, fmt.Errorf("read active project: %w", err)
	}
	character, err := worlddomain.NewCharacter(s.ids.NewID(), project.ID, input.Name, input.Aliases, worlddomain.CharacterRole(input.Role), input.Motivation, input.Fear, input.Secret, input.Arc, s.clock.Now())
	if err != nil {
		return CharacterView{}, err
	}
	status := worlddomain.CharacterStatus(input.Status)
	if status == "" {
		status = worlddomain.CharacterActive
	}
	character, err = character.UpdateProfile(input.Name, input.Aliases, worlddomain.CharacterRole(input.Role), input.Summary, input.Motivation, input.Fear, input.Secret, input.Arc, status, s.clock.Now())
	if err != nil {
		return CharacterView{}, err
	}
	if err := s.active.store.CreateCharacter(ctx, character); err != nil {
		return CharacterView{}, fmt.Errorf("create character: %w", err)
	}
	return characterView(character), nil
}

func (s *Service) UpdateCharacter(ctx context.Context, id string, input CharacterInput) (CharacterView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return CharacterView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return CharacterView{}, err
	}
	character, err := s.active.store.Character(ctx, strings.TrimSpace(id))
	if err != nil {
		return CharacterView{}, worlddomain.ErrInvalidReference
	}
	status := worlddomain.CharacterStatus(input.Status)
	if status == "" {
		status = character.Status
	}
	updated, err := character.UpdateProfile(input.Name, input.Aliases, worlddomain.CharacterRole(input.Role), input.Summary, input.Motivation, input.Fear, input.Secret, input.Arc, status, s.clock.Now())
	if err != nil {
		return CharacterView{}, err
	}
	if err := s.active.store.UpdateCharacter(ctx, updated); err != nil {
		return CharacterView{}, fmt.Errorf("update character: %w", err)
	}
	return characterView(updated), nil
}

func (s *Service) ArchiveCharacter(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error {
		if err := s.active.store.ArchiveCharacter(ctx, strings.TrimSpace(id), s.clock.Now()); err != nil {
			return fmt.Errorf("archive character: %w", err)
		}
		return nil
	})
}

func (s *Service) CreateFaction(ctx context.Context, input FactionInput) (FactionView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return FactionView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return FactionView{}, err
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return FactionView{}, fmt.Errorf("read active project: %w", err)
	}
	faction, err := worlddomain.NewFaction(s.ids.NewID(), project.ID, input.Name, input.Summary, input.Goal, s.clock.Now())
	if err != nil {
		return FactionView{}, err
	}
	status := worlddomain.FactionStatus(input.Status)
	if status == "" {
		status = worlddomain.FactionActive
	}
	faction, err = faction.UpdateProfile(input.Name, input.Summary, input.Goal, status, s.clock.Now())
	if err != nil {
		return FactionView{}, err
	}
	if err := s.active.store.CreateFaction(ctx, faction); err != nil {
		return FactionView{}, fmt.Errorf("create faction: %w", err)
	}
	return factionView(faction), nil
}

func (s *Service) UpdateFaction(ctx context.Context, id string, input FactionInput) (FactionView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return FactionView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return FactionView{}, err
	}
	faction, err := s.active.store.Faction(ctx, strings.TrimSpace(id))
	if err != nil {
		return FactionView{}, worlddomain.ErrInvalidReference
	}
	status := worlddomain.FactionStatus(input.Status)
	if status == "" {
		status = faction.Status
	}
	updated, err := faction.UpdateProfile(input.Name, input.Summary, input.Goal, status, s.clock.Now())
	if err != nil {
		return FactionView{}, err
	}
	if err := s.active.store.UpdateFaction(ctx, updated); err != nil {
		return FactionView{}, fmt.Errorf("update faction: %w", err)
	}
	return factionView(updated), nil
}

func (s *Service) ArchiveFaction(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error {
		if err := s.active.store.ArchiveFaction(ctx, strings.TrimSpace(id), s.clock.Now()); err != nil {
			return fmt.Errorf("archive faction: %w", err)
		}
		return nil
	})
}

func (s *Service) UpsertMembership(ctx context.Context, input MembershipInput) (MembershipView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return MembershipView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return MembershipView{}, err
	}
	character, err := s.active.store.Character(ctx, strings.TrimSpace(input.CharacterID))
	if err != nil {
		return MembershipView{}, worlddomain.ErrInvalidReference
	}
	faction, err := s.active.store.Faction(ctx, strings.TrimSpace(input.FactionID))
	if err != nil {
		return MembershipView{}, worlddomain.ErrInvalidReference
	}
	status := worlddomain.MembershipStatus(input.Status)
	if status == "" {
		status = worlddomain.MembershipActive
	}
	membership, err := worlddomain.NewMembership(character, faction, input.Role, status, s.clock.Now())
	if err != nil {
		return MembershipView{}, err
	}
	if err := s.active.store.UpsertMembership(ctx, membership); err != nil {
		return MembershipView{}, fmt.Errorf("upsert membership: %w", err)
	}
	return membershipView(membership), nil
}

func (s *Service) DeleteMembership(ctx context.Context, characterID, factionID string) error {
	return s.withStoryWrite(ctx, func() error {
		if err := s.active.store.DeleteMembership(ctx, strings.TrimSpace(characterID), strings.TrimSpace(factionID)); err != nil {
			return fmt.Errorf("delete membership: %w", err)
		}
		return nil
	})
}

func (s *Service) CreateRelationship(ctx context.Context, input RelationshipInput) (RelationshipView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return RelationshipView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return RelationshipView{}, err
	}
	source, err := s.active.store.Character(ctx, strings.TrimSpace(input.SourceCharacterID))
	if err != nil {
		return RelationshipView{}, worlddomain.ErrInvalidReference
	}
	target, err := s.active.store.Character(ctx, strings.TrimSpace(input.TargetCharacterID))
	if err != nil {
		return RelationshipView{}, worlddomain.ErrInvalidReference
	}
	relationship, err := worlddomain.NewRelationship(s.ids.NewID(), source, target, worlddomain.RelationshipKind(input.Kind), input.Label, input.Strength, input.PublicState, input.TrueState, s.clock.Now())
	if err != nil {
		return RelationshipView{}, err
	}
	if err := s.active.store.CreateRelationship(ctx, relationship); err != nil {
		return RelationshipView{}, fmt.Errorf("create relationship: %w", err)
	}
	return relationshipView(relationship), nil
}

func (s *Service) ArchiveRelationship(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error {
		if err := s.active.store.ArchiveRelationship(ctx, strings.TrimSpace(id), s.clock.Now()); err != nil {
			return fmt.Errorf("archive relationship: %w", err)
		}
		return nil
	})
}

func (s *Service) LinkCharacterToChapter(ctx context.Context, input AppearanceInput) error {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return err
	}
	character, err := s.active.store.Character(ctx, strings.TrimSpace(input.CharacterID))
	if err != nil {
		return worlddomain.ErrInvalidReference
	}
	chapter, err := s.active.store.Chapter(ctx, strings.TrimSpace(input.TargetID))
	if err != nil || chapter.ProjectID != character.ProjectID {
		return worlddomain.ErrInvalidReference
	}
	appearance, err := worlddomain.NewChapterAppearance(character.ProjectID, character.ID, chapter.ID, worlddomain.AppearanceRole(input.Role), s.clock.Now())
	if err != nil {
		return err
	}
	if err := s.active.store.AddChapterAppearance(ctx, appearance); err != nil {
		return fmt.Errorf("link character to chapter: %w", err)
	}
	return nil
}

func (s *Service) UnlinkCharacterFromChapter(ctx context.Context, characterID, chapterID string) error {
	return s.withStoryWrite(ctx, func() error {
		if err := s.active.store.RemoveChapterAppearance(ctx, strings.TrimSpace(characterID), strings.TrimSpace(chapterID)); err != nil {
			return fmt.Errorf("unlink character from chapter: %w", err)
		}
		return nil
	})
}

func (s *Service) LinkCharacterToScene(ctx context.Context, input AppearanceInput) error {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return err
	}
	character, err := s.active.store.Character(ctx, strings.TrimSpace(input.CharacterID))
	if err != nil {
		return worlddomain.ErrInvalidReference
	}
	card, found, err := findSceneCard(ctx, s.active.store, strings.TrimSpace(input.TargetID))
	if err != nil {
		return fmt.Errorf("find scene card: %w", err)
	}
	if !found || card.ProjectID != character.ProjectID {
		return worlddomain.ErrInvalidReference
	}
	appearance, err := worlddomain.NewSceneAppearance(character.ProjectID, character.ID, card.ID, worlddomain.AppearanceRole(input.Role), s.clock.Now())
	if err != nil {
		return err
	}
	if err := s.active.store.AddSceneAppearance(ctx, appearance); err != nil {
		return fmt.Errorf("link character to scene: %w", err)
	}
	return nil
}

func (s *Service) UnlinkCharacterFromScene(ctx context.Context, characterID, sceneCardID string) error {
	return s.withStoryWrite(ctx, func() error {
		if err := s.active.store.RemoveSceneAppearance(ctx, strings.TrimSpace(characterID), strings.TrimSpace(sceneCardID)); err != nil {
			return fmt.Errorf("unlink character from scene: %w", err)
		}
		return nil
	})
}

func (s *Service) withStoryWrite(ctx context.Context, operation func() error) error {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return err
	}
	return operation()
}

type sceneCardLister interface {
	ListChapters(context.Context) ([]projectdomain.Chapter, error)
	ListSceneCards(context.Context, string) ([]storydomain.SceneCard, error)
}

func findSceneCard(ctx context.Context, store sceneCardLister, id string) (storydomain.SceneCard, bool, error) {
	chapters, err := store.ListChapters(ctx)
	if err != nil {
		return storydomain.SceneCard{}, false, err
	}
	for _, chapter := range chapters {
		cards, err := store.ListSceneCards(ctx, chapter.ID)
		if err != nil {
			return storydomain.SceneCard{}, false, err
		}
		for _, card := range cards {
			if card.ID == id {
				return card, true, nil
			}
		}
	}
	return storydomain.SceneCard{}, false, nil
}

func storyWorldView(characters []worlddomain.Character, factions []worlddomain.Faction, memberships []worlddomain.Membership, relationships []worlddomain.Relationship, chapterAppearances []worlddomain.ChapterAppearance, sceneAppearances []worlddomain.SceneAppearance) StoryWorldView {
	view := StoryWorldView{
		Characters: make([]CharacterView, 0, len(characters)), Factions: make([]FactionView, 0, len(factions)),
		Memberships: make([]MembershipView, 0, len(memberships)), Relationships: make([]RelationshipView, 0, len(relationships)),
		ChapterAppearances: make([]ChapterAppearanceView, 0, len(chapterAppearances)), SceneAppearances: make([]SceneAppearanceView, 0, len(sceneAppearances)),
		ConsistencyIssues: consistencyIssues(characters),
	}
	for _, character := range characters {
		view.Characters = append(view.Characters, characterView(character))
	}
	for _, faction := range factions {
		view.Factions = append(view.Factions, factionView(faction))
	}
	for _, membership := range memberships {
		view.Memberships = append(view.Memberships, membershipView(membership))
	}
	for _, relationship := range relationships {
		view.Relationships = append(view.Relationships, relationshipView(relationship))
	}
	for _, appearance := range chapterAppearances {
		view.ChapterAppearances = append(view.ChapterAppearances, ChapterAppearanceView{CharacterID: appearance.CharacterID, ChapterID: appearance.ChapterID, Role: string(appearance.Role)})
	}
	for _, appearance := range sceneAppearances {
		view.SceneAppearances = append(view.SceneAppearances, SceneAppearanceView{CharacterID: appearance.CharacterID, SceneCardID: appearance.SceneCardID, Role: string(appearance.Role)})
	}
	sort.Slice(view.Characters, func(i, j int) bool {
		return view.Characters[i].Name < view.Characters[j].Name || view.Characters[i].Name == view.Characters[j].Name && view.Characters[i].ID < view.Characters[j].ID
	})
	sort.Slice(view.Factions, func(i, j int) bool { return view.Factions[i].Name < view.Factions[j].Name })
	return view
}

func consistencyIssues(characters []worlddomain.Character) []ConsistencyIssueView {
	byName := make(map[string][]string)
	for _, character := range characters {
		key := strings.ToLower(strings.TrimSpace(character.Name))
		byName[key] = append(byName[key], character.ID)
	}
	issues := make([]ConsistencyIssueView, 0)
	for name, ids := range byName {
		if name == "" || len(ids) < 2 {
			continue
		}
		sort.Strings(ids)
		issues = append(issues, ConsistencyIssueView{Code: "duplicate_character_name", Severity: "warning", Message: fmt.Sprintf("存在 %d 个同名角色“%s”，请确认是否需要合并或改名。", len(ids), name), EntityIDs: ids})
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Message < issues[j].Message })
	return issues
}

func characterView(character worlddomain.Character) CharacterView {
	aliases := append(make([]string, 0, len(character.Aliases)), character.Aliases...)
	return CharacterView{ID: character.ID, Name: character.Name, Aliases: aliases, Role: string(character.Role), Summary: character.Summary, Motivation: character.Motivation, Fear: character.Fear, Secret: character.Secret, Arc: character.Arc, Status: string(character.Status)}
}

func factionView(faction worlddomain.Faction) FactionView {
	return FactionView{ID: faction.ID, Name: faction.Name, Summary: faction.Summary, Goal: faction.Goal, Status: string(faction.Status)}
}

func membershipView(membership worlddomain.Membership) MembershipView {
	return MembershipView{CharacterID: membership.CharacterID, FactionID: membership.FactionID, Role: membership.Role, Status: string(membership.Status)}
}

func relationshipView(relationship worlddomain.Relationship) RelationshipView {
	return RelationshipView{ID: relationship.ID, SourceCharacterID: relationship.SourceCharacterID, TargetCharacterID: relationship.TargetCharacterID, Kind: string(relationship.Kind), Label: relationship.Label, Strength: relationship.Strength, PublicState: relationship.PublicState, TrueState: relationship.TrueState}
}
