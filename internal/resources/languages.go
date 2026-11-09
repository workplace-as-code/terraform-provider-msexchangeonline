package resources

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Helpers for the languages attribute of tenant-wide settings: a set of
// objects keyed by their language attribute.

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
