// This file owns recharge-bonus and cashback entitlement conservation.

package marketing

import (
	"context"
	"crypto/rand"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumValueGrantAmountMinor int64 = 9_000_000_000_000_000

const (
	ValueGrantTypeRechargeBonus = "recharge_bonus"
	ValueGrantTypeCashback      = "cashback"
)

// ValueGrantCreateInput creates one pending entitlement before financial posting.
type ValueGrantCreateInput struct {
	GrantType       string
	CampaignNo      string
	BenefitNo       string
	CustomerNo      string
	CurrencyCode    string
	SourceReference string
	AmountMinor     int64
	Withdrawable    bool
	ExpiresAt       *time.Time
}

// ValueGrantPostInput binds a pending entitlement to an already-created financial posting.
type ValueGrantPostInput struct {
	GrantNo                   string
	CustomerNo                string
	FinancialPostingReference string
}

// ValueGrantMutationInput applies an idempotent exact-grant lifecycle operation.
type ValueGrantMutationInput struct {
	GrantNo         string
	CustomerNo      string
	SourceType      string
	SourceReference string
	AmountMinor     int64
}

// ValueGrantProjection is the authoritative marketing entitlement state, not a cash ledger balance.
type ValueGrantProjection struct {
	GrantNo                   string
	GrantType                 string
	CampaignNo                string
	BenefitNo                 string
	BenefitSnapshotDigest     string
	CustomerNo                string
	CurrencyCode              string
	OriginalAmountMinor       int64
	PendingAmountMinor        int64
	AvailableAmountMinor      int64
	FrozenAmountMinor         int64
	ConsumedAmountMinor       int64
	ExpiredAmountMinor        int64
	ReversedAmountMinor       int64
	Withdrawable              bool
	Status                    string
	FinancialPostingReference string
	ExpiresAt                 *time.Time
	Reused                    bool
}

// ValueGrantService owns marketing entitlement state only. Financial callers own ledger posting.
type ValueGrantService interface {
	CreatePending(ctx context.Context, input ValueGrantCreateInput) (*ValueGrantProjection, error)
	Post(ctx context.Context, input ValueGrantPostInput) (*ValueGrantProjection, error)
	CancelPending(ctx context.Context, input ValueGrantMutationInput) (*ValueGrantProjection, error)
	Freeze(ctx context.Context, input ValueGrantMutationInput) (*ValueGrantProjection, error)
	Release(ctx context.Context, input ValueGrantMutationInput) (*ValueGrantProjection, error)
	Consume(ctx context.Context, input ValueGrantMutationInput) (*ValueGrantProjection, error)
	Expire(ctx context.Context, input ValueGrantMutationInput) (*ValueGrantProjection, error)
	Reverse(ctx context.Context, input ValueGrantMutationInput) (*ValueGrantProjection, error)
}

type valueGrantServiceImpl struct {
	database gdb.DB
	random   io.Reader
}

var _ ValueGrantService = (*valueGrantServiceImpl)(nil)

// NewValueGrantService constructs a local entitlement service without financial side effects.
func NewValueGrantService(database gdb.DB) (ValueGrantService, error) {
	return newValueGrantService(database, rand.Reader)
}

func newValueGrantService(database gdb.DB, random io.Reader) (*valueGrantServiceImpl, error) {
	if database == nil {
		return nil, gerror.New("value grant service requires a database")
	}
	if random == nil {
		return nil, gerror.New("value grant service requires a random source")
	}
	return &valueGrantServiceImpl{database: database, random: random}, nil
}

type valueGrantRow struct {
	GrantNo                   string     `orm:"grant_no"`
	GrantType                 string     `orm:"grant_type_code"`
	CampaignNo                string     `orm:"campaign_no"`
	BenefitNo                 string     `orm:"benefit_no"`
	BenefitSnapshotDigest     string     `orm:"benefit_snapshot_digest"`
	CustomerNo                string     `orm:"customer_no"`
	CurrencyCode              string     `orm:"currency_code"`
	SourceReference           string     `orm:"source_reference"`
	OriginalAmountMinor       int64      `orm:"original_amount_minor"`
	PendingAmountMinor        int64      `orm:"pending_amount_minor"`
	AvailableAmountMinor      int64      `orm:"available_amount_minor"`
	FrozenAmountMinor         int64      `orm:"frozen_amount_minor"`
	ConsumedAmountMinor       int64      `orm:"consumed_amount_minor"`
	ExpiredAmountMinor        int64      `orm:"expired_amount_minor"`
	ReversedAmountMinor       int64      `orm:"reversed_amount_minor"`
	Withdrawable              bool       `orm:"withdrawable"`
	GlobalQuotaReserved       bool       `orm:"global_quota_reserved"`
	CustomerQuotaReserved     bool       `orm:"customer_quota_reserved"`
	ExpiresAt                 *time.Time `orm:"expires_at"`
	Status                    string     `orm:"status_code"`
	FinancialPostingReference *string    `orm:"financial_posting_reference"`
}

type valueGrantRuleRow struct {
	CampaignNo            string  `orm:"campaign_no"`
	CampaignType          string  `orm:"campaign_type_code"`
	TargetSegmentNo       *string `orm:"target_segment_no"`
	TargetSnapshotNo      *string `orm:"target_snapshot_no"`
	Priority              int     `orm:"priority"`
	StackPolicy           string  `orm:"stack_policy_code"`
	StackGroup            string  `orm:"stack_group_code"`
	BenefitNo             string  `orm:"benefit_no"`
	BenefitType           string  `orm:"benefit_type_code"`
	CurrencyCode          *string `orm:"currency_code"`
	AmountMinor           *int64  `orm:"amount_minor"`
	RateBasisPoints       *int    `orm:"rate_basis_points"`
	MaximumDiscountMinor  *int64  `orm:"maximum_discount_minor"`
	MinimumSpendMinor     *int64  `orm:"minimum_spend_minor"`
	PurchasePhase         string  `orm:"purchase_phase_code"`
	CycleScope            string  `orm:"cycle_scope_code"`
	CycleCount            *int    `orm:"cycle_count"`
	CampaignVersion       int64   `orm:"campaign_version"`
	BenefitVersion        int64   `orm:"benefit_version"`
	GlobalQuotaLimit      *int64  `orm:"global_quota_limit"`
	PerCustomerQuotaLimit *int    `orm:"per_customer_quota_limit"`
}

func (row valueGrantRuleRow) candidate() candidateRow {
	return candidateRow{
		CampaignNo: row.CampaignNo, CampaignType: row.CampaignType,
		TargetSegmentNo: row.TargetSegmentNo, TargetSnapshotNo: row.TargetSnapshotNo,
		Priority: row.Priority, StackPolicy: row.StackPolicy, StackGroup: row.StackGroup,
		BenefitNo: row.BenefitNo, BenefitType: row.BenefitType, CurrencyCode: row.CurrencyCode,
		AmountMinor: row.AmountMinor, RateBasisPoints: row.RateBasisPoints,
		MaximumDiscountMinor: row.MaximumDiscountMinor, MinimumSpendMinor: row.MinimumSpendMinor,
		PurchasePhase: row.PurchasePhase, CycleScope: row.CycleScope, CycleCount: row.CycleCount,
		CampaignVersion: row.CampaignVersion, BenefitVersion: row.BenefitVersion,
	}
}

type valueEntryRow struct {
	EntryNo             string  `orm:"entry_no"`
	EntryType           string  `orm:"entry_type_code"`
	PendingDeltaMinor   int64   `orm:"pending_delta_minor"`
	AvailableDeltaMinor int64   `orm:"available_delta_minor"`
	FrozenDeltaMinor    int64   `orm:"frozen_delta_minor"`
	ConsumedDeltaMinor  int64   `orm:"consumed_delta_minor"`
	ExpiredDeltaMinor   int64   `orm:"expired_delta_minor"`
	ReversedDeltaMinor  int64   `orm:"reversed_delta_minor"`
	ReversalOfEntryNo   *string `orm:"reversal_of_entry_no"`
}

type valueEntryInsert struct {
	GrantNo             string
	CustomerNo          string
	CurrencyCode        string
	EntryType           string
	PendingDeltaMinor   int64
	AvailableDeltaMinor int64
	FrozenDeltaMinor    int64
	ConsumedDeltaMinor  int64
	ExpiredDeltaMinor   int64
	ReversedDeltaMinor  int64
	SourceType          string
	SourceReference     string
	ReversalOfEntryNo   string
}

func (s *valueGrantServiceImpl) CreatePending(
	ctx context.Context,
	input ValueGrantCreateInput,
) (*ValueGrantProjection, error) {
	normalized, err := normalizeValueGrantCreate(input)
	if err != nil {
		return nil, err
	}
	var result *ValueGrantProjection
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if lockErr := lockValueGrantCustomer(ctx, s.database, normalized.CustomerNo); lockErr != nil {
			return lockErr
		}
		existing, loadErr := loadValueGrantBySource(
			ctx, s.database, normalized.GrantType, normalized.CustomerNo,
			normalized.SourceReference, true,
		)
		if loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if !sameValueGrantCreate(existing, normalized) {
				return bizerr.NewCode(CodeValueGrantConflict)
			}
			result = projectValueGrant(existing, true)
			return nil
		}
		databaseTime, timeErr := s.database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
		if timeErr != nil {
			return gerror.Wrap(timeErr, "read value grant database time")
		}
		databaseNow := databaseTime.Time()
		if normalized.ExpiresAt != nil && !normalized.ExpiresAt.After(databaseNow) {
			return bizerr.NewCode(CodeValueGrantInvalid)
		}
		rule, ruleErr := loadValueGrantRule(ctx, s.database, normalized)
		if ruleErr != nil {
			return ruleErr
		}
		if rule == nil || rule.AmountMinor == nil || *rule.AmountMinor != normalized.AmountMinor {
			return bizerr.NewCode(CodeValueGrantConflict)
		}
		digest, digestErr := benefitDigest(rule.candidate())
		if digestErr != nil {
			return gerror.Wrap(digestErr, "digest value grant benefit")
		}
		quota := campaignQuota{global: rule.GlobalQuotaLimit, perCustomer: rule.PerCustomerQuotaLimit}
		if quotaErr := incrementCampaignQuota(
			ctx, s.database, normalized.CampaignNo, normalized.CustomerNo, quota,
		); quotaErr != nil {
			return quotaErr
		}
		grantNo, numberErr := newMarketingNumber(s.random, "vgrt_")
		if numberErr != nil {
			return numberErr
		}
		_, insertErr := s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_value_grant (
    tenant_id, grant_no, grant_type_code, campaign_no, campaign_type_code,
    benefit_no, benefit_type_code, benefit_snapshot_digest,
    customer_no, currency_code, source_reference, original_amount_minor,
	pending_amount_minor, withdrawable, global_quota_reserved,
	customer_quota_reserved, expires_at, status_code
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'pending')`,
			grantNo, normalized.GrantType, normalized.CampaignNo, normalized.GrantType,
			normalized.BenefitNo, normalized.GrantType, digest,
			normalized.CustomerNo, normalized.CurrencyCode, normalized.SourceReference,
			normalized.AmountMinor, normalized.AmountMinor, normalized.Withdrawable,
			quota.global != nil, quota.perCustomer != nil,
			normalized.ExpiresAt,
		)
		if insertErr != nil {
			return gerror.Wrap(insertErr, "insert pending value grant")
		}
		result = projectValueGrant(&valueGrantRow{
			GrantNo: grantNo, GrantType: normalized.GrantType, CampaignNo: normalized.CampaignNo,
			BenefitNo: normalized.BenefitNo, BenefitSnapshotDigest: digest,
			CustomerNo: normalized.CustomerNo, CurrencyCode: normalized.CurrencyCode,
			SourceReference: normalized.SourceReference, OriginalAmountMinor: normalized.AmountMinor,
			PendingAmountMinor: normalized.AmountMinor, Withdrawable: normalized.Withdrawable,
			GlobalQuotaReserved: quota.global != nil, CustomerQuotaReserved: quota.perCustomer != nil,
			ExpiresAt: normalized.ExpiresAt, Status: "pending",
		}, false)
		return nil
	})
	return valueGrantResult(result, err)
}

func (s *valueGrantServiceImpl) Post(
	ctx context.Context,
	input ValueGrantPostInput,
) (*ValueGrantProjection, error) {
	normalized, err := normalizeValueGrantPost(input)
	if err != nil {
		return nil, err
	}
	var result *ValueGrantProjection
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		grant, loadErr := loadValueGrant(ctx, s.database, normalized.GrantNo, normalized.CustomerNo, true)
		if loadErr != nil {
			return loadErr
		}
		if grant == nil {
			return bizerr.NewCode(CodeValueGrantConflict)
		}
		if grant.Status != "pending" {
			if grant.FinancialPostingReference != nil &&
				*grant.FinancialPostingReference == normalized.FinancialPostingReference {
				result = projectValueGrant(grant, true)
				return nil
			}
			return bizerr.NewCode(CodeValueGrantConflict)
		}
		if grant.ExpiresAt != nil {
			valid, timeErr := s.database.GetValue(ctx, `SELECT ? > CURRENT_TIMESTAMP`, grant.ExpiresAt)
			if timeErr != nil {
				return gerror.Wrap(timeErr, "validate value grant posting expiry")
			}
			if !valid.Bool() {
				return bizerr.NewCode(CodeValueGrantConflict)
			}
		}
		amount := grant.PendingAmountMinor
		updateResult, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_value_grant
SET pending_amount_minor = 0, available_amount_minor = ?, status_code = 'available',
    financial_posting_reference = ?, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND grant_no = ? AND customer_no = ?
  AND status_code = 'pending' AND pending_amount_minor = ?`,
			amount, normalized.FinancialPostingReference, grant.GrantNo, grant.CustomerNo, amount,
		)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "post value grant")
		}
		if affectedErr := requireOneAffected(updateResult, "post value grant"); affectedErr != nil {
			return affectedErr
		}
		if quotaErr := transitionCampaignCounters(
			ctx, s.database, grant.CampaignNo, grant.CustomerNo,
			grant.GlobalQuotaReserved, grant.CustomerQuotaReserved, true,
		); quotaErr != nil {
			return quotaErr
		}
		if _, entryErr := s.insertValueEntry(ctx, valueEntryInsert{
			GrantNo: grant.GrantNo, CustomerNo: grant.CustomerNo, CurrencyCode: grant.CurrencyCode,
			EntryType: "post", PendingDeltaMinor: -amount, AvailableDeltaMinor: amount,
			SourceType: "activity", SourceReference: normalized.FinancialPostingReference,
		}); entryErr != nil {
			return entryErr
		}
		grant.PendingAmountMinor = 0
		grant.AvailableAmountMinor = amount
		grant.Status = "available"
		grant.FinancialPostingReference = &normalized.FinancialPostingReference
		result = projectValueGrant(grant, false)
		return nil
	})
	return valueGrantResult(result, err)
}

func (s *valueGrantServiceImpl) CancelPending(
	ctx context.Context,
	input ValueGrantMutationInput,
) (*ValueGrantProjection, error) {
	return s.mutate(ctx, input, "cancel")
}

func (s *valueGrantServiceImpl) Freeze(
	ctx context.Context,
	input ValueGrantMutationInput,
) (*ValueGrantProjection, error) {
	return s.mutate(ctx, input, "freeze")
}

func (s *valueGrantServiceImpl) Release(
	ctx context.Context,
	input ValueGrantMutationInput,
) (*ValueGrantProjection, error) {
	return s.mutate(ctx, input, "release")
}

func (s *valueGrantServiceImpl) Consume(
	ctx context.Context,
	input ValueGrantMutationInput,
) (*ValueGrantProjection, error) {
	return s.mutate(ctx, input, "consume")
}

func (s *valueGrantServiceImpl) Expire(
	ctx context.Context,
	input ValueGrantMutationInput,
) (*ValueGrantProjection, error) {
	return s.mutate(ctx, input, "expire")
}

func (s *valueGrantServiceImpl) Reverse(
	ctx context.Context,
	input ValueGrantMutationInput,
) (*ValueGrantProjection, error) {
	return s.mutate(ctx, input, "reverse")
}

func (s *valueGrantServiceImpl) mutate(
	ctx context.Context,
	input ValueGrantMutationInput,
	operation string,
) (*ValueGrantProjection, error) {
	normalized, err := normalizeValueGrantMutation(input, operation)
	if err != nil {
		return nil, err
	}
	var result *ValueGrantProjection
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		grant, loadErr := loadValueGrant(ctx, s.database, normalized.GrantNo, normalized.CustomerNo, true)
		if loadErr != nil {
			return loadErr
		}
		if grant == nil {
			return bizerr.NewCode(CodeValueGrantConflict)
		}
		replayed, replayErr := loadValueEntryBySource(
			ctx, s.database, grant.GrantNo, normalized.SourceType, normalized.SourceReference,
		)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			if !sameValueMutation(replayed, operation, normalized.AmountMinor) {
				return bizerr.NewCode(CodeValueGrantConflict)
			}
			result = projectValueGrant(grant, true)
			return nil
		}
		entry, nextStatus, mutationErr := buildValueMutation(ctx, s.database, grant, normalized, operation)
		if mutationErr != nil {
			return mutationErr
		}
		entry.GrantNo = grant.GrantNo
		entry.CustomerNo = grant.CustomerNo
		entry.CurrencyCode = grant.CurrencyCode
		entry.SourceType = normalized.SourceType
		entry.SourceReference = normalized.SourceReference
		updateResult, updateErr := s.database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_value_grant
SET pending_amount_minor = pending_amount_minor + ?,
    available_amount_minor = available_amount_minor + ?,
    frozen_amount_minor = frozen_amount_minor + ?,
    consumed_amount_minor = consumed_amount_minor + ?,
    expired_amount_minor = expired_amount_minor + ?,
    reversed_amount_minor = reversed_amount_minor + ?,
    status_code = ?, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND grant_no = ? AND customer_no = ?
  AND pending_amount_minor + ? >= 0 AND available_amount_minor + ? >= 0
  AND frozen_amount_minor + ? >= 0 AND consumed_amount_minor + ? >= 0`,
			entry.PendingDeltaMinor, entry.AvailableDeltaMinor, entry.FrozenDeltaMinor,
			entry.ConsumedDeltaMinor, entry.ExpiredDeltaMinor, entry.ReversedDeltaMinor,
			nextStatus, grant.GrantNo, grant.CustomerNo,
			entry.PendingDeltaMinor, entry.AvailableDeltaMinor,
			entry.FrozenDeltaMinor, entry.ConsumedDeltaMinor,
		)
		if updateErr != nil {
			return gerror.Wrap(updateErr, "apply value grant "+operation)
		}
		if affectedErr := requireOneAffected(updateResult, "apply value grant "+operation); affectedErr != nil {
			return affectedErr
		}
		if operation == "cancel" {
			if quotaErr := transitionCampaignCounters(
				ctx, s.database, grant.CampaignNo, grant.CustomerNo,
				grant.GlobalQuotaReserved, grant.CustomerQuotaReserved, false,
			); quotaErr != nil {
				return quotaErr
			}
		}
		if _, entryErr := s.insertValueEntry(ctx, entry); entryErr != nil {
			return entryErr
		}
		applyValueEntry(grant, entry)
		grant.Status = nextStatus
		result = projectValueGrant(grant, false)
		return nil
	})
	return valueGrantResult(result, err)
}

func buildValueMutation(
	ctx context.Context,
	database gdb.DB,
	grant *valueGrantRow,
	input ValueGrantMutationInput,
	operation string,
) (valueEntryInsert, string, error) {
	entry := valueEntryInsert{EntryType: operation}
	if operation == "cancel" {
		if grant.Status != "pending" || input.AmountMinor != grant.PendingAmountMinor {
			return entry, "", bizerr.NewCode(CodeValueGrantConflict)
		}
		entry.PendingDeltaMinor = -input.AmountMinor
		entry.ReversedDeltaMinor = input.AmountMinor
		return entry, "cancelled", nil
	}
	if grant.Status == "pending" || grant.Status == "cancelled" || grant.FinancialPostingReference == nil {
		return entry, "", bizerr.NewCode(CodeValueGrantConflict)
	}
	databaseTime, timeErr := database.GetValue(ctx, `SELECT CURRENT_TIMESTAMP`)
	if timeErr != nil {
		return entry, "", gerror.Wrap(timeErr, "read value grant mutation database time")
	}
	databaseNow := databaseTime.Time()
	expired := grant.ExpiresAt != nil && !grant.ExpiresAt.After(databaseNow)
	switch operation {
	case "freeze":
		if expired || input.AmountMinor > grant.AvailableAmountMinor {
			return entry, "", bizerr.NewCode(CodeValueGrantConflict)
		}
		entry.AvailableDeltaMinor = -input.AmountMinor
		entry.FrozenDeltaMinor = input.AmountMinor
	case "release":
		if input.AmountMinor > grant.FrozenAmountMinor {
			return entry, "", bizerr.NewCode(CodeValueGrantConflict)
		}
		entry.FrozenDeltaMinor = -input.AmountMinor
		if expired {
			entry.EntryType = "expire"
			entry.ExpiredDeltaMinor = input.AmountMinor
		} else {
			entry.AvailableDeltaMinor = input.AmountMinor
		}
	case "consume":
		if input.AmountMinor > grant.FrozenAmountMinor {
			return entry, "", bizerr.NewCode(CodeValueGrantConflict)
		}
		entry.FrozenDeltaMinor = -input.AmountMinor
		entry.ConsumedDeltaMinor = input.AmountMinor
	case "expire":
		if !expired || input.AmountMinor > grant.AvailableAmountMinor {
			return entry, "", bizerr.NewCode(CodeValueGrantConflict)
		}
		entry.AvailableDeltaMinor = -input.AmountMinor
		entry.ExpiredDeltaMinor = input.AmountMinor
	case "reverse":
		capacity := grant.AvailableAmountMinor + grant.FrozenAmountMinor + grant.ConsumedAmountMinor
		if input.AmountMinor > capacity {
			return entry, "", bizerr.NewCode(CodeValueGrantConflict)
		}
		fromAvailable := minInt64(grant.AvailableAmountMinor, input.AmountMinor)
		remaining := input.AmountMinor - fromAvailable
		fromFrozen := minInt64(grant.FrozenAmountMinor, remaining)
		fromConsumed := remaining - fromFrozen
		entry.AvailableDeltaMinor = -fromAvailable
		entry.FrozenDeltaMinor = -fromFrozen
		entry.ConsumedDeltaMinor = -fromConsumed
		entry.ReversedDeltaMinor = input.AmountMinor
		postEntry, postErr := loadValuePostEntry(ctx, database, grant.GrantNo)
		if postErr != nil {
			return entry, "", postErr
		}
		if postEntry == nil {
			return entry, "", bizerr.NewCode(CodeValueGrantConflict)
		}
		entry.ReversalOfEntryNo = postEntry.EntryNo
	default:
		return entry, "", bizerr.NewCode(CodeValueGrantInvalid)
	}
	available := grant.AvailableAmountMinor + entry.AvailableDeltaMinor
	frozen := grant.FrozenAmountMinor + entry.FrozenDeltaMinor
	consumed := grant.ConsumedAmountMinor + entry.ConsumedDeltaMinor
	expiredAmount := grant.ExpiredAmountMinor + entry.ExpiredDeltaMinor
	reversed := grant.ReversedAmountMinor + entry.ReversedDeltaMinor
	return entry, valueGrantStatus(available, frozen, consumed, expiredAmount, reversed), nil
}

func (s *valueGrantServiceImpl) insertValueEntry(
	ctx context.Context,
	input valueEntryInsert,
) (string, error) {
	entryNo, err := newMarketingNumber(s.random, "vent_")
	if err != nil {
		return "", err
	}
	var reversal any
	if input.ReversalOfEntryNo != "" {
		reversal = input.ReversalOfEntryNo
	}
	_, err = s.database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_value_entry (
    tenant_id, entry_no, grant_no, customer_no, currency_code, entry_type_code,
    pending_delta_minor, available_delta_minor, frozen_delta_minor,
    consumed_delta_minor, expired_delta_minor, reversed_delta_minor,
    source_type_code, source_reference, reversal_of_entry_no
) VALUES (0, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		entryNo, input.GrantNo, input.CustomerNo, input.CurrencyCode, input.EntryType,
		input.PendingDeltaMinor, input.AvailableDeltaMinor, input.FrozenDeltaMinor,
		input.ConsumedDeltaMinor, input.ExpiredDeltaMinor, input.ReversedDeltaMinor,
		input.SourceType, input.SourceReference, reversal,
	)
	if err != nil {
		return "", gerror.Wrap(err, "insert value grant entry")
	}
	return entryNo, nil
}

func loadValueGrant(
	ctx context.Context,
	database gdb.DB,
	grantNo string,
	customerNo string,
	lock bool,
) (*valueGrantRow, error) {
	query := valueGrantSelect + ` WHERE tenant_id = 0 AND grant_no = ? AND customer_no = ?`
	if lock {
		query += ` FOR UPDATE`
	}
	var grant *valueGrantRow
	if err := database.GetScan(ctx, &grant, query, grantNo, customerNo); err != nil {
		return nil, gerror.Wrap(err, "load value grant")
	}
	return grant, nil
}

func lockValueGrantCustomer(ctx context.Context, database gdb.DB, customerNo string) error {
	locked, err := database.GetValue(ctx, `
SELECT customer_no FROM plugin_idc_core_customer_subject
WHERE tenant_id = 0 AND customer_no = ? AND status_code = 'active'
FOR UPDATE`, customerNo)
	if err != nil {
		return gerror.Wrap(err, "lock value grant customer")
	}
	if locked.String() != customerNo {
		return bizerr.NewCode(CodeValueGrantConflict)
	}
	return nil
}

func loadValueGrantBySource(
	ctx context.Context,
	database gdb.DB,
	grantType string,
	customerNo string,
	sourceReference string,
	lock bool,
) (*valueGrantRow, error) {
	query := valueGrantSelect + `
WHERE tenant_id = 0 AND grant_type_code = ? AND customer_no = ? AND source_reference = ?`
	if lock {
		query += ` FOR UPDATE`
	}
	var grant *valueGrantRow
	if err := database.GetScan(ctx, &grant, query, grantType, customerNo, sourceReference); err != nil {
		return nil, gerror.Wrap(err, "load value grant by source")
	}
	return grant, nil
}

func loadValueGrantRule(
	ctx context.Context,
	database gdb.DB,
	input ValueGrantCreateInput,
) (*valueGrantRuleRow, error) {
	var rule *valueGrantRuleRow
	if err := database.GetScan(ctx, &rule, `
SELECT c.campaign_no, c.campaign_type_code, c.target_segment_no,
       c.target_snapshot_no, c.priority, c.stack_policy_code, c.stack_group_code,
       c.global_quota_limit, c.per_customer_quota_limit, c.version AS campaign_version,
       b.benefit_no, b.benefit_type_code, b.currency_code, b.amount_minor,
       b.rate_basis_points, b.maximum_discount_minor, b.minimum_spend_minor,
       b.purchase_phase_code, b.cycle_scope_code, b.cycle_count,
       b.version AS benefit_version
FROM plugin_idc_core_marketing_campaign c
JOIN plugin_idc_core_marketing_benefit_rule b
  ON b.campaign_no = c.campaign_no AND b.campaign_type_code = c.campaign_type_code
WHERE c.tenant_id = 0 AND b.tenant_id = 0
  AND c.campaign_no = ? AND c.campaign_type_code = ?
  AND b.benefit_no = ? AND b.benefit_type_code = ?
  AND b.currency_code = ? AND c.status_code = 'active' AND b.status_code = 'active'
  AND c.starts_at <= CURRENT_TIMESTAMP AND c.ends_at > CURRENT_TIMESTAMP
FOR UPDATE OF c, b`, input.CampaignNo, input.GrantType, input.BenefitNo,
		input.GrantType, input.CurrencyCode); err != nil {
		return nil, gerror.Wrap(err, "lock value grant benefit")
	}
	return rule, nil
}

const valueGrantSelect = `
SELECT grant_no, grant_type_code, campaign_no, customer_no, currency_code,
       benefit_no, benefit_snapshot_digest, source_reference,
       original_amount_minor, pending_amount_minor,
       available_amount_minor, frozen_amount_minor, consumed_amount_minor,
       expired_amount_minor, reversed_amount_minor, withdrawable, expires_at,
       global_quota_reserved, customer_quota_reserved, status_code,
       financial_posting_reference
FROM plugin_idc_core_marketing_value_grant`

func loadValueEntryBySource(
	ctx context.Context,
	database gdb.DB,
	grantNo string,
	sourceType string,
	sourceReference string,
) (*valueEntryRow, error) {
	var entry *valueEntryRow
	if err := database.GetScan(ctx, &entry, `
SELECT entry_no, entry_type_code, pending_delta_minor, available_delta_minor,
       frozen_delta_minor, consumed_delta_minor, expired_delta_minor,
       reversed_delta_minor, reversal_of_entry_no
FROM plugin_idc_core_marketing_value_entry
WHERE tenant_id = 0 AND grant_no = ? AND source_type_code = ? AND source_reference = ?
FOR UPDATE`, grantNo, sourceType, sourceReference); err != nil {
		return nil, gerror.Wrap(err, "load value grant entry by source")
	}
	return entry, nil
}

func loadValuePostEntry(
	ctx context.Context,
	database gdb.DB,
	grantNo string,
) (*valueEntryRow, error) {
	var entry *valueEntryRow
	if err := database.GetScan(ctx, &entry, `
SELECT entry_no, entry_type_code, pending_delta_minor, available_delta_minor,
       frozen_delta_minor, consumed_delta_minor, expired_delta_minor,
       reversed_delta_minor, reversal_of_entry_no
FROM plugin_idc_core_marketing_value_entry
WHERE tenant_id = 0 AND grant_no = ? AND entry_type_code = 'post'
ORDER BY id ASC LIMIT 1`, grantNo); err != nil {
		return nil, gerror.Wrap(err, "load value grant post entry")
	}
	return entry, nil
}

func normalizeValueGrantCreate(input ValueGrantCreateInput) (ValueGrantCreateInput, error) {
	input.GrantType = strings.TrimSpace(input.GrantType)
	input.CampaignNo = strings.TrimSpace(input.CampaignNo)
	input.BenefitNo = strings.TrimSpace(input.BenefitNo)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	if (input.GrantType != ValueGrantTypeRechargeBonus && input.GrantType != ValueGrantTypeCashback) ||
		!identifierPattern.MatchString(input.CampaignNo) || !identifierPattern.MatchString(input.BenefitNo) ||
		!identifierPattern.MatchString(input.CustomerNo) ||
		!currencyPattern.MatchString(input.CurrencyCode) || !validBusinessReference(input.SourceReference) ||
		input.AmountMinor <= 0 || input.AmountMinor > maximumValueGrantAmountMinor ||
		(input.GrantType == ValueGrantTypeRechargeBonus && input.Withdrawable) {
		return ValueGrantCreateInput{}, bizerr.NewCode(CodeValueGrantInvalid)
	}
	if input.ExpiresAt != nil {
		normalized := input.ExpiresAt.UTC().Truncate(time.Microsecond)
		input.ExpiresAt = &normalized
	}
	return input, nil
}

func normalizeValueGrantPost(input ValueGrantPostInput) (ValueGrantPostInput, error) {
	input.GrantNo = strings.TrimSpace(input.GrantNo)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.FinancialPostingReference = strings.TrimSpace(input.FinancialPostingReference)
	if !identifierPattern.MatchString(input.GrantNo) || !identifierPattern.MatchString(input.CustomerNo) ||
		!validBusinessReference(input.FinancialPostingReference) {
		return ValueGrantPostInput{}, bizerr.NewCode(CodeValueGrantInvalid)
	}
	return input, nil
}

func normalizeValueGrantMutation(
	input ValueGrantMutationInput,
	operation string,
) (ValueGrantMutationInput, error) {
	input.GrantNo = strings.TrimSpace(input.GrantNo)
	input.CustomerNo = strings.TrimSpace(input.CustomerNo)
	input.SourceType = strings.TrimSpace(input.SourceType)
	input.SourceReference = strings.TrimSpace(input.SourceReference)
	if !identifierPattern.MatchString(input.GrantNo) || !identifierPattern.MatchString(input.CustomerNo) ||
		!validValueSource(input.SourceType) || !validBusinessReference(input.SourceReference) ||
		input.AmountMinor <= 0 || input.AmountMinor > maximumValueGrantAmountMinor {
		return ValueGrantMutationInput{}, bizerr.NewCode(CodeValueGrantInvalid)
	}
	if operation == "expire" && input.SourceType != "expiry" {
		return ValueGrantMutationInput{}, bizerr.NewCode(CodeValueGrantInvalid)
	}
	if operation == "reverse" && input.SourceType != "refund" &&
		input.SourceType != "manual" && input.SourceType != "financial" {
		return ValueGrantMutationInput{}, bizerr.NewCode(CodeValueGrantInvalid)
	}
	if operation == "cancel" && input.SourceType != "activity" && input.SourceType != "manual" {
		return ValueGrantMutationInput{}, bizerr.NewCode(CodeValueGrantInvalid)
	}
	return input, nil
}

func validValueSource(value string) bool {
	return value == "order" || value == "refund" || value == "activity" ||
		value == "manual" || value == "migration" || value == "expiry" ||
		value == "financial"
}

func sameValueGrantCreate(grant *valueGrantRow, input ValueGrantCreateInput) bool {
	if grant.GrantType != input.GrantType || grant.CampaignNo != input.CampaignNo ||
		grant.BenefitNo != input.BenefitNo ||
		grant.CustomerNo != input.CustomerNo || grant.CurrencyCode != input.CurrencyCode ||
		grant.SourceReference != input.SourceReference || grant.OriginalAmountMinor != input.AmountMinor ||
		grant.Withdrawable != input.Withdrawable {
		return false
	}
	if grant.ExpiresAt == nil || input.ExpiresAt == nil {
		return grant.ExpiresAt == nil && input.ExpiresAt == nil
	}
	return grant.ExpiresAt.UTC().Equal(input.ExpiresAt.UTC())
}

func sameValueMutation(entry *valueEntryRow, operation string, amount int64) bool {
	if operation == "release" && entry.EntryType == "expire" {
		return entry.ExpiredDeltaMinor == amount && entry.FrozenDeltaMinor == -amount
	}
	if entry.EntryType != operation {
		return false
	}
	switch operation {
	case "cancel":
		return entry.PendingDeltaMinor == -amount && entry.ReversedDeltaMinor == amount
	case "freeze":
		return entry.AvailableDeltaMinor == -amount && entry.FrozenDeltaMinor == amount
	case "release":
		return entry.AvailableDeltaMinor == amount && entry.FrozenDeltaMinor == -amount
	case "consume":
		return entry.FrozenDeltaMinor == -amount && entry.ConsumedDeltaMinor == amount
	case "expire":
		return entry.AvailableDeltaMinor == -amount && entry.ExpiredDeltaMinor == amount
	case "reverse":
		return entry.ReversedDeltaMinor == amount &&
			-entry.AvailableDeltaMinor-entry.FrozenDeltaMinor-entry.ConsumedDeltaMinor == amount
	default:
		return false
	}
}

func applyValueEntry(grant *valueGrantRow, entry valueEntryInsert) {
	grant.PendingAmountMinor += entry.PendingDeltaMinor
	grant.AvailableAmountMinor += entry.AvailableDeltaMinor
	grant.FrozenAmountMinor += entry.FrozenDeltaMinor
	grant.ConsumedAmountMinor += entry.ConsumedDeltaMinor
	grant.ExpiredAmountMinor += entry.ExpiredDeltaMinor
	grant.ReversedAmountMinor += entry.ReversedDeltaMinor
}

func valueGrantStatus(available int64, frozen int64, consumed int64, expired int64, reversed int64) string {
	if available+frozen > 0 {
		return "available"
	}
	if consumed > 0 {
		return "exhausted"
	}
	if expired > 0 {
		return "expired"
	}
	if reversed > 0 {
		return "reversed"
	}
	return "pending"
}

func projectValueGrant(grant *valueGrantRow, reused bool) *ValueGrantProjection {
	postingReference := ""
	if grant.FinancialPostingReference != nil {
		postingReference = *grant.FinancialPostingReference
	}
	return &ValueGrantProjection{
		GrantNo: grant.GrantNo, GrantType: grant.GrantType, CampaignNo: grant.CampaignNo,
		BenefitNo: grant.BenefitNo, BenefitSnapshotDigest: grant.BenefitSnapshotDigest,
		CustomerNo: grant.CustomerNo, CurrencyCode: grant.CurrencyCode,
		OriginalAmountMinor: grant.OriginalAmountMinor, PendingAmountMinor: grant.PendingAmountMinor,
		AvailableAmountMinor: grant.AvailableAmountMinor, FrozenAmountMinor: grant.FrozenAmountMinor,
		ConsumedAmountMinor: grant.ConsumedAmountMinor, ExpiredAmountMinor: grant.ExpiredAmountMinor,
		ReversedAmountMinor: grant.ReversedAmountMinor, Withdrawable: grant.Withdrawable,
		Status: grant.Status, FinancialPostingReference: postingReference,
		ExpiresAt: grant.ExpiresAt, Reused: reused,
	}
}

func valueGrantResult(result *ValueGrantProjection, err error) (*ValueGrantProjection, error) {
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeValueGrantUnavailable)
}
