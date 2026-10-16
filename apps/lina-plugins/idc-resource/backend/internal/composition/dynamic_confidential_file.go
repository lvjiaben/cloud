// This file transfers purchase files over the resource owner's governed HTTP
// transport. Basic credentials and streams remain outside the WASM boundary.
package composition

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
	"lina-plugin-idc-resource/backend/internal/service/interfacecandidate"
)

const peerPurchaseFilePath = "/x/idc-channel/api/v1/channel"

var peerUploadTokenPattern = regexp.MustCompile(`^CFU_[0-9A-F]{32}$`)

func NewProductionPurchaseFileTransferFactory(secrets secretcap.Source) interfacecandidate.FileTransferFactory {
	return func(ctx context.Context, configuration channelconfig.Configuration, file resourceinterfaceprojectioncap.PurchaseFileTransfer) (string, error) {
		if configuration.PluginID != "idc-peer" || file.Body == nil || file.SizeBytes < 1 || file.SizeBytes > 5<<30 || len(file.SHA256Hex) != 64 {
			return "", gerror.New("purchase file transfer is invalid")
		}
		environment, cleanup, err := productionProviderEnvironment(ctx, configuration, secrets)
		if err != nil {
			return "", err
		}
		defer cleanup()
		// A dedicated, still origin/DNS/TLS-governed client permits large
		// streams; ordinary provider calls retain their 90-second deadline.
		outbound, ok := environment.HTTPClient.(*outboundHTTPClient)
		if !ok {
			return "", gerror.New("purchase file transport policy is unavailable")
		}
		client, ok := outbound.client.(*http.Client)
		if !ok {
			return "", gerror.New("purchase file transport policy is unavailable")
		}
		client.Timeout = time.Hour
		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			return "", gerror.New("purchase file transport policy is unavailable")
		}
		transport.ResponseHeaderTimeout = 5 * time.Minute
		handler := &confidentialHTTPHandler{configuration: configuration, environment: environment}
		return handler.transferPurchaseFile(ctx, file)
	}
}

func (h *confidentialHTTPHandler) transferPurchaseFile(ctx context.Context, file resourceinterfaceprojectioncap.PurchaseFileTransfer) (string, error) {
	body, err := json.Marshal(struct {
		ConfigurationRevision string `json:"configurationRevision"`
		FieldToken            string `json:"fieldToken"`
		FileName              string `json:"fileName"`
		SizeBytes             int64  `json:"sizeBytes"`
	}{file.ConfigurationRevision, file.FieldToken, file.FileName, file.SizeBytes})
	if err != nil {
		return "", err
	}
	reservation, err := h.performBusiness(ctx, resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: http.MethodPost, Path: peerPurchaseFilePath + "/catalog/products/" + url.PathEscape(file.PublicSlug) + "/file-uploads", JSONBody: body, UseAuthentication: true, ResponseFields: []string{"code", "data", "uploadToken", "expiresAt"}}, nil)
	if err != nil || reservation == nil || reservation.StatusCode != http.StatusOK || h.rejectConfiguredSecretEchoes(ctx, reservation.Data) != nil {
		return "", gerror.New("upstream file reservation failed")
	}
	var reserved struct {
		Code *int `json:"code"`
		Data struct {
			UploadToken string `json:"uploadToken"`
		} `json:"data"`
	}
	if json.Unmarshal(reservation.Data, &reserved) != nil || reserved.Code == nil || *reserved.Code != 0 || !peerUploadTokenPattern.MatchString(reserved.Data.UploadToken) {
		return "", gerror.New("upstream file reservation is invalid")
	}
	request := resourcecap.ConfidentialHTTPRequest{Method: http.MethodPut, Path: peerPurchaseFilePath + "/file-uploads/" + url.PathEscape(reserved.Data.UploadToken) + "/content", UseAuthentication: true}
	var result string
	err = h.withChannelAuthentication(ctx, &request, func() error {
		base, parseErr := url.Parse(h.configuration.Config.Endpoint)
		if parseErr != nil {
			return gerror.New("upstream file endpoint is invalid")
		}
		base.Path = strings.TrimRight(base.Path, "/") + request.Path
		digest := sha256.New()
		stream := &io.LimitedReader{R: io.TeeReader(file.Body, digest), N: file.SizeBytes + 1}
		httpRequest, requestErr := http.NewRequestWithContext(ctx, http.MethodPut, base.String(), stream)
		if requestErr != nil {
			return requestErr
		}
		httpRequest.ContentLength = file.SizeBytes
		httpRequest.Header.Set("Content-Type", "application/octet-stream")
		for name, value := range request.Headers {
			httpRequest.Header.Set(name, value)
		}
		response, callErr := h.environment.HTTPClient.Do(httpRequest)
		if callErr != nil || response == nil || response.Body == nil {
			return gerror.New("upstream file content transfer failed")
		}
		defer response.Body.Close()
		content, readErr := io.ReadAll(io.LimitReader(response.Body, maximumConfidentialHTTPResponse+1))
		defer clear(content)
		if readErr != nil || len(content) > maximumConfidentialHTTPResponse || response.StatusCode != http.StatusOK || stream.N != 1 || hex.EncodeToString(digest.Sum(nil)) != file.SHA256Hex || h.rejectConfiguredSecretEchoes(ctx, content) != nil {
			return gerror.New("upstream file content was not verified")
		}
		var uploaded struct {
			Code *int `json:"code"`
			Data struct {
				FileToken string `json:"fileToken"`
				Status    string `json:"status"`
				SizeBytes int64  `json:"sizeBytes"`
			} `json:"data"`
		}
		if json.Unmarshal(content, &uploaded) != nil || uploaded.Code == nil || *uploaded.Code != 0 || uploaded.Data.Status != "ready" || uploaded.Data.SizeBytes != file.SizeBytes || uploaded.Data.FileToken != reserved.Data.UploadToken {
			return gerror.New("upstream file is not ready")
		}
		result = uploaded.Data.FileToken
		return nil
	})
	return result, err
}
