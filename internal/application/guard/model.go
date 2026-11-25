package guard

import (
	"encoding/json"
	"regexp"
	"strings"
)

const modelMsg = "model_guard: pass model for this agent: haiku for mechanical work, sonnet for implementation against a contract, opus for hard reasoning (ROLES.md, Model tiers)."

// ModelReasonLine is the prompt line that justifies passing a model above the
// role's tier: `Model reason: <text>` (WORKFLOW §4).
const ModelReasonLine = "Model reason: "

var (
	modelReason = regexp.MustCompile(`(?m)^[ \t]*Model reason:[ \t]*\S`)
	// tierRank orders the model tiers: haiku < sonnet < opus.
	tierRank = map[string]int{"haiku": 1, "sonnet": 2, "opus": 3}
)

// TierOf returns a role's default model tier ("haiku", "sonnet" or "opus"),
// or "" when the role is unknown.
type TierOf func(role string) string

func checkModel(toolInput json.RawMessage, tierOf TierOf) string {
	var in map[string]any
	if err := json.Unmarshal(toolInput, &in); err != nil {
		return ""
	}
	if untyped(in["subagent_type"]) && !truthy(in["model"]) {
		return modelMsg
	}
	return checkTier(in, tierOf)
}

// modelTier maps a model value (a tier name or a full id such as
// claude-opus-5-5) to its tier; "" for unknown values and "inherit".
func modelTier(model string) string {
	m := strings.ToLower(model)
	for _, t := range []string{"haiku", "sonnet", "opus"} {
		if strings.Contains(m, t) {
			return t
		}
	}
	return ""
}

// ModelTier is modelTier for callers outside the package.
func ModelTier(model string) string { return modelTier(model) }

// checkTier denies a model above the role's default tier unless the prompt
// carries a `Model reason:` line. Unknown roles, unknown models and a nil
// lookup are not checked; a lower tier is always allowed.
func checkTier(in map[string]any, tierOf TierOf) string {
	if tierOf == nil {
		return ""
	}
	st, _ := in["subagent_type"].(string)
	role := strings.TrimPrefix(st, "loops:")
	model, _ := in["model"].(string)
	if role == "" || model == "" {
		return ""
	}
	def, asked := tierOf(role), modelTier(model)
	if tierRank[def] == 0 || tierRank[asked] == 0 || tierRank[asked] <= tierRank[def] {
		return ""
	}
	prompt, _ := in["prompt"].(string)
	if modelReason.MatchString(prompt) {
		return ""
	}
	return "model_guard: " + role + " runs at " + def + "; you passed " + asked +
		". Pass a higher model only to escalate (WORKFLOW §4) or for a plan lane whose model column gives a reason: " +
		"drop the model, or add a `" + ModelReasonLine + "<reason>` line to the prompt."
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
