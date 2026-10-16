// This file validates bounded website navigation and resolves declared destinations.
package siteconfiguration

import (
	"net/url"
	"sort"
	"strings"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-portal/backend/cap/websitecap"
)

// validateNavigation rejects forged pages, unsafe URLs, duplicate IDs and parent cycles.
func validateNavigation(nav *websitecap.Navigation, metadata *capmodel.BusinessExtensionProviderMetadata) error {
	if nav == nil {
		return nil
	}
	pages, err := websitecap.Pages(metadata)
	if err != nil {
		return err
	}
	allowed := map[string]bool{"customer-login": true, "customer-register": true}
	for _, p := range pages {
		allowed[p.ID] = true
	}
	for _, items := range [][]websitecap.NavigationItem{nav.Header, nav.Footer} {
		if len(items) > 100 {
			return bizerr.NewCode(codeNavigationInvalid)
		}
		byID := map[string]websitecap.NavigationItem{}
		for _, item := range items {
			if strings.TrimSpace(item.ID) == "" || len(item.ID) > 80 || byID[item.ID].ID != "" || strings.TrimSpace(item.NameZh) == "" || strings.TrimSpace(item.NameEn) == "" || utf8.RuneCountInString(item.NameZh) > 80 || utf8.RuneCountInString(item.NameEn) > 80 || item.SortOrder < 0 || item.SortOrder > 9999 {
				return bizerr.NewCode(codeNavigationInvalid)
			}
			if item.Group {
				if item.PageID != "" || item.URL != "" {
					return bizerr.NewCode(codeNavigationInvalid)
				}
			} else if item.PageID != "" {
				if !allowed[item.PageID] || item.URL != "" {
					return bizerr.NewCode(codeNavigationInvalid)
				}
			} else if !safeExternalURL(item.URL) {
				return bizerr.NewCode(codeNavigationInvalid)
			}
			byID[item.ID] = item
		}
		for _, item := range items {
			seen := map[string]bool{item.ID: true}
			parent := item.ParentID
			for depth := 0; parent != ""; depth++ {
				next, ok := byID[parent]
				if !ok || seen[parent] || depth >= 2 {
					return bizerr.NewCode(codeNavigationInvalid)
				}
				seen[parent] = true
				parent = next.ParentID
			}
		}
	}
	return nil
}

// safeExternalURL accepts explicit public HTTP links without credentials or control characters.
func safeExternalURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && !strings.ContainsAny(value, "\r\n\\")
}

// ResolvedNavigation is the safe, display-only tree exposed to website renderers.
type ResolvedNavigation struct {
	Group    bool                 `json:"group,omitempty"`
	NameZh   string               `json:"nameZh"`
	NameEn   string               `json:"nameEn"`
	Href     string               `json:"href"`
	Customer bool                 `json:"customer"`
	External bool                 `json:"external"`
	Children []ResolvedNavigation `json:"children,omitempty"`
}

// ResolveNavigation filters unavailable template pages and builds a bounded sorted tree.
func ResolveNavigation(items []websitecap.NavigationItem, pages []websitecap.Page) []ResolvedNavigation {
	paths := map[string]string{"customer-login": "/auth/login", "customer-register": "/auth/register"}
	for _, p := range pages {
		paths[p.ID] = p.Path
	}
	ordered := append([]websitecap.NavigationItem{}, items...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].SortOrder < ordered[j].SortOrder })
	var children func(string, int) []ResolvedNavigation
	children = func(parent string, depth int) []ResolvedNavigation {
		result := []ResolvedNavigation{}
		if depth > 2 {
			return result
		}
		for _, item := range ordered {
			if item.ParentID != parent {
				continue
			}
			href := item.URL
			if item.PageID != "" {
				href = paths[item.PageID]
			}
			nested := children(item.ID, depth+1)
			if (item.Group && len(nested) == 0) || (!item.Group && href == "") {
				continue
			}
			result = append(result, ResolvedNavigation{Group: item.Group, NameZh: item.NameZh, NameEn: item.NameEn, Href: href, Customer: item.PageID == "customer-login" || item.PageID == "customer-register", External: !item.Group && item.PageID == "", Children: nested})
		}
		return result
	}
	return children("", 0)
}
