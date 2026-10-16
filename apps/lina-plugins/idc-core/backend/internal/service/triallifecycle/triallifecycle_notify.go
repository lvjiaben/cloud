// This file retries reminders independently, so unavailable contacts cannot
// roll back a confirmed resource suspension.

package triallifecycle

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func (s *serviceImpl) notify(ctx context.Context, task asynctask.ClaimedTask, payload taskPayload) error {
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.lockEntitlement(ctx, payload.Request)
		if err != nil {
			return err
		}
		if row == nil {
			return gerror.New("trial reminder ownership is unavailable")
		}
		if row.ChargeTypeCode != string(catalog.ChargeTypeTrial) || row.StatusCode != statusSuspended {
			_, err = s.tasks.CompleteInDatabase(ctx, s.database, asynctask.CompleteInput{
				Lease: task.Lease, ResultSnapshot: json.RawMessage(`{"outcome":"reminder_no_longer_due"}`),
			})
			return err
		}
		var contact *struct{ ContactNo string }
		if err = s.database.GetScan(ctx, &contact, `SELECT role.contact_no
FROM plugin_idc_core_customer_contact_role role
JOIN plugin_idc_core_customer_contact contact
 ON contact.tenant_id=role.tenant_id AND contact.customer_no=role.customer_no
 AND contact.contact_no=role.contact_no AND contact.status_code='active'
WHERE role.tenant_id=0 AND role.customer_no=? AND role.role_code='billing' AND role.status_code='active'
ORDER BY role.id LIMIT 1`, task.CustomerNo); err != nil {
			return err
		}
		if contact == nil || contact.ContactNo == "" {
			return gerror.New("trial reminder billing contact is unavailable")
		}
		receipt, err := s.notifications.Enqueue(ctx, notificationcap.EventRequest{
			EventCode: "service.expiry_reminder", Recipient: notificationcap.Recipient{
				Kind: notificationcap.RecipientCustomerContact, CustomerNo: task.CustomerNo, ContactNo: contact.ContactNo},
			BusinessObjectType: notificationcap.BusinessObjectService, BusinessObjectNo: task.SubjectNo,
			BusinessObjectVersion: row.Version,
			Variables:             map[string]string{"service_no": task.SubjectNo, "expires_at": payload.Request.ExpectedExpiresAt.UTC().Format(time.RFC3339)},
			CorrelationNo:         payload.Request.LifecycleTaskNo, IdempotencyKey: payload.Request.IdempotencyDigest + ":reminder",
		})
		if err != nil {
			return err
		}
		if receipt.EventNo == "" {
			return gerror.New("trial reminder receipt is missing")
		}
		encoded, err := json.Marshal(receipt)
		if err != nil {
			return err
		}
		_, err = s.tasks.CompleteInDatabase(ctx, s.database, asynctask.CompleteInput{Lease: task.Lease,
			ExternalReferenceNo: receipt.EventNo, ResultSnapshot: encoded})
		return err
	})
	if err != nil {
		if transitionErr := s.fail(ctx, task, true, "trial.reminder_unavailable", "Trial reminder delivery needs retry"); transitionErr != nil {
			return gerror.Wrap(transitionErr, "record trial reminder retry")
		}
		return gerror.Wrap(err, "enqueue trial reminder")
	}
	return nil
}
