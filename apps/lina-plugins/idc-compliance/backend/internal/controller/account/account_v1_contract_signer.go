package account

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/contractsigner"
)

func (c *ControllerV1) ContractSignerList(ctx context.Context, req *v1.ContractSignerListReq) (*v1.ContractSignerListRes, error) {
	items, err := c.contractSigners.ListCurrent(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]v1.ContractSignerItem, len(items))
	for index := range items {
		result[index] = contractSignerItem(items[index])
	}
	return &v1.ContractSignerListRes{Items: result}, nil
}

func (c *ControllerV1) ContractSignerCreate(ctx context.Context, req *v1.ContractSignerCreateReq) (*v1.ContractSignerCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(contractsigner.CodeInvalid)
	}
	name, number, phone := []byte(req.CertName), []byte(req.CertNo), []byte(req.Phone)
	req.CertName, req.CertNo, req.Phone = "", "", ""
	defer clearBytes(name)
	defer clearBytes(number)
	defer clearBytes(phone)
	item, err := c.contractSigners.Create(ctx, contractsigner.CreateInput{CertType: req.CertType, CertName: name, CertNo: number, Phone: phone,
		ConsentAccepted: req.ConsentAccepted, ConsentPolicyVersion: req.ConsentPolicyVersion, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.ContractSignerCreateRes{Item: contractSignerItem(*item)}, nil
}

func (c *ControllerV1) ContractSignerIdentityCreate(ctx context.Context, req *v1.ContractSignerIdentityCreateReq) (*v1.ContractSignerIdentityCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(contractsigner.CodeInvalid)
	}
	item, err := c.contractSigners.StartIdentity(ctx, req.SignerNo, c.signerReturnURL)
	if err != nil {
		return nil, err
	}
	result := v1.ContractSignerIdentityStartItem{SignerNo: item.SignerNo, AttemptNo: item.AttemptNo, StatusCode: item.Status, ContinuationURL: item.ContinuationURL}
	if item.ExpiresAt != nil {
		result.ExpiresAt = item.ExpiresAt.UnixMilli()
	}
	return &v1.ContractSignerIdentityCreateRes{Item: result}, nil
}

func (c *ControllerV1) ContractSignerIdentityCheckCreate(ctx context.Context, req *v1.ContractSignerIdentityCheckCreateReq) (*v1.ContractSignerIdentityCheckCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(contractsigner.CodeInvalid)
	}
	item, err := c.contractSigners.QueryIdentity(ctx, req.SignerNo, req.AttemptNo)
	if err != nil {
		return nil, err
	}
	return &v1.ContractSignerIdentityCheckCreateRes{Item: contractSignerItem(*item)}, nil
}

func (c *ControllerV1) ContractSignerPhoneChallengeCreate(ctx context.Context, req *v1.ContractSignerPhoneChallengeCreateReq) (*v1.ContractSignerPhoneChallengeCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(contractsigner.CodeInvalid)
	}
	nonce := []byte(req.BrowserNonce)
	req.BrowserNonce = ""
	defer clearBytes(nonce)
	item, err := c.contractSigners.CreatePhoneChallenge(ctx, req.SignerNo, nonce, req.Locale, req.PresentationKind)
	if err != nil {
		return nil, err
	}
	return &v1.ContractSignerPhoneChallengeCreateRes{Item: v1.ContractSignerHumanChallengeItem{ChallengeNo: item.ChallengeNo, ProviderCode: item.ProviderCode, ProviderAction: item.ProviderAction, Kind: item.Kind, ImageDataURL: item.ImageDataURL, AccessiblePrompt: item.AccessiblePrompt, SiteKey: item.SiteKey, ExpiresAt: item.ExpiresAt.UnixMilli()}}, nil
}

func (c *ControllerV1) ContractSignerPhoneChallengeCompletionCreate(ctx context.Context, req *v1.ContractSignerPhoneChallengeCompletionCreateReq) (*v1.ContractSignerPhoneChallengeCompletionCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(contractsigner.CodeInvalid)
	}
	nonce, answer, token := []byte(req.BrowserNonce), []byte(req.Answer), []byte(req.ProviderToken)
	req.BrowserNonce, req.Answer, req.ProviderToken = "", "", ""
	defer clearBytes(nonce)
	defer clearBytes(answer)
	defer clearBytes(token)
	item, err := c.contractSigners.CompletePhoneChallenge(ctx, req.SignerNo, nonce, req.Locale, req.ChallengeNo, answer, token)
	if err != nil {
		return nil, err
	}
	return &v1.ContractSignerPhoneChallengeCompletionCreateRes{Item: v1.ContractSignerHumanProofItem{ProofToken: item.ProofToken, ExpiresAt: item.ExpiresAt.UnixMilli()}}, nil
}

func (c *ControllerV1) ContractSignerPhoneCodeCreate(ctx context.Context, req *v1.ContractSignerPhoneCodeCreateReq) (*v1.ContractSignerPhoneCodeCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(contractsigner.CodeInvalid)
	}
	nonce := []byte(req.BrowserNonce)
	req.BrowserNonce = ""
	defer clearBytes(nonce)
	request := g.RequestFromCtx(ctx)
	if request == nil {
		return nil, bizerr.NewCode(contractsigner.CodeInvalid)
	}
	ip := []byte(request.GetClientIp())
	defer clearBytes(ip)
	item, err := c.contractSigners.SendPhoneCode(ctx, req.SignerNo, nonce, req.Locale, req.ProofToken, req.IdempotencyKey, ip)
	if err != nil {
		return nil, err
	}
	return &v1.ContractSignerPhoneCodeCreateRes{Item: v1.ContractSignerPhoneCodeItem{VerificationNo: item.VerificationNo, MaskedDestination: item.MaskedDestination, DeliveryState: item.DeliveryState, ExpiresAt: item.ExpiresAt.UnixMilli(), RetryAfterAt: item.RetryAfterAt.UnixMilli()}}, nil
}

func (c *ControllerV1) ContractSignerPhoneVerificationCreate(ctx context.Context, req *v1.ContractSignerPhoneVerificationCreateReq) (*v1.ContractSignerPhoneVerificationCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(contractsigner.CodeInvalid)
	}
	nonce, code := []byte(req.BrowserNonce), []byte(req.Code)
	req.BrowserNonce, req.Code = "", ""
	defer clearBytes(nonce)
	defer clearBytes(code)
	item, err := c.contractSigners.VerifyPhone(ctx, req.SignerNo, nonce, req.Locale, req.VerificationNo, code)
	if err != nil {
		return nil, err
	}
	return &v1.ContractSignerPhoneVerificationCreateRes{Item: contractSignerItem(*item)}, nil
}

func (c *ControllerV1) ContractSignerDisablementCreate(ctx context.Context, req *v1.ContractSignerDisablementCreateReq) (*v1.ContractSignerDisablementCreateRes, error) {
	if req == nil {
		return nil, bizerr.NewCode(contractsigner.CodeInvalid)
	}
	item, err := c.contractSigners.Disable(ctx, req.SignerNo, req.ExpectedVersion, req.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	return &v1.ContractSignerDisablementCreateRes{Item: contractSignerItem(*item)}, nil
}

func contractSignerItem(item contractsigner.Signer) v1.ContractSignerItem {
	result := v1.ContractSignerItem{SignerNo: item.SignerNo, MaskedName: item.MaskedName, MaskedIdentityNo: item.MaskedIdentityNo, MaskedPhone: item.MaskedPhone, IdentityStatusCode: item.IdentityStatus, IdentityAttemptNo: item.IdentityAttemptNo, IdentityVerificationVersion: item.IdentityVerificationVersion, PhoneStatusCode: item.PhoneStatus, PhoneVerificationVersion: item.PhoneVerificationVersion, StatusCode: item.Status, AuthorizationVersion: item.AuthorizationVersion, Version: item.Version, CreatedAt: item.CreatedAt.UnixMilli(), UpdatedAt: item.UpdatedAt.UnixMilli()}
	result.IdentityVerifiedAt = timeMillis(item.IdentityVerifiedAt)
	result.PhoneVerifiedAt = timeMillis(item.PhoneVerifiedAt)
	return result
}

func timeMillis(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.UnixMilli()
}
