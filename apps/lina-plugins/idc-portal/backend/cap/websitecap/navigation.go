// This file defines bounded template page discovery and merchant navigation values.
package websitecap

import (
	"encoding/json"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
)

// Page is a public route explicitly declared by an installed website template.
type Page struct {
	ID     string `json:"id" dc:"Stable template page identifier" eg:"home"`
	NameZh string `json:"nameZh" dc:"Chinese page name" eg:"Home"`
	NameEn string `json:"nameEn" dc:"English page name" eg:"Home"`
	Path   string `json:"path" dc:"Public website route" eg:"/"`
}

// NavigationItem references a template page, customer entry, or public URL.
type NavigationItem struct {
	Group     bool   `json:"group,omitempty" dc:"Display a submenu without a link destination"`
	ID        string `json:"id" dc:"Stable navigation identifier" eg:"home-link"`
	ParentID  string `json:"parentId" dc:"Parent identifier; empty for a root item" eg:""`
	NameZh    string `json:"nameZh" dc:"Chinese navigation label" eg:"Home"`
	NameEn    string `json:"nameEn" dc:"English navigation label" eg:"Home"`
	PageID    string `json:"pageId" dc:"Template page ID or customer-login/customer-register; empty for URL links" eg:"home"`
	URL       string `json:"url" dc:"HTTP or HTTPS URL, used only when pageId is empty" eg:"https://example.com"`
	SortOrder int    `json:"sortOrder" dc:"Sibling display order, 0 to 9999" eg:"0"`
}

// Navigation contains independently editable header and footer trees.
type Navigation struct {
	Header []NavigationItem `json:"header" dc:"Header navigation; at most 100 entries" eg:"[]"`
	Footer []NavigationItem `json:"footer" dc:"Footer navigation; at most 100 entries" eg:"[]"`
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
		return nil, gerror.New("website page declaration is invalid")
	}
	seen := map[string]bool{}
	for _, page := range pages {
		if page.ID == "" || seen[page.ID] || page.ID == "customer-login" || page.ID == "customer-register" || utf8.RuneCountInString(page.NameZh) > 80 || page.NameZh == "" || page.NameEn == "" || !safePagePath(page.Path) {
			return nil, gerror.New("website page declaration is invalid")
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
	for _, prefix := range []string{"/admin", "/api", "/x", "/auth", "/account"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return false
		}
	}
	return !strings.Contains(path, "..")
}
