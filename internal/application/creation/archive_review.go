package creation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

var (
	// ErrArchiveSectionExcluded 表示方案分节被排除：不能只入档半份方案。
	ErrArchiveSectionExcluded = errors.New("creation archive cannot exclude a proposal section")
	// ErrArchiveReferenceMissing 表示保留内容仍引用被排除的实体或节点。
	ErrArchiveReferenceMissing = errors.New("creation archive references an excluded entity")
)

// archiveSelection 是入档使用的显式审阅选择：被排除的单位、纳入确认摘要的审阅版本，
// 以及仍未决定的单位。未选择不是默认全选。
type archiveSelection struct {
	Excluded       map[string]bool
	ReviewVersions map[string]int64
	Missing        []ReviewRefView
	ExcludedRefs   []ReviewRefView
}

// resolveArchiveSelection 读取三个入档候选的审阅记录，计算出排除集合与未决定单位。
func (service *Service) resolveArchiveSelection(
	ctx context.Context,
	candidates []creationdomain.Candidate,
) (archiveSelection, error) {
	selection := archiveSelection{Excluded: map[string]bool{}, ReviewVersions: map[string]int64{}}
	for _, stage := range []creationdomain.Stage{
		creationdomain.StageProposal, creationdomain.StagePeopleWorld, creationdomain.StageRoadmap,
	} {
		current, ok := currentCandidate(candidates, stage)
		if !ok {
			continue
		}
		review, err := service.store.Review(ctx, current.ID)
		if err != nil {
			return archiveSelection{}, fmt.Errorf("读取候选 %s 的审阅记录: %w", current.ID, err)
		}
		if review.SessionID == "" {
			review, err = creationdomain.NewCandidateReview(current, service.clock.Now())
			if err != nil {
				return archiveSelection{}, fmt.Errorf("初始化候选 %s 的审阅记录: %w", current.ID, err)
			}
		}
		selection.ReviewVersions[current.ID] = review.ReviewVersion
		for scope, refs := range reviewRefs(current) {
			for _, ref := range refs {
				switch review.State(ref) {
				case creationdomain.DecisionReject:
					if scope == creationdomain.ReviewScopeSection {
						// 方案分节不能只入档一部分：拒绝分节必须由作者重新接受或重新生成。
						return archiveSelection{}, fmt.Errorf("%w: %s", ErrArchiveSectionExcluded, ref.ID)
					}
					selection.Excluded[ref.ID] = true
					selection.ExcludedRefs = append(selection.ExcludedRefs, ReviewRefView{Scope: string(ref.Scope), ID: ref.ID})
				case creationdomain.DecisionUnreviewed:
					selection.Missing = append(selection.Missing, ReviewRefView{Scope: string(ref.Scope), ID: ref.ID})
				}
			}
		}
	}
	sort.Slice(selection.Missing, func(i, j int) bool { return selection.Missing[i].ID < selection.Missing[j].ID })
	sort.Slice(selection.ExcludedRefs, func(i, j int) bool { return selection.ExcludedRefs[i].ID < selection.ExcludedRefs[j].ID })
	return selection, nil
}

// validateArchiveReview 要求每个适用单位都有明确决定；未决定不能入档。
func validateArchiveReview(selection archiveSelection) error {
	if len(selection.Missing) == 0 {
		return nil
	}
	ids := make([]string, 0, len(selection.Missing))
	for _, ref := range selection.Missing {
		ids = append(ids, ref.ID)
	}
	if len(ids) > 5 {
		ids = append(ids[:5], "…")
	}
	return fmt.Errorf("%w: %s", ErrReviewIncomplete, strings.Join(ids, "、"))
}

// filterArchiveCast 剔除被排除的人物世界实体；保留的关系不得引用被排除人物。
func filterArchiveCast(
	cast agentdomain.StoryCastOutput,
	excluded map[string]bool,
) (agentdomain.StoryCastOutput, error) {
	filtered := cast
	filtered.Characters = make([]agentdomain.CastCharacter, 0, len(cast.Characters))
	retained := map[string]bool{}
	for _, character := range cast.Characters {
		ref := "character:" + strings.TrimSpace(character.ID)
		if excluded[ref] {
			continue
		}
		retained[strings.TrimSpace(character.ID)] = true
		filtered.Characters = append(filtered.Characters, character)
	}
	filtered.Factions = filterEntities(cast.Factions, "faction:", func(item agentdomain.CastFaction) string { return item.Name }, excluded)
	filtered.Locations = filterEntities(cast.Locations, "location:", func(item agentdomain.CastLocation) string { return item.Name }, excluded)
	filtered.WorldRules = filterEntities(cast.WorldRules, "rule:", func(item agentdomain.CastWorldRule) string { return item.Name }, excluded)
	filtered.Relationships = make([]agentdomain.CastRelationship, 0, len(cast.Relationships))
	for _, relation := range cast.Relationships {
		source, target := strings.TrimSpace(relation.SourceCharacterRef), strings.TrimSpace(relation.TargetCharacterRef)
		if retained[source] && retained[target] {
			filtered.Relationships = append(filtered.Relationships, relation)
			continue
		}
		return agentdomain.StoryCastOutput{}, fmt.Errorf("%w: 人物关系 %s 引用被排除的人物", ErrArchiveReferenceMissing, relation.ID)
	}
	if err := filtered.Validate(); err != nil {
		return agentdomain.StoryCastOutput{}, fmt.Errorf("%w: 排除后的人物世界不满足结构要求", creationdomain.ErrStageIncomplete)
	}
	if err := filtered.ValidateStableReferences(); err != nil {
		return agentdomain.StoryCastOutput{}, err
	}
	return filtered, nil
}

func filterEntities[T any](items []T, prefix string, name func(T) string, excluded map[string]bool) []T {
	kept := make([]T, 0, len(items))
	for _, item := range items {
		if excluded[prefix+strings.TrimSpace(name(item))] {
			continue
		}
		kept = append(kept, item)
	}
	return kept
}

// filterArchiveRoadmap 剔除被排除的路线图节点；保留节点的父子引用与章序区间不得指向被排除节点。
func filterArchiveRoadmap(
	road agentdomain.StoryRoadmapOutput,
	excluded map[string]bool,
) (agentdomain.StoryRoadmapOutput, error) {
	filtered := road
	filtered.Volumes = filterEntities(road.Volumes, "volume:", func(item agentdomain.RoadmapVolume) string { return item.ID }, excluded)
	filtered.Arcs = filterEntities(road.Arcs, "arc:", func(item agentdomain.RoadmapArc) string { return item.ID }, excluded)
	filtered.NearTermChapters = filterEntities(road.NearTermChapters, "chapter:", func(item agentdomain.RoadmapChapter) string { return item.ID }, excluded)
	volumes, arcs, chapters := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, item := range filtered.Volumes {
		volumes[strings.TrimSpace(item.ID)] = true
	}
	for _, item := range filtered.Arcs {
		arcs[strings.TrimSpace(item.ID)] = true
	}
	for _, item := range filtered.NearTermChapters {
		chapters[strings.TrimSpace(item.ID)] = true
	}
	for _, arc := range filtered.Arcs {
		if !volumes[strings.TrimSpace(arc.VolumeRef)] {
			return agentdomain.StoryRoadmapOutput{}, fmt.Errorf("%w: 故事弧 %s 的所属卷已被排除", ErrArchiveReferenceMissing, arc.ID)
		}
	}
	for _, chapter := range filtered.NearTermChapters {
		if !arcs[strings.TrimSpace(chapter.ArcRef)] {
			return agentdomain.StoryRoadmapOutput{}, fmt.Errorf("%w: 章节 %s 的所属故事弧已被排除", ErrArchiveReferenceMissing, chapter.ID)
		}
	}
	if excluded["book:"+strings.TrimSpace(road.Book.ID)] {
		return agentdomain.StoryRoadmapOutput{}, fmt.Errorf("%w: 全书节点不能单独排除", ErrArchiveReferenceMissing)
	}
	if err := filtered.Validate(); err != nil {
		return agentdomain.StoryRoadmapOutput{}, fmt.Errorf("%w: 排除后的路线图不满足结构要求", creationdomain.ErrStageIncomplete)
	}
	return filtered, nil
}

// archiveOperationViews 把入档计划映射为作者可读的资产操作，预览不暴露内部 Specs 结构。
func archiveOperationViews(specs []revisiondomain.PatchOperationSpec) []ArchiveOperationView {
	views := make([]ArchiveOperationView, 0, len(specs))
	for _, spec := range specs {
		action := "create"
		if strings.TrimSpace(spec.BeforeValue) != "" {
			action = "update"
		}
		views = append(views, ArchiveOperationView{
			Kind: string(spec.Kind), Action: action, Summary: archiveOperationSummary(spec.AfterValue, string(spec.Kind)),
		})
	}
	return views
}

// archiveOperationSummary 从载荷中取出作者可读的名称，缺失时回落到操作种类。
func archiveOperationSummary(after, kind string) string {
	decoded := map[string]any{}
	if json.Unmarshal([]byte(after), &decoded) != nil {
		return kind
	}
	for _, key := range []string{"title", "name", "statement"} {
		if value, ok := decoded[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return kind
}
