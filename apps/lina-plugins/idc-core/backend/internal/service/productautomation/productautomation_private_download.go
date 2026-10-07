package productautomation

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
)

const maximumPrivateSupplierDownloadBytes = 8 << 20

type PrivateSupplierDownloader interface {
	FetchPrivateSupplierDownload(context.Context, PrivateSupplierDownloadRequest) ([]byte, string, error)
}

// PrivateSupplierDownloadRequest is constructed only by the business owner
// from one exact frozen provider declaration. It is not a host-service method
// and cannot be called by a managed guest.
type PrivateSupplierDownloadRequest struct {
	CallerPluginID, ContextID, Path, EndpointField string
	Body                                           []byte
	SecretHeaders                                  map[string]string
}

// FetchPrivateSupplierDownload returns a bounded supplier binary directly to
// the owner. The managed provider is never invoked and never observes bytes.
func (i *RuntimeCapabilityInvoker) FetchPrivateSupplierDownload(ctx context.Context, request PrivateSupplierDownloadRequest) ([]byte, string, error) {
	if i == nil {
		return nil, "", gerror.New("private supplier download is unavailable")
	}
	i.mu.RLock()
	service := i.service
	i.mu.RUnlock()
	if service == nil {
		return nil, "", gerror.New("private supplier download is unavailable")
	}
	return service.fetchPrivateSupplierDownload(ctx, request)
}

func (s *serviceImpl) fetchPrivateSupplierDownload(ctx context.Context, request PrivateSupplierDownloadRequest) ([]byte, string, error) {
	if s == nil || validatePrivateSupplierDownloadRequest(ctx, request) != nil {
		return nil, "", gerror.New("private supplier download request is invalid")
	}
	runtimeContext, ok := s.runtimeContexts.get(request.ContextID, request.CallerPluginID)
	if !ok || runtimeContext.purpose != "existing_business" {
		return nil, "", gerror.New("private supplier download context is unavailable")
	}
	endpoint, ok := runtimeEndpoint(runtimeContext, request.EndpointField)
	if !ok {
		return nil, "", gerror.New("private supplier download endpoint is unavailable")
	}
	client, cleanup, err := s.outboundClientFactory(endpoint)
	if err != nil || client == nil || cleanup == nil {
		return nil, "", gerror.New("private supplier download client is unavailable")
	}
	defer cleanup()
	httpRequest, err := newPrivateSupplierHTTPRequest(ctx, endpoint, request)
	if err != nil {
		return nil, "", gerror.New("private supplier download request is unavailable")
	}
	secretValues, err := s.attachPrivateSupplierSecrets(ctx, httpRequest, request)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		for _, value := range secretValues {
			clear(value)
		}
	}()
	return fetchPrivateSupplierResponse(client, httpRequest, secretValues)
}

func validatePrivateSupplierDownloadRequest(ctx context.Context, request PrivateSupplierDownloadRequest) error {
	if ctx == nil || ctx.Err() != nil || request.CallerPluginID == "" || request.ContextID == "" ||
		request.EndpointField != "endpoint" || len(request.Path) < 2 || len(request.Path) > 256 || request.Path[0] != '/' ||
		strings.ContainsAny(request.Path, "?#\\") || strings.Contains(request.Path, "..") || len(request.Body) == 0 || len(request.Body) > 4096 ||
		len(request.SecretHeaders) < 1 || len(request.SecretHeaders) > 8 {
		return gerror.New("private supplier download request is invalid")
	}
	return nil
}

func newPrivateSupplierHTTPRequest(ctx context.Context, endpoint string, request PrivateSupplierDownloadRequest) (*http.Request, error) {
	base, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + request.Path
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), bytes.NewReader(request.Body))
	if err != nil {
		return nil, err
	}
	httpRequest.Header.Set("Accept", "application/zip,application/octet-stream")
	httpRequest.Header.Set("Content-Type", "application/json")
	return httpRequest, nil
}

func (s *serviceImpl) attachPrivateSupplierSecrets(ctx context.Context, httpRequest *http.Request, request PrivateSupplierDownloadRequest) ([][]byte, error) {
	secretValues := make([][]byte, 0, len(request.SecretHeaders))
	headers := make([]string, 0, len(request.SecretHeaders))
	for header := range request.SecretHeaders {
		headers = append(headers, header)
	}
	slices.Sort(headers)
	for _, header := range headers {
		if header != "X-API-Key" && header != "X-API-Secret" {
			return nil, gerror.New("private supplier download credential mapping is invalid")
		}
		var copied []byte
		if err := s.runtimeSecretAccess(ctx, request.CallerPluginID, request.ContextID, request.SecretHeaders[header], func(value []byte) error {
			copied = append([]byte(nil), value...)
			httpRequest.Header.Set(header, string(value))
			return nil
		}); err != nil {
			clear(copied)
			for _, value := range secretValues {
				clear(value)
			}
			return nil, gerror.New("private supplier download credential is unavailable")
		}
		secretValues = append(secretValues, copied)
	}
	return secretValues, nil
}

func fetchPrivateSupplierResponse(client supplierOutboundDoer, httpRequest *http.Request, secretValues [][]byte) ([]byte, string, error) {
	response, err := client.Do(httpRequest)
	if err != nil || response == nil || response.Body == nil {
		return nil, "", gerror.New("private supplier download request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", gerror.New("private supplier download response is unavailable")
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, maximumPrivateSupplierDownloadBytes+1))
	if err != nil || len(content) == 0 || len(content) > maximumPrivateSupplierDownloadBytes {
		clear(content)
		return nil, "", gerror.New("private supplier download response is invalid")
	}
	for _, secret := range secretValues {
		if supplierSecretAppears(content, secret) {
			clear(content)
			return nil, "", gerror.New("private supplier download contains credential material")
		}
	}
	contentType := strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])
	return content, contentType, nil
}
