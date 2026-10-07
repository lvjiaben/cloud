// This file builds the bounded local customer-management overview projection.

package customermanagement

import (
	"context"
	"fmt"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

const maximumSegmentPreviewSize = 20

func (*daoPersistence) getOverview(ctx context.Context, customerNo string) (*Overview, error) {
	columns := dao.CustomerSubject.Columns()
	var subject *entity.CustomerSubject
	if err := dao.CustomerSubject.Ctx(ctx).Fields(
		columns.Id, columns.CustomerNo, columns.CustomerTypeCode, columns.StatusCode,
		columns.DisplayName, columns.CountryCode, columns.PreferredLocale, columns.Version,
		columns.EnterpriseContractPolicyCode, columns.EnterpriseContractTemplateNo,
		columns.CreatedAt, columns.UpdatedAt,
	).Where(do.CustomerSubject{
		TenantId: idccap.DefaultTenantID, CustomerNo: customerNo,
	}).Scan(&subject); err != nil {
		return nil, gerror.Wrap(err, "load management customer overview subject")
	}
	if subject == nil {
		return nil, bizerr.NewCode(customer.CodeNotAccessible)
	}
	projection, err := loadBatchProjection(ctx, []*entity.CustomerSubject{subject})
	if err != nil {
		return nil, err
	}
	page, err := projectPage([]*entity.CustomerSubject{subject}, normalizedSearch{limit: 1}, projection)
	if err != nil {
		return nil, gerror.Wrap(err, "project management customer overview subject")
	}
	if page == nil || len(page.Customers) != 1 {
		return nil, gerror.New("management customer overview subject is incomplete")
	}
	groups, err := loadSegmentPreview(ctx, customerNo, true)
	if err != nil {
		return nil, err
	}
	tags, err := loadSegmentPreview(ctx, customerNo, false)
	if err != nil {
		return nil, err
	}
	counts, err := loadOverviewCounts(ctx, customerNo)
	if err != nil {
		return nil, err
	}
	return &Overview{Customer: page.Customers[0], Groups: groups, Tags: tags, Counts: counts}, nil
}

func loadSegmentPreview(ctx context.Context, customerNo string, groups bool) (SegmentPreview, error) {
	definitionTable := dao.CustomerTag.Table()
	assignmentTable := dao.CustomerTagAssignment.Table()
	numberColumn, codeColumn, colorProjection := "tag_no", "tag_code", "d.color_token"
	if groups {
		definitionTable = dao.CustomerGroup.Table()
		assignmentTable = dao.CustomerGroupAssignment.Table()
		numberColumn, codeColumn, colorProjection = "group_no", "group_code", "''"
	}
	query := fmt.Sprintf(`SELECT d.%s AS number, d.%s AS code,
 d.name_zh_cn, d.name_en_us, %s AS color_token, d.status_code
 FROM %s a INNER JOIN %s d ON d.%s = a.%s
 WHERE a.tenant_id = ? AND a.customer_no = ? AND a.status_code = 'active'
 ORDER BY d.name_zh_cn ASC, d.id ASC LIMIT ?`,
		numberColumn, codeColumn, colorProjection, assignmentTable, definitionTable,
		numberColumn, numberColumn,
	)
	rows, err := g.DB().GetAll(ctx, query, idccap.DefaultTenantID, customerNo, maximumSegmentPreviewSize+1)
	if err != nil {
		return SegmentPreview{}, gerror.Wrap(err, "load management customer segment preview")
	}
	hasMore := len(rows) > maximumSegmentPreviewSize
	if hasMore {
		rows = rows[:maximumSegmentPreviewSize]
	}
	preview := SegmentPreview{Items: make([]SegmentSummary, 0, len(rows)), HasMore: hasMore}
	for _, row := range rows {
		item := SegmentSummary{
			Number: row["number"].String(), Code: row["code"].String(),
			NameZhCn: row["name_zh_cn"].String(), NameEnUs: row["name_en_us"].String(),
			ColorToken: row["color_token"].String(), Status: row["status_code"].String(),
		}
		if item.Number == "" || item.Code == "" || item.NameZhCn == "" || item.NameEnUs == "" ||
			(item.Status != "active" && item.Status != "archived") {
			return SegmentPreview{}, gerror.New("customer segment preview is inconsistent")
		}
		preview.Items = append(preview.Items, item)
	}
	return preview, nil
}

func loadOverviewCounts(ctx context.Context, customerNo string) (OverviewCounts, error) {
	query := fmt.Sprintf(`
 SELECT 'active_contacts' AS kind, COUNT(*) AS total FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code = 'active'
 UNION ALL SELECT 'active_addresses', COUNT(*) FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code = 'active'
 UNION ALL SELECT 'active_custom_fields', COUNT(*) FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code = 'active'
 UNION ALL SELECT 'active_notes', COUNT(*) FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code = 'active'
 UNION ALL SELECT 'planned_follow_ups', COUNT(*) FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code = 'planned'
 UNION ALL SELECT 'available_attachments', COUNT(*) FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code = 'available'
 UNION ALL SELECT 'active_blacklist', COUNT(*) FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code = 'active' AND starts_at <= CURRENT_TIMESTAMP AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
 UNION ALL SELECT 'open_risks', COUNT(*) FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code IN ('open', 'acknowledged')
 UNION ALL SELECT 'active_groups', COUNT(*) FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code = 'active'
 UNION ALL SELECT 'active_tags', COUNT(*) FROM %s WHERE tenant_id = ? AND customer_no = ? AND status_code = 'active'`,
		dao.CustomerContact.Table(), dao.CustomerAddress.Table(), dao.CustomerCustomFieldValue.Table(),
		dao.CustomerNote.Table(), dao.CustomerFollowUp.Table(), dao.CustomerAttachment.Table(),
		dao.CustomerBlacklistEntry.Table(), dao.CustomerRiskRecord.Table(),
		dao.CustomerGroupAssignment.Table(), dao.CustomerTagAssignment.Table(),
	)
	arguments := make([]any, 0, 20)
	for range 10 {
		arguments = append(arguments, idccap.DefaultTenantID, customerNo)
	}
	rows, err := g.DB().GetAll(ctx, query, arguments...)
	if err != nil {
		return OverviewCounts{}, gerror.Wrap(err, "load management customer overview counts")
	}
	if len(rows) != 10 {
		return OverviewCounts{}, gerror.New("customer overview counts are incomplete")
	}
	counts := OverviewCounts{}
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		kind, total := row["kind"].String(), row["total"].Int64()
		if total < 0 {
			return OverviewCounts{}, gerror.New("customer overview count is negative")
		}
		if _, exists := seen[kind]; exists {
			return OverviewCounts{}, gerror.New("customer overview count is duplicated")
		}
		seen[kind] = struct{}{}
		switch kind {
		case "active_contacts":
			counts.ActiveContacts = total
		case "active_addresses":
			counts.ActiveAddresses = total
		case "active_custom_fields":
			counts.ActiveCustomFields = total
		case "active_notes":
			counts.ActiveNotes = total
		case "planned_follow_ups":
			counts.PlannedFollowUps = total
		case "available_attachments":
			counts.AvailableAttachments = total
		case "active_blacklist":
			counts.ActiveBlacklist = total
		case "open_risks":
			counts.OpenRisks = total
		case "active_groups":
			counts.ActiveGroups = total
		case "active_tags":
			counts.ActiveTags = total
		default:
			return OverviewCounts{}, gerror.New("customer overview count kind is unknown")
		}
	}
	return counts, nil
}
