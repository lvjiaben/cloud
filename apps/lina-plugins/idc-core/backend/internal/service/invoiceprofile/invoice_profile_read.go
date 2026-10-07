package invoiceprofile

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type profileRow struct {
	ID                   int64      `orm:"id"`
	ProfileNo            string     `orm:"profile_no"`
	CustomerNo           string     `orm:"customer_no"`
	TitleTypeCode        string     `orm:"title_type_code"`
	InvoiceTitle         string     `orm:"invoice_title"`
	TaxRegistrationNo    string     `orm:"tax_registration_no"`
	RegisteredAddress    string     `orm:"registered_address"`
	RegisteredPhone      string     `orm:"registered_phone"`
	BankName             string     `orm:"bank_name"`
	BankAccountMasked    string     `orm:"bank_account_masked"`
	BankAccountSecretRef string     `orm:"bank_account_secret_ref"`
	IsDefault            bool       `orm:"is_default"`
	StatusCode           string     `orm:"status_code"`
	Version              int64      `orm:"version"`
	CreatedAt            *time.Time `orm:"created_at"`
	UpdatedAt            *time.Time `orm:"updated_at"`
}

const profileProjection = `id, profile_no, customer_no, title_type_code, invoice_title,
       tax_registration_no, registered_address, registered_phone, bank_name,
       bank_account_masked, bank_account_secret_ref, is_default, status_code,
       version, created_at, updated_at`

func (s *serviceImpl) ListCurrentCustomer(ctx context.Context, input ListInput) (*Page, error) {
	subject, err := currentCustomer(ctx, s.customers)
	if err != nil {
		return nil, err
	}
	limit := input.Limit
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > maximumPageSize {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var boundary *cursorBoundary
	if input.Cursor != "" {
		boundary, err = s.cursor.decode(input.Cursor, customerDigest(subject.CustomerNo))
		if err != nil {
			return nil, err
		}
	}
	query := `SELECT ` + profileProjection + `
FROM plugin_idc_core_customer_invoice_profile
WHERE tenant_id = 0 AND customer_no = ?`
	arguments := []any{subject.CustomerNo}
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.UpdatedAtMicroseconds).UTC()
		query += ` AND (updated_at < ? OR (updated_at = ? AND id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += ` ORDER BY updated_at DESC, id DESC LIMIT ?`
	arguments = append(arguments, limit+1)
	var rows []profileRow
	if err = s.database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return nil, gerror.Wrap(bizerr.NewCode(CodeUnavailable), "list current customer invoice profiles")
	}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	result := &Page{Items: make([]Profile, 0, len(rows))}
	for index := range rows {
		item, projectionErr := projectProfile(&rows[index])
		if projectionErr != nil {
			return nil, projectionErr
		}
		result.Items = append(result.Items, *item)
	}
	if more && len(rows) > 0 {
		last := rows[len(rows)-1]
		result.NextCursor, err = s.cursor.encode(cursorBoundary{
			UpdatedAtMicroseconds: last.UpdatedAt.UTC().UnixMicro(), ID: last.ID,
		}, customerDigest(subject.CustomerNo))
		if err != nil {
			return nil, gerror.Wrap(bizerr.NewCode(CodeUnavailable), "encode invoice profile cursor")
		}
	}
	return result, nil
}

func loadOwnedProfile(ctx context.Context, tx gdb.TX, customerNo, profileNo string, lock bool) (*profileRow, error) {
	if !validBusinessNo(profileNo) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	query := `SELECT ` + profileProjection + `
FROM plugin_idc_core_customer_invoice_profile
WHERE tenant_id = 0 AND customer_no = ? AND profile_no = ?`
	if lock {
		query += ` FOR UPDATE`
	}
	var row *profileRow
	if err := tx.GetScan(&row, query, customerNo, profileNo); err != nil {
		return nil, err
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return row, nil
}

func projectProfile(row *profileRow) (*Profile, error) {
	if row == nil || row.ID <= 0 || !validBusinessNo(row.ProfileNo) || !validBusinessNo(row.CustomerNo) ||
		row.Version <= 0 || row.CreatedAt == nil || row.UpdatedAt == nil ||
		(row.StatusCode != StatusActive && row.StatusCode != StatusArchived) ||
		((row.BankAccountMasked == "") != (row.BankAccountSecretRef == "")) {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &Profile{
		ProfileNo: row.ProfileNo, TitleTypeCode: row.TitleTypeCode, InvoiceTitle: row.InvoiceTitle,
		TaxRegistrationNo: row.TaxRegistrationNo, RegisteredAddress: row.RegisteredAddress,
		RegisteredPhone: row.RegisteredPhone, BankName: row.BankName,
		BankAccountMasked: row.BankAccountMasked, BankAccountConfigured: row.BankAccountSecretRef != "",
		IsDefault: row.IsDefault, StatusCode: row.StatusCode, Version: row.Version,
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}
