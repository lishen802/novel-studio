package creation

import (
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrStructureUnsupported   = err("creation candidate kind does not support this collection")
	ErrStructureEntityMissing = err("creation structure entity is missing")
	ErrStructureEntityExists  = err("creation structure entity already exists")
	ErrStructureIdentity      = err("creation structure entity identity mismatch")
)

// StructureOperation 是候选载荷的结构增删操作。
type StructureOperation string

const (
	StructureAdd    StructureOperation = "add"
	StructureRemove StructureOperation = "remove"
)

// StructureEdit 描述候选载荷中一个实体的新增或删除。
// 集合使用顶层字段名（可带前导斜杠），实体以稳定标识寻址，不使用数组下标。
type StructureEdit struct {
	Collection string
	Operation  StructureOperation
	EntityKey  string
	Value      string
}

// structureIdentityField 返回集合内实体的稳定标识字段；
// 人物/关系/路线图节点使用 Schema 中的 id，势力/地点/规则没有 id，使用唯一名称。
func structureIdentityField(collection string) (string, bool) {
	switch collection {
	case "characters", "relationships", "volumes", "arcs", "near_term_chapters":
		return "id", true
	case "factions", "locations", "world_rules":
		return "name", true
	default:
		return "", false
	}
}

// structureCollections 返回该候选种类允许作者增删的集合，其他结构不在本命令范围内。
func structureCollections(kind CandidateKind) []string {
	switch kind {
	case KindPeopleWorld:
		return []string{"characters", "relationships", "factions", "locations", "world_rules"}
	case KindRoadmap:
		return []string{"volumes", "arcs", "near_term_chapters"}
	default:
		return nil
	}
}

// SupportsStructure 表示候选种类是否支持结构增删。
func (candidate Candidate) SupportsStructure() bool {
	return len(structureCollections(candidate.Kind)) > 0
}

// StructureIdentity 返回集合请求的稳定标识字段，供上层拼装编辑表单。
func StructureIdentity(collection string) (string, bool) {
	return structureIdentityField(strings.TrimPrefix(strings.TrimSpace(collection), "/"))
}

// structureCollection 校验集合属于该候选种类。
func structureCollection(candidate Candidate, collection string) (string, string, error) {
	collection = strings.TrimPrefix(strings.TrimSpace(collection), "/")
	field, ok := structureIdentityField(collection)
	if !ok {
		return "", "", ErrStructureUnsupported
	}
	for _, allowed := range structureCollections(candidate.Kind) {
		if allowed == collection {
			return collection, field, nil
		}
	}
	return "", "", ErrStructureUnsupported
}

// EditStructure 在候选载荷中新增或删除一个实体并生成下一版本候选，供领域内直接使用。
func (candidate Candidate) EditStructure(edit StructureEdit, newID string, now time.Time) (Candidate, error) {
	if strings.TrimSpace(newID) == "" {
		return Candidate{}, ErrBlankValue
	}
	payload, err := candidate.ApplyStructure(edit)
	if err != nil {
		return Candidate{}, err
	}
	return candidate.SaveDraft(newID, payload, now)
}

// ApplyStructure 返回应用结构增删后的载荷；调用方负责版本化保存与跨阶段引用校验。
// 锁定实体的字段受保护：删除或改写包含锁的实体先返回锁错误，不走静默丢弃。
func (candidate Candidate) ApplyStructure(edit StructureEdit) (string, error) {
	if candidate.ID == "" {
		return "", ErrBlankValue
	}
	collection, identityField, err := structureCollection(candidate, edit.Collection)
	if err != nil {
		return "", err
	}
	key, err := structureEntityKey(edit.EntityKey)
	if err != nil {
		return "", err
	}
	if edit.Operation != StructureAdd && edit.Operation != StructureRemove {
		return "", ErrBlankValue
	}
	object, err := revisionObject(candidate.PayloadJSON)
	if err != nil {
		return "", err
	}
	items, err := structureItems(object, collection)
	if err != nil {
		return "", err
	}
	if err := candidate.rejectLockedEntity(collection, key); err != nil {
		return "", err
	}
	kept := make([]any, 0, len(items)+1)
	present := false
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return "", ErrInvalidJSON
		}
		if entityIdentity(entry, identityField) == key {
			present = true
			if edit.Operation == StructureRemove {
				continue
			}
		}
		kept = append(kept, entry)
	}
	switch edit.Operation {
	case StructureRemove:
		if !present {
			return "", ErrStructureEntityMissing
		}
	default:
		if present {
			return "", ErrStructureEntityExists
		}
		entry, err := decodeStructureEntity(edit.Value)
		if err != nil {
			return "", err
		}
		if entityIdentity(entry, identityField) != key {
			return "", ErrStructureIdentity
		}
		kept = append(kept, entry)
	}
	object[collection] = kept
	payload, err := json.Marshal(object)
	if err != nil {
		return "", ErrInvalidJSON
	}
	return string(payload), nil
}

// rejectLockedEntity 阻止通过结构增删绕过字段锁：锁路径位于该实体内（含实体自身）时不允许增删改写。
// 删除后换 ID 新增同内容的做法会因锁路径无法解析而在 SaveDraft 的锁校验处被拒绝。
func (candidate Candidate) rejectLockedEntity(collection, key string) error {
	prefix := "/" + escapePointerField(collection) + "/@" + escapePointerField(key)
	for path, locked := range candidate.LockMap {
		if !locked {
			continue
		}
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return ErrFieldLocked
		}
	}
	return nil
}

// escapePointerField 按 JSON Pointer 规则转义集合与实体标识。
func escapePointerField(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

// structureEntityKey 校验实体标识：非空、有限长度、不含分隔符歧义。
func structureEntityKey(key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" || utf8.RuneCountInString(key) > 200 || strings.ContainsAny(key, "/~") {
		return "", ErrBlankValue
	}
	return key, nil
}

// structureItems 读取顶层集合，缺失时按空集合处理，类型不符时拒绝。
func structureItems(object map[string]any, collection string) ([]any, error) {
	raw, ok := object[collection]
	if !ok || raw == nil {
		return []any{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, ErrInvalidJSON
	}
	return items, nil
}

// entityIdentity 读取实体在集合内的稳定标识。
func entityIdentity(entry map[string]any, identityField string) string {
	value, ok := entry[identityField].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

// decodeStructureEntity 解析新增实体的 JSON 对象，保留精确数字字面量。
func decodeStructureEntity(value string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(value)))
	decoder.UseNumber()
	var entry map[string]any
	if err := decoder.Decode(&entry); err != nil || entry == nil {
		return nil, ErrInvalidJSON
	}
	return entry, nil
}
