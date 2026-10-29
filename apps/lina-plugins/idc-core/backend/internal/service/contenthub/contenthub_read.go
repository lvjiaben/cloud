// Content readers apply authorization and audience filters before pagination.

package contenthub

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
)

// SearchAdmin checks the direct staff boundary before obtaining bounded rows and an exact filtered total.
func (s *serviceImpl) SearchAdmin(ctx context.Context, input SearchInput) (*Page, error) {
	if _, err := s.staff(ctx); err != nil {
		return nil, err
	}
	return s.search(ctx, input, "", true)
}
func (s *serviceImpl) SearchPublic(ctx context.Context, input SearchInput) (*Page, error) {
	return s.search(ctx, input, "", false)
}
func (s *serviceImpl) SearchAccount(ctx context.Context, input SearchInput) (*Page, error) {
	_, customerNo, err := s.customer(ctx)
	if err != nil {
		return nil, err
	}
	return s.search(ctx, input, customerNo, false)
}

func (s *serviceImpl) search(ctx context.Context, input SearchInput, customerNo string, admin bool) (*Page, error) {
	limit := input.Limit
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 || input.Page < 0 || input.Page > 1000000 || (input.Page > 0 && (!admin || strings.TrimSpace(input.Cursor) != "")) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	cursor, err := cursorDecode(strings.TrimSpace(input.Cursor))
	if err != nil {
		return nil, err
	}
	var where strings.Builder
	where.WriteString(" WHERE i.tenant_id=0 AND i.content_type<>'announcement'")
	args := make([]any, 0, 16)
	if admin {
		where.WriteString(" AND i.status_code<>'deleted'")
	} else if customerNo == "" {
		where.WriteString(" AND i.status_code='published' AND i.visibility_code='public'")
	} else {
		where.WriteString(` AND i.status_code='published' AND (i.visibility_code IN ('public','all_logged_in') OR (i.visibility_code='customer_group' AND EXISTS (SELECT 1 FROM plugin_idc_core_content_group_scope gs JOIN plugin_idc_core_customer_group_assignment ga ON ga.tenant_id=0 AND ga.group_no=gs.group_no AND ga.customer_no=? AND ga.status_code='active' WHERE gs.tenant_id=0 AND gs.content_no=i.content_no)))`)
		args = append(args, customerNo)
	}

	if categoryNo := strings.TrimSpace(input.CategoryNo); categoryNo != "" {
		if !businessNumberPattern.MatchString(categoryNo) {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		where.WriteString(" AND i.category_no=?")
		args = append(args, categoryNo)
	}
	for _, filter := range []struct{ value, column string }{{strings.TrimSpace(input.Type), "i.content_type"}, {strings.TrimSpace(input.Status), "i.status_code"}, {strings.TrimSpace(input.Visibility), "i.visibility_code"}} {
		if filter.value != "" {
			where.WriteString(" AND " + filter.column + "=?")
			args = append(args, filter.value)
		}
	}
	if query := strings.TrimSpace(input.Query); query != "" {
		if len([]byte(query)) > 128 {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		where.WriteString(" AND (i.content_no=? OR i.title ILIKE ? ESCAPE E'\\\\')")
		args = append(args, query, contentEscapeLike(query)+"%")
	}
	// Count and rows use identical authorization and business filters; query count is constant.
	total := 0
	if admin {
		count, countErr := s.database.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_content_item i"+where.String(), args...)
		if countErr != nil {
			return nil, contentWrap(countErr, "count managed content")
		}
		total = count.Int()
	}
	if cursor > 0 {
		where.WriteString(" AND i.id<?")
		args = append(args, cursor)
	}
	args = append(args, limit+1)
	querySQL := "SELECT " + contentListProjection("i", "c") + " FROM plugin_idc_core_content_item i LEFT JOIN plugin_idc_core_content_category c ON c.tenant_id=0 AND c.category_no=i.category_no" + where.String() + " ORDER BY i.id DESC LIMIT ?"
	if input.Page > 0 {
		querySQL += " OFFSET ?"
		args = append(args, (input.Page-1)*limit)
	}
	var rows []*Item
	if err = s.database.GetScan(ctx, &rows, querySQL, args...); err != nil {
		return nil, contentWrap(err, "search content")
	}
	next := ""
	if len(rows) > limit {
		next = cursorEncode(rows[limit-1].ID)
		rows = rows[:limit]
	}
	return &Page{Items: rows, NextCursor: next, Total: total}, nil
}

func (s *serviceImpl) DetailAdmin(ctx context.Context, contentNo string) (*Item, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, contentNo, "", true, userID)
}
func (s *serviceImpl) DetailPublic(ctx context.Context, contentNo string) (*Item, error) {
	return s.detail(ctx, contentNo, "", false, 0)
}
func (s *serviceImpl) DetailAccount(ctx context.Context, contentNo string) (*Item, error) {
	userID, customerNo, err := s.customer(ctx)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, contentNo, customerNo, false, userID)
}

func (s *serviceImpl) detail(ctx context.Context, contentNo, customerNo string, admin bool, userID int) (*Item, error) {
	if !businessNumberPattern.MatchString(contentNo) {
		return nil, bizerr.NewCode(CodeNotFound)
	}
	var where strings.Builder
	where.WriteString(" WHERE i.tenant_id=0 AND i.status_code<>'deleted' AND i.content_type<>'announcement' AND i.content_no=?")
	args := []any{contentNo}
	if !admin && customerNo == "" {
		where.WriteString(" AND i.status_code='published' AND i.visibility_code='public'")
	} else if !admin {
		where.WriteString(` AND i.status_code='published' AND (i.visibility_code IN ('public','all_logged_in') OR (i.visibility_code='customer_group' AND EXISTS (SELECT 1 FROM plugin_idc_core_content_group_scope gs JOIN plugin_idc_core_customer_group_assignment ga ON ga.tenant_id=0 AND ga.group_no=gs.group_no AND ga.customer_no=? AND ga.status_code='active' WHERE gs.tenant_id=0 AND gs.content_no=i.content_no)))`)
		args = append(args, customerNo)
	}
	var rows []*Item
	if err := s.database.GetScan(ctx, &rows, "SELECT "+contentProjection("i", "c")+" FROM plugin_idc_core_content_item i LEFT JOIN plugin_idc_core_content_category c ON c.tenant_id=0 AND c.category_no=i.category_no"+where.String()+" LIMIT 1", args...); err != nil {
		return nil, contentWrap(err, "load content")
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeNotFound)
	}
	// Content metadata follows title and description, including older records.
	item := rows[0]
	item.SEOTitle = item.Title
	item.SEODescription = item.Summary
	if err := s.loadTargets(ctx, item); err != nil {
		return nil, err
	}
	if admin {
		if err := s.loadUploadState(ctx, item, userID); err != nil {
			return nil, err
		}
	}
	if item.Type == "download" && item.CurrentFileNo != "" {
		if err := s.loadCurrentFile(ctx, item); err != nil {
			return nil, err
		}
	}
	if admin {
		item.AvailableActions = adminAvailableActions(item)
	}
	return item, nil
}

func (s *serviceImpl) loadCurrentFile(ctx context.Context, item *Item) error {
	var rows []CurrentFile
	if err := s.database.GetScan(ctx, &rows, `SELECT file_no,file_name,media_type,content_kind,sha256_hex AS sha256,size_bytes,version FROM plugin_idc_core_content_file WHERE tenant_id=0 AND content_no=? AND file_no=? AND role_code='current' AND status_code='active' LIMIT 1`, item.ContentNo, item.CurrentFileNo); err != nil {
		return contentWrap(err, "load current content file")
	}
	if len(rows) != 1 {
		return bizerr.NewCode(CodeNotFound)
	}
	item.CurrentFile = &rows[0]
	return nil
}

func adminAvailableActions(item *Item) []string {
	if item == nil || item.Status == "deleted" {
		return []string{}
	}
	actions := []string{"update", "delete"}
	switch item.Status {
	case "published":
		actions = append(actions, "unpublish")
	case "draft", "offline":
		if item.Type != "download" || item.CurrentFile != nil {
			actions = append(actions, "publish")
		}
	}
	if item.Type == "download" {
		if item.ActiveUpload == nil {
			actions = append(actions, "create_upload")
		} else if item.ActiveUpload.CanCancel {
			actions = append(actions, "cancel_upload")
		}
		if candidate := item.CandidateFile; candidate != nil {
			if candidate.CanCancel {
				actions = append(actions, "cancel_candidate")
			}
			if candidate.CanRescan {
				actions = append(actions, "rescan_candidate")
			}
			if candidate.CanEnable {
				actions = append(actions, "enable_candidate")
			}
		}
	}
	return actions
}

func (s *serviceImpl) loadTargets(ctx context.Context, item *Item) error {
	if item == nil {
		return nil
	}
	var audiences []TargetAudience
	if err := s.database.GetScan(ctx, &audiences, `
SELECT 'customer_group' AS kind,
       scope.group_no AS number,
       customer_group.name_zh_cn||' / '||customer_group.name_en_us AS label
FROM plugin_idc_core_content_group_scope scope
JOIN plugin_idc_core_customer_group customer_group
  ON customer_group.tenant_id=0 AND customer_group.group_no=scope.group_no
WHERE scope.tenant_id=0 AND scope.content_no=?
ORDER BY kind,number`, item.ContentNo); err != nil {
		return contentWrap(err, "load content audience labels")
	}
	item.TargetCustomerGroupNos = make([]string, 0)
	for _, audience := range audiences {
		item.TargetCustomerGroupNos = append(item.TargetCustomerGroupNos, audience.Number)
	}
	if audiences == nil {
		audiences = make([]TargetAudience, 0)
	}
	item.TargetAudiences = audiences
	return nil
}
func (s *serviceImpl) loadUploadState(ctx context.Context, item *Item, userID int) error {
	var uploads []struct {
		UploadNo, FileName                    string
		DeclaredSizeBytes, PersistedSizeBytes int64
		ChunkSizeBytes                        int64
		ExpiresAt                             time.Time
		Version, UploaderUserID               int64
	}
	if err := s.database.GetScan(ctx, &uploads, `SELECT upload_no,file_name,declared_size_bytes,persisted_size_bytes,chunk_size_bytes,expires_at,version,uploader_user_id FROM plugin_idc_core_content_upload_session WHERE tenant_id=0 AND content_no=? AND status_code='uploading' ORDER BY id DESC LIMIT 1`, item.ContentNo); err != nil {
		return contentWrap(err, "load active content upload")
	}
	if len(uploads) == 1 {
		u := uploads[0]
		item.ActiveUpload = &ActiveUpload{UploadNo: u.UploadNo, FileName: u.FileName, SizeBytes: u.DeclaredSizeBytes, ConfirmedBytes: u.PersistedSizeBytes, ChunkSizeBytes: u.ChunkSizeBytes, ExpiresAt: u.ExpiresAt, Version: u.Version, CanResume: u.UploaderUserID == int64(userID), CanCancel: true}
	}
	var files []struct {
		FileNo, FileName, MediaType, ContentKind, StatusCode, ScanStatus string
		SizeBytes, Version                                               int64
		RetentionExpiresAt                                               *time.Time
		RescanAllowed                                                    bool
	}
	if err := s.database.GetScan(ctx, &files, `SELECT file_no,file_name,media_type,content_kind,status_code,scan_status,size_bytes,version,retention_expires_at,(retention_expires_at IS NULL OR retention_expires_at>CURRENT_TIMESTAMP) AS rescan_allowed FROM plugin_idc_core_content_file WHERE tenant_id=0 AND content_no=? AND role_code='candidate' AND status_code IN ('checking','ready') ORDER BY candidate_generation DESC LIMIT 1`, item.ContentNo); err != nil {
		return contentWrap(err, "load content candidate")
	}
	if len(files) == 1 {
		f := files[0]
		canRescan := s.scannerReady() && f.RescanAllowed && (f.ScanStatus == "failed" || f.ScanStatus == "not_configured")
		item.CandidateFile = &CandidateFile{FileNo: f.FileNo, FileName: f.FileName, MediaType: f.MediaType, ContentKind: f.ContentKind, GateStatus: f.StatusCode, ScanStatus: f.ScanStatus, SizeBytes: f.SizeBytes, Version: f.Version, GateExpiresAt: f.RetentionExpiresAt, CanCancel: true, CanRescan: canRescan, CanEnable: f.StatusCode == "ready" && f.ScanStatus == "passed" && f.RescanAllowed}
	}
	return nil
}

func contentProjection(a, categoryAlias string) string {
	return fmt.Sprintf(`%s.id,%s.content_no,%s.content_type AS type,COALESCE(%s.category_no,'') AS category_no,COALESCE(%s.name,'') AS category_name_zh_cn,COALESCE(%s.name,'') AS category_name_en_us,%s.visibility_code AS visibility,%s.title,COALESCE(%s.summary,'') AS summary,%s.body,COALESCE(%s.seo_title,'') AS seo_title,COALESCE(%s.seo_description,'') AS seo_description,%s.public_path,%s.status_code AS status,%s.validity_version,COALESCE(%s.current_file_no,'') AS current_file_no,%s.published_at,%s.offline_at,%s.version,%s.created_at,%s.updated_at`, a, a, a, a, categoryAlias, categoryAlias, a, a, a, a, a, a, a, a, a, a, a, a, a, a, a)
}
func contentListProjection(a, categoryAlias string) string {
	return fmt.Sprintf(`%s.id,%s.content_no,%s.content_type AS type,COALESCE(%s.category_no,'') AS category_no,COALESCE(%s.name,'') AS category_name_zh_cn,COALESCE(%s.name,'') AS category_name_en_us,%s.visibility_code AS visibility,%s.title,COALESCE(%s.summary,'') AS summary,'' AS body,'' AS seo_title,'' AS seo_description,%s.public_path,%s.status_code AS status,%s.validity_version,COALESCE(%s.current_file_no,'') AS current_file_no,%s.published_at,%s.offline_at,%s.version,%s.created_at,%s.updated_at`, a, a, a, a, categoryAlias, categoryAlias, a, a, a, a, a, a, a, a, a, a, a, a)
}
func contentEscapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
