package storyasset

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	story "github.com/lishen802/novel-studio/internal/domain/story"
)

// PlanningField 只公开可改的规划文字，身份、引用、约束和结构不作为编辑字段。
type PlanningField struct {
	Path  string `json:"path"`
	Label string `json:"label"`
	Value string `json:"value"`
}

// PlanningFields 按固定合同枚举文字，保留原载荷未知内容，不猜测缺失的目标或引用。
func PlanningFields(kind, payload string) ([]PlanningField, error) {
	object, decodeErr := decodePlanningObject(payload)
	if decodeErr != nil || object == nil {
		return nil, ErrChangeUnsupported
	}
	fields := []PlanningField{}
	add := func(path, label string, value any) error {
		text, ok := value.(string)
		if !ok {
			return ErrChangeUnsupported
		}
		fields = append(fields, PlanningField{path, label, text})
		return nil
	}
	top := [][2]string{{"title", "标题"}}
	switch kind {
	case "proposal":
		top = append(top, [][2]string{{"genre", "类型"}, {"positioning", "作品定位"}, {"sell_point", "卖点"}, {"core_conflict", "核心冲突"}, {"protagonist_arc", "主角弧"}, {"growth_cost", "成长代价"}, {"pacing", "节奏"}, {"ending", "结局方向"}}...)
	case "outline_node":
		top = append(top, [][2]string{{"summary", "概要"}, {"goal", "目标"}, {"conflict", "冲突与人物变化"}, {"outcome", "结果与伏笔"}}...)
	default:
		return nil, ErrChangeUnsupported
	}
	for _, field := range top {
		if value, ok := object[field[0]]; ok {
			if err := add("/"+field[0], field[1], value); err != nil {
				return nil, err
			}
		}
	}
	if len(fields) == 0 || fields[0].Path != "/title" {
		return nil, ErrChangeUnsupported
	}
	if kind == "proposal" {
		for _, collection := range [][2]string{{"major_milestones", "主要里程碑"}, {"subplots", "支线"}} {
			if values, ok := object[collection[0]]; ok {
				items, ok := values.([]any)
				if !ok {
					return nil, ErrChangeUnsupported
				}
				for i, value := range items {
					if err := add(fmt.Sprintf("/%s/%d", collection[0], i), fmt.Sprintf("%s %d", collection[1], i+1), value); err != nil {
						return nil, err
					}
				}
			}
		}
	} else if raw, ok := object["planning"]; ok {
		planning, ok := raw.(map[string]any)
		if !ok {
			return nil, ErrChangeUnsupported
		}
		for _, field := range [][2]string{{"reader_expectation", "读者期待"}, {"stage_payoff", "阶段回报"}, {"irreversible_change", "不可逆变化"}, {"ending_contribution", "终局贡献"}} {
			if value, ok := planning[field[0]]; ok {
				if err := add("/planning/"+field[0], field[1], value); err != nil {
					return nil, err
				}
			}
		}
		for _, collection := range []struct {
			name, label string
			fields      [][2]string
		}{{"commitments", "承诺", [][2]string{{"statement", "内容"}, {"fulfillment_range", "兑现范围"}}}, {"retained_questions", "悬念", [][2]string{{"statement", "内容"}, {"reason", "保留理由"}}}} {
			if raw, ok := planning[collection.name]; ok {
				items, ok := raw.([]any)
				if !ok {
					return nil, ErrChangeUnsupported
				}
				for i, item := range items {
					row, ok := item.(map[string]any)
					if !ok {
						return nil, ErrChangeUnsupported
					}
					for _, field := range collection.fields {
						if value, ok := row[field[0]]; ok {
							if err := add(fmt.Sprintf("/planning/%s/%d/%s", collection.name, i, field[0]), fmt.Sprintf("%s %d · %s", collection.label, i+1, field[1]), value); err != nil {
								return nil, err
							}
						}
					}
				}
			}
		}
	}
	if len(fields) > 400 {
		return nil, ErrChangeUnsupported
	}
	return fields, nil
}

// EditPlanning 要求字段集合完全一致，仅回写白名单文字，不能借 JSON 增删结构或改绑定。
func EditPlanning(kind, payload, text string) (string, string, error) {
	fields, err := PlanningFields(kind, payload)
	if err != nil {
		return "", "", err
	}
	edits := map[string]string{}
	raw := map[string]json.RawMessage{}
	decoder := json.NewDecoder(bytes.NewBufferString(text))
	if decoder.Decode(&raw) != nil || decoder.Decode(new(any)) != io.EOF || len(raw) != len(fields) {
		return "", "", ErrChangeUnsupported
	}
	for path, value := range raw {
		var text string
		if string(value) == "null" || json.Unmarshal(value, &text) != nil {
			return "", "", ErrChangeUnsupported
		}
		edits[path] = text
	}
	object, _ := decodePlanningObject(payload)
	for _, field := range fields {
		value, ok := edits[field.Path]
		if !ok || utf8.RuneCountInString(value) > 8000 {
			return "", "", ErrChangeUnsupported
		}
		value = strings.TrimSpace(value)
		if (field.Path == "/title" && (value == "" || utf8.RuneCountInString(value) > 200)) || (kind == "proposal" && value == "") || (strings.HasPrefix(field.Path, "/planning/") && value == "") {
			return "", "", ErrChangeUnsupported
		}
		edits[field.Path] = value
		if err = setPlanningText(object, strings.Split(strings.TrimPrefix(field.Path, "/"), "/"), value); err != nil {
			return "", "", err
		}
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return "", "", err
	}
	normalized, _ := json.Marshal(edits)
	return string(encoded), string(normalized), nil
}

func setPlanningText(value any, path []string, text string) error {
	if len(path) == 0 {
		return ErrChangeUnsupported
	}
	switch object := value.(type) {
	case map[string]any:
		if len(path) == 1 {
			if _, ok := object[path[0]].(string); !ok {
				return ErrChangeUnsupported
			}
			object[path[0]] = text
			return nil
		}
		return setPlanningText(object[path[0]], path[1:], text)
	case []any:
		i, err := strconv.Atoi(path[0])
		if err != nil || i < 0 || i >= len(object) {
			return ErrChangeUnsupported
		}
		if len(path) == 1 {
			if _, ok := object[i].(string); !ok {
				return ErrChangeUnsupported
			}
			object[i] = text
			return nil
		}
		return setPlanningText(object[i], path[1:], text)
	default:
		return ErrChangeUnsupported
	}
}

// ApplyOutlinePlanning 将适用版本投影到同一节点，层级和引用始终由原节点保留。
func ApplyOutlinePlanning(node story.OutlineNode, text string) (story.OutlineNode, error) {
	object := map[string]any{"title": node.Title, "summary": node.Summary, "goal": node.Goal, "conflict": node.Conflict, "outcome": node.Outcome}
	if node.PlanningJSON != "" && node.PlanningJSON != "{}" {
		planning, err := decodePlanningObject(node.PlanningJSON)
		if err != nil {
			return node, ErrChangeUnsupported
		}
		object["planning"] = planning
	}
	encoded, _ := json.Marshal(object)
	payload, _, err := EditPlanning("outline_node", string(encoded), text)
	if err != nil {
		return node, err
	}
	var updated struct {
		Title, Summary, Goal, Conflict, Outcome string
		Planning                                json.RawMessage
	}
	if json.Unmarshal([]byte(payload), &updated) != nil {
		return node, ErrChangeUnsupported
	}
	node.Title, node.Summary, node.Goal, node.Conflict, node.Outcome = updated.Title, updated.Summary, updated.Goal, updated.Conflict, updated.Outcome
	if len(updated.Planning) > 0 {
		node.PlanningJSON = string(updated.Planning)
	}
	return node, nil
}

func decodePlanningObject(payload string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil {
		return nil, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, ErrChangeUnsupported
	}
	return object, nil
}
