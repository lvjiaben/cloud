package billing

import (
	"context"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

type creditPolicyRow struct {
	PolicyNo                 string    `orm:"policy_no"`
	Version                  int64     `orm:"policy_version"`
	CurrencyCode             string    `orm:"currency_code"`
	MaximumLimitMinor        int64     `orm:"maximum_limit_minor"`
	DualReviewThresholdMinor int64     `orm:"dual_review_threshold_minor"`
	BillingDay               int       `orm:"billing_day"`
	RepaymentPeriodDays      int       `orm:"repayment_period_days"`
	GracePeriodDays          int       `orm:"grace_period_days"`
	DailyLateFeeBasisPoints  int       `orm:"daily_late_fee_basis_points"`
	MaximumLateFeeMinor      int64     `orm:"maximum_late_fee_minor"`
	ConfiguredBy             int       `orm:"configured_by"`
	ContentDigest            string    `orm:"content_digest"`
	CreatedAt                time.Time `orm:"created_at"`
}

type creditApplicationRow struct {
	ApplicationNo         string    `orm:"application_no"`
	CustomerNo            string    `orm:"customer_no"`
	CurrencyCode          string    `orm:"currency_code"`
	PolicyNo              string    `orm:"policy_no"`
	PolicyVersion         int64     `orm:"policy_version"`
	RequestedLimitMinor   int64     `orm:"requested_limit_minor"`
	RequestedValidFrom    time.Time `orm:"requested_valid_from"`
	RequestedValidUntil   time.Time `orm:"requested_valid_until"`
	RequiredApprovalCount int       `orm:"required_approval_count"`
	RequestDigest         string    `orm:"request_digest"`
	StatusCode            string    `orm:"status_code"`
	RequestActorType      string    `orm:"request_actor_type_code"`
	RequestedBy           int       `orm:"requested_by"`
	ReviewCount           int       `orm:"review_count"`
	CreditAccountNo       string    `orm:"credit_account_no"`
	CreatedAt             time.Time `orm:"created_at"`
	UpdatedAt             time.Time `orm:"updated_at"`
}

type creditReviewRow struct {
	DecisionCode   string `orm:"decision_code"`
	CommentDigest  string `orm:"comment_digest"`
	EvidenceDigest string `orm:"evidence_digest"`
}

func (s *serviceImpl) ConfigureCreditPolicy(
	ctx context.Context,
	input CreditPolicyInput,
) (*CreditPolicy, error) {
	command, err := normalizeCreditPolicy(input)
	if err != nil {
		return nil, err
	}
	var result *CreditPolicy
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := s.lockSource(ctx, "credit_policy", command.CurrencyCode); lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadCreditPolicy(ctx, s.database, command.PolicyNo)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.ContentDigest != command.ContentDigest {
				return bizerr.NewCode(CodeCreditConflict)
			}
			result = projectCreditPolicy(existing, true)
			return nil
		}
		versionValue, versionErr := s.database.GetValue(ctx, `
SELECT COALESCE(MAX(policy_version), 0) + 1
FROM plugin_idc_core_credit_policy_revision
WHERE tenant_id = 0 AND currency_code = ?`, command.CurrencyCode)
		if versionErr != nil {
			return gerror.Wrap(versionErr, "select next credit policy version")
		}
		version := versionValue.Int64()
		if version <= 0 {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_policy_revision (
    tenant_id, policy_no, policy_version, currency_code,
    maximum_limit_minor, dual_review_threshold_minor, billing_day,
    repayment_period_days, grace_period_days, daily_late_fee_basis_points,
    maximum_late_fee_minor, configured_by, content_digest
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			command.PolicyNo, version, command.CurrencyCode,
			command.MaximumLimitMinor, command.DualReviewThresholdMinor,
			command.BillingDay, command.RepaymentPeriodDays, command.GracePeriodDays,
			command.DailyLateFeeBasisPoints, command.MaximumLateFeeMinor,
			command.ConfiguredBy, command.ContentDigest); insertErr != nil {
			return gerror.Wrap(insertErr, "append credit policy revision")
		}
		created, loadErr := loadCreditPolicy(ctx, s.database, command.PolicyNo)
		if loadErr != nil {
			return loadErr
		}
		if created == nil || created.Version != version || created.ContentDigest != command.ContentDigest {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		result = projectCreditPolicy(created, false)
		return nil
	})
	return creditPolicyResult(result, err)
}

func (s *serviceImpl) SubmitCreditApplication(
	ctx context.Context,
	input CreditApplicationInput,
) (*CreditApplication, error) {
	command, err := normalizeCreditApplication(input)
	if err != nil {
		return nil, err
	}
	var result *CreditApplication
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := s.lockSource(ctx, "credit_application", command.ApplicationNo); lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadCreditApplication(ctx, s.database, command.ApplicationNo, false)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.RequestDigest != command.RequestDigest {
				return bizerr.NewCode(CodeCreditConflict)
			}
			result = projectCreditApplication(existing, true)
			return nil
		}
		if customerErr := s.requireActiveCustomer(ctx, command.CustomerNo); customerErr != nil {
			return customerErr
		}
		var policy *creditPolicyRow
		if loadErr = s.database.GetScan(ctx, &policy, `
SELECT policy_no, policy_version, currency_code, maximum_limit_minor,
       dual_review_threshold_minor, billing_day, repayment_period_days,
       grace_period_days, daily_late_fee_basis_points, maximum_late_fee_minor,
       configured_by, content_digest, created_at
FROM plugin_idc_core_credit_policy_revision
WHERE tenant_id = 0 AND currency_code = ?
ORDER BY policy_version DESC, id DESC LIMIT 1`, command.CurrencyCode); loadErr != nil {
			return gerror.Wrap(loadErr, "load current credit policy")
		}
		if policy == nil {
			return bizerr.NewCode(CodeCreditConflict)
		}
		if command.RequestedLimitMinor > policy.MaximumLimitMinor {
			return bizerr.NewCode(CodeCreditConflict)
		}
		validity, validityErr := s.database.GetValue(ctx,
			`SELECT CURRENT_TIMESTAMP < ?`, command.RequestedValidUntil)
		if validityErr != nil {
			return gerror.Wrap(validityErr, "validate credit application period")
		}
		if !validity.Bool() {
			return bizerr.NewCode(CodeCreditConflict)
		}
		conflicts, conflictErr := s.database.GetValue(ctx, `
SELECT COUNT(*)
FROM (
    SELECT 1 FROM plugin_idc_core_credit_account
    WHERE tenant_id = 0 AND customer_no = ? AND currency_code = ?
    UNION ALL
    SELECT 1 FROM plugin_idc_core_credit_application
    WHERE tenant_id = 0 AND customer_no = ? AND currency_code = ?
      AND status_code IN ('pending_review', 'approved')
) conflict`, command.CustomerNo, command.CurrencyCode,
			command.CustomerNo, command.CurrencyCode)
		if conflictErr != nil {
			return gerror.Wrap(conflictErr, "inspect existing customer credit")
		}
		if conflicts.Int64() != 0 {
			return bizerr.NewCode(CodeCreditConflict)
		}
		approvalCount := 1
		if command.RequestedLimitMinor >= policy.DualReviewThresholdMinor {
			approvalCount = 2
		}
		command.RequiredApprovalCount = approvalCount
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_application (
    tenant_id, application_no, customer_no, currency_code, policy_no,
    policy_version, requested_limit_minor, requested_valid_from,
    requested_valid_until, required_approval_count, risk_evidence_reference,
    risk_evidence_digest, request_digest, status_code,
    request_actor_type_code, requested_by
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending_review', ?, ?)`,
			command.ApplicationNo, command.CustomerNo, command.CurrencyCode,
			policy.PolicyNo, policy.Version, command.RequestedLimitMinor,
			command.RequestedValidFrom, command.RequestedValidUntil, approvalCount,
			command.RiskEvidenceReference, command.RiskEvidenceDigest,
			command.RequestDigest, command.RequestActorType, command.RequestedBy); insertErr != nil {
			return gerror.Wrap(insertErr, "create credit application")
		}
		created, loadErr := loadCreditApplication(ctx, s.database, command.ApplicationNo, false)
		if loadErr != nil {
			return loadErr
		}
		if created == nil || created.RequestDigest != command.RequestDigest {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		result = projectCreditApplication(created, false)
		return nil
	})
	return creditApplicationResult(result, err)
}

func (s *serviceImpl) ReviewCreditApplication(
	ctx context.Context,
	input CreditReviewInput,
) (*CreditApplication, error) {
	normalized, err := normalizeCreditReview(input)
	if err != nil {
		return nil, err
	}
	var result *CreditApplication
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		application, loadErr := loadCreditApplication(ctx, s.database, normalized.ApplicationNo, true)
		if loadErr != nil {
			return loadErr
		}
		if application == nil {
			return bizerr.NewCode(CodeCreditNotFound)
		}
		var existing *creditReviewRow
		if loadErr = s.database.GetScan(ctx, &existing, `
SELECT decision_code, comment_digest, evidence_digest
FROM plugin_idc_core_credit_application_review
WHERE tenant_id = 0 AND application_no = ? AND reviewer_id = ?`,
			normalized.ApplicationNo, normalized.ReviewerID); loadErr != nil {
			return gerror.Wrap(loadErr, "load existing credit review")
		}
		if existing != nil {
			if existing.DecisionCode != string(normalized.Decision) ||
				existing.CommentDigest != normalized.CommentDigest ||
				existing.EvidenceDigest != normalized.EvidenceDigest {
				return bizerr.NewCode(CodeCreditConflict)
			}
			application, loadErr = loadCreditApplication(ctx, s.database, normalized.ApplicationNo, false)
			if loadErr != nil {
				return loadErr
			}
			result = projectCreditApplication(application, true)
			return nil
		}
		if application.StatusCode != "pending_review" ||
			(application.RequestActorType == "staff" && application.RequestedBy == normalized.ReviewerID) ||
			application.ReviewCount >= application.RequiredApprovalCount {
			return bizerr.NewCode(CodeCreditConflict)
		}
		stillValid, validityErr := s.database.GetValue(ctx,
			`SELECT CURRENT_TIMESTAMP < ?`, application.RequestedValidUntil)
		if validityErr != nil {
			return gerror.Wrap(validityErr, "validate credit application review period")
		}
		if !stillValid.Bool() {
			return bizerr.NewCode(CodeCreditConflict)
		}
		sequence := application.ReviewCount + 1
		reviewNo := deterministicCreditNo(
			"crv_", "review", application.ApplicationNo, strconv.Itoa(normalized.ReviewerID),
		)
		reviewDigest, digestErr := contentDigest(struct {
			ReviewNo, ApplicationNo, Decision, CommentDigest, EvidenceDigest string
			Sequence, ReviewerID                                             int
		}{
			reviewNo, application.ApplicationNo, string(normalized.Decision),
			normalized.CommentDigest, normalized.EvidenceDigest,
			sequence, normalized.ReviewerID,
		})
		if digestErr != nil {
			return digestErr
		}
		if _, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_application_review (
    tenant_id, review_no, application_no, customer_no, currency_code,
    review_sequence, decision_code, reviewer_id, comment_digest,
    evidence_digest, content_digest
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			reviewNo, application.ApplicationNo, application.CustomerNo,
			application.CurrencyCode, sequence, normalized.Decision,
			normalized.ReviewerID, normalized.CommentDigest,
			normalized.EvidenceDigest, reviewDigest); insertErr != nil {
			return gerror.Wrap(insertErr, "append credit application review")
		}
		finalStatus := "pending_review"
		if normalized.Decision == CreditDecisionReject {
			finalStatus = "rejected"
		} else if sequence == application.RequiredApprovalCount {
			finalStatus = "approved"
		}
		if finalStatus != "pending_review" {
			if _, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_credit_application
SET status_code = ?, decided_at = CURRENT_TIMESTAMP,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND application_no = ? AND status_code = 'pending_review'`,
				finalStatus, application.ApplicationNo); updateErr != nil {
				return gerror.Wrap(updateErr, "complete credit application review")
			}
		}
		if finalStatus == "approved" {
			if activateErr := s.activateApprovedCreditAccount(ctx, application, normalized, reviewDigest); activateErr != nil {
				return activateErr
			}
		}
		application, loadErr = loadCreditApplication(ctx, s.database, normalized.ApplicationNo, false)
		if loadErr != nil {
			return loadErr
		}
		if application == nil || application.StatusCode != finalStatus {
			return bizerr.NewCode(CodeCreditIntegrity)
		}
		result = projectCreditApplication(application, false)
		return nil
	})
	return creditApplicationResult(result, err)
}

func (s *serviceImpl) activateApprovedCreditAccount(
	ctx context.Context,
	application *creditApplicationRow,
	review CreditReviewInput,
	lastReviewDigest string,
) error {
	var policy *creditPolicyRow
	if err := s.database.GetScan(ctx, &policy, `
SELECT policy_no, policy_version, currency_code, maximum_limit_minor,
       dual_review_threshold_minor, billing_day, repayment_period_days,
       grace_period_days, daily_late_fee_basis_points, maximum_late_fee_minor,
       configured_by, content_digest, created_at
FROM plugin_idc_core_credit_policy_revision
WHERE tenant_id = 0 AND policy_no = ? AND policy_version = ? AND currency_code = ?`,
		application.PolicyNo, application.PolicyVersion, application.CurrencyCode); err != nil {
		return gerror.Wrap(err, "load approved credit policy")
	}
	if policy == nil {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	creditSpec := accountSpec{
		OwnerType: "customer", OwnerReference: application.CustomerNo,
		CustomerNo: application.CustomerNo, AccountType: AccountTypeCustomerCredit,
		NormalBalance: "debit",
	}
	accounts, err := s.ensureAndLockAccounts(ctx, application.CurrencyCode, true, creditSpec)
	if err != nil {
		return err
	}
	ledgerAccountNo := deterministicAccountNo(
		creditSpec.OwnerType, creditSpec.OwnerReference, creditSpec.AccountType,
		application.CurrencyCode,
	)
	if accounts[ledgerAccountNo] == nil {
		return bizerr.NewCode(CodeCreditIntegrity)
	}
	creditAccountNo := deterministicCreditNo(
		"crc_", "account", application.CustomerNo, application.CurrencyCode,
	)
	termsNo := deterministicCreditNo("crt_", "terms", application.ApplicationNo)
	movementNo := deterministicCreditNo("crm_", "grant", application.ApplicationNo)
	var clock struct {
		Now time.Time `orm:"now"`
	}
	if err = s.database.GetScan(ctx, &clock, `SELECT CURRENT_TIMESTAMP AS now`); err != nil {
		return gerror.Wrap(err, "read credit activation clock")
	}
	status := "pending_activation"
	if !clock.Now.Before(application.RequestedValidFrom) {
		status = "active"
	}
	approvalDigest, err := contentDigest(struct {
		ApplicationNo, LastReviewDigest string
		ApprovedBy                      int
		ApprovalCount                   int
	}{application.ApplicationNo, lastReviewDigest, review.ReviewerID, application.RequiredApprovalCount})
	if err != nil {
		return err
	}
	termsDigest, err := contentDigest(struct {
		TermsNo, CreditAccountNo, ApplicationNo, PolicyNo, CurrencyCode string
		TermsVersion, PolicyVersion                                     int64
		TotalLimitMinor                                                 int64
		BillingDay, RepaymentPeriodDays, GracePeriodDays                int
		DailyLateFeeBasisPoints                                         int
		MaximumLateFeeMinor                                             int64
		EffectiveFrom, EffectiveUntil                                   time.Time
		ApprovedBy                                                      int
		ApprovalDigest                                                  string
	}{
		termsNo, creditAccountNo, application.ApplicationNo, policy.PolicyNo,
		application.CurrencyCode, 1, policy.Version, application.RequestedLimitMinor,
		policy.BillingDay, policy.RepaymentPeriodDays, policy.GracePeriodDays,
		policy.DailyLateFeeBasisPoints, policy.MaximumLateFeeMinor,
		application.RequestedValidFrom, application.RequestedValidUntil,
		review.ReviewerID, approvalDigest,
	})
	if err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_account (
    tenant_id, credit_account_no, customer_no, currency_code,
    ledger_account_no, ledger_account_type_code, origin_application_no,
    current_terms_no, current_terms_version, status_code, total_limit_minor,
    available_limit_minor, valid_from, valid_until, version
) VALUES (0, ?, ?, ?, ?, 'customer_credit', ?, ?, 1, ?, ?, ?, ?, ?, 1)`,
		creditAccountNo, application.CustomerNo, application.CurrencyCode,
		ledgerAccountNo, application.ApplicationNo, termsNo, status,
		application.RequestedLimitMinor, application.RequestedLimitMinor,
		application.RequestedValidFrom, application.RequestedValidUntil); err != nil {
		return gerror.Wrap(err, "create approved credit account")
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_account_terms (
    tenant_id, terms_no, credit_account_no, customer_no, currency_code,
    terms_version, application_no, policy_no, policy_version,
    total_limit_minor, billing_day, repayment_period_days, grace_period_days,
    daily_late_fee_basis_points, maximum_late_fee_minor, effective_from,
    effective_until, approved_by, approval_digest, content_digest
) VALUES (0, ?, ?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		termsNo, creditAccountNo, application.CustomerNo, application.CurrencyCode,
		application.ApplicationNo, policy.PolicyNo, policy.Version,
		application.RequestedLimitMinor, policy.BillingDay,
		policy.RepaymentPeriodDays, policy.GracePeriodDays,
		policy.DailyLateFeeBasisPoints, policy.MaximumLateFeeMinor,
		application.RequestedValidFrom, application.RequestedValidUntil,
		review.ReviewerID, approvalDigest, termsDigest); err != nil {
		return gerror.Wrap(err, "append approved credit terms")
	}
	idempotencyDigest, err := contentDigest(struct {
		Operation, ApplicationNo string
	}{"credit.limit_granted", application.ApplicationNo})
	if err != nil {
		return err
	}
	movementDigest, err := contentDigest(struct {
		MovementNo, CreditAccountNo, CustomerNo, CurrencyCode string
		StatusAfter, SourceReference                          string
		AmountMinor                                           int64
		ActorID                                               int
	}{
		movementNo, creditAccountNo, application.CustomerNo,
		application.CurrencyCode, status, application.ApplicationNo,
		application.RequestedLimitMinor, review.ReviewerID,
	})
	if err != nil {
		return err
	}
	if _, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_credit_account_movement (
    tenant_id, movement_no, credit_account_no, customer_no, currency_code,
    movement_type_code, amount_minor, account_version_before,
    account_version_after, status_before_code, status_after_code,
    total_limit_before_minor, total_limit_after_minor, available_before_minor,
    available_after_minor, unbilled_before_minor, unbilled_after_minor,
    billed_before_minor, billed_after_minor, delinquent_before_minor,
    delinquent_after_minor, written_off_before_minor, written_off_after_minor,
    source_type_code, source_reference, idempotency_digest, evidence_digest,
    content_digest, actor_type_code, actor_id, occurred_at
) VALUES (
    0, ?, ?, ?, ?, 'limit_granted', ?, 0, 1, NULL, ?,
    0, ?, 0, ?, 0, 0, 0, 0, 0, 0, 0, 0,
    'application', ?, ?, ?, ?, 'staff', ?, ?
)`, movementNo, creditAccountNo, application.CustomerNo, application.CurrencyCode,
		application.RequestedLimitMinor, status, application.RequestedLimitMinor,
		application.RequestedLimitMinor, application.ApplicationNo,
		idempotencyDigest, approvalDigest, movementDigest, review.ReviewerID,
		clock.Now); err != nil {
		return gerror.Wrap(err, "append initial credit movement")
	}
	return nil
}

func loadCreditPolicy(ctx context.Context, database gdb.DB, policyNo string) (*creditPolicyRow, error) {
	var row *creditPolicyRow
	if err := database.GetScan(ctx, &row, `
SELECT policy_no, policy_version, currency_code, maximum_limit_minor,
       dual_review_threshold_minor, billing_day, repayment_period_days,
       grace_period_days, daily_late_fee_basis_points, maximum_late_fee_minor,
       configured_by, content_digest, created_at
FROM plugin_idc_core_credit_policy_revision
WHERE tenant_id = 0 AND policy_no = ?`, policyNo); err != nil {
		return nil, gerror.Wrap(err, "load credit policy")
	}
	return row, nil
}

func loadCreditApplication(
	ctx context.Context,
	database gdb.DB,
	applicationNo string,
	lock bool,
) (*creditApplicationRow, error) {
	locking := ""
	if lock {
		locking = " FOR UPDATE"
	}
	var row *creditApplicationRow
	if err := database.GetScan(ctx, &row, `
SELECT application.application_no, application.customer_no,
       application.currency_code, application.policy_no,
       application.policy_version, application.requested_limit_minor,
       application.requested_valid_from, application.requested_valid_until,
       application.required_approval_count, application.request_digest,
       application.status_code, application.request_actor_type_code,
       application.requested_by, application.created_at, application.updated_at,
       (SELECT COUNT(*) FROM plugin_idc_core_credit_application_review review
        WHERE review.application_no = application.application_no) AS review_count,
       COALESCE((SELECT account.credit_account_no
                 FROM plugin_idc_core_credit_account account
                 WHERE account.origin_application_no = application.application_no), '')
           AS credit_account_no
FROM plugin_idc_core_credit_application application
WHERE application.tenant_id = 0 AND application.application_no = ?`+locking,
		applicationNo); err != nil {
		return nil, gerror.Wrap(err, "load credit application")
	}
	return row, nil
}

func projectCreditPolicy(row *creditPolicyRow, reused bool) *CreditPolicy {
	if row == nil {
		return nil
	}
	return &CreditPolicy{
		PolicyNo: row.PolicyNo, Version: row.Version, CurrencyCode: row.CurrencyCode,
		MaximumLimitMinor:        row.MaximumLimitMinor,
		DualReviewThresholdMinor: row.DualReviewThresholdMinor,
		BillingDay:               row.BillingDay, RepaymentPeriodDays: row.RepaymentPeriodDays,
		GracePeriodDays:         row.GracePeriodDays,
		DailyLateFeeBasisPoints: row.DailyLateFeeBasisPoints,
		MaximumLateFeeMinor:     row.MaximumLateFeeMinor, ConfiguredBy: row.ConfiguredBy,
		ContentDigest: row.ContentDigest, CreatedAt: row.CreatedAt, Reused: reused,
	}
}

func projectCreditApplication(row *creditApplicationRow, reused bool) *CreditApplication {
	if row == nil {
		return nil
	}
	return &CreditApplication{
		ApplicationNo: row.ApplicationNo, CustomerNo: row.CustomerNo,
		CurrencyCode: row.CurrencyCode, PolicyNo: row.PolicyNo,
		PolicyVersion: row.PolicyVersion, RequestedLimitMinor: row.RequestedLimitMinor,
		RequestedValidFrom:    row.RequestedValidFrom,
		RequestedValidUntil:   row.RequestedValidUntil,
		RequiredApprovalCount: row.RequiredApprovalCount, StatusCode: row.StatusCode,
		RequestActorType: row.RequestActorType, RequestedBy: row.RequestedBy,
		ReviewCount: row.ReviewCount, CreditAccountNo: row.CreditAccountNo,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, Reused: reused,
	}
}

func creditPolicyResult(result *CreditPolicy, err error) (*CreditPolicy, error) {
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
}

func creditApplicationResult(result *CreditApplication, err error) (*CreditApplication, error) {
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeCreditUnavailable)
}
