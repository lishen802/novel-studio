package creation

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
)

// RevisionDiffKind 区分文本修改与结构增删，界面据此决定能否部分采用。
type RevisionDiffKind string

const (
	RevisionDiffText         RevisionDiffKind = "text"
	RevisionDiffAddEntity    RevisionDiffKind = "add_entity"
	RevisionDiffRemoveEntity RevisionDiffKind = "remove_entity"
)

// RevisionScope 描述一次定向修订被显式授权的范围。
// 文本路径之外的任何集合变化都必须在 AllowAdd/AllowRemove 中显式列出才被接受。
type RevisionScope struct {
	Paths       []string
	AllowAdd    []string
	AllowRemove []string
}

// Normalize 返回规范化后的授权集合，空字符串与未知名移除，保持稳定顺序。
func (scope RevisionScope) Normalize() RevisionScope {
	normalized := RevisionScope{
		Paths:       append([]string(nil), scope.Paths...),
		AllowAdd:    normalizeCollections(scope.AllowAdd),
		AllowRemove: normalizeCollections(scope.AllowRemove),
	}
	sort.Strings(normalized.Paths)
	return normalized
}

// AllowsAdd/AllowsRemove 回答某集合是否被显式授权增删。
func (scope RevisionScope) AllowsAdd(collection string) bool {
	return containsCollection(scope.AllowAdd, collection)
}

func (scope RevisionScope) AllowsRemove(collection string) bool {
	return containsCollection(scope.AllowRemove, collection)
}

func normalizeCollections(values []string) []string {
	seen := map[string]bool{}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimPrefix(strings.TrimSpace(value), "/")
		if value == "" || seen[value] {
			continue
		}
		if _, ok := structureIdentityField(value); !ok {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized
}

func containsCollection(values []string, collection string) bool {
	collection = strings.TrimPrefix(strings.TrimSpace(collection), "/")
	for _, value := range values {
		if value == collection {
			return true
		}
	}
	return false
}

// reconcileRevisionStructure 逐键核对基线与被校验载荷：
// 被显式授权的集合按实体集合比较（允许授权范围内的增删，保留项必须逐字段一致且顺序不变），
// 其余键必须完全一致。返回结构差异供作者审阅，不修改任何输入。
func reconcileRevisionStructure(
	baseline map[string]any,
	incoming map[string]any,
	scope RevisionScope,
) ([]TextRevisionDiff, error) {
	keys := make([]string, 0, len(baseline)+len(incoming))
	seen := map[string]bool{}
	for key := range baseline {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range incoming {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	diffs := make([]TextRevisionDiff, 0)
	for _, key := range keys {
		baselineValue, inBaseline := baseline[key]
		incomingValue, inIncoming := incoming[key]
		baselineItems, baselineArray := baselineValue.([]any)
		incomingItems, incomingArray := incomingValue.([]any)
		if !inIncoming || !inBaseline || !baselineArray || !incomingArray {
			if inBaseline != inIncoming || !reflect.DeepEqual(baselineValue, incomingValue) {
				return nil, ErrRevisionScope
			}
			continue
		}
		if !scope.AllowsAdd(key) && !scope.AllowsRemove(key) {
			if !reflect.DeepEqual(baselineValue, incomingValue) {
				return nil, ErrRevisionScope
			}
			continue
		}
		collectionDiffs, err := reconcileRevisionCollection(key, baselineItems, incomingItems, scope)
		if err != nil {
			return nil, err
		}
		diffs = append(diffs, collectionDiffs...)
	}
	return diffs, nil
}

// reconcileRevisionCollection 校验一个集合实体的增删：保留项逐字段一致且顺序不变，
// 新增必须授权并且标识唯一，删除必须授权。
func reconcileRevisionCollection(
	collection string,
	baselineItems, incomingItems []any,
	scope RevisionScope,
) ([]TextRevisionDiff, error) {
	identityField, ok := structureIdentityField(collection)
	if !ok {
		return nil, ErrRevisionScope
	}
	baselineByID, err := revisionEntities(baselineItems, identityField)
	if err != nil {
		return nil, err
	}
	incomingByID, err := revisionEntities(incomingItems, identityField)
	if err != nil {
		return nil, err
	}
	keptBaseline := make([]any, 0, len(baselineItems))
	removed := make([]string, 0)
	for _, item := range baselineItems {
		id := entityIdentity(item.(map[string]any), identityField)
		if _, kept := incomingByID[id]; kept {
			keptBaseline = append(keptBaseline, item)
			continue
		}
		removed = append(removed, id)
	}
	keptIncoming := make([]any, 0, len(incomingItems))
	added := make([]map[string]any, 0)
	for _, item := range incomingItems {
		entry := item.(map[string]any)
		id := entityIdentity(entry, identityField)
		if _, kept := baselineByID[id]; kept {
			keptIncoming = append(keptIncoming, item)
			continue
		}
		added = append(added, entry)
	}
	// 保留实体必须逐字段一致，并且顺序不能通过重排绕过锁与审阅。
	if !reflect.DeepEqual(keptBaseline, keptIncoming) {
		return nil, ErrRevisionScope
	}
	if len(removed) > 0 && !scope.AllowsRemove(collection) {
		return nil, ErrRevisionScope
	}
	if len(added) > 0 && !scope.AllowsAdd(collection) {
		return nil, ErrRevisionScope
	}
	diffs := make([]TextRevisionDiff, 0, len(removed)+len(added))
	for _, id := range removed {
		diffs = append(diffs, TextRevisionDiff{
			Path:   "/" + escapePointerField(collection) + "/@" + escapePointerField(id),
			Kind:   RevisionDiffRemoveEntity,
			Before: encodeRevisionEntity(baselineByID[id]),
		})
	}
	for _, entry := range added {
		id := entityIdentity(entry, identityField)
		diffs = append(diffs, TextRevisionDiff{
			Path:  "/" + escapePointerField(collection) + "/@" + escapePointerField(id),
			Kind:  RevisionDiffAddEntity,
			After: encodeRevisionEntity(entry),
		})
	}
	return diffs, nil
}

// revisionEntities 把集合实体按稳定标识索引，拒绝空标识与非对象项。
func revisionEntities(items []any, identityField string) (map[string]map[string]any, error) {
	indexed := make(map[string]map[string]any, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, ErrRevisionScope
		}
		id := entityIdentity(entry, identityField)
		if id == "" || indexed[id] != nil {
			return nil, ErrRevisionScope
		}
		indexed[id] = entry
	}
	return indexed, nil
}

func encodeRevisionEntity(entry map[string]any) string {
	if entry == nil {
		return ""
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		return ""
	}
	return string(encoded)
}
