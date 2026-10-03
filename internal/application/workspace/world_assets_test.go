package workspace

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

func TestWorldAssetLifecycleCreatesQueryableStructuredAssets(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	service := openTestService(t, store, &sequenceIDs{ids: []string{
		"region-root", "region-north", "location-1", "item-1", "transfer-1", "rule-1", "hook-1",
	}})
	ctx := context.Background()

	root, err := service.CreateWorldRegion(ctx, WorldRegionInput{Kind: "world", Name: "镜海世界", Direction: "center"})
	if err != nil {
		t.Fatal(err)
	}
	region, err := service.CreateWorldRegion(ctx, WorldRegionInput{ParentID: root.ID, Kind: "territory", Name: "北境", Direction: "north"})
	if err != nil {
		t.Fatal(err)
	}
	location, err := service.CreateLocation(ctx, LocationInput{RegionID: region.ID, Kind: "city", Name: "雾港城", Summary: "北境港城"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := service.CreateItem(ctx, ItemInput{Name: "问天剑", Summary: "宗门信物"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.TransferItem(ctx, ItemTransferInput{ItemID: item.ID, HolderType: "location", HolderID: location.ID, ChapterID: chapter.ID, Note: "存放于城主府"}); err != nil {
		t.Fatal(err)
	}
	rule, err := service.CreateWorldRule(ctx, WorldRuleInput{Name: "禁空", Description: "城内无法御空", ScopeType: "location", ScopeID: location.ID})
	if err != nil {
		t.Fatal(err)
	}
	hook, err := service.CreateForeshadowingHook(ctx, ForeshadowingHookInput{Title: "断剑之谜", Setup: "主角获得断剑", PlantedChapterID: chapter.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.LinkLocation(ctx, LocationLinkInput{LocationID: location.ID, TargetType: "chapter", TargetID: chapter.ID, Role: "主要舞台"}); err != nil {
		t.Fatal(err)
	}

	view, err := service.WorldAssets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Regions) != 2 || len(view.Locations) != 1 || len(view.LocationLinks) != 1 || len(view.Items) != 1 || len(view.ItemTransfers) != 1 || len(view.Rules) != 1 || len(view.Hooks) != 1 {
		t.Fatalf("world assets = %#v", view)
	}
	if view.Rules[0].ID != rule.ID || view.Hooks[0].ID != hook.ID || view.Items[0].HolderID != location.ID {
		t.Fatalf("world asset identities = %#v", view)
	}
}

func TestWorldAssetViewsUseStableLowerCamelJSONFields(t *testing.T) {
	payload, err := json.Marshal(WorldAssetsView{Regions: []WorldRegionView{{ID: "region-1", ParentID: "region-root"}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range [][]byte{[]byte(`"regions"`), []byte(`"id"`), []byte(`"parentId"`)} {
		if !bytes.Contains(payload, field) {
			t.Fatalf("payload %s missing field %s", payload, field)
		}
	}
}

func TestWorldAssetReferencesMustBelongToCurrentProject(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	service := openTestService(t, store, &sequenceIDs{ids: []string{"item-1", "transfer-1"}})
	ctx := context.Background()
	item, err := service.CreateItem(ctx, ItemInput{Name: "问天剑"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.TransferItem(ctx, ItemTransferInput{ItemID: item.ID, HolderType: "character", HolderID: "missing"}); !errors.Is(err, worlddomain.ErrInvalidReference) {
		t.Fatalf("missing holder error = %v", err)
	}
	if _, err := service.CreateWorldRule(ctx, WorldRuleInput{Name: "禁空", Description: "无法飞行", ScopeType: "region", ScopeID: "missing"}); !errors.Is(err, worlddomain.ErrInvalidReference) {
		t.Fatalf("missing scope error = %v", err)
	}
	if _, err := service.LinkLocation(ctx, LocationLinkInput{LocationID: "missing", TargetType: "chapter", TargetID: "missing"}); !errors.Is(err, worlddomain.ErrInvalidReference) {
		t.Fatalf("missing location link error = %v", err)
	}
}

func TestWorldAssetsManageUndirectedRegionAdjacencies(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	root, _ := worlddomain.NewWorldRegion("region-root", project.ID, nil, worlddomain.RegionWorld, "镜海世界", worlddomain.DirectionCenter, time.Unix(100, 0))
	east, _ := worlddomain.NewWorldRegion("region-east", project.ID, &root, worlddomain.RegionTerritory, "东境", worlddomain.DirectionEast, time.Unix(100, 0))
	west, _ := worlddomain.NewWorldRegion("region-west", project.ID, &root, worlddomain.RegionTerritory, "西境", worlddomain.DirectionWest, time.Unix(100, 0))
	store.worldRegions[root.ID] = root
	store.worldRegions[east.ID] = east
	store.worldRegions[west.ID] = west
	service := openTestService(t, store, &sequenceIDs{})

	adjacency, err := service.UpsertRegionAdjacency(context.Background(), RegionAdjacencyInput{FirstRegionID: west.ID, SecondRegionID: east.ID, TravelNote: "渡口往来"})
	if err != nil {
		t.Fatal(err)
	}
	if adjacency.FirstRegionID != east.ID || adjacency.SecondRegionID != west.ID {
		t.Fatalf("adjacency = %#v", adjacency)
	}
	view, err := service.WorldAssets(context.Background())
	if err != nil || len(view.RegionAdjacencies) != 1 || view.RegionAdjacencies[0].TravelNote != "渡口往来" {
		t.Fatalf("world assets = %#v, error = %v", view, err)
	}
	if err := service.DeleteRegionAdjacency(context.Background(), west.ID, east.ID); err != nil {
		t.Fatal(err)
	}
}

func TestWorldAssetCreationPreservesRequestedState(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	root, _ := worlddomain.NewWorldRegion("region-root", project.ID, nil, worlddomain.RegionWorld, "镜海世界", worlddomain.DirectionCenter, time.Unix(100, 0))
	store.worldRegions[root.ID] = root
	service := openTestService(t, store, &sequenceIDs{ids: []string{"location-1", "item-1", "rule-1", "hook-1"}})
	ctx := context.Background()

	location, err := service.CreateLocation(ctx, LocationInput{RegionID: root.ID, Kind: "city", Name: "旧都", Status: "destroyed"})
	if err != nil || location.Status != "destroyed" {
		t.Fatalf("location = %#v, error = %v", location, err)
	}
	item, err := service.CreateItem(ctx, ItemInput{Name: "遗失王冠", Status: "lost"})
	if err != nil || item.Status != "lost" {
		t.Fatalf("item = %#v, error = %v", item, err)
	}
	rule, err := service.CreateWorldRule(ctx, WorldRuleInput{Name: "旧律", Description: "已经失效", ScopeType: "project", Status: "inactive"})
	if err != nil || rule.Status != "inactive" {
		t.Fatalf("rule = %#v, error = %v", rule, err)
	}
	hook, err := service.CreateForeshadowingHook(ctx, ForeshadowingHookInput{Title: "旧王之谜", Setup: "残碑记载旧王", PlantedChapterID: chapter.ID, ResolvedChapterID: chapter.ID, Status: "resolved", Notes: "创建时已回收"})
	if err != nil || hook.Status != "resolved" || hook.ResolvedChapterID != chapter.ID || hook.Notes != "创建时已回收" {
		t.Fatalf("hook = %#v, error = %v", hook, err)
	}
}

func TestWorldAssetUpdatesAndArchivesUseDomainRules(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	root, _ := worlddomain.NewWorldRegion("region-root", project.ID, nil, worlddomain.RegionWorld, "镜海世界", worlddomain.DirectionCenter, time.Unix(100, 0))
	store.worldRegions[root.ID] = root
	location, _ := worlddomain.NewLocation("location-1", project.ID, root, worlddomain.LocationCity, "雾港城", "", time.Unix(100, 0))
	store.locations[location.ID] = location
	item, _ := worlddomain.NewItem("item-1", project.ID, "问天剑", "", time.Unix(100, 0))
	store.items[item.ID] = item
	rule, _ := worlddomain.NewWorldRule("rule-1", project.ID, "禁空", "无法飞行", worlddomain.RuleScopeProject, "", time.Unix(100, 0))
	store.worldRules[rule.ID] = rule
	hook, _ := worlddomain.NewForeshadowingHook("hook-1", project.ID, "断剑之谜", "主角获得断剑", chapter.ID, "", time.Unix(100, 0))
	store.foreshadowingHooks[hook.ID] = hook
	service := openTestService(t, store, &sequenceIDs{ids: []string{}})
	ctx := context.Background()

	updated, err := service.UpdateWorldRegion(ctx, root.ID, WorldRegionInput{Kind: "world", Name: "镜海诸界", Direction: "center", Summary: "多层世界"})
	if err != nil || updated.Name != "镜海诸界" {
		t.Fatalf("updated region = %#v, error = %v", updated, err)
	}
	updatedLocation, err := service.UpdateLocation(ctx, location.ID, LocationInput{RegionID: root.ID, Kind: "landmark", Name: "雾港遗址", Status: "inactive"})
	if err != nil || updatedLocation.Kind != "landmark" {
		t.Fatalf("updated location = %#v, error = %v", updatedLocation, err)
	}
	updatedItem, err := service.UpdateItem(ctx, item.ID, ItemInput{Name: "问天断剑", Status: "lost"})
	if err != nil || updatedItem.Status != "lost" {
		t.Fatalf("updated item = %#v, error = %v", updatedItem, err)
	}
	updatedRule, err := service.UpdateWorldRule(ctx, rule.ID, WorldRuleInput{Name: "禁空法则", Description: "仍然无法飞行", ScopeType: "project", Status: "inactive"})
	if err != nil || updatedRule.Status != "inactive" {
		t.Fatalf("updated rule = %#v, error = %v", updatedRule, err)
	}
	resolved, err := service.UpdateForeshadowingHook(ctx, hook.ID, ForeshadowingHookInput{Title: hook.Title, Setup: hook.Setup, PlantedChapterID: chapter.ID, ResolvedChapterID: chapter.ID, Status: "resolved", Notes: "真相揭晓"})
	if err != nil || resolved.Status != "resolved" {
		t.Fatalf("resolved hook = %#v, error = %v", resolved, err)
	}
	if err := service.ArchiveForeshadowingHook(ctx, hook.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.ArchiveLocation(ctx, location.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.ArchiveItem(ctx, item.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.ArchiveWorldRule(ctx, rule.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.foreshadowingHooks[hook.ID]; !ok || store.foreshadowingHooks[hook.ID].DeletedAt == nil {
		t.Fatalf("hook was not archived: %#v", store.foreshadowingHooks[hook.ID])
	}
}

func TestHookStatusPreservesMetadataAndRejectsInvalidRecovery(t *testing.T) {
	project := mustProject(t, "project-1", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	hook, _ := worlddomain.NewForeshadowingHook("hook-1", project.ID, "断剑之谜", "最新线索", chapter.ID, chapter.ID, time.Unix(100, 0))
	hook.Notes = "保留作者备注"
	store.foreshadowingHooks[hook.ID] = hook
	service := openTestService(t, store, &sequenceIDs{})
	ctx := context.Background()
	updated, err := service.UpdateForeshadowingHookStatus(ctx, hook.ID, ForeshadowingHookStatusInput{Status: "progressing"})
	if err != nil || updated.Status != "progressing" || updated.DueChapterID != chapter.ID || updated.Notes != hook.Notes || updated.Setup != hook.Setup {
		t.Fatalf("status update lost metadata: %#v, %v", updated, err)
	}
	for _, input := range []ForeshadowingHookStatusInput{{Status: "resolved"}, {Status: "resolved", ResolvedChapterID: "missing"}, {Status: "growing"}} {
		before := store.foreshadowingHooks[hook.ID]
		if _, err := service.UpdateForeshadowingHookStatus(ctx, hook.ID, input); err == nil {
			t.Fatalf("accepted invalid status input: %#v", input)
		}
		if store.foreshadowingHooks[hook.ID] != before {
			t.Fatal("failed update changed stored hook")
		}
	}
	updated, err = service.UpdateForeshadowingHookStatus(ctx, hook.ID, ForeshadowingHookStatusInput{Status: "resolved", ResolvedChapterID: chapter.ID})
	if err != nil || updated.ResolvedChapterID != chapter.ID || updated.Notes != hook.Notes {
		t.Fatalf("recovery = %#v, %v", updated, err)
	}
	updated, err = service.UpdateForeshadowingHookStatus(ctx, hook.ID, ForeshadowingHookStatusInput{Status: "abandoned"})
	if err != nil || updated.Status != "abandoned" || updated.Notes != hook.Notes {
		t.Fatalf("abandon = %#v, %v", updated, err)
	}
}
