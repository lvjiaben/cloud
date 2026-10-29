// Content writes persist versioned changes and acknowledge deletion without reopening tombstones.

package contenthub

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
)

// contentStatusDeleted is the existing terminal status acknowledged by deletion.
const contentStatusDeleted = "deleted"

type normalizedWrite struct{ WriteInput }

func normalizeWrite(input WriteInput) (normalizedWrite, error) {
	var ok bool
	input.Title, ok = trimBound(input.Title, 256, true)
	if !ok {
		return normalizedWrite{}, bizerr.NewCode(CodeInvalid)
	}
	input.Summary, ok = trimBound(input.Summary, 2000, false)
	if !ok {
		return normalizedWrite{}, bizerr.NewCode(CodeInvalid)
	}
	input.Body, ok = trimBound(input.Body, 1_000_000, true)
	if !ok {
		return normalizedWrite{}, bizerr.NewCode(CodeInvalid)
	}
	input.SEOTitle, ok = trimBound(input.SEOTitle, 256, false)
	if !ok {
		return normalizedWrite{}, bizerr.NewCode(CodeInvalid)
	}
	input.SEODescription, ok = trimBound(input.SEODescription, 2000, false)
	if !ok || !contentTypeValid(input.Type) || !visibilityValid(input.Visibility) {
		return normalizedWrite{}, bizerr.NewCode(CodeInvalid)
	}
	if input.CategoryNo != "" && !businessNumberPattern.MatchString(input.CategoryNo) {
		return normalizedWrite{}, bizerr.NewCode(CodeInvalid)
	}
	var valid bool
	input.TargetCustomerGroupNos, valid = uniqueBusinessNos(input.TargetCustomerGroupNos, 500)
	if !valid {
		return normalizedWrite{}, bizerr.NewCode(CodeInvalid)
	}
	if !targetsMatchVisibility(input) {
		return normalizedWrite{}, bizerr.NewCode(CodeInvalid)
	}
	return normalizedWrite{input}, nil
}
func targetsMatchVisibility(input WriteInput) bool {
	switch input.Visibility {
	case "customer_group":
		return len(input.TargetCustomerGroupNos) > 0

	default:
		return len(input.TargetCustomerGroupNos) == 0
	}
}

func (s *serviceImpl) Create(ctx context.Context, input WriteInput) (*Item, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeWrite(input)
	if err != nil {
		return nil, err
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "draft"
	}
	if status != "draft" && status != "published" {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	if normalized.Type == "download" && status == "published" {
		return nil, bizerr.NewCode(CodeConflict)
	}
	contentNo, err := s.newNo("CNT_")
	if err != nil {
		return nil, contentWrap(err, "generate content number")
	}
	contentNo = strings.ToLower(contentNo)
	actionNo, err := s.newNo("CAH_")
	if err != nil {
		return nil, contentWrap(err, "generate content history")
	}
	publicPath := contentPublicPath(normalized.Type, contentNo)
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := s.validateReferences(ctx, normalized.WriteInput); err != nil {
			return err
		}
		now, err := contentDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		var publishedAt any
		if status == "published" {
			publishedAt = now
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_item (tenant_id,content_no,content_type,category_no,visibility_code,title,summary,body,seo_title,seo_description,public_path,status_code,published_at,created_by,updated_by,created_at,updated_at) VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, contentNo, normalized.Type, contentNullable(normalized.CategoryNo), normalized.Visibility, normalized.Title, contentNullable(normalized.Summary), normalized.Body, contentNullable(normalized.SEOTitle), contentNullable(normalized.SEODescription), publicPath, status, publishedAt, userID, userID, now, now)
		if err != nil {
			return err
		}
		if err = s.replaceTargets(ctx, contentNo, normalized.WriteInput); err != nil {
			return err
		}
		action := "draft_saved"
		if status == "published" {
			action = "published"
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_action_history (tenant_id,action_no,content_no,action_code,actor_user_id,content_version,occurred_at) VALUES (0,?,?,?,?,1,?)`, actionNo, contentNo, action, userID, now)
		return err
	})
	if err != nil {
		return nil, contentWrap(err, "create content")
	}
	return s.DetailAdmin(ctx, contentNo)
}

func contentPublicPath(contentType, contentNo string) string {
	segment := contentType
	switch contentType {
	case "download":
		segment = "downloads"
	}
	return "/pages/" + segment + "/" + strings.ToLower(contentNo)
}

func (s *serviceImpl) Update(ctx context.Context, input WriteInput) (*Item, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	if !businessNumberPattern.MatchString(input.ContentNo) || input.Version <= 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	normalized, err := normalizeWrite(input)
	if err != nil {
		return nil, err
	}
	actionNo, err := s.newNo("CAH_")
	if err != nil {
		return nil, contentWrap(err, "generate content history")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []struct{ Type, Status, Visibility string }
		if err := s.database.GetScan(ctx, &rows, `SELECT content_type AS type,status_code AS status,visibility_code AS visibility FROM plugin_idc_core_content_item WHERE tenant_id=0 AND content_no=? AND deleted_at IS NULL FOR UPDATE`, input.ContentNo); err != nil {
			return err
		}
		if len(rows) != 1 {
			return bizerr.NewCode(CodeNotFound)
		}
		if rows[0].Type != normalized.Type {
			return bizerr.NewCode(CodeConflict)
		}
		if err := s.validateReferences(ctx, normalized.WriteInput); err != nil {
			return err
		}
		visibilityChanged, err := s.visibilityTargetsChanged(ctx, input.ContentNo, rows[0].Visibility, normalized.WriteInput)
		if err != nil {
			return err
		}
		validityIncrement := 0
		if visibilityChanged {
			validityIncrement = 1
		}
		now, err := contentDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_content_item SET category_no=?,visibility_code=?,title=?,summary=?,body=?,seo_title=?,seo_description=?,validity_version=validity_version+?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND content_no=? AND version=? AND status_code<>'deleted'`, contentNullable(normalized.CategoryNo), normalized.Visibility, normalized.Title, contentNullable(normalized.Summary), normalized.Body, contentNullable(normalized.SEOTitle), contentNullable(normalized.SEODescription), validityIncrement, userID, now, input.ContentNo, input.Version)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if err = s.replaceTargets(ctx, input.ContentNo, normalized.WriteInput); err != nil {
			return err
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_action_history (tenant_id,action_no,content_no,action_code,actor_user_id,content_version,occurred_at) VALUES (0,?,?, 'modified',?,?,?)`, actionNo, input.ContentNo, userID, input.Version+1, now)
		return err
	})
	if err != nil {
		return nil, contentWrap(err, "update content")
	}
	return s.DetailAdmin(ctx, input.ContentNo)
}

func (s *serviceImpl) Publish(ctx context.Context, contentNo string, version int64) (*Item, error) {
	return s.changeStatus(ctx, contentNo, version, "draft", "published", "published")
}
func (s *serviceImpl) Unpublish(ctx context.Context, contentNo string, version int64) (*Item, error) {
	return s.changeStatus(ctx, contentNo, version, "published", "offline", "offline")
}
func (s *serviceImpl) Republish(ctx context.Context, contentNo string, version int64) (*Item, error) {
	return s.changeStatus(ctx, contentNo, version, "offline", "published", "republished")
}
func (s *serviceImpl) changeStatus(ctx context.Context, contentNo string, version int64, from, to, action string) (*Item, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	if !businessNumberPattern.MatchString(contentNo) || version <= 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actionNo, err := s.newNo("CAH_")
	if err != nil {
		return nil, contentWrap(err, "generate content history")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var rows []struct{ Type, CurrentFileNo string }
		if err := s.database.GetScan(ctx, &rows, `SELECT content_type AS type,COALESCE(current_file_no,'') AS current_file_no FROM plugin_idc_core_content_item WHERE tenant_id=0 AND content_no=? AND status_code=? AND version=? FOR UPDATE`, contentNo, from, version); err != nil {
			return err
		}
		if len(rows) != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		if to == "published" && rows[0].Type == "download" && rows[0].CurrentFileNo == "" {
			return bizerr.NewCode(CodeConflict)
		}
		now, err := contentDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_content_item SET status_code=?,published_at=CASE WHEN ?='published' THEN ? ELSE published_at END,offline_at=CASE WHEN ?='offline' THEN ? WHEN ?='published' THEN NULL ELSE offline_at END,validity_version=validity_version+1,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND content_no=? AND status_code=? AND version=?`, to, to, now, to, now, to, userID, now, contentNo, from, version)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_action_history (tenant_id,action_no,content_no,action_code,actor_user_id,content_version,occurred_at) VALUES (0,?,?,?,?,?,?)`, actionNo, contentNo, action, userID, version+1, now)
		return err
	})
	if err != nil {
		return nil, contentWrap(err, "change content status")
	}
	return s.DetailAdmin(ctx, contentNo)
}

func (s *serviceImpl) Delete(ctx context.Context, contentNo string, version int64) (*Item, error) {
	userID, err := s.staff(ctx)
	if err != nil {
		return nil, err
	}
	if !businessNumberPattern.MatchString(contentNo) || version <= 0 {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	actionNo, err := s.newNo("CAH_")
	if err != nil {
		return nil, contentWrap(err, "generate content history")
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, err := contentDatabaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		result, err := s.database.Exec(ctx, `UPDATE plugin_idc_core_content_item SET status_code='deleted',title='[deleted]',summary=NULL,body='',seo_title=NULL,seo_description=NULL,category_no=NULL,current_file_no=NULL,validity_version=validity_version+1,deleted_at=?,version=version+1,updated_by=?,updated_at=? WHERE tenant_id=0 AND content_no=? AND status_code<>'deleted' AND version=?`, now, userID, now, contentNo, version)
		if err != nil {
			return err
		}
		changed, _ := result.RowsAffected()
		if changed != 1 {
			return bizerr.NewCode(CodeConflict)
		}
		// Deleting content is also the irreversible fence for every file workflow
		// owned by that content. Stop unfinished uploads and scan attempts in the
		// same transaction, make late scanner results lose their candidate fence,
		// and hand both candidate and current objects to the durable cleanup worker.
		if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_content_upload_session
            SET status_code='cancelled',version=version+1,updated_at=?
            WHERE tenant_id=0 AND content_no=? AND status_code='uploading'`, now, contentNo); err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_content_scan_attempt a
            SET status_code='failed',started_at=COALESCE(a.started_at,?),finished_at=?,
                lease_expires_at=NULL,version=a.version+1
            FROM plugin_idc_core_content_file f
            WHERE a.tenant_id=0 AND f.tenant_id=a.tenant_id AND f.file_no=a.file_no
              AND f.content_no=? AND f.role_code='candidate'
              AND a.status_code IN ('pending','scanning')`, now, now, contentNo); err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_content_file
            SET role_code=CASE WHEN role_code='current' THEN 'retired' ELSE role_code END,
                status_code=CASE WHEN role_code='current' THEN 'replaced' ELSE 'cancelled' END,
                object_ref=CASE WHEN role_code='candidate' THEN NULL ELSE object_ref END,
                cancelled_at=CASE WHEN role_code='candidate' THEN ? ELSE cancelled_at END,
                retention_expires_at=NULL,version=version+1,updated_at=?
            WHERE tenant_id=0 AND content_no=?
              AND ((role_code='current' AND status_code='active')
                   OR (role_code='candidate' AND status_code IN ('checking','ready')))`, now, now, contentNo); err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `DELETE FROM plugin_idc_core_content_group_scope WHERE tenant_id=0 AND content_no=?`, contentNo); err != nil {
			return err
		}
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_content_action_history (tenant_id,action_no,content_no,action_code,actor_user_id,content_version,occurred_at) VALUES (0,?,?, 'deleted',?,?,?)`, actionNo, contentNo, userID, version+1, now)
		return err
	})
	if err != nil {
		return nil, contentWrap(err, "delete content")
	}
	// Deletion is already committed. Return an acknowledgement rather than
	// reopening a tombstone through the ordinary detail reader.
	return &Item{ContentNo: contentNo, Status: contentStatusDeleted, Version: version + 1}, nil
}

func (s *serviceImpl) validateReferences(ctx context.Context, input WriteInput) error {
	if input.CategoryNo != "" {
		count, err := s.database.GetValue(ctx, `SELECT COUNT(1) FROM plugin_idc_core_content_category WHERE tenant_id=0 AND category_no=? AND content_type=? AND status_code='active'`, input.CategoryNo, input.Type)
		if err != nil {
			return err
		}
		if count.Int() != 1 {
			return bizerr.NewCode(CodeInvalid)
		}
	}
	checks := []struct {
		values []string
		query  string
	}{{input.TargetCustomerGroupNos, `SELECT COUNT(1) FROM plugin_idc_core_customer_group WHERE tenant_id=0 AND group_no IN (%s) AND status_code='active'`}}
	for _, check := range checks {
		if len(check.values) == 0 {
			continue
		}
		args := make([]any, len(check.values))
		for i, value := range check.values {
			args[i] = value
		}
		count, err := s.database.GetValue(ctx, fmt.Sprintf(check.query, contentPlaceholders(len(args))), args...)
		if err != nil {
			return err
		}
		if count.Int() != len(check.values) {
			return bizerr.NewCode(CodeNotAccessible)
		}
	}
	return nil
}

func (s *serviceImpl) visibilityTargetsChanged(ctx context.Context, contentNo, previousVisibility string, input WriteInput) (bool, error) {
	if previousVisibility != input.Visibility {
		return true, nil
	}
	var table, column string
	var expected []string
	switch input.Visibility {
	case "customer_group":
		table, column, expected = "plugin_idc_core_content_group_scope", "group_no", input.TargetCustomerGroupNos
	default:
		return false, nil
	}
	var rows []struct{ Value string }
	if err := s.database.GetScan(ctx, &rows, "SELECT "+column+" AS value FROM "+table+" WHERE tenant_id=0 AND content_no=?", contentNo); err != nil {
		return false, err
	}
	if len(rows) != len(expected) {
		return true, nil
	}
	values := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		values[row.Value] = struct{}{}
	}
	for _, value := range expected {
		if _, ok := values[value]; !ok {
			return true, nil
		}
	}
	return false, nil
}

func (s *serviceImpl) replaceTargets(ctx context.Context, contentNo string, input WriteInput) error {
	// The input is normalized and authorized before the enclosing transaction.
	// One delete and one batch insert, independent of audience size.
	scopes := []struct {
		table, column string
		values        []string
	}{
		{"plugin_idc_core_content_group_scope", "group_no", input.TargetCustomerGroupNos},
	}
	for _, scope := range scopes {
		if _, err := s.database.Exec(ctx, "DELETE FROM "+scope.table+" WHERE tenant_id=0 AND content_no=?", contentNo); err != nil {
			return err
		}
		if len(scope.values) == 0 {
			continue
		}
		args := make([]any, 0, len(scope.values)*2)
		for _, value := range scope.values {
			args = append(args, contentNo, value)
		}
		rows := strings.TrimSuffix(strings.Repeat("(0,?,?),", len(scope.values)), ",")
		if _, err := s.database.Exec(ctx, "INSERT INTO "+scope.table+" (tenant_id,content_no,"+scope.column+") VALUES "+rows, args...); err != nil {
			return err
		}
	}
	return nil
}
func contentPlaceholders(count int) string {
	return strings.TrimRight(strings.Repeat("?,", count), ",")
}
func contentNullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func contentDatabaseNow(ctx context.Context, database gdb.DB) (time.Time, error) {
	value, err := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if err != nil {
		return time.Time{}, err
	}
	return value.Time().UTC(), nil
}
