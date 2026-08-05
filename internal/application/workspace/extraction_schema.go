package workspace

// extractionSchema is the JSON Schema sent to the model so it returns
// structured story-asset candidates for a single chapter. The model references
// characters by name (not ID) in relationships, and chapters by title in
// foreshadowing hooks, because entities have not been persisted yet; the merge
// layer resolves those references to the TargetIDs it assigns.
const extractionSchema = `{
	"type": "object",
	"additionalProperties": false,
	"required": ["outline","characters","relationships","worldRules","storyEvents","facts","foreshadowingHooks"],
	"properties": {
		"outline": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"required": ["level","title","summary","position"],
				"properties": {
					"level":     {"type": "string", "enum": ["book","volume","arc","chapter","scene"]},
					"title":     {"type": "string"},
					"summary":   {"type": "string"},
					"goal":      {"type": "string"},
					"conflict":  {"type": "string"},
					"outcome":   {"type": "string"},
					"position":  {"type": "integer", "minimum": 1}
				}
			}
		},
		"characters": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"required": ["name","role","summary"],
				"properties": {
					"name":       {"type": "string"},
					"aliases":    {"type": "array", "items": {"type": "string"}},
					"role":      {"type": "string", "enum": ["protagonist","supporting","antagonist","minor"]},
					"summary":   {"type": "string"},
					"motivation": {"type": "string"},
					"fear":      {"type": "string"},
					"secret":    {"type": "string"},
					"arc":       {"type": "string"},
					"status":    {"type": "string", "enum": ["active","inactive","deceased"]}
				}
			}
		},
		"relationships": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"required": ["sourceName","targetName","kind"],
				"properties": {
					"sourceName":  {"type": "string"},
					"targetName":  {"type": "string"},
					"kind":        {"type": "string", "enum": ["ally","family","romantic","rival","enemy","mentor","other"]},
					"label":       {"type": "string"},
					"strength":    {"type": "integer", "minimum": -100, "maximum": 100},
					"publicState": {"type": "string"},
					"trueState":   {"type": "string"}
				}
			}
		},
		"worldRules": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"required": ["name","description","scopeType"],
				"properties": {
					"name":        {"type": "string"},
					"description": {"type": "string"},
					"scopeType":   {"type": "string", "enum": ["project","region","location"]},
					"scopeId":     {"type": "string"}
				}
			}
		},
		"storyEvents": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"required": ["title","storyOrder","narrativeOrder","status"],
				"properties": {
					"title":          {"type": "string"},
					"summary":        {"type": "string"},
					"cause":          {"type": "string"},
					"result":         {"type": "string"},
					"storyTime":      {"type": "string"},
					"storyOrder":     {"type": "integer", "minimum": 1},
					"narrativeOrder": {"type": "integer", "minimum": 1},
					"status":         {"type": "string", "enum": ["occurred","planned","cancelled"]}
				}
			}
		},
		"facts": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"required": ["text","category"],
				"properties": {
					"text":              {"type": "string"},
					"category":          {"type": "string"},
					"status":            {"type": "string", "enum": ["ai_inferred","text_evidenced","author_confirmed","rejected"]},
					"confidence":        {"type": "number", "minimum": 0, "maximum": 1},
					"validFromEventId":  {"type": "string"},
					"validUntilEventId": {"type": "string"}
				}
			}
		},
		"foreshadowingHooks": {
			"type": "array",
			"items": {
				"type": "object",
				"additionalProperties": false,
				"required": ["title","setup","plantedChapterTitle"],
				"properties": {
					"title":             {"type": "string"},
					"setup":             {"type": "string"},
					"plantedChapterTitle": {"type": "string"},
					"dueChapterTitle":   {"type": "string"}
				}
			}
		}
	}
}`
