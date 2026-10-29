// This file coordinates reviewed value-grant postings with the billing owner.

package marketing

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

const (
	managedGrantDefaultReason     = "manual_correction"
	managedGrantAdjustmentPost    = "marketing_grant_post"
	managedGrantAdjustmentReverse = "marketing_grant_reverse"
)

// ValueGrantFinancialRequestInput requests a reviewed posting or reversal.
// AmountMinor is ignored for posting and required for a partial reversal.
type ValueGrantFinancialRequestInput struct {
	RequestNo   string
	GrantNo     string
	AmountMinor int64
	ReasonCode  string
}

// ValueGrantFinancialReviewInput records an authorized staff decision.
type ValueGrantFinancialReviewInput struct {
	AdjustmentNo string
	Decision     string
	ReasonCode   string
}

// ManagedValueGrantFinancialMutation is the bounded action result shown to staff.
type ManagedValueGrantFinancialMutation struct {
	AdjustmentNo        string
	GrantNo             string
	AdjustmentType      string
	Status              string
	CustomerNo          string
	CurrencyCode        string
	AmountMinor         int64
	LedgerTransactionNo string
	Reused              bool
}

// ValueGrantFinancialAdjustmentSearchInput keeps the staff review queue bounded.
type ValueGrantFinancialAdjustmentSearchInput struct {
	Page   int
	Cursor string
	Limit  int
	Status string
}

// ManagedValueGrantFinancialAdjustment is one staff-visible review item.
type ManagedValueGrantFinancialAdjustment struct {
	ManagedValueGrantFinancialMutation
	RequestedBy int
	CreatedAt   time.Time
}

// ManagedValueGrantFinancialAdjustmentPage is a stable keyset page.
type ManagedValueGrantFinancialAdjustmentPage struct {
	Total       int
	Adjustments []ManagedValueGrantFinancialAdjustment
	NextCursor  string
}

type managedGrantAdjustmentRow struct {
	AdjustmentNo        string    `orm:"adjustment_no"`
	AdjustmentType      string    `orm:"adjustment_type_code"`
	Status              string    `orm:"status_code"`
	CustomerNo          string    `orm:"customer_no"`
	CurrencyCode        string    `orm:"currency_code"`
	AmountMinor         int64     `orm:"amount_minor"`
	GrantNo             string    `orm:"proof_reference"`
	RequestedBy         int       `orm:"requested_by"`
	CreatedAt           time.Time `orm:"created_at"`
	LedgerTransactionNo string    `orm:"ledger_transaction_no"`
}

// ListValueGrantAdjustments returns a review-only, bounded local queue.
func (s *managementActionServiceImpl) ListValueGrantAdjustments(
	ctx context.Context,
	input ValueGrantFinancialAdjustmentSearchInput,
) (*ManagedValueGrantFinancialAdjustmentPage, error) {
	if _, err := s.authorizer.AuthorizeGrantReview(ctx); err != nil {
		return nil, err
	}
	normalized, err := normalizeValueGrantFinancialAdjustmentSearch(input)
	if err != nil {
		return nil, err
	}
	page := &ManagedValueGrantFinancialAdjustmentPage{
		Adjustments: make([]ManagedValueGrantFinancialAdjustment, 0, normalized.Limit),
	}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, txErr := s.database.Exec(ctx, `SET TRANSACTION READ ONLY ISOLATION LEVEL REPEATABLE READ`); txErr != nil {
			return gerror.Wrap(txErr, "start marketing value-grant adjustment directory")
		}
		count, countErr := s.database.GetValue(ctx, `SELECT COUNT(*) FROM plugin_idc_core_billing_adjustment_request WHERE tenant_id=0 AND status_code=? AND adjustment_type_code IN ('marketing_grant_post','marketing_grant_reverse')`, normalized.Status)
		if countErr != nil {
			return gerror.Wrap(countErr, "count marketing value-grant adjustments")
		}
		page.Total = count.Int()
		var cursorCreatedAt time.Time
		if normalized.Cursor != "" {
			var cursorRows []struct {
				CreatedAt time.Time `orm:"created_at"`
			}
			if txErr := s.database.GetScan(ctx, &cursorRows, `
SELECT created_at
FROM plugin_idc_core_billing_adjustment_request
WHERE tenant_id = 0 AND adjustment_no = ? AND status_code = ?
  AND adjustment_type_code IN ('marketing_grant_post', 'marketing_grant_reverse')`,
				normalized.Cursor, normalized.Status); txErr != nil {
				return gerror.Wrap(txErr, "load marketing value-grant adjustment cursor")
			}
			if len(cursorRows) != 1 {
				return bizerr.NewCode(CodeManagementActionConflict)
			}
			cursorCreatedAt = cursorRows[0].CreatedAt
		}
		var rows []managedGrantAdjustmentRow
		if txErr := s.database.GetScan(ctx, &rows, `
SELECT adjustment_no, adjustment_type_code, status_code, customer_no,
       currency_code, amount_minor, proof_reference, requested_by, created_at,
       coalesce(ledger_transaction_no, '') AS ledger_transaction_no
FROM plugin_idc_core_billing_adjustment_request
WHERE tenant_id = 0 AND status_code = ?
  AND adjustment_type_code IN ('marketing_grant_post', 'marketing_grant_reverse')
  AND (? = '' OR (created_at, adjustment_no) < (?, ?))
ORDER BY created_at DESC, adjustment_no DESC
LIMIT ? OFFSET ?`, normalized.Status, normalized.Cursor, cursorCreatedAt, normalized.Cursor,
			normalized.Limit+1, max(normalized.Page-1, 0)*normalized.Limit); txErr != nil {
			return gerror.Wrap(txErr, "list marketing value-grant adjustments")
		}
		if len(rows) > normalized.Limit {
			page.NextCursor = rows[normalized.Limit-1].AdjustmentNo
			rows = rows[:normalized.Limit]
		}
		for _, row := range rows {
			page.Adjustments = append(page.Adjustments, ManagedValueGrantFinancialAdjustment{
				ManagedValueGrantFinancialMutation: ManagedValueGrantFinancialMutation{
					AdjustmentNo: row.AdjustmentNo, GrantNo: row.GrantNo,
					AdjustmentType: row.AdjustmentType, Status: row.Status,
					CustomerNo: row.CustomerNo, CurrencyCode: row.CurrencyCode,
					AmountMinor: row.AmountMinor, LedgerTransactionNo: row.LedgerTransactionNo,
				},
				RequestedBy: row.RequestedBy, CreatedAt: row.CreatedAt,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return page, nil
}

// RequestValueGrantPosting creates a permission-authorized financial action.
func (s *managementActionServiceImpl) RequestValueGrantPosting(
	ctx context.Context,
	input ValueGrantFinancialRequestInput,
) (*ManagedValueGrantFinancialMutation, error) {
	actorUserID, err := s.authorizer.AuthorizeGrantPost(ctx)
	if err != nil {
		return nil, err
	}
	return s.requestValueGrantFinancialAction(
		ctx, input, actorUserID, managedGrantAdjustmentPost,
	)
}

// RequestValueGrantReversal creates a permission-authorized financial action.
func (s *managementActionServiceImpl) RequestValueGrantReversal(
	ctx context.Context,
	input ValueGrantFinancialRequestInput,
) (*ManagedValueGrantFinancialMutation, error) {
	actorUserID, err := s.authorizer.AuthorizeGrantReverse(ctx)
	if err != nil {
		return nil, err
	}
	return s.requestValueGrantFinancialAction(
		ctx, input, actorUserID, managedGrantAdjustmentReverse,
	)
}

func (s *managementActionServiceImpl) requestValueGrantFinancialAction(
	ctx context.Context,
	input ValueGrantFinancialRequestInput,
	actorUserID int,
	adjustmentType string,
) (*ManagedValueGrantFinancialMutation, error) {
	normalized, err := normalizeValueGrantFinancialRequest(input, adjustmentType)
	if err != nil || actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementActionInvalid)
	}
	var result *ManagedValueGrantFinancialMutation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		grant, loadErr := loadManagedValueGrant(ctx, s.database, normalized.GrantNo)
		if loadErr != nil {
			return loadErr
		}
		if grant == nil {
			return bizerr.NewCode(CodeManagementActionConflict)
		}
		amountMinor := normalized.AmountMinor
		if adjustmentType == managedGrantAdjustmentPost {
			if grant.Status == "cancelled" || grant.OriginalAmountMinor <= 0 {
				return bizerr.NewCode(CodeManagementActionConflict)
			}
			amountMinor = grant.OriginalAmountMinor
		} else if grant.Status == "pending" || grant.Status == "cancelled" ||
			grant.FinancialPostingReference == nil {
			return bizerr.NewCode(CodeManagementActionConflict)
		}
		evidenceDigest := managedGrantEvidenceDigest(
			adjustmentType, normalized.RequestNo, grant.GrantNo, grant.CustomerNo,
			grant.CurrencyCode, amountMinor, normalized.ReasonCode,
		)
		financialInput := billing.MarketingGrantAdjustmentInput{
			GrantNo: grant.GrantNo, CustomerNo: grant.CustomerNo,
			CurrencyCode: grant.CurrencyCode, AmountMinor: amountMinor,
			ReasonCode: normalized.ReasonCode, EvidenceDigest: evidenceDigest,
			IdempotencyKey: normalized.RequestNo, RequestedBy: actorUserID,
		}
		var adjustment *billing.FinancialAdjustment
		if adjustmentType == managedGrantAdjustmentPost {
			adjustment, loadErr = s.financial.CreateMarketingGrantPosting(ctx, financialInput)
		} else {
			adjustment, loadErr = s.financial.CreateMarketingGrantReversal(ctx, financialInput)
		}
		if loadErr != nil {
			return loadErr
		}
		// Replays return the already committed action without generating a new
		// audit transition against the now-changed entitlement projection.
		if adjustment.Reused {
			result = projectManagedGrantFinancialMutation(adjustment, grant.GrantNo)
			return nil
		}
		if adjustment.Status == billing.AdjustmentStatusPosted {
			if adjustmentType == managedGrantAdjustmentPost {
				_, loadErr = s.valueGrants.Post(ctx, ValueGrantPostInput{
					GrantNo: grant.GrantNo, CustomerNo: grant.CustomerNo,
					FinancialPostingReference: adjustment.LedgerTransactionNo,
				})
			} else {
				_, loadErr = s.valueGrants.Reverse(ctx, ValueGrantMutationInput{
					GrantNo: grant.GrantNo, CustomerNo: grant.CustomerNo,
					SourceType: "financial", SourceReference: adjustment.LedgerTransactionNo,
					AmountMinor: amountMinor,
				})
			}
			if loadErr != nil {
				return loadErr
			}
		}
		result = projectManagedGrantFinancialMutation(adjustment, grant.GrantNo)
		return s.appendValueGrantFinancialAudit(
			ctx, "request", normalized.RequestNo, normalized.ReasonCode,
			actorUserID, result, map[string]any{"state": grant.Status, "pending_amount_minor": grant.PendingAmountMinor, "available_amount_minor": grant.AvailableAmountMinor},
		)
	})
	return managementActionResult(result, err)
}

// ReviewValueGrantAdjustment applies one authorized decision. An approval
// posts the ledger first and updates the marketing entitlement in the same
// outer transaction; either both facts commit or neither does.
func (s *managementActionServiceImpl) ReviewValueGrantAdjustment(
	ctx context.Context,
	input ValueGrantFinancialReviewInput,
) (*ManagedValueGrantFinancialMutation, error) {
	actorUserID, err := s.authorizer.AuthorizeGrantReview(ctx)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeValueGrantFinancialReview(input)
	if err != nil || actorUserID <= 0 {
		return nil, bizerr.NewCode(CodeManagementActionInvalid)
	}
	var result *ManagedValueGrantFinancialMutation
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		request, loadErr := loadManagedGrantAdjustment(ctx, s.database, normalized.AdjustmentNo)
		if loadErr != nil {
			return loadErr
		}
		if request == nil {
			return bizerr.NewCode(CodeManagementActionConflict)
		}
		decision := billing.AdjustmentDecisionApprove
		if normalized.Decision == "reject" {
			decision = billing.AdjustmentDecisionReject
		}
		reviewDigest := managedGrantEvidenceDigest(
			"review:"+normalized.Decision, normalized.AdjustmentNo, request.GrantNo,
			request.CustomerNo, request.CurrencyCode, request.AmountMinor,
			normalized.ReasonCode,
		)
		adjustment, reviewErr := s.financial.ReviewAdjustment(ctx, billing.AdjustmentReviewInput{
			AdjustmentNo: normalized.AdjustmentNo, Decision: decision,
			ReviewerID: actorUserID, CommentDigest: reviewDigest,
			EvidenceDigest: reviewDigest,
		})
		if reviewErr != nil {
			return reviewErr
		}
		if adjustment.Status == billing.AdjustmentStatusPosted {
			grant, grantErr := loadValueGrant(
				ctx, s.database, request.GrantNo, request.CustomerNo, true,
			)
			if grantErr != nil {
				return grantErr
			}
			if grant == nil || grant.CurrencyCode != request.CurrencyCode {
				return bizerr.NewCode(CodeManagementActionConflict)
			}
			if request.AdjustmentType == managedGrantAdjustmentPost {
				_, grantErr = s.valueGrants.Post(ctx, ValueGrantPostInput{
					GrantNo: request.GrantNo, CustomerNo: request.CustomerNo,
					FinancialPostingReference: adjustment.LedgerTransactionNo,
				})
			} else {
				_, grantErr = s.valueGrants.Reverse(ctx, ValueGrantMutationInput{
					GrantNo: request.GrantNo, CustomerNo: request.CustomerNo,
					SourceType: "financial", SourceReference: adjustment.LedgerTransactionNo,
					AmountMinor: request.AmountMinor,
				})
			}
			if grantErr != nil {
				return grantErr
			}
		}
		result = projectManagedGrantFinancialMutation(adjustment, request.GrantNo)
		return s.appendValueGrantFinancialAudit(
			ctx, "review", normalized.AdjustmentNo, normalized.ReasonCode,
			actorUserID, result, map[string]any{"state": request.Status},
		)
	})
	return managementActionResult(result, err)
}

func loadManagedGrantAdjustment(
	ctx context.Context,
	database gdb.DB,
	adjustmentNo string,
) (*managedGrantAdjustmentRow, error) {
	var rows []managedGrantAdjustmentRow
	err := database.GetScan(ctx, &rows, `
SELECT adjustment_no, adjustment_type_code, status_code, customer_no,
       currency_code, amount_minor, proof_reference, requested_by, created_at,
       coalesce(ledger_transaction_no, '') AS ledger_transaction_no
FROM plugin_idc_core_billing_adjustment_request
WHERE tenant_id = 0 AND adjustment_no = ?
  AND adjustment_type_code IN ('marketing_grant_post', 'marketing_grant_reverse')
FOR UPDATE`, adjustmentNo)
	if err != nil {
		return nil, gerror.Wrap(err, "load marketing value-grant adjustment")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, gerror.New("marketing value-grant adjustment identity is ambiguous")
	}
	return &rows[0], nil
}

func normalizeValueGrantFinancialAdjustmentSearch(
	input ValueGrantFinancialAdjustmentSearchInput,
) (ValueGrantFinancialAdjustmentSearchInput, error) {
	input.Cursor = strings.TrimSpace(input.Cursor)
	input.Status = strings.TrimSpace(input.Status)
	if input.Status == "" {
		input.Status = string(billing.AdjustmentStatusPendingReview)
	}
	if input.Limit == 0 {
		input.Limit = 25
	}
	if (input.Cursor != "" && !identifierPattern.MatchString(input.Cursor)) ||
		(input.Status != string(billing.AdjustmentStatusPendingReview) &&
			input.Status != string(billing.AdjustmentStatusRejected) &&
			input.Status != string(billing.AdjustmentStatusPosted)) ||
		input.Limit < 1 || input.Limit > 100 || !validMarketingNumberedPage(input.Page, input.Limit, input.Cursor) {
		return ValueGrantFinancialAdjustmentSearchInput{}, bizerr.NewCode(CodeManagementActionInvalid)
	}
	return input, nil
}

func loadManagedValueGrant(
	ctx context.Context,
	database gdb.DB,
	grantNo string,
) (*valueGrantRow, error) {
	var rows []valueGrantRow
	err := database.GetScan(ctx, &rows, `
SELECT grant_no, grant_type_code, campaign_no, benefit_no,
       benefit_snapshot_digest, customer_no, currency_code, source_reference,
       original_amount_minor, pending_amount_minor, available_amount_minor,
       frozen_amount_minor, consumed_amount_minor, expired_amount_minor,
       reversed_amount_minor, withdrawable, global_quota_reserved,
       customer_quota_reserved, expires_at, status_code,
       financial_posting_reference
FROM plugin_idc_core_marketing_value_grant
WHERE tenant_id = 0 AND grant_no = ?
FOR UPDATE`, grantNo)
	if err != nil {
		return nil, gerror.Wrap(err, "load managed marketing value grant")
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, gerror.New("managed marketing value-grant identity is ambiguous")
	}
	return &rows[0], nil
}

func normalizeValueGrantFinancialRequest(
	input ValueGrantFinancialRequestInput,
	adjustmentType string,
) (ValueGrantFinancialRequestInput, error) {
	input.RequestNo = strings.TrimSpace(input.RequestNo)
	input.GrantNo = strings.TrimSpace(input.GrantNo)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	if input.ReasonCode == "" {
		input.ReasonCode = managedGrantDefaultReason
	}
	if !validBusinessReference(input.RequestNo) || !identifierPattern.MatchString(input.GrantNo) ||
		!validManagementReason(input.ReasonCode) ||
		(adjustmentType == managedGrantAdjustmentPost && input.AmountMinor != 0) ||
		(adjustmentType == managedGrantAdjustmentReverse &&
			(input.AmountMinor < 1 || input.AmountMinor > maximumValueGrantAmountMinor)) {
		return ValueGrantFinancialRequestInput{}, bizerr.NewCode(CodeManagementActionInvalid)
	}
	return input, nil
}

func normalizeValueGrantFinancialReview(
	input ValueGrantFinancialReviewInput,
) (ValueGrantFinancialReviewInput, error) {
	input.AdjustmentNo = strings.TrimSpace(input.AdjustmentNo)
	input.Decision = strings.TrimSpace(input.Decision)
	input.ReasonCode = strings.TrimSpace(input.ReasonCode)
	if input.ReasonCode == "" {
		input.ReasonCode = managedGrantDefaultReason
	}
	if !identifierPattern.MatchString(input.AdjustmentNo) ||
		(input.Decision != "approve" && input.Decision != "reject") ||
		!validManagementReason(input.ReasonCode) {
		return ValueGrantFinancialReviewInput{}, bizerr.NewCode(CodeManagementActionInvalid)
	}
	return input, nil
}

func managedGrantEvidenceDigest(
	action string,
	requestNo string,
	grantNo string,
	customerNo string,
	currencyCode string,
	amountMinor int64,
	reasonCode string,
) string {
	payload := strings.Join([]string{
		"marketing-value-financial-v1", action, requestNo, grantNo,
		customerNo, currencyCode, reasonCode,
		strconv.FormatInt(amountMinor, 10),
	}, "\x00")
	digest := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(digest[:])
}

func projectManagedGrantFinancialMutation(
	adjustment *billing.FinancialAdjustment,
	grantNo string,
) *ManagedValueGrantFinancialMutation {
	if adjustment == nil {
		return nil
	}
	return &ManagedValueGrantFinancialMutation{
		AdjustmentNo: adjustment.AdjustmentNo, GrantNo: grantNo,
		AdjustmentType: adjustment.AdjustmentType, Status: string(adjustment.Status),
		CustomerNo: adjustment.CustomerNo, CurrencyCode: adjustment.CurrencyCode,
		AmountMinor:         adjustment.AmountMinor,
		LedgerTransactionNo: adjustment.LedgerTransactionNo, Reused: adjustment.Reused,
	}
}

func (s *managementActionServiceImpl) appendValueGrantFinancialAudit(
	ctx context.Context,
	operation string,
	idempotencyKey string,
	reasonCode string,
	actorUserID int,
	result *ManagedValueGrantFinancialMutation,
	before map[string]any,
) error {
	if result == nil || actorUserID <= 0 {
		return bizerr.NewCode(CodeManagementActionConflict)
	}
	permission := permissionMarketingGrantReview
	if operation == "request" && result.AdjustmentType == managedGrantAdjustmentPost {
		permission = permissionMarketingGrantPost
	} else if operation == "request" {
		permission = permissionMarketingGrantReverse
	}
	return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "marketing.grant.financial." + operation,
		IdempotencyKey:   idempotencyKey, OperationCode: "marketing.grant.financial." + operation,
		TargetType: "marketing_value_grant", TargetNo: result.GrantNo,
		CustomerNo: result.CustomerNo, ResultCode: "succeeded", ReasonCode: reasonCode,
		RiskCode: "high", RequiredPermission: permission,
		CorrelationNo:  result.AdjustmentNo,
		BeforeSnapshot: before,
		AfterSnapshot: map[string]any{
			"state": result.Status, "adjustment_no": result.AdjustmentNo,
			"adjustment_type": result.AdjustmentType,
			"currency_code":   result.CurrencyCode, "amount_minor": result.AmountMinor,
		},
	})
}
