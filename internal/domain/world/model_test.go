package world

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestNewCharacterNormalizesAuthorFacingFields(t *testing.T) {
	now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	character, err := NewCharacter(
		"character-1", "project-1", "  林默  ",
		[]string{" 阿默 ", "林默", "阿默", ""}, RoleProtagonist,
		" 追查导师失踪真相 ", " 害怕再次失去同伴 ", " 曾为敌方传递消息 ", " 从孤立到信任伙伴 ", now,
	)
	if err != nil {
		t.Fatal(err)
	}

	if character.Name != "林默" || character.Motivation != "追查导师失踪真相" {
		t.Fatalf("character = %#v", character)
	}
	if !reflect.DeepEqual(character.Aliases, []string{"阿默"}) {
		t.Fatalf("aliases = %#v", character.Aliases)
	}
	if character.Status != CharacterActive || character.CreatedAt.Location() != time.UTC {
		t.Fatalf("status/timestamps = %#v", character)
	}
}

func TestNewCharacterRejectsInvalidIdentityAndRole(t *testing.T) {
	now := time.Now()
	_, err := NewCharacter("", "project", "林默", nil, RoleProtagonist, "", "", "", "", now)
	if !errors.Is(err, ErrBlankValue) {
		t.Fatalf("blank id error = %v", err)
	}
	_, err = NewCharacter("character", "project", "林默", nil, CharacterRole("narrator"), "", "", "", "", now)
	if !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("role error = %v", err)
	}
}

func TestMembershipRequiresEntitiesFromTheSameProject(t *testing.T) {
	now := time.Now()
	character, err := NewCharacter("character", "project", "林默", nil, RoleProtagonist, "", "", "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	faction, err := NewFaction("faction", "project", "守夜人", "", "守住港城", now)
	if err != nil {
		t.Fatal(err)
	}
	membership, err := NewMembership(character, faction, "调查员", MembershipActive, now)
	if err != nil {
		t.Fatal(err)
	}
	if membership.CharacterID != character.ID || membership.FactionID != faction.ID || membership.Role != "调查员" {
		t.Fatalf("membership = %#v", membership)
	}

	faction.ProjectID = "other-project"
	_, err = NewMembership(character, faction, "调查员", MembershipActive, now)
	if !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("cross-project error = %v", err)
	}
}

func TestRelationshipProtectsGraphInvariants(t *testing.T) {
	now := time.Now()
	source, _ := NewCharacter("source", "project", "林默", nil, RoleProtagonist, "", "", "", "", now)
	target, _ := NewCharacter("target", "project", "顾青", nil, RoleSupporting, "", "", "", "", now)
	relation, err := NewRelationship("relation", source, target, RelationshipAlly, "战友", 75, "公开合作", "彼此信任", now)
	if err != nil {
		t.Fatal(err)
	}
	if relation.SourceCharacterID != source.ID || relation.TargetCharacterID != target.ID || relation.Strength != 75 {
		t.Fatalf("relationship = %#v", relation)
	}

	_, err = NewRelationship("self", source, source, RelationshipAlly, "", 10, "", "", now)
	if !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("self relationship error = %v", err)
	}
	_, err = NewRelationship("strong", source, target, RelationshipEnemy, "", 101, "", "", now)
	if !errors.Is(err, ErrInvalidStrength) {
		t.Fatalf("strength error = %v", err)
	}
}

func TestCharacterAndFactionProfilesCanBeEditedWithoutChangingIdentity(t *testing.T) {
	createdAt := time.Unix(100, 0)
	updatedAt := time.Unix(200, 0)
	character, _ := NewCharacter("character", "project", "林默", nil, RoleProtagonist, "旧动机", "", "", "", createdAt)
	updatedCharacter, err := character.UpdateProfile(" 林默之 ", []string{"林默"}, RoleSupporting, " 调查员 ", "新动机", "失去同伴", "秘密", "重新信任", CharacterInactive, updatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if updatedCharacter.ID != character.ID || updatedCharacter.ProjectID != character.ProjectID || !updatedCharacter.CreatedAt.Equal(character.CreatedAt) {
		t.Fatalf("identity changed: %#v", updatedCharacter)
	}
	if updatedCharacter.Name != "林默之" || updatedCharacter.Summary != "调查员" || updatedCharacter.Status != CharacterInactive || !updatedCharacter.UpdatedAt.Equal(updatedAt.UTC()) {
		t.Fatalf("profile = %#v", updatedCharacter)
	}

	faction, _ := NewFaction("faction", "project", "守夜人", "旧说明", "旧目标", createdAt)
	updatedFaction, err := faction.UpdateProfile(" 守夜司 ", " 新说明 ", " 新目标 ", FactionInactive, updatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if updatedFaction.ID != faction.ID || updatedFaction.Name != "守夜司" || updatedFaction.Status != FactionInactive || !updatedFaction.CreatedAt.Equal(createdAt.UTC()) {
		t.Fatalf("faction = %#v", updatedFaction)
	}
}

func TestAppearancesRequireACharacterAndTarget(t *testing.T) {
	now := time.Now()
	chapter, err := NewChapterAppearance("project", "character", "chapter", AppearancePOV, now)
	if err != nil {
		t.Fatal(err)
	}
	if chapter.Role != AppearancePOV || chapter.ChapterID != "chapter" {
		t.Fatalf("chapter appearance = %#v", chapter)
	}
	scene, err := NewSceneAppearance("project", "character", "scene-card", AppearancePresent, now)
	if err != nil {
		t.Fatal(err)
	}
	if scene.SceneCardID != "scene-card" {
		t.Fatalf("scene appearance = %#v", scene)
	}
	_, err = NewChapterAppearance("project", "character", "", AppearancePresent, now)
	if !errors.Is(err, ErrInvalidReference) {
		t.Fatalf("missing target error = %v", err)
	}
}
