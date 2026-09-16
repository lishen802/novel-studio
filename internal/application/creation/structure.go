package creation

import (
	"context"
	"strings"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

const (
	castSchemaID      = "story_cast.schema"
	castSchemaVersion = 2
	roadmapSchemaID   = "story_roadmap.schema"
)

// StructureEditRequest 是作者对候选结构的新增或删除命令，payloadValue 只用于新增。
type StructureEditRequest struct {
	CandidateID string `json:"candidateId"`
	Collection  string `json:"collection"`
	Operation   string `json:"operation"`
	EntityKey   string `json:"entityKey"`
	Value       string `json:"value"`
}

// SaveStructure 以作者命令增删候选中的实体：校验当前版本、锁与跨阶段引用后保存新候选版本。
// 与字段编辑一样不调用模型、不自动接受；删除或改写锁定实体直接返回锁错误。
func (service *Service) SaveStructure(ctx context.Context, projectID string, request StructureEditRequest) (creationdomain.Candidate, error) {
	candidate, err := service.CandidatePayload(ctx, projectID, request.CandidateID)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	edit := creationdomain.StructureEdit{
		Collection: request.Collection,
		Operation:  creationdomain.StructureOperation(strings.TrimSpace(request.Operation)),
		EntityKey:  request.EntityKey,
		Value:      request.Value,
	}
	payload, err := candidate.ApplyStructure(edit)
	if err != nil {
		return creationdomain.Candidate{}, err
	}
	if err := service.validateStructureReferences(ctx, projectID, candidate, payload); err != nil {
		return creationdomain.Candidate{}, err
	}
	return service.SaveCandidateDraft(ctx, projectID, candidate.ID, payload)
}

// validateStructureReferences 阻止结构增删破坏跨阶段引用：施删方与引用方两侧都检查。
func (service *Service) validateStructureReferences(
	ctx context.Context,
	projectID string,
	candidate creationdomain.Candidate,
	payload string,
) error {
	session, err := service.loadSession(ctx, projectID)
	if err != nil {
		return err
	}
	if candidate.SessionID != session.ID {
		return ErrSessionNotFound
	}
	candidates, err := service.store.ListCandidates(ctx, session.ID)
	if err != nil {
		return err
	}
	edited := candidate
	edited.PayloadJSON = payload
	switch candidate.Stage {
	case creationdomain.StagePeopleWorld:
		// 只有当前 Schema 的载荷参与跨阶段引用校验；旧载荷不按新规则推算正确性。
		if edited.PayloadSchemaID != castSchemaID || edited.PayloadSchemaVersion != castSchemaVersion {
			return nil
		}
		cast, err := decodeArchivableCast(edited)
		if err != nil {
			return err
		}
		roadmap, ok := currentCandidate(candidates, creationdomain.StageRoadmap)
		if !ok || roadmap.PayloadSchemaVersion < 3 || roadmap.PayloadSchemaID != roadmapSchemaID {
			return nil
		}
		road, err := decodeArchivableRoadmap(roadmap)
		if err != nil {
			return err
		}
		return road.ValidatePlanningReferences(cast)
	case creationdomain.StageRoadmap:
		if edited.PayloadSchemaID != roadmapSchemaID || edited.PayloadSchemaVersion < 3 {
			return nil
		}
		road, err := decodeArchivableRoadmap(edited)
		if err != nil {
			return err
		}
		people, ok := currentCandidate(candidates, creationdomain.StagePeopleWorld)
		if !ok || people.PayloadSchemaID != castSchemaID || people.PayloadSchemaVersion != castSchemaVersion {
			return nil
		}
		cast, err := decodeArchivableCast(people)
		if err != nil {
			return err
		}
		return road.ValidatePlanningReferences(cast)
	default:
		return nil
	}
}

// StructureEntityView 是结构编辑入口中的一个可增删实体。
type StructureEntityView struct {
	Key   string `json:"key"`
	Title string `json:"title"`
}

// StructureCollectionView 描述一个可增删集合及其稳定标识字段。
type StructureCollectionView struct {
	Collection    string                `json:"collection"`
	Title         string                `json:"title"`
	IdentityField string                `json:"identityField"`
	Entities      []StructureEntityView `json:"entities"`
}

// structureCollectionTitles 是集合的中文标签；界面不显示英文字段名。
var structureCollectionTitles = map[string]string{
	"characters": "人物", "relationships": "人物关系", "factions": "势力",
	"locations": "地点", "world_rules": "世界规则",
	"volumes": "卷", "arcs": "故事弧", "near_term_chapters": "近期章节",
}

// projectStructure 从公开载荷投影可增删集合，供驾驶舱渲染实体增删，不暴露原始 JSON。
func projectStructure(candidate creationdomain.Candidate) []StructureCollectionView {
	views := []StructureCollectionView{}
	appendCollection := func(collection, identity string, entities []StructureEntityView) {
		views = append(views, StructureCollectionView{
			Collection: collection, Title: structureCollectionTitles[collection],
			IdentityField: identity, Entities: entities,
		})
	}
	switch candidate.Kind {
	case creationdomain.KindPeopleWorld:
		cast, err := decodeCast(candidate.PayloadJSON)
		if err != nil {
			return views
		}
		characters := make([]StructureEntityView, 0, len(cast.Characters))
		for _, item := range cast.Characters {
			characters = append(characters, StructureEntityView{Key: strings.TrimSpace(item.ID), Title: item.Name})
		}
		relationships := make([]StructureEntityView, 0, len(cast.Relationships))
		for _, item := range cast.Relationships {
			title := strings.TrimSpace(item.Label)
			if title == "" {
				title = item.SourceCharacterRef + " → " + item.TargetCharacterRef
			}
			relationships = append(relationships, StructureEntityView{Key: strings.TrimSpace(item.ID), Title: title})
		}
		factions := make([]StructureEntityView, 0, len(cast.Factions))
		for _, item := range cast.Factions {
			factions = append(factions, StructureEntityView{Key: strings.TrimSpace(item.Name), Title: item.Name})
		}
		locations := make([]StructureEntityView, 0, len(cast.Locations))
		for _, item := range cast.Locations {
			locations = append(locations, StructureEntityView{Key: strings.TrimSpace(item.Name), Title: item.Name})
		}
		rules := make([]StructureEntityView, 0, len(cast.WorldRules))
		for _, item := range cast.WorldRules {
			rules = append(rules, StructureEntityView{Key: strings.TrimSpace(item.Name), Title: item.Name})
		}
		appendCollection("characters", "id", characters)
		appendCollection("relationships", "id", relationships)
		appendCollection("factions", "name", factions)
		appendCollection("locations", "name", locations)
		appendCollection("world_rules", "name", rules)
	case creationdomain.KindRoadmap:
		road, err := decodeRoadmap(candidate.PayloadJSON)
		if err != nil {
			return views
		}
		volumes := make([]StructureEntityView, 0, len(road.Volumes))
		for _, item := range road.Volumes {
			volumes = append(volumes, StructureEntityView{Key: strings.TrimSpace(item.ID), Title: item.Title})
		}
		arcs := make([]StructureEntityView, 0, len(road.Arcs))
		for _, item := range road.Arcs {
			arcs = append(arcs, StructureEntityView{Key: strings.TrimSpace(item.ID), Title: item.Title})
		}
		chapters := make([]StructureEntityView, 0, len(road.NearTermChapters))
		for _, item := range road.NearTermChapters {
			chapters = append(chapters, StructureEntityView{Key: strings.TrimSpace(item.ID), Title: item.Title})
		}
		appendCollection("volumes", "id", volumes)
		appendCollection("arcs", "id", arcs)
		appendCollection("near_term_chapters", "id", chapters)
	}
	return views
}
