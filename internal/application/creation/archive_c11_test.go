package creation

import (
	"context"
	"testing"

	storydomain "github.com/lishen802/novel-studio/internal/domain/story"
)

// c11RoadmapPayload 覆盖 C11：两卷两弧（弧标题相同）、章序跨卷、三种锚点与禁写项。
const c11RoadmapPayload = `{"book":{"id":"book-1","title":"潮汐关","proposition":"守关人必须在亲情与职责间选择","ending":"开城救人","protagonist_arc":"从逃避到承担","inviolable":["潮汐决定航路"]},` +
	`"volumes":[{"id":"vol-1","title":"第一卷","goal":"建立关隘危机","conflict":"敌船压境","payoff":"阿绫作出选择","position":1},` +
	`{"id":"vol-2","title":"第二卷","goal":"私情反噬","conflict":"旧友成敌","payoff":"代价公开","position":2}],` +
	`"arcs":[{"id":"arc-1","volume_ref":"vol-1","title":"开关","causal_chain":"敌船靠岸导致必须抉择","character_change":"阿绫接受代价","reader_expectation":"读者应感到守关人将被亲情撕裂","stage_payoff":"开城救人并失去军职","irreversible_change":"阿绫不再以军令为最高准则","ending_contribution":"为终局选择救人立下先例","commitments":[{"id":"cmt-1","statement":"钟楼必须三响","fulfillment_range":"第一卷结尾","related_refs":["arc-1"]}],"retained_questions":[{"id":"rq-1","statement":"旧令牌从何而来","reason":"留待第二卷"}],"foreshadowing":["旧令牌"],"position":1},` +
	`{"id":"arc-2","volume_ref":"vol-2","title":"开关","causal_chain":"旧友反目导致代价公开","character_change":"阿绫公开承担","reader_expectation":"读者应看到代价无法回避","stage_payoff":"失去军职后的自由身","irreversible_change":"阿绫不再依附军令体系","ending_contribution":"为终局孤立无援铺路","commitments":[{"id":"cmt-2","statement":"旧令牌必须兑现","fulfillment_range":"第二卷中段","related_refs":["arc-2"]}],"retained_questions":[{"id":"rq-2","statement":"旧令牌能否兑现","reason":"指向终局"}],"foreshadowing":["旧令牌"],"position":1}],` +
	`"near_term_chapters":[{"id":"ch-1","arc_ref":"arc-2","title":"旧识","goal":"认出仇敌","conflict":"军令与私情","outcome":"暂时藏匿","anchors":[],"position":1},` +
	`{"id":"ch-2","arc_ref":"arc-1","title":"空船","goal":"发现异常","conflict":"无人登岸","outcome":"阿绫独自登船","anchors":[{"strength":"hard","statement":"钟楼三响"}],"position":2},` +
	`{"id":"ch-3","arc_ref":"arc-2","title":"开关","goal":"作出选择","conflict":"守城与救人","outcome":"开城","anchors":[{"strength":"soft","statement":"潮汐律"}],"position":3}]}`

// C11：同标题的两卷两弧按稳定 ID 映射，跨卷章序与多作用域锚点都无损。
func TestArchiveMapsSameTitleNodesByStableIDAcrossVolumes(t *testing.T) {
	ctx := context.Background()
	raw, project := sqliteCreateStore(t)
	defer raw.Close()
	bridge := NewSQLiteStore(raw)
	seedArchiveSessionWithRoadmapSchema(t, bridge, project.ID, peopleWorldPayload, c11RoadmapPayload, 3)
	service, err := New(Config{Store: bridge, Kernel: newFakeKernel(), Clock: testClock{}, IDs: &testIDs{}})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewConfirmArchive(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConfirmArchive(ctx, ArchiveRequest{
		ProjectID: project.ID, ConfirmationDigest: preview.ConfirmationDigest, IdempotencyKey: "c11",
	}); err != nil {
		t.Fatal(err)
	}

	nodes, err := raw.ListOutlineNodes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	titleIDs := map[string][]string{}
	for _, node := range nodes {
		titleIDs[string(node.Level)+"/"+node.Title] = append(titleIDs[string(node.Level)+"/"+node.Title], node.ID)
	}
	// 同标题的两条故事弧必须映射为两个不同节点，而不是被合并（弧位次按卷内连续编号）。
	if arcs := titleIDs["arc/开关"]; len(arcs) != 2 {
		t.Fatalf("同标题故事弧应映射为两个节点，实际 = %v", arcs)
	}
	if len(titleIDs["volume/第一卷"]) != 1 || len(titleIDs["volume/第二卷"]) != 1 {
		t.Fatalf("两卷未按稳定 ID 映射: %v", titleIDs)
	}
	// 弧挂到各自的卷，章节按 arc_ref 挂到正确的弧（跨卷章序不按出现顺序猜测）。
	volumeIDs := map[string]storydomain.OutlineNode{}
	arcIDs := map[string]storydomain.OutlineNode{}
	for _, node := range nodes {
		switch node.Level {
		case storydomain.LevelVolume:
			volumeIDs[node.ID] = node
		case storydomain.LevelArc:
			arcIDs[node.ID] = node
		}
	}
	if len(volumeIDs) != 2 || len(arcIDs) != 2 {
		t.Fatalf("卷/弧数量 = %d/%d", len(volumeIDs), len(arcIDs))
	}
	linked := map[string]bool{}
	for _, arc := range arcIDs {
		if _, ok := volumeIDs[arc.ParentID]; !ok {
			t.Fatalf("故事弧父引用未落到卷: %+v", arc)
		}
		linked[arc.ParentID] = true
	}
	if len(linked) != 2 {
		t.Fatalf("两卷应各自挂接故事弧: %v", linked)
	}
	chapters := map[string]storydomain.OutlineNode{}
	for _, node := range nodes {
		if node.Level != storydomain.LevelChapter {
			continue
		}
		chapters[node.Title] = node
		if _, ok := arcIDs[node.ParentID]; !ok {
			t.Fatalf("章节父引用未落到故事弧: %+v", node)
		}
	}
	if len(chapters) != 3 {
		t.Fatalf("章节数量 = %d: %v", len(chapters), chapters)
	}
	// 跨卷章序：ch-3 属于第二卷的故事弧，ch-2 属于第一卷的故事弧。
	if chapters["开关"].ParentID == chapters["空船"].ParentID {
		t.Fatalf("跨卷章序未按 arc_ref 区分: %+v", chapters)
	}

	// 三种锚点：方案禁写项、方案硬锚点与章节锚点各自解析到正确作用域。
	anchors, err := raw.ListStoryAnchors(ctx)
	if err != nil {
		t.Fatal(err)
	}
	scopeByStatement := map[string]string{
		"钟楼必须三响": "",
		"现代武器":   "",
		"钟楼三响":   chapters["空船"].ID,
		"潮汐律":    chapters["开关"].ID,
	}
	strengths := map[string]string{
		"钟楼必须三响": "hard", "现代武器": "forbidden", "钟楼三响": "hard", "潮汐律": "soft",
	}
	if len(anchors) != len(strengths) {
		t.Fatalf("锚点数量 = %d, want %d: %+v", len(anchors), len(strengths), anchors)
	}
	for _, anchor := range anchors {
		wantStrength, ok := strengths[anchor.Statement]
		if !ok || anchor.Strength != wantStrength {
			t.Fatalf("锚点强度未保留: %+v", anchor)
		}
		wantScope, hasScope := scopeByStatement[anchor.Statement]
		if !hasScope {
			t.Fatalf("未知锚点: %+v", anchor)
		}
		if wantScope == "" {
			if anchor.ScopeKind != "proposal" {
				t.Fatalf("方案锚点作用域错误: %+v", anchor)
			}
			continue
		}
		if anchor.ScopeKind != "roadmap_node" || anchor.ScopeID != wantScope {
			t.Fatalf("章节锚点未落到正确节点: %+v want=%s", anchor, wantScope)
		}
	}
}
