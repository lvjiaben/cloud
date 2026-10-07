// This file owns immutable, provider-version-bound Tencent field and signer mappings.

package electroniccontracttemplate

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const (
	maximumProviderComponents = 512
	maximumProviderRecipients = 64
)

var providerMappingSetNoPattern = regexp.MustCompile(`^ctpms_[0-9a-f]{32}$`)

type providerFieldMappingInsert struct {
	TenantID     int    `orm:"tenant_id"`
	MappingSetNo string `orm:"mapping_set_no"`
	TemplateNo   string `orm:"template_no"`
	VersionNo    int64  `orm:"version_no"`
	ComponentRef string `orm:"component_ref"`
	ValueCode    string `orm:"value_code"`
}

type providerRecipientMappingInsert struct {
	TenantID     int    `orm:"tenant_id"`
	MappingSetNo string `orm:"mapping_set_no"`
	TemplateNo   string `orm:"template_no"`
	VersionNo    int64  `orm:"version_no"`
	RecipientRef string `orm:"recipient_ref"`
	PartyCode    string `orm:"party_code"`
}

// SaveProviderMapping confirms one complete mapping for the exact Tencent snapshot.
func (s *serviceImpl) SaveProviderMapping(
	ctx context.Context,
	input SaveProviderMappingInput,
) (*EditorDetail, error) {
	input, err := normalizeProviderMappingInput(input)
	if err != nil {
		return nil, err
	}
	actor, err := s.authorize(ctx, PermissionManage, input.TemplateNo)
	if err != nil {
		return nil, err
	}
	commandDigest := sha256Hex([]byte("contract-template-provider-mapping\x00" + input.IdempotencyKey))
	err = s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		row, loadErr := loadMutableRow(ctx, tx, input.TemplateNo, input.VersionNo)
		if loadErr != nil || row == nil {
			return firstError(loadErr, bizerr.NewCode(CodeNotAccessible))
		}
		if row.OriginCode != OriginTencentSynced || row.ProviderSnapshotDigest == nil ||
			row.ScopeCode == nil || row.SigningOrderCode == nil || row.LanguageCode == nil {
			return bizerr.NewCode(CodeConflict)
		}
		components, recipients, loadErr := loadProviderProjectionTX(
			ctx, tx, input.TemplateNo, input.VersionNo,
		)
		if loadErr != nil {
			return loadErr
		}
		if applyErr := applyProviderMappings(components, recipients, input); applyErr != nil {
			return applyErr
		}
		if reason := validateProviderMapping(
			*row.ScopeCode, *row.SigningOrderCode, components, recipients,
		); reason != "" {
			return bizerr.NewCode(CodeConflict)
		}
		fingerprint := providerMappingFingerprint(
			input.TemplateNo, input.VersionNo, *row.ProviderSnapshotDigest,
			*row.ScopeCode, *row.SigningOrderCode, components, recipients,
		)
		var existing *struct {
			MappingSetNo string `json:"mapping_set_no"`
			Fingerprint  string `json:"command_fingerprint"`
			TemplateNo   string `json:"template_no"`
			VersionNo    int64  `json:"version_no"`
		}
		if loadErr = tx.Ctx(ctx).GetScan(&existing, `
SELECT mapping_set_no, command_fingerprint, template_no, version_no
FROM plugin_idc_compliance_ctpl_provider_mapping_set
WHERE tenant_id = ? AND command_idempotency_digest = ?
FOR UPDATE`, idccap.DefaultTenantID, commandDigest); loadErr != nil {
			return loadErr
		}
		if existing != nil {
			if existing.Fingerprint != fingerprint || existing.TemplateNo != input.TemplateNo ||
				existing.VersionNo != input.VersionNo {
				return bizerr.NewCode(CodeConflict)
			}
			return nil
		}
		if row.StatusCode != StatusNeedsConfiguration || row.LockVersion != input.ExpectedVersion ||
			row.CurrentProviderMappingSetNo != nil {
			return bizerr.NewCode(CodeConflict)
		}
		mappingSetNo, createErr := newContentNo("ctpms_")
		if createErr != nil {
			return createErr
		}
		if _, createErr = tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_compliance_ctpl_provider_mapping_set
    (tenant_id, mapping_set_no, template_no, version_no, provider_snapshot_digest,
     command_idempotency_digest, command_fingerprint, created_by_user_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, idccap.DefaultTenantID, mappingSetNo,
			input.TemplateNo, input.VersionNo, *row.ProviderSnapshotDigest,
			commandDigest, fingerprint, actor); createErr != nil {
			return createErr
		}
		fieldRows := make([]providerFieldMappingInsert, 0, len(input.FieldMappings))
		for _, mapping := range input.FieldMappings {
			fieldRows = append(fieldRows, providerFieldMappingInsert{
				TenantID: idccap.DefaultTenantID, MappingSetNo: mappingSetNo,
				TemplateNo: input.TemplateNo, VersionNo: input.VersionNo,
				ComponentRef: mapping.ComponentRef, ValueCode: mapping.ValueCode,
			})
		}
		if _, createErr = tx.Ctx(ctx).
			Model("plugin_idc_compliance_ctpl_provider_field_mapping").
			Data(fieldRows).Insert(); createErr != nil {
			return createErr
		}
		recipientRows := make([]providerRecipientMappingInsert, 0, len(input.RecipientMappings))
		for _, mapping := range input.RecipientMappings {
			recipientRows = append(recipientRows, providerRecipientMappingInsert{
				TenantID: idccap.DefaultTenantID, MappingSetNo: mappingSetNo,
				TemplateNo: input.TemplateNo, VersionNo: input.VersionNo,
				RecipientRef: mapping.RecipientRef, PartyCode: mapping.PartyCode,
			})
		}
		if _, createErr = tx.Ctx(ctx).
			Model("plugin_idc_compliance_ctpl_provider_recipient_mapping").
			Data(recipientRows).Insert(); createErr != nil {
			return createErr
		}
		result, updateErr := tx.Ctx(ctx).Exec(`
UPDATE plugin_idc_compliance_contract_template_version
SET current_provider_mapping_set_no = ?, status_code = 'disabled',
    lock_version = lock_version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = ? AND template_no = ? AND version_no = ? AND lock_version = ?
  AND status_code = 'needs_configuration' AND current_provider_mapping_set_no IS NULL`,
			mappingSetNo, idccap.DefaultTenantID, input.TemplateNo,
			input.VersionNo, input.ExpectedVersion)
		if updateErr != nil {
			return updateErr
		}
		if updateErr = requireOne(result); updateErr != nil {
			return updateErr
		}
		return s.runtime.Audit.AppendBusinessTransition(ctx, idccap.BusinessAuditTransition{
			IdempotencyScope: "compliance.contract-template.provider-mapping",
			IdempotencyKey:   commandDigest,
			OperationCode:    "compliance.contract-template.provider-mapping",
			TargetType:       auditTargetType, TargetNo: input.TemplateNo,
			ResultCode: "succeeded", ReasonCode: "provider_mapping_confirmed",
			RiskCode: auditRiskHigh, RequiredPermission: PermissionManage,
			BeforeSnapshot: templateAuditSnapshot(row), AfterSnapshot: map[string]any{
				"version_no": input.VersionNo, "status": StatusDisabled,
				"field_mapping_count":     len(input.FieldMappings),
				"recipient_mapping_count": len(input.RecipientMappings),
				"lock_version":            input.ExpectedVersion + 1,
			},
		})
	})
	if err != nil {
		return nil, publicMutationError(err)
	}
	return s.GetEditor(ctx, input.TemplateNo, input.VersionNo)
}

func normalizeProviderMappingInput(input SaveProviderMappingInput) (SaveProviderMappingInput, error) {
	input.TemplateNo = strings.TrimSpace(input.TemplateNo)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if !templateNoPattern.MatchString(input.TemplateNo) || input.VersionNo <= 0 ||
		input.ExpectedVersion <= 0 || !idempotencyPattern.MatchString(input.IdempotencyKey) ||
		len(input.FieldMappings) < 1 || len(input.FieldMappings) > maximumProviderComponents ||
		len(input.RecipientMappings) < 2 || len(input.RecipientMappings) > maximumProviderRecipients {
		return SaveProviderMappingInput{}, bizerr.NewCode(CodeInvalid)
	}
	fieldSeen := make(map[string]struct{}, len(input.FieldMappings))
	for index := range input.FieldMappings {
		input.FieldMappings[index].ComponentRef = strings.TrimSpace(input.FieldMappings[index].ComponentRef)
		input.FieldMappings[index].ValueCode = strings.TrimSpace(input.FieldMappings[index].ValueCode)
		if !validOpaque(input.FieldMappings[index].ComponentRef, 128) ||
			!validProviderValueCode(input.FieldMappings[index].ValueCode) {
			return SaveProviderMappingInput{}, bizerr.NewCode(CodeInvalid)
		}
		if _, exists := fieldSeen[input.FieldMappings[index].ComponentRef]; exists {
			return SaveProviderMappingInput{}, bizerr.NewCode(CodeInvalid)
		}
		fieldSeen[input.FieldMappings[index].ComponentRef] = struct{}{}
	}
	recipientSeen := make(map[string]struct{}, len(input.RecipientMappings))
	for index := range input.RecipientMappings {
		input.RecipientMappings[index].RecipientRef = strings.TrimSpace(input.RecipientMappings[index].RecipientRef)
		input.RecipientMappings[index].PartyCode = strings.TrimSpace(input.RecipientMappings[index].PartyCode)
		if !validOpaque(input.RecipientMappings[index].RecipientRef, 128) ||
			(input.RecipientMappings[index].PartyCode != PartyCustomer &&
				input.RecipientMappings[index].PartyCode != PartyMerchant) {
			return SaveProviderMappingInput{}, bizerr.NewCode(CodeInvalid)
		}
		if _, exists := recipientSeen[input.RecipientMappings[index].RecipientRef]; exists {
			return SaveProviderMappingInput{}, bizerr.NewCode(CodeInvalid)
		}
		recipientSeen[input.RecipientMappings[index].RecipientRef] = struct{}{}
	}
	sort.Slice(input.FieldMappings, func(left, right int) bool {
		return input.FieldMappings[left].ComponentRef < input.FieldMappings[right].ComponentRef
	})
	sort.Slice(input.RecipientMappings, func(left, right int) bool {
		return input.RecipientMappings[left].RecipientRef < input.RecipientMappings[right].RecipientRef
	})
	return input, nil
}

func applyProviderMappings(
	components []ProviderComponent,
	recipients []ProviderRecipient,
	input SaveProviderMappingInput,
) error {
	componentIndex := make(map[string]int, len(components))
	for index := range components {
		componentIndex[components[index].ComponentRef] = index
	}
	for _, mapping := range input.FieldMappings {
		index, exists := componentIndex[mapping.ComponentRef]
		if !exists {
			return bizerr.NewCode(CodeConflict)
		}
		components[index].ValueCode = mapping.ValueCode
	}
	recipientIndex := make(map[string]int, len(recipients))
	for index := range recipients {
		recipientIndex[recipients[index].RecipientRef] = index
	}
	for _, mapping := range input.RecipientMappings {
		index, exists := recipientIndex[mapping.RecipientRef]
		if !exists {
			return bizerr.NewCode(CodeConflict)
		}
		recipients[index].PartyCode = mapping.PartyCode
	}
	return nil
}

func validateProviderMapping(
	scope string,
	signingOrder string,
	components []ProviderComponent,
	recipients []ProviderRecipient,
) string {
	if !validScope(scope) || !validSigningOrder(signingOrder) {
		return "provider_metadata_incomplete"
	}
	serviceLines := 0
	for _, component := range components {
		if component.Required && component.ValueCode == "" {
			return "provider_mapping_incomplete"
		}
		if component.ValueCode == "" {
			continue
		}
		if !providerComponentValueCompatible(component.TypeCode, component.ValueCode) {
			return "provider_field_incompatible"
		}
		if component.ValueCode == ProviderValueServiceLines {
			serviceLines++
		}
	}
	if serviceLines != 1 {
		return "provider_service_table_missing"
	}
	customerOrders := make([]int64, 0, len(recipients))
	merchantOrders := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		required := recipient.RequireSign || recipient.SignComponentCount > 0
		if required && recipient.PartyCode == "" {
			return "provider_mapping_incomplete"
		}
		if !required && recipient.PartyCode != "" {
			return "provider_recipient_incompatible"
		}
		if recipient.PartyCode == "" {
			continue
		}
		if recipient.RoutingOrder == nil {
			return "provider_signing_order_missing"
		}
		switch recipient.PartyCode {
		case PartyCustomer:
			if recipient.IsPromoter || !providerCustomerTypeCompatible(scope, recipient.TypeCode) {
				return "provider_scope_incompatible"
			}
			customerOrders = append(customerOrders, *recipient.RoutingOrder)
		case PartyMerchant:
			if !recipient.IsPromoter && recipient.TypeCode != ProviderRecipientEnterprise {
				return "provider_recipient_incompatible"
			}
			merchantOrders = append(merchantOrders, *recipient.RoutingOrder)
		default:
			return "provider_recipient_incompatible"
		}
	}
	if len(customerOrders) == 0 || len(merchantOrders) == 0 {
		return "provider_signers_incomplete"
	}
	for _, customerOrder := range customerOrders {
		for _, merchantOrder := range merchantOrders {
			switch signingOrder {
			case SigningOrderCustomerFirst:
				if customerOrder >= merchantOrder {
					return "provider_signing_order_mismatch"
				}
			case SigningOrderMerchantFirst:
				if merchantOrder >= customerOrder {
					return "provider_signing_order_mismatch"
				}
			case SigningOrderParallel:
				if merchantOrder != customerOrder {
					return "provider_signing_order_mismatch"
				}
			}
		}
	}
	return ""
}

func providerComponentValueCompatible(typeCode, valueCode string) bool {
	switch typeCode {
	case "DYNAMIC_TABLE":
		return valueCode == ProviderValueServiceLines
	case "SIGN_DATE":
		return valueCode == ProviderValueSigningDate
	case "TEXT", "MULTI_LINE_TEXT":
		return valueCode != ProviderValueServiceLines && valueCode != ProviderValueSigningDate &&
			validProviderValueCode(valueCode)
	default:
		return false
	}
}

func validProviderValueCode(value string) bool {
	switch value {
	case ProviderValueContractNo, ProviderValueCustomerName, ProviderValueMerchantName,
		ProviderValueOrderNo, ProviderValueServiceLines, ProviderValueTotalAmount,
		ProviderValueCurrency, ProviderValueSigningDate:
		return true
	default:
		return false
	}
}

func providerCustomerTypeCompatible(scope, typeCode string) bool {
	switch scope {
	case ScopePersonal:
		return typeCode == ProviderRecipientIndividual
	case ScopeEnterprise:
		return typeCode == ProviderRecipientEnterprise
	default:
		// Tencent's current public template contract exposes only fixed individual
		// or enterprise roles, so neither may be advertised as all customers.
		return false
	}
}

func providerScopeCompatibleTX(
	ctx context.Context,
	tx gdb.TX,
	templateNo string,
	versionNo int64,
	scope string,
) (bool, error) {
	if scope == ScopeAllCustomers {
		return false, nil
	}
	var recipients []ProviderRecipient
	if err := tx.Ctx(ctx).GetScan(&recipients, `
SELECT recipient_ref, sequence_no, type_code, role_name, require_sign, routing_order,
       is_promoter, sign_component_count, '' AS party_code
FROM plugin_idc_compliance_ctpl_provider_recipient
WHERE tenant_id = ? AND template_no = ? AND version_no = ?
ORDER BY sequence_no`, idccap.DefaultTenantID, templateNo, versionNo); err != nil {
		return false, err
	}
	for _, recipient := range recipients {
		if !recipient.IsPromoter && (recipient.RequireSign || recipient.SignComponentCount > 0) &&
			providerCustomerTypeCompatible(scope, recipient.TypeCode) {
			return true, nil
		}
	}
	return false, nil
}

func providerMappingFingerprint(
	templateNo string,
	versionNo int64,
	providerDigest string,
	scope string,
	signingOrder string,
	components []ProviderComponent,
	recipients []ProviderRecipient,
) string {
	parts := []string{
		"contract-template-provider-mapping-v1", templateNo, strconv.FormatInt(versionNo, 10),
		providerDigest, scope, signingOrder,
	}
	for _, component := range components {
		parts = append(parts, "field", component.ComponentRef, component.ValueCode)
	}
	for _, recipient := range recipients {
		parts = append(parts, "recipient", recipient.RecipientRef, recipient.PartyCode)
	}
	return sha256Hex([]byte(strings.Join(parts, "\x00")))
}

func providerMappingReason(
	item Item,
	state editorStateRow,
	components []ProviderComponent,
	recipients []ProviderRecipient,
) string {
	if item.ProviderState != ProviderStateActive {
		return "provider_unavailable"
	}
	if !validRequiredSettings(item.ScopeCode, item.SigningOrderCode, item.LanguageCode) {
		return "provider_metadata_incomplete"
	}
	if state.CurrentProviderMappingSetNo == nil {
		return "provider_mapping_incomplete"
	}
	return validateProviderMapping(item.ScopeCode, item.SigningOrderCode, components, recipients)
}

func (s *serviceImpl) loadProviderMappingProjection(
	ctx context.Context,
	templateNo string,
	versionNo int64,
	mappingSetNo *string,
) ([]ProviderComponent, []ProviderRecipient, error) {
	mapping := ""
	if mappingSetNo != nil {
		if !providerMappingSetNoPattern.MatchString(*mappingSetNo) {
			return nil, nil, bizerr.NewCode(CodeUnavailable)
		}
		mapping = *mappingSetNo
	}
	var components []ProviderComponent
	if err := s.database.GetScan(ctx, &components, `
SELECT component.component_ref, component.name, component.type_code, component.required,
       COALESCE(component.recipient_ref, '') AS recipient_ref,
       COALESCE(mapping.value_code, '') AS value_code
FROM plugin_idc_compliance_ctpl_provider_component AS component
LEFT JOIN plugin_idc_compliance_ctpl_provider_field_mapping AS mapping
  ON mapping.tenant_id = component.tenant_id
 AND mapping.template_no = component.template_no
 AND mapping.version_no = component.version_no
 AND mapping.component_ref = component.component_ref
 AND mapping.mapping_set_no = ?
WHERE component.tenant_id = ? AND component.template_no = ? AND component.version_no = ?
ORDER BY component.component_ref`, mapping, idccap.DefaultTenantID, templateNo, versionNo); err != nil {
		return nil, nil, bizerr.NewCode(CodeUnavailable)
	}
	var recipients []ProviderRecipient
	if err := s.database.GetScan(ctx, &recipients, `
SELECT recipient.recipient_ref, recipient.sequence_no, recipient.type_code, recipient.role_name,
       recipient.require_sign, recipient.routing_order, recipient.is_promoter,
       recipient.sign_component_count, COALESCE(mapping.party_code, '') AS party_code
FROM plugin_idc_compliance_ctpl_provider_recipient AS recipient
LEFT JOIN plugin_idc_compliance_ctpl_provider_recipient_mapping AS mapping
  ON mapping.tenant_id = recipient.tenant_id
 AND mapping.template_no = recipient.template_no
 AND mapping.version_no = recipient.version_no
 AND mapping.recipient_ref = recipient.recipient_ref
 AND mapping.mapping_set_no = ?
WHERE recipient.tenant_id = ? AND recipient.template_no = ? AND recipient.version_no = ?
ORDER BY recipient.sequence_no`, mapping, idccap.DefaultTenantID, templateNo, versionNo); err != nil {
		return nil, nil, bizerr.NewCode(CodeUnavailable)
	}
	if !validProviderProjection(components, recipients) {
		return nil, nil, bizerr.NewCode(CodeUnavailable)
	}
	return components, recipients, nil
}

func loadProviderProjectionTX(
	ctx context.Context,
	tx gdb.TX,
	templateNo string,
	versionNo int64,
) ([]ProviderComponent, []ProviderRecipient, error) {
	var components []ProviderComponent
	if err := tx.Ctx(ctx).GetScan(&components, `
SELECT component_ref, name, type_code, required,
       COALESCE(recipient_ref, '') AS recipient_ref, '' AS value_code
FROM plugin_idc_compliance_ctpl_provider_component
WHERE tenant_id = ? AND template_no = ? AND version_no = ?
ORDER BY component_ref
FOR SHARE`, idccap.DefaultTenantID, templateNo, versionNo); err != nil {
		return nil, nil, err
	}
	var recipients []ProviderRecipient
	if err := tx.Ctx(ctx).GetScan(&recipients, `
SELECT recipient_ref, sequence_no, type_code, role_name, require_sign, routing_order,
       is_promoter, sign_component_count, '' AS party_code
FROM plugin_idc_compliance_ctpl_provider_recipient
WHERE tenant_id = ? AND template_no = ? AND version_no = ?
ORDER BY sequence_no
FOR SHARE`, idccap.DefaultTenantID, templateNo, versionNo); err != nil {
		return nil, nil, err
	}
	if !validProviderProjection(components, recipients) {
		return nil, nil, bizerr.NewCode(CodeUnavailable)
	}
	return components, recipients, nil
}

func validProviderProjection(components []ProviderComponent, recipients []ProviderRecipient) bool {
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
			!validOpaque(component.TypeCode, 64) || component.ValueCode != "" &&
			!validProviderValueCode(component.ValueCode) {
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
