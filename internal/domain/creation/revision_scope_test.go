package creation

import (
	"errors"
	"testing"
)

func TestTextRevisionRejectsUnauthorizedChanges(t *testing.T) {
	base := Candidate{PayloadJSON: `{"title":"旧标题","characters":[{"id":"a/b~c","secret":"秘密","name":"甲"}],"count":9007199254740993}`, LockMap: map[string]bool{"/characters/@a~1b~0c/secret": true}}
	for _, tc := range []struct {
		name, payload string
		paths         []string
		want          error
	}{
		{"范围内", `{"title":"新标题","characters":[{"id":"a/b~c","secret":"秘密","name":"甲"}],"count":9007199254740993}`, []string{"/title"}, nil},
		{"顺带改名", `{"title":"新标题","characters":[{"id":"a/b~c","secret":"秘密","name":"乙"}],"count":9007199254740993}`, []string{"/title"}, ErrRevisionScope},
		{"大整数变化", `{"title":"新标题","characters":[{"id":"a/b~c","secret":"秘密","name":"甲"}],"count":9007199254740992}`, []string{"/title"}, ErrRevisionScope},
		{"锁值改变", `{"title":"新标题","characters":[{"id":"a/b~c","secret":"泄漏","name":"甲"}],"count":9007199254740993}`, []string{"/title"}, ErrFieldLocked},
		{"锁作为目标", base.PayloadJSON, []string{"/characters/@a~1b~0c/secret"}, ErrFieldLocked},
		{"结构作为目标", base.PayloadJSON, []string{"/characters"}, ErrFieldMissing},
		{"无目标", base.PayloadJSON, nil, ErrRevisionScope},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := base.ReviewTextRevision(tc.payload, tc.paths)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
		})
	}
}

func TestTextRevisionReturnsStableNestedDiff(t *testing.T) {
	base := Candidate{PayloadJSON: `{"characters":[{"id":"a/b~c","secret":"旧"}]}`}
	diffs, err := base.ReviewTextRevision(`{"characters":[{"id":"a/b~c","secret":"新"}]}`, []string{"/characters/@a~1b~0c/secret"})
	if err != nil || len(diffs) != 1 || diffs[0].Before != "旧" || diffs[0].After != "新" {
		t.Fatalf("diff=%v err=%v", diffs, err)
	}
	if base.PayloadJSON != `{"characters":[{"id":"a/b~c","secret":"旧"}]}` {
		t.Fatal("基线被修改")
	}
}

func TestTextRevisionRejectsStructuralAndAmbiguousTargets(t *testing.T) {
	for _, tc := range []struct {
		name, base, next string
		paths            []string
	}{
		{"重排", `{"rows":[{"id":"a","text":"旧"},{"id":"b","text":"乙"}]}`, `{"rows":[{"id":"b","text":"乙"},{"id":"a","text":"新"}]}`, []string{"/rows/@a/text"}},
		{"重复身份", `{"rows":[{"id":"a","text":"旧"},{"id":"a","text":"乙"}]}`, `{"rows":[{"id":"a","text":"新"},{"id":"a","text":"乙"}]}`, []string{"/rows/@a/text"}},
		{"数字下标", `{"rows":[{"id":"a","text":"旧"}]}`, `{"rows":[{"id":"a","text":"新"}]}`, []string{"/rows/0/text"}},
		{"重复目标", `{"title":"旧"}`, `{"title":"新"}`, []string{"/title", "/title"}},
		{"尾随数据", `{"title":"旧"}`, `{"title":"新"} {}`, []string{"/title"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := (Candidate{PayloadJSON: tc.base}).ReviewTextRevision(tc.next, tc.paths); err == nil {
				t.Fatal("无效修订被接受")
			}
		})
	}
}
