// Package saleschannel provides the shared domain rules for agent pricing.
package saleschannel

import (
	"context"
	"regexp"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

var customerIdentifierPattern = regexp.MustCompile(`^CUS_[0-9a-f]{32}$`)

// Service evaluates whether an agent may use a published product rule and
// calculates the price that must be frozen into the quote snapshot.
type Service interface {
	// Evaluate checks qualification, product scope, rule validity, margin and
	// credit before returning a deterministic agent offer and its snapshot.
	Evaluate(ctx context.Context, request OfferRequest) (Offer, error)
	// EvaluateStored loads the current bounded set of agent records and then
	// evaluates them using the same snapshot rules as Evaluate.
	EvaluateStored(ctx context.Context, customerNo, productNo, currencyCode string, basePriceMinor, costMinor, existingCreditUsed int64, now time.Time) (Offer, error)
	// Attribute validates one referral touch and returns an append-only decision.
	Attribute(ctx context.Context, request AttributionRequest) (AttributionDecision, error)
	// PersistAttribution validates and appends an eligible attribution exactly once.
	PersistAttribution(ctx context.Context, request AttributionRequest) (AttributionDecision, error)
	// PersistStoredAttribution loads the authoritative link, touch and verified
	// identity projections before persistence. Callers cannot supply identity facts.
	PersistStoredAttribution(ctx context.Context, now time.Time, linkNo, touchNo, referredCustomerNo string) (AttributionDecision, error)
	SelectAttribution(ctx context.Context, request AttributionSelectionRequest) (AttributionSelection, error)
	// CalculateCommission computes a capped, versioned commission snapshot.
	CalculateCommission(ctx context.Context, request CommissionRequest) (CommissionSnapshot, error)
	// Assign checks sales ownership protection before a transfer is appended.
	Assign(ctx context.Context, request AssignmentRequest) (AssignmentDecision, error)
	PlanAssignmentBatch(ctx context.Context, request AssignmentBatchRequest) (AssignmentBatchDecision, error)
	DecideMaturity(ctx context.Context, request MaturityRequest) (MaturityDecision, error)
	DecideSettlement(ctx context.Context, request SettlementRequest) (SettlementDecision, error)
	ReviewSettlement(ctx context.Context, request SettlementReviewRequest) error
	CreatePayoutBatch(ctx context.Context, request PayoutBatchRequest) (PayoutBatchResult, error)
	RecordPayoutResult(ctx context.Context, request PayoutResultRequest) error
	MatureDueCommissions(ctx context.Context, now time.Time, limit int) (int, error)
	ValidateQualificationTransition(ctx context.Context, request QualificationTransitionRequest) (QualificationTransitionDecision, error)
	LoadOverview(ctx context.Context) (Overview, error)
	LoadCustomerSnapshot(ctx context.Context, customerNo string) (CustomerSnapshot, error)
}

// Overview is a bounded, read-only summary for the sales operations page.
// Counts are kept separate so a later report never mixes currencies or scopes.
type Overview struct {
	Qualifications    int64
	AffiliateLinks    int64
	Attributions      int64
	PendingMaturity   int64
	PendingSettlement int64
	Currencies        []CurrencyOverview
}

// ManagementReferrer is the display-safe current eligible referral owner for
// one customer. Empty fields mean the customer has no eligible attribution.
type ManagementReferrer struct {
	CustomerNo   string
	CustomerCode string
	DisplayName  string
}

// ManagementReferrerReader is kept separate from the pricing service contract
// so management-only projections do not become a dependency of checkout.
type ManagementReferrerReader interface {
	LoadManagementReferrer(ctx context.Context, customerNo string) (ManagementReferrer, error)
}

type CurrencyOverview struct {
	CurrencyCode string
	AccruedMinor int64
	MaturedMinor int64
}

type serviceImpl struct {
	database gdb.DB
	bizCtx   bizctxcap.Service
}

var _ Service = (*serviceImpl)(nil)
var _ ManagementReferrerReader = (*serviceImpl)(nil)

func (s *serviceImpl) LoadManagementReferrer(ctx context.Context, customerNo string) (ManagementReferrer, error) {
	if s == nil || s.database == nil || ctx == nil || !customerIdentifierPattern.MatchString(customerNo) {
		return ManagementReferrer{}, gerror.New("management referrer request is invalid")
	}
	rows, err := s.database.GetAll(ctx, `SELECT a.referrer_customer_no,
 COALESCE(subject.customer_code::text, '') AS customer_code,
 COALESCE(subject.display_name, '') AS display_name
 FROM plugin_idc_core_sales_affiliate_attribution a
 LEFT JOIN plugin_idc_core_customer_subject subject
   ON subject.tenant_id=a.tenant_id AND subject.customer_no=a.referrer_customer_no
 WHERE a.tenant_id=0 AND a.referred_customer_no=? AND a.status_code='eligible'
 ORDER BY a.id DESC LIMIT 1`, customerNo)
	if err != nil {
		return ManagementReferrer{}, gerror.Wrap(err, "load management customer referrer")
	}
	if len(rows) == 0 {
		return ManagementReferrer{}, nil
	}
	result := ManagementReferrer{
		CustomerNo:   rows[0]["referrer_customer_no"].String(),
		CustomerCode: rows[0]["customer_code"].String(),
		DisplayName:  rows[0]["display_name"].String(),
	}
	if !customerIdentifierPattern.MatchString(result.CustomerNo) {
		return ManagementReferrer{}, gerror.New("management customer referrer is inconsistent")
	}
	return result, nil
}

// New creates the stateless sales-channel rule service for callers that already
// loaded records in a bounded query.
func New() Service { return &serviceImpl{} }

// NewWithDatabase creates a sales-channel service that can load the current
// qualification, product rule, price rule and credit limit from one database.
func NewWithDatabase(database gdb.DB, contexts ...bizctxcap.Service) (Service, error) {
	if database == nil {
		return nil, gerror.New("sales channel database is required")
	}
	var bizCtx bizctxcap.Service
	if len(contexts) > 0 {
		bizCtx = contexts[0]
	}
	return &serviceImpl{database: database, bizCtx: bizCtx}, nil
}

// QualificationStatus is the lifecycle state of an agent qualification.
type QualificationStatus string

const (
	// QualificationActive allows new agent quotes.
	QualificationActive QualificationStatus = "active"
	// QualificationSuspended keeps history but disallows new quotes.
	QualificationSuspended QualificationStatus = "suspended"
	// QualificationExpired marks a qualification past its validity window.
	QualificationExpired QualificationStatus = "expired"
	// QualificationRevoked marks a qualification explicitly withdrawn.
	QualificationRevoked QualificationStatus = "revoked"
)

// RuleStatus is the lifecycle state of a product or price rule.
type RuleStatus string

const (
	// RulePublished is the only rule state accepted for a new quote.
	RulePublished RuleStatus = "published"
)

// AdjustmentType describes how an agent price is derived from the public price.
type AdjustmentType string

const (
	// AdjustmentFixedPrice uses FixedPriceMinor as the quote amount.
	AdjustmentFixedPrice AdjustmentType = "fixed_price"
	// AdjustmentDiscountBasisPoints applies DiscountBasisPoints to the public price.
	AdjustmentDiscountBasisPoints AdjustmentType = "discount_basis_points"
	// CreditLimitActive allows the limit to be used for a new quote.
	CreditLimitActive = "active"
	// ReasonQualificationUnavailable explains an invalid agent qualification.
	ReasonQualificationUnavailable = "agent_qualification_unavailable"
	// ReasonProductNotEligible explains a product-scope rejection.
	ReasonProductNotEligible = "agent_product_not_eligible"
	// ReasonPriceRuleUnavailable explains an invalid or mismatched price rule.
	ReasonPriceRuleUnavailable = "agent_price_rule_unavailable"
	// ReasonCreditLimitUnavailable explains an invalid credit limit.
	ReasonCreditLimitUnavailable = "agent_credit_limit_unavailable"
	// ReasonCreditLimitExceeded explains a quote over the remaining limit.
	ReasonCreditLimitExceeded = "agent_credit_limit_exceeded"
)

// OfferRequest contains the already selected records and the public price
// needed to evaluate one quote. The caller obtains records in one bounded
// query and passes them here, so this service never creates N+1 lookups.
type OfferRequest struct {
	Now                time.Time
	ProductNo          string
	CurrencyCode       string
	BasePriceMinor     int64
	CostMinor          int64
	ExistingCreditUsed int64
	Qualification      *entity.SalesAgentQualification
	ProductRule        *entity.SalesAgentProductRule
	PriceRule          *entity.SalesAgentPriceRule
	CreditLimit        *entity.SalesAgentCreditLimit
}

// Offer is the frozen result of a successful agent pricing decision.
type Offer struct {
	Eligible             bool
	PriceMinor           int64
	RemainingCreditMinor *int64
	ReasonCode           string
	Snapshot             PriceSnapshot
}

// PriceSnapshot records rule identities and versions that must be copied into
// the order quote and never recalculated from later rule changes.
type PriceSnapshot struct {
	QualificationNo      string
	QualificationVersion int64
	ProductRuleNo        string
	ProductRuleVersion   int64
	PriceRuleNo          string
	PriceRuleVersion     int64
	CurrencyCode         string
	BasePriceMinor       int64
	PriceMinor           int64
}

func (s *serviceImpl) Evaluate(ctx context.Context, request OfferRequest) (Offer, error) {
	if ctx == nil {
		return Offer{}, gerror.New("sales channel evaluation context is required")
	}
	if err := ctx.Err(); err != nil {
		return Offer{}, gerror.Wrap(err, "sales channel evaluation context is unavailable")
	}
	if request.Now.IsZero() || request.ProductNo == "" || !validCurrencyCode(request.CurrencyCode) ||
		request.BasePriceMinor < 0 || request.CostMinor < 0 || request.ExistingCreditUsed < 0 {
		return Offer{}, gerror.New("sales channel offer request is invalid")
	}
	if request.Qualification == nil || !validWindow(request.Now, request.Qualification.EffectiveAt, request.Qualification.ExpiresAt) ||
		QualificationStatus(request.Qualification.StatusCode) != QualificationActive {
		return ineligible(ReasonQualificationUnavailable), nil
	}
	if request.ProductRule == nil || request.ProductRule.ProductNo != request.ProductNo ||
		RuleStatus(request.ProductRule.StatusCode) != RulePublished ||
		!validWindow(request.Now, request.ProductRule.EffectiveAt, request.ProductRule.ExpiresAt) {
		return ineligible(ReasonProductNotEligible), nil
	}
	if request.PriceRule == nil || request.PriceRule.CurrencyCode != request.CurrencyCode ||
		RuleStatus(request.PriceRule.StatusCode) != RulePublished ||
		!validWindow(request.Now, request.PriceRule.EffectiveAt, request.PriceRule.ExpiresAt) ||
		request.PriceRule.ProductRuleNo != request.ProductRule.ProductRuleNo {
		return ineligible(ReasonPriceRuleUnavailable), nil
	}

	price, err := calculatePrice(request.BasePriceMinor, request.CostMinor, request.PriceRule)
	if err != nil {
		return ineligible(err.Error()), nil
	}
	var remaining *int64
	if request.CreditLimit != nil {
		if request.CreditLimit.CurrencyCode != request.CurrencyCode ||
			request.CreditLimit.StatusCode != CreditLimitActive ||
			!validWindow(request.Now, request.CreditLimit.EffectiveAt, request.CreditLimit.ExpiresAt) {
			return ineligible(ReasonCreditLimitUnavailable), nil
		}
		available := request.CreditLimit.LimitAmountMinor - request.ExistingCreditUsed
		if available < price {
			return ineligible(ReasonCreditLimitExceeded), nil
		}
		remainingValue := available - price
		remaining = &remainingValue
	}
	snapshot := PriceSnapshot{
		QualificationNo:      request.Qualification.QualificationNo,
		QualificationVersion: request.Qualification.Version,
		ProductRuleNo:        request.ProductRule.ProductRuleNo,
		ProductRuleVersion:   request.ProductRule.Version,
		PriceRuleNo:          request.PriceRule.PriceRuleNo,
		PriceRuleVersion:     request.PriceRule.Version,
		CurrencyCode:         request.CurrencyCode,
		BasePriceMinor:       request.BasePriceMinor,
		PriceMinor:           price,
	}
	return Offer{Eligible: true, PriceMinor: price, RemainingCreditMinor: remaining, Snapshot: snapshot}, nil
}

func calculatePrice(basePrice, cost int64, rule *entity.SalesAgentPriceRule) (int64, error) {
	if basePrice < 0 || cost < 0 || rule.MinimumMarginMinor < 0 {
		return 0, gerror.New("agent_amount_invalid")
	}
	var price int64
	switch AdjustmentType(rule.AdjustmentTypeCode) {
	case AdjustmentFixedPrice:
		if rule.FixedPriceMinor == nil {
			return 0, gerror.New("agent_fixed_price_missing")
		}
		price = *rule.FixedPriceMinor
	case AdjustmentDiscountBasisPoints:
		if rule.DiscountBasisPoints == nil || *rule.DiscountBasisPoints < 0 || *rule.DiscountBasisPoints > 10000 {
			return 0, gerror.New("agent_discount_invalid")
		}
		var err error
		price, _, err = multiplyDivide(basePrice, int64(10000-*rule.DiscountBasisPoints), 10000)
		if err != nil {
			return 0, gerror.New("agent_amount_overflow")
		}
	default:
		return 0, gerror.New("agent_price_adjustment_invalid")
	}
	marginFloor, ok := safeAdd(cost, rule.MinimumMarginMinor)
	if !ok || price < 0 || price < marginFloor {
		return 0, gerror.New("agent_margin_floor_exceeded")
	}
	return price, nil
}

func safeAdd(left, right int64) (int64, bool) {
	if left < 0 || right < 0 || left > int64(^uint64(0)>>1)-right {
		return 0, false
	}
	return left + right, true
}

func validWindow(now time.Time, effective, expires *time.Time) bool {
	if effective == nil || now.Before(*effective) {
		return false
	}
	return expires == nil || now.Before(*expires)
}

func ineligible(reason string) Offer { return Offer{ReasonCode: reason} }

func validCurrencyCode(value string) bool {
	return len(value) == 3 && value[0] >= 'A' && value[0] <= 'Z' &&
		value[1] >= 'A' && value[1] <= 'Z' && value[2] >= 'A' && value[2] <= 'Z'
}
