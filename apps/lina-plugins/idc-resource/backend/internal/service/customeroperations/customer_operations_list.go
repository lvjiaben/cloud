// This file reads an indexed numbered page and exact customer-scoped total
// without querying the supplier for each row.
package customeroperations

import (
	"context"
	"errors"
	"lina-plugin-idc-resource/backend/internal/service/serverfacts"
)

// Facts reads bounded historical facts after obtaining the authenticated customer.
// The reader enforces the same customer in SQL and never invokes a provider.
func (s *Service) Facts(ctx context.Context, numbers []string) ([]serverfacts.Fact, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrUnavailable
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || subject.CustomerNo == "" {
		return nil, ErrUnavailable
	}
	items, err := s.facts.Read(ctx, subject.CustomerNo, numbers)
	if errors.Is(err, serverfacts.ErrInvalid) {
		return nil, ErrInvalid
	}
	if err != nil {
		return nil, ErrUnavailable
	}
	return items, nil
}

type ListItem struct{ ServerNo, ProductNameZhCN, ProductNameEnUS, State string }
type Page struct {
	Items []ListItem
	Total int
}

// List returns a bounded numbered page and total under the identical customer
// and native-entitlement predicates; staff visibility never replaces ownership.
func (s *Service) List(ctx context.Context, page, limit int) (*Page, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || page < 1 || page > 1000000 || limit < 1 || limit > 50 {
		return nil, ErrInvalid
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || subject.CustomerNo == "" {
		return nil, ErrUnavailable
	}
	result := &Page{Items: []ListItem{}}
	count, err := s.database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_resource_server_entitlement_projection ownership WHERE ownership.tenant_id=0 AND ownership.customer_no=?`, subject.CustomerNo)
	if err != nil {
		return nil, ErrUnavailable
	}
	result.Total = count.Int()
	err = s.database.GetScan(ctx, &result.Items, `SELECT ownership.server_no,ownership.product_name_zh_cn,ownership.product_name_en_us,COALESCE(mapping.status_code,'unknown') AS state
 FROM plugin_idc_resource_server_entitlement_projection ownership
 LEFT JOIN plugin_idc_resource_mapping mapping ON mapping.tenant_id=ownership.tenant_id AND mapping.mapping_no=ownership.mapping_no AND mapping.local_resource_no=ownership.local_resource_no
 WHERE ownership.tenant_id=0 AND ownership.customer_no=?
 ORDER BY ownership.server_no LIMIT ? OFFSET ?`, subject.CustomerNo, limit, (page-1)*limit)
	if err != nil {
		return nil, ErrUnavailable
	}
	return result, nil
}
