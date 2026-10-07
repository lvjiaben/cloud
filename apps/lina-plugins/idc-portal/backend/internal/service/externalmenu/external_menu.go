// Package externalmenu owns the bounded installation-level customer console links.
package externalmenu

import (
	"context"
	"net/url"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

const (
	maxItems = 30
	table    = "idc_portal_external_menu"
)

var allowedIcons = map[string]struct{}{
	"lucide:book-open":      {},
	"lucide:circle-help":    {},
	"lucide:external-link":  {},
	"lucide:file-text":      {},
	"lucide:globe-2":        {},
	"lucide:life-buoy":      {},
	"lucide:link":           {},
	"lucide:message-circle": {},
	"lucide:shopping-bag":   {},
}

// Item is the safe projection returned to the admin and customer applications.
type Item struct {
	ID        int64  `json:"id"`
	NameZh    string `json:"nameZh"`
	NameEn    string `json:"nameEn"`
	Icon      string `json:"icon"`
	URL       string `json:"url"`
	SortOrder int    `json:"sortOrder"`
	Enabled   bool   `json:"enabled"`
	Version   int    `json:"version"`
}

// SaveInput contains create/update fields. ID zero creates a new item.
type SaveInput struct {
	ID        int64
	NameZh    string
	NameEn    string
	Icon      string
	URL       string
	SortOrder int
	Version   int
}

// Service stores and reads customer external links without per-row queries.
type Service struct{}

func New() *Service { return &Service{} }

// List returns at most 30 rows in stable display order.
func (s *Service) List(ctx context.Context, enabledOnly bool) ([]Item, error) {
	model := g.DB().Model(table).Ctx(ctx).Fields("id,name_zh,name_en,icon,url,sort_order,status,version")
	if enabledOnly {
		model = model.Where("status", 1)
	}
	var rows []struct {
		ID        int64  `json:"id"`
		NameZh    string `json:"name_zh"`
		NameEn    string `json:"name_en"`
		Icon      string `json:"icon"`
		URL       string `json:"url"`
		SortOrder int    `json:"sort_order"`
		Status    int    `json:"status"`
		Version   int    `json:"version"`
	}
	if err := model.OrderAsc("sort_order").OrderAsc("id").Limit(maxItems).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "list customer external menus")
	}
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		items = append(items, Item{
			ID: row.ID, NameZh: row.NameZh, NameEn: row.NameEn, Icon: row.Icon,
			URL: row.URL, SortOrder: row.SortOrder, Enabled: row.Status == 1, Version: row.Version,
		})
	}
	return items, nil
}

// Save creates or updates one link and uses an optimistic version for edits.
func (s *Service) Save(ctx context.Context, input SaveInput) (*Item, error) {
	input, err := normalize(input)
	if err != nil {
		return nil, err
	}
	if input.ID == 0 {
		count, countErr := g.DB().Model(table).Ctx(ctx).Count()
		if countErr != nil {
			return nil, gerror.Wrap(countErr, "count customer external menus")
		}
		if count >= maxItems {
			return nil, gerror.New("customer external menu limit reached")
		}
		result, insertErr := g.DB().Model(table).Ctx(ctx).Data(g.Map{
			"name_zh": input.NameZh, "name_en": input.NameEn, "icon": input.Icon,
			"url": input.URL, "sort_order": input.SortOrder, "status": 1, "version": 1,
		}).Insert()
		if insertErr != nil {
			return nil, gerror.Wrap(insertErr, "create customer external menu")
		}
		id, idErr := result.LastInsertId()
		if idErr != nil {
			return nil, gerror.Wrap(idErr, "read customer external menu id")
		}
		return s.get(ctx, id)
	}
	if input.Version < 1 {
		return nil, gerror.New("customer external menu version is required")
	}
	result, updateErr := g.DB().Model(table).Ctx(ctx).
		Where("id=? AND version=?", input.ID, input.Version).
		Data(g.Map{
			"name_zh": input.NameZh, "name_en": input.NameEn, "icon": input.Icon,
			"url": input.URL, "sort_order": input.SortOrder,
			"version": input.Version + 1, "updated_at": gdb.Raw("CURRENT_TIMESTAMP"),
		}).Update()
	if updateErr != nil {
		return nil, gerror.Wrap(updateErr, "update customer external menu")
	}
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil || affected != 1 {
		return nil, gerror.New("customer external menu changed; refresh and retry")
	}
	return s.get(ctx, input.ID)
}

// SetEnabled changes only the visible state under optimistic concurrency.
func (s *Service) SetEnabled(ctx context.Context, id int64, version int, enabled bool) (*Item, error) {
	if id < 1 || version < 1 {
		return nil, gerror.New("customer external menu id and version are required")
	}
	status := 0
	if enabled {
		status = 1
	}
	result, err := g.DB().Model(table).Ctx(ctx).
		Where("id=? AND version=?", id, version).
		Data(g.Map{"status": status, "version": version + 1, "updated_at": gdb.Raw("CURRENT_TIMESTAMP")}).Update()
	if err != nil {
		return nil, gerror.Wrap(err, "change customer external menu status")
	}
	affected, affectedErr := result.RowsAffected()
	if affectedErr != nil || affected != 1 {
		return nil, gerror.New("customer external menu changed; refresh and retry")
	}
	return s.get(ctx, id)
}

func (s *Service) get(ctx context.Context, id int64) (*Item, error) {
	items, err := s.List(ctx, false)
	if err != nil {
		return nil, err
	}
	for index := range items {
		if items[index].ID == id {
			return &items[index], nil
		}
	}
	return nil, gerror.New("customer external menu not found")
}

func normalize(input SaveInput) (SaveInput, error) {
	input.NameZh = strings.TrimSpace(input.NameZh)
	input.NameEn = strings.TrimSpace(input.NameEn)
	input.Icon = strings.TrimSpace(input.Icon)
	input.URL = strings.TrimSpace(input.URL)
	if input.NameZh == "" || input.NameEn == "" || len([]rune(input.NameZh)) > 80 || len([]rune(input.NameEn)) > 80 {
		return SaveInput{}, gerror.New("Chinese and English menu names are required and must not exceed 80 characters")
	}
	if _, ok := allowedIcons[input.Icon]; !ok {
		return SaveInput{}, gerror.New("unsupported customer external menu icon")
	}
	parsed, err := url.ParseRequestURI(input.URL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return SaveInput{}, gerror.New("customer external menu URL must be a safe HTTP or HTTPS address")
	}
	if len(input.URL) > 2048 || input.SortOrder < 0 || input.SortOrder > 9999 {
		return SaveInput{}, gerror.New("customer external menu URL or sort order is invalid")
	}
	return input, nil
}
