// This file validates the optional upstream-principal Provider contract.

package resourcecap

import "github.com/gogf/gf/v2/errors/gerror"

// Validate rejects unbounded correlation references and zero username handles.
func (r PrincipalLookupRequest) Validate() error {
	if err := validateRequiredReference("principal lookup request ID", r.RequestID); err != nil {
		return err
	}
	if r.Username.IsZero() {
		return gerror.New("resource Provider principal username handle is required")
	}
	return nil
}

// Validate verifies one exact lookup projection before owner persistence.
func (r PrincipalLookupResult) Validate() error {
	if r.Exists {
		if err := validateRequiredReference("principal external reference", r.ExternalPrincipalRef); err != nil {
			return err
		}
	} else if r.ExternalPrincipalRef != "" {
		return gerror.New("resource Provider missing principal must not contain an external reference")
	}
	if err := validateObservationTime("principal observation time", r.ObservedAt); err != nil {
		return err
	}
	return validatePrincipalEvidence(r.Evidence)
}

// Validate verifies a bounded, uniquely named principal secret set. Exact
// required and optional names remain adapter-specific because not all upstream
// principal APIs expose the same profile fields.
func (r PrincipalCreateRequest) Validate() error {
	if err := validateRequiredReference("principal create request ID", r.RequestID); err != nil {
		return err
	}
	return validateSecretBindings(r.SecretBindings)
}

// Validate verifies a creation acknowledgement. It deliberately does not make
// the result sufficient for publishing an owner mapping.
func (r PrincipalCreateResult) Validate() error {
	if err := validateRequiredReference("principal external reference", r.ExternalPrincipalRef); err != nil {
		return err
	}
	if err := validateObservationTime("principal creation observation time", r.ObservedAt); err != nil {
		return err
	}
	return validatePrincipalEvidence(r.Evidence)
}

func validatePrincipalEvidence(evidence []Evidence) error {
	if len(evidence) == 0 || len(evidence) > MaximumEvidenceCount {
		return gerror.New("resource Provider principal evidence count is invalid")
	}
	for index := range evidence {
		if err := evidence[index].Validate(); err != nil {
			return err
		}
	}
	return nil
}
