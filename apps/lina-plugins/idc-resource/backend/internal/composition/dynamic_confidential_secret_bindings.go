// This file normalizes governed secret-slot arrays inside the resource owner.
// It never resolves a handle and preserves existing operation allowlist checks.

package composition

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"strings"
)

func normalizeConfidentialSecretFormBindings(request *resourcecap.ConfidentialHTTPRequest) error {
	if request == nil {
		return gerror.New("confidential Provider secret bindings are invalid")
	}
	if len(request.SecretFormBindings) == 0 {
		return nil
	}
	if len(request.SecretFormFields) != 0 || len(request.SecretFormBindings) > 8 {
		return gerror.New("confidential Provider secret bindings conflict or exceed limit")
	}
	fields := make(map[string]string, len(request.SecretFormBindings))
	for _, binding := range request.SecretFormBindings {
		if binding.Field == "" || binding.Slot == "" || len(binding.Field) > 128 || len(binding.Slot) > 128 || strings.TrimSpace(binding.Field) != binding.Field || strings.TrimSpace(binding.Slot) != binding.Slot {
			return gerror.New("confidential Provider secret binding is invalid")
		}
		if _, exists := fields[binding.Field]; exists {
			return gerror.New("confidential Provider secret binding is duplicated")
		}
		fields[binding.Field] = binding.Slot
	}
	request.SecretFormFields = fields
	request.SecretFormBindings = nil
	return nil
}

// Validate every slot before authentication or any secret access, so a later
// invalid binding cannot trigger supplier login or partial secret resolution.
func (h *confidentialHTTPHandler) validateConfidentialOperationSlots(request resourcecap.ConfidentialHTTPRequest) error {
	for _, slot := range request.SecretFormFields {
		if _, ok := confidentialBinding(h.operationSecrets, slot); !ok {
			return bizerr.NewCode(resourcecap.CodeProviderLocalAuthPolicy)
		}
	}
	return nil
}

// accessConfidentialProviderSecret preserves only owner-reviewed classifications
// before the secret service deliberately collapses consumer errors. Plaintext,
// arbitrary error text and wrapped causes never enter the saved classification.
func accessConfidentialProviderSecret(ctx context.Context, accessor secretcap.Accessor, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	var safeFailure error
	err := accessor.Access(ctx, request, func(value []byte) error {
		consumeErr := consume(value)
		safeFailure = safeConfidentialProviderError(consumeErr)
		return consumeErr
	})
	if err != nil && safeFailure != nil {
		return safeFailure
	}
	return err
}
