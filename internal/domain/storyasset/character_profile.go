package storyasset

import (
	"encoding/json"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	world "github.com/lishen802/novel-studio/internal/domain/world"
)

// CharacterProfile 只包含可按章节修订的人物设定，身份、状态及关系由原档案保留。
type CharacterProfile struct {
	Name       string              `json:"name"`
	Aliases    []string            `json:"aliases"`
	Role       world.CharacterRole `json:"role"`
	Summary    string              `json:"summary"`
	Motivation string              `json:"motivation"`
	Fear       string              `json:"fear"`
	Secret     string              `json:"secret"`
	Arc        string              `json:"arc"`
}

// ParseCharacterProfile 拒绝未知字段和多份 JSON，复用人物领域规则规范化输入。
func ParseCharacterProfile(text string) (CharacterProfile, error) {
	var p CharacterProfile
	decoder := json.NewDecoder(strings.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return p, ErrChangeUnsupported
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return p, ErrChangeUnsupported
	}
	if utf8.RuneCountInString(p.Name) > 200 || len(p.Aliases) > 50 {
		return p, ErrChangeUnsupported
	}
	for _, alias := range p.Aliases {
		if utf8.RuneCountInString(alias) > 200 {
			return p, ErrChangeUnsupported
		}
	}
	for _, field := range []string{p.Summary, p.Motivation, p.Fear, p.Secret, p.Arc} {
		if utf8.RuneCountInString(field) > 8000 {
			return p, ErrChangeUnsupported
		}
	}
	c, err := world.NewCharacter("validation", "validation", p.Name, p.Aliases, p.Role, p.Motivation, p.Fear, p.Secret, p.Arc, time.Time{})
	if err != nil {
		return p, err
	}
	c, err = p.Apply(c)
	if err != nil {
		return p, err
	}
	return ProfileOf(c), nil
}

// Apply 在同一人物身份上替换设定字段，不改变故事状态。
func (p CharacterProfile) Apply(c world.Character) (world.Character, error) {
	return c.UpdateProfile(p.Name, p.Aliases, p.Role, p.Summary, p.Motivation, p.Fear, p.Secret, p.Arc, c.Status, c.UpdatedAt)
}

func ProfileOf(c world.Character) CharacterProfile {
	return CharacterProfile{c.Name, c.Aliases, c.Role, c.Summary, c.Motivation, c.Fear, c.Secret, c.Arc}
}
