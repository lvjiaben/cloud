// This file owns bounded customer and staff order read models. It never writes
// lifecycle state and never exposes internal database identifiers or digests.
package commerce

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"math"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/customerprojection"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	defaultAccountOrderLimit    = 20
	maximumAccountOrderLimit    = 50
	defaultManagementOrderLimit = 25
	maximumManagementOrderLimit = 100
	maximumOrderDetailItems     = 100
	maximumOrderDetailUnits     = 1000
	maximumOrderDetailReviews   = 100
	maximumOrderDetailRecords   = 1000
)

var orderDirectoryFilterPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// OrderDirectoryFilter is shared by account and staff readers. CustomerNo is
// ignored by the account reader and is accepted only after staff authorization.
type OrderDirectoryFilter struct {
	CustomerNo       string
	StatusCode       string
	RiskDecisionCode string
	Cursor           string
	Limit            int
	// Page selects a one-based numbered page; zero retains cursor traversal.
	Page int
}

// OrderSummary is one bounded list row.
type OrderSummary struct {
	OrderNo              string
	CustomerNo           string
	CustomerCode         string
	CustomerDisplayName  string
	CustomerPrimaryPhone string
	CustomerTypeCode     string
	ProductSummary       string
	IPAddress            string
	PaymentMethodCode    string
	Remark               string
	StatusCode           string
	RiskDecisionCode     string
	CurrencyCode         string
	ItemCount            int
	TotalAmountMinor     int64

	PaymentDueAt      *time.Time
	CreatedAt         time.Time
	CanCancel         bool
	CanPreparePayment bool
	AssistedRequestNo string
	PaymentTermCode   string
	Version           int64
}

// OrderPage contains bounded rows and a count from the same authorized filter.
type OrderPage struct {
	Orders     []OrderSummary
	NextCursor string
	Total      int
}

type OrderInvoice struct {
	InvoiceNo             string
	StatusCode            string
	CurrencyCode          string
	SubtotalMinor         int64
	DiscountMinor         int64
	NetAmountMinor        int64
	TaxAmountMinor        int64
	TotalAmountMinor      int64
	PaidAmountMinor       int64
	RefundableAmountMinor int64
	RefundedAmountMinor   int64
	PaymentDueAt          *time.Time
	IssuedAt              *time.Time
	SettledAt             *time.Time
	CancelledAt           *time.Time
}

type OrderItemView struct {
	OrderItemNo     string
	LineNumber      int
	ProductNameZhCN string
	ProductNameEnUS string
	Quantity        int
	PurchaseCycles  int
	ChargeTypeCode  string
	PeriodUnitCode  string
	PeriodCount     int

	FulfillmentModeCode   string
	RiskDecisionCode      string
	InventoryDecisionCode string
	SetupSubtotalMinor    int64
	PeriodSubtotalMinor   int64
	SubtotalMinor         int64
	DiscountMinor         int64
	NetAmountMinor        int64
	TaxAmountMinor        int64
	TotalAmountMinor      int64
}

type OrderCancellationView struct {
	CancellationNo            string
	SourceCode                string
	ReasonCode                string
	StatusCode                string
	PaymentClosureOutcomeCode string
	RequestedAt               *time.Time
	CompletedAt               *time.Time
}

type OrderReviewView struct {
	ReviewNo       string
	ReviewTypeCode string
	StatusCode     string
	ReasonCode     string
	RequestedAt    *time.Time
	ReviewedAt     *time.Time
}

type OrderPaymentPreparationView struct {
	ContextNo          string
	StatusCode         string
	ReasonCode         string
	IdentityStatusCode string
	LastEvaluatedAt    *time.Time
	ReadyAt            *time.Time
	ExpiresAt          *time.Time
}

// OrderDetail is the single aggregate consumed by both frontends.
type OrderDetail struct {
	Summary             OrderSummary
	Invoice             *OrderInvoice
	Items               []OrderItemView
	Deliveries          []DeliveryView
	ServiceEntitlements []ServiceEntitlementView
	Cancellation        *OrderCancellationView
	Reviews             []OrderReviewView
	PaymentPreparation  *OrderPaymentPreparationView
	CanApproveRisk      bool
	CanRejectRisk       bool
}

type AccountOrderReader interface {
	ListAccountOrders(context.Context, OrderDirectoryFilter) (*OrderPage, error)
	GetAccountOrder(context.Context, string) (*OrderDetail, error)
}

type ManagementOrderReader interface {
	ListManagementOrders(context.Context, OrderDirectoryFilter) (*OrderPage, error)
	GetManagementOrder(context.Context, string) (*OrderDetail, error)
}

type ManagementOrderAuthorizer interface {
	AuthorizeOrderList(context.Context) error
	AuthorizeOrderDetail(context.Context, string) error
}

type orderDirectoryImpl struct {
	database   gdb.DB
	principal  principal.Reader
	authorizer ManagementOrderAuthorizer
	cursor     *orderCursorCodec
	management bool
}

var _ AccountOrderReader = (*orderDirectoryImpl)(nil)
var _ ManagementOrderReader = (*orderDirectoryImpl)(nil)

func NewAccountOrderReader(database gdb.DB, principalReader principal.Reader, cursorKey []byte) (AccountOrderReader, error) {
	if isNilOrderDirectoryDependency(database) || isNilOrderDirectoryDependency(principalReader) {
		return nil, gerror.New("account order directory requires database and principal services")
	}
	codec, err := newOrderCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &orderDirectoryImpl{database: database, principal: principalReader, cursor: codec}, nil
}

func NewManagementOrderReader(database gdb.DB, authorizer ManagementOrderAuthorizer, cursorKey []byte) (ManagementOrderReader, error) {
	if isNilOrderDirectoryDependency(database) || isNilOrderDirectoryDependency(authorizer) {
		return nil, gerror.New("management order directory requires database and authorization services")
	}
	codec, err := newOrderCursorCodec(cursorKey)
	if err != nil {
		return nil, err
	}
	return &orderDirectoryImpl{database: database, authorizer: authorizer, cursor: codec, management: true}, nil
}

func isNilOrderDirectoryDependency(value any) bool {
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

func (s *orderDirectoryImpl) ListAccountOrders(ctx context.Context, filter OrderDirectoryFilter) (*OrderPage, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	filter.CustomerNo = customerNo
	page, err := s.list(ctx, filter, maximumAccountOrderLimit, defaultAccountOrderLimit)
	if err != nil {
		return nil, err
	}
	// CustomerNo is a staff-only projection field. Ownership is already derived
	// from the authenticated principal and must not be reflected to the account API.
	for index := range page.Orders {
		page.Orders[index].CustomerNo = ""
		page.Orders[index].CustomerCode = ""
		page.Orders[index].CustomerDisplayName = ""
		page.Orders[index].CustomerPrimaryPhone = ""
		page.Orders[index].AssistedRequestNo = ""
	}
	return page, nil
}

func (s *orderDirectoryImpl) GetAccountOrder(ctx context.Context, orderNo string) (*OrderDetail, error) {
	customerNo, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, strings.TrimSpace(orderNo), customerNo, false)
}

func (s *orderDirectoryImpl) ListManagementOrders(ctx context.Context, filter OrderDirectoryFilter) (*OrderPage, error) {
	if s == nil || !s.management || s.authorizer == nil {
		return nil, bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	if err := s.authorizer.AuthorizeOrderList(ctx); err != nil {
		return nil, err
	}
	return s.list(ctx, filter, maximumManagementOrderLimit, defaultManagementOrderLimit)
}

func (s *orderDirectoryImpl) GetManagementOrder(ctx context.Context, orderNo string) (*OrderDetail, error) {
	orderNo = strings.TrimSpace(orderNo)
	if s == nil || !s.management || s.authorizer == nil {
		return nil, bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	if err := s.authorizer.AuthorizeOrderDetail(ctx, orderNo); err != nil {
		return nil, err
	}
	return s.detail(ctx, orderNo, "", true)
}

func (s *orderDirectoryImpl) currentCustomer(ctx context.Context) (string, error) {
	if s == nil || s.management || s.principal == nil {
		return "", bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo == "" || audience.UserID < 1 {
		return "", bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	return audience.CustomerNo, nil
}

func (s *orderDirectoryImpl) list(ctx context.Context, filter OrderDirectoryFilter, maximum, fallback int) (*OrderPage, error) {
	input, digest, boundary, err := s.normalizeFilter(filter, maximum, fallback)
	if err != nil {
		return nil, err
	}
	columns := dao.CommerceOrder.Columns()
	projectionFields := []string{
		columns.Id, columns.OrderNo, columns.CustomerNo, columns.StatusCode,
		columns.RiskDecisionCode, columns.CurrencyCode, columns.ItemCount,
		columns.TotalAmountMinor, columns.PaymentDueAt, columns.CreatedAt,
		columns.AssistedRequestNo, columns.PaymentTermCode, columns.Version,
	}
	query := s.database.Model(dao.CommerceOrder.Table()).Ctx(ctx).Where(do.CommerceOrder{TenantId: idccap.DefaultTenantID})
	if input.CustomerNo != "" {
		query = query.Where(columns.CustomerNo, input.CustomerNo)
	}
	if input.StatusCode != "" {
		query = query.Where(columns.StatusCode, input.StatusCode)
	}
	if input.RiskDecisionCode != "" {
		query = query.Where(columns.RiskDecisionCode, input.RiskDecisionCode)
	}
	rows := make([]entity.CommerceOrder, 0, input.Limit+1)
	var total int
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, readErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); readErr != nil {
			return readErr
		}
		// Clone before pagination, so the count cannot inherit cursor or offset.
		pageQuery := query.Clone().Ctx(ctx)
		var readErr error
		total, readErr = pageQuery.Clone().Count()
		if readErr != nil {
			return readErr
		}
		if boundary != nil {
			boundaryTime := time.UnixMicro(boundary.CreatedAtMicroseconds)
			pageQuery = pageQuery.Where("(created_at < ?) OR (created_at = ? AND id < ?)", boundaryTime, boundaryTime, boundary.ID)
		}
		if input.Page > 0 {
			pageQuery = pageQuery.Offset((input.Page - 1) * input.Limit)
		}
		return pageQuery.Fields(projectionFields).OrderDesc(columns.CreatedAt).OrderDesc(columns.Id).Limit(input.Limit + 1).Scan(&rows)
	})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeOrderDirectoryUnavailable)
	}
	hasMore := len(rows) > input.Limit
	if hasMore {
		rows = rows[:input.Limit]
	}
	page := &OrderPage{Orders: make([]OrderSummary, 0, len(rows)), Total: total}
	for _, row := range rows {
		if row.CreatedAt == nil {
			return nil, bizerr.NewCode(CodeOrderDirectoryUnavailable)
		}
		page.Orders = append(page.Orders, orderSummaryFromEntity(row))
	}
	if s.management && len(page.Orders) > 0 {
		customerNos := make([]string, 0, len(page.Orders))
		for _, order := range page.Orders {
			customerNos = append(customerNos, order.CustomerNo)
		}
		customers, projectionErr := customerprojection.Load(ctx, s.database, customerNos)
		if projectionErr != nil {
			return nil, bizerr.WrapCode(projectionErr, CodeOrderDirectoryUnavailable)
		}
		for index := range page.Orders {
			customer := customers[page.Orders[index].CustomerNo]
			page.Orders[index].CustomerCode = customer.CustomerCode
			page.Orders[index].CustomerDisplayName = customer.DisplayName
			page.Orders[index].CustomerPrimaryPhone = customer.PrimaryPhone
		}
	}
	if len(page.Orders) > 0 {
		if enrichmentErr := s.enrichOrderSummaries(ctx, page.Orders); enrichmentErr != nil {
			return nil, bizerr.WrapCode(enrichmentErr, CodeOrderDirectoryUnavailable)
		}
	}
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor, err = s.cursor.encode(orderCursorBoundary{
			CreatedAtMicroseconds: last.CreatedAt.UnixMicro(), ID: last.Id,
		}, digest)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodeOrderDirectoryUnavailable)
		}
	}
	return page, nil
}

type orderProductSummaryRow struct {
	OrderNo        string `orm:"order_no"`
	ProductSummary string `orm:"product_summary"`
}

type orderPaymentMethodRow struct {
	OrderNo           string `orm:"order_no"`
	PaymentMethodCode string `orm:"payment_method_code"`
}

// enrichOrderSummaries keeps list rendering bounded: one product aggregation
// and one payment aggregation are executed for the whole page, never per row.
func (s *orderDirectoryImpl) enrichOrderSummaries(ctx context.Context, orders []OrderSummary) error {
	orderNos := make([]string, 0, len(orders))
	for _, order := range orders {
		orderNos = append(orderNos, order.OrderNo)
	}
	productRows := make([]orderProductSummaryRow, 0, len(orders))
	if err := s.database.Model(dao.CommerceOrderItem.Table()).Ctx(ctx).
		Fields("order_no, string_agg(product_name_zh_cn, '、' ORDER BY line_number) AS product_summary").
		Where(do.CommerceOrderItem{TenantId: idccap.DefaultTenantID}).
		WhereIn(dao.CommerceOrderItem.Columns().OrderNo, orderNos).
		Group(dao.CommerceOrderItem.Columns().OrderNo).Scan(&productRows); err != nil {
		return err
	}
	paymentRows := make([]orderPaymentMethodRow, 0, len(orders))
	if err := s.database.Model(dao.BillingInvoice.Table()+" invoice").Ctx(ctx).
		LeftJoin(dao.BillingPaymentAllocation.Table()+" allocation",
			"allocation.tenant_id=invoice.tenant_id AND allocation.invoice_no=invoice.invoice_no AND allocation.direction_code='apply'").
		Fields("invoice.order_no, COALESCE(MIN(allocation.payment_source_type_code), '') AS payment_method_code").
		Where("invoice.tenant_id", idccap.DefaultTenantID).
		WhereIn("invoice.order_no", orderNos).
		Group("invoice.order_no").Scan(&paymentRows); err != nil {
		return err
	}
	products := make(map[string]string, len(productRows))
	for _, row := range productRows {
		products[row.OrderNo] = row.ProductSummary
	}
	paymentMethods := make(map[string]string, len(paymentRows))
	for _, row := range paymentRows {
		paymentMethods[row.OrderNo] = row.PaymentMethodCode
	}
	for index := range orders {
		orders[index].ProductSummary = products[orders[index].OrderNo]
		orders[index].PaymentMethodCode = paymentMethods[orders[index].OrderNo]
	}
	return nil
}

func (s *orderDirectoryImpl) normalizeFilter(filter OrderDirectoryFilter, maximum, fallback int) (OrderDirectoryFilter, [sha256.Size]byte, *orderCursorBoundary, error) {
	filter.CustomerNo = strings.TrimSpace(filter.CustomerNo)
	filter.StatusCode = strings.TrimSpace(filter.StatusCode)
	filter.RiskDecisionCode = strings.TrimSpace(filter.RiskDecisionCode)
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	if filter.Limit == 0 {
		filter.Limit = fallback
	}
	if filter.Limit < 1 || filter.Limit > maximum || filter.Page < 0 ||
		(filter.Page > 0 && (filter.Cursor != "" || filter.Page-1 > math.MaxInt/filter.Limit)) || len(filter.CustomerNo) > 40 ||
		(filter.StatusCode != "" && !orderDirectoryFilterPattern.MatchString(filter.StatusCode)) ||
		(filter.RiskDecisionCode != "" && !orderDirectoryFilterPattern.MatchString(filter.RiskDecisionCode)) ||
		len(filter.Cursor) > maximumOrderCursorLength {
		return OrderDirectoryFilter{}, [sha256.Size]byte{}, nil, bizerr.NewCode(CodeOrderDirectoryInvalid)
	}
	encoded, err := json.Marshal(struct {
		Management       bool   `json:"management"`
		CustomerNo       string `json:"customerNo"`
		StatusCode       string `json:"statusCode"`
		RiskDecisionCode string `json:"riskDecisionCode"`
		Limit            int    `json:"limit"`
	}{s.management, filter.CustomerNo, filter.StatusCode, filter.RiskDecisionCode, filter.Limit})
	if err != nil {
		return OrderDirectoryFilter{}, [sha256.Size]byte{}, nil, bizerr.NewCode(CodeOrderDirectoryUnavailable)
	}
	digest := sha256.Sum256(encoded)
	var boundary *orderCursorBoundary
	if filter.Cursor != "" {
		boundary, err = s.cursor.decode(filter.Cursor, digest)
		if err != nil {
			return OrderDirectoryFilter{}, [sha256.Size]byte{}, nil, bizerr.NewCode(CodeOrderDirectoryInvalid)
		}
	}
	return filter, digest, boundary, nil
}

func (s *orderDirectoryImpl) detail(ctx context.Context, orderNo, customerNo string, management bool) (*OrderDetail, error) {
	if !orderLifecycleIdentifierPattern.MatchString(orderNo) || len(orderNo) > 40 {
		return nil, bizerr.NewCode(CodeOrderDirectoryInvalid)
	}
	var result *OrderDetail
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, setErr := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); setErr != nil {
			return setErr
		}
		var order *entity.CommerceOrder
		query := tx.Model(dao.CommerceOrder.Table()).Ctx(ctx).
			Where(do.CommerceOrder{TenantId: idccap.DefaultTenantID, OrderNo: orderNo})
		if customerNo != "" {
			query = query.Where(dao.CommerceOrder.Columns().CustomerNo, customerNo)
		}
		if loadErr := query.Scan(&order); loadErr != nil {
			return loadErr
		}
		if order == nil || order.CreatedAt == nil {
			return bizerr.NewCode(CodeOrderDirectoryNotAccessible)
		}
		loaded, loadErr := loadOrderAggregate(ctx, tx, *order)
		if loadErr != nil {
			return loadErr
		}
		if !management {
			loaded.Summary.CustomerNo = ""
			loaded.Summary.CustomerCode = ""
			loaded.Summary.CustomerDisplayName = ""
			loaded.Summary.CustomerPrimaryPhone = ""
			loaded.Summary.CustomerTypeCode = ""
			loaded.CanApproveRisk = false
			loaded.CanRejectRisk = false
		}
		result = loaded
		return nil
	})
	if err != nil {
		if bizerr.Is(err, CodeOrderDirectoryInvalid) || bizerr.Is(err, CodeOrderDirectoryNotAccessible) {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeOrderDirectoryUnavailable)
	}
	if management {
		customers, projectionErr := customerprojection.Load(ctx, s.database, []string{result.Summary.CustomerNo})
		if projectionErr != nil {
			return nil, bizerr.WrapCode(projectionErr, CodeOrderDirectoryUnavailable)
		}
		customer := customers[result.Summary.CustomerNo]
		result.Summary.CustomerCode, result.Summary.CustomerDisplayName, result.Summary.CustomerPrimaryPhone = customer.CustomerCode, customer.DisplayName, customer.PrimaryPhone
	}
	return result, nil
}

func loadOrderAggregate(ctx context.Context, tx gdb.TX, order entity.CommerceOrder) (*OrderDetail, error) {
	result := &OrderDetail{
		Summary: orderSummaryFromEntity(order), Items: []OrderItemView{}, Deliveries: []DeliveryView{}, ServiceEntitlements: []ServiceEntitlementView{}, Reviews: []OrderReviewView{},
	}
	var subject *entity.CustomerSubject
	if err := tx.Model(dao.CustomerSubject.Table()).Ctx(ctx).
		Fields(dao.CustomerSubject.Columns().CustomerTypeCode).
		Where(do.CustomerSubject{TenantId: idccap.DefaultTenantID, CustomerNo: order.CustomerNo}).
		Limit(1).Scan(&subject); err != nil {
		return nil, err
	}
	if subject == nil || (subject.CustomerTypeCode != "individual" && subject.CustomerTypeCode != "enterprise") {
		return nil, gerror.New("commerce order customer type is unavailable")
	}
	result.Summary.CustomerTypeCode = subject.CustomerTypeCode
	var invoice *entity.BillingInvoice
	if err := tx.Model(dao.BillingInvoice.Table()).Ctx(ctx).
		Where(do.BillingInvoice{TenantId: idccap.DefaultTenantID, CustomerNo: order.CustomerNo}).
		Where(dao.BillingInvoice.Columns().OrderNo, order.OrderNo).
		Limit(2).Scan(&invoice); err != nil {
		return nil, err
	}
	if invoice != nil {
		result.Invoice = &OrderInvoice{
			InvoiceNo: invoice.InvoiceNo, StatusCode: invoice.StatusCode, CurrencyCode: invoice.CurrencyCode,
			SubtotalMinor: invoice.SubtotalMinor, DiscountMinor: invoice.DiscountMinor,
			NetAmountMinor: invoice.NetAmountMinor, TaxAmountMinor: invoice.TaxAmountMinor,
			TotalAmountMinor: invoice.TotalAmountMinor, PaidAmountMinor: invoice.PaidAmountMinor,
			RefundableAmountMinor: invoice.RefundableAmountMinor, RefundedAmountMinor: invoice.RefundedAmountMinor,
			PaymentDueAt: invoice.PaymentDueAt, IssuedAt: invoice.IssuedAt, SettledAt: invoice.SettledAt,
			CancelledAt: invoice.CancelledAt,
		}
	}
	var items []entity.CommerceOrderItem
	if err := tx.Model(dao.CommerceOrderItem.Table()).Ctx(ctx).
		Where(do.CommerceOrderItem{TenantId: idccap.DefaultTenantID, OrderNo: order.OrderNo}).
		OrderAsc(dao.CommerceOrderItem.Columns().LineNumber).Limit(maximumOrderDetailItems + 1).Scan(&items); err != nil {
		return nil, err
	}
	if len(items) > maximumOrderDetailItems {
		return nil, gerror.New("commerce order detail item bound exceeded")
	}
	type quantityRow struct {
		OrderItemNo string `orm:"order_item_no"`
		Quantity    int    `orm:"quantity"`
	}
	quantityRows := make([]quantityRow, 0, len(items))
	if err := tx.GetScan(&quantityRows, `SELECT order_item_no,quantity FROM plugin_idc_core_commerce_order_item_quantity_snapshot WHERE tenant_id=0 AND order_no=? ORDER BY order_item_no LIMIT ?`, order.OrderNo, maximumOrderDetailItems+1); err != nil || len(quantityRows) != len(items) {
		return nil, gerror.New("commerce order generic quantity evidence is unavailable")
	}
	quantities := make(map[string]int, len(quantityRows))
	for _, row := range quantityRows {
		quantities[row.OrderItemNo] = row.Quantity
	}
	for _, item := range items {
		result.Items = append(result.Items, OrderItemView{
			OrderItemNo: item.OrderItemNo, LineNumber: item.LineNumber,
			ProductNameZhCN: item.ProductNameZhCn, ProductNameEnUS: item.ProductNameEnUs,
			Quantity: quantities[item.OrderItemNo], PurchaseCycles: item.PurchaseCycles,
			ChargeTypeCode: item.ChargeTypeCode, PeriodUnitCode: item.PeriodUnitCode, PeriodCount: item.PeriodCount,
			FulfillmentModeCode: item.FulfillmentModeCode,
			RiskDecisionCode:    item.RiskDecisionCode, InventoryDecisionCode: item.InventoryDecisionCode,
			SetupSubtotalMinor: item.SetupSubtotalMinor, PeriodSubtotalMinor: item.PeriodSubtotalMinor,
			SubtotalMinor: item.SubtotalMinor, DiscountMinor: item.DiscountMinor, NetAmountMinor: item.NetAmountMinor,
			TaxAmountMinor: item.TaxAmountMinor, TotalAmountMinor: item.TotalAmountMinor,
		})
	}
	deliveries, entitlements, deliveryErr := loadOrderDeliveryDirectory(
		ctx, tx, order.OrderNo, maximumOrderDetailItems,
	)
	if deliveryErr != nil {
		return nil, deliveryErr
	}
	result.Deliveries, result.ServiceEntitlements = deliveries, entitlements
	var cancellation *entity.CommerceOrderCancellation
	if err := tx.Model(dao.CommerceOrderCancellation.Table()).Ctx(ctx).
		Where(do.CommerceOrderCancellation{TenantId: idccap.DefaultTenantID, OrderNo: order.OrderNo}).
		OrderDesc(dao.CommerceOrderCancellation.Columns().Id).Limit(1).Scan(&cancellation); err != nil {
		return nil, err
	}
	if cancellation != nil {
		result.Cancellation = &OrderCancellationView{
			CancellationNo: cancellation.CancellationNo, SourceCode: cancellation.SourceCode,
			ReasonCode: cancellation.ReasonCode, StatusCode: cancellation.StatusCode,
			PaymentClosureOutcomeCode: optionalString(cancellation.PaymentClosureOutcomeCode),
			RequestedAt:               cancellation.RequestedAt, CompletedAt: cancellation.CompletedAt,
		}
	}
	var reviews []entity.CommerceOrderReview
	if err := tx.Model(dao.CommerceOrderReview.Table()).Ctx(ctx).
		Where(do.CommerceOrderReview{TenantId: idccap.DefaultTenantID, OrderNo: order.OrderNo}).
		OrderDesc(dao.CommerceOrderReview.Columns().RequestedAt).
		Limit(maximumOrderDetailReviews + 1).Scan(&reviews); err != nil {
		return nil, err
	}
	if len(reviews) > maximumOrderDetailReviews {
		return nil, gerror.New("commerce order review bound exceeded")
	}
	for _, review := range reviews {
		result.Reviews = append(result.Reviews, OrderReviewView{
			ReviewNo: review.ReviewNo, ReviewTypeCode: review.ReviewTypeCode, StatusCode: review.StatusCode,
			ReasonCode: review.ReasonCode, RequestedAt: review.RequestedAt, ReviewedAt: review.ReviewedAt,
		})
		if review.ReviewTypeCode == "risk" && review.StatusCode == "pending" {
			result.CanApproveRisk, result.CanRejectRisk = true, true
		}
	}
	var paymentContext *entity.CommercePaymentContext
	if err := tx.Model(dao.CommercePaymentContext.Table()).Ctx(ctx).
		Where(do.CommercePaymentContext{TenantId: idccap.DefaultTenantID, OrderNo: order.OrderNo, CustomerNo: order.CustomerNo}).
		Limit(1).Scan(&paymentContext); err != nil {
		return nil, err
	}
	if paymentContext != nil {
		result.PaymentPreparation = &OrderPaymentPreparationView{
			ContextNo: paymentContext.ContextNo, StatusCode: paymentContext.StatusCode,
			ReasonCode: paymentContext.ReasonCode, IdentityStatusCode: paymentContext.IdentityStatusCode,
			LastEvaluatedAt: paymentContext.LastEvaluatedAt, ReadyAt: paymentContext.ReadyAt,
			ExpiresAt: paymentContext.ExpiresAt,
		}
	}
	return result, nil
}

func orderSummaryFromEntity(order entity.CommerceOrder) OrderSummary {
	createdAt := time.Time{}
	if order.CreatedAt != nil {
		createdAt = *order.CreatedAt
	}
	canCancel := order.StatusCode == "pending_payment" || order.StatusCode == "manual_review" || order.StatusCode == "payment_processing"
	canPreparePayment := order.StatusCode == "pending_payment" || order.StatusCode == "manual_review"
	return OrderSummary{
		OrderNo: order.OrderNo, CustomerNo: order.CustomerNo, StatusCode: order.StatusCode,
		RiskDecisionCode: order.RiskDecisionCode, CurrencyCode: order.CurrencyCode,
		ItemCount: order.ItemCount, TotalAmountMinor: order.TotalAmountMinor,
		PaymentDueAt: order.PaymentDueAt,
		CreatedAt:    createdAt, CanCancel: canCancel, CanPreparePayment: canPreparePayment,
		AssistedRequestNo: optionalString(order.AssistedRequestNo), PaymentTermCode: order.PaymentTermCode,
		Version: order.Version,
	}
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
