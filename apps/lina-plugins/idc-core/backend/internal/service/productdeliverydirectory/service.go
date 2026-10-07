// Package productdeliverydirectory implements the customer-safe product-plugin
// directory from core-owned delivery facts.
package productdeliverydirectory

import (
	"context"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/productdeliverycap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type Service struct {
	database  gdb.DB
	principal principal.Reader
}

var _ productdeliverycap.CurrentCustomerDeliveryService = (*Service)(nil)

func New(database gdb.DB, principalReader principal.Reader) (*Service, error) {
	if database == nil || principalReader == nil {
		return nil, gerror.New("product delivery directory requires database and principal services")
	}
	return &Service{database: database, principal: principalReader}, nil
}

type deliveryRow struct {
	ID                int64      `orm:"id"`
	DeliveryNo        string     `orm:"delivery_no"`
	ProductNameZhCN   string     `orm:"product_name_zh_cn"`
	ProductNameEnUS   string     `orm:"product_name_en_us"`
	StatusCode        string     `orm:"status_code"`
	Quantity          int        `orm:"quantity"`
	OngoingManagement bool       `orm:"ongoing_management"`
	ExternalReference string     `orm:"external_reference"`
	StartedAt         *time.Time `orm:"started_at"`
	CompletedAt       *time.Time `orm:"completed_at"`
	UpdatedAt         *time.Time `orm:"updated_at"`
}

func (s *Service) ListCurrentCustomerDeliveries(ctx context.Context, callerPluginID string, request productdeliverycap.CurrentCustomerDeliveryListRequest) (*productdeliverycap.CurrentCustomerDeliveryPage, error) {
	customerNo, err := s.audience(ctx, callerPluginID)
	if err != nil || productdeliverycap.ValidateListRequest(request) != nil {
		return nil, gerror.New("product delivery directory is not accessible")
	}
	afterID, err := decodeCursor(request.Cursor)
	if err != nil {
		return nil, gerror.New("product delivery directory is not accessible")
	}
	rows := make([]deliveryRow, 0, request.Limit+1)
	err = s.database.GetScan(ctx, &rows, `
SELECT delivery.id,delivery.delivery_no,intent.product_name_zh_cn,intent.product_name_en_us,
       delivery.status_code,delivery.quantity,delivery.ongoing_management,
       COALESCE(task.external_reference,'') external_reference,
       delivery.started_at,delivery.completed_at,delivery.updated_at
FROM plugin_idc_core_product_delivery_record delivery
JOIN plugin_idc_core_product_fulfillment_intent intent
  ON intent.tenant_id=delivery.tenant_id AND intent.order_item_no=delivery.order_item_no
LEFT JOIN plugin_idc_core_product_fulfillment_task task
  ON task.tenant_id=intent.tenant_id AND task.intent_no=intent.intent_no
WHERE delivery.tenant_id=0 AND delivery.customer_no=? AND delivery.plugin_id=? AND delivery.id<?
ORDER BY delivery.id DESC LIMIT ?`, customerNo, callerPluginID, afterID, request.Limit+1)
	if err != nil {
		return nil, gerror.Wrap(err, "read current customer product deliveries")
	}
	page := &productdeliverycap.CurrentCustomerDeliveryPage{}
	if len(rows) > request.Limit {
		page.NextCursor = encodeCursor(rows[request.Limit-1].ID)
		rows = rows[:request.Limit]
	}
	page.Items = make([]productdeliverycap.CurrentCustomerDeliveryItem, len(rows))
	for index, row := range rows {
		item, itemErr := project(row)
		if itemErr != nil {
			return nil, itemErr
		}
		page.Items[index] = item
	}
	if err = productdeliverycap.ValidatePage(page, request); err != nil {
		return nil, err
	}
	return page, nil
}

func (s *Service) GetCurrentCustomerDelivery(ctx context.Context, callerPluginID string, request productdeliverycap.CurrentCustomerDeliveryGetRequest) (*productdeliverycap.CurrentCustomerDeliveryResult, error) {
	customerNo, err := s.audience(ctx, callerPluginID)
	if err != nil || productdeliverycap.ValidateGetRequest(request) != nil {
		return nil, gerror.New("product delivery directory is not accessible")
	}
	var row *deliveryRow
	err = s.database.GetScan(ctx, &row, `
SELECT delivery.id,delivery.delivery_no,intent.product_name_zh_cn,intent.product_name_en_us,
       delivery.status_code,delivery.quantity,delivery.ongoing_management,
       COALESCE(task.external_reference,'') external_reference,
       delivery.started_at,delivery.completed_at,delivery.updated_at
FROM plugin_idc_core_product_delivery_record delivery
JOIN plugin_idc_core_product_fulfillment_intent intent
  ON intent.tenant_id=delivery.tenant_id AND intent.order_item_no=delivery.order_item_no
LEFT JOIN plugin_idc_core_product_fulfillment_task task
  ON task.tenant_id=intent.tenant_id AND task.intent_no=intent.intent_no
WHERE delivery.tenant_id=0 AND delivery.customer_no=? AND delivery.plugin_id=? AND delivery.delivery_no=?
LIMIT 1`, customerNo, callerPluginID, request.DeliveryNo)
	if err != nil || row == nil {
		return nil, gerror.New("product delivery directory is not accessible")
	}
	item, err := project(*row)
	if err != nil {
		return nil, err
	}
	result := &productdeliverycap.CurrentCustomerDeliveryResult{Item: item}
	if err = productdeliverycap.ValidateResult(result, request); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) audience(ctx context.Context, callerPluginID string) (string, error) {
	callerPluginID = strings.TrimSpace(callerPluginID)
	if s == nil || s.database == nil || s.principal == nil || ctx == nil || ctx.Err() != nil ||
		callerPluginID == "" || callerPluginID == productdeliverycap.OwnerPluginID {
		return "", gerror.New("product delivery audience is invalid")
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID < 1 || audience.Code != principal.AudienceCustomer ||
		audience.Status != principal.AudienceStatusActive || strings.TrimSpace(audience.CustomerNo) == "" {
		return "", gerror.New("product delivery audience is invalid")
	}
	return audience.CustomerNo, nil
}

func project(row deliveryRow) (productdeliverycap.CurrentCustomerDeliveryItem, error) {
	status := safeStatus(row.StatusCode)
	if status == "" || row.UpdatedAt == nil {
		return productdeliverycap.CurrentCustomerDeliveryItem{}, gerror.New("product delivery projection is unavailable")
	}
	item := productdeliverycap.CurrentCustomerDeliveryItem{
		DeliveryNo: row.DeliveryNo, ProductNameZhCN: row.ProductNameZhCN,
		ProductNameEnUS: row.ProductNameEnUS, Status: status, Quantity: row.Quantity,
		Ongoing: row.OngoingManagement, ProviderReference: row.ExternalReference,
		StartedAt: millis(row.StartedAt), CompletedAt: millis(row.CompletedAt), UpdatedAt: row.UpdatedAt.UTC().UnixMilli(),
	}
	return item, nil
}

func safeStatus(value string) string {
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

func millis(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UTC().UnixMilli()
}

func encodeCursor(id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
}

func decodeCursor(value string) (int64, error) {
	if value == "" {
		return int64(^uint64(0) >> 1), nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0, err
	}
	id, err := strconv.ParseInt(string(decoded), 10, 64)
	if err != nil || id < 1 || encodeCursor(id) != value {
		return 0, gerror.New("product delivery cursor is invalid")
	}
	return id, nil
}
