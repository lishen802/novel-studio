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
	Path   string           `json:"path"`
	Kind   RevisionDiffKind `json:"kind"`
	Before string           `json:"before"`
	After  string           `json:"after"`
}

// ReviewTextRevision 校验锁与文本范围，所有未授权字段、结构及顺序必须保持不变。
// 返回差异仅表示确定性范围检查通过，不替代完整 Schema 与独立质量审阅。
func (candidate Candidate) ReviewTextRevision(payload string, paths []string) ([]TextRevisionDiff, error) {
	return candidate.ReviewScopedRevision(payload, RevisionScope{Paths: paths})
}

// ReviewScopedRevision 在文本范围之外额外校验被显式授权的结构增删：
// 未授权集合的任何变化、实体重排与换标识都会被拒绝。
func (candidate Candidate) ReviewScopedRevision(payload string, scope RevisionScope) ([]TextRevisionDiff, error) {
	scope = scope.Normalize()
	paths := scope.Paths
	if len(paths) == 0 && len(scope.AllowAdd) == 0 && len(scope.AllowRemove) == 0 {
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
	removedTargets := make([]string, 0)
	for i, path := range targets {
		if path != strings.TrimSpace(path) || (i > 0 && path == targets[i-1]) {
			return nil, ErrRevisionScope
		}
		if candidate.Locked(path) {
			return nil, ErrFieldLocked
		}
		before, e1 := pointerValue(baseline, path)
		beforeText, ok1 := before.(string)
		if e1 != nil || !ok1 {
			return nil, fmt.Errorf("%w: %s", ErrFieldMissing, path)
		}
		after, e2 := pointerValue(incoming, path)
		if e2 != nil {
			// 目标可能落在本次被授权删除的实体里，先记录并在结构结算后确认。
			removedTargets = append(removedTargets, path)
			continue
		}
		afterText, ok2 := after.(string)
		if !ok2 {
			return nil, fmt.Errorf("%w: %s", ErrFieldMissing, path)
		}
		if beforeText != afterText {
			diffs = append(diffs, TextRevisionDiff{Path: path, Kind: RevisionDiffText, Before: beforeText, After: afterText})
		}
		if err := setPointerValue(baseline, path, afterText); err != nil {
			return nil, err
		}
	}
	// 文本路径已应用到基线副本；其余键逐项比较，结构增删只在授权集合内按实体集合结算。
	structural, err := reconcileRevisionStructure(baseline, incoming, scope)
	if err != nil {
		return nil, err
	}
	for _, path := range removedTargets {
		if !underRemovedEntity(path, structural) {
			return nil, fmt.Errorf("%w: %s", ErrRevisionScope, path)
		}
	}
	return append(diffs, structural...), nil
}

// underRemovedEntity 判断文本目标是否属于本次被显式授权删除的实体。
func underRemovedEntity(path string, diffs []TextRevisionDiff) bool {
	for _, diff := range diffs {
		if diff.Kind == RevisionDiffRemoveEntity && strings.HasPrefix(path, diff.Path+"/") {
			return true
		}
	}
	return false
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

// AdoptTextRevisionDiffs 把作者选中的差异子集转换为字段修改集合，供确定性合成新候选使用。
// 只能选择范围门禁已经确认的差异路径；未选择的路径保持基线原文。
func AdoptTextRevisionDiffs(diffs []TextRevisionDiff, selected []string) (map[string]string, error) {
	if len(selected) == 0 || len(selected) > 1000 {
		return nil, ErrRevisionScope
	}
	available := make(map[string]string, len(diffs))
	for _, diff := range diffs {
		available[diff.Path] = diff.After
	}
	changes := make(map[string]string, len(selected))
	for i, path := range selected {
		path = strings.TrimSpace(path)
		if path == "" || path != selected[i] || (i > 0 && path == selected[i-1]) {
			return nil, ErrRevisionScope
		}
		after, ok := available[path]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrRevisionScope, path)
		}
		changes[path] = after
	}
	return changes, nil
}
