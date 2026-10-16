package serviceaction

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"path"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

type DownloadInput struct {
	EntitlementNo string
	DownloadKey   string
	Actor         Actor
}

type DownloadTicket struct {
	Ticket, FileName string
	ExpiresAt        time.Time
}

type DownloadContent struct {
	FileName, ContentType string
	Content               []byte
}

func (s *service) CreateDownloadTicket(ctx context.Context, input DownloadInput) (*DownloadTicket, error) {
	if s == nil || s.downloader == nil || s.contexts == nil || s.storage == nil || validateActor(input.Actor) != nil ||
		!safeReferencePattern.MatchString(input.EntitlementNo) || !safeReferencePattern.MatchString(input.DownloadKey) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	frozen, declaration, err := s.resolveProvider(ctx, input.EntitlementNo)
	if err != nil || input.Actor.Audience == AudienceCustomer && frozen.CustomerNo != input.Actor.CustomerNo {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var declared *productfulfillmentcap.ManagedServiceDownload
	for index := range declaration.ServiceDownloads {
		if declaration.ServiceDownloads[index].Key == input.DownloadKey {
			declared = &declaration.ServiceDownloads[index]
			break
		}
	}
	if declared == nil || !contains(declared.Audiences, input.Actor.Audience) {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	// A fresh owner query must prove issuance before any supplier binary call.
	if err = s.requireIssued(ctx, frozen, declaration); err != nil {
		return nil, err
	}
	runtime, err := s.contexts.IssueExistingBusinessRuntimeContext(ctx, productautomation.ExistingBusinessRuntimeContextInput{PluginID: frozen.PluginID, ReleaseVersion: frozen.ReleaseVersion, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: frozen.ContractVersion, ReleaseDigest: frozen.ReleaseDigest, DeclarationDigest: frozen.DeclarationDigest, Method: declared.Method, InterfaceInstanceNo: frozen.InterfaceInstanceNo, InterfaceConfigurationRevision: frozen.InterfaceConfigurationRevision, IntentNo: frozen.SourceIntentNo, OrderNo: frozen.OrderNo, OrderItemNo: frozen.OrderItemNo, ConfigurationSnapshotNo: frozen.ConfigurationSnapshotNo})
	if err != nil || runtime == nil || runtime.Release == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer runtime.Release()
	bodyFields := make(map[string]string, len(declared.FixedBody)+1)
	for field, value := range declared.FixedBody {
		bodyFields[field] = value
	}
	bodyFields[declared.ExternalReferenceField] = frozen.ExternalReference
	body, err := json.Marshal(bodyFields)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	upstream, _, err := s.downloader.FetchPrivateSupplierDownload(ctx, productautomation.PrivateSupplierDownloadRequest{CallerPluginID: frozen.PluginID, ContextID: runtime.ContextID, Path: declared.Path, EndpointField: declared.EndpointField, Body: body, SecretHeaders: declared.SecretHeaders})
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	defer clear(upstream)
	safeArchive, err := sanitizeCertificateArchive(upstream)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	defer clear(safeArchive)
	ticket, err := randomReference(s.random, "sdt_")
	if err != nil {
		return nil, err
	}
	ticketDigest := digest([]byte(ticket))
	objectDigest := digest(safeArchive)
	objectPath := "private/service-download/" + ticketDigest + ".zip"
	if _, err = s.storage.Put(ctx, storagecap.PutInput{Path: objectPath, Body: bytes.NewReader(safeArchive), Size: int64(len(safeArchive)), ContentType: "application/zip", Overwrite: false}); err != nil {
		return nil, bizerr.WrapCode(gerror.Wrap(err, "store private certificate download"), CodeUnavailable)
	}
	fileName := "certificate-" + input.EntitlementNo + ".zip"
	var expiresAt time.Time
	err = s.database.GetScan(ctx, &expiresAt, `INSERT INTO plugin_idc_core_service_download_ticket(tenant_id,ticket_digest,entitlement_no,customer_no,audience_code,actor_user_id,action_key,object_path,object_digest,file_name,content_type,expires_at) VALUES(0,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP+INTERVAL '2 minutes') RETURNING expires_at`, ticketDigest, frozen.EntitlementNo, frozen.CustomerNo, input.Actor.Audience, input.Actor.UserID, declared.Key, objectPath, objectDigest, fileName, "application/zip")
	if err != nil {
		_ = s.storage.Delete(ctx, storagecap.DeleteInput{Path: objectPath})
		return nil, err
	}
	return &DownloadTicket{Ticket: ticket, FileName: fileName, ExpiresAt: expiresAt}, nil
}

func (s *service) requireIssued(ctx context.Context, frozen *frozenEntitlement, declaration *productfulfillmentcap.ProviderDeclaration) error {
	action := actionByKey(declaration, "sync")
	if action == nil || action.Execution != productfulfillmentcap.ServiceActionExecutionProjection {
		return bizerr.NewCode(CodeUnavailable)
	}
	runtime, err := s.contexts.IssueExistingBusinessRuntimeContext(ctx, productautomation.ExistingBusinessRuntimeContextInput{PluginID: frozen.PluginID, ReleaseVersion: frozen.ReleaseVersion, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: frozen.ContractVersion, ReleaseDigest: frozen.ReleaseDigest, DeclarationDigest: frozen.DeclarationDigest, Method: action.Method, InterfaceInstanceNo: frozen.InterfaceInstanceNo, InterfaceConfigurationRevision: frozen.InterfaceConfigurationRevision, IntentNo: frozen.SourceIntentNo, OrderNo: frozen.OrderNo, OrderItemNo: frozen.OrderItemNo, ConfigurationSnapshotNo: frozen.ConfigurationSnapshotNo})
	if err != nil || runtime == nil || runtime.Release == nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	defer runtime.Release()
	request := &productfulfillmentcap.ManagedServiceActionRequest{Version: productfulfillmentcap.ManagedServiceActionVersion, ActionKey: action.Key, EntitlementNo: frozen.EntitlementNo, ExternalReference: frozen.ExternalReference, ProductConfiguration: frozen.ProductConfiguration}
	payload, err := json.Marshal(request)
	if err != nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	host, err := s.extensions.InvokeProviderMethod(ctx, pluginhost.BusinessExtensionMethodInvokeInput{PluginID: frozen.PluginID, Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: frozen.ContractVersion, Method: action.Method, ReleaseVersion: frozen.ReleaseVersion, ReleaseDigest: frozen.ReleaseDigest, DeclarationDigest: frozen.DeclarationDigest, Purpose: capmodel.BusinessExtensionInvocationPurposeExistingBusiness, RuntimeContext: &capmodel.BusinessExtensionRuntimeContext{ContextID: runtime.ContextID}, Payload: payload})
	if err != nil || host == nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	var result *productfulfillmentcap.ManagedServiceActionResult
	if json.Unmarshal(host.Payload, &result) != nil || productfulfillmentcap.ValidateManagedServiceActionResult(result, action.Key, true) != nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	var projection struct {
		UpstreamStatus string `json:"upstreamStatus"`
	}
	if json.Unmarshal(result.Projection, &projection) != nil || strings.ToLower(strings.TrimSpace(projection.UpstreamStatus)) != "issued" && strings.ToLower(strings.TrimSpace(projection.UpstreamStatus)) != "active" {
		return bizerr.NewCode(CodeNotReady)
	}
	return nil
}

func (s *service) RedeemDownload(ctx context.Context, ticket string, actor Actor) (*DownloadContent, error) {
	if s == nil || s.storage == nil || validateActor(actor) != nil || !safeReferencePattern.MatchString(ticket) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var row *struct{ EntitlementNo, CustomerNo, ObjectPath, ObjectDigest, FileName, ContentType string }
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		if err := s.database.GetScan(txCtx, &row, `SELECT entitlement_no,customer_no,object_path,object_digest,file_name,content_type FROM plugin_idc_core_service_download_ticket WHERE tenant_id=0 AND ticket_digest=? AND audience_code=? AND actor_user_id=? AND consumed_at IS NULL AND expires_at>CURRENT_TIMESTAMP AND (?='admin' OR customer_no=?) LIMIT 1 FOR UPDATE`, digest([]byte(ticket)), actor.Audience, actor.UserID, actor.Audience, actor.CustomerNo); err != nil || row == nil {
			return errors.New("service download is unavailable")
		}
		var ownerCount int
		if err := s.database.GetScan(txCtx, &ownerCount, `SELECT count(*)::int FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=? AND (?='admin' OR customer_no=?)`, row.EntitlementNo, actor.Audience, actor.CustomerNo); err != nil || ownerCount != 1 {
			return errors.New("service download is unavailable")
		}
		result, err := s.database.Exec(txCtx, `UPDATE plugin_idc_core_service_download_ticket SET consumed_at=CURRENT_TIMESTAMP WHERE ticket_digest=? AND consumed_at IS NULL`, digest([]byte(ticket)))
		if err != nil {
			return err
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			return errors.New("service download is unavailable")
		}
		return nil
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	object, err := s.storage.Get(ctx, storagecap.GetInput{Path: row.ObjectPath})
	if err != nil || object == nil || !object.Found || object.Body == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	defer object.Body.Close()
	content, err := io.ReadAll(io.LimitReader(object.Body, 8<<20+1))
	if err != nil || len(content) == 0 || len(content) > 8<<20 || digest(content) != row.ObjectDigest {
		clear(content)
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	_ = s.storage.Delete(ctx, storagecap.DeleteInput{Path: row.ObjectPath})
	return &DownloadContent{FileName: row.FileName, ContentType: row.ContentType, Content: content}, nil
}

func (s *service) resolveProvider(ctx context.Context, entitlementNo string) (*frozenEntitlement, *productfulfillmentcap.ProviderDeclaration, error) {
	row, declaration, _, err := s.resolve(ctx, entitlementNo, "")
	return row, declaration, err
}

func sanitizeCertificateArchive(source []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	if err != nil || len(reader.File) < 1 || len(reader.File) > 32 {
		return nil, gerror.New("supplier certificate archive is invalid")
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	seen := map[string]struct{}{}
	for _, file := range reader.File {
		name := path.Base(strings.ReplaceAll(file.Name, "\\", "/"))
		lower := strings.ToLower(name)
		if file.FileInfo().IsDir() {
			continue
		}
		if name == "." || name == "" || strings.Contains(lower, "private") || strings.HasSuffix(lower, ".key") || strings.HasSuffix(lower, ".pfx") || strings.HasSuffix(lower, ".p12") || file.UncompressedSize64 > 2<<20 {
			return nil, gerror.New("supplier archive contains forbidden private-key material")
		}
		if !strings.HasSuffix(lower, ".crt") && !strings.HasSuffix(lower, ".cer") && !strings.HasSuffix(lower, ".pem") {
			continue
		}
		if _, duplicate := seen[lower]; duplicate {
			return nil, gerror.New("supplier certificate archive is ambiguous")
		}
		opened, openErr := file.Open()
		if openErr != nil {
			return nil, gerror.New("supplier certificate archive is invalid")
		}
		content, readErr := io.ReadAll(io.LimitReader(opened, 2<<20+1))
		_ = opened.Close()
		if readErr != nil || len(content) == 0 || len(content) > 2<<20 {
			clear(content)
			return nil, gerror.New("supplier archive contains forbidden certificate material")
		}
		publicCertificates, parseErr := publicCertificatePEM(content)
		clear(content)
		if parseErr != nil {
			return nil, gerror.New("supplier archive contains forbidden certificate material")
		}
		entry, createErr := writer.Create(name)
		if createErr != nil {
			clear(publicCertificates)
			return nil, createErr
		}
		_, createErr = entry.Write(publicCertificates)
		clear(publicCertificates)
		if createErr != nil {
			return nil, createErr
		}
		seen[lower] = struct{}{}
	}
	if len(seen) == 0 {
		return nil, gerror.New("supplier certificate archive contains no public certificate")
	}
	if err = writer.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// publicCertificatePEM parses certificate data rather than trusting file names
// or PEM marker strings. It emits only normalized public certificates, so
// private keys, unknown PEM blocks and trailing binary data cannot be smuggled
// into the customer download archive.
func publicCertificatePEM(source []byte) ([]byte, error) {
	remaining := bytes.TrimSpace(source)
	if len(remaining) == 0 {
		return nil, errors.New("certificate is empty")
	}
	var certificates [][]byte
	if bytes.HasPrefix(remaining, []byte("-----BEGIN")) {
		for len(remaining) > 0 {
			block, rest := pem.Decode(remaining)
			if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
				return nil, errors.New("certificate PEM is invalid")
			}
			if _, err := x509.ParseCertificate(block.Bytes); err != nil {
				return nil, err
			}
			certificates = append(certificates, bytes.Clone(block.Bytes))
			remaining = bytes.TrimSpace(rest)
		}
	} else {
		// DER is binary: a signature may legitimately end in a whitespace byte.
		// Parse the exact original bytes so trailing material is also rejected.
		if _, err := x509.ParseCertificate(source); err != nil {
			return nil, err
		}
		certificates = append(certificates, bytes.Clone(source))
	}
	var output bytes.Buffer
	for _, certificate := range certificates {
		if err := pem.Encode(&output, &pem.Block{Type: "CERTIFICATE", Bytes: certificate}); err != nil {
			return nil, err
		}
		clear(certificate)
	}
	return output.Bytes(), nil
}
