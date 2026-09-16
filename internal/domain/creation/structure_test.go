package creation

import (
	"errors"
	"testing"
	"time"
)

// 新增人物实体时保留稳定标识并生成新版本，原候选不被修改。
func TestEditStructureAddsEntityWithStableIdentity(t *testing.T) {
	now := time.Unix(400, 0)
	cast, err := NewCandidate("cast-1", "session-1", StagePeopleWorld, KindPeopleWorld, 1, "", "run-1", "art-1",
		`{"characters":[{"id":"char-1","name":"阿绫"}],"factions":[],"locations":[],"world_rules":[]}`, "story_cast.schema", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	next, err := cast.EditStructure(StructureEdit{
		Collection: "/characters", Operation: StructureAdd, EntityKey: "char-2",
		Value: `{"id":"char-2","name":"老周","strength":60}`,
	}, "cast-2", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if next.Version != 2 || next.ParentCandidateID != cast.ID || next.Origin != OriginAuthor {
		t.Fatalf("next = %#v", next)
	}
	if cast.PayloadJSON != `{"characters":[{"id":"char-1","name":"阿绫"}],"factions":[],"locations":[],"world_rules":[]}` {
		t.Fatal("baseline candidate must not change")
	}
	value, err := next.FieldString("/characters/@char-2/name")
	if err != nil || value != "老周" {
		t.Fatalf("added entity = %q, %v", value, err)
	}
	if _, err := next.FieldString("/characters/@char-1/name"); err != nil {
		t.Fatalf("existing entity must be preserved: %v", err)
	}
}

// 删除按稳定标识寻址，不使用数组下标。
func TestEditStructureRemovesEntityByStableIdentity(t *testing.T) {
	now := time.Unix(400, 0)
	cast, err := NewCandidate("cast-1", "session-1", StagePeopleWorld, KindPeopleWorld, 1, "", "run-1", "art-1",
		`{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-2","name":"老周"}],"factions":[],"locations":[],"world_rules":[]}`,
		"story_cast.schema", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	next, err := cast.EditStructure(StructureEdit{
		Collection: "characters", Operation: StructureRemove, EntityKey: "char-2",
	}, "cast-2", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := next.FieldString("/characters/@char-2/name"); !errors.Is(err, ErrFieldMissing) {
		t.Fatalf("removed entity still present: %v", err)
	}
	if value, err := next.FieldString("/characters/@char-1/name"); err != nil || value != "阿绫" {
		t.Fatalf("kept entity = %q, %v", value, err)
	}
	if _, err := cast.EditStructure(StructureEdit{
		Collection: "characters", Operation: StructureRemove, EntityKey: "char-9",
	}, "cast-3", now.Add(time.Second)); !errors.Is(err, ErrStructureEntityMissing) {
		t.Fatalf("missing entity error = %v", err)
	}
}

// 无稳定 ID 的集合使用唯一名称作为标识。
func TestEditStructureUsesNameForKeylessCollections(t *testing.T) {
	now := time.Unix(400, 0)
	cast, err := NewCandidate("cast-1", "session-1", StagePeopleWorld, KindPeopleWorld, 1, "", "run-1", "art-1",
		`{"characters":[{"id":"char-1","name":"阿绫"}],"factions":[{"name":"守军","summary":"残部","goal":"守关"}],"locations":[],"world_rules":[]}`,
		"story_cast.schema", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cast.EditStructure(StructureEdit{
		Collection: "factions", Operation: StructureAdd, EntityKey: "守军", Value: `{"name":"守军","summary":"另一支","goal":"走私"}`,
	}, "cast-2", now.Add(time.Second)); !errors.Is(err, ErrStructureEntityExists) {
		t.Fatalf("duplicate name error = %v", err)
	}
	added, err := cast.EditStructure(StructureEdit{
		Collection: "factions", Operation: StructureAdd, EntityKey: "盐帮", Value: `{"name":"盐帮","summary":"走私盐铁","goal":"开通私路"}`,
	}, "cast-3", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if value, err := added.FieldString("/factions/@盐帮/summary"); err != nil || value != "走私盐铁" {
		t.Fatalf("added faction = %q, %v", value, err)
	}
}

// 新增实体的标识必须与请求一致，避免静默换 ID。
func TestEditStructureRejectsIdentityMismatch(t *testing.T) {
	now := time.Unix(400, 0)
	roadmap, err := NewCandidate("roadmap-1", "session-1", StageRoadmap, KindRoadmap, 1, "", "run-1", "art-1",
		`{"book":{"id":"book-1"},"volumes":[{"id":"vol-1","title":"第一卷"}],"arcs":[],"near_term_chapters":[]}`,
		"story_roadmap.schema", 3, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := roadmap.EditStructure(StructureEdit{
		Collection: "volumes", Operation: StructureAdd, EntityKey: "vol-2", Value: `{"id":"vol-9","title":"第二卷"}`,
	}, "roadmap-2", now.Add(time.Second)); !errors.Is(err, ErrStructureIdentity) {
		t.Fatalf("identity mismatch error = %v", err)
	}
	if _, err := roadmap.EditStructure(StructureEdit{
		Collection: "characters", Operation: StructureAdd, EntityKey: "char-1", Value: `{"id":"char-1"}`,
	}, "roadmap-2", now.Add(time.Second)); !errors.Is(err, ErrStructureUnsupported) {
		t.Fatalf("unsupported collection error = %v", err)
	}
	if _, err := roadmap.EditStructure(StructureEdit{
		Collection: "volumes", Operation: "replace", EntityKey: "vol-1", Value: `{}`,
	}, "roadmap-2", now.Add(time.Second)); !errors.Is(err, ErrBlankValue) {
		t.Fatalf("unknown operation error = %v", err)
	}
}

// 锁定实体的字段受保护：不能通过删除实体绕过锁。
func TestEditStructureCannotBypassEntityLock(t *testing.T) {
	now := time.Unix(400, 0)
	cast, err := NewCandidate("cast-1", "session-1", StagePeopleWorld, KindPeopleWorld, 1, "", "run-1", "art-1",
		`{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-2","name":"老周"}],"factions":[],"locations":[],"world_rules":[]}`,
		"story_cast.schema", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	locked, err := cast.LockField("/characters/@char-2/name", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := locked.EditStructure(StructureEdit{
		Collection: "characters", Operation: StructureRemove, EntityKey: "char-2",
	}, "cast-2", now.Add(2*time.Second)); !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("locked entity removal error = %v", err)
	}
	// 锁定的实体仍然可以被删除以外的编辑影响：其他实体的增删不受影响。
	if _, err := locked.EditStructure(StructureEdit{
		Collection: "characters", Operation: StructureRemove, EntityKey: "char-1",
	}, "cast-3", now.Add(2*time.Second)); err != nil {
		t.Fatalf("unlocked entity removal = %v", err)
	}
}

// 集合不存在时按空集合处理，缺失集合里的首个实体可以直接新增。
func TestEditStructureCreatesMissingCollection(t *testing.T) {
	now := time.Unix(400, 0)
	cast, err := NewCandidate("cast-1", "session-1", StagePeopleWorld, KindPeopleWorld, 1, "", "run-1", "art-1",
		`{"characters":[{"id":"char-1","name":"阿绫"}]}`, "story_cast.schema", 2, now)
	if err != nil {
		t.Fatal(err)
	}
	next, err := cast.EditStructure(StructureEdit{
		Collection: "locations", Operation: StructureAdd, EntityKey: "潮汐关",
		Value: `{"name":"潮汐关","kind":"city","summary":"关隘城"}`,
	}, "cast-2", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if value, err := next.FieldString("/locations/@潮汐关/kind"); err != nil || value != "city" {
		t.Fatalf("created collection = %q, %v", value, err)
	}
}
