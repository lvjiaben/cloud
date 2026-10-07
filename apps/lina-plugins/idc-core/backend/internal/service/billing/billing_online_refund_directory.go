// This file provides bounded customer and staff refund directories plus one
// batched staff detail projection. Customer ownership and staff permissions are
// applied before any refund row is returned.

package billing

import (
	"context"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/customerprojection"
)

const maximumRefundDirectoryLimit = 100

type refundDirectoryBoundary struct {
	ID          int64
	SubmittedAt time.Time
}

type refundDirectoryItemRow struct {
	RefundItemNo          string
	InvoiceItemNo         string
	OrderItemNo           string
	Quantity              int
	AmountMinor           int64
	RefundableAmountMinor int64
	DescriptionZhCN       string
	DescriptionEnUS       string
}

type refundDirectoryImpactRow struct {
	RefundItemNo        string
	EntitlementNo       string
	OrderItemNo         string
	EntitlementVersion  int64
	StatusCode          string
	RequiresTermination bool
	ImpactDigest        string
}

func (s *onlineRefundService) ListOnlineRefunds(
	ctx context.Context,
	filter RefundDirectoryFilter,
) (*RefundDirectoryPage, error) {
	audience, err := currentFinancialCustomer(ctx, s.principal)
	if err != nil {
		return nil, bizerr.NewCode(CodeRefundNotAccessible)
	}
	filter.CustomerNo = audience.CustomerNo
	page, err := listRefundDirectory(ctx, s.database, filter)
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	return page, nil
}

func (s *managementRefundService) ListRefunds(
	ctx context.Context,
	filter RefundDirectoryFilter,
) (*RefundDirectoryPage, error) {
	if _, err := s.authorizer.AuthorizeRefundList(ctx); err != nil {
		return nil, err
	}
	page, err := listRefundDirectory(ctx, s.database, filter)
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	if err = projectRefundCustomers(ctx, s.database, page.Refunds); err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	return page, nil
}

func (s *managementRefundService) GetRefundDetail(
	ctx context.Context,
	refundNo string,
) (*RefundDirectoryDetail, error) {
	refundNo = strings.TrimSpace(refundNo)
	if _, err := s.authorizer.AuthorizeRefundDetail(ctx, refundNo); err != nil {
		return nil, err
	}
	request, err := s.loadManagementRefund(ctx, refundNo, false)
	if err != nil || request == nil {
		if err == nil {
			err = bizerr.NewCode(CodeRefundNotAccessible)
		}
		return nil, normalizeOnlineRefundError(err)
	}
	var (
		items   []refundDirectoryItemRow
		impacts []refundDirectoryImpactRow
	)
	if err = s.database.GetScan(ctx, &items, `
SELECT item.refund_item_no,item.invoice_item_no,item.order_item_no,
	   item.quantity,
       item.requested_amount_minor AS amount_minor,
       item.refundable_amount_minor_snapshot AS refundable_amount_minor,
       invoice_item.description_zh_cn,invoice_item.description_en_us
FROM plugin_idc_core_refund_request_item item
JOIN plugin_idc_core_billing_invoice_item invoice_item
  ON invoice_item.invoice_no=item.invoice_no
 AND invoice_item.invoice_item_no=item.invoice_item_no
 AND invoice_item.customer_no=item.customer_no
 AND invoice_item.currency_code=item.currency_code
WHERE item.tenant_id=0 AND item.refund_no=?
ORDER BY item.line_number,item.id`, refundNo); err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	if len(items) == 0 || len(items) > maximumRefundSelections {
		return nil, bizerr.NewCode(CodeRefundIntegrity)
	}
	if err = s.database.GetScan(ctx, &impacts, `
SELECT impact.refund_item_no,impact.entitlement_no,impact.order_item_no,
       impact.entitlement_version_snapshot AS entitlement_version,
       impact.status_snapshot AS status_code,
       impact.requires_termination,impact.impact_digest
FROM plugin_idc_core_refund_entitlement_impact impact
WHERE impact.tenant_id=0 AND impact.refund_no=?
ORDER BY impact.refund_item_no,impact.entitlement_no,impact.id`, refundNo); err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	byItem := make(map[string][]RefundEntitlementImpact, len(items))
	for _, impact := range impacts {
		byItem[impact.RefundItemNo] = append(byItem[impact.RefundItemNo], RefundEntitlementImpact{
			EntitlementNo: impact.EntitlementNo, OrderItemNo: impact.OrderItemNo,
			EntitlementVersion: impact.EntitlementVersion, StatusCode: impact.StatusCode,
			RequiresTermination: impact.RequiresTermination, ImpactDigest: impact.ImpactDigest,
		})
	}
	detail := &RefundDirectoryDetail{
		Refund: request.toOnlineRefund(),
		Items:  make([]RefundDirectoryItem, 0, len(items)),
	}
	if err = projectRefundCustomers(ctx, s.database, []*OnlineRefund{detail.Refund}); err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	for _, item := range items {
		detail.Items = append(detail.Items, RefundDirectoryItem{
			RefundItemNo: item.RefundItemNo, InvoiceItemNo: item.InvoiceItemNo,
			OrderItemNo: item.OrderItemNo, Quantity: item.Quantity,
			AmountMinor: item.AmountMinor, RefundableAmountMinor: item.RefundableAmountMinor,
			DescriptionZhCN: item.DescriptionZhCN, DescriptionEnUS: item.DescriptionEnUS,
			Impacts: byItem[item.RefundItemNo],
		})
	}
	return detail, nil
}

func projectRefundCustomers(ctx context.Context, database gdb.DB, refunds []*OnlineRefund) error {
	customerNos := make([]string, 0, len(refunds))
	for _, refund := range refunds {
		if refund != nil {
			customerNos = append(customerNos, refund.CustomerNo)
		}
	}
	customers, err := customerprojection.Load(ctx, database, customerNos)
	if err != nil {
		return err
	}
	for _, refund := range refunds {
		if refund == nil {
			continue
		}
		customer := customers[refund.CustomerNo]
		refund.CustomerCode, refund.CustomerDisplayName, refund.CustomerPrimaryPhone = customer.CustomerCode, customer.DisplayName, customer.PrimaryPhone
	}
	return nil
}

func listRefundDirectory(
	ctx context.Context,
	database gdb.DB,
	filter RefundDirectoryFilter,
) (*RefundDirectoryPage, error) {
	normalized, err := normalizeRefundDirectoryFilter(filter)
	if err != nil {
		return nil, err
	}
	var rows []managementRefundRow
	var total int
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if _, readErr := database.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); readErr != nil {
			return readErr
		}
		model := refundDirectoryModel(database, ctx, normalized)
		var readErr error
		total, readErr = model.Clone().Count()
		if readErr != nil {
			return readErr
		}
		if normalized.Cursor != "" {
			boundary, boundaryErr := loadRefundDirectoryBoundary(ctx, database, normalized)
			if boundaryErr != nil {
				return boundaryErr
			}
			model = model.Where("submitted_at < ? OR (submitted_at = ? AND id < ?)", boundary.SubmittedAt, boundary.SubmittedAt, boundary.ID)
		}
		return model.Fields(`refund_no,customer_no,order_no,invoice_no,currency_code,
status_code AS status,reason_code AS reason,reason_detail,
requested_amount_minor AS amount_minor,approval_mode_code AS approval_mode,
refund_destination_code AS destination,requires_entitlement_termination,
has_resource_impact,has_dangerous_operation,has_fund_anomaly,high_amount,
version,impact_digest,request_digest,submitted_at,approved_at,completed_at`).
			OrderDesc("submitted_at").OrderDesc("id").Offset(max(normalized.Page-1, 0) * normalized.Limit).Limit(normalized.Limit + 1).Scan(&rows)
	})
	if err != nil {
		return nil, err
	}
	page := &RefundDirectoryPage{Refunds: make([]*OnlineRefund, 0, min(len(rows), normalized.Limit)), Total: total}
	if len(rows) > normalized.Limit {
		rows = rows[:normalized.Limit]
		page.NextCursor = rows[len(rows)-1].RefundNo
	}
	for index := range rows {
		page.Refunds = append(page.Refunds, rows[index].toOnlineRefund())
	}
	return page, nil
}

func refundDirectoryModel(database gdb.DB, ctx context.Context, filter RefundDirectoryFilter) *gdb.Model {
	model := database.Model("plugin_idc_core_refund_request").Ctx(ctx).Where("tenant_id", 0)
	if filter.CustomerNo != "" {
		model = model.Where("customer_no", filter.CustomerNo)
	}
	if filter.Status != "" {
		model = model.Where("status_code", filter.Status)
	}
	return model
}

func loadRefundDirectoryBoundary(
	ctx context.Context,
	database gdb.DB,
	filter RefundDirectoryFilter,
) (*refundDirectoryBoundary, error) {
	var boundary *refundDirectoryBoundary
	err := refundDirectoryModel(database, ctx, filter).
		Fields("id,submitted_at").Where("refund_no", filter.Cursor).Scan(&boundary)
	if err != nil {
		return nil, err
	}
	if boundary == nil || boundary.ID < 1 {
		return nil, bizerr.NewCode(CodeRefundInvalid)
	}
	return boundary, nil
}

func normalizeRefundDirectoryFilter(filter RefundDirectoryFilter) (RefundDirectoryFilter, error) {
	filter.CustomerNo = strings.TrimSpace(filter.CustomerNo)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if (filter.CustomerNo != "" && !validBusinessReference(filter.CustomerNo, 40)) ||
		(filter.Status != "" && !validRefundStatus(filter.Status)) ||
		(filter.Cursor != "" && !validBusinessReference(filter.Cursor, 40)) ||
		filter.Limit < 1 || filter.Limit > maximumRefundDirectoryLimit || !validManagementPage(filter.Page, filter.Limit, filter.Cursor) {
		return RefundDirectoryFilter{}, bizerr.NewCode(CodeRefundInvalid)
	}
	return filter, nil
}

func validRefundStatus(value string) bool {
	switch value {
	case "pending_review", "approved", "awaiting_balance_consent", "executing",
		"succeeded", "failed", "result_unknown", "manual_confirmation", "rejected":
		return true
	default:
		return false
	}
}
