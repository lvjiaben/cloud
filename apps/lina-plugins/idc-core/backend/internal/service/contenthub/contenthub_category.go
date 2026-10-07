package contenthub

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

func (s *serviceImpl) SearchCategories(ctx context.Context, input CategorySearchInput) (*CategoryPage, error) {
	if _, err := s.staff(ctx); err != nil {
		return nil, err
	}
	if input.Type != "" && !contentCategoryType(input.Type) || (input.Status != "" && input.Status != "active" && input.Status != "disabled") || (input.Locale != "" && input.Locale != "zh-CN" && input.Locale != "en-US") {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	limit := input.Limit
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 100 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	cursorSort, cursorID, err := decodeContentCategoryCursor(strings.TrimSpace(input.Cursor))
	if err != nil {
		return nil, err
	}
	query := `SELECT id,category_no,content_type AS type,name,sort_order,status_code AS status,version,created_at,updated_at FROM plugin_idc_core_content_category WHERE tenant_id=0`
	args := make([]any, 0, 5)
	if input.Type != "" {
		query += ` AND content_type=?`
		args = append(args, input.Type)
	}
	if input.Status != "" {
		query += ` AND status_code=?`
		args = append(args, input.Status)
	}
	if cursorID > 0 {
		query += ` AND (sort_order,id)>(?,?)`
		args = append(args, cursorSort, cursorID)
	}
	query += ` ORDER BY sort_order,id LIMIT ?`
	args = append(args, limit+1)
	var rows []Category
	if err = s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, contentWrap(err, "search content categories")
	}
	next := ""
	if len(rows) > limit {
		next = encodeContentCategoryCursor(rows[limit-1].SortOrder, rows[limit-1].ID)
		rows = rows[:limit]
	}
	items := rows
	if input.Cursor == "" && input.Type != "" && (input.Status == "" || input.Status == "active") {
		name := s.i18n.Translate(ctx, "dict.idc_content_category_group.uncategorized.label", "Uncategorized")
		items = append([]Category{{Type: input.Type, Name: name, Status: "active", Synthetic: true}}, rows...)
	}
	if items == nil {
		items = make([]Category, 0)
	}
	return &CategoryPage{Items: items, NextCursor: next}, nil
}

func (s *serviceImpl) CreateCategory(ctx context.Context, input CategoryCreateInput) (*Category, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	input.Type = strings.TrimSpace(input.Type)
	input.Name = strings.TrimSpace(input.Name)
	if !contentCategoryType(input.Type) || input.Name == "" || len([]byte(input.Name)) > 128 || input.SortOrder < 0 || input.SortOrder > 1_000_000 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	categoryNo, err := s.newNo("CAT_")
	if err != nil {
		return nil, contentWrap(err, "generate content category number")
	}
	categoryNo = strings.ToLower(categoryNo)
	if _, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_category (tenant_id,category_no,content_type,name,sort_order,status_code,created_by,updated_by) VALUES (0,?,?,?,?,'active',?,?)`, categoryNo, input.Type, input.Name, input.SortOrder, userID, userID); err != nil {
		return nil, contentWrap(err, "create content category")
	}
	var rows []Category
	if err = s.database.GetScan(ctx, &rows, `SELECT id,category_no,content_type AS type,name,sort_order,status_code AS status,version,created_at,updated_at FROM plugin_idc_core_content_category WHERE tenant_id=0 AND category_no=? LIMIT 1`, categoryNo); err != nil || len(rows) != 1 {
		return nil, contentWrap(err, "load created content category")
	}
	return &rows[0], nil
}

func (s *serviceImpl) DeleteCategory(ctx context.Context, input CategoryDeleteInput) error {
	userID, err := s.staff(ctx)
	if err != nil {
		return err
	}
	input.CategoryNo = strings.TrimSpace(input.CategoryNo)
	if !businessNumberPattern.MatchString(input.CategoryNo) || input.Version < 1 || !input.Confirmed {
		return bizerr.NewCode(CodeInvalid)
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []Category
		if loadErr := s.database.GetScan(ctx, &rows, `SELECT id,category_no,content_type AS type,name,sort_order,status_code AS status,version,created_at,updated_at FROM plugin_idc_core_content_category WHERE tenant_id=0 AND category_no=? FOR UPDATE`, input.CategoryNo); loadErr != nil {
			return loadErr
		}
		if len(rows) != 1 {
			return bizerr.NewCode(CodeNotAccessible)
		}
		if rows[0].Version != input.Version {
			return bizerr.NewCode(CodeConflict)
		}
		now, nowErr := contentDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if _, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_content_item SET category_no=NULL,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND category_no=?`, userID, now, input.CategoryNo); updateErr != nil {
			return updateErr
		}
		deleted, deleteErr := s.database.Exec(ctx, `DELETE FROM plugin_idc_core_content_category WHERE tenant_id=0 AND category_no=? AND version=?`, input.CategoryNo, input.Version)
		if deleteErr != nil {
			return deleteErr
		}
		if changed, _ := deleted.RowsAffected(); changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return err
		}
		return contentWrap(err, "delete content category")
	}
	return nil
}

func contentCategoryType(value string) bool {
	return value == "news" || value == "knowledge" || value == "download"
}
func encodeContentCategoryCursor(sortOrder int, id int64) string {
	if id <= 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(sortOrder) + ":" + strconv.FormatInt(id, 10)))
}
func decodeContentCategoryCursor(value string) (int, int64, error) {
	if value == "" {
		return 0, 0, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, 0, bizerr.NewCode(CodeInvalid)
	}
	parts := strings.Split(string(raw), ":")
	if len(parts) != 2 {
		return 0, 0, bizerr.NewCode(CodeInvalid)
	}
	sortOrder, sortErr := strconv.Atoi(parts[0])
	id, idErr := strconv.ParseInt(parts[1], 10, 64)
	if sortErr != nil || idErr != nil || sortOrder < 0 || id <= 0 {
		return 0, 0, bizerr.NewCode(CodeInvalid)
	}
	return sortOrder, id, nil
}
