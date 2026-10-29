// This file atomically consumes verified contact proofs and invokes the existing
// profile route. Both host writes and plugin evidence share the same context.
package notification

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/profilechangecap"
	"lina-plugin-idc-notify/backend/internal/model/do"
)

// Apply permits exactly one verified contact patch for the current customer.
func (s *serviceImpl) Apply(ctx context.Context, input profilechangecap.Input, write profilechangecap.Write) error {
	if write == nil || (input.Email == nil) == (input.Phone == nil) || input.OldVerification == "" || input.NewVerification == "" || len(input.OldVerification) > 48 || len(input.NewVerification) > 48 {
		return bizerr.NewCode(CodeVerificationInvalid)
	}
	current := s.bizctx.Current(ctx)
	return publicError(s.lockContact(ctx, current.UserID, func(txCtx context.Context, revision int64) error {
		old, err := s.loadVerificationByNo(txCtx, input.OldVerification)
		if err != nil {
			return err
		}
		if old == nil {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		bindingInput := LoginContactInput{Kind: ContactEmail, OldVerification: input.OldVerification, Locale: old.LocaleCode}
		if input.Phone != nil {
			bindingInput.Kind = ContactPhone
			bindingInput.Destination = *input.Phone
		} else {
			bindingInput.Destination = *input.Email
		}
		binding, err := s.currentContact(txCtx, bindingInput, revision)
		if err != nil {
			return err
		}
		defer clear(binding.destination)
		proof, err := s.loadVerificationByNo(txCtx, input.NewVerification)
		if err != nil {
			return err
		}
		if !validContactProof(proof, binding, contactNewPurpose, binding.shape, s.now().UTC()) {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		// Consume before invoking the route. A route or later persistence failure
		// rolls back both this consumption and the original host profile write.
		result, err := s.database.Model(contactVerificationTable).Ctx(txCtx).Where(do.ContactVerification{TenantId: notificationTenantID, StatusCode: contactVerified}).WhereIn("verification_no", []string{input.OldVerification, input.NewVerification}).Data(do.ContactVerification{StatusCode: contactSuperseded}).Update()
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 2 {
			return bizerr.NewCode(CodeVerificationInvalid)
		}
		if err = write(txCtx); err != nil {
			return err
		}
		result, err = s.database.Model(contactGuardTable).Ctx(txCtx).Where(do.LoginContactGuard{TenantId: notificationTenantID, UserId: current.UserID, Revision: revision}).Data(do.LoginContactGuard{Revision: revision + 1}).Update()
		if err != nil {
			return err
		}
		return requireOne(result)
	}))
}
