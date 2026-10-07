// Package contractsnapshot reads the core-owned order and verified customer
// truth required by the compliance contract workflow.
package contractsnapshot

import (
	"context"
	"crypto/sha256"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/contractcap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

var (
	orderNoPattern    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,39}$`)
	lowerHex64Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type serviceImpl struct {
	database  gdb.DB
	lookupKey []byte
}

var _ contractcap.Service = (*serviceImpl)(nil)

// New constructs the source-only contract snapshot reader.
func New(database gdb.DB, lookupKey []byte) (contractcap.Service, error) {
	if nilDependency(database) || len(lookupKey) != sha256.Size {
		clear(lookupKey)
		return nil, gerror.New("contract order snapshot database is unavailable")
	}
	return &serviceImpl{database: database, lookupKey: append([]byte(nil), lookupKey...)}, nil
}

type orderRow struct {
	OrderNo                    string `orm:"order_no"`
	CustomerNo                 string `orm:"customer_no"`
	CustomerTypeCode           string `orm:"customer_type_code"`
	StatusCode                 string `orm:"status_code"`
	CurrencyCode               string `orm:"currency_code"`
	TotalAmountMinor           int64  `orm:"total_amount_minor"`
	RequiresContract           bool   `orm:"requires_contract"`
	Version                    int64  `orm:"version"`
	ContractTemplateNo         string `orm:"contract_template_no"`
	ContractTemplateVersionNo  int64  `orm:"contract_template_version_no"`
	ContractTemplateName       string `orm:"contract_template_name"`
	ContractLanguageCode       string `orm:"contract_language_code"`
	ContractScopeCode          string `orm:"contract_scope_code"`
	ContractSigningOrderCode   string `orm:"contract_signing_order_code"`
	MerchantNo                 string `orm:"merchant_no"`
	MerchantLegalName          string `orm:"merchant_legal_name"`
	MerchantCountryCode        string `orm:"merchant_country_code"`
	MerchantVersion            int64  `orm:"merchant_version"`
	InvoiceStatusCode          string `orm:"invoice_status_code"`
	InvoicePaidAmountMinor     int64  `orm:"invoice_paid_amount_minor"`
	InvoiceRefundedAmountMinor int64  `orm:"invoice_refunded_amount_minor"`
}

type personalRow struct {
	LegalName           string     `orm:"legal_name"`
	LookupDigest        string     `orm:"identity_lookup_digest"`
	CaseNo              string     `orm:"identity_case_no"`
	StatusCode          string     `orm:"identity_status_code"`
	VerifiedAt          *time.Time `orm:"identity_verified_at"`
	ExpiresAt           *time.Time `orm:"identity_expires_at"`
	VerificationVersion int64      `orm:"version"`
}

type verifiedPhoneRow struct {
	ContactNo       string     `orm:"contact_no"`
	CountryCode     string     `orm:"phone_country_code"`
	Phone           string     `orm:"phone"`
	PhoneVerifiedAt *time.Time `orm:"phone_verified_at"`
	Version         int64      `orm:"version"`
}

type enterpriseRow struct {
	LegalName                    string     `orm:"legal_name"`
	LookupDigest                 string     `orm:"registration_lookup_digest"`
	RegistrationCountryCode      string     `orm:"registration_country_code"`
	RegistrationIdentifier       string     `orm:"registration_identifier"`
	RegistrationIdentifierMasked string     `orm:"registration_identifier_masked"`
	CaseNo                       string     `orm:"verification_case_no"`
	StatusCode                   string     `orm:"verification_status_code"`
	VerifiedAt                   *time.Time `orm:"verified_at"`
	ExpiresAt                    *time.Time `orm:"expires_at"`
	VerificationVersion          int64      `orm:"version"`
}

type itemRow struct {
	OrderItemNo      string `orm:"order_item_no"`
	ProductNo        string `orm:"product_no"`
	ProductNameZhCN  string `orm:"product_name_zh_cn"`
	ProductNameEnUS  string `orm:"product_name_en_us"`
	Quantity         int    `orm:"quantity"`
	PurchaseCycles   int    `orm:"purchase_cycles"`
	PeriodUnitCode   string `orm:"period_unit_code"`
	PeriodCount      int    `orm:"period_count"`
	RequiresContract bool   `orm:"requires_contract"`
	TotalAmountMinor int64  `orm:"total_amount_minor"`
}

// PrepareOrder returns one repeatable snapshot and refuses missing, terminal,
// unverified, expired, oversized, or internally inconsistent business state.
func (s *serviceImpl) PrepareOrder(ctx context.Context, orderNo string) (*contractcap.OrderSnapshot, error) {
	return s.prepareOrder(ctx, orderNo, true)
}

// PrepareOptionalOrder returns the same authoritative order truth for an
// administrator-started optional contract. It never changes the order's
// frozen contract requirement or fulfillment path.
func (s *serviceImpl) PrepareOptionalOrder(
	ctx context.Context,
	orderNo string,
) (*contractcap.OrderSnapshot, error) {
	return s.prepareOrder(ctx, orderNo, false)
}

func (s *serviceImpl) prepareOrder(
	ctx context.Context,
	orderNo string,
	requiredForFulfillment bool,
) (*contractcap.OrderSnapshot, error) {
	if s == nil || ctx == nil || ctx.Err() != nil || !orderNoPattern.MatchString(orderNo) {
		return nil, gerror.New("contract order snapshot is not accessible")
	}
	var snapshot *contractcap.OrderSnapshot
	err := s.database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SET TRANSACTION ISOLATION LEVEL REPEATABLE READ, READ ONLY"); err != nil {
			return err
		}
		var current time.Time
		if value, err := tx.GetValue("SELECT CURRENT_TIMESTAMP"); err != nil {
			return err
		} else {
			current = value.Time().UTC()
		}
		var order *orderRow
		if err := tx.GetScan(&order, `
SELECT order_row.order_no, order_row.customer_no, customer.customer_type_code,
       order_row.status_code, order_row.currency_code, order_row.total_amount_minor,
       order_row.requires_contract, order_row.version,
       order_row.contract_template_no, order_row.contract_template_version_no,
       order_row.contract_template_name, order_row.contract_language_code,
       order_row.contract_scope_code, order_row.contract_signing_order_code,
       merchant.merchant_no, merchant.legal_name AS merchant_legal_name,
       merchant.country_code AS merchant_country_code, merchant.version AS merchant_version,
       invoice.status_code AS invoice_status_code,
       invoice.paid_amount_minor AS invoice_paid_amount_minor,
       invoice.refunded_amount_minor AS invoice_refunded_amount_minor
FROM plugin_idc_core_commerce_order AS order_row
JOIN plugin_idc_core_customer_subject AS customer
  ON customer.tenant_id = order_row.tenant_id
 AND customer.customer_no = order_row.customer_no
JOIN plugin_idc_core_merchant_profile AS merchant
  ON merchant.tenant_id = order_row.tenant_id AND merchant.enabled
JOIN plugin_idc_core_billing_invoice AS invoice
  ON invoice.tenant_id = order_row.tenant_id
 AND invoice.order_no = order_row.order_no
 AND invoice.source_code = 'order'
WHERE order_row.tenant_id = ? AND order_row.order_no = ?
LIMIT 1`, idccap.DefaultTenantID, orderNo); err != nil {
			return err
		}
		if order == nil || order.Version <= 0 || order.TotalAmountMinor < 0 ||
			!validOrderForPreparation(order, requiredForFulfillment) {
			return gerror.New("contract order snapshot is not accessible")
		}
		identity, err := loadVerifiedIdentity(ctx, tx, order, current)
		if err != nil {
			return err
		}
		var rows []itemRow
		if err = tx.GetScan(&rows, `
SELECT item.order_item_no, item.product_no, item.product_name_zh_cn, item.product_name_en_us,
	   quantity.quantity, item.purchase_cycles, item.period_unit_code, item.period_count,
	   item.requires_contract, item.total_amount_minor
FROM plugin_idc_core_commerce_order_item AS item
JOIN plugin_idc_core_commerce_order_item_quantity_snapshot AS quantity
  ON quantity.tenant_id = item.tenant_id
 AND quantity.order_no = item.order_no
 AND quantity.order_item_no = item.order_item_no
WHERE item.tenant_id = ? AND item.order_no = ?
ORDER BY item.line_number ASC, item.id ASC
LIMIT ?`, idccap.DefaultTenantID, orderNo, contractcap.MaximumOrderItems+1); err != nil {
			identity.Clear()
			return err
		}
		if len(rows) == 0 || len(rows) > contractcap.MaximumOrderItems {
			identity.Clear()
			return gerror.New("contract order item snapshot is unavailable")
		}
		items := make([]contractcap.OrderItem, 0, len(rows))
		var total int64
		for _, row := range rows {
			contractProductName := ""
			if requiredForFulfillment {
				contractProductName = row.ProductNameZhCN
				if order.ContractLanguageCode == "en-US" {
					contractProductName = row.ProductNameEnUS
				}
			}
			if row.OrderItemNo == "" || row.ProductNo == "" || row.Quantity < 1 ||
				row.PurchaseCycles < 1 || row.PeriodCount < 1 || row.TotalAmountMinor < 0 ||
				total > order.TotalAmountMinor-row.TotalAmountMinor ||
				(requiredForFulfillment && (contractProductName == "" ||
					contractProductName != strings.TrimSpace(contractProductName))) {
				identity.Clear()
				return gerror.New("contract order item snapshot is inconsistent")
			}
			total += row.TotalAmountMinor
			items = append(items, contractcap.OrderItem{
				OrderItemNo: row.OrderItemNo, ProductNo: row.ProductNo,
				ProductNameZhCN: row.ProductNameZhCN, ProductNameEnUS: row.ProductNameEnUS,
				ContractProductName: contractProductName,
				Quantity:            row.Quantity, PurchaseCycles: row.PurchaseCycles,
				PeriodUnitCode: row.PeriodUnitCode, PeriodCount: row.PeriodCount,
				RequiresContract: row.RequiresContract, AmountMinor: row.TotalAmountMinor,
			})
		}
		if total != order.TotalAmountMinor {
			identity.Clear()
			return gerror.New("contract order amount or requirement snapshot is inconsistent")
		}
		snapshot = &contractcap.OrderSnapshot{
			OrderNo: order.OrderNo, StatusCode: order.StatusCode,
			CurrencyCode: order.CurrencyCode, AmountMinor: order.TotalAmountMinor,
			RequiresContract: order.RequiresContract, OrderVersion: order.Version,
			Template: contractcap.TemplateSnapshot{
				TemplateNo: order.ContractTemplateNo, VersionNo: order.ContractTemplateVersionNo,
				Name: order.ContractTemplateName, LanguageCode: order.ContractLanguageCode,
				ScopeCode: order.ContractScopeCode, SigningOrderCode: order.ContractSigningOrderCode,
			},
			Merchant: contractcap.MerchantIdentity{
				MerchantNo: order.MerchantNo, LegalName: []byte(order.MerchantLegalName),
				CountryCode: order.MerchantCountryCode, Version: order.MerchantVersion,
			},
			Customer: *identity, Items: items, SnapshotObservedAt: current,
		}
		return nil
	})
	if err != nil {
		if snapshot != nil {
			snapshot.Clear()
		}
		return nil, gerror.Wrap(err, "prepare contract order snapshot")
	}
	return snapshot, nil
}

func validOrderForPreparation(order *orderRow, requiredForFulfillment bool) bool {
	if requiredForFulfillment {
		return activeOrderStatus(order.StatusCode) && validContractOrder(order)
	}
	return validOptionalContractOrder(order)
}

func validContractOrder(order *orderRow) bool {
	if order == nil || !order.RequiresContract || order.ContractTemplateNo == "" ||
		order.ContractTemplateVersionNo < 1 || strings.TrimSpace(order.ContractTemplateName) == "" ||
		(order.ContractLanguageCode != "zh-CN" && order.ContractLanguageCode != "en-US") ||
		(order.ContractSigningOrderCode != "customer_first" &&
			order.ContractSigningOrderCode != "merchant_first" &&
			order.ContractSigningOrderCode != "parallel") || order.MerchantNo == "" ||
		strings.TrimSpace(order.MerchantLegalName) == "" || order.MerchantCountryCode == "" ||
		order.MerchantVersion < 1 {
		return false
	}
	if order.ContractScopeCode == "all_customers" {
		return true
	}
	return (order.ContractScopeCode == "personal" && order.CustomerTypeCode == "individual") ||
		(order.ContractScopeCode == "enterprise" && order.CustomerTypeCode == "enterprise")
}

func validOptionalContractOrder(order *orderRow) bool {
	if order == nil || order.RequiresContract || !optionalOrderStatus(order.StatusCode) ||
		!optionalInvoiceStatus(order.InvoiceStatusCode) ||
		order.InvoicePaidAmountMinor < 0 || order.InvoiceRefundedAmountMinor < 0 ||
		(order.InvoicePaidAmountMinor > 0 &&
			order.InvoiceRefundedAmountMinor >= order.InvoicePaidAmountMinor) ||
		order.ContractTemplateNo != "" || order.ContractTemplateVersionNo != 0 ||
		order.ContractTemplateName != "" || order.ContractLanguageCode != "" ||
		order.ContractScopeCode != "" || order.ContractSigningOrderCode != "" ||
		order.MerchantNo == "" || strings.TrimSpace(order.MerchantLegalName) == "" ||
		order.MerchantCountryCode == "" || order.MerchantVersion < 1 {
		return false
	}
	return true
}

func loadVerifiedIdentity(
	ctx context.Context,
	tx gdb.TX,
	order *orderRow,
	current time.Time,
) (*contractcap.VerifiedIdentity, error) {
	switch contractcap.CustomerType(order.CustomerTypeCode) {
	case contractcap.CustomerTypeIndividual:
		var profile *personalRow
		if err := tx.GetScan(&profile, `
SELECT legal_name, identity_lookup_digest, identity_case_no, identity_status_code, identity_verified_at,
       identity_expires_at, version
FROM plugin_idc_core_customer_individual_profile
WHERE tenant_id = ? AND customer_no = ?
LIMIT 1`, idccap.DefaultTenantID, order.CustomerNo); err != nil {
			return nil, err
		}
		if profile == nil || profile.StatusCode != "verified" || profile.LegalName == "" ||
			!lowerHex64Pattern.MatchString(profile.LookupDigest) ||
			profile.CaseNo == "" || profile.VerifiedAt == nil || profile.VerificationVersion <= 0 ||
			expired(profile.ExpiresAt, current) {
			return nil, gerror.New("contract personal signer is not currently verified")
		}
		var phones []verifiedPhoneRow
		if err := tx.GetScan(&phones, `
SELECT contact.contact_no,contact.phone_country_code,contact.phone,contact.phone_verified_at,contact.version
FROM plugin_idc_core_customer_contact contact
JOIN plugin_idc_core_customer_contact_role role
  ON role.tenant_id=contact.tenant_id AND role.customer_no=contact.customer_no AND role.contact_no=contact.contact_no
WHERE contact.tenant_id=? AND contact.customer_no=? AND contact.status_code='active'
  AND contact.phone<>'' AND contact.phone_verified_at IS NOT NULL
  AND role.role_code='primary' AND role.status_code='active'
ORDER BY contact.id ASC LIMIT 2`, idccap.DefaultTenantID, order.CustomerNo); err != nil {
			return nil, err
		}
		if len(phones) != 1 || phones[0].ContactNo == "" || phones[0].CountryCode == "" || phones[0].Phone == "" ||
			phones[0].PhoneVerifiedAt == nil || phones[0].Version <= 0 {
			return nil, gerror.New("contract personal signer phone is not currently verified")
		}
		return &contractcap.VerifiedIdentity{
			CustomerType: contractcap.CustomerTypeIndividual, CustomerNo: order.CustomerNo,
			LegalName: []byte(profile.LegalName), SubjectLookupDigest: profile.LookupDigest,
			VerificationCaseNo:  profile.CaseNo,
			VerificationVersion: profile.VerificationVersion, VerifiedAt: profile.VerifiedAt.UTC(),
			ExpiresAt:     normalizedTime(profile.ExpiresAt),
			VerifiedPhone: []byte(phones[0].CountryCode + phones[0].Phone), PhoneContactNo: phones[0].ContactNo,
			PhoneContactVersion: phones[0].Version, PhoneVerifiedAt: phones[0].PhoneVerifiedAt.UTC(),
		}, nil
	case contractcap.CustomerTypeEnterprise:
		var profile *enterpriseRow
		if err := tx.GetScan(&profile, `
SELECT legal_name, registration_country_code, registration_identifier,
       registration_lookup_digest,
       registration_identifier_masked, verification_case_no,
       verification_status_code, verified_at, expires_at, version
FROM plugin_idc_core_customer_enterprise_profile
WHERE tenant_id = ? AND customer_no = ?
LIMIT 1`, idccap.DefaultTenantID, order.CustomerNo); err != nil {
			return nil, err
		}
		if profile == nil || profile.StatusCode != "verified" || profile.LegalName == "" ||
			profile.RegistrationCountryCode == "" || profile.RegistrationIdentifier == "" ||
			!lowerHex64Pattern.MatchString(profile.LookupDigest) ||
			profile.CaseNo == "" || profile.VerifiedAt == nil || profile.VerificationVersion <= 0 ||
			expired(profile.ExpiresAt, current) {
			return nil, gerror.New("contract enterprise signer is not currently verified")
		}
		return &contractcap.VerifiedIdentity{
			CustomerType: contractcap.CustomerTypeEnterprise, CustomerNo: order.CustomerNo,
			LegalName: []byte(profile.LegalName), SubjectLookupDigest: profile.LookupDigest,
			VerificationCaseNo:  profile.CaseNo,
			VerificationVersion: profile.VerificationVersion, VerifiedAt: profile.VerifiedAt.UTC(),
			ExpiresAt:                    normalizedTime(profile.ExpiresAt),
			RegistrationCountryCode:      profile.RegistrationCountryCode,
			RegistrationIdentifier:       []byte(profile.RegistrationIdentifier),
			RegistrationIdentifierMasked: profile.RegistrationIdentifierMasked,
		}, nil
	default:
		return nil, gerror.New("contract customer type is unsupported")
	}
}

func activeOrderStatus(value string) bool {
	switch value {
	case "pending_payment", "payment_processing", "paid", "fulfilling",
		"partially_fulfilled", "fulfilled", "manual_review":
		return true
	default:
		return false
	}
}

func optionalOrderStatus(value string) bool {
	switch value {
	case "pending_payment", "payment_processing", "paid", "fulfilling",
		"partially_fulfilled", "fulfilled", "partially_refunded", "manual_review":
		return true
	default:
		return false
	}
}

func optionalInvoiceStatus(value string) bool {
	switch value {
	case "pending_payment", "partially_paid", "paid", "overdue",
		"partially_refunded", "credit_settled":
		return true
	default:
		return false
	}
}

func expired(value *time.Time, now time.Time) bool {
	return value != nil && !value.After(now)
}

func normalizedTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
