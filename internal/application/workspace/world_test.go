package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

func TestCreateAndUpdateCharacterUsesStoryCapability(t *testing.T) {
	project := mustProject(t, "project", "长夜织境")
	store := newFakeStore(project)
	licenses := &fakeLicense{}
	service := openTestServiceWithLicense(t, store, &sequenceIDs{ids: []string{"character-1"}}, licenses)

	created, err := service.CreateCharacter(context.Background(), CharacterInput{
		Name: " 林默 ", Aliases: []string{"阿默"}, Role: "protagonist", Summary: "调查员",
		Motivation: "寻找导师", Fear: "失去同伴", Secret: "曾经背叛", Arc: "学会信任",
	})
	if err != nil {
		t.Fatal(err)
	}
	if licenses.lastCapability != license.EditStory || created.ID != "character-1" || created.Name != "林默" || created.Status != "active" {
		t.Fatalf("created = %#v, capability = %q", created, licenses.lastCapability)
	}

	updated, err := service.UpdateCharacter(context.Background(), created.ID, CharacterInput{
		Name: "林默之", Aliases: []string{"林默"}, Role: "supporting", Summary: "前调查员",
		Motivation: "重建守夜司", Status: "inactive",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Name != "林默之" || updated.Status != "inactive" || store.characters[created.ID].Summary != "前调查员" {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestCreateWorldEntitiesPreservesExplicitInitialStatus(t *testing.T) {
	project := mustProject(t, "project", "长夜织境")
	store := newFakeStore(project)
	service := openTestService(t, store, &sequenceIDs{ids: []string{"character", "faction"}})

	character, err := service.CreateCharacter(context.Background(), CharacterInput{
		Name: "旧王", Role: "antagonist", Status: "deceased",
	})
	if err != nil {
		t.Fatal(err)
	}
	faction, err := service.CreateFaction(context.Background(), FactionInput{
		Name: "旧王庭", Status: "disbanded",
	})
	if err != nil {
		t.Fatal(err)
	}
	if character.Status != "deceased" || faction.Status != "disbanded" {
		t.Fatalf("initial statuses = character:%q faction:%q", character.Status, faction.Status)
	}
}

func TestStoryWorldProjectsGraphAppearancesAndConsistencyIssues(t *testing.T) {
	project := mustProject(t, "project", "长夜织境")
	store := newFakeStore(project)
	first := mustWorldCharacter(t, "first", project.ID, "林默", worlddomain.RoleProtagonist)
	second := mustWorldCharacter(t, "second", project.ID, "林默", worlddomain.RoleSupporting)
	store.characters[first.ID], store.characters[second.ID] = first, second
	faction, _ := worlddomain.NewFaction("faction", project.ID, "守夜人", "", "守护港城", time.Unix(100, 0))
	store.factions[faction.ID] = faction
	membership, _ := worlddomain.NewMembership(first, faction, "调查员", worlddomain.MembershipActive, time.Unix(101, 0))
	store.memberships[first.ID+"\x00"+faction.ID] = membership
	relation, _ := worlddomain.NewRelationship("relation", first, second, worlddomain.RelationshipAlly, "战友", 60, "", "互相信任", time.Unix(102, 0))
	store.relationships[relation.ID] = relation
	chapter := mustChapter(t, "chapter", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	store.chapterAppearances[chapter.ID+"\x00"+first.ID] = worlddomain.ChapterAppearance{ProjectID: project.ID, ChapterID: chapter.ID, CharacterID: first.ID, Role: worlddomain.AppearancePOV}
	store.sceneAppearances["scene\x00"+first.ID] = worlddomain.SceneAppearance{ProjectID: project.ID, SceneCardID: "scene", CharacterID: first.ID, Role: worlddomain.AppearancePresent}
	service := openTestService(t, store, &sequenceIDs{})

	view, err := service.StoryWorld(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Characters) != 2 || len(view.Factions) != 1 || len(view.Memberships) != 1 || len(view.Relationships) != 1 || len(view.ChapterAppearances) != 1 || len(view.SceneAppearances) != 1 {
		t.Fatalf("world = %#v", view)
	}
	if len(view.ConsistencyIssues) != 1 || view.ConsistencyIssues[0].Code != "duplicate_character_name" || len(view.ConsistencyIssues[0].EntityIDs) != 2 {
		t.Fatalf("issues = %#v", view.ConsistencyIssues)
	}
}

func TestWorldCommandsValidateRelationshipAndAppearanceReferences(t *testing.T) {
	project := mustProject(t, "project", "长夜织境")
	store := newFakeStore(project)
	character := mustWorldCharacter(t, "character", project.ID, "林默", worlddomain.RoleProtagonist)
	other := mustWorldCharacter(t, "other", project.ID, "顾青", worlddomain.RoleSupporting)
	store.characters[character.ID], store.characters[other.ID] = character, other
	chapter := mustChapter(t, "chapter", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	store.sceneCards["scene"] = storydomain.SceneCard{ID: "scene", ProjectID: project.ID, ChapterID: chapter.ID, Title: "雨夜"}
	service := openTestService(t, store, &sequenceIDs{ids: []string{"relation"}})

	if _, err := service.CreateRelationship(context.Background(), RelationshipInput{SourceCharacterID: character.ID, TargetCharacterID: "missing", Kind: "ally"}); !errors.Is(err, worlddomain.ErrInvalidReference) {
		t.Fatalf("missing target error = %v", err)
	}
	if _, err := service.CreateRelationship(context.Background(), RelationshipInput{SourceCharacterID: character.ID, TargetCharacterID: other.ID, Kind: "ally", Label: "战友", Strength: 70}); err != nil {
		t.Fatal(err)
	}
	if err := service.LinkCharacterToChapter(context.Background(), AppearanceInput{CharacterID: character.ID, TargetID: chapter.ID, Role: "pov"}); err != nil {
		t.Fatal(err)
	}
	if err := service.LinkCharacterToScene(context.Background(), AppearanceInput{CharacterID: character.ID, TargetID: "scene", Role: "present"}); err != nil {
		t.Fatal(err)
	}
	if err := service.LinkCharacterToChapter(context.Background(), AppearanceInput{CharacterID: character.ID, TargetID: "missing", Role: "present"}); !errors.Is(err, worlddomain.ErrInvalidReference) {
		t.Fatalf("missing chapter error = %v", err)
	}
}

func TestWorldLifecycleCommandsManageFactionsMembershipsAndArchives(t *testing.T) {
	project := mustProject(t, "project", "长夜织境")
	store := newFakeStore(project)
	chapter := mustChapter(t, "chapter", project.ID, "第一章", 1)
	store.chapters[chapter.ID] = chapter
	store.sceneCards["scene"] = storydomain.SceneCard{ID: "scene", ProjectID: project.ID, ChapterID: chapter.ID, Title: "雨夜"}
	service := openTestService(t, store, &sequenceIDs{ids: []string{"character", "faction", "relationship"}})

	character, err := service.CreateCharacter(context.Background(), CharacterInput{Name: "林默", Role: "protagonist"})
	if err != nil {
		t.Fatal(err)
	}
	faction, err := service.CreateFaction(context.Background(), FactionInput{Name: "守夜人", Summary: "秘密组织", Goal: "守护港城"})
	if err != nil {
		t.Fatal(err)
	}
	updatedFaction, err := service.UpdateFaction(context.Background(), faction.ID, FactionInput{Name: "守夜司", Summary: "重组组织", Goal: "封锁裂隙", Status: "inactive"})
	if err != nil || updatedFaction.Name != "守夜司" {
		t.Fatalf("updated faction = %#v, error = %v", updatedFaction, err)
	}
	if _, err := service.UpsertMembership(context.Background(), MembershipInput{CharacterID: character.ID, FactionID: faction.ID, Role: "调查员", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteMembership(context.Background(), character.ID, faction.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.ArchiveFaction(context.Background(), faction.ID); err != nil {
		t.Fatal(err)
	}

	if err := service.LinkCharacterToChapter(context.Background(), AppearanceInput{CharacterID: character.ID, TargetID: chapter.ID, Role: "pov"}); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlinkCharacterFromChapter(context.Background(), character.ID, chapter.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.LinkCharacterToScene(context.Background(), AppearanceInput{CharacterID: character.ID, TargetID: "scene", Role: "present"}); err != nil {
		t.Fatal(err)
	}
	if err := service.UnlinkCharacterFromScene(context.Background(), character.ID, "scene"); err != nil {
		t.Fatal(err)
	}

	other := mustWorldCharacter(t, "other", project.ID, "顾青", worlddomain.RoleSupporting)
	store.characters[other.ID] = other
	relationship, err := service.CreateRelationship(context.Background(), RelationshipInput{SourceCharacterID: character.ID, TargetCharacterID: other.ID, Kind: "ally"})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ArchiveRelationship(context.Background(), relationship.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.ArchiveCharacter(context.Background(), character.ID); err != nil {
		t.Fatal(err)
	}
	if store.characters[character.ID].DeletedAt == nil || store.factions[faction.ID].DeletedAt == nil || store.relationships[relationship.ID].DeletedAt == nil {
		t.Fatalf("archives = character:%#v faction:%#v relationship:%#v", store.characters[character.ID], store.factions[faction.ID], store.relationships[relationship.ID])
	}
}

func mustWorldCharacter(t *testing.T, id, projectID, name string, role worlddomain.CharacterRole) worlddomain.Character {
	t.Helper()
	character, err := worlddomain.NewCharacter(id, projectID, name, nil, role, "", "", "", "", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	return character
}
