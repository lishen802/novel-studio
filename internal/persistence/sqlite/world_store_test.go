package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
	worlddomain "github.com/lishen802/novel-studio/internal/domain/world"
)

func TestStorePersistsAndUpdatesStoryWorldProfiles(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Unix(100, 0)

	character := mustCharacter(t, "character-1", project.ID, "林默", worlddomain.RoleProtagonist)
	character.Aliases = []string{"阿默"}
	character.Summary = "守夜人调查员"
	if err := store.CreateCharacter(ctx, character); err != nil {
		t.Fatal(err)
	}
	faction, err := worlddomain.NewFaction("faction-1", project.ID, "守夜人", "城市秘密组织", "保护港城", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateFaction(ctx, faction); err != nil {
		t.Fatal(err)
	}
	membership, err := worlddomain.NewMembership(character, faction, "调查员", worlddomain.MembershipActive, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertMembership(ctx, membership); err != nil {
		t.Fatal(err)
	}

	updated, err := character.UpdateProfile("林默之", []string{"林默", "阿默"}, worlddomain.RoleSupporting, "前守夜人", "寻找真相", "失去伙伴", "曾经背叛", "重新信任", worlddomain.CharacterInactive, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateCharacter(ctx, updated); err != nil {
		t.Fatal(err)
	}
	updatedFaction, err := faction.UpdateProfile("守夜司", "重组后的组织", "封锁裂隙", worlddomain.FactionInactive, time.Unix(201, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateFaction(ctx, updatedFaction); err != nil {
		t.Fatal(err)
	}

	characters, err := store.ListCharacters(ctx)
	if err != nil || len(characters) != 1 || characters[0].Name != "林默之" || characters[0].Aliases[1] != "阿默" || characters[0].Summary != "前守夜人" {
		t.Fatalf("characters = %#v, error = %v", characters, err)
	}
	factions, err := store.ListFactions(ctx)
	if err != nil || len(factions) != 1 || factions[0].Name != "守夜司" {
		t.Fatalf("factions = %#v, error = %v", factions, err)
	}
	memberships, err := store.ListMemberships(ctx)
	if err != nil || len(memberships) != 1 || memberships[0].Role != "调查员" {
		t.Fatalf("memberships = %#v, error = %v", memberships, err)
	}
	gotCharacter, err := store.Character(ctx, character.ID)
	if err != nil || gotCharacter.Name != "林默之" {
		t.Fatalf("character = %#v, error = %v", gotCharacter, err)
	}
	gotFaction, err := store.Faction(ctx, faction.ID)
	if err != nil || gotFaction.Name != "守夜司" {
		t.Fatalf("faction = %#v, error = %v", gotFaction, err)
	}
	if err := store.ArchiveFaction(ctx, faction.ID, time.Unix(300, 0)); !errors.Is(err, worlddomain.ErrReferenced) {
		t.Fatalf("archive referenced faction error = %v", err)
	}
	if err := store.DeleteMembership(ctx, character.ID, faction.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.ArchiveFaction(ctx, faction.ID, time.Unix(301, 0)); err != nil {
		t.Fatal(err)
	}
}

func TestStorePersistsRelationshipsAndProtectsReferencedEntities(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	source := mustCharacter(t, "source", project.ID, "林默", worlddomain.RoleProtagonist)
	target := mustCharacter(t, "target", project.ID, "顾青", worlddomain.RoleSupporting)
	for _, character := range []worlddomain.Character{source, target} {
		if err := store.CreateCharacter(ctx, character); err != nil {
			t.Fatal(err)
		}
	}
	relation, err := worlddomain.NewRelationship("relation", source, target, worlddomain.RelationshipAlly, "战友", 80, "公开合作", "互相信任", time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRelationship(ctx, relation); err != nil {
		t.Fatal(err)
	}
	relations, err := store.ListRelationships(ctx)
	if err != nil || len(relations) != 1 || relations[0].TargetCharacterID != target.ID {
		t.Fatalf("relationships = %#v, error = %v", relations, err)
	}

	if err := store.ArchiveCharacter(ctx, source.ID, time.Unix(200, 0)); !errors.Is(err, worlddomain.ErrReferenced) {
		t.Fatalf("archive referenced character error = %v", err)
	}
	if err := store.ArchiveRelationship(ctx, relation.ID, time.Unix(201, 0)); err != nil {
		t.Fatal(err)
	}
	if err := store.ArchiveCharacter(ctx, source.ID, time.Unix(202, 0)); err != nil {
		t.Fatal(err)
	}
	characters, err := store.ListCharacters(ctx)
	if err != nil || len(characters) != 1 || characters[0].ID != target.ID {
		t.Fatalf("active characters = %#v, error = %v", characters, err)
	}
}

func TestStoreLinksCharactersToChaptersAndSceneCards(t *testing.T) {
	store, project := createTestStore(t)
	defer store.Close()
	ctx := context.Background()
	character := mustCharacter(t, "character", project.ID, "林默", worlddomain.RoleProtagonist)
	if err := store.CreateCharacter(ctx, character); err != nil {
		t.Fatal(err)
	}
	chapter := mustChapter(t, "chapter", project.ID, "第一章", 1)
	if err := store.CreateChapter(ctx, chapter); err != nil {
		t.Fatal(err)
	}
	book := mustOutlineNode(t, "book", project.ID, nil, storydomain.LevelBook, "全书", 1)
	volume := mustOutlineNode(t, "volume", project.ID, &book, storydomain.LevelVolume, "第一卷", 1)
	arc := mustOutlineNode(t, "arc", project.ID, &volume, storydomain.LevelArc, "主线", 1)
	chapterNode := mustOutlineNode(t, "chapter-node", project.ID, &arc, storydomain.LevelChapter, "第一章", 1)
	chapterNode.ChapterID = chapter.ID
	sceneNode := mustOutlineNode(t, "scene-node", project.ID, &chapterNode, storydomain.LevelScene, "雨夜", 1)
	for _, node := range []storydomain.OutlineNode{book, volume, arc, chapterNode, sceneNode} {
		if err := store.CreateOutlineNode(ctx, node); err != nil {
			t.Fatal(err)
		}
	}
	card, err := storydomain.NewSceneCard("scene-card", project.ID, chapter.ID, sceneNode.ID, "雨夜", 1, time.Unix(102, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSceneCard(ctx, card); err != nil {
		t.Fatal(err)
	}

	chapterAppearance, _ := worlddomain.NewChapterAppearance(project.ID, character.ID, chapter.ID, worlddomain.AppearancePOV, time.Unix(103, 0))
	sceneAppearance, _ := worlddomain.NewSceneAppearance(project.ID, character.ID, card.ID, worlddomain.AppearancePresent, time.Unix(104, 0))
	if err := store.AddChapterAppearance(ctx, chapterAppearance); err != nil {
		t.Fatal(err)
	}
	if err := store.AddSceneAppearance(ctx, sceneAppearance); err != nil {
		t.Fatal(err)
	}
	chapterLinks, err := store.ListChapterAppearances(ctx)
	if err != nil || len(chapterLinks) != 1 || chapterLinks[0].Role != worlddomain.AppearancePOV {
		t.Fatalf("chapter appearances = %#v, error = %v", chapterLinks, err)
	}
	sceneLinks, err := store.ListSceneAppearances(ctx)
	if err != nil || len(sceneLinks) != 1 || sceneLinks[0].SceneCardID != card.ID {
		t.Fatalf("scene appearances = %#v, error = %v", sceneLinks, err)
	}
	if err := store.ArchiveCharacter(ctx, character.ID, time.Unix(200, 0)); !errors.Is(err, worlddomain.ErrReferenced) {
		t.Fatalf("archive character with appearances error = %v", err)
	}
	if err := store.RemoveChapterAppearance(ctx, character.ID, chapter.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.RemoveSceneAppearance(ctx, character.ID, card.ID); err != nil {
		t.Fatal(err)
	}
	if err := store.ArchiveCharacter(ctx, character.ID, time.Unix(201, 0)); err != nil {
		t.Fatal(err)
	}
}

func mustCharacter(t *testing.T, id, projectID, name string, role worlddomain.CharacterRole) worlddomain.Character {
	t.Helper()
	character, err := worlddomain.NewCharacter(id, projectID, name, nil, role, "", "", "", "", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	return character
}
