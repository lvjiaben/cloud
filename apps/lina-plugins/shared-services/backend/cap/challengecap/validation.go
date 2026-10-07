package challengecap

import (
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

var purposePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._:-]{2,127}$`)

func ValidateCreateRequest(input CreateRequest) error {
	if err := validatePurposeAndBinding(input.Purpose, input.Binding); err != nil {
		return err
	}
	if input.Kind != PresentationImage && input.Kind != PresentationAccessible {
		return gerror.New("challengecap: unsupported presentation kind")
	}
	return nil
}

func ValidateCompleteRequest(input CompleteRequest) error {
	if err := validateOpaque(input.ChallengeNo, 16, 64, "challenge number"); err != nil {
		return err
	}
	if err := validatePurposeAndBinding(input.Purpose, input.Binding); err != nil {
		return err
	}
	if (strings.TrimSpace(input.Answer) == "") == (strings.TrimSpace(input.ProviderToken) == "") {
		return gerror.New("challengecap: exactly one challenge response is required")
	}
	if len(input.Answer) > 64 || len(input.ProviderToken) > 4096 {
		return gerror.New("challengecap: challenge response exceeds its size bound")
	}
	return nil
}

func ValidateConsumeRequest(input ConsumeRequest) error {
	if err := validateOpaque(input.Token, 32, 128, "proof token"); err != nil {
		return err
	}
	return validatePurposeAndBinding(input.Purpose, input.Binding)
}

func validatePurposeAndBinding(purpose string, binding string) error {
	if purpose != strings.TrimSpace(purpose) || !purposePattern.MatchString(purpose) {
		return gerror.New("challengecap: purpose is invalid")
	}
	if binding != strings.TrimSpace(binding) || len(binding) < 16 || len(binding) > 256 {
		return gerror.New("challengecap: binding is invalid")
	}
	return nil
}

func validateOpaque(value string, minimum int, maximum int, label string) error {
	if value != strings.TrimSpace(value) || len(value) < minimum || len(value) > maximum {
		return gerror.Newf("challengecap: %s is invalid", label)
	}
	return nil
}
