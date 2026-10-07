// Privacy governance validation normalizes bounded safe text and fixed business codes. Invalid or sensitive-looking input fails closed before persistence.

package privacygovernance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

var (
	businessNoPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
)

var aiFieldCatalog = map[string]struct{}{
	"personal_verification_status":   {},
	"enterprise_verification_status": {},
	"contract_status":                {},
	"tax_invoice_status":             {},
	"aggregate_case_count":           {},
	"aggregate_contract_count":       {},
	"aggregate_invoice_count":        {},
}

func (s *serviceImpl) authorize(ctx context.Context, permission, target string) (int, error) {
	if s == nil || ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	request := managementcap.AuthorizationRequest{Permission: permission, TargetNo: target}
	if managementcap.ValidateAuthorizationRequest(request) != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := s.runtime.Authorizer.Authorize(ctx, request)
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return actor, nil
}

func validAssessmentType(value AssessmentType) bool {
	switch value {
	case AssessmentPersonalVerification, AssessmentEnterpriseVerification,
		AssessmentElectronicContract, AssessmentTaxInvoice,
		AssessmentLocalSensitiveExport, AssessmentAISafeProjection:
		return true
	default:
		return false
	}
}

func normalizeDraft(input AssessmentDraftInput) (AssessmentDraftInput, error) {
	fields := []*string{
		&input.Purpose, &input.Necessity, &input.Impact, &input.Recipients,
		&input.StorageLocation, &input.RetentionDescription, &input.RightsEntry,
		&input.MerchantRole, &input.ThirdPartyRole, &input.ProcessingInstruction,
		&input.SubcontractingBoundary, &input.AuditEvidenceDescription,
	}
	for _, field := range fields {
		*field = strings.TrimSpace(*field)
		if !validGovernanceText(*field, 2000) {
			return AssessmentDraftInput{}, bizerr.NewCode(CodeInvalid)
		}
	}
	if !validAssessmentType(input.Type) || input.ExpectedRowVersion < 0 {
		return AssessmentDraftInput{}, bizerr.NewCode(CodeInvalid)
	}
	if utf8.RuneCountInString(input.StorageLocation) > 1000 ||
		utf8.RuneCountInString(input.RetentionDescription) > 1000 ||
		utf8.RuneCountInString(input.RightsEntry) > 1000 ||
		utf8.RuneCountInString(input.MerchantRole) > 1000 ||
		utf8.RuneCountInString(input.ThirdPartyRole) > 1000 {
		return AssessmentDraftInput{}, bizerr.NewCode(CodeInvalid)
	}
	fieldsCopy := append([]string(nil), input.AIAllowedFields...)
	for index := range fieldsCopy {
		fieldsCopy[index] = strings.TrimSpace(fieldsCopy[index])
	}
	sort.Strings(fieldsCopy)
	for index, field := range fieldsCopy {
		if _, allowed := aiFieldCatalog[field]; !allowed || index > 0 && field == fieldsCopy[index-1] {
			return AssessmentDraftInput{}, bizerr.NewCode(CodeInvalid)
		}
	}
	if input.Type != AssessmentAISafeProjection && len(fieldsCopy) != 0 {
		return AssessmentDraftInput{}, bizerr.NewCode(CodeInvalid)
	}
	input.AIAllowedFields = fieldsCopy
	return input, nil
}

func validGovernanceText(value string, maximum int) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return false
	}
	for _, character := range value {
		if character == 0 || unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t' {
			return false
		}
	}
	return true
}

func normalizePublish(input PublishAssessmentInput) (PublishAssessmentInput, string, string, error) {
	input.AssessmentNo = strings.TrimSpace(input.AssessmentNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !input.Confirmed || !businessNoPattern.MatchString(input.AssessmentNo) ||
		input.ExpectedRowVersion <= 0 || !idempotencyPattern.MatchString(input.IdempotencyKey) {
		return PublishAssessmentInput{}, "", "", bizerr.NewCode(CodeInvalid)
	}
	digest := sha256Hex("privacy-publish-key\x00" + input.IdempotencyKey)
	fingerprint := sha256Hex("privacy-publish-v1\x00" + input.AssessmentNo + "\x00" +
		strconvFormatInt(input.ExpectedRowVersion))
	return input, digest, fingerprint, nil
}

func newBusinessNo(prefix string) (string, error) {
	var content [16]byte
	if _, err := rand.Read(content[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(content[:]), nil
}

func sha256Hex(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func strconvFormatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
