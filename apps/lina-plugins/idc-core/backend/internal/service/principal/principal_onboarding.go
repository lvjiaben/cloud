// This file implements the atomic, replay-safe customer onboarding redemption transaction.

package principal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

type onboardingStatus string
type customerStatus string
type bindingSource string
type businessNumberType string
type businessNumberOwnerType string

// Stable onboarding persistence values.
const (
	onboardingStatusVerified        onboardingStatus        = "verified"
	onboardingStatusConsumed        onboardingStatus        = "consumed"
	customerStatusPendingActivation customerStatus          = "pending_activation"
	customerStatusActive            customerStatus          = "active"
	customerStatusRestricted        customerStatus          = "restricted"
	customerStatusDisabled          customerStatus          = "disabled"
	customerStatusAnonymized        customerStatus          = "anonymized"
	bindingSourceOnboarding         bindingSource           = "onboarding"
	numberTypeCustomer              businessNumberType      = "customer"
	numberTypeBinding               businessNumberType      = "customer_identity_binding"
	numberOwnerCustomer             businessNumberOwnerType = "customer"
	numberOwnerBinding              businessNumberOwnerType = "customer_identity_binding"
	maximumNumberAttempts                                   = 4
)

// redeem consumes one intent and writes the customer, audience, binding,
// business-number ledger entries, and audit evidence in one transaction.
func (p *daoPersistence) redeem(ctx context.Context, input redeemInput) (*CustomerBinding, error) {
	if p.audit == nil {
		return nil, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	var result *CustomerBinding
	err := dao.CustomerOnboardingIntent.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		intent, err := p.loadIntent(ctx, input.TokenDigest)
		if err != nil {
			return err
		}
		replay, err := validateOnboardingIntent(intent, input, p.now().UTC())
		if err != nil {
			return err
		}
		if replay {
			result, err = p.loadRedeemedBinding(ctx, intent, input.UserID)
			return err
		}
		if intent.HostRegistrationUserId == 0 {
			updateResult, updateErr := dao.CustomerOnboardingIntent.Ctx(ctx).
				Where(do.CustomerOnboardingIntent{Id: intent.Id, TenantId: idccap.DefaultTenantID, StatusCode: string(onboardingStatusVerified), Version: intent.Version}).
				Data(do.CustomerOnboardingIntent{HostRegistrationUserId: input.HostRegistrationUserID, Version: intent.Version + 1}).Update()
			if updateErr != nil {
				return gerror.Wrap(updateErr, "attach host identity to customer onboarding intent")
			}
			affected, rowsErr := updateResult.RowsAffected()
			if rowsErr != nil || affected != 1 {
				return bizerr.NewCode(CodeOnboardingUnavailable)
			}
			intent.HostRegistrationUserId = input.HostRegistrationUserID
			intent.Version++
		}

		audience, err := p.loadAudience(ctx, input.UserID, true)
		if err != nil {
			return err
		}
		if audience != nil {
			return bizerr.NewCode(CodeAudienceConflict)
		}
		binding, err := p.loadIdentityBinding(ctx, input.UserID, true)
		if err != nil {
			return err
		}
		if binding != nil {
			return bizerr.NewCode(CodeAudienceConflict)
		}

		var (
			reservationScope = digestValue("customer-onboarding:" + intent.IntentNo)
			idempotencyKey   = digestValue(intent.IntentNo)
			customerNo       string
			bindingNo        string
		)
		customerNo, err = p.reserveBusinessNumber(ctx, numberTypeCustomer, "CUS_", reservationScope, idempotencyKey)
		if err != nil {
			return err
		}
		bindingNo, err = p.reserveBusinessNumber(ctx, numberTypeBinding, "BND_", reservationScope, idempotencyKey)
		if err != nil {
			return err
		}

		insertResult, err := dao.PrincipalAudience.Ctx(ctx).Data(do.PrincipalAudience{
			TenantId:     idccap.DefaultTenantID,
			SysUserId:    input.UserID,
			AudienceCode: string(AudienceCustomer),
			StatusCode:   string(AudienceStatusActive),
			CreatedBy:    input.UserID,
			UpdatedBy:    input.UserID,
		}).InsertIgnore()
		if err != nil {
			return gerror.Wrap(err, "insert customer audience")
		}
		affected, err := insertResult.RowsAffected()
		if err != nil {
			return gerror.Wrap(err, "read inserted customer audience result")
		}
		if affected == 0 {
			return bizerr.NewCode(CodeAudienceConflict)
		}

		customerID, err := dao.CustomerSubject.Ctx(ctx).Data(do.CustomerSubject{
			TenantId:         idccap.DefaultTenantID,
			CustomerNo:       customerNo,
			CustomerTypeCode: intent.CustomerTypeCode,
			StatusCode:       string(customerStatusActive),
			DisplayName:      input.DisplayName,
			SearchName:       strings.ToLower(input.DisplayName),
			CreatedBy:        input.UserID,
			UpdatedBy:        input.UserID,
		}).InsertAndGetId()
		if err != nil {
			return gerror.Wrap(err, "insert customer subject")
		}
		if err = p.insertInitialCustomerProfile(ctx, customerNo, intent.CustomerTypeCode); err != nil {
			return err
		}
		if err = p.assignBusinessNumber(ctx, customerNo, numberOwnerCustomer, customerID); err != nil {
			return err
		}

		bindingID, err := dao.CustomerIdentityBinding.Ctx(ctx).Data(do.CustomerIdentityBinding{
			TenantId:          idccap.DefaultTenantID,
			BindingNo:         bindingNo,
			SysUserId:         input.UserID,
			CustomerNo:        customerNo,
			SourceCode:        string(bindingSourceOnboarding),
			SourceReferenceNo: intent.IntentNo,
			StatusCode:        string(AudienceStatusActive),
			CreatedBy:         input.UserID,
			UpdatedBy:         input.UserID,
		}).InsertAndGetId()
		if err != nil {
			return gerror.Wrap(err, "insert customer identity binding")
		}
		if err = p.assignBusinessNumber(ctx, bindingNo, numberOwnerBinding, bindingID); err != nil {
			return err
		}

		consumedAt := p.now().UTC()
		updateResult, err := dao.CustomerOnboardingIntent.Ctx(ctx).
			Where(do.CustomerOnboardingIntent{
				Id:         intent.Id,
				TenantId:   idccap.DefaultTenantID,
				StatusCode: string(onboardingStatusVerified),
				Version:    intent.Version,
			}).
			Data(do.CustomerOnboardingIntent{
				StatusCode:       string(onboardingStatusConsumed),
				ConsumedByUserId: input.UserID,
				ResultCustomerNo: customerNo,
				ConsumedAt:       &consumedAt,
				Version:          intent.Version + 1,
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "consume customer onboarding intent")
		}
		affected, err = updateResult.RowsAffected()
		if err != nil {
			return gerror.Wrap(err, "read onboarding consume result")
		}
		if affected != 1 {
			return bizerr.NewCode(CodeOnboardingUnavailable)
		}

		if err = p.audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "principal.customer-onboarding.redeem",
			IdempotencyKey:   intent.IntentNo,
			OperationCode:    "customer.onboarding.redeem",
			TargetType:       "customer",
			TargetNo:         customerNo,
			CustomerNo:       customerNo,
			ResultCode:       "succeeded",
			ReasonCode:       "onboarding",
			RiskCode:         "low",
			CorrelationNo:    intent.IntentNo,
			BeforeSnapshot:   map[string]any{},
			AfterSnapshot: map[string]any{
				"customer_no": customerNo,
				"type":        intent.CustomerTypeCode,
				"status":      string(customerStatusActive),
			},
		}); err != nil {
			return gerror.Wrap(err, "append onboarding audit transition")
		}

		result = &CustomerBinding{
			UserID:       input.UserID,
			CustomerNo:   customerNo,
			CustomerType: CustomerType(intent.CustomerTypeCode),
		}
		return nil
	})
	return result, err
}

// insertInitialCustomerProfile creates both personal-first identity shells in
// the same onboarding transaction. It deliberately does not copy an unverified
// display name into legal identity fields.
func (p *daoPersistence) insertInitialCustomerProfile(
	ctx context.Context,
	customerNo string,
	customerType string,
) error {
	if CustomerType(customerType) != CustomerTypeIndividual {
		return bizerr.NewCode(CodeOnboardingUnavailable)
	}
	if _, err := dao.CustomerIndividualProfile.Ctx(ctx).Data(do.CustomerIndividualProfile{
		TenantId:         idccap.DefaultTenantID,
		CustomerNo:       customerNo,
		CustomerTypeCode: string(CustomerTypeIndividual),
	}).Insert(); err != nil {
		return gerror.Wrap(err, "insert individual customer profile")
	}
	if _, err := dao.CustomerEnterpriseProfile.Ctx(ctx).Data(do.CustomerEnterpriseProfile{
		TenantId:         idccap.DefaultTenantID,
		CustomerNo:       customerNo,
		CustomerTypeCode: string(CustomerTypeEnterprise),
	}).Insert(); err != nil {
		return gerror.Wrap(err, "insert enterprise application profile")
	}
	return nil
}

// loadIntent locks the single onboarding intent selected only by its token digest.
func (p *daoPersistence) loadIntent(ctx context.Context, tokenDigest string) (*entity.CustomerOnboardingIntent, error) {
	var intent *entity.CustomerOnboardingIntent
	err := dao.CustomerOnboardingIntent.Ctx(ctx).
		Where(do.CustomerOnboardingIntent{
			TenantId:    idccap.DefaultTenantID,
			TokenDigest: tokenDigest,
		}).
		LockUpdate().
		Scan(&intent)
	if err != nil {
		return nil, gerror.Wrap(err, "load customer onboarding intent")
	}
	return intent, nil
}

// validateOnboardingIntent applies non-enumerating intent, identity, and expiry checks.
func validateOnboardingIntent(intent *entity.CustomerOnboardingIntent, input redeemInput, now time.Time) (bool, error) {
	if intent == nil || intent.TenantId != idccap.DefaultTenantID {
		return false, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	if onboardingStatus(intent.StatusCode) == onboardingStatusConsumed {
		if intent.ConsumedByUserId == input.UserID && intent.ResultCustomerNo != "" {
			return true, nil
		}
		return false, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	if onboardingStatus(intent.StatusCode) != onboardingStatusVerified ||
		intent.VerifiedContactAt == nil ||
		intent.ExpiresAt == nil ||
		!now.Before(intent.ExpiresAt.UTC()) ||
		(intent.HostRegistrationUserId != 0 && intent.HostRegistrationUserId != input.HostRegistrationUserID) ||
		intent.NormalizedUsernameHash != input.UsernameDigest ||
		intent.BrowserNonceDigest != input.BrowserDigest ||
		!validSHA256Digest(intent.ContactHash) ||
		intent.CustomerTypeCode != string(CustomerTypeIndividual) {
		return false, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	return false, nil
}

// loadRedeemedBinding verifies a consumed intent still resolves to the same active customer binding.
func (p *daoPersistence) loadRedeemedBinding(
	ctx context.Context,
	intent *entity.CustomerOnboardingIntent,
	userID int,
) (*CustomerBinding, error) {
	audience, err := p.loadAudience(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	binding, err := p.loadIdentityBinding(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	if audience == nil ||
		audience.Code != AudienceCustomer ||
		audience.Status != AudienceStatusActive ||
		binding == nil ||
		binding.StatusCode != string(AudienceStatusActive) ||
		binding.CustomerNo != intent.ResultCustomerNo {
		return nil, bizerr.NewCode(CodeOnboardingUnavailable)
	}

	customer, err := p.loadCustomerSubject(ctx, intent.ResultCustomerNo, true)
	if err != nil {
		return nil, err
	}
	if customer == nil ||
		!validCustomerType(customer.CustomerTypeCode) ||
		!validCustomerStatus(customer.StatusCode) {
		return nil, bizerr.NewCode(CodeOnboardingUnavailable)
	}
	return &CustomerBinding{
		UserID:       userID,
		CustomerNo:   customer.CustomerNo,
		CustomerType: CustomerType(customer.CustomerTypeCode),
		Reused:       true,
	}, nil
}

// reserveBusinessNumber returns an idempotently reserved opaque number inside the caller transaction.
func (p *daoPersistence) reserveBusinessNumber(
	ctx context.Context,
	numberType businessNumberType,
	prefix string,
	reservationScopeDigest string,
	idempotencyKeyDigest string,
) (string, error) {
	request := do.BusinessNumber{
		TenantId:               idccap.DefaultTenantID,
		NumberType:             string(numberType),
		ReservationScopeDigest: reservationScopeDigest,
		IdempotencyKeyDigest:   idempotencyKeyDigest,
	}
	if existing, err := p.findReservedBusinessNumber(ctx, request); err != nil || existing != "" {
		return existing, err
	}
	for range maximumNumberAttempts {
		candidate, err := newOpaqueBusinessNumber(p.random, prefix)
		if err != nil {
			return "", err
		}
		insertResult, err := dao.BusinessNumber.Ctx(ctx).Data(do.BusinessNumber{
			TenantId:               idccap.DefaultTenantID,
			NumberType:             string(numberType),
			BusinessNo:             candidate,
			ReservationScopeDigest: reservationScopeDigest,
			IdempotencyKeyDigest:   idempotencyKeyDigest,
		}).InsertIgnore()
		if err != nil {
			return "", gerror.Wrap(err, "reserve business number")
		}
		affected, err := insertResult.RowsAffected()
		if err != nil {
			return "", gerror.Wrap(err, "read business number reservation result")
		}
		if affected == 1 {
			return candidate, nil
		}
		if existing, err := p.findReservedBusinessNumber(ctx, request); err != nil || existing != "" {
			return existing, err
		}
	}
	return "", gerror.New("business number collision retry limit reached")
}

// findReservedBusinessNumber resolves one reservation by its full idempotency scope.
func (p *daoPersistence) findReservedBusinessNumber(ctx context.Context, request do.BusinessNumber) (string, error) {
	var row *entity.BusinessNumber
	if err := dao.BusinessNumber.Ctx(ctx).Where(request).Scan(&row); err != nil {
		return "", gerror.Wrap(err, "load business number reservation")
	}
	if row == nil {
		return "", nil
	}
	return row.BusinessNo, nil
}

// assignBusinessNumber freezes the internal owner of one reserved number.
func (p *daoPersistence) assignBusinessNumber(
	ctx context.Context,
	businessNo string,
	ownerType businessNumberOwnerType,
	ownerInternalID int64,
) error {
	assignedAt := p.now().UTC()
	result, err := dao.BusinessNumber.Ctx(ctx).
		Where(do.BusinessNumber{
			TenantId:   idccap.DefaultTenantID,
			BusinessNo: businessNo,
		}).
		Data(do.BusinessNumber{
			OwnerType:       string(ownerType),
			OwnerInternalId: ownerInternalID,
			AssignedAt:      &assignedAt,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "assign business number")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read business number assignment result")
	}
	if affected != 1 {
		return gerror.New("business number reservation disappeared before assignment")
	}
	return nil
}

// newOpaqueBusinessNumber generates a 128-bit opaque identifier with a stable type prefix.
func newOpaqueBusinessNumber(random io.Reader, prefix string) (string, error) {
	var content [16]byte
	if _, err := io.ReadFull(random, content[:]); err != nil {
		return "", gerror.Wrap(err, "generate opaque business number")
	}
	return prefix + hex.EncodeToString(content[:]), nil
}

// digestValue returns the lowercase SHA-256 hex digest of value.
func digestValue(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

// validSHA256Digest accepts only the lowercase hexadecimal representation used
// by onboarding creation, rejecting same-length malformed proof values.
func validSHA256Digest(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// normalizeUsername returns the canonical username form shared with intent creation.
func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

// validCustomerType reports whether value is a supported first-release customer type.
func validCustomerType(value string) bool {
	switch CustomerType(value) {
	case CustomerTypeIndividual, CustomerTypeEnterprise:
		return true
	default:
		return false
	}
}

// validCustomerStatus reports whether value is a supported customer lifecycle state.
func validCustomerStatus(value string) bool {
	switch customerStatus(value) {
	case customerStatusPendingActivation,
		customerStatusActive,
		customerStatusRestricted,
		customerStatusDisabled,
		customerStatusAnonymized:
		return true
	default:
		return false
	}
}
