package creation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
)

var ErrRevisionScope = errors.New("修订超出授权文本范围")

// TextRevisionDiff 记录实际文本差异，不把模型自述作为范围校验依据。
type TextRevisionDiff struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// ReviewTextRevision 校验锁与文本范围，所有未授权字段、结构及顺序必须保持不变。
// 返回差异仅表示确定性范围检查通过，不替代完整 Schema 与独立质量审阅。
func (candidate Candidate) ReviewTextRevision(payload string, paths []string) ([]TextRevisionDiff, error) {
	if len(paths) == 0 || len(paths) > 1000 {
		return nil, ErrRevisionScope
	}
	baseline, err := revisionObject(candidate.PayloadJSON)
	if err != nil {
		return nil, err
	}
	incoming, err := revisionObject(payload)
	if err != nil {
		return nil, err
	}
	lockPaths := make([]string, 0, len(candidate.LockMap))
	for path, locked := range candidate.LockMap {
		if locked {
			lockPaths = append(lockPaths, path)
		}
	}
	sort.Strings(lockPaths)
	for _, path := range lockPaths {
		before, e1 := pointerValue(baseline, path)
		after, e2 := pointerValue(incoming, path)
		if e1 != nil || e2 != nil || !reflect.DeepEqual(before, after) {
			return nil, fmt.Errorf("%w: %s", ErrFieldLocked, path)
		}
	}
	targets := append([]string(nil), paths...)
	sort.Strings(targets)
	diffs := make([]TextRevisionDiff, 0, len(targets))
	for i, path := range targets {
		if path != strings.TrimSpace(path) || (i > 0 && path == targets[i-1]) {
			return nil, ErrRevisionScope
		}
		if candidate.Locked(path) {
			return nil, ErrFieldLocked
		}
		before, e1 := pointerValue(baseline, path)
		after, e2 := pointerValue(incoming, path)
		beforeText, ok1 := before.(string)
		afterText, ok2 := after.(string)
		if e1 != nil || e2 != nil || !ok1 || !ok2 {
			return nil, fmt.Errorf("%w: %s", ErrFieldMissing, path)
		}
		if beforeText != afterText {
			diffs = append(diffs, TextRevisionDiff{Path: path, Before: beforeText, After: afterText})
		}
		if err := setPointerValue(baseline, path, afterText); err != nil {
			return nil, err
		}
	}
	if !reflect.DeepEqual(baseline, incoming) {
		return nil, ErrRevisionScope
	}
	return diffs, nil
}

// revisionObject 保留精确数字，避免浮点转换掩盖授权范围外的数量变化。
func revisionObject(payload string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	var object map[string]any
	if err := decoder.Decode(&object); err != nil || object == nil {
		return nil, ErrInvalidJSON
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, ErrInvalidJSON
	}
	return object, nil
}
