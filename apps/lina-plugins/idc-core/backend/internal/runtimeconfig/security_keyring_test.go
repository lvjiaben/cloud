package runtimeconfig

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"testing"
)

func TestParseAuditConfigDerivesDistinctVersionedPurposes(t *testing.T) {
	reservation := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, securityRootKeyBytes))
	rootV1 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x22}, securityRootKeyBytes))
	rootV2 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x33}, securityRootKeyBytes))
	raw := fmt.Sprintf(
		`{"active_key_id":"v2","reservation_key":"%s","keys":{"v1":"%s","v2":"%s"}}`,
		reservation,
		rootV1,
		rootV2,
	)
	config, err := ParseAuditConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if config.ActiveReplayDigestKey.ID != "replay-v2" ||
		config.ActiveIntegrityKey.ID != "integrity-v2" ||
		len(config.RetainedReplayDigestKeys) != 1 ||
		config.RetainedReplayDigestKeys[0].ID != "replay-v1" ||
		len(config.RetainedIntegrityKeys) != 1 ||
		config.RetainedIntegrityKeys[0].ID != "integrity-v1" ||
		bytes.Equal(config.ActiveReplayDigestKey.Secret, config.ActiveIntegrityKey.Secret) ||
		bytes.Equal(config.ReservationLookupKey.Secret, config.ActiveReplayDigestKey.Secret) {
		t.Fatalf("unexpected purpose-separated keyring: %+v", config)
	}
}

func TestParseAuditConfigRejectsUnsafeOrAmbiguousInput(t *testing.T) {
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, securityRootKeyBytes))
	tests := []string{
		"",
		`{"active_key_id":"v1","reservation_key":"short","keys":{"v1":"short"}}`,
		fmt.Sprintf(`{"active_key_id":"v2","reservation_key":"%s","keys":{"v1":"%s"}}`, key, key),
		fmt.Sprintf(`{"active_key_id":"v1","reservation_key":"%s","keys":{"v1":"%s"}}`, key, key),
		fmt.Sprintf(`{"active_key_id":"v1","reservation_key":"%s","keys":{"V1":"%s"}}`, key, key),
		fmt.Sprintf(`{"active_key_id":"v1","reservation_key":"%s","keys":{"v1":"%s"},"unknown":1}`, key, key),
	}
	for _, raw := range tests {
		if _, err := ParseAuditConfig(raw); err == nil {
			t.Errorf("unsafe keyring was accepted: %s", raw)
		}
	}
}

func TestLoadAuditConfigRequiresExplicitDeploymentInput(t *testing.T) {
	t.Setenv(SecurityKeyringEnvironment, "")
	if _, err := LoadAuditConfig(); err == nil {
		t.Fatal("empty deployment keyring was accepted")
	}
	if _, err := LoadSecretKeyProvider(); err == nil {
		t.Fatal("empty deployment keyring was accepted for secret envelopes")
	}
	if _, err := LoadCatalogCursorKey(); err == nil {
		t.Fatal("empty deployment keyring was accepted for catalog cursors")
	}
	if _, err := LoadRecentAuthSigningKeyProvider(); err == nil {
		t.Fatal("empty deployment keyring was accepted for recent authentication")
	}
	if _, err := LoadIdentityLookupKey(); err == nil {
		t.Fatal("empty deployment keyring was accepted for identity lookup")
	}
	if _, err := LoadProductAutomationSelectionKey(); err == nil {
		t.Fatal("empty deployment keyring was accepted for product automation selection")
	}
	if _, err := LoadProductAutomationSelectionAudienceKey(); err == nil {
		t.Fatal("empty deployment keyring was accepted for product automation selection audience")
	}
}

func TestLoadProductAutomationSelectionKeyIsStableAndPurposeIsolated(t *testing.T) {
	reservation := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, securityRootKeyBytes))
	root := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x22}, securityRootKeyBytes))
	t.Setenv(SecurityKeyringEnvironment, fmt.Sprintf(
		`{"active_key_id":"v1","reservation_key":"%s","keys":{"v1":"%s"}}`,
		reservation,
		root,
	))

	first, err := LoadProductAutomationSelectionKey()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(first)
	second, err := LoadProductAutomationSelectionKey()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(second)
	audience, err := LoadProductAutomationSelectionAudienceKey()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(audience)
	identity, err := LoadIdentityLookupKey()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(identity)
	onboarding, err := LoadCustomerOnboardingTokenKey()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(onboarding)
	assistance, err := LoadCustomerAssistanceTokenKey()
	if err != nil {
		t.Fatal(err)
	}
	defer clear(assistance)

	if len(first) != derivedSecurityKeyBytes || !bytes.Equal(first, second) {
		t.Fatal("product automation selection key is not stable")
	}
	if len(audience) != derivedSecurityKeyBytes || bytes.Equal(first, audience) {
		t.Fatal("product automation token encryption and audience binding keys are not isolated")
	}
	for name, other := range map[string][]byte{
		"identity": identity, "onboarding": onboarding, "assistance": assistance,
	} {
		if bytes.Equal(first, other) {
			t.Fatalf("product automation selection key reused the %s purpose", name)
		}
	}
}

func TestParseSecretKeyProviderDerivesDetachedVersionedEnvelopeKeys(t *testing.T) {
	reservation := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, securityRootKeyBytes))
	rootV1 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x22}, securityRootKeyBytes))
	rootV2 := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{0x33}, securityRootKeyBytes))
	raw := fmt.Sprintf(
		`{"active_key_id":"v2","reservation_key":"%s","keys":{"v1":"%s","v2":"%s"}}`,
		reservation,
		rootV1,
		rootV2,
	)
	provider, err := ParseSecretKeyProvider(raw)
	if err != nil {
		t.Fatal(err)
	}
	active, err := provider.Active(context.Background())
	if err != nil || active.ID != "v2" || len(active.Key) != securityRootKeyBytes {
		t.Fatalf("active envelope key=%+v err=%v", active, err)
	}
	resolved, err := provider.Resolve(context.Background(), []string{"v1", "missing"})
	if err != nil || len(resolved) != 1 || len(resolved["v1"]) != securityRootKeyBytes {
		t.Fatalf("resolved envelope keys=%+v err=%v", resolved, err)
	}
	auditConfig, err := ParseAuditConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(active.Key, auditConfig.ActiveIntegrityKey.Secret) ||
		bytes.Equal(active.Key, auditConfig.ActiveReplayDigestKey.Secret) {
		t.Fatal("secret envelope and audit purposes derived the same key")
	}
	t.Setenv(SecurityKeyringEnvironment, raw)
	cursorKey, err := LoadCatalogCursorKey()
	if err != nil || len(cursorKey) != derivedSecurityKeyBytes ||
		bytes.Equal(cursorKey, active.Key) ||
		bytes.Equal(cursorKey, auditConfig.ActiveIntegrityKey.Secret) ||
		bytes.Equal(cursorKey, auditConfig.ActiveReplayDigestKey.Secret) {
		t.Fatalf("catalog cursor key is not purpose-separated: err=%v", err)
	}
	evidenceKeyID, evidenceKey, err := LoadProductFulfillmentEvidenceKey()
	if err != nil || evidenceKeyID != "v2" || len(evidenceKey) != derivedSecurityKeyBytes ||
		bytes.Equal(evidenceKey, cursorKey) || bytes.Equal(evidenceKey, active.Key) ||
		bytes.Equal(evidenceKey, auditConfig.ActiveIntegrityKey.Secret) {
		t.Fatalf("product fulfillment evidence key is not purpose-separated: id=%q err=%v", evidenceKeyID, err)
	}
	defer clear(evidenceKey)
	marketingCursorKey, err := LoadMarketingCursorKey()
	if err != nil || len(marketingCursorKey) != derivedSecurityKeyBytes ||
		bytes.Equal(marketingCursorKey, cursorKey) || bytes.Equal(marketingCursorKey, active.Key) ||
		bytes.Equal(marketingCursorKey, auditConfig.ActiveIntegrityKey.Secret) ||
		bytes.Equal(marketingCursorKey, auditConfig.ActiveReplayDigestKey.Secret) {
		t.Fatalf("marketing cursor key is not purpose-separated: err=%v", err)
	}
	commerceCursorKey, err := LoadCommerceOrderCursorKey()
	if err != nil || len(commerceCursorKey) != derivedSecurityKeyBytes ||
		bytes.Equal(commerceCursorKey, cursorKey) || bytes.Equal(commerceCursorKey, marketingCursorKey) ||
		bytes.Equal(commerceCursorKey, active.Key) ||
		bytes.Equal(commerceCursorKey, auditConfig.ActiveIntegrityKey.Secret) ||
		bytes.Equal(commerceCursorKey, auditConfig.ActiveReplayDigestKey.Secret) {
		t.Fatalf("commerce cursor key is not purpose-separated: err=%v", err)
	}
	billingCursorKey, err := LoadBillingDirectoryCursorKey()
	if err != nil || len(billingCursorKey) != derivedSecurityKeyBytes ||
		bytes.Equal(billingCursorKey, cursorKey) || bytes.Equal(billingCursorKey, marketingCursorKey) ||
		bytes.Equal(billingCursorKey, commerceCursorKey) || bytes.Equal(billingCursorKey, active.Key) ||
		bytes.Equal(billingCursorKey, auditConfig.ActiveIntegrityKey.Secret) ||
		bytes.Equal(billingCursorKey, auditConfig.ActiveReplayDigestKey.Secret) {
		t.Fatalf("billing cursor key is not purpose-separated: err=%v", err)
	}
	assistanceTokenKey, err := LoadCustomerAssistanceTokenKey()
	if err != nil || len(assistanceTokenKey) != derivedSecurityKeyBytes ||
		bytes.Equal(assistanceTokenKey, cursorKey) || bytes.Equal(assistanceTokenKey, marketingCursorKey) ||
		bytes.Equal(assistanceTokenKey, commerceCursorKey) || bytes.Equal(assistanceTokenKey, billingCursorKey) ||
		bytes.Equal(assistanceTokenKey, active.Key) ||
		bytes.Equal(assistanceTokenKey, auditConfig.ActiveIntegrityKey.Secret) ||
		bytes.Equal(assistanceTokenKey, auditConfig.ReservationLookupKey.Secret) {
		t.Fatalf("customer assistance token key is not purpose-separated: err=%v", err)
	}
	identityLookupKey, err := LoadIdentityLookupKey()
	if err != nil || len(identityLookupKey) != derivedSecurityKeyBytes ||
		bytes.Equal(identityLookupKey, cursorKey) || bytes.Equal(identityLookupKey, marketingCursorKey) ||
		bytes.Equal(identityLookupKey, commerceCursorKey) || bytes.Equal(identityLookupKey, billingCursorKey) ||
		bytes.Equal(identityLookupKey, assistanceTokenKey) ||
		bytes.Equal(identityLookupKey, active.Key) ||
		bytes.Equal(identityLookupKey, auditConfig.ActiveIntegrityKey.Secret) ||
		bytes.Equal(identityLookupKey, auditConfig.ReservationLookupKey.Secret) {
		t.Fatalf("identity lookup key is not purpose-separated: err=%v", err)
	}
	recentAuthKeys, err := ParseRecentAuthSigningKeyProvider(raw)
	if err != nil {
		t.Fatal(err)
	}
	recentAuthActive, err := recentAuthKeys.Active(context.Background())
	if err != nil || recentAuthActive.KeyID != "v2" || len(recentAuthActive.Key) != derivedSecurityKeyBytes ||
		bytes.Equal(recentAuthActive.Key, billingCursorKey) || bytes.Equal(recentAuthActive.Key, active.Key) ||
		bytes.Equal(recentAuthActive.Key, auditConfig.ActiveIntegrityKey.Secret) {
		t.Fatalf("recent authentication key is not purpose-separated: err=%v", err)
	}
	recentAuthResolved, err := recentAuthKeys.Resolve(context.Background(), "v1")
	if err != nil || len(recentAuthResolved) != derivedSecurityKeyBytes {
		t.Fatalf("retained recent authentication key unavailable: err=%v", err)
	}
	if unknown, resolveErr := recentAuthKeys.Resolve(context.Background(), "missing"); resolveErr != nil || len(unknown) != 0 {
		t.Fatalf("unknown recent authentication key was not opaque: value=%x err=%v", unknown, resolveErr)
	}
	recentAuthActive.Key[0] ^= 0xff
	recentAuthAgain, err := recentAuthKeys.Active(context.Background())
	if err != nil || bytes.Equal(recentAuthActive.Key, recentAuthAgain.Key) {
		t.Fatal("recent authentication active key was not detached")
	}
	active.Key[0] ^= 0xff
	again, err := provider.Active(context.Background())
	if err != nil || bytes.Equal(active.Key, again.Key) {
		t.Fatal("active key result was not detached from provider-owned material")
	}
	resolved["v1"][0] ^= 0xff
	againResolved, err := provider.Resolve(context.Background(), []string{"v1"})
	if err != nil || bytes.Equal(resolved["v1"], againResolved["v1"]) {
		t.Fatal("resolved key result was not detached from provider-owned material")
	}
}
