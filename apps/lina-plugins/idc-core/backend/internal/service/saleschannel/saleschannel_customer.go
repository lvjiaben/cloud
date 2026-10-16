// This file exposes customer-owned referrals and commission records. Customer
// queries exclude staff accounts, and the old settlement application is retired.
package saleschannel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
)

type CustomerService interface {
	LoadCustomerSnapshot(ctx context.Context, customerNo string) (CustomerSnapshot, error)
	LoadCustomerReferral(ctx context.Context, customerNo string) (CustomerReferralOverview, error)
	CreateCustomerReferralLink(ctx context.Context, customerNo string, input CustomerReferralLinkRequest) (CustomerReferralLinkResult, error)
	LoadCustomerCommissionOverview(ctx context.Context, customerNo, currencyCode string, limit int) (CustomerCommissionOverview, error)
	ListCustomerCommissionSettlements(ctx context.Context, customerNo, currencyCode string, limit int) (CustomerCommissionSettlementPage, error)
	RequestCustomerCommissionSettlement(ctx context.Context, customerNo string, input CustomerCommissionSettlementRequest) (CustomerCommissionSettlementResult, error)
}

type CustomerSnapshot struct {
	Links      []CustomerReferralLink
	Currencies []CustomerCommissionCurrency
}

type CustomerReferralLink struct{ LinkNo, Policy, Status string }

type CustomerCommissionCurrency struct {
	CurrencyCode                                         string
	PendingMinor, AvailableMinor, FrozenMinor, PaidMinor int64
}

type CustomerReferralOverview struct {
	ReferralCode, ReferralURL, AttributionPolicyCode string
	AttributionExpiresAt                             int64
	ReferredCustomerCount, AttributedOrderCount      int64
	QualifiedOrderCount, RejectedAttributionCount    int64
}

// CustomerReferralLinkRequest describes the public attribution choices. The
// raw referral code is never stored; the idempotency key is hashed and the
// resulting opaque link number is the only value exposed to the customer.
type CustomerReferralLinkRequest struct {
	PolicyCode               string
	AttributionWindowSeconds int
	ExpiresAt                *time.Time
	IdempotencyKey           string
}

type CustomerReferralLinkResult struct {
	Item   CustomerReferralLink
	Reused bool
}

type CustomerCommissionAccount struct {
	CurrencyCode                                                 string
	AvailableAmountMinor, PendingAmountMinor, MaturedAmountMinor int64
	SettledAmountMinor, FrozenAmountMinor                        int64
	StatusCode                                                   string
}

type CustomerCommissionEntry struct {
	EntryNo, CurrencyCode, StatusCode, SourceOrderNo string
	AmountMinor, ReversedAmountMinor                 int64
	MaturityAt, CreatedAt                            int64
}

type CustomerCommissionOverview struct {
	Accounts []CustomerCommissionAccount
	Entries  []CustomerCommissionEntry
}

type CustomerCommissionSettlement struct {
	SettlementNo, CurrencyCode, StatusCode, FailureReasonCode string
	AmountMinor                                               int64
	SubmittedAt, ReviewedAt, PaidAt                           int64
}

type CustomerCommissionSettlementPage struct {
	Items []CustomerCommissionSettlement
}

type CustomerCommissionSettlementRequest struct {
	CurrencyCode, IdempotencyKey string
	AmountMinor                  int64
}

type CustomerCommissionSettlementResult struct {
	Item   CustomerCommissionSettlement
	Reused bool
}

var _ CustomerService = (*serviceImpl)(nil)

func (s *serviceImpl) LoadCustomerSnapshot(ctx context.Context, customerNo string) (CustomerSnapshot, error) {
	if s.database == nil {
		return CustomerSnapshot{}, gerror.New("sales channel database is unavailable")
	}
	if ctx == nil || customerNo == "" {
		return CustomerSnapshot{}, gerror.New("customer sales context is invalid")
	}
	linksRows, err := s.database.GetAll(ctx, `SELECT link_no, attribution_policy_code, status_code FROM plugin_idc_core_sales_affiliate_link WHERE tenant_id=0 AND owner_customer_no=? ORDER BY id DESC LIMIT 100`, customerNo)
	if err != nil {
		return CustomerSnapshot{}, gerror.Wrap(err, "load customer referral links")
	}
	accountRows, err := s.database.GetAll(ctx, `SELECT currency_code, COALESCE(SUM(pending_amount_minor),0) AS pending_minor, COALESCE(SUM(available_amount_minor),0) AS available_minor, COALESCE(SUM(frozen_amount_minor),0) AS frozen_minor, COALESCE(SUM(paid_amount_minor),0) AS paid_minor FROM plugin_idc_core_sales_commission_account WHERE tenant_id=0 AND beneficiary_type_code IN ('agent','affiliate') AND beneficiary_reference=? GROUP BY currency_code ORDER BY currency_code`, customerNo)
	if err != nil {
		return CustomerSnapshot{}, gerror.Wrap(err, "load customer commission accounts")
	}
	snapshot := CustomerSnapshot{Links: make([]CustomerReferralLink, 0, len(linksRows)), Currencies: make([]CustomerCommissionCurrency, 0, len(accountRows))}
	for _, row := range linksRows {
		snapshot.Links = append(snapshot.Links, CustomerReferralLink{LinkNo: row["link_no"].String(), Policy: row["attribution_policy_code"].String(), Status: row["status_code"].String()})
	}
	for _, row := range accountRows {
		snapshot.Currencies = append(snapshot.Currencies, CustomerCommissionCurrency{CurrencyCode: row["currency_code"].String(), PendingMinor: row["pending_minor"].Int64(), AvailableMinor: row["available_minor"].Int64(), FrozenMinor: row["frozen_minor"].Int64(), PaidMinor: row["paid_minor"].Int64()})
	}
	return snapshot, nil
}

func (s *serviceImpl) LoadCustomerReferral(ctx context.Context, customerNo string) (CustomerReferralOverview, error) {
	if s.database == nil || ctx == nil || strings.TrimSpace(customerNo) == "" {
		return CustomerReferralOverview{}, gerror.New("customer sales context is invalid")
	}
	rows, err := s.database.GetAll(ctx, `SELECT link_no, attribution_policy_code, expires_at, (SELECT COUNT(*) FROM plugin_idc_core_sales_affiliate_attribution a WHERE a.tenant_id=0 AND a.link_no=l.link_no AND a.status_code IN ('eligible','rejected_self_referral','rejected_risk','revoked')) AS attributed_count, (SELECT COUNT(*) FROM plugin_idc_core_sales_affiliate_attribution a WHERE a.tenant_id=0 AND a.link_no=l.link_no AND a.status_code='eligible') AS qualified_count, (SELECT COUNT(DISTINCT referred_customer_no) FROM plugin_idc_core_sales_affiliate_attribution a WHERE a.tenant_id=0 AND a.link_no=l.link_no) AS referred_count, (SELECT COUNT(*) FROM plugin_idc_core_sales_affiliate_attribution a WHERE a.tenant_id=0 AND a.link_no=l.link_no AND a.status_code <> 'eligible') AS rejected_count FROM plugin_idc_core_sales_affiliate_link l WHERE l.tenant_id=0 AND l.owner_customer_no=? ORDER BY l.id DESC LIMIT 1`, customerNo)
	if err != nil {
		return CustomerReferralOverview{}, gerror.Wrap(err, "load customer referral overview")
	}
	if len(rows) == 0 {
		return CustomerReferralOverview{}, nil
	}
	row := rows[0]
	expiresAt := row["expires_at"].Time()
	return CustomerReferralOverview{
		ReferralCode: row["link_no"].String(), ReferralURL: "/r/" + row["link_no"].String(),
		AttributionPolicyCode: row["attribution_policy_code"].String(), AttributionExpiresAt: expiresAt.UnixMilli(),
		ReferredCustomerCount: row["referred_count"].Int64(), AttributedOrderCount: row["attributed_count"].Int64(),
		QualifiedOrderCount: row["qualified_count"].Int64(), RejectedAttributionCount: row["rejected_count"].Int64(),
	}, nil
}

// CreateCustomerReferralLink creates one idempotent public referral link for
// the authenticated customer. The database unique digest constraint makes
// retries and concurrent submissions converge on the same link.
func (s *serviceImpl) CreateCustomerReferralLink(ctx context.Context, customerNo string, input CustomerReferralLinkRequest) (CustomerReferralLinkResult, error) {
	if s.database == nil || ctx == nil || strings.TrimSpace(customerNo) == "" ||
		(input.PolicyCode != AffiliateFirstTouch && input.PolicyCode != AffiliateLastTouch) ||
		input.AttributionWindowSeconds < 60 || input.AttributionWindowSeconds > 31536000 || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 {
		return CustomerReferralLinkResult{}, gerror.New("customer referral link request is invalid")
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now()) {
		return CustomerReferralLinkResult{}, gerror.New("customer referral link expiry is invalid")
	}
	hash := sha256.Sum256([]byte(customerNo + "\x00" + input.IdempotencyKey))
	digest := hex.EncodeToString(hash[:])
	linkNo := "AFF_" + digest[:32]
	var result CustomerReferralLinkResult
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		rows, err := tx.GetAll(`SELECT link_no,attribution_policy_code,attribution_window_seconds,status_code FROM plugin_idc_core_sales_affiliate_link WHERE tenant_id=0 AND owner_customer_no=? AND code_digest=? LIMIT 1`, customerNo, digest)
		if err != nil {
			return err
		}
		if len(rows) > 0 {
			result.Reused = true
			result.Item = CustomerReferralLink{LinkNo: rows[0]["link_no"].String(), Policy: rows[0]["attribution_policy_code"].String(), Status: rows[0]["status_code"].String()}
			return nil
		}
		_, err = tx.Exec(`INSERT INTO plugin_idc_core_sales_affiliate_link (tenant_id,link_no,owner_customer_no,code_digest,attribution_policy_code,attribution_window_seconds,status_code,expires_at) VALUES (0,?,?,?,?,?,'active',?)`, linkNo, customerNo, digest, input.PolicyCode, input.AttributionWindowSeconds, input.ExpiresAt)
		if err != nil {
			return err
		}
		result.Item = CustomerReferralLink{LinkNo: linkNo, Policy: input.PolicyCode, Status: AffiliateLinkActive}
		return nil
	})
	if err != nil {
		return CustomerReferralLinkResult{}, gerror.Wrap(err, "create customer referral link")
	}
	return result, nil
}

func (s *serviceImpl) LoadCustomerCommissionOverview(ctx context.Context, customerNo, currencyCode string, limit int) (CustomerCommissionOverview, error) {
	if s.database == nil || ctx == nil || strings.TrimSpace(customerNo) == "" {
		return CustomerCommissionOverview{}, gerror.New("customer sales context is invalid")
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	accounts, err := s.database.GetAll(ctx, `SELECT currency_code, available_amount_minor, pending_amount_minor, credited_amount_minor - pending_amount_minor - reversed_amount_minor AS matured_amount_minor, paid_amount_minor AS settled_amount_minor, frozen_amount_minor, status_code FROM plugin_idc_core_sales_commission_account WHERE tenant_id=0 AND beneficiary_type_code IN ('agent','affiliate') AND beneficiary_reference=? AND (?='' OR currency_code=?) ORDER BY currency_code LIMIT ?`, customerNo, currencyCode, currencyCode, limit)
	if err != nil {
		return CustomerCommissionOverview{}, gerror.Wrap(err, "load customer commission accounts")
	}
	entries, err := s.database.GetAll(ctx, `SELECT e.entry_no,e.currency_code,e.entry_type_code,e.source_reference,e.delta_available_minor,e.delta_reversed_minor,e.occurred_at FROM plugin_idc_core_sales_commission_account_entry e JOIN plugin_idc_core_sales_commission_account a ON a.account_no=e.account_no AND a.currency_code=e.currency_code WHERE e.tenant_id=0 AND a.beneficiary_type_code IN ('agent','affiliate') AND a.beneficiary_reference=? AND (?='' OR e.currency_code=?) ORDER BY e.id DESC LIMIT ?`, customerNo, currencyCode, currencyCode, limit)
	if err != nil {
		return CustomerCommissionOverview{}, gerror.Wrap(err, "load customer commission entries")
	}
	result := CustomerCommissionOverview{Accounts: make([]CustomerCommissionAccount, 0, len(accounts)), Entries: make([]CustomerCommissionEntry, 0, len(entries))}
	for _, row := range accounts {
		result.Accounts = append(result.Accounts, CustomerCommissionAccount{CurrencyCode: row["currency_code"].String(), AvailableAmountMinor: row["available_amount_minor"].Int64(), PendingAmountMinor: row["pending_amount_minor"].Int64(), MaturedAmountMinor: row["matured_amount_minor"].Int64(), SettledAmountMinor: row["settled_amount_minor"].Int64(), FrozenAmountMinor: row["frozen_amount_minor"].Int64(), StatusCode: row["status_code"].String()})
	}
	for _, row := range entries {
		result.Entries = append(result.Entries, CustomerCommissionEntry{EntryNo: row["entry_no"].String(), CurrencyCode: row["currency_code"].String(), StatusCode: row["entry_type_code"].String(), SourceOrderNo: row["source_reference"].String(), AmountMinor: row["delta_available_minor"].Int64(), ReversedAmountMinor: row["delta_reversed_minor"].Int64(), CreatedAt: row["occurred_at"].Time().UnixMilli()})
	}
	return result, nil
}

func (s *serviceImpl) ListCustomerCommissionSettlements(ctx context.Context, customerNo, currencyCode string, limit int) (CustomerCommissionSettlementPage, error) {
	if s.database == nil || ctx == nil || strings.TrimSpace(customerNo) == "" {
		return CustomerCommissionSettlementPage{}, gerror.New("customer sales context is invalid")
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	rows, err := s.database.GetAll(ctx, `SELECT settlement_no,currency_code,status_code,requested_amount_minor,requested_at,completed_at FROM (
 SELECT settlement_no,currency_code,requested_amount_minor,status_code,requested_at,completed_at FROM plugin_idc_core_sales_commission_settlement WHERE tenant_id=0 AND beneficiary_type_code IN ('agent','affiliate') AND beneficiary_reference=? AND (?='' OR currency_code=?)
 UNION ALL
 SELECT p.payout_no,p.currency_code,p.amount_minor,'paid',p.created_at,p.paid_at FROM plugin_idc_core_sales_commission_offline_payout p JOIN plugin_idc_core_sales_commission_account a ON a.account_no=p.account_no WHERE a.tenant_id=0 AND a.beneficiary_type_code IN ('agent','affiliate') AND a.beneficiary_reference=? AND (?='' OR p.currency_code=?)
 ) history ORDER BY requested_at DESC,settlement_no DESC LIMIT ?`, customerNo, currencyCode, currencyCode, customerNo, currencyCode, currencyCode, limit)

	if err != nil {
		return CustomerCommissionSettlementPage{}, gerror.Wrap(err, "load customer commission settlements")
	}
	result := CustomerCommissionSettlementPage{Items: make([]CustomerCommissionSettlement, 0, len(rows))}
	for _, row := range rows {
		completed := row["completed_at"].Time()
		result.Items = append(result.Items, CustomerCommissionSettlement{SettlementNo: row["settlement_no"].String(), CurrencyCode: row["currency_code"].String(), AmountMinor: row["requested_amount_minor"].Int64(), StatusCode: row["status_code"].String(), SubmittedAt: row["requested_at"].Time().UnixMilli(), PaidAt: completed.UnixMilli()})
	}
	return result, nil
}

// RequestCustomerCommissionSettlement rejects the retired customer approval workflow without freezing money.
func (s *serviceImpl) RequestCustomerCommissionSettlement(context.Context, string, CustomerCommissionSettlementRequest) (CustomerCommissionSettlementResult, error) {
	return CustomerCommissionSettlementResult{}, bizerr.NewCode(codeSettlementRetired)
}
