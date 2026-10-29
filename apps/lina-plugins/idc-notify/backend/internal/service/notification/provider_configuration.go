package notification

import (
	"context"
	"strconv"
)

const (
	providerSMTP       = "smtp_tls_v1"
	providerAliyunSMS  = "aliyun_sms_v20170525"
	providerSMSBao     = "smsbao_http_v1"
	providerTencentSMS = "tencent_cloud_sms_v20210111"
)

func (s *serviceImpl) ConfigureAliyunSMS(ctx context.Context, input AliyunSMSConfigurationInput) (*Channel, error) {
	defer clear(input.AccessKeyID)
	defer clear(input.AccessKeySecret)
	return s.ConfigureChannel(ctx, ChannelConfigurationInput{
		ChannelCode: "sms", ProviderCode: providerAliyunSMS,
		NonSecret: map[string]string{"sign_name": input.SignName},
		Secrets: map[string][]byte{
			"access_key_id":     append([]byte(nil), input.AccessKeyID...),
			"access_key_secret": append([]byte(nil), input.AccessKeySecret...),
		},
		ExpectedVersion: input.ExpectedVersion, IdempotencyKey: input.IdempotencyKey,
	})
}

func (s *serviceImpl) ConfigureSMSBao(ctx context.Context, input SMSBaoConfigurationInput) (*Channel, error) {
	defer clear(input.Username)
	defer clear(input.APIKey)
	return s.ConfigureChannel(ctx, ChannelConfigurationInput{
		ChannelCode: "sms", ProviderCode: providerSMSBao,
		NonSecret: map[string]string{"sign_name": input.SignName, "product_id": input.ProductID},
		Secrets: map[string][]byte{
			"username": append([]byte(nil), input.Username...),
			"api_key":  append([]byte(nil), input.APIKey...),
		},
		ExpectedVersion: input.ExpectedVersion, IdempotencyKey: input.IdempotencyKey,
	})
}

func (s *serviceImpl) ConfigureSMTP(ctx context.Context, input SMTPConfigurationInput) (*Channel, error) {
	defer clear(input.Password)
	return s.ConfigureChannel(ctx, ChannelConfigurationInput{
		ChannelCode: "email", ProviderCode: providerSMTP,
		NonSecret: map[string]string{
			"host": input.Host, "port": strconv.Itoa(input.Port), "security": input.Security,
			"username": input.Username, "from_address": input.FromAddress, "from_name": input.FromName,
		},
		Secrets:         map[string][]byte{"password": append([]byte(nil), input.Password...)},
		ExpectedVersion: input.ExpectedVersion, IdempotencyKey: input.IdempotencyKey,
	})
}

func (s *serviceImpl) ConfigureTencentSMS(ctx context.Context, input TencentSMSConfigurationInput) (*Channel, error) {
	defer clear(input.SecretID)
	defer clear(input.SecretKey)
	return s.ConfigureChannel(ctx, ChannelConfigurationInput{
		ChannelCode: "sms", ProviderCode: providerTencentSMS,
		NonSecret:       map[string]string{"region": input.Region, "sms_sdk_app_id": input.SmsSdkAppID, "sign_name": input.SignName},
		Secrets:         map[string][]byte{"secret_id": append([]byte(nil), input.SecretID...), "secret_key": append([]byte(nil), input.SecretKey...)},
		ExpectedVersion: input.ExpectedVersion, IdempotencyKey: input.IdempotencyKey,
	})
}
