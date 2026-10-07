package account

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/api/account/v1"
	"lina-plugin-idc-compliance/backend/internal/service/personalverification"
)

func (c *ControllerV1) PersonalVerificationCreate(ctx context.Context, req *v1.PersonalVerificationCreateReq) (res *v1.PersonalVerificationCreateRes, err error) {
	if req == nil {
		return nil, bizerr.NewCode(personalverification.CodeInvalid)
	}
	name := []byte(req.CertName)
	number := []byte(req.CertNo)
	req.CertName, req.CertNo = "", ""
	defer clearBytes(name)
	defer clearBytes(number)
	started, err := c.verification.Start(ctx, personalverification.StartInput{
		ConsentNo: req.ConsentNo, CertType: req.CertType,
		CertName: name, CertNo: number, ReturnURL: c.returnURL,
	})
	if err != nil {
		return nil, err
	}
	return &v1.PersonalVerificationCreateRes{Item: personalVerificationItem(started)}, nil
}
