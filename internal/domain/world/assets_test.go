package world

import (
	"errors"
	"testing"
	"time"
)

func TestWorldRegionHierarchyRequiresSameProjectAndValidParent(t *testing.T) {
	now := time.Unix(100, 0)
	root, err := NewWorldRegion("region-root", "project-1", nil, RegionWorld, "镜海世界", DirectionCenter, now)
	if err != nil {
		t.Fatal(err)
	}
	child, err := NewWorldRegion("region-north", "project-1", &root, RegionTerritory, "北境", DirectionNorth, now)
	if err != nil || child.ParentID != root.ID {
		t.Fatalf("child = %#v, error = %v", child, err)
	}
	otherProject := root
	otherProject.ProjectID = "project-2"
	if _, err := NewWorldRegion("region-bad", "project-1", &otherProject, RegionTerritory, "越界区域", DirectionNorth, now); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("cross-project parent error = %v", err)
	}
	if _, err := NewWorldRegion("region-root-2", "project-1", nil, RegionTerritory, "无父区域", DirectionCenter, now); !errors.Is(err, ErrInvalidHierarchy) {
		t.Fatalf("orphan region error = %v", err)
	}
}

func TestLocationRequiresRegionFromSameProject(t *testing.T) {
	now := time.Unix(100, 0)
	region, err := NewWorldRegion("region-1", "project-1", nil, RegionWorld, "镜海世界", DirectionCenter, now)
	if err != nil {
		t.Fatal(err)
	}
	location, err := NewLocation("location-1", "project-1", region, LocationCity, "雾港城", "北陆港城", now)
	if err != nil || location.RegionID != region.ID {
		t.Fatalf("location = %#v, error = %v", location, err)
	}
	region.ProjectID = "project-2"
	if _, err := NewLocation("location-2", "project-1", region, LocationCity, "越界城市", "", now); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("cross-project region error = %v", err)
	}
}

func TestLocationLinkRequiresSupportedTarget(t *testing.T) {
	now := time.Unix(100, 0)
	region, _ := NewWorldRegion("region-1", "project-1", nil, RegionWorld, "镜海世界", DirectionCenter, now)
	location, _ := NewLocation("location-1", "project-1", region, LocationCity, "雾港城", "", now)
	link, err := NewLocationLink(location, LocationTargetChapter, "chapter-1", "主场景", now)
	if err != nil || link.TargetID != "chapter-1" {
		t.Fatalf("link = %#v, error = %v", link, err)
	}
	if _, err := NewLocationLink(location, LocationTargetType("unknown"), "target-1", "", now); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("invalid target error = %v", err)
	}
}

func TestItemTransferUpdatesCurrentHolderAndKeepsHistoryRecord(t *testing.T) {
	now := time.Unix(100, 0)
	item, err := NewItem("item-1", "project-1", "问天剑", "宗门信物", now)
	if err != nil {
		t.Fatal(err)
	}
	updated, transfer, err := item.Transfer("transfer-1", HolderCharacter, "character-1", "chapter-1", "秘境中获得", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if updated.HolderType != HolderCharacter || updated.HolderID != "character-1" {
		t.Fatalf("updated item = %#v", updated)
	}
	if transfer.ItemID != item.ID || transfer.ToType != HolderCharacter || transfer.ToID != "character-1" || transfer.FromID != "" {
		t.Fatalf("transfer = %#v", transfer)
	}
	if _, _, err := updated.Transfer("transfer-2", HolderCharacter, "", "", "", now); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("blank holder error = %v", err)
	}
}

func TestWorldRuleValidatesScope(t *testing.T) {
	now := time.Unix(100, 0)
	projectRule, err := NewWorldRule("rule-1", "project-1", "灵气守恒", "灵气不会凭空产生", RuleScopeProject, "", now)
	if err != nil || projectRule.ScopeID != "" {
		t.Fatalf("project rule = %#v, error = %v", projectRule, err)
	}
	if _, err := NewWorldRule("rule-2", "project-1", "禁空", "无法飞行", RuleScopeRegion, "", now); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("region rule without scope error = %v", err)
	}
	if _, err := NewWorldRule("rule-3", "project-1", "错误规则", "项目规则不应带范围", RuleScopeProject, "region-1", now); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("project rule with scope error = %v", err)
	}
}

func TestForeshadowingHookRequiresResolvedChapterWhenResolved(t *testing.T) {
	now := time.Unix(100, 0)
	hook, err := NewForeshadowingHook("hook-1", "project-1", "断剑之谜", "主角得到断剑", "chapter-1", "chapter-8", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := hook.Update("断剑之谜", "主角得到断剑", "chapter-1", "chapter-8", "", HookResolved, "真相揭晓", now); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("resolved hook without chapter error = %v", err)
	}
	resolved, err := hook.Update("断剑之谜", "主角得到断剑", "chapter-1", "chapter-8", "chapter-7", HookResolved, "真相揭晓", now)
	if err != nil || resolved.ResolvedChapterID != "chapter-7" {
		t.Fatalf("resolved hook = %#v, error = %v", resolved, err)
	}
}

func TestWorldAssetsRejectInvalidProfileUpdates(t *testing.T) {
	now := time.Unix(100, 0)
	root, _ := NewWorldRegion("region-root", "project-1", nil, RegionWorld, "镜海世界", DirectionCenter, now)
	child, _ := NewWorldRegion("region-1", "project-1", &root, RegionTerritory, "北境", DirectionNorth, now)
	if _, err := child.UpdateProfile(&child, RegionTerritory, "循环区域", DirectionNorth, "", "", "", now); !errors.Is(err, ErrInvalidHierarchy) {
		t.Fatalf("cyclic region error = %v", err)
	}
	location, _ := NewLocation("location-1", "project-1", child, LocationCity, "雾港城", "", now)
	if _, err := location.UpdateProfile(root, LocationKind("unknown"), "雾港", "", LocationActive, now); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("invalid location kind error = %v", err)
	}
	item, _ := NewItem("item-1", "project-1", "问天剑", "", now)
	if _, err := item.UpdateProfile("问天剑", "", ItemStatus("unknown"), now); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("invalid item status error = %v", err)
	}
	rule, _ := NewWorldRule("rule-1", "project-1", "禁空", "无法飞行", RuleScopeProject, "", now)
	if _, err := rule.UpdateProfile("禁空", "无法飞行", RuleScopeRegion, "", RuleActive, now); !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("invalid rule scope error = %v", err)
	}
}
