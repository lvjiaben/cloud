// This file defines the customer refund contract, immutable projections, and service construction.

package billing

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const maximumRefundSelections = 100

// RefundReason is the fixed customer-visible reason code.
type RefundReason string

// Refund target and reason constants form the closed online-refund input vocabulary.
const (
	RefundReasonDuplicatePurchase  RefundReason = "duplicate_purchase"
	RefundReasonWrongConfiguration RefundReason = "wrong_configuration"
	RefundReasonServiceIssue       RefundReason = "service_issue"
	RefundReasonNoLongerNeeded     RefundReason = "no_longer_needed"
	RefundReasonOther              RefundReason = "other"
)

// RefundSelectionInput selects one bounded refundable target and amount.
type RefundSelectionInput struct {
	InvoiceItemNo  string
	EntitlementNos []string
	Quantity       int
	AmountMinor    int64
}

// RefundPreviewInput requests a current-facts refund preview for one order.
type RefundPreviewInput struct {
	OrderNo    string
	Selections []RefundSelectionInput
}

// RefundSubmissionInput commits the exact preview and customer reason.
type RefundSubmissionInput struct {
	PreviewDigest  string
	Reason         RefundReason
	ReasonDetail   string
	IdempotencyKey string
	Preview        RefundPreviewInput
}

// RefundBalanceConsentInput records the customer's decision for unsupported sources.
type RefundBalanceConsentInput struct {
	RefundNo        string
	ExpectedVersion int64
	Consent         bool
	IdempotencyKey  string
}

// RefundEntitlementImpact freezes the selected long-lived entitlement state.
type RefundEntitlementImpact struct {
	EntitlementNo       string
	OrderItemNo         string
	ProductNameZhCN     string
	ProductNameEnUS     string
	EntitlementVersion  int64
	StatusCode          string
	RequiresTermination bool
	ImpactDigest        string
}

// RefundSourceAllocation binds part of the refund to one original funding source.
type RefundSourceAllocation struct {
	PaymentAllocationNo         string
	PaymentSourceType           string
	PaymentSourceReference      string
	ExecutionOwner              string
	AmountMinor                 int64
	CreditExposureNo            string
	CreditComponent             CreditRefundComponent
	CreditRepaymentAllocationNo string
	CreditPlanDigest            string
	ContentDigest               string
}

// RefundPreviewItem projects one selected item with bounded impacts and source splits.
type RefundPreviewItem struct {
	InvoiceItemNo         string
	OrderItemNo           string
	Quantity              int
	AmountMinor           int64
	RefundableAmountMinor int64
	DescriptionZhCN       string
	DescriptionEnUS       string
	Impacts               []RefundEntitlementImpact
	Sources               []RefundSourceAllocation
	ImpactDigest          string
}

// RefundPreview is the immutable customer-confirmed financial and service impact projection.
type RefundPreview struct {
	CustomerNo                     string
	OrderNo                        string
	InvoiceNo                      string
	CurrencyCode                   string
	AmountMinor                    int64
	ApprovalMode                   string
	RequiresEntitlementTermination bool
	HasResourceImpact              bool
	HasDangerousOperation          bool
	HasFundAnomaly                 bool
	HighAmount                     bool
	PolicyNo                       string
	PolicyVersion                  int64
	Items                          []RefundPreviewItem
	ImpactDigest                   string
}

// OnlineRefund is the current authoritative workflow projection for one request.
type OnlineRefund struct {
	RefundNo                       string
	CustomerNo                     string
	CustomerCode                   string
	CustomerDisplayName            string
	CustomerPrimaryPhone           string
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
	SubmittedAt                    time.Time
	ApprovedAt                     *time.Time
	CompletedAt                    *time.Time
	Reused                         bool
}

// RefundPolicyInput defines one new currency-local review policy version.
type RefundPolicyInput struct {
	CurrencyCode             string
	HighAmountThresholdMinor int64
	AutomaticApprovalEnabled bool
	ConfiguredBy             int
}

// RefundPolicy is one immutable currency-local review policy version.
type RefundPolicy struct {
	PolicyNo                 string
	CurrencyCode             string
	Version                  int64
	HighAmountThresholdMinor int64
	AutomaticApprovalEnabled bool
	ConfiguredBy             int
	EffectiveAt              time.Time
}

// RefundDirectoryFilter defines bounded exact filters and a keyset cursor.
type RefundDirectoryFilter struct {
	Page       int
	CustomerNo string
	Status     string
	Cursor     string
	Limit      int
}

// RefundDirectoryPage contains one newest-first bounded page.
type RefundDirectoryPage struct {
	Total      int
	Refunds    []*OnlineRefund
	NextCursor string
}

// RefundDirectoryItem is one immutable selected-item snapshot for staff review.
type RefundDirectoryItem struct {
	RefundItemNo          string
	InvoiceItemNo         string
	OrderItemNo           string
	Quantity              int
	AmountMinor           int64
	RefundableAmountMinor int64
	DescriptionZhCN       string
	DescriptionEnUS       string
	Impacts               []RefundEntitlementImpact
}

// RefundDirectoryDetail combines one refund with its bounded immutable items.
type RefundDirectoryDetail struct {
	Refund *OnlineRefund
	Items  []RefundDirectoryItem
}

// AccountRefundService owns current-customer preview, submission, history, and consent behavior.
type AccountRefundService interface {
	// ListOnlineRefunds returns one bounded newest-first page owned by the current customer.
	ListOnlineRefunds(context.Context, RefundDirectoryFilter) (*RefundDirectoryPage, error)
	// PreviewOnlineRefund validates current financial and service facts and returns a frozen impact preview.
	PreviewOnlineRefund(context.Context, RefundPreviewInput) (*RefundPreview, error)
	// SubmitOnlineRefund commits the exact accepted preview or safely replays the same request.
	SubmitOnlineRefund(context.Context, RefundSubmissionInput) (*OnlineRefund, error)
	// RespondRefundBalanceConsent records the current customer's mixed-source balance decision.
	RespondRefundBalanceConsent(context.Context, RefundBalanceConsentInput) (*OnlineRefund, error)
}

type onlineRefundService struct {
	database  gdb.DB
	principal principal.Reader
	audit     idccap.BusinessAuditWriter
	credit    CreditUsageService
	random    io.Reader
}

var _ AccountRefundService = (*onlineRefundService)(nil)

func NewAccountRefundService(
	database gdb.DB,
	principalReader principal.Reader,
	auditWriter idccap.BusinessAuditWriter,
	creditRefunds CreditUsageService,
) (AccountRefundService, error) {
	if isNilFinanceDirectoryDependency(database) || isNilFinanceDirectoryDependency(principalReader) ||
		isNilFinanceDirectoryDependency(auditWriter) ||
		isNilFinanceDirectoryDependency(creditRefunds) {
		return nil, gerror.New("online refund service requires database, customer principal, audit writer, and credit refunds")
	}
	return &onlineRefundService{
		database: database, principal: principalReader, audit: auditWriter,
		credit: creditRefunds, random: rand.Reader,
	}, nil
}

func (s *onlineRefundService) PreviewOnlineRefund(ctx context.Context, input RefundPreviewInput) (*RefundPreview, error) {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return nil, bizerr.NewCode(CodeRefundNotAccessible)
	}
	normalized, err := normalizeRefundPreviewInput(input)
	if err != nil {
		return nil, err
	}
	preview, err := s.previewForCustomer(ctx, audience.CustomerNo, normalized, false)
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	return preview, nil
}

func (s *onlineRefundService) SubmitOnlineRefund(ctx context.Context, input RefundSubmissionInput) (*OnlineRefund, error) {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return nil, bizerr.NewCode(CodeRefundNotAccessible)
	}
	normalizedPreview, err := normalizeRefundPreviewInput(input.Preview)
	if err != nil {
		return nil, err
	}
	reason, detail, keyDigest, requestErr := normalizeRefundSubmission(input)
	if requestErr != nil {
		return nil, requestErr
	}
	var result *OnlineRefund
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		lockedInvoice, lockErr := s.loadRefundInvoice(
			ctx, audience.CustomerNo, normalizedPreview.OrderNo, true,
		)
		if lockErr != nil {
			return lockErr
		}
		if lockedInvoice == nil {
			return bizerr.NewCode(CodeRefundNotAccessible)
		}
		replayed, replayErr := s.loadRefundReplay(ctx, audience.CustomerNo, keyDigest)
		if replayErr != nil {
			return replayErr
		}
		if replayed != nil {
			if replayed.ImpactDigest != strings.TrimSpace(input.PreviewDigest) ||
				replayed.Reason != reason || replayed.ReasonDetail != detail {
				return bizerr.NewCode(CodeRefundConflict)
			}
			replayed.Reused = true
			result = replayed
			return nil
		}
		preview, previewErr := s.previewForCustomer(ctx, audience.CustomerNo, normalizedPreview, true)
		if previewErr != nil {
			return previewErr
		}
		if preview.ImpactDigest != strings.TrimSpace(input.PreviewDigest) {
			return bizerr.NewCode(CodeRefundConflict)
		}
		requestDigest, digestErr := contentDigest(struct {
			PreviewDigest string       `json:"previewDigest"`
			Reason        RefundReason `json:"reason"`
			ReasonDetail  string       `json:"reasonDetail"`
		}{preview.ImpactDigest, reason, detail})
		if digestErr != nil {
			return digestErr
		}
		created, createErr := s.insertRefundRequest(
			ctx, audience.UserID, preview, reason, detail, keyDigest, requestDigest,
		)
		if createErr != nil {
			return createErr
		}
		result = created
		return s.appendRefundAudit(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "billing.refund.submit", IdempotencyKey: created.RefundNo,
			OperationCode: "billing.refund.submit", TargetType: "refund_request",
			TargetNo: created.RefundNo, CustomerNo: audience.CustomerNo,
			ResultCode: "succeeded", ReasonCode: string(created.Reason), RiskCode: "high",
			CorrelationNo: created.OrderNo, AfterSnapshot: refundAuditSnapshot(created),
		})
	})
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	return result, nil
}

func normalizeRefundPreviewInput(input RefundPreviewInput) (RefundPreviewInput, error) {
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	if !validBusinessReference(input.OrderNo, 40) ||
		len(input.Selections) < 1 || len(input.Selections) > maximumRefundSelections {
		return RefundPreviewInput{}, bizerr.NewCode(CodeRefundInvalid)
	}
	seen := make(map[string]struct{}, len(input.Selections))
	for index := range input.Selections {
		selection := &input.Selections[index]
		selection.InvoiceItemNo = strings.TrimSpace(selection.InvoiceItemNo)
		entitlementSeen := make(map[string]struct{}, len(selection.EntitlementNos))
		for entitlementIndex := range selection.EntitlementNos {
			entitlementNo := strings.TrimSpace(selection.EntitlementNos[entitlementIndex])
			if !validBusinessReference(entitlementNo, 40) {
				return RefundPreviewInput{}, bizerr.NewCode(CodeRefundInvalid)
			}
			if _, duplicate := entitlementSeen[entitlementNo]; duplicate {
				return RefundPreviewInput{}, bizerr.NewCode(CodeRefundInvalid)
			}
			selection.EntitlementNos[entitlementIndex] = entitlementNo
			entitlementSeen[entitlementNo] = struct{}{}
		}
		sort.Strings(selection.EntitlementNos)
		if !validBusinessReference(selection.InvoiceItemNo, 40) || selection.AmountMinor <= 0 ||
			selection.Quantity < 1 || len(selection.EntitlementNos) > selection.Quantity {
			return RefundPreviewInput{}, bizerr.NewCode(CodeRefundInvalid)
		}
		if _, exists := seen[selection.InvoiceItemNo]; exists {
			return RefundPreviewInput{}, bizerr.NewCode(CodeRefundInvalid)
		}
		seen[selection.InvoiceItemNo] = struct{}{}
	}
	return input, nil
}

func normalizeRefundSubmission(input RefundSubmissionInput) (RefundReason, string, string, error) {
	detail := strings.TrimSpace(input.ReasonDetail)
	key := strings.TrimSpace(input.IdempotencyKey)
	validReason := input.Reason == RefundReasonDuplicatePurchase ||
		input.Reason == RefundReasonWrongConfiguration ||
		input.Reason == RefundReasonServiceIssue ||
		input.Reason == RefundReasonNoLongerNeeded ||
		input.Reason == RefundReasonOther
	if !validReason || len(detail) > 1000 || (input.Reason == RefundReasonOther && detail == "") ||
		len(key) < 8 || len(key) > 128 || !canonicalDigestPattern.MatchString(strings.TrimSpace(input.PreviewDigest)) {
		return "", "", "", bizerr.NewCode(CodeRefundInvalid)
	}
	digest := sha256.Sum256([]byte("online-refund-idempotency\x00" + key))
	return input.Reason, detail, hex.EncodeToString(digest[:]), nil
}

func normalizeOnlineRefundError(err error) error {
	if err == nil || bizerr.Is(err, CodeRefundInvalid) || bizerr.Is(err, CodeRefundNotAccessible) ||
		bizerr.Is(err, CodeRefundNotEligible) || bizerr.Is(err, CodeRefundConflict) ||
		bizerr.Is(err, CodeRefundIntegrity) {
		return err
	}
	return bizerr.WrapCode(err, CodeRefundUnavailable)
}

func refundExecutionOwner(sourceType string) string {
	switch sourceType {
	case "channel_payment":
		return "payment"
	case "balance", "credit":
		return "core"
	default:
		return "manual"
	}
}

func sortRefundSelections(input []RefundSelectionInput) {
	sort.SliceStable(input, func(i, j int) bool {
		return input[i].InvoiceItemNo < input[j].InvoiceItemNo
	})
}
