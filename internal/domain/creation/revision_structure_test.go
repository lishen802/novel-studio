package creation

import (
	"errors"
	"strings"
	"testing"
)

// 授权集合内的新增被接受，并返回结构差异供作者审阅。
func TestScopedRevisionAcceptsAuthorizedAdd(t *testing.T) {
	base := Candidate{PayloadJSON: `{"characters":[{"id":"char-1","name":"阿绫"}],"factions":[]}`, LockMap: map[string]bool{}}
	incoming := `{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-2","name":"老周"}],"factions":[]}`
	diffs, err := base.ReviewScopedRevision(incoming, RevisionScope{Paths: []string{"/characters/@char-1/name"}, AllowAdd: []string{"characters"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || diffs[0].Kind != RevisionDiffAddEntity || diffs[0].Path != "/characters/@char-2" || diffs[0].After == "" {
		t.Fatalf("diffs = %#v", diffs)
	}
}

// 未授权的增删与集合外变化一律拒绝。
func TestScopedRevisionRejectsUnauthorizedChanges(t *testing.T) {
	base := Candidate{PayloadJSON: `{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-2","name":"老周"}],"factions":[{"name":"守军"}]}`, LockMap: map[string]bool{}}
	cases := []struct {
		name    string
		payload string
		scope   RevisionScope
	}{
		{"未授权新增", `{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-2","name":"老周"},{"id":"char-3","name":"新兵"}],"factions":[{"name":"守军"}]}`, RevisionScope{Paths: []string{"/characters/@char-2/name"}}},
		{"未授权删除", `{"characters":[{"id":"char-1","name":"阿绫"}],"factions":[{"name":"守军"}]}`, RevisionScope{Paths: []string{"/characters/@char-2/name"}, AllowAdd: []string{"characters"}}},
		{"未授权集合变化", `{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-2","name":"老周"}],"factions":[]}`, RevisionScope{Paths: []string{"/characters/@char-2/name"}, AllowRemove: []string{"characters"}}},
		{"重排", `{"characters":[{"id":"char-2","name":"老周"},{"id":"char-1","name":"阿绫"}],"factions":[{"name":"守军"}]}`, RevisionScope{Paths: []string{"/characters/@char-2/name"}, AllowAdd: []string{"characters"}}},
		{"换标识", `{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-9","name":"老周"}],"factions":[{"name":"守军"}]}`, RevisionScope{Paths: []string{"/characters/@char-2/name"}, AllowAdd: []string{"characters"}}},
		{"保留项被改写", `{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-2","name":"老周","secret":"新增"}],"factions":[{"name":"守军"}]}`, RevisionScope{Paths: []string{"/characters/@char-2/name"}, AllowAdd: []string{"characters"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := base.ReviewScopedRevision(tc.payload, tc.scope); !errors.Is(err, ErrRevisionScope) {
				t.Fatalf("错误 = %v", err)
			}
		})
	}
}

// 只授权结构增删、不改任何文本也成立；完全无授权仍拒绝。
func TestScopedRevisionRequiresSomeAuthorization(t *testing.T) {
	base := Candidate{PayloadJSON: `{"characters":[{"id":"char-1","name":"阿绫"}]}`, LockMap: map[string]bool{}}
	incoming := `{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-2","name":"老周"}]}`
	diffs, err := base.ReviewScopedRevision(incoming, RevisionScope{AllowAdd: []string{"/characters"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || diffs[0].Kind != RevisionDiffAddEntity {
		t.Fatalf("diffs = %#v", diffs)
	}
	if _, err := base.ReviewScopedRevision(incoming, RevisionScope{}); !errors.Is(err, ErrRevisionScope) {
		t.Fatalf("无授权错误 = %v", err)
	}
	if _, err := base.ReviewScopedRevision(incoming, RevisionScope{AllowAdd: []string{"characters"}, Paths: []string{"/characters/@char-1/name"}, AllowRemove: []string{"unknown_collection"}}); err != nil {
		t.Fatalf("未知集合应被忽略而不是报错: %v", err)
	}
}

// 无 id 的集合按唯一名称增删，重名与空名都被拒绝。
func TestScopedRevisionHandlesNameKeyedCollections(t *testing.T) {
	base := Candidate{PayloadJSON: `{"factions":[{"name":"守军","goal":"守关"}]}`, LockMap: map[string]bool{}}
	added, err := base.ReviewScopedRevision(`{"factions":[{"name":"守军","goal":"守关"},{"name":"盐帮","goal":"私路"}]}`, RevisionScope{AllowAdd: []string{"factions"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0].Path != "/factions/@盐帮" {
		t.Fatalf("diffs = %#v", added)
	}
	removed, err := base.ReviewScopedRevision(`{"factions":[]}`, RevisionScope{AllowRemove: []string{"factions"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0].Kind != RevisionDiffRemoveEntity || !strings.Contains(removed[0].Before, "守军") {
		t.Fatalf("diffs = %#v", removed)
	}
	if _, err := base.ReviewScopedRevision(`{"factions":[{"name":"守军","goal":"守关"},{"name":"","goal":"x"}]}`, RevisionScope{AllowAdd: []string{"factions"}}); !errors.Is(err, ErrRevisionScope) {
		t.Fatalf("空标识错误 = %v", err)
	}
}

// 锁定实体的字段在结构增删中同样受保护。
func TestScopedRevisionKeepsLocks(t *testing.T) {
	base := Candidate{
		PayloadJSON: `{"characters":[{"id":"char-1","name":"阿绫"},{"id":"char-2","name":"老周"}]}`,
		LockMap:     map[string]bool{"/characters/@char-2/name": true},
	}
	if _, err := base.ReviewScopedRevision(`{"characters":[{"id":"char-1","name":"阿绫"}]}`, RevisionScope{AllowRemove: []string{"characters"}}); !errors.Is(err, ErrFieldLocked) {
		t.Fatalf("锁定实体删除错误 = %v", err)
	}
}
