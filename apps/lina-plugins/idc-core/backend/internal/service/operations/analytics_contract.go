// File analytics_contract.go contains the small, deterministic contracts used
// by operations reports, risk review, and bounded export planning.  It keeps
// report values in their original currency and never performs ledger writes.
package operations

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
)

// RiskEvent identifies the business event for which a merchant may configure
// a risk rule.  Values are deliberately strings so they can match dictionary
// values and remain explainable in audit records.
type RiskEvent string

const (
	RiskEventRegistration  RiskEvent = "registration"
	RiskEventIdentity      RiskEvent = "identity"
	RiskEventOrder         RiskEvent = "order"
	RiskEventPayment       RiskEvent = "payment"
	RiskEventRefund        RiskEvent = "refund"
	RiskEventIPChange      RiskEvent = "ip_change"
	RiskEventCommission    RiskEvent = "commission"
	RiskEventHighFrequency RiskEvent = "high_frequency"
)

// RiskRuleMatch is the explainable result of one enabled merchant rule.
type RiskRuleMatch struct {
	RuleNo           string
	EventType        string
	Action           string
	AmountMinor      int64
	AmountLimitMinor *int64
	EventCount       int
	EventLimit       int
	EvidenceCode     string
}

// EvaluateRiskRules evaluates all supplied rules in deterministic rule-number
// order.  Returning every match lets the review queue show all reasons instead
// of hiding a second risk behind the first one.
func EvaluateRiskRules(ctx context.Context, rules []RiskRule, input RiskInput) ([]RiskRuleMatch, error) {
	if ctx == nil {
		return nil, gerror.New("risk rules context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.EventType == "" || input.AmountMinor < 0 || input.EventCount < 0 {
		return nil, gerror.New("risk input is invalid")
	}
	if !validRiskEvent(input.EventType) {
		return nil, gerror.New("risk event is invalid")
	}
	ordered := append([]RiskRule(nil), rules...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].RuleNo < ordered[j].RuleNo })
	result := make([]RiskRuleMatch, 0, len(ordered))
	for _, rule := range ordered {
		if rule.RuleNo == "" || rule.EventType == "" || rule.EventType != input.EventType {
			continue
		}
		if !validRiskEvent(rule.EventType) {
			return nil, gerror.New("risk rule event is invalid")
		}
		if rule.MaxAmountMinor != nil && *rule.MaxAmountMinor < 0 || rule.MaxEvents < 0 {
			return nil, gerror.New("risk rule threshold is invalid")
		}
		if !validRiskAction(rule.Action) {
			return nil, gerror.New("risk rule action is invalid")
		}
		amountHit := rule.MaxAmountMinor != nil && input.AmountMinor > *rule.MaxAmountMinor
		eventsHit := rule.MaxEvents > 0 && input.EventCount > rule.MaxEvents
		if !amountHit && !eventsHit {
			continue
		}
		evidence := "threshold_exceeded"
		if amountHit && eventsHit {
			evidence = "amount_and_event_threshold_exceeded"
		} else if amountHit {
			evidence = "amount_threshold_exceeded"
		} else if eventsHit {
			evidence = "event_threshold_exceeded"
		}
		result = append(result, RiskRuleMatch{RuleNo: rule.RuleNo, EventType: rule.EventType, Action: rule.Action, AmountMinor: input.AmountMinor, AmountLimitMinor: rule.MaxAmountMinor, EventCount: input.EventCount, EventLimit: rule.MaxEvents, EvidenceCode: evidence})
	}
	return result, nil
}

func validRiskEvent(value string) bool {
	switch RiskEvent(value) {
	case RiskEventRegistration, RiskEventIdentity, RiskEventOrder, RiskEventPayment,
		RiskEventRefund, RiskEventIPChange, RiskEventCommission, RiskEventHighFrequency:
		return true
	default:
		return false
	}
}

func validRiskAction(value string) bool {
	switch value {
	case "allow", "remind", "restrict", "delay", "manual_review":
		return true
	default:
		return false
	}
}

// RiskReviewInput validates a manual review transition for an existing risk
// record.  It does not mutate the record; the owning transaction must persist
// the returned status and audit event together.
type RiskReviewInput struct {
	CurrentStatus string
	Decision      string
	Reason        string
}

// ValidateRiskReview accepts only the explicit lifecycle decisions supported
// by the risk record table.  A reason is mandatory for every human decision.
func ValidateRiskReview(input RiskReviewInput) (string, error) {
	if input.CurrentStatus != "open" && input.CurrentStatus != "acknowledged" {
		return "", gerror.New("risk record is not reviewable")
	}
	if input.Decision != "acknowledge" && input.Decision != "resolve" && input.Decision != "dismiss" {
		return "", gerror.New("risk review decision is invalid")
	}
	reason := strings.TrimSpace(input.Reason)
	if len([]rune(reason)) < 2 || len([]rune(reason)) > 2048 {
		return "", gerror.New("risk review reason is invalid")
	}
	switch input.Decision {
	case "acknowledge":
		return "acknowledged", nil
	case "resolve":
		return "resolved", nil
	default:
		return "dismissed", nil
	}
}

// ReportFact is one already-authorized, immutable source projection.  The
// caller loads this projection with database-side data-scope predicates; the
// aggregator itself performs no extra queries.
type ReportFact struct {
	OccurredAt      time.Time
	CustomerNo      string
	CurrencyCode    string
	RevenueMinor    int64
	ReceivableMinor int64
	PaidMinor       int64
	CashFlowMinor   int64
	BalanceMinor    int64
	RefundMinor     int64
	CostMinor       int64
	CostKnown       bool
	CustomerCount   int64
	OrderCount      int64
	ServiceCount    int64
	ProductCount    int64
	SalesCount      int64
	AffiliateCount  int64
	SupplierSuccess int64
	SupplierFailure int64
}

// ReportMetric is the report row returned for one original currency.
type ReportMetric struct {
	CurrencyCode     string
	RevenueMinor     int64
	ReceivableMinor  int64
	PaidMinor        int64
	CashFlowMinor    int64
	BalanceMinor     int64
	RefundMinor      int64
	CostMinor        int64
	CostKnown        bool
	GrossProfitMinor int64
	CustomerCount    int64
	OrderCount       int64
	ServiceCount     int64
	ProductCount     int64
	SalesCount       int64
	AffiliateCount   int64
	SupplierSuccess  int64
	SupplierFailure  int64
}

// AggregateReport performs one bounded in-memory pass over a projection that
// was loaded with the caller's data permission.  No currency is merged.
func AggregateReport(ctx context.Context, facts []ReportFact, from, to time.Time, currencyCode string) ([]ReportMetric, error) {
	if ctx == nil {
		return nil, gerror.New("report context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if from.IsZero() || to.IsZero() || from.After(to) || to.Sub(from) > 366*24*time.Hour {
		return nil, gerror.New("report range is invalid")
	}
	if currencyCode != "" {
		currencyCode = strings.ToUpper(strings.TrimSpace(currencyCode))
		if !validCurrencyCode(currencyCode) {
			return nil, gerror.New("report currency is invalid")
		}
	}
	byCurrency := make(map[string]ReportMetric)
	for _, fact := range facts {
		if fact.OccurredAt.Before(from) || !fact.OccurredAt.Before(to) || !validCurrencyCode(fact.CurrencyCode) {
			continue
		}
		if currencyCode != "" && fact.CurrencyCode != currencyCode {
			continue
		}
		if fact.RevenueMinor < 0 || fact.ReceivableMinor < 0 || fact.PaidMinor < 0 || fact.RefundMinor < 0 || fact.CostMinor < 0 {
			return nil, gerror.New("report amount is invalid")
		}
		metric, exists := byCurrency[fact.CurrencyCode]
		metric.CurrencyCode = fact.CurrencyCode
		if !exists {
			metric.CostKnown = fact.CostKnown
		}
		metric.RevenueMinor += fact.RevenueMinor
		metric.ReceivableMinor += fact.ReceivableMinor
		metric.PaidMinor += fact.PaidMinor
		metric.CashFlowMinor += fact.CashFlowMinor
		metric.BalanceMinor += fact.BalanceMinor
		metric.RefundMinor += fact.RefundMinor
		metric.CustomerCount += fact.CustomerCount
		metric.OrderCount += fact.OrderCount
		metric.ServiceCount += fact.ServiceCount
		metric.ProductCount += fact.ProductCount
		metric.SalesCount += fact.SalesCount
		metric.AffiliateCount += fact.AffiliateCount
		metric.SupplierSuccess += fact.SupplierSuccess
		metric.SupplierFailure += fact.SupplierFailure
		if fact.CostKnown {
			metric.CostMinor += fact.CostMinor
		} else {
			metric.CostKnown = false
		}
		byCurrency[fact.CurrencyCode] = metric
	}
	result := make([]ReportMetric, 0, len(byCurrency))
	for _, metric := range byCurrency {
		if metric.CostKnown {
			metric.GrossProfitMinor = metric.PaidMinor - metric.RefundMinor - metric.CostMinor
		}
		result = append(result, metric)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CurrencyCode < result[j].CurrencyCode })
	return result, nil
}

// ExchangeRate is an explicitly configured report-only rational rate.  It is
// never used for account, payment, refund, or ledger writes.
type ExchangeRate struct {
	FromCurrency string
	ToCurrency   string
	Numerator    int64
	Denominator  int64
}

// ConvertReportAmount rounds half up to the smallest unit of the target
// currency.  The overflow check makes an invalid configuration fail closed.
func ConvertReportAmount(amount int64, rate ExchangeRate) (int64, error) {
	if amount < 0 || !validCurrencyCode(strings.ToUpper(rate.FromCurrency)) || !validCurrencyCode(strings.ToUpper(rate.ToCurrency)) || rate.Numerator <= 0 || rate.Denominator <= 0 {
		return 0, gerror.New("report exchange rate is invalid")
	}
	if amount > math.MaxInt64/rate.Numerator {
		return 0, gerror.New("report exchange amount overflows")
	}
	product := amount * rate.Numerator
	if product > math.MaxInt64-rate.Denominator/2 {
		return 0, gerror.New("report exchange amount overflows")
	}
	return (product + rate.Denominator/2) / rate.Denominator, nil
}

// ConvertReportMetricForDisplay creates an optional, report-only converted
// copy of one metric. The caller keeps the original metric alongside this
// value, so currencies are never silently merged. Cash-flow and balance can
// legitimately be negative; they use the same half-up rate with a signed
// overflow guard. No database or ledger write is performed.
func ConvertReportMetricForDisplay(ctx context.Context, metric ReportMetric, rate ExchangeRate) (ReportMetric, error) {
	if ctx == nil {
		return ReportMetric{}, gerror.New("report conversion context is required")
	}
	if err := ctx.Err(); err != nil {
		return ReportMetric{}, err
	}
	if !validCurrencyCode(metric.CurrencyCode) || strings.ToUpper(rate.FromCurrency) != metric.CurrencyCode {
		return ReportMetric{}, gerror.New("report metric currency does not match exchange rate")
	}
	convert := func(amount int64) (int64, error) {
		if amount >= 0 {
			return ConvertReportAmount(amount, rate)
		}
		// Avoid negating MinInt64, which would overflow before conversion.
		if amount == math.MinInt64 {
			return 0, gerror.New("report exchange amount overflows")
		}
		value, err := ConvertReportAmount(-amount, rate)
		if err != nil {
			return 0, err
		}
		if value == math.MaxInt64 {
			return math.MinInt64, nil
		}
		return -value, nil
	}
	converted := metric
	converted.CurrencyCode = strings.ToUpper(rate.ToCurrency)
	var err error
	if converted.RevenueMinor, err = ConvertReportAmount(metric.RevenueMinor, rate); err != nil {
		return ReportMetric{}, err
	}
	if converted.ReceivableMinor, err = ConvertReportAmount(metric.ReceivableMinor, rate); err != nil {
		return ReportMetric{}, err
	}
	if converted.PaidMinor, err = ConvertReportAmount(metric.PaidMinor, rate); err != nil {
		return ReportMetric{}, err
	}
	if converted.RefundMinor, err = ConvertReportAmount(metric.RefundMinor, rate); err != nil {
		return ReportMetric{}, err
	}
	if converted.CostKnown {
		if converted.CostMinor, err = ConvertReportAmount(metric.CostMinor, rate); err != nil {
			return ReportMetric{}, err
		}
	}
	if converted.CashFlowMinor, err = convert(metric.CashFlowMinor); err != nil {
		return ReportMetric{}, err
	}
	if converted.BalanceMinor, err = convert(metric.BalanceMinor); err != nil {
		return ReportMetric{}, err
	}
	if converted.CostKnown {
		converted.GrossProfitMinor = converted.PaidMinor - converted.RefundMinor - converted.CostMinor
	} else {
		converted.GrossProfitMinor = 0
	}
	return converted, nil
}

// Scope is the authorized projection scope captured at query time.
type Scope struct {
	AllCustomers bool
	CustomerNos  []string
}

// FilterReportFacts applies the same scope semantics to an already loaded
// projection.  Production loaders must apply the equivalent predicate in SQL;
// this helper is useful for tests and non-database projections.
func FilterReportFacts(facts []ReportFact, scope Scope, customerByFact func(ReportFact) string) ([]ReportFact, error) {
	if customerByFact == nil {
		return nil, gerror.New("report scope resolver is required")
	}
	if scope.AllCustomers {
		return append([]ReportFact(nil), facts...), nil
	}
	allowed := make(map[string]struct{}, len(scope.CustomerNos))
	for _, customerNo := range scope.CustomerNos {
		customerNo = strings.TrimSpace(customerNo)
		if customerNo == "" || len(customerNo) > 40 {
			return nil, gerror.New("report scope customer is invalid")
		}
		allowed[customerNo] = struct{}{}
	}
	result := make([]ReportFact, 0, len(facts))
	for _, fact := range facts {
		if _, ok := allowed[customerByFact(fact)]; ok {
			result = append(result, fact)
		}
	}
	return result, nil
}

// BuildCustomerScopePredicate creates the SQL predicate that a production
// projection loader should append before aggregation.  It is intentionally
// small and parameterized: customer values never become SQL text.  An all-data
// scope must only be supplied after the host has already checked the exact
// management data-scope permission.
func BuildCustomerScopePredicate(column string, scope Scope) (string, []any, error) {
	column = strings.TrimSpace(column)
	if column == "" || len(column) > 64 {
		return "", nil, gerror.New("report scope column is invalid")
	}
	for _, r := range column {
		if !(r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return "", nil, gerror.New("report scope column is invalid")
		}
	}
	if scope.AllCustomers {
		return "TRUE", nil, nil
	}
	if len(scope.CustomerNos) == 0 || len(scope.CustomerNos) > 1024 {
		return "", nil, gerror.New("report scope is invalid")
	}
	args := make([]any, 0, len(scope.CustomerNos))
	placeholders := make([]string, 0, len(scope.CustomerNos))
	seen := make(map[string]struct{}, len(scope.CustomerNos))
	for _, customerNo := range scope.CustomerNos {
		customerNo = strings.TrimSpace(customerNo)
		if customerNo == "" || len(customerNo) > 40 {
			return "", nil, gerror.New("report scope customer is invalid")
		}
		if _, exists := seen[customerNo]; exists {
			continue
		}
		seen[customerNo] = struct{}{}
		placeholders = append(placeholders, "?")
		args = append(args, customerNo)
	}
	if len(args) == 0 {
		return "", nil, gerror.New("report scope is invalid")
	}
	return column + " IN (" + strings.Join(placeholders, ",") + ")", args, nil
}

// ExportPlan limits asynchronous report export before any file is generated.
type ExportPlan struct {
	Format         string
	MaxRows        int
	PermissionCode string
	Scope          Scope
}

// ValidateExportPlan enforces the report export boundary: XLSX only, finite
// rows, and an explicit permission plus data scope.
func ValidateExportPlan(plan ExportPlan) error {
	if plan.Format != "xlsx" || plan.MaxRows < 1 || plan.MaxRows > 100000 || strings.TrimSpace(plan.PermissionCode) == "" {
		return gerror.New("report export plan is invalid")
	}
	if !plan.Scope.AllCustomers && len(plan.Scope.CustomerNos) == 0 {
		return gerror.New("report export scope is empty")
	}
	return nil
}

// ExplainRiskMatch produces a short operator-facing explanation without raw
// request payloads, secrets, or provider text.
func ExplainRiskMatch(match RiskRuleMatch) string {
	if match.AmountLimitMinor != nil && match.EventLimit > 0 {
		return fmt.Sprintf("rule %s: amount %d exceeds %d or count %d exceeds %d", match.RuleNo, match.AmountMinor, *match.AmountLimitMinor, match.EventCount, match.EventLimit)
	}
	if match.AmountLimitMinor != nil {
		return fmt.Sprintf("rule %s: amount %d exceeds %d", match.RuleNo, match.AmountMinor, *match.AmountLimitMinor)
	}
	return fmt.Sprintf("rule %s: count %d exceeds %d", match.RuleNo, match.EventCount, match.EventLimit)
}
