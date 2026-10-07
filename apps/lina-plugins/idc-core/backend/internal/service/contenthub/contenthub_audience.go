package contenthub

import (
	"context"
	"fmt"
	"strings"

	"lina-core/pkg/bizerr"
)

func (s *serviceImpl) SearchAudienceCustomers(ctx context.Context, input AudienceSearchInput) (*CandidatePage, error) {
	if _, err := s.staff(ctx); err != nil {
		return nil, err
	}
	return s.searchAudience(ctx, input, "customer")
}
func (s *serviceImpl) SearchAudienceServices(ctx context.Context, input AudienceSearchInput) (*CandidatePage, error) {
	if _, err := s.staff(ctx); err != nil {
		return nil, err
	}
	return s.searchAudience(ctx, input, "service")
}
func (s *serviceImpl) SearchAudienceCustomerGroups(ctx context.Context, input AudienceSearchInput) (*CandidatePage, error) {
	if _, err := s.staff(ctx); err != nil {
		return nil, err
	}
	return s.searchAudience(ctx, input, "group")
}

func (s *serviceImpl) searchAudience(ctx context.Context, input AudienceSearchInput, kind string) (*CandidatePage, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 50 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	cursor, err := cursorDecode(strings.TrimSpace(input.Cursor))
	if err != nil {
		return nil, err
	}
	keyword := strings.TrimSpace(input.Keyword)
	if len([]byte(keyword)) > 128 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	args := make([]any, 0, 4)
	where := ""
	if cursor > 0 {
		where += " AND id<?"
		args = append(args, cursor)
	}
	if keyword != "" {
		where += " AND (%s)"
	}
	var query string
	switch kind {
	case "customer":
		if keyword != "" {
			where = fmt.Sprintf(where, "customer_no=? OR display_name ILIKE ? ESCAPE '\\\\'")
			args = append(args, keyword, contentEscapeLike(keyword)+"%")
		}
		query = `SELECT id,customer_no AS number,CASE WHEN display_name='' THEN customer_no ELSE display_name END AS label FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND status_code IN ('active','restricted')` + where + ` ORDER BY id DESC LIMIT ?`
	case "service":
		where = strings.ReplaceAll(where, " AND id<?", " AND e.id<?")
		if keyword != "" {
			where = fmt.Sprintf(where, "e.entitlement_no=? OR i.product_name_zh_cn ILIKE ? ESCAPE '\\' OR i.product_name_en_us ILIKE ? ESCAPE '\\'")
			escaped := contentEscapeLike(keyword) + "%"
			args = append(args, keyword, escaped, escaped)
		}
		query = `SELECT e.id,e.entitlement_no AS number,e.entitlement_no||' · '||i.product_name_zh_cn||' / '||i.product_name_en_us AS label FROM plugin_idc_core_service_entitlement e JOIN plugin_idc_core_product_fulfillment_intent i ON i.tenant_id=e.tenant_id AND i.intent_no=e.source_intent_no WHERE e.tenant_id=0 AND e.status_code IN ('active','suspended')` + where + ` ORDER BY e.id DESC LIMIT ?`
	case "group":
		if keyword != "" {
			where = fmt.Sprintf(where, "group_no=? OR name_zh_cn ILIKE ? ESCAPE '\\\\' OR name_en_us ILIKE ? ESCAPE '\\\\'")
			escaped := contentEscapeLike(keyword) + "%"
			args = append(args, keyword, escaped, escaped)
		}
		query = `SELECT id,group_no AS number,name_zh_cn||' / '||name_en_us AS label FROM plugin_idc_core_customer_group WHERE tenant_id=0 AND status_code='active'` + where + ` ORDER BY id DESC LIMIT ?`
	default:
		return nil, bizerr.NewCode(CodeInvalid)
	}
	args = append(args, limit+1)
	var rows []struct {
		ID            int64
		Number, Label string
	}
	if err = s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, contentWrap(err, "search content audience")
	}
	next := ""
	if len(rows) > limit {
		next = cursorEncode(rows[limit-1].ID)
		rows = rows[:limit]
	}
	items := make([]Candidate, 0, len(rows))
	for _, row := range rows {
		items = append(items, Candidate{Number: row.Number, Label: row.Label})
	}
	return &CandidatePage{Items: items, NextCursor: next}, nil
}
