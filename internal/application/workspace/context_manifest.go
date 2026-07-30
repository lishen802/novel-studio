package workspace

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"unicode/utf8"
)

var ErrContextBudgetExceeded = errors.New("required authoring context exceeds configured budget")

type contextManifestInput struct {
	Header  map[string]any
	Domains []contextDomainInput
}

type contextDomainInput struct {
	Name     string
	Priority int
	Required bool
	Items    []json.RawMessage
}

type contextManifestStats struct {
	CharCount int
	Truncated bool
}

type contextDomainStats struct {
	Total      int    `json:"total"`
	Included   int    `json:"included"`
	Omitted    int    `json:"omitted"`
	SourceHash string `json:"sourceHash"`
}

type contextManifestMeta struct {
	Version    string                        `json:"version"`
	CharCount  int                           `json:"charCount"`
	CharBudget int                           `json:"charBudget"`
	Truncated  bool                          `json:"truncated"`
	Domains    map[string]contextDomainStats `json:"domains"`
}

type contextManifestDocument struct {
	Meta    contextManifestMeta          `json:"meta"`
	Header  map[string]any               `json:"header"`
	Domains map[string][]json.RawMessage `json:"domains"`
}

func buildContextManifest(
	input contextManifestInput,
	charBudget int,
) (string, contextManifestStats, error) {
	if charBudget < 1 {
		return "", contextManifestStats{}, ErrContextBudgetExceeded
	}
	domains := make([]contextDomainInput, len(input.Domains))
	for index, domain := range input.Domains {
		domains[index] = domain
		domains[index].Items = append([]json.RawMessage(nil), domain.Items...)
	}
	sort.SliceStable(domains, func(first, second int) bool {
		if domains[first].Priority == domains[second].Priority {
			return domains[first].Name < domains[second].Name
		}
		return domains[first].Priority < domains[second].Priority
	})

	original := make(map[string][]json.RawMessage, len(domains))
	for _, domain := range domains {
		original[domain.Name] = append([]json.RawMessage(nil), domain.Items...)
	}
	encode := func() ([]byte, contextManifestStats, error) {
		document := contextManifestDocument{
			Meta: contextManifestMeta{
				Version:    "context-v2",
				CharBudget: charBudget,
				Domains:    make(map[string]contextDomainStats, len(domains)),
			},
			Header:  input.Header,
			Domains: make(map[string][]json.RawMessage, len(domains)),
		}
		for _, domain := range domains {
			source, err := json.Marshal(original[domain.Name])
			if err != nil {
				return nil, contextManifestStats{}, err
			}
			sum := sha256.Sum256(source)
			total := len(original[domain.Name])
			included := len(domain.Items)
			document.Domains[domain.Name] = domain.Items
			document.Meta.Domains[domain.Name] = contextDomainStats{
				Total:      total,
				Included:   included,
				Omitted:    total - included,
				SourceHash: hex.EncodeToString(sum[:]),
			}
			if included < total {
				document.Meta.Truncated = true
			}
		}
		var encoded []byte
		for range 4 {
			var err error
			encoded, err = json.Marshal(document)
			if err != nil {
				return nil, contextManifestStats{}, err
			}
			count := utf8.RuneCount(encoded)
			if document.Meta.CharCount == count {
				break
			}
			document.Meta.CharCount = count
		}
		return encoded, contextManifestStats{
			CharCount: document.Meta.CharCount,
			Truncated: document.Meta.Truncated,
		}, nil
	}

	for {
		encoded, stats, err := encode()
		if err != nil {
			return "", contextManifestStats{}, err
		}
		if stats.CharCount <= charBudget {
			return string(encoded), stats, nil
		}
		removed := false
		for index := len(domains) - 1; index >= 0; index-- {
			if domains[index].Required || len(domains[index].Items) == 0 {
				continue
			}
			domains[index].Items = domains[index].Items[:len(domains[index].Items)-1]
			removed = true
			break
		}
		if !removed {
			return "", contextManifestStats{}, ErrContextBudgetExceeded
		}
	}
}

func rawContextItems[T any](values []T) ([]json.RawMessage, error) {
	result := make([]json.RawMessage, 0, len(values))
	for _, value := range values {
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		result = append(result, encoded)
	}
	sort.Slice(result, func(first, second int) bool {
		return bytes.Compare(result[first], result[second]) < 0
	})
	return result, nil
}

func newContextDomain(
	name string,
	priority int,
	required bool,
	values any,
) (contextDomainInput, error) {
	encoded, err := json.Marshal(values)
	if err != nil {
		return contextDomainInput{}, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(encoded, &items); err != nil {
		return contextDomainInput{}, err
	}
	sort.Slice(items, func(first, second int) bool {
		return bytes.Compare(items[first], items[second]) < 0
	})
	return contextDomainInput{
		Name: name, Priority: priority, Required: required, Items: items,
	}, nil
}
