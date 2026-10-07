// This file owns the customer-scoped, recoverable payment preparation gate.
// Identity remains customer/compliance-owned and is re-read on every call.
package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	PaymentPreparationAwaitingIdentity    = "awaiting_identity"
	PaymentPreparationAwaitingOrderReview = "awaiting_order_review"
	PaymentPreparationReady               = "ready"
	PaymentPreparationExpired             = "expired"
	PaymentPreparationCancelled           = "cancelled"
	PaymentPreparationClosed              = "closed"
)

const (
	PaymentPreparationIdentityUnverified = "identity_unverified"
	PaymentPreparationIdentityPending    = "identity_pending"
	PaymentPreparationIdentityRejected   = "identity_rejected"
	PaymentPreparationIdentityExpired    = "identity_expired"
	PaymentPreparationIdentityRevoked    = "identity_revoked"
	PaymentPreparationOrderReview        = "order_review_required"
	PaymentPreparationPaymentReady       = "payment_ready"
	PaymentPreparationPaymentExpired     = "payment_expired"
	PaymentPreparationOrderCancelled     = "order_cancelled"
	PaymentPreparationOrderNotPayable    = "order_not_payable"
)

// PaymentPreparation is a bounded recovery projection. RequiresContract is
// informational only and never participates in payment readiness.
type PaymentPreparation struct {
	ContextNo          string
	OrderNo            string
	InvoiceNo          string
	CustomerTypeCode   string
	StatusCode         string
	ReasonCode         string
	IdentityStatusCode string
	CurrencyCode       string
	AmountDueMinor     int64
	RequiresContract   bool
	PaymentDueAt       time.Time
	LastEvaluatedAt    time.Time
	Ready              bool
	Reused             bool
}

// PaymentPreparationService creates and resumes one customer-owned payment
// context. Payment owners must call it again immediately before creating an attempt.
type PaymentPreparationService interface {
	PrepareAccountOrderPayment(context.Context, string) (*PaymentPreparation, error)
	ResumeAccountOrderPayment(context.Context, string) (*PaymentPreparation, error)
}

type paymentPreparationServiceImpl struct {
	database  gdb.DB
	principal principal.Reader
}

var _ PaymentPreparationService = (*paymentPreparationServiceImpl)(nil)

// NewPaymentPreparationService creates the customer payment identity gate.
func NewPaymentPreparationService(
	database gdb.DB,
	principalReader principal.Reader,
) (PaymentPreparationService, error) {
	if isNilPaymentPreparationDependency(database) ||
		isNilPaymentPreparationDependency(principalReader) {
		return nil, gerror.New("payment preparation requires database and principal services")
	}
	return &paymentPreparationServiceImpl{database: database, principal: principalReader}, nil
}

func isNilPaymentPreparationDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func (s *paymentPreparationServiceImpl) PrepareAccountOrderPayment(
	ctx context.Context,
	orderNo string,
) (*PaymentPreparation, error) {
	orderNo = strings.TrimSpace(orderNo)
	if !orderLifecycleIdentifierPattern.MatchString(orderNo) || len(orderNo) > 40 {
		return nil, bizerr.NewCode(CodePaymentPreparationInvalid)
	}
	audience, err := currentPaymentPreparationAudience(ctx, s.principal)
	if err != nil {
		return nil, err
	}
	return s.evaluate(ctx, *audience, orderNo, "")
}

func (s *paymentPreparationServiceImpl) ResumeAccountOrderPayment(
	ctx context.Context,
	contextNo string,
) (*PaymentPreparation, error) {
	contextNo = strings.TrimSpace(contextNo)
	if !orderLifecycleIdentifierPattern.MatchString(contextNo) || len(contextNo) > 40 {
		return nil, bizerr.NewCode(CodePaymentPreparationInvalid)
	}
	audience, err := currentPaymentPreparationAudience(ctx, s.principal)
	if err != nil {
		return nil, err
	}
	var orderNo string
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		var reference *struct {
			OrderNo string `orm:"order_no"`
		}
		loadErr := tx.Model(dao.CommercePaymentContext.Table()).Ctx(ctx).
			Fields(dao.CommercePaymentContext.Columns().OrderNo).
			Where(do.CommercePaymentContext{
				TenantId: idccap.DefaultTenantID, ContextNo: contextNo,
				CustomerNo: audience.CustomerNo,
			}).Scan(&reference)
		if loadErr != nil {
			return loadErr
		}
		if reference != nil {
			orderNo = reference.OrderNo
		}
		return nil
	})
	if err != nil {
		return nil, normalizePaymentPreparationError(err)
	}
	if orderNo == "" {
		return nil, bizerr.NewCode(CodePaymentPreparationNotAccessible)
	}
	return s.evaluate(ctx, *audience, orderNo, contextNo)
}

func (s *paymentPreparationServiceImpl) evaluate(
	ctx context.Context,
	audience principal.Audience,
	orderNo string,
	expectedContextNo string,
) (*PaymentPreparation, error) {
	var result *PaymentPreparation
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		order, loadErr := lockOrderByNumber(ctx, tx, orderNo)
		if loadErr != nil {
			return loadErr
		}
		if order == nil || order.CustomerNo != audience.CustomerNo {
			return bizerr.NewCode(CodePaymentPreparationNotAccessible)
		}
		existing, loadErr := lockPaymentContextByOrder(ctx, tx, order.OrderNo)
		if loadErr != nil {
			return loadErr
		}
		if expectedContextNo != "" && (existing == nil || existing.ContextNo != expectedContextNo) {
			return bizerr.NewCode(CodePaymentPreparationNotAccessible)
		}
		result, loadErr = evaluatePaymentPreparation(ctx, tx, order, existing)
		return loadErr
	})
	if err != nil {
		return nil, normalizePaymentPreparationError(err)
	}
	return result, nil
}

func currentPaymentPreparationAudience(
	ctx context.Context,
	reader principal.Reader,
) (*principal.Audience, error) {
	audience, err := reader.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo == "" || audience.UserID < 1 {
		return nil, bizerr.NewCode(CodePaymentPreparationNotAccessible)
	}
	return audience, nil
}

type paymentIdentityProjection struct {
	StatusCode string
	CaseNo     string
	Version    int64
	VerifiedAt *time.Time
	ExpiresAt  *time.Time
}

type paymentPreparationDecision struct {
	status string
	reason string
}

func evaluatePaymentPreparation(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	existing *entity.CommercePaymentContext,
) (*PaymentPreparation, error) {
	now, err := checkoutDatabaseNow(ctx, tx)
	if err != nil {
		return nil, err
	}
	invoice, err := lockOrderInvoice(ctx, tx, order.OrderNo)
	if err != nil {
		return nil, err
	}
	if !paymentPreparationInvoiceMatches(order, invoice) {
		return nil, bizerr.NewCode(CodePaymentPreparationConflict)
	}
	subject, err := lockPaymentPreparationCustomer(ctx, tx, order.CustomerNo)
	if err != nil {
		return nil, err
	}
	if subject == nil || subject.StatusCode != "active" {
		return nil, bizerr.NewCode(CodePaymentPreparationNotAccessible)
	}
	identity, err := lockPaymentIdentity(ctx, tx, subject, now)
	if err != nil {
		return nil, err
	}
	allocations, err := countPaymentPreparationAllocations(ctx, tx, invoice.InvoiceNo)
	if err != nil {
		return nil, err
	}
	decision := decidePaymentPreparation(order, invoice, identity, allocations, now)
	contextRow, reused, err := persistPaymentPreparation(
		ctx, tx, order, invoice, subject, identity, existing, decision, now,
	)
	if err != nil {
		return nil, err
	}
	return projectPaymentPreparation(contextRow, order, invoice, reused)
}

func paymentPreparationInvoiceMatches(
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
) bool {
	return order != nil && invoice != nil && invoice.OrderNo != nil &&
		*invoice.OrderNo == order.OrderNo && invoice.CustomerNo == order.CustomerNo &&
		invoice.CurrencyCode == order.CurrencyCode && invoice.TotalAmountMinor == order.TotalAmountMinor &&
		invoice.PaymentDueAt != nil && order.PaymentDueAt != nil &&
		invoice.PaymentDueAt.Equal(*order.PaymentDueAt)
}

func lockPaymentPreparationCustomer(
	ctx context.Context,
	tx gdb.TX,
	customerNo string,
) (*entity.CustomerSubject, error) {
	var subject *entity.CustomerSubject
	if err := tx.Ctx(ctx).GetScan(&subject, `
SELECT id, tenant_id, customer_no, customer_type_code, status_code, version
FROM plugin_idc_core_customer_subject
WHERE tenant_id = 0 AND customer_no = ?
FOR SHARE`, customerNo); err != nil {
		return nil, gerror.Wrap(err, "lock payment preparation customer")
	}
	return subject, nil
}

func lockPaymentIdentity(
	ctx context.Context,
	tx gdb.TX,
	subject *entity.CustomerSubject,
	now time.Time,
) (paymentIdentityProjection, error) {
	identity := paymentIdentityProjection{StatusCode: "unverified"}
	switch subject.CustomerTypeCode {
	case "individual":
		var profile *entity.CustomerIndividualProfile
		if err := tx.Ctx(ctx).GetScan(&profile, `
SELECT identity_status_code, identity_case_no, identity_verified_at,
       identity_expires_at, version
FROM plugin_idc_core_customer_individual_profile
WHERE tenant_id = 0 AND customer_no = ?
FOR SHARE`, subject.CustomerNo); err != nil {
			return paymentIdentityProjection{}, gerror.Wrap(err, "lock individual payment identity")
		}
		if profile != nil {
			identity = paymentIdentityProjection{
				StatusCode: profile.IdentityStatusCode, CaseNo: profile.IdentityCaseNo,
				Version: profile.Version, VerifiedAt: profile.IdentityVerifiedAt,
				ExpiresAt: profile.IdentityExpiresAt,
			}
		}
	case "enterprise":
		var profile *entity.CustomerEnterpriseProfile
		if err := tx.Ctx(ctx).GetScan(&profile, `
SELECT verification_status_code, verification_case_no, verified_at,
       expires_at, version
FROM plugin_idc_core_customer_enterprise_profile
WHERE tenant_id = 0 AND customer_no = ?
FOR SHARE`, subject.CustomerNo); err != nil {
			return paymentIdentityProjection{}, gerror.Wrap(err, "lock enterprise payment identity")
		}
		if profile != nil {
			identity = paymentIdentityProjection{
				StatusCode: profile.VerificationStatusCode, CaseNo: profile.VerificationCaseNo,
				Version: profile.Version, VerifiedAt: profile.VerifiedAt, ExpiresAt: profile.ExpiresAt,
			}
		}
	default:
		return paymentIdentityProjection{}, gerror.New("payment customer type is unsupported")
	}
	if identity.StatusCode == "verified" && identity.ExpiresAt != nil && !identity.ExpiresAt.After(now) {
		identity.StatusCode = "expired"
	}
	return identity, nil
}

func countPaymentPreparationAllocations(
	ctx context.Context,
	tx gdb.TX,
	invoiceNo string,
) (int64, error) {
	value, err := tx.Ctx(ctx).GetValue(`
SELECT COUNT(*)
FROM plugin_idc_core_billing_payment_allocation
WHERE tenant_id = 0 AND invoice_no = ?`, invoiceNo)
	if err != nil {
		return 0, gerror.Wrap(err, "count payment preparation allocations")
	}
	return value.Int64(), nil
}

func decidePaymentPreparation(
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	identity paymentIdentityProjection,
	allocations int64,
	now time.Time,
) paymentPreparationDecision {
	if order.StatusCode == "cancelled" || invoice.StatusCode == "cancelled" {
		return paymentPreparationDecision{PaymentPreparationCancelled, PaymentPreparationOrderCancelled}
	}
	if order.PaymentDueAt == nil || !order.PaymentDueAt.After(now) {
		return paymentPreparationDecision{PaymentPreparationExpired, PaymentPreparationPaymentExpired}
	}
	payableOrder := order.StatusCode == "pending_payment" || order.StatusCode == "manual_review"
	if !payableOrder || invoice.StatusCode != "pending_payment" || order.TotalAmountMinor <= 0 ||
		invoice.PaidAmountMinor != 0 || invoice.RefundableAmountMinor != 0 ||
		invoice.RefundedAmountMinor != 0 || allocations != 0 {
		return paymentPreparationDecision{PaymentPreparationClosed, PaymentPreparationOrderNotPayable}
	}
	if identity.StatusCode != "verified" || identity.CaseNo == "" || identity.Version < 1 ||
		identity.VerifiedAt == nil {
		return paymentPreparationDecision{
			PaymentPreparationAwaitingIdentity, paymentIdentityReason(identity.StatusCode),
		}
	}
	if order.StatusCode == "manual_review" {
		return paymentPreparationDecision{PaymentPreparationAwaitingOrderReview, PaymentPreparationOrderReview}
	}
	return paymentPreparationDecision{PaymentPreparationReady, PaymentPreparationPaymentReady}
}

func paymentIdentityReason(status string) string {
	switch status {
	case "pending":
		return PaymentPreparationIdentityPending
	case "rejected":
		return PaymentPreparationIdentityRejected
	case "expired":
		return PaymentPreparationIdentityExpired
	case "revoked":
		return PaymentPreparationIdentityRevoked
	default:
		return PaymentPreparationIdentityUnverified
	}
}

func persistPaymentPreparation(
	ctx context.Context,
	tx gdb.TX,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	subject *entity.CustomerSubject,
	identity paymentIdentityProjection,
	existing *entity.CommercePaymentContext,
	decision paymentPreparationDecision,
	now time.Time,
) (*entity.CommercePaymentContext, bool, error) {
	readyAt := (*time.Time)(nil)
	if decision.status == PaymentPreparationReady {
		readyAt = &now
	}
	if existing == nil {
		contextNo := paymentPreparationContextNo(subject.CustomerNo, order.OrderNo)
		row := do.CommercePaymentContext{
			TenantId: idccap.DefaultTenantID, ContextNo: contextNo,
			OrderNo: order.OrderNo, InvoiceNo: invoice.InvoiceNo,
			CustomerNo: subject.CustomerNo, CustomerTypeCode: subject.CustomerTypeCode,
			StatusCode: decision.status, ReasonCode: decision.reason,
			IdentityStatusCode: identity.StatusCode, IdentityCaseNo: identity.CaseNo,
			IdentityVersion: identity.Version, FirstRequestedAt: &now,
			LastEvaluatedAt: &now, ReadyAt: readyAt, ExpiresAt: order.PaymentDueAt,
			Version: 1, CreatedAt: &now, UpdatedAt: &now,
		}
		if _, err := tx.Model(dao.CommercePaymentContext.Table()).Ctx(ctx).Data(row).Insert(); err != nil {
			return nil, false, gerror.Wrap(err, "insert payment preparation context")
		}
		contextRow, err := lockPaymentContextByOrder(ctx, tx, order.OrderNo)
		return contextRow, false, err
	}
	if existing.InvoiceNo != invoice.InvoiceNo || existing.CustomerNo != subject.CustomerNo ||
		existing.CustomerTypeCode != subject.CustomerTypeCode || existing.ExpiresAt == nil ||
		order.PaymentDueAt == nil || !existing.ExpiresAt.Equal(*order.PaymentDueAt) {
		return nil, false, bizerr.NewCode(CodePaymentPreparationConflict)
	}
	updated, err := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_core_commerce_payment_context
SET status_code = ?, reason_code = ?, identity_status_code = ?,
    identity_case_no = ?, identity_version = ?, last_evaluated_at = ?,
    ready_at = ?, version = version + 1, updated_at = ?
WHERE tenant_id = 0 AND context_no = ? AND version = ?`,
		decision.status, decision.reason, identity.StatusCode, identity.CaseNo,
		identity.Version, now, readyAt, now, existing.ContextNo, existing.Version,
	)
	if err != nil {
		return nil, false, gerror.Wrap(err, "update payment preparation context")
	}
	if err = requireOrderLifecycleAffected(updated, "update payment preparation context"); err != nil {
		return nil, false, bizerr.NewCode(CodePaymentPreparationConflict)
	}
	contextRow, err := lockPaymentContextByOrder(ctx, tx, order.OrderNo)
	return contextRow, true, err
}

func lockPaymentContextByOrder(
	ctx context.Context,
	tx gdb.TX,
	orderNo string,
) (*entity.CommercePaymentContext, error) {
	var contextRow *entity.CommercePaymentContext
	if err := tx.Model(dao.CommercePaymentContext.Table()).Ctx(ctx).Where(do.CommercePaymentContext{
		TenantId: idccap.DefaultTenantID, OrderNo: orderNo,
	}).LockUpdate().Scan(&contextRow); err != nil {
		return nil, gerror.Wrap(err, "lock payment preparation context")
	}
	return contextRow, nil
}

func paymentPreparationContextNo(customerNo string, orderNo string) string {
	digest := sha256.Sum256([]byte("commerce-payment-context\x00" + customerNo + "\x00" + orderNo))
	return "PAY_" + hex.EncodeToString(digest[:16])
}

func projectPaymentPreparation(
	contextRow *entity.CommercePaymentContext,
	order *entity.CommerceOrder,
	invoice *entity.BillingInvoice,
	reused bool,
) (*PaymentPreparation, error) {
	if contextRow == nil || contextRow.ExpiresAt == nil || contextRow.LastEvaluatedAt == nil ||
		order == nil || invoice == nil {
		return nil, gerror.New("payment preparation projection is incomplete")
	}
	return &PaymentPreparation{
		ContextNo: contextRow.ContextNo, OrderNo: order.OrderNo, InvoiceNo: invoice.InvoiceNo,
		CustomerTypeCode: contextRow.CustomerTypeCode, StatusCode: contextRow.StatusCode,
		ReasonCode: contextRow.ReasonCode, IdentityStatusCode: contextRow.IdentityStatusCode,
		CurrencyCode: order.CurrencyCode, AmountDueMinor: order.TotalAmountMinor,
		RequiresContract: order.RequiresContract, PaymentDueAt: *contextRow.ExpiresAt,
		LastEvaluatedAt: *contextRow.LastEvaluatedAt,
		Ready:           contextRow.StatusCode == PaymentPreparationReady, Reused: reused,
	}, nil
}

func normalizePaymentPreparationError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := bizerr.As(err); ok {
		return err
	}
	return bizerr.WrapCode(err, CodePaymentPreparationUnavailable)
}
