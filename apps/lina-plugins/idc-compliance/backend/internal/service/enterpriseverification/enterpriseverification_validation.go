// This file validates legal identifiers, material types, and bounded text.

package enterpriseverification

import (
	"bytes"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
)

// unifiedSocialCreditAlphabet is the official base-31 character set.
const unifiedSocialCreditAlphabet = "0123456789ABCDEFGHJKLMNPQRTUWXY"

// unifiedSocialCreditWeights contains the 17 position checksum weights.
var unifiedSocialCreditWeights = [...]int{1, 3, 9, 27, 19, 26, 16, 17, 20, 29, 25, 13, 8, 24, 10, 30, 28}

type validatedLegalProfile struct {
	legalName                         []byte
	registrationIdentifier            []byte
	legalRepresentativeName           []byte
	legalRepresentativeIdentityNumber []byte
	agentName                         []byte
	agentIdentityNumber               []byte
	maskedName                        string
	maskedIdentifier                  string
}

func (p *validatedLegalProfile) clear() {
	if p == nil {
		return
	}
	clearBytes(p.legalName)
	clearBytes(p.registrationIdentifier)
	clearBytes(p.legalRepresentativeName)
	clearBytes(p.legalRepresentativeIdentityNumber)
	clearBytes(p.agentName)
	clearBytes(p.agentIdentityNumber)
}

func validateLegalProfile(input SubmitInput) (validatedLegalProfile, error) {
	name := bytes.TrimSpace(input.LegalName)
	identifier := bytes.TrimSpace(input.RegistrationIdentifier)
	representativeName := bytes.TrimSpace(input.LegalRepresentativeName)
	representativeIdentity := bytes.TrimSpace(input.LegalRepresentativeIdentityNumber)
	agentName := bytes.TrimSpace(input.AgentName)
	agentIdentity := bytes.TrimSpace(input.AgentIdentityNumber)
	if !validSensitiveText(name, 1, 128) || input.RegistrationCountryCode != "CN" ||
		!validUnifiedSocialCreditCode(identifier) || !validOptionalCode(input.IndustryCode, 64) ||
		(input.ApplicantRole != ApplicantLegalRepresentative && input.ApplicantRole != ApplicantAuthorizedAgent) ||
		!validSensitiveText(representativeName, 1, 128) ||
		!validSensitiveText(representativeIdentity, 1, 128) ||
		!validApplicantIdentity(input.ApplicantRole, agentName, agentIdentity) ||
		!validBusinessNo(input.CaseNo, 64) || !validIdempotencyKey(input.IdempotencyKey) {
		return validatedLegalProfile{}, bizerr.NewCode(CodeInvalid)
	}
	return validatedLegalProfile{
		legalName:                         append([]byte(nil), name...),
		registrationIdentifier:            append([]byte(nil), identifier...),
		legalRepresentativeName:           append([]byte(nil), representativeName...),
		legalRepresentativeIdentityNumber: append([]byte(nil), representativeIdentity...),
		agentName:                         append([]byte(nil), agentName...),
		agentIdentityNumber:               append([]byte(nil), agentIdentity...),
		maskedName:                        maskLegalName(name), maskedIdentifier: maskRegistrationIdentifier(identifier),
	}, nil
}

func validApplicantIdentity(role ApplicantRole, agentName, agentIdentity []byte) bool {
	if role == ApplicantLegalRepresentative {
		return len(agentName) == 0 && len(agentIdentity) == 0
	}
	return validSensitiveText(agentName, 1, 128) && validSensitiveText(agentIdentity, 1, 128)
}

func validUnifiedSocialCreditCode(value []byte) bool {
	if len(value) != 18 {
		return false
	}
	sum := 0
	for index := 0; index < 17; index++ {
		position := strings.IndexByte(unifiedSocialCreditAlphabet, value[index])
		if position < 0 {
			return false
		}
		sum += position * unifiedSocialCreditWeights[index]
	}
	check := (31 - sum%31) % 31
	return value[17] == unifiedSocialCreditAlphabet[check]
}

func validateMaterialType(value MaterialType) bool {
	switch value {
	case MaterialBusinessLicense, MaterialLegalRepresentativeID, MaterialAgentID,
		MaterialAuthorizationLetter, MaterialOrganizationCert, MaterialOther:
		return true
	default:
		return false
	}
}

func detectMaterialMediaType(body []byte, declared string) (string, error) {
	if len(body) == 0 || len(body) > maximumMaterialBytes {
		if len(body) > maximumMaterialBytes {
			return "", bizerr.NewCode(CodeMaterialTooLarge)
		}
		return "", bizerr.NewCode(CodeMaterialUnsupported)
	}
	detected := http.DetectContentType(body)
	switch detected {
	case "application/pdf", "image/jpeg", "image/png":
	default:
		return "", bizerr.NewCode(CodeMaterialUnsupported)
	}
	normalizedDeclared := strings.ToLower(strings.TrimSpace(strings.Split(declared, ";")[0]))
	if normalizedDeclared != "" && normalizedDeclared != "application/octet-stream" &&
		normalizedDeclared != detected {
		return "", bizerr.NewCode(CodeMaterialUnsupported)
	}
	return detected, nil
}

func validSensitiveText(value []byte, minimumRunes, maximumRunes int) bool {
	if !utf8.Valid(value) {
		return false
	}
	count := utf8.RuneCount(value)
	if count < minimumRunes || count > maximumRunes {
		return false
	}
	for remaining := value; len(remaining) > 0; {
		character, size := utf8.DecodeRune(remaining)
		if size == 0 || unicode.IsControl(character) {
			return false
		}
		remaining = remaining[size:]
	}
	return true
}

func validOptionalCode(value string, maximumBytes int) bool {
	if value == "" {
		return true
	}
	if value != strings.TrimSpace(value) || len(value) > maximumBytes {
		return false
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') &&
			character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func validBusinessNo(value string, maximumBytes int) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > maximumBytes {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '_' && character != '-' {
			return false
		}
	}
	return true
}

func validIdempotencyKey(value string) bool {
	return len(value) >= 16 && len(value) <= 128 && value == strings.TrimSpace(value) &&
		utf8.ValidString(value) && strings.IndexFunc(value, unicode.IsControl) < 0
}

func maskLegalName(value []byte) string {
	runes := []rune(string(value))
	if len(runes) == 0 {
		return ""
	}
	if len(runes) == 1 {
		return "*"
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-1)
}

func maskRegistrationIdentifier(value []byte) string {
	if len(value) != 18 {
		return ""
	}
	return string(value[:2]) + strings.Repeat("*", 12) + string(value[14:])
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
