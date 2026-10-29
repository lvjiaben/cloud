// This file binds resource-owner confirmation to the current customer and real
// session, with shared SQL policy/replay authority across application instances.
package entitlementaccess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/customerauthcap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/reauth"
)

var confirmationDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// NewWithSessionConfirmation injects the startup-owned session confirmation
// service into the same current-customer entitlement gate used by resources.
func NewWithSessionConfirmation(database gdb.DB, customers customerauthcap.Reader, confirmation reauth.SessionConfirmationService) (*Service, error) {
	if confirmation == nil {
		return nil, gerror.New("customer session confirmation is unavailable")
	}
	service, err := New(database, customers)
	if err != nil {
		return nil, err
	}
	service.confirmation = confirmation
	return service, nil
}

func (s *Service) confirmationBinding(ctx context.Context, digest string) (reauth.Binding, *customerauthcap.Subject, error) {
	if s == nil || s.Reader == nil || s.database == nil || s.confirmation == nil || ctx == nil || ctx.Err() != nil || !confirmationDigestPattern.MatchString(digest) {
		return reauth.Binding{}, nil, bizerr.NewCode(reauth.CodeInvalid)
	}
	subject, err := s.CurrentCustomer(ctx)
	if err != nil {
		return reauth.Binding{}, nil, err
	}
	if subject == nil || subject.CustomerNo == "" || subject.ActorUserID <= 0 {
		return reauth.Binding{}, nil, bizerr.NewCode(reauth.CodeProofRejected)
	}
	raw, err := json.Marshal(struct{ Domain, Customer, Operation string }{"idc-core/native-session-confirmation/v1", subject.CustomerNo, digest})
	if err != nil {
		return reauth.Binding{}, nil, bizerr.WrapCode(err, reauth.CodeUnavailable)
	}
	hash := sha256.Sum256(raw)
	return reauth.Binding{Purpose: reauth.PurposeServerOperation, TargetDigest: hex.EncodeToString(hash[:])}, subject, nil
}

func (s *Service) confirmationRequired(ctx context.Context, lock bool) (bool, error) {
	model := s.database.Model(do.ServerOperationPolicy{}).Ctx(ctx).Where(do.ServerOperationPolicy{TenantId: 0}).Fields("require_high_risk_reauth")
	if lock {
		model = model.Lock(gdb.LockForShare)
	}
	var policy *struct {
		Required bool `orm:"require_high_risk_reauth"`
	}
	if err := model.Scan(&policy); err != nil {
		return false, bizerr.WrapCode(err, reauth.CodeUnavailable)
	}
	if policy == nil {
		return false, bizerr.NewCode(reauth.CodeUnavailable)
	}
	return policy.Required, nil
}

// PrepareCurrentConfirmation never treats confirmation as resource admission.
func (s *Service) PrepareCurrentConfirmation(ctx context.Context, digest string) (*customerauthcap.SessionConfirmation, error) {
	binding, _, err := s.confirmationBinding(ctx, digest)
	if err != nil {
		return nil, err
	}
	required, err := s.confirmationRequired(ctx, false)
	if err != nil {
		return nil, err
	}
	if !required {
		return &customerauthcap.SessionConfirmation{}, nil
	}
	proof, err := s.confirmation.IssueSessionConfirmation(ctx, binding)
	if err != nil {
		return nil, err
	}
	if proof == nil || proof.Token == "" {
		return nil, bizerr.NewCode(reauth.CodeUnavailable)
	}
	return &customerauthcap.SessionConfirmation{Required: true, Token: proof.Token, ExpiresAt: proof.ExpiresAt}, nil
}

// ConsumeCurrentConfirmation keeps policy and replay consumption in the owner
// operation transaction. A policy toggle cannot reuse a previous no-proof result.
func (s *Service) ConsumeCurrentConfirmation(ctx context.Context, digest, token string) error {
	binding, subject, err := s.confirmationBinding(ctx, digest)
	if err != nil {
		return err
	}
	if gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil {
		return bizerr.NewCode(reauth.CodeInvalid)
	}
	required, err := s.confirmationRequired(ctx, true)
	if err != nil {
		return err
	}
	if !required && token == "" {
		return nil
	}
	proof, err := s.confirmation.Consume(ctx, token, binding)
	if err != nil {
		return err
	}
	if proof == nil || proof.UserID != subject.ActorUserID || proof.TenantID != 0 {
		return bizerr.NewCode(reauth.CodeProofRejected)
	}
	return nil
}
