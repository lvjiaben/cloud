// This file maps bounded channel pages and preserves configuration write contracts.
package admin

import (
	"context"
	"encoding/json"
	"strings"

	"lina-plugin-idc-payment/backend/api/admin/v1"
	"lina-plugin-idc-payment/backend/cap/paymentcap"
	"lina-plugin-idc-payment/backend/internal/service/channelconfig"
)

func (c *ControllerV1) PaymentChannelList(ctx context.Context, req *v1.PaymentChannelListReq) (*v1.PaymentChannelListRes, error) {
	page, err := c.channels.ListManaged(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	providers, err := c.channels.ListProviderCandidates(ctx)
	if err != nil {
		return nil, err
	}
	result := &v1.PaymentChannelListRes{
		Channels:           make([]*v1.PaymentChannelItem, 0, len(page.Items)),
		ProviderCandidates: make([]*v1.PaymentProviderCandidateItem, 0, len(providers)),
		Total:              page.Total,
	}
	for index := range page.Items {
		result.Channels = append(result.Channels, mapPaymentChannel(&page.Items[index]))
	}
	for _, provider := range providers {
		schema, marshalErr := json.Marshal(provider.InterfaceConfiguration)
		if marshalErr != nil {
			return nil, marshalErr
		}
		modes := make([]string, len(provider.Descriptor.Modes))
		for index, mode := range provider.Descriptor.Modes {
			modes[index] = string(mode)
		}
		result.ProviderCandidates = append(result.ProviderCandidates, &v1.PaymentProviderCandidateItem{
			PluginID: provider.PluginID, ProviderCode: string(provider.Descriptor.ProviderCode),
			DisplayName: provider.DisplayName, ReleaseVersion: provider.ReleaseVersion,
			EndpointOrigin: provider.Descriptor.EndpointOrigin, Modes: modes,
			Currencies: append([]string(nil), provider.Descriptor.Currencies...), InterfaceConfiguration: schema,
		})
	}
	return result, nil
}

func (c *ControllerV1) PaymentChannelCreate(ctx context.Context, req *v1.PaymentChannelCreateReq) (*v1.PaymentChannelCreateRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, err
	}
	modes := make([]paymentcap.Mode, 0, len(req.EnabledModes))
	for _, mode := range req.EnabledModes {
		modes = append(modes, paymentcap.Mode(mode))
	}
	merchantReference := strings.TrimSpace(req.MerchantReference)
	if merchantReference == "" {
		merchantReference = strings.TrimSpace(req.MerchantID)
	}
	channel, err := c.channels.Save(ctx, channelconfig.SaveInput{
		ChannelNo:         strings.TrimSpace(req.ChannelNo),
		Provider:          paymentcap.ProviderCode(req.ProviderCode),
		DisplayName:       strings.TrimSpace(req.DisplayName),
		EndpointOrigin:    strings.TrimSpace(req.EndpointOrigin),
		MerchantReference: merchantReference,
		EnabledModes:      modes,
		IdempotencyKey:    strings.TrimSpace(req.ChannelNo),
		ActorID:           actor,
		Credential: channelconfig.CredentialEnvelope{
			MerchantID: strings.TrimSpace(req.MerchantID),
			AppID:      strings.TrimSpace(req.AppID),
			Secret:     req.Secret,
			PrivateKey: req.PrivateKey,
			PublicKey:  req.PublicKey,
			Extra: map[string]string{
				"api_v3_key":              req.APIV3Key,
				"certificate_serial_no":   strings.TrimSpace(req.CertificateSerialNo),
				"jsapi_authorized":        boolText(req.JSAPIAuthorized),
				"wechatpay_public_key_id": strings.TrimSpace(req.WechatPayPublicKeyID),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	return &v1.PaymentChannelCreateRes{Channel: mapPaymentChannel(channel)}, nil
}

func (c *ControllerV1) PaymentChannelStatus(ctx context.Context, req *v1.PaymentChannelStatusReq) (*v1.PaymentChannelStatusRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, err
	}
	channel, err := c.channels.SetStatus(ctx, req.ChannelNo, req.StatusCode, req.ExpectedVersion, actor)
	if err != nil {
		return nil, err
	}
	return &v1.PaymentChannelStatusRes{Channel: mapPaymentChannel(channel)}, nil
}

func (c *ControllerV1) PaymentChannelConnectionVerification(ctx context.Context, req *v1.PaymentChannelConnectionVerificationReq) (*v1.PaymentChannelConnectionVerificationRes, error) {
	actor, err := c.currentActor(ctx)
	if err != nil {
		return nil, err
	}
	channel, err := c.channels.VerifyConnection(ctx, req.ChannelNo, req.ExpectedVersion, actor)
	if err != nil {
		return nil, err
	}
	return &v1.PaymentChannelConnectionVerificationRes{Channel: mapPaymentChannel(channel)}, nil
}

func mapPaymentChannel(item *channelconfig.Channel) *v1.PaymentChannelItem {
	if item == nil {
		return nil
	}
	modes := make([]string, 0, len(item.Modes))
	for _, mode := range item.Modes {
		modes = append(modes, string(mode))
	}
	verifiedAt := int64(0)
	if !item.VerifiedAt.IsZero() {
		verifiedAt = item.VerifiedAt.UnixMilli()
	}
	return &v1.PaymentChannelItem{
		ChannelNo: item.ChannelNo, ProviderCode: string(item.Provider), DisplayName: item.DisplayName,
		EndpointOrigin: item.EndpointOrigin, MerchantReference: item.MerchantReference,
		StatusCode: item.StatusCode, Modes: modes, Currencies: item.Currencies,
		Version: item.Version, VerifiedAt: verifiedAt,
	}
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
