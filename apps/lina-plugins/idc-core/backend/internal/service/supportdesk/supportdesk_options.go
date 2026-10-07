package supportdesk

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
)

// SearchPublicCategories returns only active customer-facing labels. Internal
// department routing is deliberately not projected to customers or visitors.
func (s *serviceImpl) SearchPublicCategories(ctx context.Context, input CategorySearchInput) (*CategoryPage, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, bizInvalid()
	}
	cursorSort, cursorID, err := decodeCategoryCursor(strings.TrimSpace(input.Cursor))
	if err != nil {
		return nil, err
	}
	keyword := strings.TrimSpace(input.Keyword)
	if len([]byte(keyword)) > 128 {
		return nil, bizInvalid()
	}
	query := `SELECT id,category_no,name_zh_cn,name_en_us,priority_code AS priority,sort_order FROM plugin_idc_core_support_issue_category WHERE tenant_id=0 AND status_code='active'`
	args := make([]any, 0, 4)
	if cursorID > 0 {
		query += ` AND (sort_order,id)>(?,?)`
		args = append(args, cursorSort, cursorID)
	}
	if keyword != "" {
		query += ` AND (category_no=? OR name_zh_cn ILIKE ? ESCAPE '\' OR name_en_us ILIKE ? ESCAPE '\')`
		like := escapeLike(keyword) + "%"
		args = append(args, keyword, like, like)
	}
	query += ` ORDER BY sort_order,id LIMIT ?`
	args = append(args, limit+1)
	var rows []CategoryOption
	if err = s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, wrapUnavailable(err, "search public support categories")
	}
	next := ""
	if len(rows) > limit {
		next = encodeCategoryCursor(rows[limit-1].SortOrder, rows[limit-1].ID)
		rows = rows[:limit]
	}
	if rows == nil {
		rows = make([]CategoryOption, 0)
	}
	return &CategoryPage{Items: rows, NextCursor: next}, nil
}

func encodeCategoryCursor(sortOrder int, id int64) string {
	if id <= 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(sortOrder) + ":" + strconv.FormatInt(id, 10)))
}

func decodeCategoryCursor(value string) (int, int64, error) {
	if value == "" {
		return 0, 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, 0, bizInvalid()
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 2 {
		return 0, 0, bizInvalid()
	}
	sortOrder, sortErr := strconv.Atoi(parts[0])
	id, idErr := strconv.ParseInt(parts[1], 10, 64)
	if sortErr != nil || idErr != nil || sortOrder < 0 || id <= 0 {
		return 0, 0, bizInvalid()
	}
	return sortOrder, id, nil
}
