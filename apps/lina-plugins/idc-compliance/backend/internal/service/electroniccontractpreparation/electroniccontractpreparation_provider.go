package electroniccontractpreparation

import (
	"bytes"
	"context"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

var tencentTemplateRefPattern = regexp.MustCompile(`^[A-Za-z0-9]{32}$`)

const (
	providerPartyCustomer = "customer"
	providerPartyMerchant = "merchant"

	providerRecipientIndividual = "INDIVIDUAL"
	providerRecipientEnterprise = "ENTERPRISE"
	providerRecipientPromoter   = "PROMOTER"

	providerValueServiceLines = "service_lines"
	providerValueSigningDate  = "signing_date"
)

type providerLaunchSnapshot struct {
	ProviderCode           string
	TemplateRef            string
	ProviderVersion        string
	ProviderSnapshotDigest string
	MappingSetNo           string
	ScopeCode              string
	SigningOrderCode       string
	LanguageCode           string
	Fields                 []providerFieldSnapshot
	Recipients             []providerRecipientSnapshot
}

type providerFieldSnapshot struct {
	ComponentRef string
	Name         string
	TypeCode     string
	Required     bool
	ValueCode    string
}

type providerRecipientSnapshot struct {
	RecipientRef       string
	SequenceNo         int
	TypeCode           string
	RequireSign        bool
	RoutingOrder       *int64
	IsPromoter         bool
	SignComponentCount int
	PartyCode          string
}

type providerHeaderRow struct {
	ProviderCode           string `orm:"provider_code"`
	TemplateRef            string `orm:"provider_template_ref"`
	ProviderVersion        string `orm:"provider_version"`
	ProviderSnapshotDigest string `orm:"provider_snapshot_digest"`
	MappingSetNo           string `orm:"current_provider_mapping_set_no"`
	ScopeCode              string `orm:"scope_code"`
	SigningOrderCode       string `orm:"signing_order_code"`
	LanguageCode           string `orm:"language_code"`
}

func loadProviderLaunchSnapshot(
	ctx context.Context,
	tx gdb.TX,
	order *contractcap.OrderSnapshot,
) (*providerLaunchSnapshot, error) {
	if ctx == nil || tx == nil || order == nil || order.Template.TemplateNo == "" ||
		order.Template.VersionNo < 1 {
		return nil, gerror.New("contract provider launch snapshot input is invalid")
	}
	var headers []providerHeaderRow
	if err := tx.Ctx(ctx).GetScan(&headers, `
SELECT template.provider_code, template.provider_template_ref,
       version.provider_version, version.provider_snapshot_digest,
       version.current_provider_mapping_set_no, version.scope_code,
       version.signing_order_code, version.language_code
FROM plugin_idc_compliance_contract_template AS template
JOIN plugin_idc_compliance_contract_template_version AS version
  ON version.tenant_id = template.tenant_id
 AND version.template_no = template.template_no
WHERE template.tenant_id = ? AND template.template_no = ?
  AND version.version_no = ? AND template.origin_code = 'tencent_synced'
  AND template.provider_code = 'tencent_essbasic'
  AND template.provider_state_code = 'active'
  AND version.status_code = 'enabled'
FOR SHARE OF template, version`, idccap.DefaultTenantID,
		order.Template.TemplateNo, order.Template.VersionNo); err != nil {
		return nil, gerror.Wrap(err, "load enabled contract provider snapshot")
	}
	if len(headers) != 1 {
		return nil, gerror.New("enabled Tencent contract template is unavailable")
	}
	header := headers[0]
	result := &providerLaunchSnapshot{
		ProviderCode: header.ProviderCode, TemplateRef: header.TemplateRef,
		ProviderVersion: header.ProviderVersion, ProviderSnapshotDigest: header.ProviderSnapshotDigest,
		MappingSetNo: header.MappingSetNo, ScopeCode: header.ScopeCode,
		SigningOrderCode: header.SigningOrderCode, LanguageCode: header.LanguageCode,
	}
	if err := tx.Ctx(ctx).GetScan(&result.Fields, `
SELECT component.component_ref, component.name, component.type_code, component.required, mapping.value_code
FROM plugin_idc_compliance_ctpl_provider_component AS component
LEFT JOIN plugin_idc_compliance_ctpl_provider_field_mapping AS mapping
  ON mapping.tenant_id = component.tenant_id
 AND mapping.template_no = component.template_no
 AND mapping.version_no = component.version_no
 AND mapping.component_ref = component.component_ref
 AND mapping.mapping_set_no = ?
WHERE component.tenant_id = ? AND component.template_no = ? AND component.version_no = ?
ORDER BY component.component_ref
FOR SHARE OF component`, result.MappingSetNo, idccap.DefaultTenantID,
		order.Template.TemplateNo, order.Template.VersionNo); err != nil {
		result.Clear()
		return nil, gerror.Wrap(err, "load contract provider field snapshot")
	}
	if err := tx.Ctx(ctx).GetScan(&result.Recipients, `
SELECT recipient.recipient_ref, recipient.sequence_no, recipient.type_code,
       recipient.require_sign, recipient.routing_order, recipient.is_promoter,
       recipient.sign_component_count, mapping.party_code
FROM plugin_idc_compliance_ctpl_provider_recipient AS recipient
LEFT JOIN plugin_idc_compliance_ctpl_provider_recipient_mapping AS mapping
  ON mapping.tenant_id = recipient.tenant_id
 AND mapping.template_no = recipient.template_no
 AND mapping.version_no = recipient.version_no
 AND mapping.recipient_ref = recipient.recipient_ref
 AND mapping.mapping_set_no = ?
WHERE recipient.tenant_id = ? AND recipient.template_no = ? AND recipient.version_no = ?
ORDER BY recipient.sequence_no
FOR SHARE OF recipient`, result.MappingSetNo, idccap.DefaultTenantID,
		order.Template.TemplateNo, order.Template.VersionNo); err != nil {
		result.Clear()
		return nil, gerror.Wrap(err, "load contract provider signer snapshot")
	}
	if err := validateProviderLaunchSnapshot(result, order); err != nil {
		result.Clear()
		return nil, err
	}
	return result, nil
}

func validateCustomerSignerSnapshot(signer *customerSignerSnapshot, order *contractcap.OrderSnapshot) error {
	if signer == nil || order == nil || signer.CustomerNo != order.Customer.CustomerNo ||
		signer.CustomerType != order.Customer.CustomerType || !validOpaqueSnapshot(signer.SignerNo, 64) ||
		len(signer.Name) == 0 || len(signer.Name) > 200 || len(signer.Phone) == 0 || len(signer.Phone) > 32 ||
		signer.IdentityVerificationVersion < 1 || signer.IdentityVerifiedAt.IsZero() ||
		!validBusinessNo(signer.PhoneVerificationNo, 64) || signer.PhoneVerificationVersion < 1 ||
		signer.PhoneVerifiedAt.IsZero() || signer.AuthorizationVersion < 1 {
		return gerror.New("frozen contract signer is invalid")
	}
	switch signer.CustomerType {
	case contractcap.CustomerTypeIndividual:
		if signer.SignerNo != "personal:"+order.Customer.VerificationCaseNo ||
			!bytes.Equal(signer.Name, order.Customer.LegalName) ||
			!bytes.Equal(signer.Phone, order.Customer.VerifiedPhone) ||
			signer.IdentityVerificationVersion != order.Customer.VerificationVersion ||
			!signer.IdentityVerifiedAt.Equal(order.Customer.VerifiedAt) ||
			signer.PhoneVerificationNo != order.Customer.PhoneContactNo ||
			signer.PhoneVerificationVersion != order.Customer.PhoneContactVersion ||
			!signer.PhoneVerifiedAt.Equal(order.Customer.PhoneVerifiedAt) ||
			signer.AuthorizationVersion != 1 {
			return gerror.New("frozen personal contract signer does not match verified customer")
		}
	case contractcap.CustomerTypeEnterprise:
		if strings.HasPrefix(signer.SignerNo, "personal:") {
			return gerror.New("frozen enterprise contract signer is invalid")
		}
	default:
		return gerror.New("frozen contract signer type is unsupported")
	}
	return nil
}

func validateProviderLaunchSnapshot(snapshot *providerLaunchSnapshot, order *contractcap.OrderSnapshot) error {
	if !validProviderLaunchHeader(snapshot, order) {
		return gerror.New("frozen contract provider snapshot is invalid")
	}
	if order.Customer.CustomerType == contractcap.CustomerTypeIndividual && snapshot.ScopeCode != "personal" ||
		order.Customer.CustomerType == contractcap.CustomerTypeEnterprise && snapshot.ScopeCode != "enterprise" {
		return gerror.New("contract provider template does not match customer type")
	}
	if err := validateProviderFields(snapshot.Fields); err != nil {
		return err
	}
	customerOrder, merchantOrder, err := validateProviderRecipients(snapshot.Recipients, order.Customer.CustomerType)
	if err != nil {
		return err
	}
	return validateProviderSigningOrder(snapshot.SigningOrderCode, customerOrder, merchantOrder)
}

func validProviderLaunchHeader(snapshot *providerLaunchSnapshot, order *contractcap.OrderSnapshot) bool {
	return snapshot != nil && order != nil && snapshot.ProviderCode == ProviderTencentESSBasic &&
		tencentTemplateRefPattern.MatchString(snapshot.TemplateRef) &&
		validOpaqueSnapshot(snapshot.ProviderVersion, 32) && validHexDigest(snapshot.ProviderSnapshotDigest) &&
		validBusinessNo(snapshot.MappingSetNo, 64) && snapshot.ScopeCode == order.Template.ScopeCode &&
		snapshot.SigningOrderCode == order.Template.SigningOrderCode &&
		snapshot.LanguageCode == order.Template.LanguageCode &&
		validProviderSigningOrderCode(snapshot.SigningOrderCode) &&
		len(snapshot.Fields) >= 1 && len(snapshot.Fields) <= 256 &&
		len(snapshot.Recipients) >= 2 && len(snapshot.Recipients) <= 64
}

func validProviderSigningOrderCode(code string) bool {
	return code == "customer_first" || code == "merchant_first" || code == "parallel"
}

func validateProviderFields(fields []providerFieldSnapshot) error {
	fieldRefs := make(map[string]struct{}, len(fields))
	mappedFieldNames := make(map[string]struct{}, len(fields))
	serviceLines := 0
	for _, field := range fields {
		if !validOpaqueSnapshot(field.ComponentRef, 128) || !validOpaqueSnapshot(field.Name, 100) ||
			!validOpaqueSnapshot(field.TypeCode, 64) ||
			field.Required && field.ValueCode == "" || field.ValueCode != "" &&
			!providerFieldValueCompatible(field.TypeCode, field.ValueCode) {
			return gerror.New("frozen contract provider field mapping is invalid")
		}
		if _, exists := fieldRefs[field.ComponentRef]; exists {
			return gerror.New("frozen contract provider field mapping is duplicated")
		}
		fieldRefs[field.ComponentRef] = struct{}{}
		if field.ValueCode != "" && field.TypeCode != "SIGN_DATE" {
			if _, exists := mappedFieldNames[field.Name]; exists {
				return gerror.New("frozen contract provider field name is ambiguous")
			}
			mappedFieldNames[field.Name] = struct{}{}
		}
		if field.ValueCode == providerValueServiceLines {
			serviceLines++
		}
	}
	if serviceLines != 1 {
		return gerror.New("frozen contract provider service table mapping is invalid")
	}
	return nil
}

func validateProviderRecipients(recipients []providerRecipientSnapshot, customerType contractcap.CustomerType) (int64, int64, error) {
	recipientRefs := make(map[string]struct{}, len(recipients))
	sequences := make(map[int]struct{}, len(recipients))
	customerOrders := make([]int64, 0, len(recipients))
	merchantOrders := make([]int64, 0, len(recipients))
	for _, recipient := range recipients {
		if !validProviderRecipient(recipient, len(recipients)) {
			return 0, 0, gerror.New("frozen contract provider signer mapping is invalid")
		}
		if _, exists := recipientRefs[recipient.RecipientRef]; exists {
			return 0, 0, gerror.New("frozen contract provider signer mapping is duplicated")
		}
		recipientRefs[recipient.RecipientRef] = struct{}{}
		if _, exists := sequences[recipient.SequenceNo]; exists {
			return 0, 0, gerror.New("frozen contract provider signer sequence is duplicated")
		}
		sequences[recipient.SequenceNo] = struct{}{}
		switch recipient.PartyCode {
		case providerPartyCustomer:
			if recipient.IsPromoter || !providerCustomerTypeMatches(customerType, recipient.TypeCode) {
				return 0, 0, gerror.New("frozen contract provider customer signer is incompatible")
			}
			customerOrders = append(customerOrders, *recipient.RoutingOrder)
		case providerPartyMerchant:
			if !recipient.IsPromoter && recipient.TypeCode != providerRecipientEnterprise {
				return 0, 0, gerror.New("frozen contract provider merchant signer is incompatible")
			}
			merchantOrders = append(merchantOrders, *recipient.RoutingOrder)
		case "":
		default:
			return 0, 0, gerror.New("frozen contract provider signer party is invalid")
		}
	}
	if len(customerOrders) != 1 || len(merchantOrders) != 1 {
		return 0, 0, gerror.New("frozen contract provider signer mapping is incomplete")
	}
	return customerOrders[0], merchantOrders[0], nil
}

func validProviderRecipient(recipient providerRecipientSnapshot, total int) bool {
	validType := recipient.TypeCode == providerRecipientIndividual ||
		recipient.TypeCode == providerRecipientEnterprise || recipient.TypeCode == providerRecipientPromoter
	validRouting := recipient.PartyCode == "" || recipient.RoutingOrder != nil &&
		*recipient.RoutingOrder >= 0 && *recipient.RoutingOrder <= 1000000
	return validOpaqueSnapshot(recipient.RecipientRef, 128) && recipient.SequenceNo >= 1 &&
		recipient.SequenceNo <= total && recipient.SignComponentCount >= 1 && validType && validRouting &&
		recipient.RequireSign && recipient.PartyCode != ""
}

func validateProviderSigningOrder(code string, customerOrder, merchantOrder int64) error {
	if code == "customer_first" && customerOrder >= merchantOrder ||
		code == "merchant_first" && merchantOrder >= customerOrder ||
		code == "parallel" && merchantOrder != customerOrder {
		return gerror.New("frozen contract provider signing order is inconsistent")
	}
	return nil
}

func providerFieldValueCompatible(typeCode, valueCode string) bool {
	switch typeCode {
	case "DYNAMIC_TABLE":
		return valueCode == providerValueServiceLines
	case "SIGN_DATE":
		return valueCode == providerValueSigningDate
	case "TEXT", "MULTI_LINE_TEXT":
		switch valueCode {
		case "contract_no", "customer_name", "merchant_name", "order_no", "total_amount", "currency":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func providerCustomerTypeMatches(customerType contractcap.CustomerType, providerType string) bool {
	return customerType == contractcap.CustomerTypeIndividual && providerType == providerRecipientIndividual ||
		customerType == contractcap.CustomerTypeEnterprise && providerType == providerRecipientEnterprise
}

func validOpaqueSnapshot(value string, maximum int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= maximum &&
		!strings.ContainsAny(value, "\r\n\x00")
}

func (snapshot *providerLaunchSnapshot) Clone() providerLaunchSnapshot {
	if snapshot == nil {
		return providerLaunchSnapshot{}
	}
	result := *snapshot
	result.Fields = append([]providerFieldSnapshot(nil), snapshot.Fields...)
	result.Recipients = make([]providerRecipientSnapshot, len(snapshot.Recipients))
	for index := range snapshot.Recipients {
		result.Recipients[index] = snapshot.Recipients[index]
		if snapshot.Recipients[index].RoutingOrder != nil {
			routingOrder := *snapshot.Recipients[index].RoutingOrder
			result.Recipients[index].RoutingOrder = &routingOrder
		}
	}
	return result
}

func (snapshot *providerLaunchSnapshot) Clear() {
	if snapshot == nil {
		return
	}
	clear(snapshot.Fields)
	snapshot.Fields = nil
	clear(snapshot.Recipients)
	snapshot.Recipients = nil
}
