package creation

import (
	"encoding/json"
	"strings"
)

// pointerParts 解码稳定字段路径；数组使用 @实体ID，不接受位置索引。
func pointerParts(path string) ([]string, error) {
	path = normalizePointer(path)
	if !strings.HasPrefix(path, "/") {
		return nil, ErrFieldMissing
	}
	parts := strings.Split(path[1:], "/")
	if len(parts) > 32 {
		return nil, ErrFieldMissing
	}
	for i, p := range parts {
		if p == "" {
			return nil, ErrFieldMissing
		}
		for n := 0; n < len(p); n++ {
			if p[n] == '~' {
				if n+1 >= len(p) || (p[n+1] != '0' && p[n+1] != '1') {
					return nil, ErrFieldMissing
				}
				n++
			}
		}
		parts[i] = strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~")
	}
	return parts, nil
}
func pointerChild(value any, key string) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		child, ok := v[key]
		if ok {
			return child, nil
		}
	case []any:
		if !strings.HasPrefix(key, "@") || len(key) < 2 {
			return nil, ErrFieldMissing
		}
		var found any
		count := 0
		for _, child := range v {
			if object, ok := child.(map[string]any); ok && object["id"] == key[1:] {
				found = object
				count++
			}
		}
		if count == 1 {
			return found, nil
		}
	}
	return nil, ErrFieldMissing
}
func pointerValue(root any, path string) (any, error) {
	parts, err := pointerParts(path)
	if err != nil {
		return nil, err
	}
	value := root
	for _, key := range parts {
		value, err = pointerChild(value, key)
		if err != nil {
			return nil, err
		}
	}
	return value, nil
}
func setPointerValue(root any, path string, value any) error {
	parts, err := pointerParts(path)
	if err != nil {
		return err
	}
	parent := root
	for _, key := range parts[:len(parts)-1] {
		parent, err = pointerChild(parent, key)
		if err != nil {
			return err
		}
	}
	object, ok := parent.(map[string]any)
	if !ok {
		return ErrFieldMissing
	}
	key := parts[len(parts)-1]
	if _, ok := object[key]; !ok {
		return ErrFieldMissing
	}
	object[key] = value
	return nil
}

// EditTextFields 仅修改已有字符串字段；完整载荷与锁校验由保存用例执行。
func (candidate Candidate) EditTextFields(changes map[string]string) (string, error) {
	object, err := decodeObject(candidate.PayloadJSON)
	if err != nil {
		return "", err
	}
	for path, value := range changes {
		if _, err := candidate.FieldString(path); err != nil {
			return "", err
		}
		if err := setPointerValue(object, path, value); err != nil {
			return "", err
		}
	}
	encoded, err := json.Marshal(object)
	if err != nil {
		return "", ErrInvalidJSON
	}
	return string(encoded), nil
}
