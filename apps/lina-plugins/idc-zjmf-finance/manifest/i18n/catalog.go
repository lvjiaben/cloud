// Package catalogtext renders plugin-owned fixed catalog labels from the same
// locale JSON bundled for the host. It is a pure embedded-resource projection,
// not an independent runtime translation service or mutable service graph.
package catalogtext

import (
	"embed"
	"lina-core/pkg/i18nresource"
	"sync"
)

//go:embed en-US/plugin.json zh-CN/plugin.json
var resources embed.FS
var messages = sync.OnceValue(func() map[string]map[string]string {
	result := map[string]map[string]string{}
	for _, locale := range []string{"en-US", "zh-CN"} {
		raw, err := resources.ReadFile(locale + "/plugin.json")
		if err != nil {
			continue
		}
		catalog, err := i18nresource.ParseCatalog(raw, i18nresource.ValueModeStringOnly)
		if err == nil {
			result[locale] = catalog
		}
	}
	return result
})

// Render returns one explicit language of a bilingual public declaration.
// Missing resources retain the English source fallback; no locale is guessed.
func Render(locale, key, fallback string) string {
	if value := messages()[locale][key]; value != "" {
		return value
	}
	return fallback
}
