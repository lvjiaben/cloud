package serveroperation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

const transferLifetime = 24 * time.Hour

type transferServiceRow struct {
	CustomerNo string `orm:"customer_no"`
	OrderNo    string `orm:"order_no"`
	Version    int64  `orm:"version"`
}

type transferRequestRow struct {
	TransferNo             string    `orm:"transfer_no"`
	ServiceNo              string    `orm:"service_no"`
	SourceCustomerNo       string    `orm:"source_customer_no"`
	TargetCustomerNo       string    `orm:"target_customer_no"`
	StatusCode             string    `orm:"status_code"`
	ExpectedServiceVersion int64     `orm:"expected_service_version"`
	ExpiresAt              time.Time `orm:"expires_at"`
}

type transferTargetCustomerRow struct {
	CustomerNo string `orm:"customer_no"`
}

// RequestCurrentCustomerTransfer intentionally returns the same receipt when
// the normalized email is not attached to one eligible customer. This keeps
// account existence out of the response while preserving an auditable digest.
func (s *accountServiceImpl) RequestCurrentCustomerTransfer(ctx context.Context,
	command TransferRequestCommand) (*TransferRequestReceipt, error) {
	if s == nil || s.database == nil || s.customers == nil || ctx == nil || ctx.Err() != nil ||
		!validReference(command.ServiceNo, 40) || !validIdempotency(command.IdempotencyKey) {
		return nil, ErrInvalidInput
	}
	email := normalizeTransferEmail(command.RecipientEmail)
	if !validTransferEmail(email) {
		return nil, ErrInvalidInput
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) {
		return nil, ErrServiceUnavailable
	}
	var receipt *TransferRequestReceipt
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		var replay *struct {
			RequestDigest string `orm:"request_digest"`
		}
		idempotencyDigest := transferDigest(command.IdempotencyKey)
		requestDigest := transferDigest(command.ServiceNo + "\x00" + email)
		if loadErr := s.database.GetScan(ctx, &replay, `SELECT request_digest
FROM plugin_idc_core_service_transfer_request
WHERE tenant_id=0 AND source_customer_no=? AND idempotency_digest=? FOR UPDATE`,
			subject.CustomerNo, idempotencyDigest); loadErr != nil {
			return loadErr
		}
		if replay != nil {
			if replay.RequestDigest != requestDigest {
				return ErrOperationConflict
			}
			receipt = &TransferRequestReceipt{Accepted: true, Reused: true}
			return nil
		}
		var enabled bool
		if loadErr := s.database.GetScan(ctx, &enabled, `SELECT allow_customer_transfer
FROM plugin_idc_core_server_operation_policy WHERE tenant_id=0`); loadErr != nil || !enabled {
			return ErrActionUnavailable
		}
		var service *transferServiceRow
		if loadErr := s.database.GetScan(ctx, &service, `SELECT customer_no,COALESCE(order_no,'') AS order_no,version
FROM plugin_idc_core_service_instance
WHERE tenant_id=0 AND service_no=? AND customer_no=?
  AND entitlement_status_code IN ('active','grace') FOR UPDATE`, command.ServiceNo, subject.CustomerNo); loadErr != nil {
			return loadErr
		}
		if service == nil || transferBlocked(ctx, s.database, command.ServiceNo, service.OrderNo) {
			return ErrActionUnavailable
		}
		// Expired requests no longer occupy the single active-transfer slot. Keep
		// this write on the explicit request command; the incoming GET stays read-only.
		if _, expireErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_service_transfer_request
SET status_code='expired',version=version+1,updated_at=?
WHERE tenant_id=0 AND service_no=? AND status_code='pending' AND expires_at<=?`,
			now, command.ServiceNo, now); expireErr != nil {
			return expireErr
		}
		var targetCustomers []transferTargetCustomerRow
		if loadErr := s.database.GetScan(ctx, &targetCustomers, `SELECT contact.customer_no
FROM plugin_idc_core_customer_contact contact
JOIN plugin_idc_core_customer_subject customer ON customer.customer_no=contact.customer_no
WHERE contact.tenant_id=0 AND lower(contact.email)=? AND contact.status_code='active'
  AND contact.email_verified_at IS NOT NULL AND customer.status_code='active'
  AND contact.customer_no<>? AND (
    (customer.customer_type_code='individual' AND EXISTS (
      SELECT 1 FROM plugin_idc_core_customer_individual_profile profile
      WHERE profile.tenant_id=0 AND profile.customer_no=customer.customer_no
        AND profile.identity_status_code='verified'
        AND (profile.identity_expires_at IS NULL OR profile.identity_expires_at>?)))
    OR (customer.customer_type_code='enterprise' AND EXISTS (
      SELECT 1 FROM plugin_idc_core_customer_enterprise_profile profile
      WHERE profile.tenant_id=0 AND profile.customer_no=customer.customer_no
        AND profile.verification_status_code='verified')))
GROUP BY contact.customer_no HAVING count(*)=1
ORDER BY contact.customer_no LIMIT 2`, email, subject.CustomerNo, now); loadErr != nil {
			return loadErr
		}
		targetCustomerNo := ""
		if len(targetCustomers) == 1 {
			targetCustomerNo = targetCustomers[0].CustomerNo
		}
		status := "pending"
		var target any = targetCustomerNo
		if targetCustomerNo == "" {
			status, target = "unmatched", nil
		}
		transferNo, numberErr := newTransferNumber("TRF")
		if numberErr != nil {
			return numberErr
		}
		_, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_transfer_request
(tenant_id,transfer_no,service_no,source_customer_no,target_customer_no,target_email_digest,
 expected_service_version,request_digest,idempotency_digest,status_code,expires_at,created_at,updated_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,?)`, transferNo, command.ServiceNo, subject.CustomerNo, target,
			transferDigest(email), service.Version, requestDigest, idempotencyDigest, status,
			now.Add(transferLifetime), now, now)
		if insertErr != nil {
			return insertErr
		}
		if auditErr := appendTransferAudit(ctx, s.operations, idccap.BusinessAuditTransition{
			IdempotencyScope: "server.transfer.request", IdempotencyKey: transferNo,
			OperationCode: "server.transfer.request", TargetType: "server_service", TargetNo: command.ServiceNo,
			CustomerNo: subject.CustomerNo, ResultCode: "accepted", ReasonCode: "customer_request",
			BeforeSnapshot: map[string]any{"service_version": service.Version},
			AfterSnapshot:  map[string]any{"recipient_matched": targetCustomerNo != "", "expires_at": now.Add(transferLifetime)},
		}); auditErr != nil {
			return auditErr
		}
		receipt = &TransferRequestReceipt{Accepted: true}
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "request customer service transfer")
	}
	return receipt, nil
}

func (s *accountServiceImpl) ListCurrentCustomerIncomingTransfers(ctx context.Context) ([]IncomingTransfer, error) {
	if s == nil || s.database == nil || s.customers == nil || ctx == nil || ctx.Err() != nil {
		return nil, ErrInvalidInput
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) {
		return nil, ErrServiceUnavailable
	}
	items := []IncomingTransfer{}
	if err = s.database.GetScan(ctx, &items, `SELECT transfer_no,service_no,expires_at
FROM plugin_idc_core_service_transfer_request
WHERE tenant_id=0 AND target_customer_no=? AND status_code='pending' AND expires_at>CURRENT_TIMESTAMP
ORDER BY expires_at,id LIMIT 50`, subject.CustomerNo); err != nil {
		return nil, gerror.Wrap(err, "list customer service transfers")
	}
	return items, nil
}

func (s *accountServiceImpl) ConfirmCurrentCustomerTransfer(ctx context.Context,
	transferNo, idempotencyKey string) (*TransferConfirmation, error) {
	if s == nil || s.database == nil || s.customers == nil || ctx == nil || ctx.Err() != nil ||
		!validReference(transferNo, 40) || !validIdempotency(idempotencyKey) {
		return nil, ErrInvalidInput
	}
	subject, err := s.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || !validReference(subject.CustomerNo, 40) {
		return nil, ErrServiceUnavailable
	}
	result := &TransferConfirmation{TransferNo: transferNo}
	err = s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now, nowErr := databaseNow(ctx, s.database)
		if nowErr != nil {
			return nowErr
		}
		var request *transferRequestRow
		if loadErr := s.database.GetScan(ctx, &request, `SELECT transfer_no,service_no,source_customer_no,
COALESCE(target_customer_no,'') AS target_customer_no,status_code,expected_service_version,expires_at
FROM plugin_idc_core_service_transfer_request
WHERE tenant_id=0 AND transfer_no=? AND target_customer_no=? FOR UPDATE`, transferNo, subject.CustomerNo); loadErr != nil {
			return loadErr
		}
		if request == nil {
			return ErrServiceUnavailable
		}
		result.ServiceNo = request.ServiceNo
		if request.StatusCode == "confirmed" {
			result.Status, result.Reused = "confirmed", true
			return nil
		}
		if request.StatusCode != "pending" || !request.ExpiresAt.After(now) {
			return ErrActionUnavailable
		}
		var service *transferServiceRow
		if loadErr := s.database.GetScan(ctx, &service, `SELECT customer_no,COALESCE(order_no,'') AS order_no,version
FROM plugin_idc_core_service_instance WHERE tenant_id=0 AND service_no=? FOR UPDATE`, request.ServiceNo); loadErr != nil {
			return loadErr
		}
		if service == nil || service.CustomerNo != request.SourceCustomerNo ||
			service.Version != request.ExpectedServiceVersion {
			return ErrActionUnavailable
		}
		if lockErr := lockTransferAccessoryEntitlements(ctx, s.database, request.ServiceNo); lockErr != nil ||
			transferBlocked(ctx, s.database, request.ServiceNo, service.OrderNo) {
			return ErrActionUnavailable
		}
		var enabled bool
		if loadErr := s.database.GetScan(ctx, &enabled, `SELECT allow_customer_transfer
FROM plugin_idc_core_server_operation_policy WHERE tenant_id=0`); loadErr != nil || !enabled {
			return ErrActionUnavailable
		}
		if transferErr := s.ownership.TransferOwnership(ctx, serveroperationresourcecap.OwnershipTransferRequest{
			TransferNo: request.TransferNo, ServiceNo: request.ServiceNo,
			SourceCustomerNo: request.SourceCustomerNo, TargetCustomerNo: subject.CustomerNo,
		}); transferErr != nil {
			return ErrActionUnavailable
		}
		update, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_core_service_instance
SET customer_no=?,ownership_started_at=?,current_renewal_policy_no=NULL,
    current_renewal_policy_version=NULL,version=version+1,updated_at=?
WHERE tenant_id=0 AND service_no=? AND customer_no=? AND version=?`, subject.CustomerNo, now, now,
			request.ServiceNo, request.SourceCustomerNo, request.ExpectedServiceVersion)
		if updateErr != nil {
			return updateErr
		}
		if affected, affectedErr := update.RowsAffected(); affectedErr != nil || affected != 1 {
			return ErrOperationConflict
		}
		if updateErr = transferEligibleAccessoryOwnership(ctx, s.database, request.ServiceNo, subject.CustomerNo, now); updateErr != nil {
			return updateErr
		}
		eventNo, numberErr := newTransferNumber("OWN")
		if numberErr != nil {
			return numberErr
		}
		if _, insertErr := s.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_ownership_event
(tenant_id,event_no,service_no,transfer_no,source_customer_no,target_customer_no,effective_at,created_at)
VALUES (0,?,?,?,?,?,?,?)`, eventNo, request.ServiceNo, request.TransferNo,
			request.SourceCustomerNo, subject.CustomerNo, now, now); insertErr != nil {
			return insertErr
		}
		if _, updateErr = s.database.Exec(ctx, `UPDATE plugin_idc_core_service_transfer_request
SET status_code='confirmed',confirmed_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND transfer_no=? AND status_code='pending'`, now, now, transferNo); updateErr != nil {
			return updateErr
		}
		if auditErr := appendTransferAudit(ctx, s.operations, idccap.BusinessAuditTransition{
			IdempotencyScope: "server.transfer.confirm", IdempotencyKey: transferNo,
			OperationCode: "server.transfer.confirm", TargetType: "server_service", TargetNo: request.ServiceNo,
			CustomerNo: subject.CustomerNo, ResultCode: "confirmed", ReasonCode: "recipient_confirmation",
			BeforeSnapshot: map[string]any{"customer_digest": transferDigest(request.SourceCustomerNo), "service_version": service.Version},
			AfterSnapshot:  map[string]any{"customer_digest": transferDigest(subject.CustomerNo), "service_version": service.Version + 1},
		}); auditErr != nil {
			return auditErr
		}
		result.Status = "confirmed"
		return nil
	})
	if err != nil {
		return nil, gerror.Wrap(err, "confirm customer service transfer")
	}
	return result, nil
}

func transferEligibleAccessoryOwnership(ctx context.Context, database gdb.DB, serviceNo, customerNo string, now time.Time) error {
	_, err := database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_entitlement
SET customer_no=?,auto_renew_enabled=FALSE,version=version+1,updated_at=?
WHERE tenant_id=0 AND service_no=? AND deleted_at IS NULL
  AND billing_mode_code='paid' AND status_code='active'
  AND current_period_end_at>? AND pending_invoice_no IS NULL`, customerNo, now, serviceNo, now)
	return err
}

func transferBlocked(ctx context.Context, database gdb.DB, serviceNo, orderNo string) bool {
	blocked, err := transferBlockState(ctx, database, serviceNo, orderNo)
	return err != nil || blocked
}

func transferBlockState(ctx context.Context, database gdb.DB, serviceNo, orderNo string) (bool, error) {
	value, err := database.GetValue(ctx, `SELECT
EXISTS (SELECT 1 FROM plugin_idc_core_server_operation_execution
 WHERE tenant_id=0 AND service_no=? AND status_code IN ('ready','pending','running','result_unknown','manual_review'))
OR EXISTS (SELECT 1
 FROM plugin_idc_core_service_instance service
 JOIN plugin_idc_core_commerce_fulfillment_unit fulfillment
   ON fulfillment.tenant_id=service.tenant_id
  AND fulfillment.fulfillment_no=service.source_fulfillment_no
 JOIN plugin_idc_core_service_entitlement entitlement
   ON entitlement.tenant_id=service.tenant_id
  AND entitlement.order_item_no=service.order_item_no
  AND entitlement.unit_sequence=fulfillment.unit_sequence
 JOIN plugin_idc_core_refund_entitlement_lock entitlement_lock
   ON entitlement_lock.tenant_id=entitlement.tenant_id
  AND entitlement_lock.entitlement_no=entitlement.entitlement_no
  AND entitlement_lock.released_at IS NULL
 WHERE service.tenant_id=0 AND service.service_no=?)
OR EXISTS (SELECT 1 FROM plugin_idc_core_billing_invoice invoice
 WHERE invoice.tenant_id=0 AND invoice.status_code IN ('draft','pending_payment','partially_paid','overdue')
   AND (invoice.order_no=? OR EXISTS (
      SELECT 1 FROM plugin_idc_core_billing_invoice_item item
      WHERE item.tenant_id=0 AND item.invoice_no=invoice.invoice_no AND item.source_reference=?)))
OR EXISTS (SELECT 1
 FROM plugin_idc_core_business_operation operation
 JOIN plugin_idc_core_billing_invoice invoice
   ON invoice.tenant_id=operation.tenant_id AND invoice.invoice_no=operation.invoice_no
 WHERE operation.tenant_id=0 AND operation.service_no=?
   AND operation.operation_type_code IN ('renewal','addon')
   AND invoice.status_code IN ('draft','pending_payment','partially_paid','overdue'))
OR EXISTS (SELECT 1
 FROM plugin_idc_core_server_accessory_entitlement accessory
 JOIN plugin_idc_core_billing_invoice invoice
   ON invoice.tenant_id=accessory.tenant_id AND invoice.invoice_no=accessory.pending_invoice_no
 WHERE accessory.tenant_id=0 AND accessory.service_no=?
   AND invoice.status_code IN ('draft','pending_payment','partially_paid','overdue'))
OR EXISTS (SELECT 1 FROM plugin_idc_core_commerce_order contract_order
 WHERE contract_order.tenant_id=0 AND contract_order.order_no=?
   AND contract_order.requires_contract
   AND contract_order.status_code IN ('paid','fulfilling','partially_fulfilled','manual_review'))`,
		serviceNo, serviceNo, orderNo, serviceNo, serviceNo, serviceNo, orderNo)
	if err != nil {
		return false, err
	}
	return value.Bool(), nil
}

func lockTransferAccessoryEntitlements(ctx context.Context, database gdb.DB, serviceNo string) error {
	var ids []int64
	return database.GetScan(ctx, &ids, `SELECT id
FROM plugin_idc_core_server_accessory_entitlement
WHERE tenant_id=0 AND service_no=? AND deleted_at IS NULL
ORDER BY id FOR UPDATE`, serviceNo)
}

type transferAuditService interface {
	AppendTransferAudit(context.Context, idccap.BusinessAuditTransition) error
}

func (s *serviceImpl) AppendTransferAudit(ctx context.Context, transition idccap.BusinessAuditTransition) error {
	return s.audit.AppendBusinessTransition(ctx, transition)
}

func appendTransferAudit(ctx context.Context, service Service, transition idccap.BusinessAuditTransition) error {
	auditor, ok := service.(transferAuditService)
	if !ok || auditor == nil {
		return ErrServiceUnavailable
	}
	return auditor.AppendTransferAudit(ctx, transition)
}

func normalizeTransferEmail(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func validTransferEmail(value string) bool {
	return len(value) >= 3 && len(value) <= 320 && strings.Count(value, "@") == 1 &&
		!strings.ContainsAny(value, " \t\r\n")
}

func validIdempotency(value string) bool {
	return len(value) >= 8 && len(value) <= 256 && value == strings.TrimSpace(value)
}

func transferDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func newTransferNumber(prefix string) (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", gerror.Wrap(err, "generate service transfer number")
	}
	return prefix + "_" + hex.EncodeToString(buffer), nil
}
