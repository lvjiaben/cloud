// This file exposes only currently public product campaigns and records privacy-safe deduplicated exposures.

package marketing

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
)

const maximumPublicCampaignsPerProduct = 64

var publicCampaignSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// PublicCampaign is the bounded bilingual projection safe for anonymous product pages.
type PublicCampaign struct {
	CampaignNo      string
	CampaignType    string
	NameZhCN        string
	NameEnUS        string
	DescriptionZhCN string
	DescriptionEnUS string
	StartsAt        time.Time
	EndsAt          time.Time
}

// PublicCampaignListInput identifies one published product and the visitor's explicit currency.
type PublicCampaignListInput struct {
	PublicSlug   string
	CurrencyCode string
}

// PublicCampaignExposureInput carries the browser-local random identifier only for immediate hashing.
type PublicCampaignExposureInput struct {
	PublicSlug   string
	CampaignNo   string
	CurrencyCode string
	DeviceID     string
}

// PublicCampaignExposure reports whether this call created the current rolling-window fact.
type PublicCampaignExposure struct {
	Recorded bool
}

// PublicCampaignService is a public exception to staff/customer data permissions: it reads only active,
// untargeted campaigns for an active published product and never returns customer or internal catalog data.
type PublicCampaignService interface {
	// List returns at most 64 active, untargeted campaigns whose scopes and active benefits match the product and currency.
	List(ctx context.Context, input PublicCampaignListInput) ([]PublicCampaign, error)
	// RecordExposure hashes the raw device value in memory and records at most one fact per device, campaign, and currency in 30 minutes.
	RecordExposure(ctx context.Context, input PublicCampaignExposureInput) (*PublicCampaignExposure, error)
}

type publicCampaignServiceImpl struct {
	database  gdb.DB
	digestKey []byte
}

var _ PublicCampaignService = (*publicCampaignServiceImpl)(nil)

// NewPublicCampaignService constructs the anonymous campaign projection with a startup-owned digest key copy.
func NewPublicCampaignService(database gdb.DB, digestKey []byte) (PublicCampaignService, error) {
	if database == nil || len(digestKey) < 32 {
		return nil, gerror.New("public campaign service requires database and 256-bit digest key")
	}
	return &publicCampaignServiceImpl{database: database, digestKey: append([]byte(nil), digestKey...)}, nil
}

func (s *publicCampaignServiceImpl) List(
	ctx context.Context,
	input PublicCampaignListInput,
) ([]PublicCampaign, error) {
	normalized, err := normalizePublicCampaignListInput(input)
	if err != nil {
		return nil, err
	}
	rows := make([]PublicCampaign, 0)
	if err = s.database.GetScan(
		ctx, &rows, publicCampaignListSQL,
		normalized.PublicSlug, normalized.CurrencyCode, maximumPublicCampaignsPerProduct+1,
	); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicCampaignUnavailable)
	}
	if len(rows) > maximumPublicCampaignsPerProduct {
		return nil, bizerr.NewCode(CodePublicCampaignUnavailable)
	}
	return rows, nil
}

func (s *publicCampaignServiceImpl) RecordExposure(
	ctx context.Context,
	input PublicCampaignExposureInput,
) (*PublicCampaignExposure, error) {
	normalizedList, err := normalizePublicCampaignListInput(PublicCampaignListInput{
		PublicSlug: input.PublicSlug, CurrencyCode: input.CurrencyCode,
	})
	if err != nil {
		return nil, err
	}
	input.CampaignNo = strings.TrimSpace(input.CampaignNo)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	if !identifierPattern.MatchString(input.CampaignNo) || !validPublicDeviceID(input.DeviceID) {
		return nil, bizerr.NewCode(CodePublicCampaignInvalid)
	}
	deviceDigest := s.purposeDigest("public-campaign-device", input.DeviceID)
	result := &PublicCampaignExposure{}
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		eligible, readErr := s.database.GetValue(
			txCtx, publicCampaignEligibleSQL,
			normalizedList.PublicSlug, normalizedList.CurrencyCode, input.CampaignNo,
		)
		if readErr != nil {
			return readErr
		}
		if eligible.Int() != 1 {
			return bizerr.NewCode(CodePublicCampaignInvalid)
		}
		var window *struct {
			StartedAt time.Time `orm:"window_started_at"`
		}
		if writeErr := s.database.GetScan(txCtx, &window, publicCampaignExposureFenceSQL,
			deviceDigest, input.CampaignNo, normalizedList.CurrencyCode,
		); writeErr != nil {
			return writeErr
		}
		if window == nil {
			return nil
		}
		sourceReference := "public_exposure:" + s.purposeDigest(
			"public-campaign-exposure-source", deviceDigest, input.CampaignNo,
			normalizedList.CurrencyCode, window.StartedAt.UTC().Format(time.RFC3339Nano),
		)
		factNo, numberErr := newMarketingNumber(rand.Reader, "mfx")
		if numberErr != nil {
			return numberErr
		}
		contentDigest := s.purposeDigest(
			"public-campaign-exposure-content", sourceReference, normalizedList.PublicSlug,
		)
		if _, insertErr := s.database.Exec(txCtx, `
INSERT INTO plugin_idc_core_marketing_effect_fact
 (tenant_id, fact_no, campaign_no, customer_no, event_type_code, currency_code,
  amount_minor, source_type_code, source_reference, content_digest, occurred_at)
VALUES (0, ?, ?, NULL, 'exposure', ?, 0, 'public_website', ?, ?, ?)`,
			factNo, input.CampaignNo, normalizedList.CurrencyCode, sourceReference,
			contentDigest, window.StartedAt.UTC(),
		); insertErr != nil {
			return insertErr
		}
		result.Recorded = true
		return nil
	})
	if err != nil {
		if _, ok := bizerr.As(err); ok {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodePublicCampaignUnavailable)
	}
	return result, nil
}

func normalizePublicCampaignListInput(input PublicCampaignListInput) (PublicCampaignListInput, error) {
	input.PublicSlug = strings.ToLower(strings.TrimSpace(input.PublicSlug))
	input.CurrencyCode = strings.TrimSpace(input.CurrencyCode)
	if !publicCampaignSlugPattern.MatchString(input.PublicSlug) ||
		utf8.RuneCountInString(input.PublicSlug) > 128 ||
		!currencyPattern.MatchString(input.CurrencyCode) {
		return PublicCampaignListInput{}, bizerr.NewCode(CodePublicCampaignInvalid)
	}
	return input, nil
}

func validPublicDeviceID(value string) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) >= 16 &&
		utf8.RuneCountInString(value) <= 128 && !strings.ContainsRune(value, '\x00')
}

func (s *publicCampaignServiceImpl) purposeDigest(purpose string, parts ...string) string {
	mac := hmac.New(sha256.New, s.digestKey)
	if _, err := mac.Write([]byte(purpose)); err != nil {
		return ""
	}
	for _, part := range parts {
		if _, err := mac.Write([]byte{0}); err != nil {
			return ""
		}
		if _, err := mac.Write([]byte(part)); err != nil {
			return ""
		}
	}
	return hex.EncodeToString(mac.Sum(nil))
}

const publicCampaignBaseSQL = `
SELECT campaign.campaign_no, campaign.campaign_type_code AS campaign_type,
       campaign.name_zh_cn, campaign.name_en_us,
       campaign.description_zh_cn, campaign.description_en_us,
       campaign.starts_at, campaign.ends_at
FROM plugin_idc_core_catalog_product product
JOIN plugin_idc_core_catalog_category category
  ON category.tenant_id = product.tenant_id AND category.category_no = product.category_no
 AND category.deleted_at IS NULL AND category.status_code = 'active'
JOIN plugin_idc_core_catalog_product_version version
  ON version.tenant_id = product.tenant_id AND version.product_no = product.product_no
 AND version.version_no = product.current_version_no AND version.status_code = 'published'
JOIN plugin_idc_core_marketing_campaign campaign
  ON campaign.tenant_id = product.tenant_id AND campaign.status_code = 'active'
 AND campaign.starts_at <= CURRENT_TIMESTAMP AND campaign.ends_at > CURRENT_TIMESTAMP
 AND campaign.target_segment_no IS NULL AND campaign.target_snapshot_no IS NULL
WHERE product.tenant_id = 0 AND product.deleted_at IS NULL AND product.status_code = 'active'
  AND product.public_slug = ?
  AND EXISTS (
    SELECT 1 FROM plugin_idc_core_marketing_benefit_rule benefit
    WHERE benefit.tenant_id = campaign.tenant_id AND benefit.campaign_no = campaign.campaign_no
      AND benefit.status_code = 'active' AND benefit.currency_code = ?
  )
  AND EXISTS (
    SELECT 1 FROM plugin_idc_core_marketing_campaign_scope scope
    WHERE scope.tenant_id = campaign.tenant_id AND scope.campaign_no = campaign.campaign_no
      AND scope.match_mode_code = 'include'
      AND (scope.scope_kind_code = 'all_products'
        OR (scope.scope_kind_code = 'product' AND scope.reference_code = product.product_no)
        OR (scope.scope_kind_code = 'category' AND scope.reference_code = product.category_no))
  )
  AND NOT EXISTS (
    SELECT 1 FROM plugin_idc_core_marketing_campaign_scope scope
    WHERE scope.tenant_id = campaign.tenant_id AND scope.campaign_no = campaign.campaign_no
      AND scope.match_mode_code = 'exclude'
      AND ((scope.scope_kind_code = 'product' AND scope.reference_code = product.product_no)
        OR (scope.scope_kind_code = 'category' AND scope.reference_code = product.category_no))
  )`

const publicCampaignListSQL = publicCampaignBaseSQL + `
ORDER BY campaign.priority ASC, campaign.campaign_no ASC
LIMIT ?`

const publicCampaignEligibleSQL = `
SELECT count(*) FROM (` + publicCampaignBaseSQL + `) eligible
WHERE eligible.campaign_no = ?`

const publicCampaignExposureFenceSQL = `
INSERT INTO plugin_idc_core_marketing_public_exposure_state
 (tenant_id, device_digest, campaign_no, currency_code, window_started_at)
VALUES (0, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (device_digest, campaign_no, currency_code) DO UPDATE
SET window_started_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
WHERE plugin_idc_core_marketing_public_exposure_state.window_started_at
      <= CURRENT_TIMESTAMP - INTERVAL '30 minutes'
RETURNING window_started_at`
