// This file loads bounded customer and management credit-application projections.

package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type creditApplicationDirectoryRow struct {
	ID                    int64     `orm:"id"`
	ApplicationNo         string    `orm:"application_no"`
	CustomerNo            string    `orm:"customer_no"`
	CurrencyCode          string    `orm:"currency_code"`
	PolicyNo              string    `orm:"policy_no"`
	PolicyVersion         int64     `orm:"policy_version"`
	RequestedLimitMinor   int64     `orm:"requested_limit_minor"`
	RequestedValidFrom    time.Time `orm:"requested_valid_from"`
	RequestedValidUntil   time.Time `orm:"requested_valid_until"`
	RequiredApprovalCount int       `orm:"required_approval_count"`
	ReviewCount           int       `orm:"review_count"`
	StatusCode            string    `orm:"status_code"`
	CreditAccountNo       string    `orm:"credit_account_no"`
	CreatedAt             time.Time `orm:"created_at"`
	UpdatedAt             time.Time `orm:"updated_at"`
}

type creditApplicationReviewDirectoryRow struct {
	ReviewNo       string    `orm:"review_no"`
	ReviewSequence int       `orm:"review_sequence"`
	DecisionCode   string    `orm:"decision_code"`
	ReviewerID     int       `orm:"reviewer_id"`
	CreatedAt      time.Time `orm:"created_at"`
}

func (s *accountCreditDirectoryImpl) ListAccountCreditApplications(
	ctx context.Context,
	filter CreditApplicationDirectoryFilter,
) (*CreditApplicationDirectoryPage, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	filter.CustomerNo = audience.CustomerNo
	return s.store.listCreditApplications(ctx, filter, false)
}

func (s *managementCreditDirectoryImpl) ListManagementCreditApplications(
	ctx context.Context,
	filter CreditApplicationDirectoryFilter,
) (*CreditApplicationDirectoryPage, error) {
	if _, err := s.authorizer.AuthorizeApplicationList(ctx); err != nil {
		return nil, err
	}
	return s.store.listCreditApplications(ctx, filter, true)
}

func (s *creditDirectoryStore) listCreditApplications(
	ctx context.Context,
	filter CreditApplicationDirectoryFilter,
	management bool,
) (*CreditApplicationDirectoryPage, error) {
	var err error
	filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Cursor, filter.Limit, err =
		normalizeCreditDirectoryFilter(
			filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Cursor, filter.Limit,
		)
	if err != nil {
		return nil, err
	}
	kind := "account_credit_application"
	offset, err := creditPageOffset(filter.Page, filter.Limit, filter.Cursor)
	if err != nil {
		return nil, err
	}
	if management {
		kind = "management_credit_application"
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, StatusCode, CurrencyCode string
		Limit                                      int
	}{kind, filter.CustomerNo, filter.StatusCode, filter.CurrencyCode, filter.Limit})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	var boundary *directoryCursorBoundary
	if filter.Cursor != "" {
		boundary, err = s.cursor.decode(filter.Cursor, digest)
		if err != nil {
			return nil, bizerr.NewCode(CodeDirectoryInvalid)
		}
	}
	query := `
SELECT application.id, application.application_no, application.customer_no,
       application.currency_code, application.policy_no, application.policy_version,
       application.requested_limit_minor, application.requested_valid_from,
       application.requested_valid_until, application.required_approval_count,
       application.status_code, application.created_at, application.updated_at,
       COALESCE(review.review_count, 0) AS review_count,
       COALESCE(account.credit_account_no, '') AS credit_account_no
FROM plugin_idc_core_credit_application application
LEFT JOIN LATERAL (
    SELECT COUNT(*)::INT AS review_count
    FROM plugin_idc_core_credit_application_review
    WHERE tenant_id = 0 AND application_no = application.application_no
) review ON TRUE
LEFT JOIN plugin_idc_core_credit_account account
  ON account.tenant_id = 0 AND account.origin_application_no = application.application_no
WHERE application.tenant_id = 0`
	arguments := make([]any, 0, 8)
	if filter.CustomerNo != "" {
		query += ` AND application.customer_no = ?`
		arguments = append(arguments, filter.CustomerNo)
	}
	if filter.StatusCode != "" {
		query += ` AND application.status_code = ?`
		arguments = append(arguments, filter.StatusCode)
	}
	if filter.CurrencyCode != "" {
		query += ` AND application.currency_code = ?`
		arguments = append(arguments, filter.CurrencyCode)
	}
	filteredQuery, filteredArguments := query, append([]any(nil), arguments...)
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.SortTimeMicroseconds)
		query += ` AND ((application.created_at < ?) OR (application.created_at = ? AND application.id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += ` ORDER BY application.created_at DESC, application.id DESC LIMIT ? OFFSET ?`
	arguments = append(arguments, filter.Limit+1, offset)
	rows := make([]creditApplicationDirectoryRow, 0, filter.Limit+1)
	total, err := readCreditDirectoryPage(ctx, s.database, &rows, filteredQuery, filteredArguments, query, arguments)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	page := &CreditApplicationDirectoryPage{
		Total:        total,
		Applications: make([]CreditApplicationDirectoryItem, 0, len(rows)),
	}
	for _, row := range rows {
		item, projectErr := projectCreditApplicationDirectoryItem(row, management)
		if projectErr != nil {
			return nil, projectErr
		}
		page.Applications = append(page.Applications, item)
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor, err = s.cursor.encode(directoryCursorBoundary{
			SortTimeMicroseconds: last.CreatedAt.UnixMicro(), ID: last.ID,
		}, digest)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
		}
	}
	return page, nil
}

func (s *managementCreditDirectoryImpl) GetManagementCreditApplication(
	ctx context.Context,
	applicationNo string,
) (*CreditApplicationDirectoryDetail, error) {
	applicationNo = strings.TrimSpace(applicationNo)
	if _, err := s.authorizer.AuthorizeApplicationDetail(ctx, applicationNo); err != nil {
		return nil, err
	}
	if !validBusinessReference(applicationNo, 40) {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	result := &CreditApplicationDirectoryDetail{Reviews: []CreditApplicationReviewDirectoryItem{}}
	err := s.store.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, setErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); setErr != nil {
			return setErr
		}
		var row *creditApplicationDirectoryRow
		if loadErr := tx.GetScan(&row, `
SELECT application.id, application.application_no, application.customer_no,
       application.currency_code, application.policy_no, application.policy_version,
       application.requested_limit_minor, application.requested_valid_from,
       application.requested_valid_until, application.required_approval_count,
       application.status_code, application.created_at, application.updated_at,
       COALESCE(review.review_count, 0) AS review_count,
       COALESCE(account.credit_account_no, '') AS credit_account_no
FROM plugin_idc_core_credit_application application
LEFT JOIN LATERAL (
    SELECT COUNT(*)::INT AS review_count
    FROM plugin_idc_core_credit_application_review
    WHERE tenant_id = 0 AND application_no = application.application_no
) review ON TRUE
LEFT JOIN plugin_idc_core_credit_account account
  ON account.tenant_id = 0 AND account.origin_application_no = application.application_no
WHERE application.tenant_id = 0 AND application.application_no = ?`, applicationNo); loadErr != nil {
			return gerror.Wrap(loadErr, "load management credit application")
		}
		if row == nil {
			return bizerr.NewCode(CodeDirectoryNotAccessible)
		}
		projected, projectErr := projectCreditApplicationDirectoryItem(*row, true)
		if projectErr != nil {
			return projectErr
		}
		result.Application = projected
		rows := make([]creditApplicationReviewDirectoryRow, 0, maximumCreditReviewCount)
		if loadErr := tx.GetScan(&rows, `
SELECT review_no, review_sequence, decision_code, reviewer_id, created_at
FROM plugin_idc_core_credit_application_review
WHERE tenant_id = 0 AND application_no = ?
ORDER BY review_sequence ASC, id ASC
LIMIT ?`, applicationNo, maximumCreditReviewCount+1); loadErr != nil {
			return gerror.Wrap(loadErr, "load management credit application reviews")
		}
		if len(rows) > maximumCreditReviewCount {
			return bizerr.NewCode(CodeDirectoryIntegrity)
		}
		for _, review := range rows {
			if review.ReviewNo == "" || review.ReviewSequence < 1 ||
				review.ReviewSequence > maximumCreditReviewCount || review.ReviewerID < 1 ||
				review.CreatedAt.IsZero() {
				return bizerr.NewCode(CodeDirectoryIntegrity)
			}
			result.Reviews = append(result.Reviews, CreditApplicationReviewDirectoryItem{
				ReviewNo: review.ReviewNo, ReviewSequence: review.ReviewSequence,
				DecisionCode: review.DecisionCode, ReviewerID: review.ReviewerID,
				CreatedAt: review.CreatedAt.UTC(),
			})
		}
		return nil
	})
	if err != nil {
		return nil, creditDirectoryResultError(err)
	}
	return result, nil
}

func projectCreditApplicationDirectoryItem(
	row creditApplicationDirectoryRow,
	management bool,
) (CreditApplicationDirectoryItem, error) {
	if row.ID < 1 || !validBusinessReference(row.ApplicationNo, 40) ||
		!currencyPattern.MatchString(row.CurrencyCode) || row.PolicyVersion < 1 ||
		row.RequestedLimitMinor <= 0 || row.RequestedValidFrom.IsZero() ||
		!row.RequestedValidUntil.After(row.RequestedValidFrom) ||
		row.RequiredApprovalCount < 1 || row.RequiredApprovalCount > maximumCreditReviewCount ||
		row.ReviewCount < 0 || row.ReviewCount > row.RequiredApprovalCount ||
		row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() {
		return CreditApplicationDirectoryItem{}, bizerr.NewCode(CodeDirectoryIntegrity)
	}
	item := CreditApplicationDirectoryItem{
		ApplicationNo: row.ApplicationNo, CurrencyCode: row.CurrencyCode,
		PolicyNo: row.PolicyNo, PolicyVersion: row.PolicyVersion,
		RequestedLimitMinor:   row.RequestedLimitMinor,
		RequestedValidFrom:    row.RequestedValidFrom.UTC(),
		RequestedValidUntil:   row.RequestedValidUntil.UTC(),
		RequiredApprovalCount: row.RequiredApprovalCount, ReviewCount: row.ReviewCount,
		StatusCode: row.StatusCode, CreditAccountNo: row.CreditAccountNo,
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}
	if management {
		item.CustomerNo = row.CustomerNo
	}
	return item, nil
}
