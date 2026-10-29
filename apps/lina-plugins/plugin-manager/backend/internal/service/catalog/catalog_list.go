// This file classifies a bounded authorized host snapshot before server-side pagination.

package catalog

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	pluginv1 "lina-core/api/plugin/v1"
	"lina-core/pkg/bizerr"
	v1 "lina-plugin-plugin-manager/backend/api/management/v1"
	"strings"
)

const (
	// Collection uses the public host's 100-item page limit, up to ten pages.
	hostPageSize   = 100
	maxCatalogSize = 1000
	// Result pages are independent of collection pages and retain the published API limit.
	maxResultPageSize = 200
)

func validateConfig(config Config) error {
	if len(config.Categories) == 0 || len(config.Categories) > 50 {
		return gerror.New("configure between 1 and 50 plugin categories")
	}
	var (
		keys     = map[string]bool{}
		owners   = map[string]string{}
		prefixes = map[string]string{}
	)
	for _, category := range config.Categories {
		if strings.TrimSpace(category.ID) == "" || strings.TrimSpace(category.Name) == "" || keys[category.ID] {
			return gerror.New("plugin category identifiers and names must be nonempty and identifiers unique")
		}
		keys[category.ID] = true
		for _, id := range category.Plugins {
			if id == "" || owners[id] != "" {
				return gerror.New("each exact plugin identifier must belong to one category")
			}
			owners[id] = category.ID
		}
		for _, prefix := range category.Prefixes {
			if prefix == "" || prefixes[prefix] != "" {
				return gerror.New("category prefixes must be nonempty and unique")
			}
			prefixes[prefix] = category.ID
		}
	}
	if !keys[config.Fallback] {
		return gerror.New("plugin fallback category must exist")
	}
	return nil
}

// Categories returns a fresh presentation projection of the immutable startup config.
func (s *serviceImpl) Categories(_ context.Context) []v1.Category {
	result := make([]v1.Category, 0, len(s.config.Categories))
	for _, item := range s.config.Categories {
		result = append(result, v1.Category{ID: item.ID, Name: item.Name})
	}
	return result
}

func (s *serviceImpl) category(id string) string {
	for _, c := range s.config.Categories {
		for _, exact := range c.Plugins {
			if id == exact {
				return c.ID
			}
		}
	}
	var (
		selected = s.config.Fallback
		longest  = 0
	)
	for _, c := range s.config.Categories {
		for _, prefix := range c.Prefixes {
			if len(prefix) > longest && strings.HasPrefix(id, prefix) {
				selected, longest = c.ID, len(prefix)
			}
		}
	}
	return selected
}

// List classifies at most 1000 summaries already authorized by the public host list API.
func (s *serviceImpl) List(ctx context.Context, input *v1.ListReq) (*v1.ListRes, error) {
	if input == nil || input.PageNum < 1 || input.PageSize < 1 || input.PageSize > maxResultPageSize {
		return nil, bizerr.NewCode(CodeCategoryInvalid)
	}
	found := false
	for _, c := range s.config.Categories {
		if c.ID == input.Category {
			found = true
			break
		}
	}
	if !found {
		return nil, bizerr.NewCode(CodeCategoryInvalid)
	}
	var (
		items     = make([]*pluginv1.PluginListItem, 0)
		seen      = map[string]bool{}
		expected  = -1
		collected = 0
	)
	for page := 1; page <= maxCatalogSize/hostPageSize; page++ {
		if err := ctx.Err(); err != nil {
			return nil, bizerr.WrapCode(err, CodeCatalogIncomplete)
		}
		out, err := s.reader.List(ctx, page, hostPageSize, input.ID, input.Name)
		if err != nil {
			return nil, err
		}
		if out == nil || out.Total < 0 || out.Total > maxCatalogSize || (expected >= 0 && out.Total != expected) {
			return nil, bizerr.NewCode(CodeCatalogIncomplete)
		}
		expected = out.Total
		if len(out.List) == 0 && collected < expected {
			return nil, bizerr.NewCode(CodeCatalogIncomplete)
		}
		for _, item := range out.List {
			if item == nil || seen[item.Id] {
				return nil, bizerr.NewCode(CodeCatalogIncomplete)
			}
			seen[item.Id] = true
			collected++
			if item.Type == pluginv1.PluginTypeDynamic && item.Distribution == pluginv1.PluginDistributionManaged && s.category(item.Id) == input.Category {
				items = append(items, item)
			}
		}
		if collected >= expected {
			break
		}
	}
	if collected != expected {
		return nil, bizerr.NewCode(CodeCatalogIncomplete)
	}
	total := len(items)
	// Check page bounds before multiplication to avoid integer overflow on untrusted input.
	if total == 0 || input.PageNum > (total+input.PageSize-1)/input.PageSize {
		return &v1.ListRes{List: []*pluginv1.PluginListItem{}, Total: total}, nil
	}
	start := (input.PageNum - 1) * input.PageSize
	end := min(start+input.PageSize, total)
	return &v1.ListRes{List: items[start:end], Total: total}, nil
}
