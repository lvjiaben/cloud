// This file consumes a confirmed initial credential once before HTTP delivery.
package customeroperations

import (
	"context"
	"encoding/hex"

	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/internal/composition"
)

func encodeDigest(value [32]byte) string { return hex.EncodeToString(value[:]) }

// Reveal never returns plaintext in a service model. A lost HTTP response is
// still consumed; the client must never retry a secret delivery automatically.
func (s *Service) Reveal(ctx context.Context, serverNo string, confirmed bool, consume secretcap.Consumer) error {
	if !confirmed || consume == nil {
		return ErrInvalid
	}
	owned, err := s.ownedRoute(ctx, serverNo, false)
	if err != nil {
		return err
	}
	var handle secretcap.Handle
	err = s.entitlements.WithCurrentEntitlement(ctx, owned.EntitlementNo, func(ctx context.Context, _ *customerauthcap.Entitlement) error {
		locked, lockErr := s.ownedRoute(ctx, serverNo, true)
		if lockErr != nil || locked.LocalResourceNo != owned.LocalResourceNo || locked.ChannelNo != owned.ChannelNo {
			return ErrUnavailable
		}
		busy, checkErr := s.busy(ctx, owned.LocalResourceNo)
		if checkErr != nil || busy {
			return ErrConflict
		}
		var credential *struct{ CredentialNo, SecretRef string }
		if queryErr := s.database.GetScan(ctx, &credential, `SELECT credential.credential_no,credential.secret_ref
FROM plugin_idc_resource_provision_credential credential
JOIN plugin_idc_resource_provision_outcome_receipt receipt ON receipt.tenant_id=credential.tenant_id AND receipt.task_no=credential.task_no AND receipt.service_no=credential.service_no
JOIN plugin_idc_resource_task task ON task.tenant_id=credential.tenant_id AND task.task_no=credential.task_no
WHERE credential.tenant_id=0 AND credential.service_no=? AND credential.channel_no=? AND credential.status_code='revealable' AND receipt.customer_no=? AND receipt.status_code='succeeded' AND task.status_code='succeeded' FOR UPDATE OF credential`, owned.LocalResourceNo, owned.ChannelNo, owned.CustomerNo); queryErr != nil || credential == nil {
			return ErrUnavailable
		}
		var handleErr error
		handle, handleErr = secretcap.NewHandle(credential.SecretRef)
		if handleErr != nil {
			return ErrUnavailable
		}
		updated, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_resource_provision_credential SET status_code='revealed',revealed_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND credential_no=? AND status_code='revealable'`, credential.CredentialNo)
		if updateErr != nil {
			return ErrUnavailable
		}
		count, countErr := updated.RowsAffected()
		if countErr != nil || count != 1 {
			return ErrConflict
		}
		return s.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{IdempotencyScope: "resource.customer-credential.reveal", IdempotencyKey: credential.CredentialNo, OperationCode: "resource.customer-credential.reveal", TargetType: "server_service", TargetNo: serverNo, CustomerNo: owned.CustomerNo, ResultCode: "succeeded", ReasonCode: "customer_request", RiskCode: "high", BeforeSnapshot: map[string]any{"initial_credential_consumed": false}, AfterSnapshot: map[string]any{"initial_credential_consumed": true}})
	})
	if err != nil {
		return err
	}
	accessor, err := composition.BindProviderSecretAccessor(s.secrets, owned.ChannelNo)
	if err != nil {
		return ErrUnavailable
	}
	purpose, err := secretcap.NewPurpose("resource.customer-initial-credential")
	if err != nil {
		return ErrUnavailable
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return ErrUnavailable
	}
	if err = accessor.Access(ctx, request, func(value []byte) error {
		if len(value) < 8 || len(value) > 1024 {
			return ErrUnavailable
		}
		return consume(value)
	}); err != nil {
		return ErrUnavailable
	}
	return nil
}
