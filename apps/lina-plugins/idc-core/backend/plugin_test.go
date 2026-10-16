// This file verifies source-plugin registration and embedded manifest ownership.

package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/billingcap"
	billingspi "lina-plugin-idc-core/backend/cap/billingcap/spi"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/productdeliverycap"
	secretspi "lina-plugin-idc-core/backend/cap/secretcap/spi"
	"lina-plugin-idc-core/backend/cap/supplierinterfacecap"
	"lina-plugin-idc-core/backend/internal/runtimeconfig"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

// TestPluginRegistrationIncludesEmbeddedManifest verifies core plugin delivery identity.
func TestPluginRegistrationIncludesEmbeddedManifest(t *testing.T) {
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("expected %s to be registered", pluginID)
	}
	if registered.GetEmbeddedFiles() == nil {
		t.Fatal("expected registered plugin to expose embedded files")
	}
	if registrars := registered.GetRouteRegistrars(); len(registrars) != 1 {
		t.Fatalf("expected one management route registrar, got %d", len(registrars))
	}
	descriptors := registered.GetCapabilityDescriptors()
	if len(descriptors) != 4 {
		t.Fatalf("unexpected capability descriptor count: %#v", descriptors)
	}
	foundIDC, foundBilling, foundProductDelivery, foundSupplierRuntime := false, false, false, false
	for _, descriptor := range descriptors {
		if descriptor.OwnerPluginID != pluginID || descriptor.Version != "v1" {
			t.Fatalf("unexpected capability descriptor: %#v", descriptor)
		}
		switch descriptor.Service {
		case idccap.ServiceName:
			if len(descriptor.Methods) != 1 {
				t.Fatalf("unexpected IDC descriptor: %#v", descriptor)
			}
			foundIDC = descriptor.Methods[0].Method == string(idccap.MethodMerchantContextCurrent)
		case billingcap.ServiceName:
			foundBilling = len(descriptor.Methods) == 13 &&
				descriptor.Methods[0].Method == string(billingcap.MethodChannelPaymentPrepare) &&
				descriptor.Methods[1].Method == string(billingcap.MethodChannelPaymentConfirm) &&
				descriptor.Methods[2].Method == string(billingcap.MethodChannelPaymentRelease) &&
				descriptor.Methods[3].Method == string(billingcap.MethodExceptionFundAllocate) &&
				descriptor.Methods[4].Method == string(billingcap.MethodPaymentClosureClaim) &&
				descriptor.Methods[5].Method == string(billingcap.MethodPaymentClosureReport) &&
				descriptor.Methods[6].Method == string(billingcap.MethodPaymentActorCurrent) &&
				descriptor.Methods[11].Method == string(billingcap.MethodRefundExecutionClaim) &&
				descriptor.Methods[12].Method == string(billingcap.MethodRefundExecutionReport)
		case productdeliverycap.ServiceName:
			foundProductDelivery = len(descriptor.Methods) == 2 &&
				descriptor.Methods[0].Method == productdeliverycap.MethodCurrentCustomerDeliveriesList &&
				descriptor.Methods[1].Method == productdeliverycap.MethodCurrentCustomerDeliveryGet
		case supplierinterfacecap.ServiceName:
			foundSupplierRuntime = len(descriptor.Methods) == 5 &&
				descriptor.Methods[0].Method == supplierinterfacecap.MethodConfigurationGet &&
				descriptor.Methods[0].ResourceKind == "resource" &&
				descriptor.Methods[1].Method == supplierinterfacecap.MethodFilesList &&
				descriptor.Methods[2].Method == supplierinterfacecap.MethodFileRead &&
				descriptor.Methods[3].Method == supplierinterfacecap.MethodOutboundHTTP &&
				descriptor.Methods[3].Risk == "execute" &&
				descriptor.Methods[4].Method == supplierinterfacecap.MethodOutboundSession &&
				descriptor.Methods[4].Risk == "execute"
		}
	}
	if !foundIDC || !foundBilling || !foundProductDelivery || !foundSupplierRuntime {
		t.Fatalf("missing IDC, billing, product delivery, or supplier runtime capability descriptor: %#v", descriptors)
	}

	manifest, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded manifest: %v", err)
	}
	if !strings.Contains(string(manifest), "id: "+pluginID) {
		t.Fatalf("embedded manifest does not identify %s", pluginID)
	}
	for _, fragment := range []string{
		"version: v0.1.101",
		"      - provision",
		"      - query",
		"key: plugin:idc-core:catalog-automation-interfaces",
		"path: /idc/catalog/automation-interfaces",
	} {
		if !strings.Contains(string(manifest), fragment) {
			t.Fatalf("embedded manifest is missing release contract %q", fragment)
		}
	}
	if strings.Contains(string(manifest), "fulfillment_contract:") {
		t.Fatal("product-fulfillment owner must not declare a provider fulfillment contract")
	}
	var declaration struct {
		BusinessExtensions []*capmodel.BusinessExtensionDeclaration `yaml:"business_extensions"`
	}
	if err := yaml.Unmarshal(manifest, &declaration); err != nil {
		t.Fatalf("decode embedded owner manifest: %v", err)
	}
	var ownerCount int
	for _, extension := range declaration.BusinessExtensions {
		if extension == nil || extension.Point != capmodel.BusinessExtensionProductFulfillment {
			continue
		}
		ownerCount++
		if extension.Role != capmodel.BusinessExtensionRoleOwner || extension.Owner != pluginID ||
			extension.Version != "v1" || !slices.Contains(extension.Methods, "product.configuration.validate") {
			t.Fatalf("product validation must be published by the v1 fulfillment owner: %+v", extension)
		}
	}
	if ownerCount != 1 {
		t.Fatalf("expected one product fulfillment owner declaration, got %d", ownerCount)
	}
}

func TestProductFulfillmentSystemReasonResourcesAreComplete(t *testing.T) {
	catalog, err := loadProductFulfillmentSystemReasonCatalog()
	if err != nil {
		t.Fatalf("load product fulfillment system reasons: %v", err)
	}
	for _, code := range productfulfillment.RequiredSystemTransitionReasonCodes() {
		reason, resolveErr := catalog.ResolveSystemTransitionReason(code)
		if resolveErr != nil || reason.LabelZhCN == "" || reason.LabelEnUS == "" {
			t.Fatalf("system reason %q is incomplete: reason=%+v err=%v", code, reason, resolveErr)
		}
	}
}

func TestPluginDeclaresBuildableSourceSecretFactory(t *testing.T) {
	reservation := base64.StdEncoding.EncodeToString(repeatedBytes(0x31))
	root := base64.StdEncoding.EncodeToString(repeatedBytes(0x72))
	t.Setenv(runtimeconfig.SecurityKeyringEnvironment, fmt.Sprintf(
		`{"active_key_id":"v1","reservation_key":"%s","keys":{"v1":"%s"}}`,
		reservation,
		root,
	))
	snapshot, err := secretspi.Freeze()
	if err != nil || snapshot.Factory() == nil {
		t.Fatalf("freeze declared source secret factory: factory=%v err=%v", snapshot.Factory(), err)
	}
	source, err := snapshot.Factory()(context.Background())
	if err != nil || source == nil {
		t.Fatalf("construct declared source secret service: source=%T err=%v", source, err)
	}
}

func TestPluginDeclaresSourceBillingFactory(t *testing.T) {
	snapshot, err := billingspi.Freeze()
	if err != nil || snapshot.Factory() == nil {
		t.Fatalf("freeze declared source billing factory: factory=%v err=%v", snapshot.Factory(), err)
	}
	service, err := snapshot.Factory()(context.Background())
	if err != nil || service == nil {
		t.Fatalf("construct declared source billing client: service=%T err=%v", service, err)
	}
}

func repeatedBytes(value byte) []byte {
	result := make([]byte, 32)
	for index := range result {
		result[index] = value
	}
	return result
}

// TestEmbeddedCapabilityLifecycleContract verifies the embedded lifecycle declaration.
func TestEmbeddedCapabilityLifecycleContract(t *testing.T) {
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %q is not registered", pluginID)
	}
	content, err := fs.ReadFile(registered.GetEmbeddedFiles(), "manifest/config/capabilities.yaml")
	if err != nil {
		t.Fatalf("read embedded capability lifecycle: %v", err)
	}
	for _, fragment := range []string{
		"stop_new",
		"drain_active",
		"preserve_history",
		"resume_after_validation",
	} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("embedded capability lifecycle is missing %q", fragment)
		}
	}
}
