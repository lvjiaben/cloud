package callback

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"io"

	"github.com/gogf/gf/v2/frame/g"

	"lina-plugin-idc-notify/backend/api/callback/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

var errInvalidCallback = errors.New("invalid notification callback")

func (c *ControllerV1) WeChatOfficialReceive(ctx context.Context, req *v1.WeChatOfficialReceiveReq) (res *v1.WeChatOfficialReceiveRes, err error) {
	request := g.RequestFromCtx(ctx)
	body, err := io.ReadAll(io.LimitReader(request.Body, 256*1024+1))
	if err != nil || len(body) == 0 || len(body) > 256*1024 {
		return nil, errInvalidCallback
	}
	defer clear(body)
	var envelope struct {
		Encrypted string `xml:"Encrypt"`
	}
	decoder := xml.NewDecoder(io.LimitReader(bytes.NewReader(body), 256*1024+1))
	decoder.Strict = true
	if err = decoder.Decode(&envelope); err != nil || envelope.Encrypted == "" || len(envelope.Encrypted) > 256*1024 {
		return nil, errInvalidCallback
	}
	if err = c.notifications.AcceptWeChatCallback(ctx, notification.EncryptedCallbackInput{
		Signature: req.Signature, Timestamp: req.Timestamp, Nonce: req.Nonce, Encrypted: []byte(envelope.Encrypted),
	}); err != nil {
		return nil, err
	}
	request.Response.Write("success")
	return nil, nil
}
