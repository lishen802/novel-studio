package creation

import (
	"context"
	"errors"
	"strings"
	"testing"

	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

// 排除测试用的人物世界载荷：两个势力与两个人物，排除其一后仍满足结构要求。
const archiveExclusionCastPayload = `{"characters":[{"id":"char-1","name":"阿绫","role":"protagonist","aliases":["守关人"],"summary":"潮汐关最后的守将","motivation":"赎罪","fear":"再失一城","secret":"放过过仇敌","arc":"学会取舍"},{"id":"char-2","name":"副将老周","role":"supporting","aliases":[],"summary":"跟随阿绫多年的副将","motivation":"护住守军","fear":"重演旧败","secret":"隐瞒军令","arc":"接受军令之外的判断"}],"relationships":[{"id":"rel-1","source_character_ref":"char-1","target_character_ref":"char-2","kind":"ally","label":"上下级","strength":60}],"factions":[{"name":"守军","summary":"潮汐关的残部","goal":"守住关隘"},{"name":"盐帮","summary":"走私盐铁的帮派","goal":"开通私路"}],"locations":[],"world_rules":[{"name":"潮汐律","description":"潮位决定航路开闭","scope_type":"project"}],"forbidden":[],"omission_reasons":["本方向不需要地点"]}`

// decide 以作者身份对入档候选的某个单位作出决定，供入档选择测试使用。
func decide(t *testing.T, service *Service, candidateID, scope, refID, decision string) {
	t.Helper()
	if _, err := service.SetReview(context.Background(), "project-1", ReviewRequest{
		CandidateID: candidateID, Scope: scope, RefID: refID, Decision: decision,
	}); err != nil {
		t.Fatalf("决定 %s/%s = %v: %v", scope, refID, decision, err)
	}
}

// 未决定的单位阻断入档，且错误指出具体单位。
func TestArchivePreviewRequiresEveryUnitDecided(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	decide(t, service, "people-1", "entity", "rule:潮汐律", "unreviewed")
	_, err = service.PreviewConfirmArchive(ctx, project.ID)
	if !errors.Is(err, ErrReviewIncomplete) || !strings.Contains(err.Error(), "rule:潮汐律") {
		t.Fatalf("未决定单位应阻断入档: %v", err)
	}
	decide(t, service, "people-1", "entity", "rule:潮汐律", "accept")
	if _, err := service.PreviewConfirmArchive(ctx, project.ID); err != nil {
		t.Fatalf("补全决定后应可预览: %v", err)
	}
}

// 被排除的实体不进入正式资产，其余资产照常入档；预览展示排除项与将写入的资产。
func TestArchiveExcludesRejectedEntity(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSessionWithAllPayloads(t, bridge, project.ID, proposalPayload, archiveExclusionCastPayload, roadmapPayload)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	decide(t, service, "people-1", "entity", "faction:盐帮", "reject")
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Excluded) != 1 || preview.Excluded[0].ID != "faction:盐帮" {
		t.Fatalf("预览排除项 = %#v", preview.Excluded)
	}
	factions, characters := 0, 0
	for _, operation := range preview.Operations {
		switch operation.Kind {
		case string(revisiondomain.OperationCreateFaction):
			factions++
		case string(revisiondomain.OperationCreateCharacter):
			characters++
		}
	}
	if factions != 1 || characters != 2 || len(preview.Operations) == 0 || preview.AnchorCount == 0 {
		t.Fatalf("预览操作 factions=%d characters=%d anchors=%d", factions, characters, preview.AnchorCount)
	}
	result, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "exclude-rule",
	})
	if err != nil {
		t.Fatal(err)
	}
	changeSets, err := raw.ListChangeSets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var applied revisiondomain.ChangeSet
	for _, changeSet := range changeSets {
		if changeSet.ID == result.ChangeSetID {
			applied = changeSet
		}
	}
	appliedFactions, appliedCharacters := 0, 0
	for _, operation := range applied.Operations {
		switch operation.Kind {
		case revisiondomain.OperationCreateFaction:
			appliedFactions++
		case revisiondomain.OperationCreateCharacter:
			appliedCharacters++
		}
	}
	if appliedFactions != 1 || appliedCharacters != 2 {
		t.Fatalf("排除后资产数量错误: factions=%d characters=%d", appliedFactions, appliedCharacters)
	}
}

// 保留内容仍引用被排除实体时阻断入档，不静默丢弃引用。
func TestArchiveRejectsReferenceToExcludedEntity(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	payload := strings.Replace(roadmapPayloadV3, `"related_refs":["arc-1"]`, `"related_refs":["arc-1","character:char-1"]`, 1)
	seedArchiveSessionWithSchemas(t, bridge, project.ID, proposalPayload, archiveExclusionCastPayload, payload, 2, 3)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	decide(t, service, "people-1", "entity", "character:char-1", "reject")
	if _, err := service.PreviewConfirmArchive(ctx, project.ID); !errors.Is(err, ErrArchiveReferenceMissing) {
		t.Fatalf("引用被排除人物应阻断: %v", err)
	}
	characters, err := raw.ListCharacters(ctx)
	if err != nil || len(characters) != 0 {
		t.Fatalf("阻断后不应写入正式资产: %v %v", characters, err)
	}
}

// 方案分节不能单独排除：只入档半份方案必须由作者重新接受或重新生成。
func TestArchiveRejectsExcludedProposalSection(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	decide(t, service, "proposal-1", "section", "ending", "reject")
	if _, err := service.PreviewConfirmArchive(ctx, project.ID); !errors.Is(err, ErrArchiveSectionExcluded) {
		t.Fatalf("排除方案分节应阻断: %v", err)
	}
	if raw2, err := raw.ListChangeSets(ctx); err == nil && len(raw2) != 0 {
		t.Fatalf("阻断后不应产生 ChangeSet: %#v", raw2)
	}
}

// 预览后改变审阅决定使确认摘要失效，不能按旧摘要入档。
func TestArchiveConfirmRejectsReviewChangeAfterPreview(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSessionWithAllPayloads(t, bridge, project.ID, proposalPayload, archiveExclusionCastPayload, roadmapPayload)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	decide(t, service, "people-1", "entity", "faction:盐帮", "reject")
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "review-changed",
	}); !errors.Is(err, ErrConfirmationRequired) {
		t.Fatalf("审阅变化后旧摘要应失效: %v", err)
	}
	fresh, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.ConfirmationDigest == preview.ConfirmationDigest {
		t.Fatal("审阅变化必须改变确认摘要")
	}
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: fresh.ConfirmationDigest, IdempotencyKey: "review-changed",
	}); err != nil {
		t.Fatalf("重新预览后应可入档: %v", err)
	}
}

// 新候选版本必须重新审阅，旧版本的接受不进入入档集合。
func TestArchiveRequiresFreshReviewAfterNewVersion(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSession(t, bridge, project.ID)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	edited, err := service.SaveFields(ctx, project.ID, SaveFieldsRequest{
		CandidateID: "roadmap-1", Changes: map[string]string{"/book/proposition": "守关人必须在亲情与职责间选择，且代价不可逆"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.AcceptCandidate(ctx, project.ID, edited.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewConfirmArchive(ctx, project.ID); !errors.Is(err, ErrReviewIncomplete) {
		t.Fatalf("新版本应重新审阅后才能入档: %v", err)
	}
	if _, err := service.AcceptAllReview(ctx, project.ID, edited.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PreviewConfirmArchive(ctx, project.ID); err != nil {
		t.Fatalf("重新审阅后应可预览: %v", err)
	}
}
