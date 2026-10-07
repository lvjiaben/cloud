// This file owns exact-permission, audited risk-review decisions. Risk review
// is independent from inventory confirmation and fulfillment review.
package commerce

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const (
	permissionOrderRiskReviewApprove = "idc:admin:commerce:risk-review:approve"
	permissionOrderRiskReviewReject  = "idc:admin:commerce:risk-review:reject"

	OrderRiskReviewDecisionApproved = "approved"
	OrderRiskReviewDecisionRejected = "rejected"
)

var (
	CodeOrderRiskReviewInvalid = bizerr.MustDefine(
		"IDC_COMMERCE_RISK_REVIEW_INVALID",
		"The order risk review request is invalid",
		gcode.CodeInvalidParameter,
	)
	CodeOrderRiskReviewNotAccessible = bizerr.MustDefine(
		"IDC_COMMERCE_RISK_REVIEW_NOT_ACCESSIBLE",
		"The order risk review is not accessible",
		gcode.CodeNotAuthorized,
	)
	CodeOrderRiskReviewConflict = bizerr.MustDefine(
		"IDC_COMMERCE_RISK_REVIEW_CONFLICT",
		"The order risk review state changed; reload it and try again",
		gcode.CodeInvalidOperation,
	)
	CodeOrderRiskReviewUnavailable = bizerr.MustDefine(
		"IDC_COMMERCE_RISK_REVIEW_UNAVAILABLE",
		"The order risk review is temporarily unavailable",
		gcode.CodeInternalError,
	)
)

// OrderRiskReviewAuthorizer keeps approval and rejection behind separate exact
// permissions and derives the actor from trusted host context.
type OrderRiskReviewAuthorizer interface {
	AuthorizeRiskReviewApprove(ctx context.Context, reviewNo string) (actorUserID int, err error)
	AuthorizeRiskReviewReject(ctx context.Context, reviewNo string) (actorUserID int, err error)
}

// OrderRiskReviewDecisionInput contains only stable request and digested
// evidence identifiers. Raw evidence never enters commerce persistence.
type OrderRiskReviewDecisionInput struct {
	ReviewNo       string
	RequestNo      string
	DecisionCode   string
	EvidenceDigest string
}

// OrderRiskReviewDecision is the bounded committed review projection.
type OrderRiskReviewDecision struct {
	ReviewNo    string
	OrderNo     string
	CustomerNo  string
	StatusCode  string
	ReasonCode  string
	ReviewedBy  int
	ReviewedAt  time.Time
	OrderStatus string
	Reused      bool
}

// OrderRiskReviewService exposes one decision command. Exact authorization is
// selected from the normalized decision before any target data is read.
type OrderRiskReviewService interface {
	DecideRiskReview(context.Context, OrderRiskReviewDecisionInput) (*OrderRiskReviewDecision, error)
}

type orderRiskReviewServiceImpl struct {
	database   gdb.DB
	audit      idccap.BusinessAuditWriter
	authorizer OrderRiskReviewAuthorizer
	cancel     cancellationDependencies
}

var _ OrderRiskReviewService = (*orderRiskReviewServiceImpl)(nil)

// NewOrderRiskReviewService creates the audited staff risk-review owner.
func NewOrderRiskReviewService(
	database gdb.DB,
	auditWriter idccap.BusinessAuditWriter,
	authorizer OrderRiskReviewAuthorizer,
) (OrderRiskReviewService, error) {
	if isNilOrderRiskDependency(database) {
		return nil, gerror.New("order risk review requires a database")
	}
	marketingService, err := marketing.New(database, marketing.Config{})
	if err != nil {
		return nil, gerror.Wrap(err, "create order risk review marketing service")
	}
	evaluator, err := NewMarketingQuoteAdjustmentEvaluator(marketingService)
	if err != nil {
		return nil, err
	}
	reverser, ok := evaluator.(QuoteReservationReverser)
	if !ok {
		return nil, gerror.New("order risk review marketing reverser is unavailable")
	}
	outboxWriter, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		return nil, gerror.Wrap(err, "create order risk review outbox service")
	}
	return newOrderRiskReviewServiceWithDependencies(
		database, auditWriter, authorizer, reverser, outboxWriter,
	)
}

func newOrderRiskReviewServiceWithDependencies(
	database gdb.DB,
	auditWriter idccap.BusinessAuditWriter,
	authorizer OrderRiskReviewAuthorizer,
	reverser QuoteReservationReverser,
	outboxWriter checkoutEventWriter,
) (OrderRiskReviewService, error) {
	if isNilOrderRiskDependency(database) || isNilOrderRiskDependency(auditWriter) ||
		isNilOrderRiskDependency(authorizer) || isNilOrderRiskDependency(outboxWriter) {
		return nil, gerror.New("order risk review requires database, audit, authorization, and outbox services")
	}
	hasReverser := !isNilOrderRiskDependency(reverser)
	if !hasReverser {
		reverser = noQuoteReservationReverser{}
	}
	return &orderRiskReviewServiceImpl{
		database: database, audit: auditWriter, authorizer: authorizer,
		cancel: cancellationDependencies{
			reverser: reverser, hasReverser: hasReverser, outbox: outboxWriter,
		},
	}, nil
}

func (s *orderRiskReviewServiceImpl) DecideRiskReview(
	ctx context.Context,
	request OrderRiskReviewDecisionInput,
) (*OrderRiskReviewDecision, error) {
	input, err := normalizeOrderRiskReviewDecision(request)
	if err != nil {
		return nil, err
	}
	actorUserID, err := s.authorizeDecision(ctx, input)
	if err != nil {
		return nil, err
	}
	if actorUserID < 1 {
		return nil, bizerr.NewCode(CodeOrderRiskReviewNotAccessible)
	}
	var result *OrderRiskReviewDecision
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		result, err = s.decideInTransaction(ctx, tx, input, actorUserID)
		return err
	})
	if err == nil {
		return result, nil
	}
	if _, ok := bizerr.As(err); ok {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeOrderRiskReviewUnavailable)
}

func (s *orderRiskReviewServiceImpl) authorizeDecision(
	ctx context.Context,
	input orderRiskReviewInput,
) (int, error) {
	if input.decision == OrderRiskReviewDecisionApproved {
		return s.authorizer.AuthorizeRiskReviewApprove(ctx, input.reviewNo)
	}
	return s.authorizer.AuthorizeRiskReviewReject(ctx, input.reviewNo)
}

type orderRiskReviewInput struct {
	reviewNo       string
	requestNo      string
	decision       string
	evidenceDigest string
}

func normalizeOrderRiskReviewDecision(
	request OrderRiskReviewDecisionInput,
) (orderRiskReviewInput, error) {
	reviewNo := strings.TrimSpace(request.ReviewNo)
	requestNo := strings.TrimSpace(request.RequestNo)
	decision := strings.TrimSpace(request.DecisionCode)
	evidenceDigest := strings.TrimSpace(request.EvidenceDigest)
	if reviewNo != request.ReviewNo || requestNo != request.RequestNo ||
		decision != request.DecisionCode || evidenceDigest != request.EvidenceDigest ||
		!orderLifecycleIdentifierPattern.MatchString(reviewNo) || len(reviewNo) > 40 ||
		!checkoutIdempotencyPattern.MatchString(requestNo) ||
		(decision != OrderRiskReviewDecisionApproved && decision != OrderRiskReviewDecisionRejected) ||
		!quoteDigestPattern.MatchString(evidenceDigest) {
		return orderRiskReviewInput{}, bizerr.NewCode(CodeOrderRiskReviewInvalid)
	}
	return orderRiskReviewInput{
		reviewNo: reviewNo, requestNo: requestNo, decision: decision,
		evidenceDigest: evidenceDigest,
	}, nil
}

func (s *orderRiskReviewServiceImpl) decideInTransaction(
	ctx context.Context,
	tx gdb.TX,
	input orderRiskReviewInput,
	actorUserID int,
) (*OrderRiskReviewDecision, error) {
	review, err := lockOrderRiskReview(ctx, tx, input.reviewNo)
	if err != nil {
		return nil, err
	}
	if review == nil {
		return nil, bizerr.NewCode(CodeOrderRiskReviewNotAccessible)
	}
	requestDigest, err := orderRiskReviewRequestDigest(review, input)
	if err != nil {
		return nil, err
	}
	if review.StatusCode != "pending" {
		if riskReviewReplayMatches(review, input, requestDigest, actorUserID) {
			order, loadErr := lockOrderByNumber(ctx, tx, review.OrderNo)
			if loadErr != nil || order == nil || order.CustomerNo != review.CustomerNo {
				return nil, bizerr.NewCode(CodeOrderRiskReviewConflict)
			}
			result := projectOrderRiskReview(review, order.StatusCode, true)
			return result, s.appendRiskReviewAudit(ctx, input, result)
		}
		return nil, bizerr.NewCode(CodeOrderRiskReviewConflict)
	}
	order, err := lockOrderByNumber(ctx, tx, review.OrderNo)
	if err != nil {
		return nil, err
	}
	if order == nil || order.CustomerNo != review.CustomerNo || order.StatusCode != "manual_review" ||
		order.RiskDecisionCode != "manual_review" {
		return nil, bizerr.NewCode(CodeOrderRiskReviewConflict)
	}
	now, err := checkoutDatabaseNow(ctx, tx)
	if err != nil {
		return nil, err
	}
	if order.PaymentDueAt == nil || !order.PaymentDueAt.After(now) {
		return nil, bizerr.NewCode(CodeOrderRiskReviewConflict)
	}
	reason := "risk_approved"
	if input.decision == OrderRiskReviewDecisionRejected {
		reason = "risk_rejected"
	}
	updated, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_commerce_order_review
SET status_code = ?, reason_code = ?, request_digest = ?, evidence_digest = ?,
    reviewed_by = ?, reviewed_at = ?, updated_at = ?
WHERE tenant_id = 0 AND review_no = ? AND status_code = 'pending'`,
		input.decision, reason, requestDigest, input.evidenceDigest,
		actorUserID, now, now, review.ReviewNo,
	)
	if err != nil {
		return nil, gerror.Wrap(err, "record order risk review decision")
	}
	if err = requireOrderLifecycleAffected(updated, "record order risk review decision"); err != nil {
		return nil, err
	}

	orderStatus := order.StatusCode
	if input.decision == OrderRiskReviewDecisionApproved {
		otherGates, gateErr := countOtherManualOrderGates(ctx, tx, order.OrderNo)
		if gateErr != nil {
			return nil, gateErr
		}
		if otherGates == 0 {
			orderStatus = "pending_payment"
		}
		updated, err = tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_commerce_order
SET risk_decision_code = 'approved', status_code = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND order_no = ? AND status_code = 'manual_review'
  AND risk_decision_code = 'manual_review'`, orderStatus, now, order.OrderNo)
		if err != nil {
			return nil, gerror.Wrap(err, "approve order risk review")
		}
		if err = requireOrderLifecycleAffected(updated, "approve order risk review"); err != nil {
			return nil, err
		}
	} else {
		cancellationDigest, digestErr := orderCancellationRequestDigest(
			order.CustomerNo, order.OrderNo, OrderCancellationSourceAdmin,
			OrderCancellationReasonRiskRejected, input.requestNo,
		)
		if digestErr != nil {
			return nil, digestErr
		}
		_, err = initiateOrderCancellation(ctx, tx, order, orderCancellationInput{
			orderNo: order.OrderNo, source: OrderCancellationSourceAdmin,
			reason: OrderCancellationReasonRiskRejected, requestDigest: cancellationDigest,
			requestedBy: actorUserID,
		}, s.cancel)
		if err != nil {
			return nil, err
		}
		orderStatus = "cancelled"
	}
	review, err = lockOrderRiskReview(ctx, tx, input.reviewNo)
	if err != nil {
		return nil, err
	}
	result := projectOrderRiskReview(review, orderStatus, false)
	if err = s.appendRiskReviewAudit(ctx, input, result); err != nil {
		return nil, err
	}
	return result, nil
}

func lockOrderRiskReview(
	ctx context.Context,
	tx gdb.TX,
	reviewNo string,
) (*entity.CommerceOrderReview, error) {
	var review *entity.CommerceOrderReview
	if err := tx.Model(dao.CommerceOrderReview.Table()).Ctx(ctx).Where(do.CommerceOrderReview{
		TenantId: idccap.DefaultTenantID, ReviewNo: reviewNo, ReviewTypeCode: "risk",
	}).LockUpdate().Scan(&review); err != nil {
		return nil, gerror.Wrap(err, "lock order risk review")
	}
	return review, nil
}

func countOtherManualOrderGates(ctx context.Context, tx gdb.TX, orderNo string) (int64, error) {
	value, err := tx.Ctx(ctx).GetValue(`
SELECT COUNT(*)
FROM plugin_idc_core_commerce_order_item
WHERE tenant_id = 0 AND order_no = ?
  AND fulfillment_mode_code = 'manual_review'`, orderNo)
	if err != nil {
		return 0, gerror.Wrap(err, "count independent order manual gates")
	}
	return value.Int64(), nil
}

func orderRiskReviewRequestDigest(
	review *entity.CommerceOrderReview,
	input orderRiskReviewInput,
) (string, error) {
	return quoteContentDigest(struct {
		ReviewNo     string `json:"reviewNo"`
		OrderNo      string `json:"orderNo"`
		CustomerNo   string `json:"customerNo"`
		DecisionCode string `json:"decisionCode"`
		RequestNo    string `json:"requestNo"`
	}{review.ReviewNo, review.OrderNo, review.CustomerNo, input.decision, input.requestNo})
}

func riskReviewReplayMatches(
	review *entity.CommerceOrderReview,
	input orderRiskReviewInput,
	requestDigest string,
	actorUserID int,
) bool {
	return review != nil && review.RequestDigest != nil && review.EvidenceDigest != nil &&
		review.ReviewedBy != nil && review.ReviewedAt != nil &&
		review.StatusCode == input.decision && *review.RequestDigest == requestDigest &&
		*review.EvidenceDigest == input.evidenceDigest && *review.ReviewedBy == actorUserID
}

func projectOrderRiskReview(
	review *entity.CommerceOrderReview,
	orderStatus string,
	reused bool,
) *OrderRiskReviewDecision {
	if review == nil || review.ReviewedBy == nil || review.ReviewedAt == nil {
		return nil
	}
	return &OrderRiskReviewDecision{
		ReviewNo: review.ReviewNo, OrderNo: review.OrderNo, CustomerNo: review.CustomerNo,
		StatusCode: review.StatusCode, ReasonCode: review.ReasonCode,
		ReviewedBy: *review.ReviewedBy, ReviewedAt: *review.ReviewedAt,
		OrderStatus: orderStatus, Reused: reused,
	}
}

func (s *orderRiskReviewServiceImpl) appendRiskReviewAudit(
	ctx context.Context,
	input orderRiskReviewInput,
	result *OrderRiskReviewDecision,
) error {
	if result == nil {
		return gerror.New("order risk review audit projection is unavailable")
	}
	permission := permissionOrderRiskReviewApprove
	if input.decision == OrderRiskReviewDecisionRejected {
		permission = permissionOrderRiskReviewReject
	}
	if err := s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
		IdempotencyScope: "commerce.order.risk-review." + input.decision,
		IdempotencyKey:   input.requestNo,
		OperationCode:    "commerce.order.risk-review." + input.decision,
		TargetType:       "commerce_order_review", TargetNo: result.ReviewNo,
		CustomerNo: result.CustomerNo, ResultCode: "succeeded",
		ReasonCode: result.ReasonCode, RiskCode: "high",
		RequiredPermission: permission, CorrelationNo: result.OrderNo,
		BeforeSnapshot: map[string]any{"state": "pending", "order_status": "manual_review"},
		AfterSnapshot: map[string]any{
			"state": result.StatusCode, "order_status": result.OrderStatus,
			"evidence_digest": input.evidenceDigest,
		},
	}); err != nil {
		return gerror.Wrap(err, "append order risk review audit")
	}
	return nil
}

func isNilOrderRiskDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
