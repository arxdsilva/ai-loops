package guard

import "encoding/json"

const modelMsg = "model_guard: pass model for this agent: haiku for mechanical work, sonnet for implementation against a contract, opus for hard reasoning (ROLES.md, Model tiers)."

func checkModel(toolInput json.RawMessage) string {
	var in map[string]any
	if err := json.Unmarshal(toolInput, &in); err != nil {
		return ""
	}
	if untyped(in["subagent_type"]) && !truthy(in["model"]) {
		return modelMsg
	}
	return ""
}

func untyped(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && (s == "" || s == "general-purpose")
}

// truthy mirrors Python truthiness for decoded JSON values.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return true
}
