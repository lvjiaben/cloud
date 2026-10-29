// Customer-group candidates remain bounded and search user input as literal prefixes.

package contenthub

import (
	"context"
	"fmt"
	"strings"

	"lina-core/pkg/bizerr"
)

func (s *serviceImpl) SearchAudienceCustomerGroups(ctx context.Context, input AudienceSearchInput) (*CandidatePage, error) {
	if _, err := s.staff(ctx); err != nil {
		return nil, err
	}
	return s.searchAudience(ctx, input)
}

func (s *serviceImpl) searchAudience(ctx context.Context, input AudienceSearchInput) (*CandidatePage, error) {
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
	if keyword != "" {
		where = fmt.Sprintf(where, "group_no=? OR name_zh_cn ILIKE ? ESCAPE E'\\\\' OR name_en_us ILIKE ? ESCAPE E'\\\\'")
		escaped := contentEscapeLike(keyword) + "%"
		args = append(args, keyword, escaped, escaped)
	}
	query := `SELECT id,group_no AS number,name_zh_cn||' / '||name_en_us AS label FROM plugin_idc_core_customer_group WHERE tenant_id=0 AND status_code='active'` + where + ` ORDER BY id DESC LIMIT ?`
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
