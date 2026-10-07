// This file validates the closed template-management command vocabulary.

package electroniccontracttemplate

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
	"lina-plugin-idc-compliance/backend/internal/localgatewayclient"
	"lina-plugin-idc-core/backend/cap/managementcap"
)

const templateDirectoryTarget = "contract-template-directory"

var (
	templateNoPattern  = regexp.MustCompile(`^ctpl_[0-9a-f]{32}$`)
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	lowerHex64Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

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

func normalizeListInput(input ListInput) (ListInput, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ScopeCode = strings.TrimSpace(input.ScopeCode)
	input.StatusCode = strings.TrimSpace(input.StatusCode)
	if input.Page == 0 {
		input.Page = 1
	}
	if input.PageSize == 0 {
		input.PageSize = PageSize
	}
	if input.Page < 1 || input.Page > 1000000 || input.PageSize < 1 || input.PageSize > 100 || !validOptionalName(input.Name) ||
		(input.ScopeCode != "" && !validScope(input.ScopeCode)) ||
		(input.StatusCode != "" && !validStatusFilter(input.StatusCode)) {
		return ListInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeCreateInput(input CreateInput) (CreateInput, string, string, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.ScopeCode = strings.TrimSpace(input.ScopeCode)
	input.SigningOrderCode = strings.TrimSpace(input.SigningOrderCode)
	input.LanguageCode = strings.TrimSpace(input.LanguageCode)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !validName(input.Name) || !validRequiredSettings(
		input.ScopeCode, input.SigningOrderCode, input.LanguageCode,
	) ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) {
		return CreateInput{}, "", "", bizerr.NewCode(CodeInvalid)
	}
	digest := sha256Hex([]byte("contract-template-create\x00" + input.IdempotencyKey))
	fingerprint := sha256Hex([]byte(strings.Join([]string{
		"contract-template-create-v2", input.Name, input.ScopeCode, input.SigningOrderCode,
		input.LanguageCode,
	}, "\x00")))
	return input, digest, fingerprint, nil
}

func normalizeUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	input.Name = strings.TrimSpace(input.Name)
	input.ScopeCode = strings.TrimSpace(input.ScopeCode)
	input.SigningOrderCode = strings.TrimSpace(input.SigningOrderCode)
	input.LanguageCode = strings.TrimSpace(input.LanguageCode)
	if !templateNoPattern.MatchString(input.TemplateNo) || input.VersionNo <= 0 ||
		input.ExpectedVersion <= 0 || !validName(input.Name) ||
		!validRequiredSettings(input.ScopeCode, input.SigningOrderCode, input.LanguageCode) {
		return UpdateInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeStateInput(input StateInput) (StateInput, error) {
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	if !templateNoPattern.MatchString(input.TemplateNo) || input.VersionNo <= 0 || input.ExpectedVersion <= 0 {
		return StateInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func normalizeCopyInput(input CopyInput) (CopyInput, error) {
	input.SourceTemplateNo = strings.TrimSpace(input.SourceTemplateNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !templateNoPattern.MatchString(input.SourceTemplateNo) || input.SourceVersionNo <= 0 ||
		!idempotencyPattern.MatchString(input.IdempotencyKey) {
		return CopyInput{}, bizerr.NewCode(CodeInvalid)
	}
	return input, nil
}

func validName(value string) bool {
	if !validOptionalName(value) || value == "" {
		return false
	}
	return true
}

func validOptionalName(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 200 {
		return false
	}
	for _, character := range value {
		if character == 0 || unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validScope(value string) bool {
	return value == ScopePersonal || value == ScopeEnterprise || value == ScopeAllCustomers
}

func validSigningOrder(value string) bool {
	return value == SigningOrderCustomerFirst || value == SigningOrderMerchantFirst ||
		value == SigningOrderParallel
}

func validOptionalSettings(scope, order, language string) bool {
	return scope == "" && order == "" && language == "" ||
		validRequiredSettings(scope, order, language)
}

func validRequiredSettings(scope, order, language string) bool {
	return validScope(scope) && validSigningOrder(order) && validLanguage(language)
}

func validLanguage(value string) bool {
	return value == LanguageZhCN || value == LanguageEnUS
}

func validStatusFilter(value string) bool {
	switch value {
	case StatusDraft, StatusNeedsConfiguration, StatusSubmissionUnavailable, StatusSubmitted,
		StatusEnabled, StatusDisabled, StatusProviderDisabled, StatusProviderOffline:
		return true
	default:
		return false
	}
}

func newTemplateNo() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "ctpl_" + hex.EncodeToString(value), nil
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func providerSnapshotDigest(item localgatewayclient.TencentTemplate) string {
	components := append([]localgatewayclient.TencentTemplateComponent(nil), item.Components...)
	recipients := append([]localgatewayclient.TencentTemplateRecipient(nil), item.Recipients...)
	sort.Slice(components, func(left, right int) bool {
		return components[left].ComponentRef < components[right].ComponentRef
	})
	sort.Slice(recipients, func(left, right int) bool {
		return recipients[left].RecipientRef < recipients[right].RecipientRef
	})
	parts := []string{
		"tencent-template-snapshot-v2", item.TemplateRef, item.Name, item.Version,
		strconv.FormatInt(item.ProviderTimeMS, 10),
		strconv.Itoa(item.CustomerSignatureCount), strconv.Itoa(item.CustomerSealCount),
		strconv.Itoa(item.MerchantSignatureCount), strconv.Itoa(item.MerchantSealCount),
	}
	for _, component := range components {
		parts = append(parts, "component", component.ComponentRef, component.Name,
			component.TypeCode, strconv.FormatBool(component.Required), component.RecipientRef)
	}
	for _, recipient := range recipients {
		routingOrder := ""
		if recipient.RoutingOrder != nil {
			routingOrder = strconv.FormatInt(*recipient.RoutingOrder, 10)
		}
		parts = append(parts, "recipient", recipient.RecipientRef, recipient.TypeCode,
			strconv.Itoa(recipient.SequenceNo), recipient.RoleName,
			strconv.FormatBool(recipient.RequireSign), routingOrder,
			strconv.FormatBool(recipient.IsPromoter), strconv.Itoa(recipient.SignComponentCount))
	}
	return sha256Hex([]byte(strings.Join(parts, "\x00")))
}

func validProviderItem(item localgatewayclient.TencentTemplate) bool {
	return validOpaque(item.TemplateRef, 256) && validName(item.Name) && len(item.Version) <= 32 &&
		(item.ProviderState == ProviderStateActive || item.ProviderState == ProviderStateDisabled) &&
		item.ProviderTimeMS >= 0 && item.CustomerSignatureCount >= 0 &&
		item.CustomerSealCount >= 0 && item.MerchantSignatureCount >= 0 &&
		item.MerchantSealCount >= 0 && validProviderGatewayProjection(item.Components, item.Recipients)
}

func validProviderGatewayProjection(
	components []localgatewayclient.TencentTemplateComponent,
	recipients []localgatewayclient.TencentTemplateRecipient,
) bool {
	if len(components) < 1 || len(components) > maximumProviderComponents ||
		len(recipients) < 2 || len(recipients) > maximumProviderRecipients {
		return false
	}
	recipientRefs := make(map[string]struct{}, len(recipients))
	sequenceNos := make(map[int]struct{}, len(recipients))
	for _, recipient := range recipients {
		if !validOpaque(recipient.RecipientRef, 128) || !validOpaque(recipient.RoleName, 100) ||
			(recipient.TypeCode != ProviderRecipientIndividual &&
				recipient.TypeCode != ProviderRecipientEnterprise &&
				recipient.TypeCode != ProviderRecipientPromoter) ||
			recipient.SequenceNo < 1 || recipient.SequenceNo > len(recipients) ||
			recipient.SignComponentCount < 0 || recipient.RoutingOrder != nil &&
			(*recipient.RoutingOrder < 0 || *recipient.RoutingOrder > 1000000) {
			return false
		}
		if _, exists := recipientRefs[recipient.RecipientRef]; exists {
			return false
		}
		recipientRefs[recipient.RecipientRef] = struct{}{}
		if _, exists := sequenceNos[recipient.SequenceNo]; exists {
			return false
		}
		sequenceNos[recipient.SequenceNo] = struct{}{}
	}
	componentRefs := make(map[string]struct{}, len(components))
	for _, component := range components {
		if !validOpaque(component.ComponentRef, 128) || !validOpaque(component.Name, 100) ||
			!validOpaque(component.TypeCode, 64) {
			return false
		}
		if component.RecipientRef != "" {
			if _, exists := recipientRefs[component.RecipientRef]; !exists {
				return false
			}
		}
		if _, exists := componentRefs[component.ComponentRef]; exists {
			return false
		}
		componentRefs[component.ComponentRef] = struct{}{}
	}
	return true
}

func validOpaque(value string, maximum int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= maximum && !strings.ContainsAny(value, "\r\n\x00")
}

func creationFingerprintForCopy(source Item) string {
	return sha256Hex([]byte(strings.Join([]string{
		"contract-template-copy-v1", source.TemplateNo, strconv.FormatInt(source.VersionNo, 10),
	}, "\x00")))
}

func validDigest(value string) bool { return lowerHex64Pattern.MatchString(value) }
