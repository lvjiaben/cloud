// Package channelconfig owns merchant payment-channel configuration and secret envelopes.
package channelconfig

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
)

type Verify func(context.Context, paymentcap.ProviderCode, paymentcap.Credentials) error

// ProviderDirectory keeps payment-channel discovery on the narrow read-only
// extension seam it actually needs instead of depending on the whole host
// registrar surface.
type ProviderDirectory interface {
	ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error)
}

type Service struct {
	database   gdb.DB
	accessor   secretcap.Accessor
	manager    secretcap.Manager
	adapters   map[paymentcap.ProviderCode]paymentcap.Adapter
	verify     Verify
	extensions ProviderDirectory
}

type CredentialEnvelope struct {
	MerchantID, AppID, Secret, PrivateKey, PublicKey string
	Extra                                            map[string]string
}
type SaveInput struct {
	ChannelNo                                      string
	Provider                                       paymentcap.ProviderCode
	DisplayName, EndpointOrigin, MerchantReference string
	Credential                                     CredentialEnvelope
	EnabledModes                                   []paymentcap.Mode
	IdempotencyKey                                 string
	ActorID                                        int
}
type Channel struct {
	ChannelNo                                                  string
	Provider                                                   paymentcap.ProviderCode
	DisplayName, EndpointOrigin, MerchantReference, StatusCode string
	Modes                                                      []paymentcap.Mode
	Currencies                                                 []string
	Version                                                    int64
	VerifiedAt                                                 time.Time
}

// ChannelPage contains one numeric page and its exact filtered total.
type ChannelPage struct {
	Items []Channel
	Total int
}
type AvailableChannel struct {
	ChannelNo, DisplayName string
	Provider               paymentcap.ProviderCode
	Modes                  []paymentcap.Mode
	Currencies             []string
}

type ProviderCandidate struct {
	PluginID, DisplayName, ReleaseVersion string
	Descriptor                            paymentcap.DynamicProviderDescriptor
	InterfaceConfiguration                *capmodel.ConfigurationSchemaDeclaration
}

func (s *Service) ListProviderCandidates(ctx context.Context) ([]ProviderCandidate, error) {
	if s.extensions == nil {
		return nil, nil
	}
	page, err := s.extensions.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{
		Point: paymentcap.DynamicProviderPoint, ContractVersion: paymentcap.DynamicProviderContractVersion,
		Page: capmodel.PageRequest{PageNum: 1, PageSize: 100, Limit: 100},
	})
	if err != nil || page == nil || len(page.Items) > 100 {
		return nil, errors.New("payment method discovery is unavailable")
	}
	result := make([]ProviderCandidate, 0, len(page.Items))
	seen := map[paymentcap.ProviderCode]bool{}
	for _, item := range page.Items {
		if item == nil || item.InterfaceConfiguration == nil || item.PluginID == "" || item.ReleaseVersion == "" {
			continue
		}
		descriptor, decodeErr := paymentcap.DecodeDynamicProviderMetadata(item.ProviderMetadata)
		if decodeErr != nil || seen[descriptor.ProviderCode] || s.adapters[descriptor.ProviderCode] == nil {
			continue
		}
		seen[descriptor.ProviderCode] = true
		result = append(result, ProviderCandidate{
			PluginID: item.PluginID, DisplayName: strings.TrimSpace(item.DisplayName),
			ReleaseVersion: item.ReleaseVersion, Descriptor: descriptor,
			InterfaceConfiguration: item.InterfaceConfiguration,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].DisplayName < result[j].DisplayName })
	return result, nil
}

type RefundCapabilityEvidence struct {
	Supported      bool
	EvidenceDigest string
}

func (s *Service) RefundCapability(
	ctx context.Context,
	channelNo string,
	mode paymentcap.Mode,
	currencyCode string,
) (*RefundCapabilityEvidence, error) {
	modeJSON, err := json.Marshal([]string{string(mode)})
	if err != nil {
		return nil, err
	}
	currencyJSON, err := json.Marshal([]string{currencyCode})
	if err != nil {
		return nil, err
	}
	row, err := s.database.GetOne(ctx, `
SELECT channel_no,provider_code,status_code,version,verified_at,
       supported_modes @> CAST(? AS jsonb) AS mode_supported,
       supported_currencies @> CAST(? AS jsonb) AS currency_supported,
       COALESCE((capability_snapshot->>'refund')::boolean,FALSE) AS refund_supported
FROM plugin_idc_payment_channel
WHERE tenant_id=0 AND channel_no=? AND deleted_at IS NULL`,
		string(modeJSON), string(currencyJSON), channelNo)
	if err != nil {
		return nil, err
	}
	if row.IsEmpty() {
		return nil, errors.New("payment channel is not visible")
	}
	supported := row["status_code"].String() == "enabled" &&
		row["mode_supported"].Bool() && row["currency_supported"].Bool() &&
		row["refund_supported"].Bool()
	evidence := struct {
		ChannelNo, Provider, Status, Mode, Currency string
		Version                                     int64
		VerifiedAt                                  int64
		Supported                                   bool
	}{
		row["channel_no"].String(), row["provider_code"].String(),
		row["status_code"].String(), string(mode), currencyCode,
		row["version"].Int64(), row["verified_at"].Time().UTC().UnixMilli(), supported,
	}
	raw, err := json.Marshal(evidence)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(raw)
	return &RefundCapabilityEvidence{Supported: supported, EvidenceDigest: hex.EncodeToString(digest[:])}, nil
}

func (s *Service) ListAvailable(ctx context.Context, currencyCode string) ([]AvailableChannel, error) {
	availableProviders, err := s.availableProviderCodes(ctx)
	if err != nil {
		return nil, err
	}
	currencyJSON, err := json.Marshal([]string{currencyCode})
	if err != nil {
		return nil, err
	}
	rows, err := s.database.GetAll(ctx, `SELECT channel_no,provider_code,display_name,supported_modes,supported_currencies FROM plugin_idc_payment_channel WHERE tenant_id=0 AND status_code='enabled' AND deleted_at IS NULL AND (?='' OR supported_currencies @> CAST(? AS jsonb)) ORDER BY display_name,channel_no LIMIT 100`, currencyCode, string(currencyJSON))
	if err != nil {
		return nil, err
	}
	result := make([]AvailableChannel, 0, len(rows))
	for _, row := range rows {
		provider := paymentcap.ProviderCode(row["provider_code"].String())
		if availableProviders != nil && !availableProviders[provider] {
			continue
		}
		var modes []paymentcap.Mode
		var currencies []string
		if json.Unmarshal(row["supported_modes"].Bytes(), &modes) != nil || json.Unmarshal(row["supported_currencies"].Bytes(), &currencies) != nil {
			return nil, errors.New("payment channel capability is invalid")
		}
		result = append(result, AvailableChannel{ChannelNo: row["channel_no"].String(), Provider: provider, DisplayName: row["display_name"].String(), Modes: modes, Currencies: currencies})
	}
	return result, nil
}

func New(database gdb.DB, accessor secretcap.Accessor, manager secretcap.Manager, adapters []paymentcap.Adapter, verify Verify, extensions ...ProviderDirectory) (*Service, error) {
	if database == nil || accessor == nil || manager == nil || verify == nil {
		return nil, errors.New("payment channel dependencies are required")
	}
	registry := make(map[paymentcap.ProviderCode]paymentcap.Adapter, len(adapters))
	for _, adapter := range adapters {
		if adapter == nil {
			return nil, errors.New("payment adapter is required")
		}
		provider := adapter.Provider()
		if _, exists := registry[provider]; exists {
			return nil, errors.New("duplicate payment adapter")
		}
		registry[provider] = adapter
	}
	if len(registry) == 0 {
		return nil, errors.New("payment adapters are required")
	}
	var extensionRegistrar ProviderDirectory
	if len(extensions) > 0 {
		extensionRegistrar = extensions[0]
	}
	return &Service{database: database, accessor: accessor, manager: manager, adapters: registry, verify: verify, extensions: extensionRegistrar}, nil
}

func (s *Service) Save(ctx context.Context, input SaveInput) (*Channel, error) {
	adapter := s.adapters[input.Provider]
	if adapter == nil {
		return nil, errors.New("payment provider is unavailable")
	}
	if s.extensions != nil {
		candidates, err := s.ListProviderCandidates(ctx)
		if err != nil {
			return nil, err
		}
		available := false
		for _, candidate := range candidates {
			if candidate.Descriptor.ProviderCode == input.Provider {
				available = true
				break
			}
		}
		if !available {
			return nil, errors.New("installed payment method is unavailable")
		}
	}
	if err := validateSave(input); err != nil {
		return nil, err
	}
	credentials, plaintext, err := envelope(input.Credential)
	if err != nil {
		return nil, err
	}
	defer clear(plaintext)
	if err = s.verify(ctx, input.Provider, credentials); err != nil {
		return nil, errors.New("payment channel credential format is invalid")
	}
	created, err := s.manager.Create(ctx, "payment-channel."+input.ChannelNo+"."+input.IdempotencyKey, func(consume secretcap.Consumer) error { return consume(plaintext) })
	if err != nil {
		return nil, err
	}
	capabilities, err := verifiedCapabilities(adapter.Capabilities(), input.Provider, input.EnabledModes, input.Credential)
	if err != nil {
		return nil, err
	}
	modes, currencies, capJSON, err := capabilityProjection(capabilities)
	if err != nil {
		return nil, err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_payment_channel
 (channel_no,provider_code,display_name,endpoint_origin,merchant_reference,secret_ref,supported_modes,supported_currencies,capability_snapshot,status_code,verified_at,created_by,updated_by)
VALUES(?,?,?,?,?,?,CAST(? AS jsonb),CAST(? AS jsonb),CAST(? AS jsonb),'disabled',NULL,?,?)
ON CONFLICT(channel_no) DO NOTHING`, input.ChannelNo, string(input.Provider), input.DisplayName, input.EndpointOrigin, input.MerchantReference, created.Handle.OpaqueRef(), modes, currencies, capJSON, input.ActorID, input.ActorID)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, input.ChannelNo)
}

func (s *Service) availableProviderCodes(ctx context.Context) (map[paymentcap.ProviderCode]bool, error) {
	if s.extensions == nil {
		return nil, nil
	}
	candidates, err := s.ListProviderCandidates(ctx)
	if err != nil {
		return nil, err
	}
	available := make(map[paymentcap.ProviderCode]bool, len(candidates))
	for _, candidate := range candidates {
		available[candidate.Descriptor.ProviderCode] = true
	}
	return available, nil
}

func (s *Service) requireAvailableProvider(ctx context.Context, provider paymentcap.ProviderCode) error {
	available, err := s.availableProviderCodes(ctx)
	if err != nil {
		return err
	}
	if available != nil && !available[provider] {
		return errors.New("installed payment method is unavailable")
	}
	return nil
}

// VerifyConnection executes one adapter-owned, read-only upstream probe using
// the stored opaque secret. Saving configuration never calls this method and
// can therefore never make a channel available by itself.
func (s *Service) VerifyConnection(ctx context.Context, channelNo string, expectedVersion int64, actorID int) (*Channel, error) {
	if strings.TrimSpace(channelNo) == "" || expectedVersion <= 0 || actorID <= 0 {
		return nil, errors.New("invalid payment channel connection check")
	}
	row, err := s.database.GetOne(ctx, `SELECT provider_code,secret_ref,endpoint_origin,status_code,version
FROM plugin_idc_payment_channel
WHERE tenant_id=0 AND channel_no=? AND version=? AND deleted_at IS NULL`, channelNo, expectedVersion)
	if err != nil {
		return nil, err
	}
	if row.IsEmpty() {
		return nil, errors.New("payment channel changed; refresh and try again")
	}
	provider := paymentcap.ProviderCode(row["provider_code"].String())
	if err = s.requireAvailableProvider(ctx, provider); err != nil {
		return nil, err
	}
	adapter := s.adapters[provider]
	if adapter == nil {
		return nil, errors.New("payment provider is unavailable")
	}
	if binder, ok := adapter.(paymentcap.EndpointBoundAdapter); ok {
		adapter, err = binder.WithEndpoint(row["endpoint_origin"].String())
		if err != nil {
			return nil, errors.New("payment channel endpoint is invalid")
		}
	}
	verifier, ok := adapter.(paymentcap.ConnectionVerifier)
	if !ok {
		return nil, errors.New("payment provider connection check is unavailable")
	}
	handle, err := secretcap.NewHandle(row["secret_ref"].String())
	if err != nil {
		return nil, err
	}
	purpose, err := secretcap.NewPurpose("payment.channel-verify")
	if err != nil {
		return nil, err
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	verificationErr := s.accessor.Access(probeCtx, request, func(plaintext []byte) error {
		var raw CredentialEnvelope
		if decodeErr := json.Unmarshal(plaintext, &raw); decodeErr != nil {
			return errors.New("payment credential envelope is invalid")
		}
		credentials, temporary, decodeErr := envelope(raw)
		if decodeErr != nil {
			return decodeErr
		}
		defer clear(temporary)
		return verifier.VerifyConnection(probeCtx, credentials)
	})
	if verificationErr != nil {
		_, _ = s.database.Exec(ctx, `UPDATE plugin_idc_payment_channel
SET status_code='disabled',verified_at=NULL,version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND channel_no=? AND version=? AND deleted_at IS NULL`, actorID, channelNo, expectedVersion)
		return nil, errors.New("payment channel connection verification failed")
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_payment_channel
SET status_code=CASE WHEN status_code='enabled' THEN 'enabled' ELSE 'disabled' END,
    verified_at=CURRENT_TIMESTAMP,version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND channel_no=? AND version=? AND deleted_at IS NULL`, actorID, channelNo, expectedVersion)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, errors.New("payment channel changed; refresh and try again")
	}
	return s.Get(ctx, channelNo)
}

func (s *Service) Get(ctx context.Context, channelNo string) (*Channel, error) {
	row, err := s.database.GetOne(ctx, `SELECT channel_no,provider_code,display_name,endpoint_origin,merchant_reference,status_code,supported_modes,supported_currencies,version,verified_at FROM plugin_idc_payment_channel WHERE tenant_id=0 AND channel_no=? AND deleted_at IS NULL`, channelNo)
	if err != nil {
		return nil, err
	}
	if row.IsEmpty() {
		return nil, errors.New("payment channel is not visible")
	}
	return decodeChannel(row)
}

// SetStatus enables or disables a verified channel behind an optimistic fence.
func (s *Service) SetStatus(ctx context.Context, channelNo, status string, expectedVersion int64, actorID int) (*Channel, error) {
	if strings.TrimSpace(channelNo) == "" || (status != "enabled" && status != "disabled") || expectedVersion <= 0 || actorID <= 0 {
		return nil, errors.New("invalid payment channel status change")
	}
	if status == "enabled" {
		row, err := s.database.GetOne(ctx, `SELECT provider_code FROM plugin_idc_payment_channel
WHERE tenant_id=0 AND channel_no=? AND version=? AND deleted_at IS NULL`, channelNo, expectedVersion)
		if err != nil {
			return nil, err
		}
		if row.IsEmpty() {
			return nil, errors.New("payment channel changed; refresh and try again")
		}
		if err = s.requireAvailableProvider(ctx, paymentcap.ProviderCode(row["provider_code"].String())); err != nil {
			return nil, err
		}
	}
	result, err := s.database.Exec(ctx, `UPDATE plugin_idc_payment_channel
SET status_code=?,version=version+1,updated_by=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND channel_no=? AND version=? AND deleted_at IS NULL
  AND (? <> 'enabled' OR verified_at IS NOT NULL)`, status, actorID, channelNo, expectedVersion, status)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, errors.New("payment channel changed; refresh and try again")
	}
	return s.Get(ctx, channelNo)
}

func decodeChannel(row gdb.Record) (*Channel, error) {
	var modes []paymentcap.Mode
	var currencies []string
	if err := json.Unmarshal(row["supported_modes"].Bytes(), &modes); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(row["supported_currencies"].Bytes(), &currencies); err != nil {
		return nil, err
	}
	return &Channel{ChannelNo: row["channel_no"].String(), Provider: paymentcap.ProviderCode(row["provider_code"].String()), DisplayName: row["display_name"].String(), EndpointOrigin: row["endpoint_origin"].String(), MerchantReference: row["merchant_reference"].String(), StatusCode: row["status_code"].String(), Modes: modes, Currencies: currencies, Version: row["version"].Int64(), VerifiedAt: row["verified_at"].Time().UTC()}, nil
}

// WithCreateCredentials requires an enabled channel before initiating a new payment.
func (s *Service) WithCreateCredentials(ctx context.Context, channelNo string, consume func(paymentcap.Adapter, paymentcap.Credentials) error) error {
	return s.withCredentials(ctx, channelNo, false, consume)
}

// WithCredentials permits existing transaction settlement on disabled channels.
// Callers must validate the original transaction or verified callback; deleted
// channels and revoked secret handles remain unavailable.
func (s *Service) WithCredentials(ctx context.Context, channelNo string, consume func(paymentcap.Adapter, paymentcap.Credentials) error) error {
	return s.withCredentials(ctx, channelNo, true, consume)
}

func (s *Service) withCredentials(ctx context.Context, channelNo string, allowDisabled bool, consume func(paymentcap.Adapter, paymentcap.Credentials) error) error {
	if consume == nil {
		return errors.New("payment credential consumer is required")
	}
	row, err := s.database.GetOne(ctx, `SELECT provider_code,secret_ref,endpoint_origin FROM plugin_idc_payment_channel WHERE channel_no=? AND status_code IN ('enabled','disabled') AND (? OR status_code='enabled') AND deleted_at IS NULL`, channelNo, allowDisabled)
	if err != nil {
		return err
	}
	if row.IsEmpty() {
		return errors.New("payment channel is unavailable")
	}
	handle, err := secretcap.NewHandle(row["secret_ref"].String())
	if err != nil {
		return err
	}
	purpose, err := secretcap.NewPurpose("payment.channel-use")
	if err != nil {
		return err
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return err
	}
	provider := paymentcap.ProviderCode(row["provider_code"].String())
	adapter := s.adapters[provider]
	if adapter == nil {
		return errors.New("payment adapter is unavailable")
	}
	if binder, ok := adapter.(paymentcap.EndpointBoundAdapter); ok {
		adapter, err = binder.WithEndpoint(row["endpoint_origin"].String())
		if err != nil {
			return errors.New("payment channel endpoint is invalid")
		}
	}
	return s.accessor.Access(ctx, request, func(plaintext []byte) error {
		var raw CredentialEnvelope
		if err := json.Unmarshal(plaintext, &raw); err != nil {
			return errors.New("payment credential envelope is invalid")
		}
		credentials, temporary, err := envelope(raw)
		if err != nil {
			return err
		}
		defer clear(temporary)
		return consume(adapter, credentials)
	})
}

func validateSave(input SaveInput) error {
	if strings.TrimSpace(input.ChannelNo) == "" || len(input.ChannelNo) > 64 || strings.TrimSpace(input.DisplayName) == "" || len(input.DisplayName) > 128 || strings.TrimSpace(input.MerchantReference) == "" || strings.TrimSpace(input.IdempotencyKey) == "" || input.ActorID <= 0 {
		return errors.New("invalid payment channel configuration")
	}
	parsed, err := url.Parse(input.EndpointOrigin)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("payment channel endpoint must be an HTTPS origin")
	}
	switch input.Provider {
	case paymentcap.ProviderAlipay:
		if parsed.String() != "https://openapi.alipay.com/gateway.do" {
			return errors.New("alipay channel endpoint is not official")
		}
	case paymentcap.ProviderWechatPay:
		if parsed.String() != "https://api.mch.weixin.qq.com" {
			return errors.New("wechat pay channel endpoint is not official")
		}
	case paymentcap.ProviderEPay:
		if !strings.HasSuffix(parsed.Path, "/submit.php") {
			return errors.New("epay channel endpoint must end with submit.php")
		}
	case paymentcap.ProviderUnionPay:
		if (parsed.Host != "gateway.95516.com" && parsed.Host != "gateway.test.95516.com") ||
			parsed.Path != "/gateway/api/frontTransReq.do" {
			return errors.New("unionpay channel endpoint is not official")
		}
	default:
		return errors.New("payment provider is unavailable")
	}
	return nil
}

func verifiedCapabilities(
	declared paymentcap.Capabilities,
	provider paymentcap.ProviderCode,
	requested []paymentcap.Mode,
	credential CredentialEnvelope,
) (paymentcap.Capabilities, error) {
	capabilities := paymentcap.CloneCapabilities(declared)
	selected := make(map[paymentcap.Mode]bool, len(requested))
	if len(requested) == 0 {
		for mode, enabled := range capabilities.Modes {
			if enabled && mode != paymentcap.ModeWechatJSAPI {
				selected[mode] = true
			}
		}
	} else {
		for _, mode := range requested {
			if !capabilities.Modes[mode] || selected[mode] {
				return paymentcap.Capabilities{}, errors.New("payment mode is not verified")
			}
			selected[mode] = true
		}
	}
	if selected[paymentcap.ModeWechatJSAPI] &&
		(provider != paymentcap.ProviderWechatPay || credential.Extra["jsapi_authorized"] != "true") {
		return paymentcap.Capabilities{}, errors.New("wechat JSAPI authorization is not verified")
	}
	if len(selected) == 0 {
		return paymentcap.Capabilities{}, errors.New("payment channel has no verified mode")
	}
	for mode := range capabilities.Modes {
		capabilities.Modes[mode] = selected[mode]
	}
	return capabilities, nil
}
func envelope(raw CredentialEnvelope) (paymentcap.Credentials, []byte, error) {
	encoded, err := json.Marshal(raw)
	if err != nil || len(encoded) > 1<<20 {
		return paymentcap.Credentials{}, nil, errors.New("payment credential envelope is invalid")
	}
	extra := make(map[string][]byte, len(raw.Extra))
	for key, value := range raw.Extra {
		extra[key] = []byte(value)
	}
	credentials := paymentcap.Credentials{MerchantID: raw.MerchantID, AppID: raw.AppID, Secret: []byte(raw.Secret), PrivateKey: []byte(raw.PrivateKey), PublicKey: []byte(raw.PublicKey), Extra: extra}
	if credentials.MerchantID == "" {
		clear(encoded)
		return paymentcap.Credentials{}, nil, errors.New("payment merchant identifier is required")
	}
	return credentials, encoded, nil
}
func capabilityProjection(capabilities paymentcap.Capabilities) (string, string, string, error) {
	modes := make([]string, 0)
	for value, enabled := range capabilities.Modes {
		if enabled {
			modes = append(modes, string(value))
		}
	}
	sort.Strings(modes)
	currencies := make([]string, 0)
	for value, enabled := range capabilities.Currencies {
		if enabled {
			currencies = append(currencies, value)
		}
	}
	sort.Strings(currencies)
	operations := make(map[string]bool, len(capabilities.Operations))
	for value, enabled := range capabilities.Operations {
		operations[string(value)] = enabled
	}
	modeJSON, err := json.Marshal(modes)
	if err != nil {
		return "", "", "", err
	}
	currencyJSON, err := json.Marshal(currencies)
	if err != nil {
		return "", "", "", err
	}
	operationJSON, err := json.Marshal(operations)
	if err != nil {
		return "", "", "", err
	}
	return string(modeJSON), string(currencyJSON), string(operationJSON), nil
}
