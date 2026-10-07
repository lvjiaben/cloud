package callback

import (
	"context"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-notify/backend/api/callback/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) WeChatOfficialVerify(ctx context.Context, req *v1.WeChatOfficialVerifyReq) (res *v1.WeChatOfficialVerifyRes, err error) {
	plaintext, err := c.notifications.VerifyWeChatCallback(ctx, notification.EncryptedCallbackInput{
		Signature: req.Signature, Timestamp: req.Timestamp, Nonce: req.Nonce, Encrypted: []byte(req.Encrypted),
	})
	if err != nil {
		return nil, err
	}
	g.RequestFromCtx(ctx).Response.Write(plaintext)
	return nil, nil
}
