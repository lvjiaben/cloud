// This file implements bounded staff list queries with matching count scopes.
package channel

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

const (
	defaultPageSize   = 10
	maximumPageSize   = 100
	maximumPageNumber = 1000000
)

// PageQuery identifies a bounded numeric page, optionally filtering channel candidates.
type PageQuery struct {
	Page     int
	PageSize int
	Keyword  string
}

// PageResult contains the exact visible total and one page, never a truncated snapshot.
type PageResult[T any] struct {
	List     []T
	Total    int
	Page     int
	PageSize int
}

func normalizePage(query PageQuery) (PageQuery, error) {
	if query.Page == 0 {
		query.Page = 1
	}
	if query.PageSize == 0 {
		query.PageSize = defaultPageSize
	}
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.Page < 1 || query.Page > maximumPageNumber || query.PageSize < 1 || query.PageSize > maximumPageSize || len([]rune(query.Keyword)) > 160 {
		return PageQuery{}, bizerr.NewCode(codePageInvalid)
	}
	return query, nil
}

func queryPage[T any](model *gdb.Model, query PageQuery, scan func(*gdb.Model) ([]T, error)) (PageResult[T], error) {
	query, err := normalizePage(query)
	if err != nil {
		return PageResult[T]{}, err
	}
	total, err := model.Clone().Count()
	if err != nil {
		return PageResult[T]{}, err
	}
	rows, err := scan(model.Clone().Page(query.Page, query.PageSize))
	if err != nil {
		return PageResult[T]{}, err
	}
	return PageResult[T]{List: rows, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

// ListTokenPage exposes metadata only and preserves the existing staff channel filter.
func (s *service) ListTokenPage(ctx context.Context, channelID string, query PageQuery) (PageResult[TokenSummary], error) {
	if err := s.authorizeStaff(ctx); err != nil {
		return PageResult[TokenSummary]{}, err
	}
	model := s.db.Model("plugin_idc_channel_token").Ctx(ctx).
		Order("id DESC")
	if channelID = strings.TrimSpace(channelID); channelID != "" {
		model = model.Where("channel_id=?", channelID)
	}
	return queryPage(model, query, scanTokenSummaries)
}

// CatalogPage preserves the approved-channel and published-only catalog view.
func (s *service) CatalogPage(ctx context.Context, channelID string, query PageQuery) (PageResult[CatalogItem], error) {
	if _, err := s.requireChannel(ctx, channelID, true); err != nil {
		return PageResult[CatalogItem]{}, err
	}
	model := s.db.Model("plugin_idc_channel_publication").Ctx(ctx).
		Where("channel_id=? AND status='published'", strings.TrimSpace(channelID)).Order("id ASC")
	return queryPage(model, query, func(page *gdb.Model) ([]CatalogItem, error) {
		var rows []CatalogItem
		err := page.Fields("product_version_id,currency,price,stock").Scan(&rows)
		return rows, err
	})
}
