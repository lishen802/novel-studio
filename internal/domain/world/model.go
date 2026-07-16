package world

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankValue       = errors.New("world value must not be blank")
	ErrInvalidRole      = errors.New("invalid character role")
	ErrInvalidStatus    = errors.New("invalid world status")
	ErrInvalidHierarchy = errors.New("invalid world hierarchy")
	ErrInvalidReference = errors.New("invalid world reference")
	ErrInvalidStrength  = errors.New("relationship strength must be between -100 and 100")
	ErrInvalidRelation  = errors.New("invalid relationship kind")
	ErrReferenced       = errors.New("world entity is still referenced")
)

type CharacterRole string

const (
	RoleProtagonist CharacterRole = "protagonist"
	RoleSupporting  CharacterRole = "supporting"
	RoleAntagonist  CharacterRole = "antagonist"
	RoleMinor       CharacterRole = "minor"
)

type CharacterStatus string

const (
	CharacterActive   CharacterStatus = "active"
	CharacterInactive CharacterStatus = "inactive"
	CharacterDeceased CharacterStatus = "deceased"
)

type Character struct {
	ID         string
	ProjectID  string
	Name       string
	Aliases    []string
	Role       CharacterRole
	Summary    string
	Motivation string
	Fear       string
	Secret     string
	Arc        string
	Status     CharacterStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

func NewCharacter(id, projectID, name string, aliases []string, role CharacterRole, motivation, fear, secret, arc string, now time.Time) (Character, error) {
	id, projectID, name = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(name)
	if id == "" || projectID == "" || name == "" {
		return Character{}, ErrBlankValue
	}
	if !validCharacterRole(role) {
		return Character{}, ErrInvalidRole
	}
	now = now.UTC()
	return Character{
		ID: id, ProjectID: projectID, Name: name, Aliases: normalizeAliases(name, aliases), Role: role,
		Motivation: strings.TrimSpace(motivation), Fear: strings.TrimSpace(fear), Secret: strings.TrimSpace(secret),
		Arc: strings.TrimSpace(arc), Status: CharacterActive, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func (c Character) UpdateProfile(name string, aliases []string, role CharacterRole, summary, motivation, fear, secret, arc string, status CharacterStatus, now time.Time) (Character, error) {
	name = strings.TrimSpace(name)
	if c.ID == "" || c.ProjectID == "" || c.DeletedAt != nil || name == "" {
		return Character{}, ErrBlankValue
	}
	if !validCharacterRole(role) {
		return Character{}, ErrInvalidRole
	}
	switch status {
	case CharacterActive, CharacterInactive, CharacterDeceased:
	default:
		return Character{}, ErrInvalidStatus
	}
	c.Name, c.Aliases, c.Role = name, normalizeAliases(name, aliases), role
	c.Summary, c.Motivation = strings.TrimSpace(summary), strings.TrimSpace(motivation)
	c.Fear, c.Secret, c.Arc = strings.TrimSpace(fear), strings.TrimSpace(secret), strings.TrimSpace(arc)
	c.Status, c.UpdatedAt = status, now.UTC()
	return c, nil
}

func validCharacterRole(role CharacterRole) bool {
	switch role {
	case RoleProtagonist, RoleSupporting, RoleAntagonist, RoleMinor:
		return true
	default:
		return false
	}
}

func normalizeAliases(name string, aliases []string) []string {
	result := make([]string, 0, len(aliases))
	seen := map[string]struct{}{name: {}}
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		if _, exists := seen[alias]; exists {
			continue
		}
		seen[alias] = struct{}{}
		result = append(result, alias)
	}
	return result
}

type FactionStatus string

const (
	FactionActive    FactionStatus = "active"
	FactionInactive  FactionStatus = "inactive"
	FactionDisbanded FactionStatus = "disbanded"
)

type Faction struct {
	ID        string
	ProjectID string
	Name      string
	Summary   string
	Goal      string
	Status    FactionStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewFaction(id, projectID, name, summary, goal string, now time.Time) (Faction, error) {
	id, projectID, name = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(name)
	if id == "" || projectID == "" || name == "" {
		return Faction{}, ErrBlankValue
	}
	now = now.UTC()
	return Faction{ID: id, ProjectID: projectID, Name: name, Summary: strings.TrimSpace(summary), Goal: strings.TrimSpace(goal), Status: FactionActive, CreatedAt: now, UpdatedAt: now}, nil
}

func (f Faction) UpdateProfile(name, summary, goal string, status FactionStatus, now time.Time) (Faction, error) {
	name = strings.TrimSpace(name)
	if f.ID == "" || f.ProjectID == "" || f.DeletedAt != nil || name == "" {
		return Faction{}, ErrBlankValue
	}
	switch status {
	case FactionActive, FactionInactive, FactionDisbanded:
	default:
		return Faction{}, ErrInvalidStatus
	}
	f.Name, f.Summary, f.Goal, f.Status, f.UpdatedAt = name, strings.TrimSpace(summary), strings.TrimSpace(goal), status, now.UTC()
	return f, nil
}

type MembershipStatus string

const (
	MembershipActive MembershipStatus = "active"
	MembershipFormer MembershipStatus = "former"
	MembershipSecret MembershipStatus = "secret"
)

type Membership struct {
	ProjectID   string
	CharacterID string
	FactionID   string
	Role        string
	Status      MembershipStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewMembership(character Character, faction Faction, role string, status MembershipStatus, now time.Time) (Membership, error) {
	role = strings.TrimSpace(role)
	if character.ID == "" || faction.ID == "" || character.DeletedAt != nil || faction.DeletedAt != nil || character.ProjectID != faction.ProjectID {
		return Membership{}, ErrInvalidReference
	}
	if role == "" {
		return Membership{}, ErrBlankValue
	}
	switch status {
	case MembershipActive, MembershipFormer, MembershipSecret:
	default:
		return Membership{}, ErrInvalidStatus
	}
	now = now.UTC()
	return Membership{ProjectID: character.ProjectID, CharacterID: character.ID, FactionID: faction.ID, Role: role, Status: status, CreatedAt: now, UpdatedAt: now}, nil
}

type RelationshipKind string

const (
	RelationshipAlly     RelationshipKind = "ally"
	RelationshipFamily   RelationshipKind = "family"
	RelationshipRomantic RelationshipKind = "romantic"
	RelationshipRival    RelationshipKind = "rival"
	RelationshipEnemy    RelationshipKind = "enemy"
	RelationshipMentor   RelationshipKind = "mentor"
	RelationshipOther    RelationshipKind = "other"
)

type Relationship struct {
	ID                string
	ProjectID         string
	SourceCharacterID string
	TargetCharacterID string
	Kind              RelationshipKind
	Label             string
	Strength          int
	PublicState       string
	TrueState         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

func NewRelationship(id string, source, target Character, kind RelationshipKind, label string, strength int, publicState, trueState string, now time.Time) (Relationship, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Relationship{}, ErrBlankValue
	}
	if source.ID == "" || target.ID == "" || source.ID == target.ID || source.ProjectID != target.ProjectID || source.DeletedAt != nil || target.DeletedAt != nil {
		return Relationship{}, ErrInvalidReference
	}
	if strength < -100 || strength > 100 {
		return Relationship{}, ErrInvalidStrength
	}
	switch kind {
	case RelationshipAlly, RelationshipFamily, RelationshipRomantic, RelationshipRival, RelationshipEnemy, RelationshipMentor, RelationshipOther:
	default:
		return Relationship{}, ErrInvalidRelation
	}
	now = now.UTC()
	return Relationship{
		ID: id, ProjectID: source.ProjectID, SourceCharacterID: source.ID, TargetCharacterID: target.ID,
		Kind: kind, Label: strings.TrimSpace(label), Strength: strength, PublicState: strings.TrimSpace(publicState),
		TrueState: strings.TrimSpace(trueState), CreatedAt: now, UpdatedAt: now,
	}, nil
}

type AppearanceRole string

const (
	AppearancePOV       AppearanceRole = "pov"
	AppearancePresent   AppearanceRole = "present"
	AppearanceMentioned AppearanceRole = "mentioned"
)

type ChapterAppearance struct {
	ProjectID   string
	CharacterID string
	ChapterID   string
	Role        AppearanceRole
	CreatedAt   time.Time
}

type SceneAppearance struct {
	ProjectID   string
	CharacterID string
	SceneCardID string
	Role        AppearanceRole
	CreatedAt   time.Time
}

func NewChapterAppearance(projectID, characterID, chapterID string, role AppearanceRole, now time.Time) (ChapterAppearance, error) {
	projectID, characterID, chapterID = strings.TrimSpace(projectID), strings.TrimSpace(characterID), strings.TrimSpace(chapterID)
	if projectID == "" || characterID == "" || chapterID == "" {
		return ChapterAppearance{}, ErrInvalidReference
	}
	if !validAppearanceRole(role) {
		return ChapterAppearance{}, ErrInvalidRole
	}
	return ChapterAppearance{ProjectID: projectID, CharacterID: characterID, ChapterID: chapterID, Role: role, CreatedAt: now.UTC()}, nil
}

func NewSceneAppearance(projectID, characterID, sceneCardID string, role AppearanceRole, now time.Time) (SceneAppearance, error) {
	projectID, characterID, sceneCardID = strings.TrimSpace(projectID), strings.TrimSpace(characterID), strings.TrimSpace(sceneCardID)
	if projectID == "" || characterID == "" || sceneCardID == "" {
		return SceneAppearance{}, ErrInvalidReference
	}
	if !validAppearanceRole(role) {
		return SceneAppearance{}, ErrInvalidRole
	}
	return SceneAppearance{ProjectID: projectID, CharacterID: characterID, SceneCardID: sceneCardID, Role: role, CreatedAt: now.UTC()}, nil
}

func validAppearanceRole(role AppearanceRole) bool {
	switch role {
	case AppearancePOV, AppearancePresent, AppearanceMentioned:
		return true
	default:
		return false
	}
}
