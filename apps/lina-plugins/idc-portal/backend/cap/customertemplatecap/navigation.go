// This file defines bounded template page discovery and merchant navigation values.
package customertemplatecap

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
)

// Page is a public route explicitly declared by an installed customer template.
type Page struct {
	ID     string `json:"id" dc:"Stable template page identifier" eg:"home"`
	NameZh string `json:"nameZh" dc:"Chinese page name" eg:"Home"`
	NameEn string `json:"nameEn" dc:"English page name" eg:"Home"`
	Path   string `json:"path" dc:"Public website route" eg:"/"`
}

// Pages reads frozen template metadata and rejects unsafe or duplicated paths.
func Pages(metadata *capmodel.BusinessExtensionProviderMetadata) ([]Page, error) {
	if metadata == nil || metadata.Values["pages"] == nil {
		return []Page{}, nil
	}
	data, err := json.Marshal(metadata.Values["pages"])
	if err != nil {
		return nil, err
	}
	var pages []Page
	if err = json.Unmarshal(data, &pages); err != nil || len(pages) > 100 {
		return nil, gerror.New("customer page declaration is invalid")
	}
	seen := map[string]bool{}
	for _, page := range pages {
		if page.ID == "" || seen[page.ID] || utf8.RuneCountInString(page.NameZh) > 80 || page.NameZh == "" || page.NameEn == "" || !safePagePath(page.Path) {
			return nil, gerror.New("customer page declaration is invalid")
		}
		seen[page.ID] = true
	}
	return pages, nil
}

// safePagePath restricts template declarations to public website routes.
func safePagePath(path string) bool {
	parsed, err := url.Parse(path)
	if err != nil || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\\\r\n") || parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	for _, prefix := range []string{"/admin", "/api", "/x", "/x-assets"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return false
		}
	}
	return !strings.Contains(path, "..")
}
