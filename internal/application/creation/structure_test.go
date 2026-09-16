package creation

import (
	"context"
	"errors"
	"strings"
	"testing"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

// seedStructureCast 建立人物世界阶段的当前候选，载荷含两个人物与两个势力，便于增删。
func seedStructureCast(t *testing.T, service *Service, store *memoryStore) creationdomain.Candidate {
	t.Helper()
	seedIntentSession(t, service)
	return seedStageCandidate(t, service, store, creationdomain.StagePeopleWorld, archiveExclusionCastPayload)
}

// 结构增删生成新的作者候选版本，基线候选不被改写。
func TestSaveStructureCreatesNewVersion(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	baseline := seedStructureCast(t, service, store)
	added, err := service.SaveStructure(context.Background(), "project-1", StructureEditRequest{
		CandidateID: baseline.ID, Collection: "locations", Operation: "add", EntityKey: "潮汐关",
		Value: `{"name":"潮汐关","kind":"city","summary":"潮汐关隘城"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if added.Version != baseline.Version+1 || added.ParentCandidateID != baseline.ID || added.Origin != creationdomain.OriginAuthor {
		t.Fatalf("added = %#v", added)
	}
	stored, err := store.Candidate(context.Background(), baseline.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != creationdomain.StatusSuperseded {
		t.Fatalf("baseline status = %q", stored.Status)
	}
	view, err := service.Workbench(context.Background(), "project-1", "branch-1")
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for _, collection := range view.Structure {
		for _, entity := range collection.Entities {
			keys[collection.Collection+":"+entity.Key] = true
		}
	}
	if !keys["locations:潮汐关"] || !keys["characters:char-1"] {
		t.Fatalf("structure projection = %#v", keys)
	}
}

// 结构变更不得破坏跨阶段引用：删除仍被路线图引用的人物必须被拒绝。
func TestSaveStructureRejectsRemovingReferencedCharacter(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	payload := strings.Replace(roadmapPayloadV3, `"related_refs":["arc-1"]`, `"related_refs":["arc-1","character:char-1"]`, 1)
	seedArchiveSessionWithRoadmapSchema(t, bridge, project.ID, peopleWorldPayload, payload, 3)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveStructure(ctx, project.ID, StructureEditRequest{
		CandidateID: "people-1", Collection: "characters", Operation: "remove", EntityKey: "char-1",
	}); !errors.Is(err, agentdomain.ErrInvalidTaskOutput) {
		t.Fatalf("删除被引用人物应被拒绝: %v", err)
	}
	candidates, err := bridge.ListCandidates(ctx, "session-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range candidates {
		if candidate.Stage == creationdomain.StagePeopleWorld && candidate.Version > 1 {
			t.Fatalf("拒绝后不应产生新版本: %#v", candidate)
		}
	}
}

// 锁定实体不能被结构增删绕过。
func TestSaveStructureRespectsEntityLock(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	baseline := seedStructureCast(t, service, store)
	locked, err := service.LockCandidateField(context.Background(), "project-1", baseline.ID, "/characters/@char-1/summary")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveStructure(context.Background(), "project-1", StructureEditRequest{
		CandidateID: locked.ID, Collection: "characters", Operation: "remove", EntityKey: "char-1",
	}); !errors.Is(err, creationdomain.ErrFieldLocked) {
		t.Fatalf("锁定实体删除错误 = %v", err)
	}
	// 其他实体不受该锁影响。
	if _, err := service.SaveStructure(context.Background(), "project-1", StructureEditRequest{
		CandidateID: locked.ID, Collection: "factions", Operation: "remove", EntityKey: "盐帮",
	}); err != nil {
		t.Fatalf("未锁定实体删除 = %v", err)
	}
}

// 结构增删不落入未支持的种类，也不接受伪造的实体标识。
func TestSaveStructureRejectsUnsupportedEdits(t *testing.T) {
	service, store, _ := newTestService(t, newFakeKernel(), nil)
	baseline := seedStructureCast(t, service, store)
	if _, err := service.SaveStructure(context.Background(), "project-1", StructureEditRequest{
		CandidateID: baseline.ID, Collection: "volumes", Operation: "add", EntityKey: "vol-2", Value: `{"id":"vol-2"}`,
	}); !errors.Is(err, creationdomain.ErrStructureUnsupported) {
		t.Fatalf("人物世界不接受卷集合: %v", err)
	}
	if _, err := service.SaveStructure(context.Background(), "project-1", StructureEditRequest{
		CandidateID: baseline.ID, Collection: "characters", Operation: "add", EntityKey: "char/2", Value: `{"id":"char/2"}`,
	}); !errors.Is(err, creationdomain.ErrBlankValue) {
		t.Fatalf("非法实体标识: %v", err)
	}
	if _, err := service.SaveStructure(context.Background(), "project-1", StructureEditRequest{
		CandidateID: baseline.ID, Collection: "characters", Operation: "add", EntityKey: "char-2", Value: `{"id":"char-2"}`,
	}); err == nil {
		t.Fatal("不完整人物应被 Schema 校验拒绝")
	}
}
