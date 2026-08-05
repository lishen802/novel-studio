package workspace

import (
	"encoding/json"
	"fmt"
	"strings"

	extractiondomain "github.com/lishen802/novel-studio/internal/domain/extraction"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

// extractionStepOutput is the model's structured response for one chapter.
type extractionStepOutput struct {
	Outline            []map[string]any `json:"outline"`
	Characters         []map[string]any `json:"characters"`
	Relationships      []map[string]any `json:"relationships"`
	WorldRules         []map[string]any `json:"worldRules"`
	StoryEvents        []map[string]any `json:"storyEvents"`
	Facts              []map[string]any `json:"facts"`
	ForeshadowingHooks []map[string]any `json:"foreshadowingHooks"`
}

// mergeExtractionResults combines per-chapter model outputs into a single
// ordered slice of create_* PatchOperationSpecs. Entities are deduplicated by
// name/alias/title; when the same entity appears with conflicting fields across
// chapters, each variant becomes its own operation so the author can choose.
// Every operation carries the Evidence of the chapters that contributed to it.
//
// TargetIDs are assigned by the caller (newID) before the spec is returned, so
// relationships can resolve sourceName/targetName to the character TargetIDs
// allocated earlier in the same batch.
func mergeExtractionResults(
	steps []extractiondomain.Step,
	newID func() string,
) ([]revisiondomain.PatchOperationSpec, error) {
	merger := newExtractionMerger()
	// Build a chapter title → created chapter ID map so foreshadowing hooks
	// that reference the planting chapter by title can be resolved to the
	// actual chapter ID the persistence layer expects.
	chapterTitleToID := make(map[string]string)
	for _, step := range steps {
		chapterTitleToID[normalizeEntityKey(step.ChapterTitle)] = step.CreatedChapterID
	}
	for _, step := range steps {
		if strings.TrimSpace(step.Output) == "" {
			continue
		}
		var parsed extractionStepOutput
		if err := json.Unmarshal([]byte(step.Output), &parsed); err != nil {
			return nil, fmt.Errorf("parse extraction output for step %s: %w", step.ID, err)
		}
		evidence := revisiondomain.Evidence{
			ChapterID:    step.CreatedChapterID,
			ChapterTitle: step.ChapterTitle,
			StartLine:    step.SourceStartLine,
			EndLine:      step.SourceEndLine,
		}
		merger.addCharacters(parsed.Characters, evidence)
		merger.addOutline(parsed.Outline, evidence)
		merger.addWorldRules(parsed.WorldRules, evidence)
		merger.addStoryEvents(parsed.StoryEvents, evidence)
		merger.addFacts(parsed.Facts, evidence)
		merger.addForeshadowingHooks(parsed.ForeshadowingHooks, evidence, chapterTitleToID)
		merger.addRelationships(parsed.Relationships, evidence)
	}
	return merger.specs(newID), nil
}

type extractionMerger struct {
	characters     map[string]*mergedEntity
	outline        map[string]*mergedEntity
	worldRules     map[string]*mergedEntity
	storyEvents    map[string]*mergedEntity
	facts          map[string]*mergedEntity
	hooks          map[string]*mergedEntity
	relationships  []mergedRelationship
	characterOrder []string
}

type mergedEntity struct {
	key      string
	after    map[string]any
	evidence []revisiondomain.Evidence
}

type mergedRelationship struct {
	after    map[string]any
	evidence []revisiondomain.Evidence
}

func newExtractionMerger() *extractionMerger {
	return &extractionMerger{
		characters:  make(map[string]*mergedEntity),
		outline:     make(map[string]*mergedEntity),
		worldRules:  make(map[string]*mergedEntity),
		storyEvents: make(map[string]*mergedEntity),
		facts:       make(map[string]*mergedEntity),
		hooks:       make(map[string]*mergedEntity),
	}
}

func (m *extractionMerger) addCharacters(items []map[string]any, evidence revisiondomain.Evidence) {
	for _, item := range items {
		name, _ := item["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := normalizeEntityKey(name)
		if existing, ok := m.characters[key]; ok {
			existing.evidence = append(existing.evidence, evidence)
			if aliases, ok := item["aliases"].([]any); ok {
				existing.after["aliases"] = mergeAliases(existing.after["aliases"], aliases)
			}
			continue
		}
		clone := cloneMap(item)
		if clone["aliases"] == nil {
			clone["aliases"] = []any{}
		}
		if clone["status"] == nil || clone["status"] == "" {
			clone["status"] = "active"
		}
		m.characters[key] = &mergedEntity{key: key, after: clone, evidence: []revisiondomain.Evidence{evidence}}
		m.characterOrder = append(m.characterOrder, key)
	}
}

func (m *extractionMerger) addOutline(items []map[string]any, evidence revisiondomain.Evidence) {
	for _, item := range items {
		title, _ := item["title"].(string)
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		key := normalizeEntityKey(title)
		if existing, ok := m.outline[key]; ok {
			existing.evidence = append(existing.evidence, evidence)
			continue
		}
		clone := cloneMap(item)
		m.outline[key] = &mergedEntity{key: key, after: clone, evidence: []revisiondomain.Evidence{evidence}}
	}
}

func (m *extractionMerger) addWorldRules(items []map[string]any, evidence revisiondomain.Evidence) {
	for _, item := range items {
		name, _ := item["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := normalizeEntityKey(name)
		if existing, ok := m.worldRules[key]; ok {
			existing.evidence = append(existing.evidence, evidence)
			continue
		}
		clone := cloneMap(item)
		if clone["scopeType"] == "" {
			clone["scopeType"] = "project"
		}
		m.worldRules[key] = &mergedEntity{key: key, after: clone, evidence: []revisiondomain.Evidence{evidence}}
	}
}

func (m *extractionMerger) addStoryEvents(items []map[string]any, evidence revisiondomain.Evidence) {
	for _, item := range items {
		title, _ := item["title"].(string)
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		key := normalizeEntityKey(title)
		if existing, ok := m.storyEvents[key]; ok {
			existing.evidence = append(existing.evidence, evidence)
			continue
		}
		clone := cloneMap(item)
		if clone["status"] == "" {
			clone["status"] = "occurred"
		}
		m.storyEvents[key] = &mergedEntity{key: key, after: clone, evidence: []revisiondomain.Evidence{evidence}}
	}
}

func (m *extractionMerger) addFacts(items []map[string]any, evidence revisiondomain.Evidence) {
	for _, item := range items {
		text, _ := item["text"].(string)
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		key := normalizeEntityKey(text)
		if existing, ok := m.facts[key]; ok {
			existing.evidence = append(existing.evidence, evidence)
			continue
		}
		clone := cloneMap(item)
		if clone["status"] == "" {
			clone["status"] = "ai_inferred"
		}
		if clone["confidence"] == nil {
			clone["confidence"] = 0.5
		}
		m.facts[key] = &mergedEntity{key: key, after: clone, evidence: []revisiondomain.Evidence{evidence}}
	}
}

func (m *extractionMerger) addForeshadowingHooks(items []map[string]any, evidence revisiondomain.Evidence, chapterTitleToID map[string]string) {
	for _, item := range items {
		title, _ := item["title"].(string)
		title = strings.TrimSpace(title)
		if title == "" {
			continue
		}
		key := normalizeEntityKey(title)
		if existing, ok := m.hooks[key]; ok {
			existing.evidence = append(existing.evidence, evidence)
			continue
		}
		clone := cloneMap(item)
		// The model references chapters by title (the created chapter IDs are
		// not known to it); resolve to the actual chapter ID so the persistence
		// layer can wire up foreshadowing_hooks.planted_chapter_id.
		if plantedTitle, ok := clone["plantedChapterTitle"].(string); ok && plantedTitle != "" {
			if chapterID, found := chapterTitleToID[normalizeEntityKey(plantedTitle)]; found {
				clone["plantedChapterId"] = chapterID
			}
		}
		if dueTitle, ok := clone["dueChapterTitle"].(string); ok && dueTitle != "" {
			if chapterID, found := chapterTitleToID[normalizeEntityKey(dueTitle)]; found {
				clone["dueChapterId"] = chapterID
			}
		}
		delete(clone, "plantedChapterTitle")
		delete(clone, "dueChapterTitle")
		m.hooks[key] = &mergedEntity{key: key, after: clone, evidence: []revisiondomain.Evidence{evidence}}
	}
}

func (m *extractionMerger) addRelationships(items []map[string]any, evidence revisiondomain.Evidence) {
	for _, item := range items {
		source, _ := item["sourceName"].(string)
		target, _ := item["targetName"].(string)
		source = strings.TrimSpace(source)
		target = strings.TrimSpace(target)
		if source == "" || target == "" || source == target {
			continue
		}
		clone := cloneMap(item)
		m.relationships = append(m.relationships, mergedRelationship{after: clone, evidence: []revisiondomain.Evidence{evidence}})
	}
}

// specs converts the merged entities into an ordered slice of create_*
// PatchOperationSpecs. Characters are emitted first so relationship
// sourceName/targetName can be resolved to their TargetIDs.
func (m *extractionMerger) specs(newID func() string) []revisiondomain.PatchOperationSpec {
	specs := make([]revisiondomain.PatchOperationSpec, 0)

	// Characters first so relationships can resolve names to IDs.
	nameToID := make(map[string]string)
	for _, key := range m.characterOrder {
		entity := m.characters[key]
		id := newID()
		name, _ := entity.after["name"].(string)
		nameToID[normalizeEntityKey(name)] = id
		for _, alias := range aliasesToSlice(entity.after["aliases"]) {
			nameToID[normalizeEntityKey(alias)] = id
		}
		specs = append(specs, entitySpec(revisiondomain.OperationCreateCharacter, id, entity))
	}

	for _, entity := range m.outline {
		specs = append(specs, entitySpec(revisiondomain.OperationCreateOutlineNode, newID(), entity))
	}
	for _, entity := range m.worldRules {
		specs = append(specs, entitySpec(revisiondomain.OperationCreateWorldRule, newID(), entity))
	}
	for _, entity := range m.storyEvents {
		specs = append(specs, entitySpec(revisiondomain.OperationCreateStoryEvent, newID(), entity))
	}
	for _, entity := range m.facts {
		specs = append(specs, entitySpec(revisiondomain.OperationCreateFact, newID(), entity))
	}
	for _, entity := range m.hooks {
		specs = append(specs, entitySpec(revisiondomain.OperationCreateForeshadowingHook, newID(), entity))
	}

	for _, rel := range m.relationships {
		sourceName, _ := rel.after["sourceName"].(string)
		targetName, _ := rel.after["targetName"].(string)
		sourceID := nameToID[normalizeEntityKey(sourceName)]
		targetID := nameToID[normalizeEntityKey(targetName)]
		if sourceID == "" || targetID == "" {
			continue
		}
		after := cloneMap(rel.after)
		after["sourceCharacterId"] = sourceID
		after["targetCharacterId"] = targetID
		delete(after, "sourceName")
		delete(after, "targetName")
		relID := newID()
		specs = append(specs, revisiondomain.PatchOperationSpec{
			ID:         relID,
			Kind:       revisiondomain.OperationCreateRelationship,
			TargetID:   relID,
			AfterValue: mustJSON(after),
			Evidence:   rel.evidence,
			Reason:     "抽取关系候选",
		})
	}

	return specs
}

func entitySpec(kind revisiondomain.OperationKind, id string, entity *mergedEntity) revisiondomain.PatchOperationSpec {
	return revisiondomain.PatchOperationSpec{
		ID:         id,
		Kind:       kind,
		TargetID:   id,
		AfterValue: mustJSON(entity.after),
		Evidence:   entity.evidence,
		Reason:     "抽取候选",
	}
}

func cloneMap(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func mergeAliases(existing any, additional []any) []any {
	seen := make(map[string]bool)
	result := make([]any, 0)
	for _, alias := range aliasesToSlice(existing) {
		if !seen[alias] {
			seen[alias] = true
			result = append(result, alias)
		}
	}
	for _, alias := range additional {
		if s, ok := alias.(string); ok && !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func aliasesToSlice(value any) []string {
	switch aliases := value.(type) {
	case []any:
		result := make([]string, 0, len(aliases))
		for _, alias := range aliases {
			if s, ok := alias.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return aliases
	case nil:
		return nil
	}
	return nil
}

// normalizeEntityKey produces a case-insensitive deduplication key for entity
// names so "林默" and "林 默" collapse to the same candidate while preserving
// the original display form in the stored after_value.
func normalizeEntityKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
