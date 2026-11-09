package resources

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Helpers for the languages attribute of tenant-wide settings: a set of
// objects keyed by their language attribute.

// knownLanguages returns the attributes of the elements of the set whose value
// is known: an element built from a computed value is not, at validation.
func knownLanguages(languages types.Set) []map[string]attr.Value {
	var out []map[string]attr.Value
	for _, e := range languages.Elements() {
		if object, ok := e.(types.Object); ok && !object.IsUnknown() && !object.IsNull() {
			out = append(out, object.Attributes())
		}
	}
	return out
}

// emptyLanguageText returns an attribute set to "" in the set, "" if none. An
// empty text is stored as unset: the plan and the state would not match.
func emptyLanguageText(languages types.Set) string {
	for _, attributes := range knownLanguages(languages) {
		for name, v := range attributes {
			if text, ok := v.(types.String); ok && !text.IsNull() && !text.IsUnknown() && text.ValueString() == "" {
				return name
			}
		}
	}
	return ""
}

// duplicateLanguage returns a language found twice in the set, "" if none.
func duplicateLanguage(languages types.Set) string {
	seen := map[string]bool{}
	for _, e := range languages.Elements() {
		object, ok := e.(types.Object)
		if !ok {
			continue
		}
		language, ok := object.Attributes()["language"].(types.String)
		if !ok || language.IsNull() || language.IsUnknown() {
			continue
		}
		key := strings.ToLower(language.ValueString())
		if seen[key] {
			return language.ValueString()
		}
		seen[key] = true
	}
	return ""
}

// optionalLanguageText is the state of a text of a language: Exchange pads its
// lists with empty strings, and an empty text is an attribute left unset.
func optionalLanguageText(value string) attr.Value {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}
