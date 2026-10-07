// This file verifies the Tencent provider-mapping completeness rules.

package electroniccontracttemplate

import "testing"

func TestValidateProviderMapping(t *testing.T) {
	order := int64(1)
	components := []ProviderComponent{
		{ComponentRef: "contract", Name: "Contract", TypeCode: "TEXT", Required: true, ValueCode: ProviderValueContractNo},
		{ComponentRef: "services", Name: "Services", TypeCode: "DYNAMIC_TABLE", Required: true, ValueCode: ProviderValueServiceLines},
		{ComponentRef: "date", Name: "Date", TypeCode: "SIGN_DATE", Required: true, RecipientRef: "customer", ValueCode: ProviderValueSigningDate},
	}
	recipients := []ProviderRecipient{
		{RecipientRef: "customer", SequenceNo: 1, TypeCode: ProviderRecipientEnterprise, RoleName: "Customer", RequireSign: true, RoutingOrder: &order, SignComponentCount: 2, PartyCode: PartyCustomer},
		{RecipientRef: "merchant", SequenceNo: 2, TypeCode: ProviderRecipientPromoter, RoleName: "Merchant", RequireSign: true, RoutingOrder: &order, IsPromoter: true, SignComponentCount: 1, PartyCode: PartyMerchant},
	}
	if reason := validateProviderMapping(ScopeEnterprise, SigningOrderParallel, components, recipients); reason != "" {
		t.Fatalf("valid mapping rejected: %s", reason)
	}
	if reason := validateProviderMapping(ScopeAllCustomers, SigningOrderParallel, components, recipients); reason != "provider_scope_incompatible" {
		t.Fatalf("all-customers fixed role reason=%q", reason)
	}
	components[1].ValueCode = ""
	if reason := validateProviderMapping(ScopeEnterprise, SigningOrderParallel, components, recipients); reason != "provider_mapping_incomplete" {
		t.Fatalf("missing required service table reason=%q", reason)
	}
}

func TestValidateProviderMappingChecksSigningOrder(t *testing.T) {
	customerOrder := int64(2)
	merchantOrder := int64(1)
	components := []ProviderComponent{{
		ComponentRef: "services", Name: "Services", TypeCode: "DYNAMIC_TABLE",
		Required: true, ValueCode: ProviderValueServiceLines,
	}}
	recipients := []ProviderRecipient{
		{RecipientRef: "customer", SequenceNo: 1, TypeCode: ProviderRecipientIndividual, RoleName: "Customer", RequireSign: true, RoutingOrder: &customerOrder, SignComponentCount: 1, PartyCode: PartyCustomer},
		{RecipientRef: "merchant", SequenceNo: 2, TypeCode: ProviderRecipientPromoter, RoleName: "Merchant", RequireSign: true, RoutingOrder: &merchantOrder, IsPromoter: true, SignComponentCount: 1, PartyCode: PartyMerchant},
	}
	if reason := validateProviderMapping(ScopePersonal, SigningOrderCustomerFirst, components, recipients); reason != "provider_signing_order_mismatch" {
		t.Fatalf("reversed signing order reason=%q", reason)
	}
}
