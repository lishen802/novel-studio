package creation

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type TextFieldView struct {
	Path   string `json:"path"`
	Label  string `json:"label"`
	Group  string `json:"group"`
	Value  string `json:"value"`
	Locked bool   `json:"locked"`
}
type FieldEditorView struct {
	CandidateID string          `json:"candidateId"`
	Version     int             `json:"version"`
	Fields      []TextFieldView `json:"fields"`
}
type SaveFieldsRequest struct {
	CandidateID string            `json:"candidateId"`
	Changes     map[string]string `json:"changes"`
}

var fieldLabels = map[string]string{
	"causal_chain": "因果链", "character_change": "人物变化", "reader_expectation": "读者期待", "stage_payoff": "阶段回报", "irreversible_change": "不可逆变化", "ending_contribution": "终局贡献", "fulfillment_range": "兑现范围", "commitments": "待兑现承诺", "commitments_omission": "承诺省略原因", "retained_questions": "保留悬念", "retained_questions_omission": "悬念省略原因", "label": "关系说明", "outcome": "阶段结果", "proposition": "故事命题", "protagonist_arc": "主角成长", "scope_hint": "适用范围", "sell_point": "作品卖点",
	"directions": "故事方向", "characters": "人物", "relationships": "人物关系", "factions": "势力", "locations": "地点", "items": "物品", "world_rules": "世界规则", "book": "全书", "volumes": "卷", "arcs": "故事弧", "chapters": "章节", "anchors": "锚点", "title": "标题", "name": "名称", "summary": "概要", "description": "描述", "secret": "秘密", "desire": "欲望", "fear": "恐惧", "motivation": "动机", "arc": "人物弧", "role": "角色定位", "hook": "卖点", "genre": "题材", "protagonist": "主角", "core_conflict": "核心冲突", "growth_cost": "成长代价", "pacing": "节奏", "ending": "结局方向", "positioning": "作品定位", "main_conflict": "主线冲突", "character_arc": "人物成长", "setting": "故事背景", "theme": "主题", "goal": "目标", "conflict": "冲突", "turning_point": "转折", "payoff": "回报", "content": "内容", "reason": "理由", "text": "说明", "statement": "叙事承诺", "ending_direction": "结局方向", "narrative_role": "叙事作用", "tone": "语气", "point_of_view": "视角", "value": "内容", "risk": "风险", "omit_reason": "省略原因",
}

func fieldLabel(key string) string {
	if label, ok := fieldLabels[key]; ok {
		return label
	}
	return "补充说明"
}
func escapeField(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

// candidateTextFields 按稳定实体路径提供文本编辑投影，身份和引用字段不作为普通文本开放。
func candidateTextFields(candidate creationdomain.Candidate) FieldEditorView {
	view := FieldEditorView{CandidateID: candidate.ID, Version: candidate.Version, Fields: []TextFieldView{}}
	var root any
	if json.Unmarshal([]byte(candidate.PayloadJSON), &root) != nil {
		return view
	}
	var walk func(any, string, string, int)
	walk = func(value any, path, group string, depth int) {
		if depth > 30 || len(view.Fields) >= 1000 {
			return
		}
		switch object := value.(type) {
		case map[string]any:
			keys := make([]string, 0, len(object))
			for key := range object {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				if key == "id" || key == "kind" || key == "role" || key == "scope_type" || key == "strength" || strings.HasSuffix(key, "_ref") || strings.HasSuffix(key, "_refs") || key == "parent_id" || strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_ids") {
					continue
				}
				child := object[key]
				next := path + "/" + escapeField(key)
				if text, ok := child.(string); ok {
					if _, err := candidate.FieldString(next); err == nil {
						view.Fields = append(view.Fields, TextFieldView{Path: next, Label: fieldLabel(key), Group: group, Value: text, Locked: candidate.Locked(next)})
					}
					continue
				}
				walk(child, next, group+" · "+fieldLabel(key), depth+1)
			}
		case []any:
			for index, child := range object {
				entity, ok := child.(map[string]any)
				if !ok {
					continue
				}
				// 集合实体优先用 Schema 的 id 寻址；势力/地点/规则没有 id，按唯一名称寻址。
				id, _ := entity["id"].(string)
				if strings.TrimSpace(id) == "" {
					id, _ = entity["name"].(string)
				}
				if strings.TrimSpace(id) == "" {
					continue
				}
				name, _ := entity["name"].(string)
				if name == "" {
					name, _ = entity["title"].(string)
				}
				if name == "" {
					name = strconv.Itoa(index + 1)
				}
				walk(entity, path+"/"+escapeField("@"+id), group+" · "+name, depth+1)
			}
		}
	}
	walk(root, "", "当前候选", 0)
	return view
}

// SaveFields 以准确候选为基线应用字段补丁，随后复用完整草稿验证与原子版本保存。
func (service *Service) SaveFields(ctx context.Context, projectID string, r SaveFieldsRequest) (creationdomain.Candidate, error) {
	if len(r.Changes) == 0 || len(r.Changes) > 1000 {
		return creationdomain.Candidate{}, creationdomain.ErrBlankValue
	}
	candidate, err := service.CandidatePayload(ctx, projectID, r.CandidateID)
	if err != nil {
		return candidate, err
	}
	allowed := map[string]bool{}
	for _, field := range candidateTextFields(candidate).Fields {
		allowed[field.Path] = true
	}
	for path, value := range r.Changes {
		if !allowed[path] || utf8.RuneCountInString(value) > 100000 {
			return candidate, creationdomain.ErrFieldMissing
		}
	}
	payload, err := candidate.EditTextFields(r.Changes)
	if err != nil {
		return candidate, err
	}
	return service.SaveCandidateDraft(ctx, projectID, candidate.ID, payload)
}
