package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

func TestStorePersistsWorldAssetsAndItemTransferAtomically(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Unix(100, 0)

	chapter := mustChapter(t, "chapter-1", project.ID, "第一章", 1)
	if err := store.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	root, _ := worlddomain.NewWorldRegion("region-root", project.ID, nil, worlddomain.RegionWorld, "镜海世界", worlddomain.DirectionCenter, now)
	region, _ := worlddomain.NewWorldRegion("region-north", project.ID, &root, worlddomain.RegionTerritory, "北境", worlddomain.DirectionNorth, now)
	if err := store.CreateWorldRegion(ctx, root); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorldRegion(ctx, region); err != nil {
		t.Fatal(err)
	}
	location, _ := worlddomain.NewLocation("location-1", project.ID, region, worlddomain.LocationCity, "雾港城", "北境港城", now)
	if err := store.CreateLocation(ctx, location); err != nil {
		t.Fatal(err)
	}
	link, _ := worlddomain.NewLocationLink(location, worlddomain.LocationTargetChapter, chapter.ID, "主要舞台", now)
	if err := store.UpsertLocationLink(ctx, link); err != nil {
		t.Fatal(err)
	}
	item, _ := worlddomain.NewItem("item-1", project.ID, "问天剑", "宗门信物", now)
	if err := store.CreateItem(ctx, item); err != nil {
		t.Fatal(err)
	}
	updatedItem, transfer, _ := item.Transfer("transfer-1", worlddomain.HolderLocation, location.ID, chapter.ID, "存放于城主府", now.Add(time.Hour))
	if err := store.TransferItem(ctx, updatedItem, transfer); err != nil {
		t.Fatal(err)
	}
	rule, _ := worlddomain.NewWorldRule("rule-1", project.ID, "禁空", "城内无法御空", worlddomain.RuleScopeLocation, location.ID, now)
	if err := store.CreateWorldRule(ctx, rule); err != nil {
		t.Fatal(err)
	}
	hook, _ := worlddomain.NewForeshadowingHook("hook-1", project.ID, "断剑之谜", "主角获得断剑", chapter.ID, "", now)
	if err := store.CreateForeshadowingHook(ctx, hook); err != nil {
		t.Fatal(err)
	}

	regions, err := store.ListWorldRegions(ctx)
	if err != nil || len(regions) != 2 || regions[0].ID != root.ID || regions[1].ID != region.ID {
		t.Fatalf("regions = %#v, error = %v", regions, err)
	}
	locations, err := store.ListLocations(ctx)
	if err != nil || len(locations) != 1 || locations[0].ID != location.ID {
		t.Fatalf("locations = %#v, error = %v", locations, err)
	}
	links, err := store.ListLocationLinks(ctx)
	if err != nil || len(links) != 1 || links[0].TargetID != chapter.ID {
		t.Fatalf("links = %#v, error = %v", links, err)
	}
	storedItem, err := store.Item(ctx, item.ID)
	if err != nil || storedItem.HolderID != location.ID {
		t.Fatalf("item = %#v, error = %v", storedItem, err)
	}
	transfers, err := store.ListItemTransfers(ctx, item.ID)
	if err != nil || len(transfers) != 1 || transfers[0].ID != transfer.ID {
		t.Fatalf("transfers = %#v, error = %v", transfers, err)
	}
	rules, err := store.ListWorldRules(ctx)
	if err != nil || len(rules) != 1 || rules[0].ID != rule.ID {
		t.Fatalf("rules = %#v, error = %v", rules, err)
	}
	hooks, err := store.ListForeshadowingHooks(ctx)
	if err != nil || len(hooks) != 1 || hooks[0].ID != hook.ID {
		t.Fatalf("hooks = %#v, error = %v", hooks, err)
	}
}

func TestStoreBlocksArchivingReferencedRegionAndLocation(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Unix(100, 0)
	root, _ := worlddomain.NewWorldRegion("region-root", project.ID, nil, worlddomain.RegionWorld, "镜海世界", worlddomain.DirectionCenter, now)
	region, _ := worlddomain.NewWorldRegion("region-north", project.ID, &root, worlddomain.RegionTerritory, "北境", worlddomain.DirectionNorth, now)
	_ = store.CreateWorldRegion(ctx, root)
	_ = store.CreateWorldRegion(ctx, region)
	location, _ := worlddomain.NewLocation("location-1", project.ID, region, worlddomain.LocationCity, "雾港城", "", now)
	_ = store.CreateLocation(ctx, location)

	if err := store.ArchiveWorldRegion(ctx, root.ID, now); !errors.Is(err, worlddomain.ErrReferenced) {
		t.Fatalf("archive root error = %v", err)
	}
	item, _ := worlddomain.NewItem("item-1", project.ID, "问天剑", "", now)
	_ = store.CreateItem(ctx, item)
	updated, transfer, _ := item.Transfer("transfer-1", worlddomain.HolderLocation, location.ID, "", "", now)
	_ = store.TransferItem(ctx, updated, transfer)
	if err := store.ArchiveLocation(ctx, location.ID, now); !errors.Is(err, worlddomain.ErrReferenced) {
		t.Fatalf("archive location error = %v", err)
	}
}

func TestStorePersistsUndirectedRegionAdjacencyAndProtectsReferences(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Unix(100, 0)
	root, _ := worlddomain.NewWorldRegion("region-root", project.ID, nil, worlddomain.RegionWorld, "镜海世界", worlddomain.DirectionCenter, now)
	east, _ := worlddomain.NewWorldRegion("region-east", project.ID, &root, worlddomain.RegionTerritory, "东境", worlddomain.DirectionEast, now)
	west, _ := worlddomain.NewWorldRegion("region-west", project.ID, &root, worlddomain.RegionTerritory, "西境", worlddomain.DirectionWest, now)
	for _, region := range []worlddomain.WorldRegion{root, east, west} {
		if err := store.CreateWorldRegion(ctx, region); err != nil {
			t.Fatal(err)
		}
	}
	adjacency, _ := worlddomain.NewRegionAdjacency(west, east, "渡口往来", now)
	if err := store.UpsertRegionAdjacency(ctx, adjacency); err != nil {
		t.Fatal(err)
	}

	adjacencies, err := store.ListRegionAdjacencies(ctx)
	if err != nil || len(adjacencies) != 1 || adjacencies[0].FirstRegionID != east.ID || adjacencies[0].SecondRegionID != west.ID {
		t.Fatalf("adjacencies = %#v, error = %v", adjacencies, err)
	}
	if err := store.ArchiveWorldRegion(ctx, east.ID, now.Add(time.Hour)); !errors.Is(err, worlddomain.ErrReferenced) {
		t.Fatalf("archive adjacent region error = %v", err)
	}
	if err := store.DeleteRegionAdjacency(ctx, west.ID, east.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.ArchiveWorldRegion(ctx, east.ID, now.Add(time.Hour)); err != nil {
		t.Fatalf("archive unlinked region: %v", err)
	}
}
