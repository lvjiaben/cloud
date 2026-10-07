// This file owns the bounded, provider-neutral delivery projection used by
// customer and staff order details. It deliberately excludes plugin-private
// evidence, supplier references, digests, and long-lived service internals.
package commerce

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const maximumDeliveryDirectoryItems = 100

// DeliveryView is the safe delivery progress attached to one order item.
// OrderItemNo is retained only as the bounded aggregate join key.
type DeliveryView struct {
	OrderItemNo       string
	Quantity          int
	StatusCode        string
	OngoingManagement bool
	StartedAt         *time.Time
	CompletedAt       *time.Time
	UpdatedAt         time.Time
}

// ServiceEntitlementView is present only when the frozen plugin contract says
// the delivered product needs ongoing customer management.
type ServiceEntitlementView struct {
	EntitlementNo string
	OrderItemNo   string
	UnitSequence  int
	StatusCode    string
	StartedAt     time.Time
	ExpiresAt     *time.Time
}

// AccountDeliveryDirectory reads delivery progress only for the authenticated
// customer. The customer number is always derived from the trusted principal.
type AccountDeliveryDirectory interface {
	BatchGetAccount(context.Context, string, []string) ([]DeliveryView, error)
}

// ManagementDeliveryDirectory reads delivery progress only after the existing
// order-detail authorizer has accepted the employee and effective data scope.
type ManagementDeliveryDirectory interface {
	BatchGetManagement(context.Context, string, []string) ([]DeliveryView, error)
}

type deliveryDirectoryImpl struct {
	database   gdb.DB
	principal  principal.Reader
	authorizer ManagementOrderAuthorizer
	management bool
}

var (
	_ AccountDeliveryDirectory    = (*deliveryDirectoryImpl)(nil)
	_ ManagementDeliveryDirectory = (*deliveryDirectoryImpl)(nil)
)

func NewAccountDeliveryDirectory(
	database gdb.DB,
	principalReader principal.Reader,
) (AccountDeliveryDirectory, error) {
	if isNilOrderDirectoryDependency(database) || isNilOrderDirectoryDependency(principalReader) {
		return nil, gerror.New("account delivery directory requires database and principal services")
	}
	return &deliveryDirectoryImpl{database: database, principal: principalReader}, nil
}

func NewManagementDeliveryDirectory(
	database gdb.DB,
	authorizer ManagementOrderAuthorizer,
) (ManagementDeliveryDirectory, error) {
	if isNilOrderDirectoryDependency(database) || isNilOrderDirectoryDependency(authorizer) {
		return nil, gerror.New("management delivery directory requires database and authorization services")
	}
	return &deliveryDirectoryImpl{database: database, authorizer: authorizer, management: true}, nil
}

func (s *deliveryDirectoryImpl) BatchGetAccount(
	ctx context.Context,
	orderNo string,
	orderItemNos []string,
) ([]DeliveryView, error) {
	if s == nil || s.management || s.principal == nil {
		return nil, bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID < 1 ||
		audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive ||
		strings.TrimSpace(audience.CustomerNo) == "" {
		return nil, bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	return s.batchGet(ctx, orderNo, orderItemNos, audience.CustomerNo)
}

func (s *deliveryDirectoryImpl) BatchGetManagement(
	ctx context.Context,
	orderNo string,
	orderItemNos []string,
) ([]DeliveryView, error) {
	orderNo = strings.TrimSpace(orderNo)
	if s == nil || !s.management || s.authorizer == nil {
		return nil, bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	// Authorization precedes input and persistence checks so an employee outside
	// the effective data scope cannot probe whether an order item exists.
	if err := s.authorizer.AuthorizeOrderDetail(ctx, orderNo); err != nil {
		return nil, err
	}
	return s.batchGet(ctx, orderNo, orderItemNos, "")
}

type deliveryDirectoryRow struct {
	OrderItemNo       string     `orm:"order_item_no"`
	Quantity          int        `orm:"quantity"`
	StatusCode        string     `orm:"status_code"`
	OngoingManagement bool       `orm:"ongoing_management"`
	StartedAt         *time.Time `orm:"started_at"`
	CompletedAt       *time.Time `orm:"completed_at"`
	UpdatedAt         *time.Time `orm:"updated_at"`
}

func (s *deliveryDirectoryImpl) batchGet(
	ctx context.Context,
	orderNo string,
	orderItemNos []string,
	customerNo string,
) ([]DeliveryView, error) {
	orderNo = strings.TrimSpace(orderNo)
	itemNos, err := normalizeDeliveryItemNos(orderNo, orderItemNos)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(itemNos)
	if err != nil {
		return nil, bizerr.NewCode(CodeOrderDirectoryUnavailable)
	}
	rows := make([]deliveryDirectoryRow, 0, len(itemNos))
	if err = s.database.GetScan(ctx, &rows, `
SELECT delivery.order_item_no,delivery.quantity,delivery.status_code,
       delivery.ongoing_management,delivery.started_at,delivery.completed_at,delivery.updated_at
FROM plugin_idc_core_product_delivery_record delivery
WHERE delivery.tenant_id=0 AND delivery.order_no=?
  AND delivery.order_item_no IN (SELECT jsonb_array_elements_text(?::jsonb))
  AND (?='' OR delivery.customer_no=?)
ORDER BY delivery.id`, orderNo, string(encoded), customerNo, customerNo); err != nil {
		return nil, bizerr.WrapCode(err, CodeOrderDirectoryUnavailable)
	}
	if len(rows) != len(itemNos) {
		return nil, bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	byItem := make(map[string]DeliveryView, len(rows))
	for _, row := range rows {
		status := safeDeliveryStatus(row.StatusCode)
		if status == "" || row.Quantity < 1 || row.UpdatedAt == nil {
			return nil, bizerr.NewCode(CodeOrderDirectoryUnavailable)
		}
		if _, exists := byItem[row.OrderItemNo]; exists {
			return nil, bizerr.NewCode(CodeOrderDirectoryUnavailable)
		}
		byItem[row.OrderItemNo] = DeliveryView{
			OrderItemNo: row.OrderItemNo, Quantity: row.Quantity, StatusCode: status,
			OngoingManagement: row.OngoingManagement, StartedAt: row.StartedAt,
			CompletedAt: row.CompletedAt, UpdatedAt: *row.UpdatedAt,
		}
	}
	result := make([]DeliveryView, 0, len(itemNos))
	for _, itemNo := range itemNos {
		row, exists := byItem[itemNo]
		if !exists {
			return nil, bizerr.NewCode(CodeOrderDirectoryNotAccessible)
		}
		result = append(result, row)
	}
	return result, nil
}

func normalizeDeliveryItemNos(orderNo string, orderItemNos []string) ([]string, error) {
	if !orderLifecycleIdentifierPattern.MatchString(orderNo) || len(orderNo) > 40 ||
		len(orderItemNos) == 0 || len(orderItemNos) > maximumDeliveryDirectoryItems {
		return nil, bizerr.NewCode(CodeOrderDirectoryInvalid)
	}
	seen := make(map[string]struct{}, len(orderItemNos))
	result := make([]string, 0, len(orderItemNos))
	for _, raw := range orderItemNos {
		itemNo := strings.TrimSpace(raw)
		if !orderLifecycleIdentifierPattern.MatchString(itemNo) || len(itemNo) > 40 {
			return nil, bizerr.NewCode(CodeOrderDirectoryInvalid)
		}
		if _, exists := seen[itemNo]; exists {
			return nil, bizerr.NewCode(CodeOrderDirectoryInvalid)
		}
		seen[itemNo] = struct{}{}
		result = append(result, itemNo)
	}
	return result, nil
}

func safeDeliveryStatus(value string) string {
	switch value {
	case "awaiting_settlement":
		return "awaiting_payment"
	case "review_pending":
		return "awaiting_review"
	case "queued", "processing":
		return "processing"
	case "manual_pending":
		return "manual_processing"
	case "completed":
		return "completed"
	case "incomplete":
		return "not_completed"
	case "canceled":
		return "cancelled"
	default:
		return ""
	}
}

func loadOrderDeliveryDirectory(
	ctx context.Context,
	tx gdb.TX,
	orderNo string,
	maximumItems int,
) ([]DeliveryView, []ServiceEntitlementView, error) {
	deliveryRows := make([]deliveryDirectoryRow, 0, maximumItems)
	if err := tx.GetScan(&deliveryRows, `
SELECT order_item_no,quantity,status_code,ongoing_management,started_at,completed_at,updated_at
FROM plugin_idc_core_product_delivery_record
WHERE tenant_id=0 AND order_no=? ORDER BY id LIMIT ?`, orderNo, maximumItems+1); err != nil {
		return nil, nil, err
	}
	if len(deliveryRows) > maximumItems {
		return nil, nil, gerror.New("commerce order delivery bound exceeded")
	}
	deliveries := make([]DeliveryView, 0, len(deliveryRows))
	for _, row := range deliveryRows {
		status := safeDeliveryStatus(row.StatusCode)
		if status == "" || row.Quantity < 1 || row.UpdatedAt == nil {
			return nil, nil, gerror.New("commerce order delivery evidence is unavailable")
		}
		deliveries = append(deliveries, DeliveryView{
			OrderItemNo: row.OrderItemNo, Quantity: row.Quantity, StatusCode: status,
			OngoingManagement: row.OngoingManagement, StartedAt: row.StartedAt,
			CompletedAt: row.CompletedAt, UpdatedAt: *row.UpdatedAt,
		})
	}
	type entitlementRow struct {
		EntitlementNo string     `orm:"entitlement_no"`
		OrderItemNo   string     `orm:"order_item_no"`
		UnitSequence  int        `orm:"unit_sequence"`
		StatusCode    string     `orm:"status_code"`
		StartedAt     *time.Time `orm:"started_at"`
		ExpiresAt     *time.Time `orm:"expires_at"`
	}
	entitlementRows := make([]entitlementRow, 0)
	if err := tx.GetScan(&entitlementRows, `
SELECT entitlement_no,order_item_no,unit_sequence,status_code,started_at,expires_at
FROM plugin_idc_core_service_entitlement
WHERE tenant_id=0 AND order_no=? ORDER BY order_item_no,unit_sequence LIMIT ?`,
		orderNo, maximumOrderDetailUnits+1); err != nil {
		return nil, nil, err
	}
	if len(entitlementRows) > maximumOrderDetailUnits {
		return nil, nil, gerror.New("commerce order service entitlement bound exceeded")
	}
	entitlements := make([]ServiceEntitlementView, 0, len(entitlementRows))
	for _, row := range entitlementRows {
		if !orderLifecycleIdentifierPattern.MatchString(row.EntitlementNo) ||
			row.UnitSequence < 1 || row.StartedAt == nil || !safeEntitlementStatus(row.StatusCode) {
			return nil, nil, gerror.New("commerce order service entitlement evidence is unavailable")
		}
		entitlements = append(entitlements, ServiceEntitlementView{
			EntitlementNo: row.EntitlementNo, OrderItemNo: row.OrderItemNo,
			UnitSequence: row.UnitSequence,
			StatusCode:   row.StatusCode, StartedAt: *row.StartedAt, ExpiresAt: row.ExpiresAt,
		})
	}
	return deliveries, entitlements, nil
}

func safeEntitlementStatus(value string) bool {
	return value == "active" || value == "suspended" || value == "expired" || value == "terminated"
}
