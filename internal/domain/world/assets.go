package world

import (
	"strings"
	"time"
)

type RegionKind string

const (
	RegionWorld       RegionKind = "world"
	RegionContinent   RegionKind = "continent"
	RegionTerritory   RegionKind = "territory"
	RegionSettlement  RegionKind = "settlement"
	RegionTerrain     RegionKind = "terrain"
	RegionSecretRealm RegionKind = "secret_realm"
	RegionResource    RegionKind = "resource"
)

type RelativeDirection string

const (
	DirectionNorth     RelativeDirection = "north"
	DirectionNorthEast RelativeDirection = "north_east"
	DirectionEast      RelativeDirection = "east"
	DirectionSouthEast RelativeDirection = "south_east"
	DirectionSouth     RelativeDirection = "south"
	DirectionSouthWest RelativeDirection = "south_west"
	DirectionWest      RelativeDirection = "west"
	DirectionNorthWest RelativeDirection = "north_west"
	DirectionCenter    RelativeDirection = "center"
	DirectionSurround  RelativeDirection = "surrounding"
)

type WorldRegion struct {
	ID        string
	ProjectID string
	ParentID  string
	Kind      RegionKind
	Name      string
	Direction RelativeDirection
	Summary   string
	Terrain   string
	Climate   string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewWorldRegion(id, projectID string, parent *WorldRegion, kind RegionKind, name string, direction RelativeDirection, now time.Time) (WorldRegion, error) {
	id, projectID, name = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(name)
	if id == "" || projectID == "" || name == "" {
		return WorldRegion{}, ErrBlankValue
	}
	if !validRegionKind(kind) || !validDirection(direction) {
		return WorldRegion{}, ErrInvalidStatus
	}
	parentID := ""
	if parent == nil {
		if kind != RegionWorld {
			return WorldRegion{}, ErrInvalidHierarchy
		}
	} else {
		if kind == RegionWorld || parent.ProjectID != projectID || parent.DeletedAt != nil {
			return WorldRegion{}, ErrInvalidReference
		}
		parentID = parent.ID
	}
	now = now.UTC()
	return WorldRegion{ID: id, ProjectID: projectID, ParentID: parentID, Kind: kind, Name: name, Direction: direction, CreatedAt: now, UpdatedAt: now}, nil
}

func (r WorldRegion) UpdateProfile(parent *WorldRegion, kind RegionKind, name string, direction RelativeDirection, summary, terrain, climate string, now time.Time) (WorldRegion, error) {
	name = strings.TrimSpace(name)
	if r.ID == "" || r.ProjectID == "" || r.DeletedAt != nil || name == "" {
		return WorldRegion{}, ErrBlankValue
	}
	if !validRegionKind(kind) || !validDirection(direction) {
		return WorldRegion{}, ErrInvalidStatus
	}
	parentID := ""
	if parent == nil {
		if kind != RegionWorld {
			return WorldRegion{}, ErrInvalidHierarchy
		}
	} else {
		if kind == RegionWorld || parent.ID == r.ID || parent.ProjectID != r.ProjectID || parent.DeletedAt != nil {
			return WorldRegion{}, ErrInvalidHierarchy
		}
		parentID = parent.ID
	}
	r.ParentID, r.Kind, r.Name, r.Direction = parentID, kind, name, direction
	r.Summary, r.Terrain, r.Climate, r.UpdatedAt = strings.TrimSpace(summary), strings.TrimSpace(terrain), strings.TrimSpace(climate), now.UTC()
	return r, nil
}

func validRegionKind(kind RegionKind) bool {
	switch kind {
	case RegionWorld, RegionContinent, RegionTerritory, RegionSettlement, RegionTerrain, RegionSecretRealm, RegionResource:
		return true
	default:
		return false
	}
}

func validDirection(direction RelativeDirection) bool {
	switch direction {
	case DirectionNorth, DirectionNorthEast, DirectionEast, DirectionSouthEast, DirectionSouth, DirectionSouthWest, DirectionWest, DirectionNorthWest, DirectionCenter, DirectionSurround:
		return true
	default:
		return false
	}
}

type LocationKind string

const (
	LocationCity       LocationKind = "city"
	LocationSettlement LocationKind = "settlement"
	LocationBuilding   LocationKind = "building"
	LocationLandmark   LocationKind = "landmark"
	LocationWilderness LocationKind = "wilderness"
	LocationOther      LocationKind = "other"
)

type LocationStatus string

const (
	LocationActive    LocationStatus = "active"
	LocationInactive  LocationStatus = "inactive"
	LocationDestroyed LocationStatus = "destroyed"
)

type Location struct {
	ID        string
	ProjectID string
	RegionID  string
	Kind      LocationKind
	Name      string
	Summary   string
	Status    LocationStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewLocation(id, projectID string, region WorldRegion, kind LocationKind, name, summary string, now time.Time) (Location, error) {
	id, projectID, name = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(name)
	if id == "" || projectID == "" || name == "" {
		return Location{}, ErrBlankValue
	}
	if region.ID == "" || region.ProjectID != projectID || region.DeletedAt != nil {
		return Location{}, ErrInvalidReference
	}
	if !validLocationKind(kind) {
		return Location{}, ErrInvalidStatus
	}
	now = now.UTC()
	return Location{ID: id, ProjectID: projectID, RegionID: region.ID, Kind: kind, Name: name, Summary: strings.TrimSpace(summary), Status: LocationActive, CreatedAt: now, UpdatedAt: now}, nil
}

func (l Location) UpdateProfile(region WorldRegion, kind LocationKind, name, summary string, status LocationStatus, now time.Time) (Location, error) {
	name = strings.TrimSpace(name)
	if l.ID == "" || l.ProjectID == "" || l.DeletedAt != nil || name == "" {
		return Location{}, ErrBlankValue
	}
	if region.ID == "" || region.ProjectID != l.ProjectID || region.DeletedAt != nil {
		return Location{}, ErrInvalidReference
	}
	if !validLocationKind(kind) {
		return Location{}, ErrInvalidStatus
	}
	switch status {
	case LocationActive, LocationInactive, LocationDestroyed:
	default:
		return Location{}, ErrInvalidStatus
	}
	l.RegionID, l.Kind, l.Name, l.Summary, l.Status, l.UpdatedAt = region.ID, kind, name, strings.TrimSpace(summary), status, now.UTC()
	return l, nil
}

func validLocationKind(kind LocationKind) bool {
	switch kind {
	case LocationCity, LocationSettlement, LocationBuilding, LocationLandmark, LocationWilderness, LocationOther:
		return true
	default:
		return false
	}
}

type LocationTargetType string

const (
	LocationTargetChapter   LocationTargetType = "chapter"
	LocationTargetScene     LocationTargetType = "scene"
	LocationTargetCharacter LocationTargetType = "character"
	LocationTargetFaction   LocationTargetType = "faction"
)

type LocationLink struct {
	ProjectID  string
	LocationID string
	TargetType LocationTargetType
	TargetID   string
	Role       string
	CreatedAt  time.Time
}

func NewLocationLink(location Location, targetType LocationTargetType, targetID, role string, now time.Time) (LocationLink, error) {
	targetID = strings.TrimSpace(targetID)
	if location.ID == "" || location.ProjectID == "" || location.DeletedAt != nil || targetID == "" {
		return LocationLink{}, ErrInvalidReference
	}
	switch targetType {
	case LocationTargetChapter, LocationTargetScene, LocationTargetCharacter, LocationTargetFaction:
	default:
		return LocationLink{}, ErrInvalidReference
	}
	return LocationLink{ProjectID: location.ProjectID, LocationID: location.ID, TargetType: targetType, TargetID: targetID, Role: strings.TrimSpace(role), CreatedAt: now.UTC()}, nil
}

type ItemStatus string

const (
	ItemActive    ItemStatus = "active"
	ItemLost      ItemStatus = "lost"
	ItemDestroyed ItemStatus = "destroyed"
)

type HolderType string

const (
	HolderCharacter HolderType = "character"
	HolderFaction   HolderType = "faction"
	HolderLocation  HolderType = "location"
)

type Item struct {
	ID         string
	ProjectID  string
	Name       string
	Summary    string
	Status     ItemStatus
	HolderType HolderType
	HolderID   string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

type ItemTransfer struct {
	ID        string
	ProjectID string
	ItemID    string
	FromType  HolderType
	FromID    string
	ToType    HolderType
	ToID      string
	ChapterID string
	Note      string
	CreatedAt time.Time
}

func NewItem(id, projectID, name, summary string, now time.Time) (Item, error) {
	id, projectID, name = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(name)
	if id == "" || projectID == "" || name == "" {
		return Item{}, ErrBlankValue
	}
	now = now.UTC()
	return Item{ID: id, ProjectID: projectID, Name: name, Summary: strings.TrimSpace(summary), Status: ItemActive, CreatedAt: now, UpdatedAt: now}, nil
}

func (i Item) UpdateProfile(name, summary string, status ItemStatus, now time.Time) (Item, error) {
	name = strings.TrimSpace(name)
	if i.ID == "" || i.ProjectID == "" || i.DeletedAt != nil || name == "" {
		return Item{}, ErrBlankValue
	}
	switch status {
	case ItemActive, ItemLost, ItemDestroyed:
	default:
		return Item{}, ErrInvalidStatus
	}
	i.Name, i.Summary, i.Status, i.UpdatedAt = name, strings.TrimSpace(summary), status, now.UTC()
	return i, nil
}

func (i Item) Transfer(id string, toType HolderType, toID, chapterID, note string, now time.Time) (Item, ItemTransfer, error) {
	id, toID = strings.TrimSpace(id), strings.TrimSpace(toID)
	if i.ID == "" || i.ProjectID == "" || i.DeletedAt != nil || id == "" || toID == "" || !validHolderType(toType) {
		return Item{}, ItemTransfer{}, ErrInvalidReference
	}
	now = now.UTC()
	transfer := ItemTransfer{
		ID: id, ProjectID: i.ProjectID, ItemID: i.ID, FromType: i.HolderType, FromID: i.HolderID,
		ToType: toType, ToID: toID, ChapterID: strings.TrimSpace(chapterID), Note: strings.TrimSpace(note), CreatedAt: now,
	}
	i.HolderType, i.HolderID, i.UpdatedAt = toType, toID, now
	return i, transfer, nil
}

func validHolderType(holderType HolderType) bool {
	switch holderType {
	case HolderCharacter, HolderFaction, HolderLocation:
		return true
	default:
		return false
	}
}

type RuleScopeType string

const (
	RuleScopeProject  RuleScopeType = "project"
	RuleScopeRegion   RuleScopeType = "region"
	RuleScopeLocation RuleScopeType = "location"
)

type RuleStatus string

const (
	RuleActive   RuleStatus = "active"
	RuleInactive RuleStatus = "inactive"
)

type WorldRule struct {
	ID          string
	ProjectID   string
	Name        string
	Description string
	ScopeType   RuleScopeType
	ScopeID     string
	Status      RuleStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func NewWorldRule(id, projectID, name, description string, scopeType RuleScopeType, scopeID string, now time.Time) (WorldRule, error) {
	id, projectID, name = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(name)
	scopeID = strings.TrimSpace(scopeID)
	if id == "" || projectID == "" || name == "" || strings.TrimSpace(description) == "" {
		return WorldRule{}, ErrBlankValue
	}
	switch scopeType {
	case RuleScopeProject:
		if scopeID != "" {
			return WorldRule{}, ErrInvalidReference
		}
	case RuleScopeRegion, RuleScopeLocation:
		if scopeID == "" {
			return WorldRule{}, ErrInvalidReference
		}
	default:
		return WorldRule{}, ErrInvalidStatus
	}
	now = now.UTC()
	return WorldRule{ID: id, ProjectID: projectID, Name: name, Description: strings.TrimSpace(description), ScopeType: scopeType, ScopeID: scopeID, Status: RuleActive, CreatedAt: now, UpdatedAt: now}, nil
}

func (r WorldRule) UpdateProfile(name, description string, scopeType RuleScopeType, scopeID string, status RuleStatus, now time.Time) (WorldRule, error) {
	name, description, scopeID = strings.TrimSpace(name), strings.TrimSpace(description), strings.TrimSpace(scopeID)
	if r.ID == "" || r.ProjectID == "" || r.DeletedAt != nil || name == "" || description == "" {
		return WorldRule{}, ErrBlankValue
	}
	switch scopeType {
	case RuleScopeProject:
		if scopeID != "" {
			return WorldRule{}, ErrInvalidReference
		}
	case RuleScopeRegion, RuleScopeLocation:
		if scopeID == "" {
			return WorldRule{}, ErrInvalidReference
		}
	default:
		return WorldRule{}, ErrInvalidStatus
	}
	if status != RuleActive && status != RuleInactive {
		return WorldRule{}, ErrInvalidStatus
	}
	r.Name, r.Description, r.ScopeType, r.ScopeID, r.Status, r.UpdatedAt = name, description, scopeType, scopeID, status, now.UTC()
	return r, nil
}

type HookStatus string

const (
	HookPlanted     HookStatus = "planted"
	HookProgressing HookStatus = "progressing"
	HookResolved    HookStatus = "resolved"
	HookAbandoned   HookStatus = "abandoned"
)

type ForeshadowingHook struct {
	ID                string
	ProjectID         string
	Title             string
	Setup             string
	PlantedChapterID  string
	DueChapterID      string
	ResolvedChapterID string
	Status            HookStatus
	Notes             string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

func NewForeshadowingHook(id, projectID, title, setup, plantedChapterID, dueChapterID string, now time.Time) (ForeshadowingHook, error) {
	id, projectID, title = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(title)
	plantedChapterID = strings.TrimSpace(plantedChapterID)
	if id == "" || projectID == "" || title == "" || strings.TrimSpace(setup) == "" {
		return ForeshadowingHook{}, ErrBlankValue
	}
	if plantedChapterID == "" {
		return ForeshadowingHook{}, ErrInvalidReference
	}
	now = now.UTC()
	return ForeshadowingHook{ID: id, ProjectID: projectID, Title: title, Setup: strings.TrimSpace(setup), PlantedChapterID: plantedChapterID, DueChapterID: strings.TrimSpace(dueChapterID), Status: HookPlanted, CreatedAt: now, UpdatedAt: now}, nil
}

func (h ForeshadowingHook) Update(title, setup, plantedChapterID, dueChapterID, resolvedChapterID string, status HookStatus, notes string, now time.Time) (ForeshadowingHook, error) {
	title, setup = strings.TrimSpace(title), strings.TrimSpace(setup)
	plantedChapterID, dueChapterID = strings.TrimSpace(plantedChapterID), strings.TrimSpace(dueChapterID)
	resolvedChapterID = strings.TrimSpace(resolvedChapterID)
	if h.ID == "" || h.ProjectID == "" || h.DeletedAt != nil || title == "" || setup == "" {
		return ForeshadowingHook{}, ErrBlankValue
	}
	if plantedChapterID == "" || (status == HookResolved && resolvedChapterID == "") {
		return ForeshadowingHook{}, ErrInvalidReference
	}
	switch status {
	case HookPlanted, HookProgressing, HookResolved, HookAbandoned:
	default:
		return ForeshadowingHook{}, ErrInvalidStatus
	}
	h.Title, h.Setup, h.PlantedChapterID, h.DueChapterID = title, setup, plantedChapterID, dueChapterID
	h.ResolvedChapterID, h.Status, h.Notes, h.UpdatedAt = resolvedChapterID, status, strings.TrimSpace(notes), now.UTC()
	return h, nil
}
