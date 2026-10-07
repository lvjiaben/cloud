package billing

import (
	"context"
	"strings"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customerprojection"
)

type managementAdjustmentRow struct {
	ID                  int64     `orm:"id"`
	AdjustmentNo        string    `orm:"adjustment_no"`
	AdjustmentTypeCode  string    `orm:"adjustment_type_code"`
	StatusCode          string    `orm:"status_code"`
	CustomerNo          string    `orm:"customer_no"`
	InvoiceNo           *string   `orm:"invoice_no"`
	CurrencyCode        string    `orm:"currency_code"`
	AmountMinor         int64     `orm:"amount_minor"`
	ReasonCode          string    `orm:"reason_code"`
	ApprovalRequired    bool      `orm:"approval_required"`
	RequestedBy         int       `orm:"requested_by"`
	LedgerTransactionNo *string   `orm:"ledger_transaction_no"`
	CreatedAt           time.Time `orm:"created_at"`
	UpdatedAt           time.Time `orm:"updated_at"`
}

type managementAdjustmentDetailRow struct {
	ID                        int64      `orm:"id"`
	AdjustmentNo              string     `orm:"adjustment_no"`
	AdjustmentTypeCode        string     `orm:"adjustment_type_code"`
	StatusCode                string     `orm:"status_code"`
	CustomerNo                string     `orm:"customer_no"`
	InvoiceNo                 *string    `orm:"invoice_no"`
	CurrencyCode              string     `orm:"currency_code"`
	AmountMinor               int64      `orm:"amount_minor"`
	ReasonCode                string     `orm:"reason_code"`
	ApprovalRequired          bool       `orm:"approval_required"`
	RequestedBy               int        `orm:"requested_by"`
	LedgerTransactionNo       *string    `orm:"ledger_transaction_no"`
	CreatedAt                 time.Time  `orm:"created_at"`
	UpdatedAt                 time.Time  `orm:"updated_at"`
	EvidenceDigest            string     `orm:"evidence_digest"`
	ProofReference            string     `orm:"proof_reference"`
	ReceivingAccountReference string     `orm:"receiving_account_reference"`
	BusinessPurposeCode       string     `orm:"business_purpose_code"`
	PolicyNo                  string     `orm:"policy_no"`
	PolicyVersion             int64      `orm:"policy_version"`
	Version                   int64      `orm:"version"`
	ReviewerID                *int       `orm:"reviewer_id"`
	DecisionCode              *string    `orm:"decision_code"`
	CommentDigest             *string    `orm:"comment_digest"`
	ReviewEvidenceDigest      *string    `orm:"review_evidence_digest"`
	DecidedAt                 *time.Time `orm:"decided_at"`
	PostedAt                  *time.Time `orm:"posted_at"`
}

type ManagementAdjustmentDetail struct {
	ManagementAdjustmentDirectoryItem
	EvidenceDigest            string
	ProofReference            string
	ReceivingAccountReference string
	BusinessPurposeCode       string
	PolicyNo                  string
	PolicyVersion             int64
	Version                   int64
	ReviewerID                int
	DecisionCode              string
	CommentDigest             string
	ReviewEvidenceDigest      string
	DecidedAt                 *time.Time
	PostedAt                  *time.Time
}

func (s *managementFinanceDirectoryImpl) ListManagementAdjustments(
	ctx context.Context,
	filter ManagementAdjustmentDirectoryFilter,
) (*ManagementAdjustmentDirectoryPage, error) {
	if _, err := s.authorizer.AuthorizeAdjustmentList(ctx); err != nil {
		return nil, err
	}
	var err error
	filter.CustomerNo, err = normalizeManagementReference(filter.CustomerNo)
	if err != nil {
		return nil, err
	}
	filter.AdjustmentTypeCode, err = normalizeManagementType(filter.AdjustmentTypeCode)
	if err != nil {
		return nil, err
	}
	filter.StatusCode, err = normalizeDirectoryStatus(filter.StatusCode)
	if err != nil {
		return nil, err
	}
	filter.CurrencyCode, err = normalizeDirectoryCurrency(filter.CurrencyCode)
	if err != nil {
		return nil, err
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	filter.Limit, err = normalizeDirectoryLimit(filter.Limit, defaultAccountInvoiceLimit, maximumManagementDirectoryLimit)
	if err != nil || len(filter.Cursor) > maximumDirectoryCursorLength || !validManagementPage(filter.Page, filter.Limit, filter.Cursor) {
		return nil, bizerr.NewCode(CodeDirectoryInvalid)
	}
	digest, err := directoryFilterDigest(struct {
		Kind, CustomerNo, AdjustmentTypeCode, StatusCode, CurrencyCode string
		Limit                                                          int
	}{"management_adjustment", filter.CustomerNo, filter.AdjustmentTypeCode,
		filter.StatusCode, filter.CurrencyCode, filter.Limit})
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
SELECT id, adjustment_no, adjustment_type_code, status_code, customer_no,
       invoice_no, currency_code, amount_minor, reason_code, approval_required,
       requested_by, ledger_transaction_no, created_at, updated_at
FROM plugin_idc_core_billing_adjustment_request
WHERE tenant_id = 0`
	arguments := make([]any, 0, 10)
	if filter.CustomerNo != "" {
		query += ` AND customer_no = ?`
		arguments = append(arguments, filter.CustomerNo)
	}
	if filter.AdjustmentTypeCode != "" {
		query += ` AND adjustment_type_code = ?`
		arguments = append(arguments, filter.AdjustmentTypeCode)
	}
	if filter.StatusCode != "" {
		query += ` AND status_code = ?`
		arguments = append(arguments, filter.StatusCode)
	}
	if filter.CurrencyCode != "" {
		query += ` AND currency_code = ?`
		arguments = append(arguments, filter.CurrencyCode)
	}
	countQuery, countArgs := "SELECT COUNT(*) FROM ("+query+") eligible", append([]any(nil), arguments...)
	if boundary != nil {
		boundaryTime := time.UnixMicro(boundary.SortTimeMicroseconds)
		query += ` AND ((created_at < ?) OR (created_at = ? AND id < ?))`
		arguments = append(arguments, boundaryTime, boundaryTime, boundary.ID)
	}
	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	arguments = append(arguments, filter.Limit+1, max(filter.Page-1, 0)*filter.Limit)
	rows := make([]managementAdjustmentRow, 0, filter.Limit+1)
	total, err := s.readManagementDirectoryPage(ctx, &rows, countQuery, countArgs, query, arguments)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	page := &ManagementAdjustmentDirectoryPage{Adjustments: make([]ManagementAdjustmentDirectoryItem, 0, len(rows)), Total: total}
	for _, row := range rows {
		item, projectErr := projectManagementAdjustment(row)
		if projectErr != nil {
			return nil, projectErr
		}
		page.Adjustments = append(page.Adjustments, item)
	}
	customerNos := make([]string, 0, len(page.Adjustments))
	for _, item := range page.Adjustments {
		customerNos = append(customerNos, item.CustomerNo)
	}
	customers, projectionErr := customerprojection.Load(ctx, s.database, customerNos)
	if projectionErr != nil {
		return nil, bizerr.WrapCode(projectionErr, CodeDirectoryUnavailable)
	}
	for index := range page.Adjustments {
		customer := customers[page.Adjustments[index].CustomerNo]
		page.Adjustments[index].CustomerCode = customer.CustomerCode
		page.Adjustments[index].CustomerDisplayName = customer.DisplayName
		page.Adjustments[index].CustomerPrimaryPhone = customer.PrimaryPhone
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

func (s *managementFinanceDirectoryImpl) GetManagementAdjustment(
	ctx context.Context,
	adjustmentNo string,
) (*ManagementAdjustmentDetail, error) {
	adjustmentNo = strings.TrimSpace(adjustmentNo)
	if _, err := s.authorizer.AuthorizeAdjustmentDetail(ctx, adjustmentNo); err != nil {
		return nil, err
	}
	var row *managementAdjustmentDetailRow
	if err := s.database.GetScan(ctx, &row, `
SELECT request.id, request.adjustment_no, request.adjustment_type_code,
       request.status_code, request.customer_no, request.invoice_no,
       request.currency_code, request.amount_minor, request.reason_code,
       request.approval_required, request.requested_by,
       request.ledger_transaction_no, request.created_at, request.updated_at,
       request.evidence_digest, COALESCE(request.proof_reference, '') AS proof_reference,
       COALESCE(request.receiving_account_reference, '') AS receiving_account_reference,
       COALESCE(request.business_purpose_code, '') AS business_purpose_code,
       COALESCE(request.policy_no, '') AS policy_no,
       COALESCE(request.policy_version, 0) AS policy_version,
       request.version, approval.reviewer_id, approval.decision_code,
       approval.comment_digest, approval.evidence_digest AS review_evidence_digest,
       approval.decided_at, request.posted_at
FROM plugin_idc_core_billing_adjustment_request request
LEFT JOIN plugin_idc_core_billing_adjustment_approval approval
  ON approval.tenant_id = request.tenant_id AND approval.adjustment_no = request.adjustment_no
WHERE request.tenant_id = 0 AND request.adjustment_no = ?`, adjustmentNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeDirectoryUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	item, err := projectManagementAdjustment(managementAdjustmentRow{
		ID: row.ID, AdjustmentNo: row.AdjustmentNo,
		AdjustmentTypeCode: row.AdjustmentTypeCode, StatusCode: row.StatusCode,
		CustomerNo: row.CustomerNo, InvoiceNo: row.InvoiceNo,
		CurrencyCode: row.CurrencyCode, AmountMinor: row.AmountMinor,
		ReasonCode: row.ReasonCode, ApprovalRequired: row.ApprovalRequired,
		RequestedBy: row.RequestedBy, LedgerTransactionNo: row.LedgerTransactionNo,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	})
	if err != nil {
		return nil, err
	}
	detail := &ManagementAdjustmentDetail{
		ManagementAdjustmentDirectoryItem: item,
		EvidenceDigest:                    row.EvidenceDigest, ProofReference: row.ProofReference,
		ReceivingAccountReference: row.ReceivingAccountReference,
		BusinessPurposeCode:       row.BusinessPurposeCode, PolicyNo: row.PolicyNo,
		PolicyVersion: row.PolicyVersion, Version: row.Version,
		CommentDigest:        optionalDirectoryString(row.CommentDigest),
		ReviewEvidenceDigest: optionalDirectoryString(row.ReviewEvidenceDigest),
		DecidedAt:            row.DecidedAt, PostedAt: row.PostedAt,
	}
	if row.ReviewerID != nil {
		detail.ReviewerID = *row.ReviewerID
	}
	detail.DecisionCode = optionalDirectoryString(row.DecisionCode)
	customers, projectionErr := customerprojection.Load(ctx, s.database, []string{detail.CustomerNo})
	if projectionErr != nil {
		return nil, bizerr.WrapCode(projectionErr, CodeDirectoryUnavailable)
	}
	customer := customers[detail.CustomerNo]
	detail.CustomerCode = customer.CustomerCode
	detail.CustomerDisplayName = customer.DisplayName
	detail.CustomerPrimaryPhone = customer.PrimaryPhone
	return detail, nil
}

func projectManagementAdjustment(row managementAdjustmentRow) (ManagementAdjustmentDirectoryItem, error) {
	if row.ID < 1 || row.CreatedAt.IsZero() || row.UpdatedAt.IsZero() ||
		row.AmountMinor <= 0 || row.RequestedBy <= 0 || !canonicalDigestSafeStatus(row.StatusCode) {
		return ManagementAdjustmentDirectoryItem{}, bizerr.NewCode(CodeDirectoryIntegrity)
	}
	return ManagementAdjustmentDirectoryItem{
		AdjustmentNo: row.AdjustmentNo, AdjustmentTypeCode: row.AdjustmentTypeCode,
		StatusCode: row.StatusCode, CustomerNo: row.CustomerNo,
		InvoiceNo: optionalDirectoryString(row.InvoiceNo), CurrencyCode: row.CurrencyCode,
		AmountMinor: row.AmountMinor, ReasonCode: row.ReasonCode,
		ApprovalRequired: row.ApprovalRequired, RequestedBy: row.RequestedBy,
		LedgerTransactionNo: optionalDirectoryString(row.LedgerTransactionNo),
		CreatedAt:           row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

func canonicalDigestSafeStatus(value string) bool {
	return directoryStatusPattern.MatchString(value)
}
