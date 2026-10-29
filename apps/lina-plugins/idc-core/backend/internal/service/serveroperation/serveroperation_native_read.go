// Commercial history reads one bounded current-owner snapshot. Identity and
// document ownership are SQL predicates, including after entitlement transfer;
// suspended or expired parents remain readable. No provider or writer is called.

package serveroperation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

// Verify the independent source capability without expanding the write service.
var _ servercommercialcap.CommercialReadAccess = (*nativeCommercialService)(nil)

// nativeReadCursor binds a keyset boundary to the exact reader and filter set.
type nativeReadCursor struct {
	ID    int64
	Scope string
}

// nativeOperationReadRow includes the parent sentinel for a valid empty page.
type nativeOperationReadRow struct {
	OwnerEntitlementNo string
	ID                 int64
	servercommercialcap.OperationHistory
}

// nativeBillingReadRow exposes no identity beyond the public local object key.
type nativeBillingReadRow struct {
	OwnerEntitlementNo string
	ID                 int64
	servercommercialcap.AccessoryBilling
}

// authorizeNativeCommercialRead grants customer self-read or the existing staff
// accessory-view permission with supported scope 1/2; no write grant is borrowed.
func (s *nativeCommercialService) authorizeNativeCommercialRead(ctx context.Context, customer string) error {
	if s == nil || s.database == nil || s.pricing == nil || ctx == nil || ctx.Err() != nil {
		return bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	current := s.pricing.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != 0 || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant {
		return bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	audience, err := s.pricing.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Status != principal.AudienceStatusActive {
		return bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	if audience.Code == principal.AudienceCustomer {
		subject, readErr := s.pricing.customers.CurrentCustomer(ctx)
		if readErr != nil || subject == nil || subject.CustomerNo != customer || audience.CustomerNo != customer {
			return bizerr.NewCode(CodeAccessoryPriceNotAccessible)
		}
		return nil
	}
	_, err = s.pricing.authorize(ctx, PermissionServerAccessoryView)
	return err
}

// ListOperations reads persisted native addon/IP/resize intents without guessing
// outcomes from task status or refreshing a supplier capability snapshot.
func (s *nativeCommercialService) ListOperations(ctx context.Context, input servercommercialcap.OperationListRequest) (*servercommercialcap.OperationPage, error) {
	if !validReference(input.EntitlementNo, 40) || !validReference(input.CustomerNo, 40) || input.Limit < 1 || input.Limit > 100 ||
		(input.OperationNo != "" && !validReference(input.OperationNo, 40)) {
		return nil, PublicError(ErrInvalidInput)
	}
	if err := s.authorizeNativeCommercialRead(ctx, input.CustomerNo); err != nil {
		return nil, err
	}
	id, scope, err := nativeDecodeReadCursor(input.Cursor, "operation", input.EntitlementNo, input.CustomerNo, input.OperationNo, "")
	if err != nil {
		return nil, PublicError(err)
	}
	query := nativeOperationHistorySQL
	args := []any{input.EntitlementNo, input.CustomerNo}
	if input.OperationNo != "" {
		query += " AND op.operation_no=?"
		args = append(args, input.OperationNo)
	}
	if id > 0 {
		query += " AND op.id<?"
		args = append(args, id)
	}
	query += " ORDER BY op.id DESC LIMIT ?) entry ON TRUE ORDER BY entry.id DESC NULLS LAST"
	args = append(args, input.Limit+1)
	var rows []nativeOperationReadRow
	if err = s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, PublicError(err)
	}
	if len(rows) == 0 {
		return nil, bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	page := &servercommercialcap.OperationPage{Items: make([]servercommercialcap.OperationHistory, 0, input.Limit)}
	for index, row := range rows {
		if row.ID == 0 {
			continue
		}
		if index == input.Limit {
			page.NextCursor, err = nativeEncodeReadCursor(rows[index-1].ID, scope)
			break
		}
		row.LastErrorCode = nativePublicHistoryError(row.LastErrorCode)
		page.Items = append(page.Items, row.OperationHistory)
	}
	return page, PublicError(err)
}

// ListAccessoryBilling returns only persisted object bindings, including their
// real deleted state. Pending financial references require current-owner invoices.
func (s *nativeCommercialService) ListAccessoryBilling(ctx context.Context, input servercommercialcap.AccessoryBillingListRequest) (*servercommercialcap.AccessoryBillingPage, error) {
	if !validReference(input.EntitlementNo, 40) || !validReference(input.CustomerNo, 40) || input.Limit < 1 || input.Limit > 100 ||
		(input.ObjectNo != "" && !validReference(input.ObjectNo, 128)) ||
		(input.Kind != "" && input.Kind != serveroperationresourcecap.AccessoryKindSnapshot && input.Kind != serveroperationresourcecap.AccessoryKindBackup) {
		return nil, PublicError(ErrInvalidInput)
	}
	if err := s.authorizeNativeCommercialRead(ctx, input.CustomerNo); err != nil {
		return nil, err
	}
	id, scope, err := nativeDecodeReadCursor(input.Cursor, "billing", input.EntitlementNo, input.CustomerNo, input.ObjectNo, string(input.Kind))
	if err != nil {
		return nil, PublicError(err)
	}
	query := nativeAccessoryBillingSQL
	args := []any{input.EntitlementNo, input.CustomerNo}
	if input.ObjectNo != "" {
		query += " AND accessory.object_no=?"
		args = append(args, input.ObjectNo)
	}
	if input.Kind != "" {
		query += " AND accessory.accessory_kind=?"
		args = append(args, input.Kind)
	}
	if id > 0 {
		query += " AND accessory.id<?"
		args = append(args, id)
	}
	query += " ORDER BY accessory.id DESC LIMIT ?) entry ON TRUE ORDER BY entry.id DESC NULLS LAST"
	args = append(args, input.Limit+1)
	var rows []nativeBillingReadRow
	if err = s.database.GetScan(ctx, &rows, query, args...); err != nil {
		return nil, PublicError(err)
	}
	if len(rows) == 0 {
		return nil, bizerr.NewCode(CodeAccessoryPriceNotAccessible)
	}
	page := &servercommercialcap.AccessoryBillingPage{Items: make([]servercommercialcap.AccessoryBilling, 0, input.Limit)}
	for index, row := range rows {
		if row.ID == 0 {
			continue
		}
		if index == input.Limit {
			page.NextCursor, err = nativeEncodeReadCursor(rows[index-1].ID, scope)
			break
		}
		page.Items = append(page.Items, row.AccessoryBilling)
	}
	return page, PublicError(err)
}

// nativeDecodeReadCursor rejects cursor reuse for another parent or filter set.
func nativeDecodeReadCursor(cursor, kind, parent, customer, object, filter string) (int64, string, error) {
	scope, err := digestValue(struct{ Kind, Parent, Customer, Object, Filter string }{kind, parent, customer, object, filter})
	if err != nil || cursor == "" {
		return 0, scope, err
	}
	if len(cursor) > 256 {
		return 0, "", ErrInvalidInput
	}
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0, "", ErrInvalidInput
	}
	var value nativeReadCursor
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || decoder.Decode(&struct{}{}) != io.EOF || value.ID < 1 || value.Scope != scope {
		return 0, "", ErrInvalidInput
	}
	return value.ID, scope, nil
}

// nativeEncodeReadCursor contains no financial or provider identity.
func nativeEncodeReadCursor(id int64, scope string) (string, error) {
	data, err := json.Marshal(nativeReadCursor{ID: id, Scope: scope})
	return base64.RawURLEncoding.EncodeToString(data), err
}

// nativePublicHistoryError emits only known local reason codes. A syntactically
// valid database code can still contain provider text or credentials.
func nativePublicHistoryError(code string) string {
	switch nativeReadyReason(code) {
	case nativeReadyRefund, nativeReadyParent, nativeReadyCustomer, nativeReadyVersion, nativeReadyFinancial:
		return code
	}
	switch code {
	case "server_commercial.ready_admission.capability_unavailable", "server_commercial.ready_admission.target_conflict",
		"server_commercial.ready_admission.resource_unavailable", "server_commercial.ready_admission.object_identity_unknown":
		return code
	default:
		return ""
	}
}

// nativeOperationHistorySQL keeps ownership and page contents in one snapshot.
const nativeOperationHistorySQL = `WITH owner AS (
 SELECT tenant_id,entitlement_no,customer_no FROM plugin_idc_core_service_entitlement
 WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND entitlement_owner_id='idc-resource')
SELECT owner.entitlement_no AS owner_entitlement_no,entry.* FROM owner LEFT JOIN LATERAL (
 SELECT op.id,op.operation_no,COALESCE(aq.quote_no,pq.quote_no,'') AS quote_no,
 CASE WHEN op.operation_type_code='addon' THEN COALESCE(aq.action_code,'') ELSE op.operation_type_code END AS action,
 COALESCE(document.order_no,'') AS order_no,COALESCE(invoice.invoice_no,'') AS invoice_no,
 op.currency_code,COALESCE(aq.total_amount_minor::TEXT,pq.amount_minor::TEXT,'') AS total_amount_minor,
 op.status_code AS status,op.last_error_code,op.created_at,op.updated_at,op.completed_at
 FROM plugin_idc_core_business_operation op
 LEFT JOIN plugin_idc_core_server_accessory_quote aq ON op.operation_type_code='addon' AND aq.tenant_id=op.tenant_id
  AND aq.quote_no=op.quote_no AND aq.subject_type_code='entitlement' AND aq.subject_no=op.subject_no
  AND aq.customer_no=op.customer_no AND aq.currency_code=op.currency_code
 LEFT JOIN plugin_idc_core_channel_service_quote pq ON op.operation_type_code IN ('resize','ip_change') AND pq.tenant_id=op.tenant_id
  AND pq.quote_no=op.quote_no AND pq.source_code='native' AND pq.service_no=op.subject_no
  AND pq.service_owner_customer_no=op.customer_no AND pq.payer_customer_no=op.customer_no AND pq.currency_code=op.currency_code
 LEFT JOIN plugin_idc_core_commerce_order document ON document.tenant_id=op.tenant_id AND document.order_no=op.order_no
  AND document.customer_no=op.customer_no AND document.currency_code=op.currency_code
 LEFT JOIN plugin_idc_core_billing_invoice invoice ON invoice.tenant_id=op.tenant_id AND invoice.invoice_no=op.invoice_no
  AND invoice.customer_no=op.customer_no AND invoice.currency_code=op.currency_code
  AND (op.order_no IS NULL OR invoice.order_no=op.order_no)
 WHERE op.tenant_id=owner.tenant_id AND op.subject_no=owner.entitlement_no AND op.customer_no=owner.customer_no
 AND op.subject_type_code='entitlement' AND op.service_no IS NULL AND op.resource_owner_code='idc-resource'
 AND op.operation_type_code IN ('addon','resize','ip_change')`

// nativeAccessoryBillingSQL never exposes another owner's pending invoice.
const nativeAccessoryBillingSQL = `WITH owner AS (
 SELECT tenant_id,entitlement_no,customer_no FROM plugin_idc_core_service_entitlement
 WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND entitlement_owner_id='idc-resource')
SELECT owner.entitlement_no AS owner_entitlement_no,entry.* FROM owner LEFT JOIN LATERAL (
 SELECT accessory.id,accessory.entitlement_no AS accessory_entitlement_no,accessory.object_no,accessory.accessory_kind AS kind,
 accessory.billing_mode_code AS billing_mode,accessory.status_code AS status,accessory.currency_code,
 accessory.period_unit_code AS period_unit,accessory.period_count,accessory.current_period_start_at,accessory.current_period_end_at,
 accessory.auto_renew_enabled,COALESCE(invoice.invoice_no,'') AS pending_invoice_no,
 COALESCE(invoice.status_code,'') AS pending_invoice_status,
 invoice.total_amount_minor::TEXT AS pending_amount_minor,
 accessory.created_at,accessory.updated_at
 FROM plugin_idc_core_server_accessory_entitlement accessory
 LEFT JOIN plugin_idc_core_billing_invoice invoice ON invoice.tenant_id=accessory.tenant_id
  AND invoice.invoice_no=accessory.pending_invoice_no AND invoice.customer_no=accessory.customer_no AND invoice.currency_code=accessory.currency_code
 WHERE accessory.tenant_id=owner.tenant_id AND accessory.subject_no=owner.entitlement_no AND accessory.customer_no=owner.customer_no
 AND accessory.subject_type_code='entitlement' AND accessory.object_no IS NOT NULL`
