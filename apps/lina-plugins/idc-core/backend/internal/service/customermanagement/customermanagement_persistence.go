// This file executes the bounded, tenant-scoped customer list projection.

package customermanagement

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/customer"
)

type daoPersistence struct{}

func newDAOPersistence() persistence {
	return &daoPersistence{}
}

func (p *daoPersistence) search(ctx context.Context, input normalizedSearch) (page *Page, err error) {
	err = dao.CustomerSubject.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, readErr := dao.CustomerSubject.DB().Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); readErr != nil {
			return readErr
		}
		var readErr error
		page, readErr = p.searchSnapshot(ctx, input)
		return readErr
	})
	return page, err
}

// searchSnapshot counts the authorized filtered set before cursor/offset and
// batches related projections for the bounded current page only.
func (*daoPersistence) searchSnapshot(ctx context.Context, input normalizedSearch) (*Page, error) {
	columns := dao.CustomerSubject.Columns()
	projectionFields := []string{
		columns.Id,
		columns.CustomerNo,
		columns.CustomerTypeCode,
		columns.StatusCode,
		columns.DisplayName,
		columns.CountryCode,
		columns.PreferredLocale,
		columns.EnterpriseContractPolicyCode,
		columns.EnterpriseContractTemplateNo,
		columns.Version,
		columns.CreatedAt,
		columns.UpdatedAt,
	}
	model := dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{TenantId: idccap.DefaultTenantID})
	if input.customerCode != "" {
		model = model.Where("customer_code", input.customerCode)
	}
	if input.namePrefix != "" {
		model = model.Where(
			columns.SearchName+" LIKE ? ESCAPE '\\'",
			escapeLikeLiteral(input.namePrefix)+"%",
		)
	}
	if input.typeCode != "" {
		model = model.Where(columns.CustomerTypeCode, string(input.typeCode))
	}
	if input.status != "" {
		model = model.Where(columns.StatusCode, string(input.status))
	}
	if input.phone != "" {
		model = model.Where(
			"EXISTS (SELECT 1 FROM "+dao.CustomerContact.Table()+" AS customer_phone_filter "+
				"WHERE customer_phone_filter.tenant_id = ? AND customer_phone_filter.customer_no = "+dao.CustomerSubject.Table()+"."+columns.CustomerNo+" AND "+
				"customer_phone_filter.status_code = 'active' AND customer_phone_filter.phone = ?)",
			idccap.DefaultTenantID,
			input.phone,
		)
	}
	if input.groupNo != "" {
		model = model.Where(
			"EXISTS (SELECT 1 FROM "+dao.CustomerGroupAssignment.Table()+" AS customer_group_filter "+
				"WHERE customer_group_filter.tenant_id = ? AND "+
				"customer_group_filter.customer_no = "+dao.CustomerSubject.Table()+"."+columns.CustomerNo+" AND "+
				"customer_group_filter.group_no = ? AND customer_group_filter.status_code = 'active')",
			idccap.DefaultTenantID,
			input.groupNo,
		)
	}
	if input.tagNo != "" {
		model = model.Where(
			"EXISTS (SELECT 1 FROM "+dao.CustomerTagAssignment.Table()+" AS customer_tag_filter "+
				"WHERE customer_tag_filter.tenant_id = ? AND "+
				"customer_tag_filter.customer_no = "+dao.CustomerSubject.Table()+"."+columns.CustomerNo+" AND "+
				"customer_tag_filter.tag_no = ? AND customer_tag_filter.status_code = 'active')",
			idccap.DefaultTenantID,
			input.tagNo,
		)
	}
	total, err := model.Clone().Count()
	if err != nil {
		return nil, gerror.Wrap(err, "count management customers")
	}
	if input.page > 0 {
		model = model.Offset((input.page - 1) * input.limit)
	}
	if input.cursor != nil {
		cursorCount, cursorErr := dao.CustomerSubject.Ctx(ctx).Where(do.CustomerSubject{
			TenantId: idccap.DefaultTenantID, Id: input.cursor.InternalID,
			CustomerNo: input.cursor.CustomerNo, CreatedAt: &input.cursor.CreatedAt,
		}).Count()
		if cursorErr != nil {
			return nil, gerror.Wrap(cursorErr, "validate management customer cursor boundary")
		}
		if cursorCount != 1 {
			return nil, gerror.New("validate management customer cursor boundary: record mismatch")
		}
		model = model.Where(
			"("+columns.CreatedAt+" < ?) OR ("+columns.CreatedAt+" = ? AND "+columns.Id+" < ?)",
			input.cursor.CreatedAt,
			input.cursor.CreatedAt,
			input.cursor.InternalID,
		)
	}
	var rows []*entity.CustomerSubject
	if err := model.Fields(projectionFields).OrderDesc(columns.CreatedAt).OrderDesc(columns.Id).
		Limit(input.limit + 1).Scan(&rows); err != nil {
		return nil, gerror.Wrap(err, "search management customers")
	}
	pageRows := rows
	if len(pageRows) > input.limit {
		pageRows = pageRows[:input.limit]
	}
	projection, err := loadBatchProjection(ctx, pageRows)
	if err != nil {
		return nil, err
	}
	page, err := projectPage(rows, input, projection)
	if err != nil {
		return nil, err
	}
	page.Total = total
	return page, nil
}

func projectPage(rows []*entity.CustomerSubject, input normalizedSearch, projection batchProjection) (*Page, error) {
	hasMore := len(rows) > input.limit
	if hasMore {
		rows = rows[:input.limit]
	}
	page := &Page{Customers: make([]Summary, 0, len(rows))}
	for _, row := range rows {
		if row == nil || row.Id <= 0 || !validPersistedCustomerNo(row.CustomerNo) ||
			row.CreatedAt == nil || row.UpdatedAt == nil || row.Version <= 0 ||
			!validPersistedType(customer.Type(row.CustomerTypeCode)) ||
			!validPersistedStatus(customer.Status(row.StatusCode)) {
			return nil, gerror.New("customer management row projection is inconsistent")
		}
		verification, ok := projection.verificationByCustomer[row.CustomerNo]
		if !ok || !validPersistedVerification(verification) || projection.tagCountByCustomer[row.CustomerNo] < 0 {
			return nil, gerror.New("customer management batch projection is inconsistent")
		}
		page.Customers = append(page.Customers, Summary{
			CustomerNo: row.CustomerNo, Type: customer.Type(row.CustomerTypeCode),
			CustomerCode: projection.codeByCustomer[row.CustomerNo], PrimaryPhone: projection.phoneByCustomer[row.CustomerNo],
			Status: customer.Status(row.StatusCode), DisplayName: row.DisplayName,
			CountryCode: row.CountryCode, PreferredLocale: row.PreferredLocale,
			EnterpriseContractPolicyCode: row.EnterpriseContractPolicyCode,
			EnterpriseContractTemplateNo: row.EnterpriseContractTemplateNo,
			Verification:                 verification, Level: projection.levelByCustomer[row.CustomerNo],
			TagCount: projection.tagCountByCustomer[row.CustomerNo],
			Version:  row.Version, CreatedAt: *row.CreatedAt, UpdatedAt: *row.UpdatedAt,
		})
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = encodeCustomerCursor(cursorBoundary{
			CreatedAt: *last.CreatedAt, InternalID: last.Id, CustomerNo: last.CustomerNo,
		}, input)
	}
	return page, nil
}

func escapeLikeLiteral(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

func validPersistedCustomerNo(value string) bool {
	if len(value) != 36 || !strings.HasPrefix(value, "CUS_") {
		return false
	}
	for _, char := range value[4:] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return false
		}
	}
	return true
}

func validPersistedType(value customer.Type) bool {
	return value == customer.TypeIndividual || value == customer.TypeEnterprise
}

func validPersistedStatus(value customer.Status) bool {
	return value != "" && validStatus(value)
}

func validPersistedVerification(value customer.VerificationStatus) bool {
	switch value {
	case customer.VerificationUnverified, customer.VerificationPending,
		customer.VerificationVerified, customer.VerificationRejected,
		customer.VerificationExpired, customer.VerificationRevoked:
		return true
	default:
		return false
	}
}
