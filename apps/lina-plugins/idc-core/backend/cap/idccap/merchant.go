// This file defines the single-merchant context contract shared by IDC plugins.

package idccap

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

// Service defines the broad IDC base capability supplied through explicit
// constructor injection. Implementations must enforce the caller, tenant, and
// dependency boundary before returning data.
type Service interface {
	// CurrentMerchant returns the single merchant context for this installation.
	CurrentMerchant(ctx context.Context) (MerchantContext, error)
}

// MerchantContext is the non-secret installation context that dependent
// plugins need for locale, currency, time, and business ownership decisions.
type MerchantContext struct {
	// TenantID is the host tenant discriminator and must be DefaultTenantID.
	TenantID int `json:"tenantId"`
	// MerchantNo is the opaque public merchant business number.
	MerchantNo string `json:"merchantNo"`
	// DisplayName is the merchant-facing display name.
	DisplayName string `json:"displayName"`
	// LegalName is the registered enterprise name already held by this installation.
	LegalName string `json:"legalName"`
	// CountryCode is the merchant's ISO 3166-1 alpha-2 registration region.
	CountryCode string `json:"countryCode"`
	// DefaultLocale is the merchant's default IETF locale tag.
	DefaultLocale string `json:"defaultLocale"`
	// DefaultCurrency is the merchant's default ISO 4217 currency code.
	DefaultCurrency string `json:"defaultCurrency"`
	// Timezone is the merchant's default IANA time zone.
	Timezone string `json:"timezone"`
	// Enabled reports whether normal merchant business traffic is accepted.
	Enabled bool `json:"enabled"`
	// Version is the merchant profile optimistic-concurrency version.
	Version int64 `json:"version"`
}

// RequireDefaultTenant rejects any context outside the single-merchant tenant.
func RequireDefaultTenant(tenantID int) error {
	if tenantID != DefaultTenantID {
		return gerror.Newf(
			"idccap: tenant is outside the single-merchant boundary: required=%d actual=%d",
			DefaultTenantID,
			tenantID,
		)
	}
	return nil
}

// ValidateMerchantContext validates the minimum stable projection before it is
// returned to a source consumer or encoded by an authorized bridge.
func ValidateMerchantContext(current MerchantContext) error {
	if err := RequireDefaultTenant(current.TenantID); err != nil {
		return err
	}
	if current.MerchantNo == "" || current.MerchantNo != strings.TrimSpace(current.MerchantNo) {
		return gerror.New("idccap: merchant number is required and must be canonical")
	}
	if current.DefaultLocale == "" || current.DefaultLocale != strings.TrimSpace(current.DefaultLocale) {
		return gerror.New("idccap: default locale is required and must be canonical")
	}
	if current.DefaultCurrency == "" || current.DefaultCurrency != strings.TrimSpace(current.DefaultCurrency) {
		return gerror.New("idccap: default currency is required and must be canonical")
	}
	if current.Timezone == "" || current.Timezone != strings.TrimSpace(current.Timezone) {
		return gerror.New("idccap: timezone is required and must be canonical")
	}
	if current.Version <= 0 {
		return gerror.New("idccap: merchant context version must be positive")
	}
	return nil
}
