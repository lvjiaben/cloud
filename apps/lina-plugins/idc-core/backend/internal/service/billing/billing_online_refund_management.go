// This file implements staff refund review, high-amount proof, and immutable policy updates.

package billing

import (
	"context"
	"crypto/rand"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

// RefundReviewDecision is the fixed staff approval or rejection action.
type RefundReviewDecision string

// Staff review decision constants form the closed approval vocabulary.
const (
	RefundReviewApprove RefundReviewDecision = "approve"
	RefundReviewReject  RefundReviewDecision = "reject"
)

// ManagementRefundReviewIntent binds a staff decision to one exact refund version and evidence.
type ManagementRefundReviewIntent struct {
	RequestNo       string
	RefundNo        string
	ExpectedVersion int64
	Decision        RefundReviewDecision
	CommentDigest   string
	EvidenceDigest  string
}

// ManagementRefundService owns staff refund reading, review proofs, decisions, and policy versions.
type ManagementRefundService interface {
	// RefundFailedFulfillment records a staff-selected refund for a proven failed, unfulfilled item.
	RefundFailedFulfillment(context.Context, FailedFulfillmentRefundInput) (*OnlineRefund, error)
	// ListRefunds returns one bounded newest-first staff directory after exact list authorization.
	ListRefunds(context.Context, RefundDirectoryFilter) (*RefundDirectoryPage, error)
	// GetRefundDetail returns one exact request and its immutable item and service-impact snapshots.
	GetRefundDetail(context.Context, string) (*RefundDirectoryDetail, error)
	// IssueRefundReviewProof issues a single-use proof for one exact high-amount approval intent.
	IssueRefundReviewProof(context.Context, ManagementRefundReviewIntent) (*reauth.ProofContext, error)
	// ReviewRefund approves or rejects one exact pending version with immutable evidence.
	ReviewRefund(context.Context, ManagementRefundReviewIntent, string) (*OnlineRefund, error)
	// ConfigureRefundPolicy appends one currency-local policy version after optimistic concurrency checks.
	ConfigureRefundPolicy(context.Context, RefundPolicyInput, int64) (*RefundPolicy, error)
}

type managementRefundService struct {
	database   gdb.DB
	authorizer ManagementRefundAuthorizer
	recentAuth reauth.Service
	audit      idccap.BusinessAuditWriter
	random     io.Reader
	commands   *onlineRefundService
}

var _ ManagementRefundService = (*managementRefundService)(nil)

func NewManagementRefundService(
	database gdb.DB,
	authorizer ManagementRefundAuthorizer,
	recentAuth reauth.Service,
	auditWriter idccap.BusinessAuditWriter,
	creditRefunds CreditUsageService,
) (ManagementRefundService, error) {
	for _, dependency := range []any{database, authorizer, recentAuth, auditWriter, creditRefunds} {
		if isNilFinanceDirectoryDependency(dependency) {
			return nil, gerror.New("management refunds require database, authorization, and recent authentication")
		}
	}
	return &managementRefundService{
		database: database, authorizer: authorizer, recentAuth: recentAuth,
		audit: auditWriter, random: rand.Reader,
		commands: &onlineRefundService{database: database, audit: auditWriter, credit: creditRefunds, random: rand.Reader},
	}, nil
}

func (s *managementRefundService) IssueRefundReviewProof(
	ctx context.Context,
	intent ManagementRefundReviewIntent,
) (*reauth.ProofContext, error) {
	actor, normalized, request, binding, _, err := s.prepareRefundReview(ctx, intent, false)
	if err != nil {
		return nil, err
	}
	if actor < 1 || normalized.Decision != RefundReviewApprove || !request.HighAmount {
		return nil, bizerr.NewCode(CodeRefundInvalid)
	}
	return s.recentAuth.Issue(ctx, binding)
}

func (s *managementRefundService) ReviewRefund(
	ctx context.Context,
	intent ManagementRefundReviewIntent,
	proofToken string,
) (*OnlineRefund, error) {
	actor, normalized, request, binding, decisionEvidence, err := s.prepareRefundReview(ctx, intent, true)
	if err != nil {
		return nil, err
	}
	if replayed, replayErr := s.loadRefundDecisionReplay(ctx, request, normalized, decisionEvidence); replayErr != nil {
		return nil, normalizeOnlineRefundError(replayErr)
	} else if replayed != nil {
		return replayed, nil
	}
	proofToken = strings.TrimSpace(proofToken)
	if proofToken != "" {
		proof, consumeErr := s.recentAuth.Consume(ctx, proofToken, binding)
		if consumeErr != nil {
			return nil, consumeErr
		}
		if proof == nil || proof.UserID != actor || proof.TenantID != idccap.DefaultTenantID ||
			proof.Purpose != binding.Purpose || proof.TargetDigest != binding.TargetDigest {
			return nil, bizerr.NewCode(CodeRefundNotAccessible)
		}
	}

	var result *OnlineRefund
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		locked, loadErr := s.loadManagementRefund(ctx, normalized.RefundNo, true)
		if loadErr != nil {
			return loadErr
		}
		if locked == nil {
			return bizerr.NewCode(CodeRefundNotAccessible)
		}
		if locked.Status != "pending_review" || locked.Version != normalized.ExpectedVersion ||
			locked.ImpactDigest != request.ImpactDigest || locked.RequestDigest != request.RequestDigest {
			return bizerr.NewCode(CodeRefundConflict)
		}
		now, nowErr := refundDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		if normalized.Decision == RefundReviewApprove {
			if updateErr := s.approveRefund(ctx, locked, actor, normalized, decisionEvidence, now); updateErr != nil {
				return updateErr
			}
			locked.Status = "approved"
			locked.Version++
			locked.ApprovedAt = &now
		} else {
			if updateErr := s.rejectRefund(ctx, locked, actor, normalized, decisionEvidence, now); updateErr != nil {
				return updateErr
			}
			locked.Status = "rejected"
			locked.Version++
			locked.CompletedAt = &now
		}
		result = locked.toOnlineRefund()
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "billing.refund.review", IdempotencyKey: normalized.RequestNo,
			OperationCode: "billing.refund.review", TargetType: "refund_request",
			TargetNo: locked.RefundNo, CustomerNo: locked.CustomerNo,
			ResultCode: "succeeded", ReasonCode: "refund_" + string(normalized.Decision),
			RiskCode: "critical", RequiredPermission: permissionBillingRefundReview,
			CorrelationNo:  locked.OrderNo,
			BeforeSnapshot: map[string]any{"state": "pending_review"},
			AfterSnapshot:  refundAuditSnapshot(result),
		})
	})
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	return result, nil
}

func (s *managementRefundService) ConfigureRefundPolicy(
	ctx context.Context,
	input RefundPolicyInput,
	expectedVersion int64,
) (*RefundPolicy, error) {
	actor, err := s.authorizer.AuthorizeRefundPolicyWrite(ctx)
	if err != nil {
		return nil, err
	}
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	if !currencyPattern.MatchString(input.CurrencyCode) || input.HighAmountThresholdMinor <= 0 ||
		expectedVersion < 0 || (input.ConfiguredBy != 0 && input.ConfiguredBy != actor) {
		return nil, bizerr.NewCode(CodeRefundInvalid)
	}
	input.ConfiguredBy = actor
	var result *RefundPolicy
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var current *refundPolicyRow
		if loadErr := s.database.GetScan(ctx, &current, `
SELECT policy_no,currency_code,policy_version,high_amount_threshold_minor,
       automatic_approval_enabled
FROM plugin_idc_core_refund_policy_version
WHERE tenant_id=0 AND currency_code=?
ORDER BY policy_version DESC LIMIT 1 FOR UPDATE`, input.CurrencyCode); loadErr != nil {
			return loadErr
		}
		currentVersion := int64(0)
		if current != nil {
			currentVersion = current.PolicyVersion
		}
		if currentVersion != expectedVersion {
			return bizerr.NewCode(CodeRefundConflict)
		}
		now, nowErr := refundDatabaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		policyNo, numberErr := newBillingNumber(s.random, "rfp_")
		if numberErr != nil {
			return numberErr
		}
		version := currentVersion + 1
		digest, digestErr := contentDigest(struct {
			CurrencyCode             string `json:"currencyCode"`
			Version                  int64  `json:"version"`
			HighAmountThresholdMinor int64  `json:"highAmountThresholdMinor"`
			AutomaticApprovalEnabled bool   `json:"automaticApprovalEnabled"`
			ConfiguredBy             int    `json:"configuredBy"`
		}{input.CurrencyCode, version, input.HighAmountThresholdMinor,
			input.AutomaticApprovalEnabled, actor})
		if digestErr != nil {
			return digestErr
		}
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_refund_policy_version
    (tenant_id,policy_no,currency_code,policy_version,high_amount_threshold_minor,
     automatic_approval_enabled,content_digest,configured_by,effective_at,created_at)
VALUES (0,?,?,?,?,?,?,?,?,?)`, policyNo, input.CurrencyCode, version,
			input.HighAmountThresholdMinor, input.AutomaticApprovalEnabled,
			digest, actor, now, now); insertErr != nil {
			return insertErr
		}
		result = &RefundPolicy{
			PolicyNo: policyNo, CurrencyCode: input.CurrencyCode, Version: version,
			HighAmountThresholdMinor: input.HighAmountThresholdMinor,
			AutomaticApprovalEnabled: input.AutomaticApprovalEnabled,
			ConfiguredBy:             actor, EffectiveAt: now,
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{BeforeSnapshot: map[string]any{"policy": current},
			IdempotencyScope: "billing.refund_policy.configure", IdempotencyKey: policyNo,
			OperationCode: "billing.refund_policy.configure",
			TargetType:    "refund_policy", TargetNo: policyNo,
			ResultCode: "succeeded", ReasonCode: "refund_policy_update",
			RiskCode: "critical", RequiredPermission: permissionBillingRefundPolicyWrite,
			AfterSnapshot: map[string]any{
				"currency_code": input.CurrencyCode, "version": version,
				"high_amount_threshold_minor": input.HighAmountThresholdMinor,
				"automatic_approval_enabled":  input.AutomaticApprovalEnabled,
			},
		})
	})
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	return result, nil
}

type managementRefundRow struct {
	RefundNo                       string
	CustomerNo                     string
	OrderNo                        string
	InvoiceNo                      string
	CurrencyCode                   string
	Status                         string
	Reason                         RefundReason
	ReasonDetail                   string
	AmountMinor                    int64
	ApprovalMode                   string
	Destination                    string
	RequiresEntitlementTermination bool
	HasResourceImpact              bool
	HasDangerousOperation          bool
	HasFundAnomaly                 bool
	HighAmount                     bool
	Version                        int64
	ImpactDigest                   string
	RequestDigest                  string
	SubmittedAt                    time.Time
	ApprovedAt                     *time.Time
	CompletedAt                    *time.Time
}

func (s *managementRefundService) prepareRefundReview(
	ctx context.Context,
	intent ManagementRefundReviewIntent,
	allowTerminalReplay bool,
) (int, ManagementRefundReviewIntent, *managementRefundRow, reauth.Binding, string, error) {
	intent.RequestNo = strings.TrimSpace(intent.RequestNo)
	intent.RefundNo = strings.TrimSpace(intent.RefundNo)
	intent.CommentDigest = strings.TrimSpace(intent.CommentDigest)
	intent.EvidenceDigest = strings.TrimSpace(intent.EvidenceDigest)
	_, intent.EvidenceDigest = adjustmentMetadata("", intent.EvidenceDigest, "", intent.RequestNo)
	actor, err := s.authorizer.AuthorizeRefundReview(ctx, intent.RefundNo)
	if err != nil {
		return 0, intent, nil, reauth.Binding{}, "", err
	}
	if !validBusinessReference(intent.RequestNo, 128) || intent.ExpectedVersion < 1 ||
		(intent.Decision != RefundReviewApprove && intent.Decision != RefundReviewReject) ||
		(intent.CommentDigest != "" && !canonicalDigestPattern.MatchString(intent.CommentDigest)) ||
		!canonicalDigestPattern.MatchString(intent.EvidenceDigest) {
		return 0, intent, nil, reauth.Binding{}, "", bizerr.NewCode(CodeRefundInvalid)
	}
	request, err := s.loadManagementRefund(ctx, intent.RefundNo, false)
	if err != nil || request == nil {
		if err == nil {
			err = bizerr.NewCode(CodeRefundNotAccessible)
		}
		return 0, intent, nil, reauth.Binding{}, "", err
	}
	if request.Status != "pending_review" && !allowTerminalReplay {
		return 0, intent, nil, reauth.Binding{}, "", bizerr.NewCode(CodeRefundConflict)
	}
	decisionEvidence, err := contentDigest(struct {
		RequestNo       string               `json:"requestNo"`
		RefundNo        string               `json:"refundNo"`
		ExpectedVersion int64                `json:"expectedVersion"`
		Decision        RefundReviewDecision `json:"decision"`
		EvidenceDigest  string               `json:"evidenceDigest"`
	}{intent.RequestNo, intent.RefundNo, intent.ExpectedVersion, intent.Decision, intent.EvidenceDigest})
	if err != nil {
		return 0, intent, nil, reauth.Binding{}, "", err
	}
	targetDigest, err := contentDigest(struct {
		RequestNo       string               `json:"requestNo"`
		RefundNo        string               `json:"refundNo"`
		ExpectedVersion int64                `json:"expectedVersion"`
		Decision        RefundReviewDecision `json:"decision"`
		ImpactDigest    string               `json:"impactDigest"`
		RequestDigest   string               `json:"requestDigest"`
	}{intent.RequestNo, intent.RefundNo, intent.ExpectedVersion, intent.Decision,
		request.ImpactDigest, request.RequestDigest})
	return actor, intent, request,
		reauth.Binding{Purpose: reauth.PurposeHighValueFunds, TargetDigest: targetDigest},
		decisionEvidence, err
}

func (s *managementRefundService) loadManagementRefund(
	ctx context.Context,
	refundNo string,
	lock bool,
) (*managementRefundRow, error) {
	query := `
SELECT refund_no,customer_no,order_no,invoice_no,currency_code,status_code AS status,
       reason_code AS reason,reason_detail,requested_amount_minor AS amount_minor,
       approval_mode_code AS approval_mode,refund_destination_code AS destination,
       requires_entitlement_termination,has_resource_impact,has_dangerous_operation,
       has_fund_anomaly,high_amount,version,impact_digest,request_digest,
       submitted_at,approved_at,completed_at
FROM plugin_idc_core_refund_request
WHERE tenant_id=0 AND refund_no=?`
	if lock {
		query += " FOR UPDATE"
	}
	var row *managementRefundRow
	err := s.database.GetScan(ctx, &row, query, refundNo)
	return row, err
}

func (r *managementRefundRow) toOnlineRefund() *OnlineRefund {
	if r == nil {
		return nil
	}
	return &OnlineRefund{
		RefundNo: r.RefundNo, CustomerNo: r.CustomerNo, OrderNo: r.OrderNo, InvoiceNo: r.InvoiceNo,
		CurrencyCode: r.CurrencyCode, Status: r.Status, Reason: r.Reason,
		ReasonDetail: r.ReasonDetail, AmountMinor: r.AmountMinor,
		ApprovalMode: r.ApprovalMode, Destination: r.Destination,
		RequiresEntitlementTermination: r.RequiresEntitlementTermination,
		HasResourceImpact:              r.HasResourceImpact, HasDangerousOperation: r.HasDangerousOperation,
		HasFundAnomaly: r.HasFundAnomaly, HighAmount: r.HighAmount,
		Version: r.Version, ImpactDigest: r.ImpactDigest, SubmittedAt: r.SubmittedAt.UTC(),
		ApprovedAt: refundUTCPointer(r.ApprovedAt), CompletedAt: refundUTCPointer(r.CompletedAt),
	}
}

func (s *managementRefundService) loadRefundDecisionReplay(
	ctx context.Context,
	request *managementRefundRow,
	intent ManagementRefundReviewIntent,
	evidence string,
) (*OnlineRefund, error) {
	if request == nil || request.Version != intent.ExpectedVersion+1 {
		return nil, nil
	}
	wantType := "manual_approve"
	wantStatus := "approved"
	if intent.Decision == RefundReviewReject {
		wantType = "manual_reject"
		wantStatus = "rejected"
	}
	if request.Status != wantStatus {
		return nil, nil
	}
	value, err := s.database.GetValue(ctx, `
SELECT COUNT(*) FROM plugin_idc_core_refund_decision
WHERE refund_no=? AND decision_type_code=? AND actor_type_code='staff'
  AND evidence_digest=?`, intent.RefundNo, wantType, evidence)
	if err != nil || value.Int64() != 1 {
		return nil, err
	}
	result := request.toOnlineRefund()
	result.Reused = true
	return result, nil
}

func (s *managementRefundService) approveRefund(
	ctx context.Context,
	request *managementRefundRow,
	actor int,
	intent ManagementRefundReviewIntent,
	evidence string,
	now time.Time,
) error {
	result, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code='approved',approved_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND refund_no=? AND status_code='pending_review' AND version=?`,
		now, now, request.RefundNo, intent.ExpectedVersion)
	if err != nil {
		return err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		if affectedErr != nil {
			return affectedErr
		}
		return bizerr.NewCode(CodeRefundConflict)
	}
	var sources []struct {
		RefundSourceNo         string `orm:"refund_source_no"`
		PaymentAllocationNo    string `orm:"payment_allocation_no"`
		PaymentSourceType      string `orm:"payment_source_type_code"`
		PaymentSourceReference string `orm:"payment_source_reference"`
		ExecutionOwner         string `orm:"execution_owner_code"`
		AmountMinor            int64  `orm:"amount_minor"`
		ContentDigest          string `orm:"content_digest"`
	}
	if err = s.database.GetScan(ctx, &sources, `
SELECT refund_source_no,COALESCE(payment_allocation_no,'') AS payment_allocation_no,
       payment_source_type_code,
       payment_source_reference,execution_owner_code,amount_minor,content_digest
FROM plugin_idc_core_refund_source_allocation
WHERE tenant_id=0 AND refund_no=? ORDER BY id`, request.RefundNo); err != nil {
		return err
	}
	if len(sources) == 0 {
		return bizerr.NewCode(CodeRefundIntegrity)
	}
	commandWriter := &onlineRefundService{database: s.database, random: s.random}
	commands := make([]refundExecutionCommandInsert, 0, len(sources))
	for _, row := range sources {
		source := RefundSourceAllocation{
			PaymentAllocationNo:    row.PaymentAllocationNo,
			PaymentSourceType:      row.PaymentSourceType,
			PaymentSourceReference: row.PaymentSourceReference,
			ExecutionOwner:         row.ExecutionOwner, AmountMinor: row.AmountMinor,
			ContentDigest: row.ContentDigest,
		}
		command, commandErr := commandWriter.newRefundExecutionCommand(
			request.RefundNo, row.RefundSourceNo, request.CurrencyCode,
			source, "original_channel", now,
		)
		if commandErr != nil {
			return commandErr
		}
		commands = append(commands, command)
	}
	if err = commandWriter.insertRefundExecutionCommands(ctx, commands); err != nil {
		return err
	}
	sequence, err := s.nextRefundDecisionSequence(ctx, request.RefundNo)
	if err != nil {
		return err
	}
	return commandWriter.insertRefundDecision(
		ctx, request.RefundNo, sequence, "manual_approve", "pending_review", "approved",
		"staff", actor, "staff_approved", intent.CommentDigest, evidence, now,
	)
}

func (s *managementRefundService) rejectRefund(
	ctx context.Context,
	request *managementRefundRow,
	actor int,
	intent ManagementRefundReviewIntent,
	evidence string,
	now time.Time,
) error {
	result, err := s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code='rejected',completed_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND refund_no=? AND status_code='pending_review' AND version=?`,
		now, now, request.RefundNo, intent.ExpectedVersion)
	if err != nil {
		return err
	}
	if affected, affectedErr := result.RowsAffected(); affectedErr != nil || affected != 1 {
		if affectedErr != nil {
			return affectedErr
		}
		return bizerr.NewCode(CodeRefundConflict)
	}
	if _, err = s.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_entitlement_lock
SET released_at=?,release_reason_code='rejected'
WHERE tenant_id=0 AND refund_no=? AND released_at IS NULL`, now, request.RefundNo); err != nil {
		return err
	}
	sequence, err := s.nextRefundDecisionSequence(ctx, request.RefundNo)
	if err != nil {
		return err
	}
	commandWriter := &onlineRefundService{database: s.database, random: s.random}
	return commandWriter.insertRefundDecision(
		ctx, request.RefundNo, sequence, "manual_reject", "pending_review", "rejected",
		"staff", actor, "staff_rejected", intent.CommentDigest, evidence, now,
	)
}

func (s *managementRefundService) nextRefundDecisionSequence(
	ctx context.Context,
	refundNo string,
) (int, error) {
	value, err := s.database.GetValue(ctx, `
SELECT COALESCE(MAX(sequence_no),0)+1
FROM plugin_idc_core_refund_decision WHERE refund_no=?`, refundNo)
	if err != nil || value.Int() < 1 {
		return 0, err
	}
	return value.Int(), nil
}
