// This file signs a new token only after notify has verified the same
// anonymous destination again. Expired tokens are never changed or revived.
package supportdesk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/supportcap"
)

func (s *verifiedAnonymousCreator) RenewAnonymousPresalesAccess(
	ctx context.Context,
	request supportcap.RenewAnonymousPresalesAccessRequest,
) (*supportcap.RenewAnonymousPresalesAccessResult, error) {
	if ctx == nil || ctx.Err() != nil || supportcap.ValidateRenewRequest(request) != nil {
		return nil, bizInvalid()
	}
	operationDigest, err := supportcap.RenewOperationDigest(request.OperationNo)
	if err != nil {
		return nil, bizInvalid()
	}
	requestDigest, err := anonymousRenewRequestDigest(request)
	if err != nil {
		return nil, wrapUnavailable(err, "digest anonymous support renewal")
	}
	if existing, loadErr := s.loadAnonymousCreateOperation(ctx, operationDigest); loadErr != nil {
		return nil, wrapUnavailable(loadErr, "load anonymous support renewal")
	} else if existing != nil {
		return s.recoverAnonymousRenewResult(ctx, request, operationDigest, requestDigest, existing)
	}

	secretResult, err := s.tokens.CreateGenerated(
		ctx, operationDigest, secretcap.GeneratedSecretPolicyURLSafe256,
	)
	if err != nil || secretResult.Handle.IsZero() {
		return nil, wrapUnavailable(firstAnonymousCreateError(err), "prepare renewed anonymous support token")
	}
	accessToken, err := s.readAnonymousAccessToken(ctx, secretResult.Handle)
	if err != nil {
		return nil, err
	}
	operation := anonymousCreateOperation{
		TicketNo: request.TicketNo, RequestDigest: requestDigest,
		TokenSecretRef: secretResult.Handle.OpaqueRef(), TokenDigest: anonymousTokenDigest(accessToken),
		DestinationKind: string(request.DestinationKind), DestinationDigest: request.DestinationDigest,
	}
	insertErr := s.insertAnonymousRenewal(ctx, request, operationDigest, &operation)
	if insertErr == nil {
		return anonymousRenewResult(operation, accessToken)
	}
	existing, loadErr := s.loadAnonymousCreateOperation(ctx, operationDigest)
	if loadErr == nil && existing != nil {
		return s.recoverAnonymousRenewResult(ctx, request, operationDigest, requestDigest, existing)
	}
	if _, ok := bizerr.As(insertErr); ok {
		return nil, insertErr
	}
	return nil, wrapUnavailable(insertErr, "renew anonymous support access")
}

func (s *verifiedAnonymousCreator) insertAnonymousRenewal(
	ctx context.Context,
	request supportcap.RenewAnonymousPresalesAccessRequest,
	operationDigest string,
	operation *anonymousCreateOperation,
) error {
	return s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if existing, err := s.loadAnonymousCreateOperationForUpdate(ctx, operationDigest); err != nil {
			return err
		} else if existing != nil {
			if !anonymousRenewOperationMatches(request, operation.RequestDigest, existing) ||
				existing.TokenSecretRef != operation.TokenSecretRef ||
				existing.TokenDigest != operation.TokenDigest {
				return bizerr.NewCode(CodeConflict)
			}
			operation.ExpiresAt = existing.ExpiresAt
			return nil
		}
		var tickets []struct {
			TicketNo string `orm:"ticket_no"`
		}
		if err := s.database.GetScan(ctx, &tickets, `SELECT ticket_no FROM plugin_idc_core_support_ticket WHERE tenant_id=0 AND ticket_no=? AND ticket_type='presales' AND visibility_code='anonymous' AND anonymous_destination_kind=? AND anonymous_destination_digest=? FOR UPDATE`, request.TicketNo, request.DestinationKind, request.DestinationDigest); err != nil {
			return err
		}
		if len(tickets) != 1 {
			return bizNotAccessible()
		}
		// A same-operation caller may have inserted while this transaction was
		// waiting for the ticket lock. Recheck before revoking any prior link.
		if existing, err := s.loadAnonymousCreateOperationForUpdate(ctx, operationDigest); err != nil {
			return err
		} else if existing != nil {
			if !anonymousRenewOperationMatches(request, operation.RequestDigest, existing) ||
				existing.TokenSecretRef != operation.TokenSecretRef ||
				existing.TokenDigest != operation.TokenDigest {
				return bizerr.NewCode(CodeConflict)
			}
			operation.ExpiresAt = existing.ExpiresAt
			return nil
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		if _, err = s.database.Exec(ctx, `UPDATE plugin_idc_core_support_ticket_access_link SET revoked_at=? WHERE tenant_id=0 AND ticket_no=? AND revoked_at IS NULL`, now, request.TicketNo); err != nil {
			return err
		}
		operation.ExpiresAt = anonymousAccessExpiry(now)
		linkNo := deterministicAnonymousNo("SAL_", operationDigest)
		_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_support_ticket_access_link (tenant_id,link_no,ticket_no,token_digest,destination_digest,destination_kind,operation_digest,request_digest,token_secret_ref,expires_at,created_at) VALUES (0,?,?,?,?,?,?,?,?,?,?)`, linkNo, request.TicketNo, operation.TokenDigest, request.DestinationDigest, request.DestinationKind, operationDigest, operation.RequestDigest, operation.TokenSecretRef, operation.ExpiresAt, now)
		return err
	})
}

func (s *verifiedAnonymousCreator) recoverAnonymousRenewResult(
	ctx context.Context,
	request supportcap.RenewAnonymousPresalesAccessRequest,
	operationDigest string,
	requestDigest string,
	existing *anonymousCreateOperation,
) (*supportcap.RenewAnonymousPresalesAccessResult, error) {
	if !anonymousRenewOperationMatches(request, requestDigest, existing) {
		return nil, bizerr.NewCode(CodeConflict)
	}
	secretResult, err := s.tokens.CreateGenerated(
		ctx, operationDigest, secretcap.GeneratedSecretPolicyURLSafe256,
	)
	if err != nil || secretResult.Handle.IsZero() || secretResult.Handle.OpaqueRef() != existing.TokenSecretRef {
		return nil, wrapUnavailable(firstAnonymousCreateError(err), "recover renewed anonymous support token")
	}
	token, err := s.readAnonymousAccessToken(ctx, secretResult.Handle)
	if err != nil {
		return nil, err
	}
	if anonymousTokenDigest(token) != existing.TokenDigest {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return anonymousRenewResult(*existing, token)
}

func anonymousRenewResult(
	operation anonymousCreateOperation,
	token string,
) (*supportcap.RenewAnonymousPresalesAccessResult, error) {
	result := &supportcap.RenewAnonymousPresalesAccessResult{
		TicketNo: operation.TicketNo, AccessToken: token, ExpiresAt: operation.ExpiresAt,
	}
	if supportcap.ValidateRenewResult(*result) != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return result, nil
}

func anonymousRenewOperationMatches(
	request supportcap.RenewAnonymousPresalesAccessRequest,
	requestDigest string,
	existing *anonymousCreateOperation,
) bool {
	return existing != nil && existing.TicketNo == request.TicketNo &&
		existing.RequestDigest == requestDigest &&
		existing.DestinationKind == string(request.DestinationKind) &&
		existing.DestinationDigest == request.DestinationDigest &&
		existing.TokenSecretRef != "" && existing.TokenDigest != "" && existing.RevokedAt == nil
}

func anonymousRenewRequestDigest(request supportcap.RenewAnonymousPresalesAccessRequest) (string, error) {
	payload, err := json.Marshal(struct {
		ContractVersion   string
		Operation         string
		DestinationKind   supportcap.DestinationKind
		DestinationDigest string
		TicketNo          string
	}{
		ContractVersion: supportcap.ContractVersion, Operation: "renew",
		DestinationKind: request.DestinationKind, DestinationDigest: request.DestinationDigest,
		TicketNo: request.TicketNo,
	})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:]), nil
}
