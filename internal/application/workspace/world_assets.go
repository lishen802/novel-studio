package workspace

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/lishen802/novel-studio/internal/application/license"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

type WorldRegionInput struct {
	ParentID  string `json:"parentId"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Summary   string `json:"summary"`
	Terrain   string `json:"terrain"`
	Climate   string `json:"climate"`
}

type LocationInput struct {
	RegionID string `json:"regionId"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
}

type RegionAdjacencyInput struct {
	FirstRegionID  string `json:"firstRegionId"`
	SecondRegionID string `json:"secondRegionId"`
	TravelNote     string `json:"travelNote"`
}

type LocationLinkInput struct {
	LocationID string `json:"locationId"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Role       string `json:"role"`
}

type ItemInput struct {
	Name    string `json:"name"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

type ItemTransferInput struct {
	ItemID     string `json:"itemId"`
	HolderType string `json:"holderType"`
	HolderID   string `json:"holderId"`
	ChapterID  string `json:"chapterId"`
	Note       string `json:"note"`
}

type WorldRuleInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ScopeType   string `json:"scopeType"`
	ScopeID     string `json:"scopeId"`
	Status      string `json:"status"`
}

type ForeshadowingHookInput struct {
	Title             string `json:"title"`
	Setup             string `json:"setup"`
	PlantedChapterID  string `json:"plantedChapterId"`
	DueChapterID      string `json:"dueChapterId"`
	ResolvedChapterID string `json:"resolvedChapterId"`
	Status            string `json:"status"`
	Notes             string `json:"notes"`
}

type WorldRegionView struct {
	ID        string `json:"id"`
	ParentID  string `json:"parentId"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Summary   string `json:"summary"`
	Terrain   string `json:"terrain"`
	Climate   string `json:"climate"`
}

type LocationView struct {
	ID       string `json:"id"`
	RegionID string `json:"regionId"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Summary  string `json:"summary"`
	Status   string `json:"status"`
}

type RegionAdjacencyView struct {
	FirstRegionID  string `json:"firstRegionId"`
	SecondRegionID string `json:"secondRegionId"`
	TravelNote     string `json:"travelNote"`
}

type LocationLinkView struct {
	LocationID string `json:"locationId"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Role       string `json:"role"`
}

type ItemView struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Summary    string `json:"summary"`
	Status     string `json:"status"`
	HolderType string `json:"holderType"`
	HolderID   string `json:"holderId"`
}

type ItemTransferView struct {
	ID        string `json:"id"`
	ItemID    string `json:"itemId"`
	FromType  string `json:"fromType"`
	FromID    string `json:"fromId"`
	ToType    string `json:"toType"`
	ToID      string `json:"toId"`
	ChapterID string `json:"chapterId"`
	Note      string `json:"note"`
}

type WorldRuleView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ScopeType   string `json:"scopeType"`
	ScopeID     string `json:"scopeId"`
	Status      string `json:"status"`
}

type ForeshadowingHookView struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Setup             string `json:"setup"`
	PlantedChapterID  string `json:"plantedChapterId"`
	DueChapterID      string `json:"dueChapterId"`
	ResolvedChapterID string `json:"resolvedChapterId"`
	Status            string `json:"status"`
	Notes             string `json:"notes"`
}

type WorldAssetsView struct {
	Regions           []WorldRegionView       `json:"regions"`
	RegionAdjacencies []RegionAdjacencyView   `json:"regionAdjacencies"`
	Locations         []LocationView          `json:"locations"`
	LocationLinks     []LocationLinkView      `json:"locationLinks"`
	Items             []ItemView              `json:"items"`
	ItemTransfers     []ItemTransferView      `json:"itemTransfers"`
	Rules             []WorldRuleView         `json:"rules"`
	Hooks             []ForeshadowingHookView `json:"hooks"`
	ConsistencyIssues []ConsistencyIssueView  `json:"consistencyIssues"`
}

func (s *Service) WorldAssets(ctx context.Context) (WorldAssetsView, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.active == nil {
		return WorldAssetsView{}, ErrNoActiveProject
	}
	regions, err := s.active.store.ListWorldRegions(ctx)
	if err != nil {
		return WorldAssetsView{}, fmt.Errorf("list world regions: %w", err)
	}
	adjacencies, err := s.active.store.ListRegionAdjacencies(ctx)
	if err != nil {
		return WorldAssetsView{}, fmt.Errorf("list region adjacencies: %w", err)
	}
	locations, err := s.active.store.ListLocations(ctx)
	if err != nil {
		return WorldAssetsView{}, fmt.Errorf("list locations: %w", err)
	}
	links, err := s.active.store.ListLocationLinks(ctx)
	if err != nil {
		return WorldAssetsView{}, fmt.Errorf("list location links: %w", err)
	}
	items, err := s.active.store.ListItems(ctx)
	if err != nil {
		return WorldAssetsView{}, fmt.Errorf("list items: %w", err)
	}
	transfers := make([]worlddomain.ItemTransfer, 0)
	for _, item := range items {
		itemTransfers, err := s.active.store.ListItemTransfers(ctx, item.ID)
		if err != nil {
			return WorldAssetsView{}, fmt.Errorf("list item transfers: %w", err)
		}
		transfers = append(transfers, itemTransfers...)
	}
	rules, err := s.active.store.ListWorldRules(ctx)
	if err != nil {
		return WorldAssetsView{}, fmt.Errorf("list world rules: %w", err)
	}
	hooks, err := s.active.store.ListForeshadowingHooks(ctx)
	if err != nil {
		return WorldAssetsView{}, fmt.Errorf("list foreshadowing hooks: %w", err)
	}
	chapters, err := s.active.store.ListChapters(ctx)
	if err != nil {
		return WorldAssetsView{}, fmt.Errorf("list chapters for world assets: %w", err)
	}
	return worldAssetsView(regions, adjacencies, locations, links, items, transfers, rules, hooks, chapters), nil
}

func (s *Service) CreateWorldRegion(ctx context.Context, input WorldRegionInput) (WorldRegionView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return WorldRegionView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return WorldRegionView{}, err
	}
	parent, err := s.regionParentLocked(ctx, input.ParentID)
	if err != nil {
		return WorldRegionView{}, err
	}
	region, err := worlddomain.NewWorldRegion(s.ids.NewID(), project.ID, parent, worlddomain.RegionKind(input.Kind), input.Name, worlddomain.RelativeDirection(input.Direction), s.clock.Now())
	if err != nil {
		return WorldRegionView{}, err
	}
	region.Summary, region.Terrain, region.Climate = strings.TrimSpace(input.Summary), strings.TrimSpace(input.Terrain), strings.TrimSpace(input.Climate)
	if err := s.active.store.CreateWorldRegion(ctx, region); err != nil {
		return WorldRegionView{}, fmt.Errorf("create world region: %w", err)
	}
	return worldRegionView(region), nil
}

func (s *Service) UpdateWorldRegion(ctx context.Context, id string, input WorldRegionInput) (WorldRegionView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return WorldRegionView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return WorldRegionView{}, err
	}
	region, err := s.active.store.WorldRegion(ctx, strings.TrimSpace(id))
	if err != nil {
		return WorldRegionView{}, worlddomain.ErrInvalidReference
	}
	parent, err := s.regionParentLocked(ctx, input.ParentID)
	if err != nil {
		return WorldRegionView{}, err
	}
	regions, err := s.active.store.ListWorldRegions(ctx)
	if err != nil {
		return WorldRegionView{}, err
	}
	if parent != nil && regionContains(regions, region.ID, parent.ID) {
		return WorldRegionView{}, worlddomain.ErrInvalidHierarchy
	}
	updated, err := region.UpdateProfile(parent, worlddomain.RegionKind(input.Kind), input.Name, worlddomain.RelativeDirection(input.Direction), input.Summary, input.Terrain, input.Climate, s.clock.Now())
	if err != nil {
		return WorldRegionView{}, err
	}
	if err := s.active.store.UpdateWorldRegion(ctx, updated); err != nil {
		return WorldRegionView{}, fmt.Errorf("update world region: %w", err)
	}
	return worldRegionView(updated), nil
}

func (s *Service) UpsertRegionAdjacency(ctx context.Context, input RegionAdjacencyInput) (RegionAdjacencyView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return RegionAdjacencyView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.activeProjectLocked(ctx); err != nil {
		return RegionAdjacencyView{}, err
	}
	first, err := s.active.store.WorldRegion(ctx, strings.TrimSpace(input.FirstRegionID))
	if err != nil {
		return RegionAdjacencyView{}, worlddomain.ErrInvalidReference
	}
	second, err := s.active.store.WorldRegion(ctx, strings.TrimSpace(input.SecondRegionID))
	if err != nil {
		return RegionAdjacencyView{}, worlddomain.ErrInvalidReference
	}
	adjacency, err := worlddomain.NewRegionAdjacency(first, second, input.TravelNote, s.clock.Now())
	if err != nil {
		return RegionAdjacencyView{}, err
	}
	if err := s.active.store.UpsertRegionAdjacency(ctx, adjacency); err != nil {
		return RegionAdjacencyView{}, fmt.Errorf("upsert region adjacency: %w", err)
	}
	return regionAdjacencyView(adjacency), nil
}

func (s *Service) DeleteRegionAdjacency(ctx context.Context, firstRegionID, secondRegionID string) error {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.activeProjectLocked(ctx); err != nil {
		return err
	}
	first, err := s.active.store.WorldRegion(ctx, strings.TrimSpace(firstRegionID))
	if err != nil {
		return worlddomain.ErrInvalidReference
	}
	second, err := s.active.store.WorldRegion(ctx, strings.TrimSpace(secondRegionID))
	if err != nil {
		return worlddomain.ErrInvalidReference
	}
	adjacency, err := worlddomain.NewRegionAdjacency(first, second, "", s.clock.Now())
	if err != nil {
		return err
	}
	if err := s.active.store.DeleteRegionAdjacency(ctx, adjacency.FirstRegionID, adjacency.SecondRegionID); err != nil {
		return fmt.Errorf("delete region adjacency: %w", err)
	}
	return nil
}

func (s *Service) ArchiveWorldRegion(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error { return s.active.store.ArchiveWorldRegion(ctx, strings.TrimSpace(id), s.clock.Now()) })
}

func (s *Service) CreateLocation(ctx context.Context, input LocationInput) (LocationView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return LocationView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return LocationView{}, err
	}
	region, err := s.active.store.WorldRegion(ctx, strings.TrimSpace(input.RegionID))
	if err != nil || region.ProjectID != project.ID {
		return LocationView{}, worlddomain.ErrInvalidReference
	}
	location, err := worlddomain.NewLocation(s.ids.NewID(), project.ID, region, worlddomain.LocationKind(input.Kind), input.Name, input.Summary, s.clock.Now())
	if err != nil {
		return LocationView{}, err
	}
	status := worlddomain.LocationStatus(input.Status)
	if status == "" {
		status = worlddomain.LocationActive
	}
	location, err = location.UpdateProfile(region, worlddomain.LocationKind(input.Kind), input.Name, input.Summary, status, s.clock.Now())
	if err != nil {
		return LocationView{}, err
	}
	if err := s.active.store.CreateLocation(ctx, location); err != nil {
		return LocationView{}, fmt.Errorf("create location: %w", err)
	}
	return locationView(location), nil
}

func (s *Service) UpdateLocation(ctx context.Context, id string, input LocationInput) (LocationView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return LocationView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return LocationView{}, err
	}
	location, err := s.active.store.Location(ctx, strings.TrimSpace(id))
	if err != nil {
		return LocationView{}, worlddomain.ErrInvalidReference
	}
	region, err := s.active.store.WorldRegion(ctx, strings.TrimSpace(input.RegionID))
	if err != nil || region.ProjectID != location.ProjectID {
		return LocationView{}, worlddomain.ErrInvalidReference
	}
	status := worlddomain.LocationStatus(input.Status)
	if status == "" {
		status = worlddomain.LocationActive
	}
	updated, err := location.UpdateProfile(region, worlddomain.LocationKind(input.Kind), input.Name, input.Summary, status, s.clock.Now())
	if err != nil {
		return LocationView{}, err
	}
	if err := s.active.store.UpdateLocation(ctx, updated); err != nil {
		return LocationView{}, fmt.Errorf("update location: %w", err)
	}
	return locationView(updated), nil
}

func (s *Service) ArchiveLocation(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error { return s.active.store.ArchiveLocation(ctx, strings.TrimSpace(id), s.clock.Now()) })
}

func (s *Service) LinkLocation(ctx context.Context, input LocationLinkInput) (LocationLinkView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return LocationLinkView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return LocationLinkView{}, err
	}
	location, err := s.active.store.Location(ctx, strings.TrimSpace(input.LocationID))
	if err != nil {
		return LocationLinkView{}, worlddomain.ErrInvalidReference
	}
	targetType := worlddomain.LocationTargetType(input.TargetType)
	if err := s.validateLocationTargetLocked(ctx, location.ProjectID, targetType, input.TargetID); err != nil {
		return LocationLinkView{}, err
	}
	link, err := worlddomain.NewLocationLink(location, targetType, input.TargetID, input.Role, s.clock.Now())
	if err != nil {
		return LocationLinkView{}, err
	}
	if err := s.active.store.UpsertLocationLink(ctx, link); err != nil {
		return LocationLinkView{}, fmt.Errorf("link location: %w", err)
	}
	return locationLinkView(link), nil
}

func (s *Service) UnlinkLocation(ctx context.Context, locationID, targetType, targetID string) error {
	return s.withStoryWrite(ctx, func() error {
		return s.active.store.DeleteLocationLink(ctx, strings.TrimSpace(locationID), worlddomain.LocationTargetType(targetType), strings.TrimSpace(targetID))
	})
}

func (s *Service) CreateItem(ctx context.Context, input ItemInput) (ItemView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return ItemView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return ItemView{}, err
	}
	item, err := worlddomain.NewItem(s.ids.NewID(), project.ID, input.Name, input.Summary, s.clock.Now())
	if err != nil {
		return ItemView{}, err
	}
	status := worlddomain.ItemStatus(input.Status)
	if status == "" {
		status = worlddomain.ItemActive
	}
	item, err = item.UpdateProfile(input.Name, input.Summary, status, s.clock.Now())
	if err != nil {
		return ItemView{}, err
	}
	if err := s.active.store.CreateItem(ctx, item); err != nil {
		return ItemView{}, fmt.Errorf("create item: %w", err)
	}
	return itemView(item), nil
}

func (s *Service) UpdateItem(ctx context.Context, id string, input ItemInput) (ItemView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return ItemView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ItemView{}, err
	}
	item, err := s.active.store.Item(ctx, strings.TrimSpace(id))
	if err != nil {
		return ItemView{}, worlddomain.ErrInvalidReference
	}
	status := worlddomain.ItemStatus(input.Status)
	if status == "" {
		status = worlddomain.ItemActive
	}
	updated, err := item.UpdateProfile(input.Name, input.Summary, status, s.clock.Now())
	if err != nil {
		return ItemView{}, err
	}
	if err := s.active.store.UpdateItem(ctx, updated); err != nil {
		return ItemView{}, fmt.Errorf("update item: %w", err)
	}
	return itemView(updated), nil
}

func (s *Service) TransferItem(ctx context.Context, input ItemTransferInput) (ItemTransferView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return ItemTransferView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ItemTransferView{}, err
	}
	item, err := s.active.store.Item(ctx, strings.TrimSpace(input.ItemID))
	if err != nil {
		return ItemTransferView{}, worlddomain.ErrInvalidReference
	}
	holderType := worlddomain.HolderType(input.HolderType)
	if err := s.validateHolderLocked(ctx, item.ProjectID, holderType, input.HolderID); err != nil {
		return ItemTransferView{}, err
	}
	if strings.TrimSpace(input.ChapterID) != "" {
		chapter, err := s.active.store.Chapter(ctx, strings.TrimSpace(input.ChapterID))
		if err != nil || chapter.ProjectID != item.ProjectID {
			return ItemTransferView{}, worlddomain.ErrInvalidReference
		}
	}
	updated, transfer, err := item.Transfer(s.ids.NewID(), holderType, input.HolderID, input.ChapterID, input.Note, s.clock.Now())
	if err != nil {
		return ItemTransferView{}, err
	}
	if err := s.active.store.TransferItem(ctx, updated, transfer); err != nil {
		return ItemTransferView{}, fmt.Errorf("transfer item: %w", err)
	}
	return itemTransferView(transfer), nil
}

func (s *Service) ArchiveItem(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error { return s.active.store.ArchiveItem(ctx, strings.TrimSpace(id), s.clock.Now()) })
}

func (s *Service) CreateWorldRule(ctx context.Context, input WorldRuleInput) (WorldRuleView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return WorldRuleView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return WorldRuleView{}, err
	}
	if err := s.validateRuleScopeLocked(ctx, project.ID, worlddomain.RuleScopeType(input.ScopeType), input.ScopeID); err != nil {
		return WorldRuleView{}, err
	}
	rule, err := worlddomain.NewWorldRule(s.ids.NewID(), project.ID, input.Name, input.Description, worlddomain.RuleScopeType(input.ScopeType), input.ScopeID, s.clock.Now())
	if err != nil {
		return WorldRuleView{}, err
	}
	status := worlddomain.RuleStatus(input.Status)
	if status == "" {
		status = worlddomain.RuleActive
	}
	rule, err = rule.UpdateProfile(input.Name, input.Description, worlddomain.RuleScopeType(input.ScopeType), input.ScopeID, status, s.clock.Now())
	if err != nil {
		return WorldRuleView{}, err
	}
	if err := s.active.store.CreateWorldRule(ctx, rule); err != nil {
		return WorldRuleView{}, fmt.Errorf("create world rule: %w", err)
	}
	return worldRuleView(rule), nil
}

func (s *Service) UpdateWorldRule(ctx context.Context, id string, input WorldRuleInput) (WorldRuleView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return WorldRuleView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return WorldRuleView{}, err
	}
	rule, err := s.active.store.WorldRule(ctx, strings.TrimSpace(id))
	if err != nil {
		return WorldRuleView{}, worlddomain.ErrInvalidReference
	}
	if err := s.validateRuleScopeLocked(ctx, rule.ProjectID, worlddomain.RuleScopeType(input.ScopeType), input.ScopeID); err != nil {
		return WorldRuleView{}, err
	}
	status := worlddomain.RuleStatus(input.Status)
	if status == "" {
		status = worlddomain.RuleActive
	}
	updated, err := rule.UpdateProfile(input.Name, input.Description, worlddomain.RuleScopeType(input.ScopeType), input.ScopeID, status, s.clock.Now())
	if err != nil {
		return WorldRuleView{}, err
	}
	if err := s.active.store.UpdateWorldRule(ctx, updated); err != nil {
		return WorldRuleView{}, fmt.Errorf("update world rule: %w", err)
	}
	return worldRuleView(updated), nil
}

func (s *Service) ArchiveWorldRule(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error { return s.active.store.ArchiveWorldRule(ctx, strings.TrimSpace(id), s.clock.Now()) })
}

func (s *Service) CreateForeshadowingHook(ctx context.Context, input ForeshadowingHookInput) (ForeshadowingHookView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return ForeshadowingHookView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	project, err := s.activeProjectLocked(ctx)
	if err != nil {
		return ForeshadowingHookView{}, err
	}
	if err := s.validateHookChaptersLocked(ctx, project.ID, input.PlantedChapterID, input.DueChapterID, input.ResolvedChapterID); err != nil {
		return ForeshadowingHookView{}, err
	}
	hook, err := worlddomain.NewForeshadowingHook(s.ids.NewID(), project.ID, input.Title, input.Setup, input.PlantedChapterID, input.DueChapterID, s.clock.Now())
	if err != nil {
		return ForeshadowingHookView{}, err
	}
	status := worlddomain.HookStatus(input.Status)
	if status == "" {
		status = worlddomain.HookPlanted
	}
	hook, err = hook.Update(input.Title, input.Setup, input.PlantedChapterID, input.DueChapterID, input.ResolvedChapterID, status, input.Notes, s.clock.Now())
	if err != nil {
		return ForeshadowingHookView{}, err
	}
	if err := s.active.store.CreateForeshadowingHook(ctx, hook); err != nil {
		return ForeshadowingHookView{}, fmt.Errorf("create foreshadowing hook: %w", err)
	}
	return foreshadowingHookView(hook), nil
}

func (s *Service) UpdateForeshadowingHook(ctx context.Context, id string, input ForeshadowingHookInput) (ForeshadowingHookView, error) {
	if err := s.license.Require(ctx, license.EditStory); err != nil {
		return ForeshadowingHookView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ForeshadowingHookView{}, err
	}
	hook, err := s.active.store.ForeshadowingHook(ctx, strings.TrimSpace(id))
	if err != nil {
		return ForeshadowingHookView{}, worlddomain.ErrInvalidReference
	}
	if err := s.validateHookChaptersLocked(ctx, hook.ProjectID, input.PlantedChapterID, input.DueChapterID, input.ResolvedChapterID); err != nil {
		return ForeshadowingHookView{}, err
	}
	status := worlddomain.HookStatus(input.Status)
	if status == "" {
		status = worlddomain.HookPlanted
	}
	updated, err := hook.Update(input.Title, input.Setup, input.PlantedChapterID, input.DueChapterID, input.ResolvedChapterID, status, input.Notes, s.clock.Now())
	if err != nil {
		return ForeshadowingHookView{}, err
	}
	if err := s.active.store.UpdateForeshadowingHook(ctx, updated); err != nil {
		return ForeshadowingHookView{}, fmt.Errorf("update foreshadowing hook: %w", err)
	}
	return foreshadowingHookView(updated), nil
}

func (s *Service) ArchiveForeshadowingHook(ctx context.Context, id string) error {
	return s.withStoryWrite(ctx, func() error {
		return s.active.store.ArchiveForeshadowingHook(ctx, strings.TrimSpace(id), s.clock.Now())
	})
}

func (s *Service) activeProjectLocked(ctx context.Context) (projectdomain.Project, error) {
	if err := s.requireEditableLocked(); err != nil {
		return projectdomain.Project{}, err
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return projectdomain.Project{}, fmt.Errorf("read active project: %w", err)
	}
	return project, nil
}

func (s *Service) regionParentLocked(ctx context.Context, id string) (*worlddomain.WorldRegion, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}
	parent, err := s.active.store.WorldRegion(ctx, id)
	if err != nil {
		return nil, worlddomain.ErrInvalidReference
	}
	return &parent, nil
}

func (s *Service) validateHolderLocked(ctx context.Context, projectID string, holderType worlddomain.HolderType, holderID string) error {
	holderID = strings.TrimSpace(holderID)
	if holderID == "" {
		return worlddomain.ErrInvalidReference
	}
	switch holderType {
	case worlddomain.HolderCharacter:
		character, err := s.active.store.Character(ctx, holderID)
		if err != nil || character.ProjectID != projectID {
			return worlddomain.ErrInvalidReference
		}
	case worlddomain.HolderFaction:
		faction, err := s.active.store.Faction(ctx, holderID)
		if err != nil || faction.ProjectID != projectID {
			return worlddomain.ErrInvalidReference
		}
	case worlddomain.HolderLocation:
		location, err := s.active.store.Location(ctx, holderID)
		if err != nil || location.ProjectID != projectID {
			return worlddomain.ErrInvalidReference
		}
	default:
		return worlddomain.ErrInvalidReference
	}
	return nil
}

func (s *Service) validateRuleScopeLocked(ctx context.Context, projectID string, scopeType worlddomain.RuleScopeType, scopeID string) error {
	scopeID = strings.TrimSpace(scopeID)
	switch scopeType {
	case worlddomain.RuleScopeProject:
		if scopeID != "" {
			return worlddomain.ErrInvalidReference
		}
	case worlddomain.RuleScopeRegion:
		region, err := s.active.store.WorldRegion(ctx, scopeID)
		if err != nil || region.ProjectID != projectID {
			return worlddomain.ErrInvalidReference
		}
	case worlddomain.RuleScopeLocation:
		location, err := s.active.store.Location(ctx, scopeID)
		if err != nil || location.ProjectID != projectID {
			return worlddomain.ErrInvalidReference
		}
	default:
		return worlddomain.ErrInvalidReference
	}
	return nil
}

func (s *Service) validateHookChaptersLocked(ctx context.Context, projectID string, chapterIDs ...string) error {
	for index, chapterID := range chapterIDs {
		chapterID = strings.TrimSpace(chapterID)
		if chapterID == "" {
			if index == 0 {
				return worlddomain.ErrInvalidReference
			}
			continue
		}
		chapter, err := s.active.store.Chapter(ctx, chapterID)
		if err != nil || chapter.ProjectID != projectID {
			return worlddomain.ErrInvalidReference
		}
	}
	return nil
}

func (s *Service) validateLocationTargetLocked(ctx context.Context, projectID string, targetType worlddomain.LocationTargetType, targetID string) error {
	targetID = strings.TrimSpace(targetID)
	switch targetType {
	case worlddomain.LocationTargetChapter:
		chapter, err := s.active.store.Chapter(ctx, targetID)
		if err != nil || chapter.ProjectID != projectID {
			return worlddomain.ErrInvalidReference
		}
	case worlddomain.LocationTargetCharacter:
		character, err := s.active.store.Character(ctx, targetID)
		if err != nil || character.ProjectID != projectID {
			return worlddomain.ErrInvalidReference
		}
	case worlddomain.LocationTargetFaction:
		faction, err := s.active.store.Faction(ctx, targetID)
		if err != nil || faction.ProjectID != projectID {
			return worlddomain.ErrInvalidReference
		}
	case worlddomain.LocationTargetScene:
		chapters, err := s.active.store.ListChapters(ctx)
		if err != nil {
			return worlddomain.ErrInvalidReference
		}
		found := false
		for _, chapter := range chapters {
			cards, err := s.active.store.ListSceneCards(ctx, chapter.ID)
			if err != nil {
				return worlddomain.ErrInvalidReference
			}
			for _, card := range cards {
				if card.ID == targetID && card.ProjectID == projectID {
					found = true
				}
			}
		}
		if !found {
			return worlddomain.ErrInvalidReference
		}
	default:
		return worlddomain.ErrInvalidReference
	}
	return nil
}

func regionContains(regions []worlddomain.WorldRegion, ancestorID, candidateID string) bool {
	parents := make(map[string]string, len(regions))
	for _, region := range regions {
		parents[region.ID] = region.ParentID
	}
	seen := make(map[string]struct{})
	for candidateID != "" {
		if candidateID == ancestorID {
			return true
		}
		if _, exists := seen[candidateID]; exists {
			return true
		}
		seen[candidateID] = struct{}{}
		candidateID = parents[candidateID]
	}
	return false
}

func worldAssetsView(regions []worlddomain.WorldRegion, adjacencies []worlddomain.RegionAdjacency, locations []worlddomain.Location, links []worlddomain.LocationLink, items []worlddomain.Item, transfers []worlddomain.ItemTransfer, rules []worlddomain.WorldRule, hooks []worlddomain.ForeshadowingHook, chapters []projectdomain.Chapter) WorldAssetsView {
	view := WorldAssetsView{
		Regions: make([]WorldRegionView, 0, len(regions)), Locations: make([]LocationView, 0, len(locations)),
		RegionAdjacencies: make([]RegionAdjacencyView, 0, len(adjacencies)),
		LocationLinks:     make([]LocationLinkView, 0, len(links)), Items: make([]ItemView, 0, len(items)),
		ItemTransfers: make([]ItemTransferView, 0, len(transfers)), Rules: make([]WorldRuleView, 0, len(rules)), Hooks: make([]ForeshadowingHookView, 0, len(hooks)),
	}
	for _, region := range regions {
		view.Regions = append(view.Regions, worldRegionView(region))
	}
	for _, adjacency := range adjacencies {
		view.RegionAdjacencies = append(view.RegionAdjacencies, regionAdjacencyView(adjacency))
	}
	for _, location := range locations {
		view.Locations = append(view.Locations, locationView(location))
	}
	for _, link := range links {
		view.LocationLinks = append(view.LocationLinks, locationLinkView(link))
	}
	for _, item := range items {
		view.Items = append(view.Items, itemView(item))
	}
	for _, transfer := range transfers {
		view.ItemTransfers = append(view.ItemTransfers, itemTransferView(transfer))
	}
	for _, rule := range rules {
		view.Rules = append(view.Rules, worldRuleView(rule))
	}
	for _, hook := range hooks {
		view.Hooks = append(view.Hooks, foreshadowingHookView(hook))
	}
	view.ConsistencyIssues = worldAssetConsistencyIssues(locations, hooks, chapters)
	return view
}

func worldAssetConsistencyIssues(locations []worlddomain.Location, hooks []worlddomain.ForeshadowingHook, chapters []projectdomain.Chapter) []ConsistencyIssueView {
	issues := make([]ConsistencyIssueView, 0)
	locationNames := make(map[string][]string)
	for _, location := range locations {
		key := strings.ToLower(strings.TrimSpace(location.Name))
		locationNames[key] = append(locationNames[key], location.ID)
	}
	for name, ids := range locationNames {
		if name != "" && len(ids) > 1 {
			sort.Strings(ids)
			issues = append(issues, ConsistencyIssueView{Code: "duplicate_location_name", Severity: "warning", Message: fmt.Sprintf("存在 %d 个同名地点“%s”，请确认是否需要合并或改名。", len(ids), name), EntityIDs: ids})
		}
	}
	positions := make(map[string]int, len(chapters))
	maxPosition := 0
	for _, chapter := range chapters {
		positions[chapter.ID] = chapter.Position
		if chapter.Position > maxPosition {
			maxPosition = chapter.Position
		}
	}
	for _, hook := range hooks {
		if hook.Status != worlddomain.HookResolved && hook.Status != worlddomain.HookAbandoned && hook.DueChapterID != "" && positions[hook.DueChapterID] > 0 && positions[hook.DueChapterID] <= maxPosition {
			issues = append(issues, ConsistencyIssueView{Code: "overdue_foreshadowing", Severity: "warning", Message: fmt.Sprintf("伏笔“%s”已到计划回收章节但尚未回收。", hook.Title), EntityIDs: []string{hook.ID, hook.DueChapterID}})
		}
	}
	sort.Slice(issues, func(i, j int) bool { return issues[i].Code+issues[i].Message < issues[j].Code+issues[j].Message })
	return issues
}

func worldRegionView(region worlddomain.WorldRegion) WorldRegionView {
	return WorldRegionView{ID: region.ID, ParentID: region.ParentID, Kind: string(region.Kind), Name: region.Name, Direction: string(region.Direction), Summary: region.Summary, Terrain: region.Terrain, Climate: region.Climate}
}
func regionAdjacencyView(adjacency worlddomain.RegionAdjacency) RegionAdjacencyView {
	return RegionAdjacencyView{FirstRegionID: adjacency.FirstRegionID, SecondRegionID: adjacency.SecondRegionID, TravelNote: adjacency.TravelNote}
}
func locationView(location worlddomain.Location) LocationView {
	return LocationView{ID: location.ID, RegionID: location.RegionID, Kind: string(location.Kind), Name: location.Name, Summary: location.Summary, Status: string(location.Status)}
}
func locationLinkView(link worlddomain.LocationLink) LocationLinkView {
	return LocationLinkView{LocationID: link.LocationID, TargetType: string(link.TargetType), TargetID: link.TargetID, Role: link.Role}
}
func itemView(item worlddomain.Item) ItemView {
	return ItemView{ID: item.ID, Name: item.Name, Summary: item.Summary, Status: string(item.Status), HolderType: string(item.HolderType), HolderID: item.HolderID}
}
func itemTransferView(transfer worlddomain.ItemTransfer) ItemTransferView {
	return ItemTransferView{ID: transfer.ID, ItemID: transfer.ItemID, FromType: string(transfer.FromType), FromID: transfer.FromID, ToType: string(transfer.ToType), ToID: transfer.ToID, ChapterID: transfer.ChapterID, Note: transfer.Note}
}
func worldRuleView(rule worlddomain.WorldRule) WorldRuleView {
	return WorldRuleView{ID: rule.ID, Name: rule.Name, Description: rule.Description, ScopeType: string(rule.ScopeType), ScopeID: rule.ScopeID, Status: string(rule.Status)}
}
func foreshadowingHookView(hook worlddomain.ForeshadowingHook) ForeshadowingHookView {
	return ForeshadowingHookView{ID: hook.ID, Title: hook.Title, Setup: hook.Setup, PlantedChapterID: hook.PlantedChapterID, DueChapterID: hook.DueChapterID, ResolvedChapterID: hook.ResolvedChapterID, Status: string(hook.Status), Notes: hook.Notes}
}
