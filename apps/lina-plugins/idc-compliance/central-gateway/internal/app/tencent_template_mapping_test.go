// This file verifies the bounded, safe Tencent template mapping projection.

package app

import (
	"testing"

	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"
)

func TestTencentTemplateMappingProjection(t *testing.T) {
	order := int64(1)
	source := &essbasic.TemplateInfo{
		Components: []*essbasic.Component{
			{
				ComponentId: templateString("service-details"), ComponentName: templateString("Service details"),
				ComponentType: templateString("DYNAMIC_TABLE"), ComponentRequired: templateBool(true),
			},
		},
		Recipients: []*essbasic.Recipient{
			{
				RecipientId: templateString("customer"), RecipientType: templateString("ENTERPRISE"),
				RoleName: templateString("Customer"), RequireSign: templateBool(true), RoutingOrder: &order,
			},
			{
				RecipientId: templateString("merchant"), RecipientType: templateString("PROMOTER"),
				RoleName: templateString("Merchant"), RequireSign: templateBool(true), RoutingOrder: &order,
				IsPromoter: templateBool(true),
			},
		},
		SignComponents: []*essbasic.Component{
			{
				ComponentId: templateString("customer-signature"), ComponentName: templateString("Customer signature"),
				ComponentType: templateString("SIGN_SIGNATURE"), ComponentRecipientId: templateString("customer"),
			},
			{
				ComponentId: templateString("customer-date"), ComponentName: templateString("Customer date"),
				ComponentType: templateString("SIGN_DATE"), ComponentRecipientId: templateString("customer"),
			},
			{
				ComponentId: templateString("merchant-seal"), ComponentName: templateString("Merchant seal"),
				ComponentType: templateString("SIGN_SEAL"), ComponentRecipientId: templateString("merchant"),
			},
		},
	}
	components, recipients, customerSignatures, customerSeals,
		merchantSignatures, merchantSeals, err := tencentTemplateMappingProjection(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(components) != 2 || components[0].ComponentRef != "customer-date" ||
		components[0].TypeCode != "SIGN_DATE" || !components[0].Required ||
		components[1].ComponentRef != "service-details" {
		t.Fatalf("unexpected components: %+v", components)
	}
	if len(recipients) != 2 || recipients[0].RecipientRef != "customer" ||
		recipients[0].SignComponentCount != 2 || recipients[1].RecipientRef != "merchant" ||
		recipients[1].SignComponentCount != 1 {
		t.Fatalf("unexpected recipients: %+v", recipients)
	}
	if customerSignatures != 1 || customerSeals != 0 || merchantSignatures != 0 || merchantSeals != 1 {
		t.Fatalf("unexpected signing counts: %d %d %d %d",
			customerSignatures, customerSeals, merchantSignatures, merchantSeals)
	}
}

func TestTencentTemplateMappingProjectionRejectsUnknownRecipient(t *testing.T) {
	source := &essbasic.TemplateInfo{
		Recipients: []*essbasic.Recipient{{
			RecipientId: templateString("customer"), RecipientType: templateString("ENTERPRISE"),
			RoleName: templateString("Customer"),
		}},
		Components: []*essbasic.Component{{
			ComponentId: templateString("service-details"), ComponentName: templateString("Service details"),
			ComponentType: templateString("DYNAMIC_TABLE"), ComponentRecipientId: templateString("missing"),
		}},
	}
	if _, _, _, _, _, _, err := tencentTemplateMappingProjection(source); err == nil {
		t.Fatal("unknown component recipient was accepted")
	}
}

func TestTencentTemplateMappingProjectionRejectsCombinedComponentOverflow(t *testing.T) {
	source := &essbasic.TemplateInfo{
		Components:     make([]*essbasic.Component, maximumTencentComponentsPerTemplate),
		SignComponents: []*essbasic.Component{nil},
	}
	if _, _, _, _, _, _, err := tencentTemplateMappingProjection(source); err == nil {
		t.Fatal("combined component overflow was accepted")
	}
}

func templateString(value string) *string { return &value }

func templateBool(value bool) *bool { return &value }
