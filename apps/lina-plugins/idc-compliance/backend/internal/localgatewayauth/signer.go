// Package localgatewayauth owns the source plugin side of installation-local
// process authentication. It never registers with or contacts a vendor service.
package localgatewayauth

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
)

const (
	// KeyringFileEnvironment is shared with the colocated gateway process.
	KeyringFileEnvironment = "LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_KEYRING_FILE"
	// InstallationIDFileEnvironment names the installation-private identifier file.
	InstallationIDFileEnvironment = "LINAPRO_COMPLIANCE_GATEWAY_INSTALLATION_ID_FILE"
	// AuthenticationWindowEnvironment is the shared bounded signature lifetime.
	AuthenticationWindowEnvironment = "LINAPRO_COMPLIANCE_GATEWAY_PROCESS_AUTH_WINDOW"
	maximumPathBytes                = 4096
)

// Signer signs exact HTTP requests with the current installation's active key.
// A signer is immutable and safe for concurrent request signing.
type Signer struct {
	ring           *processauth.Ring
	installationID string
	validity       time.Duration
	now            func() time.Time
}

// Load reads the installation identifier and keyring from distinct private files.
func Load(
	keyringPath string,
	installationIDPath string,
	validity time.Duration,
) (*Signer, error) {
	if filepath.Clean(strings.TrimSpace(keyringPath)) ==
		filepath.Clean(strings.TrimSpace(installationIDPath)) {
		return nil, fmt.Errorf("local gateway keyring and installation id files must be distinct")
	}
	keyringContent, err := readPrivateFile(keyringPath, processauth.MaximumKeyringSize)
	if err != nil {
		return nil, fmt.Errorf("read local gateway process-auth keyring: %w", err)
	}
	defer clearBytes(keyringContent)
	keyring, err := processauth.Parse(keyringContent)
	if err != nil {
		return nil, fmt.Errorf("parse local gateway process-auth keyring: %w", err)
	}
	installationContent, err := readPrivateFile(
		installationIDPath, processauth.MaximumInstallationIDFileSize,
	)
	if err != nil {
		return nil, fmt.Errorf("read local gateway installation id: %w", err)
	}
	defer clearBytes(installationContent)
	installationID, err := processauth.ParseInstallationID(installationContent)
	if err != nil {
		return nil, fmt.Errorf("parse local gateway installation id: %w", err)
	}
	return New(keyring, installationID, validity)
}

// LoadFromEnvironment resolves the same keyring file configured for the local
// gateway. The caller decides when to reload after an atomic rotation.
func LoadFromEnvironment(getenv func(string) string) (*Signer, error) {
	if getenv == nil {
		return nil, fmt.Errorf("environment lookup is required")
	}
	validity, err := time.ParseDuration(strings.TrimSpace(getenv(AuthenticationWindowEnvironment)))
	if err != nil {
		return nil, fmt.Errorf("local gateway process-auth window is invalid")
	}
	return Load(
		getenv(KeyringFileEnvironment), getenv(InstallationIDFileEnvironment), validity,
	)
}

// New constructs a signer from a validated keyring and copies live secrets.
func New(
	file processauth.File,
	installationID string,
	validity time.Duration,
) (*Signer, error) {
	parsedInstallationID, err := processauth.ParseInstallationID([]byte(installationID))
	if err != nil || parsedInstallationID != installationID || validity <= 0 ||
		validity > processauth.MaximumAuthenticationWindow {
		return nil, fmt.Errorf("local gateway process-auth signer configuration is invalid")
	}
	ring, err := processauth.New(file)
	if err != nil {
		return nil, fmt.Errorf("construct local gateway process-auth signer: %w", err)
	}
	return &Signer{
		ring: ring, installationID: installationID, validity: validity, now: time.Now,
	}, nil
}

// Sign adds only short-lived authentication headers for the exact request
// target and server-declared payload mode.
func (s *Signer) Sign(
	request *http.Request,
	operationScope string,
	payloadMode processauth.PayloadMode,
	body []byte,
	businessIdempotencyKey string,
) error {
	if s == nil || s.ring == nil || s.now == nil || request == nil || request.URL == nil {
		return fmt.Errorf("local gateway request signer is unavailable")
	}
	credentials, err := s.ring.Sign(
		s.installationID, operationScope, request.Method, request.URL.RequestURI(),
		payloadMode, body, businessIdempotencyKey, s.now().UTC(), s.validity, rand.Reader,
	)
	if err != nil {
		return fmt.Errorf("sign local gateway request: %w", err)
	}
	request.Header.Set(processauth.HeaderProtocol, credentials.Protocol)
	request.Header.Set(processauth.HeaderInstallationID, credentials.InstallationID)
	request.Header.Set(processauth.HeaderOperationScope, credentials.OperationScope)
	request.Header.Set(processauth.HeaderKeyID, credentials.KeyID)
	request.Header.Set(processauth.HeaderTimestampMS, fmt.Sprintf("%d", credentials.TimestampMS))
	request.Header.Set(processauth.HeaderExpiresMS, fmt.Sprintf("%d", credentials.ExpiresMS))
	request.Header.Set(processauth.HeaderNonce, credentials.Nonce)
	request.Header.Set(
		processauth.HeaderBusinessIdempotency, credentials.BusinessIdempotencyKey,
	)
	request.Header.Set(processauth.HeaderContentSHA, credentials.ContentSHA256)
	request.Header.Set(
		processauth.HeaderBusinessContentSHA, credentials.BusinessContentSHA256,
	)
	request.Header.Set(processauth.HeaderSignature, credentials.Signature)
	return nil
}

// SignInstallationRead signs the only currently enabled business route with a
// fresh read correlation key and a server-fixed operation scope.
func (s *Signer) SignInstallationRead(request *http.Request) error {
	if request == nil || request.URL == nil || request.Method != http.MethodGet ||
		request.URL.RequestURI() != "/internal/v1/installation" {
		return fmt.Errorf("installation-read signer requires the exact local route")
	}
	businessKey, err := processauth.GenerateBusinessIdempotencyKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate installation-read correlation key: %w", err)
	}
	return s.Sign(
		request, "installation.read", processauth.PayloadModeEmpty, nil, businessKey,
	)
}

// SignTencentOnboardingRead signs the exact safe activation-status route.
func (s *Signer) SignTencentOnboardingRead(request *http.Request) error {
	if request == nil || request.URL == nil || request.Method != http.MethodGet ||
		request.URL.RequestURI() != processauth.TencentOnboardingPath {
		return fmt.Errorf("tencent onboarding read signer requires the exact local route")
	}
	businessKey, err := processauth.GenerateBusinessIdempotencyKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate Tencent onboarding read correlation key: %w", err)
	}
	return s.Sign(
		request, processauth.TencentOnboardingReadScope,
		processauth.PayloadModeEmpty, nil, businessKey,
	)
}

// SignTencentTemplatesRead signs the exact explicit template-sync route.
func (s *Signer) SignTencentTemplatesRead(request *http.Request) error {
	if request == nil || request.URL == nil || request.Method != http.MethodGet ||
		request.URL.RequestURI() != processauth.TencentTemplatesPath {
		return fmt.Errorf("tencent templates read signer requires the exact local route")
	}
	businessKey, err := processauth.GenerateBusinessIdempotencyKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate Tencent templates read correlation key: %w", err)
	}
	return s.Sign(
		request, processauth.TencentTemplatesReadScope,
		processauth.PayloadModeEmpty, nil, businessKey,
	)
}

// SignTencentTemplatePreviewRead signs one exact server-owned template preview request.
func (s *Signer) SignTencentTemplatePreviewRead(request *http.Request, body []byte) error {
	if request == nil || request.URL == nil || request.Method != http.MethodPost || len(body) == 0 ||
		request.URL.RequestURI() != processauth.TencentTemplatePreviewsPath {
		return fmt.Errorf("tencent template preview signer requires the exact local route")
	}
	businessKey, err := processauth.GenerateBusinessIdempotencyKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate Tencent template preview correlation key: %w", err)
	}
	return s.Sign(
		request, processauth.TencentTemplatePreviewReadScope,
		processauth.PayloadModeJSON, body, businessKey,
	)
}

// SignTencentContractFlowCreate signs one exact guarded contract-flow write.
func (s *Signer) SignTencentContractFlowCreate(
	request *http.Request,
	body []byte,
	businessIdempotencyKey string,
) error {
	if request == nil || request.URL == nil || request.Method != http.MethodPost || len(body) == 0 ||
		request.URL.RequestURI() != processauth.TencentContractFlowsPath {
		return fmt.Errorf("tencent contract flow signer requires the exact local route")
	}
	return s.Sign(
		request, processauth.TencentContractFlowCreateScope,
		processauth.PayloadModeJSON, body, businessIdempotencyKey,
	)
}

// SignTencentContractTerminationCreate signs one exact guarded release-agreement write.
func (s *Signer) SignTencentContractTerminationCreate(
	request *http.Request,
	body []byte,
	businessIdempotencyKey string,
) error {
	if request == nil || request.URL == nil || request.Method != http.MethodPost || len(body) == 0 ||
		request.URL.RequestURI() != processauth.TencentContractTerminationsPath {
		return fmt.Errorf("tencent contract termination signer requires the exact local route")
	}
	return s.Sign(
		request, processauth.TencentContractTerminationCreateScope,
		processauth.PayloadModeJSON, body, businessIdempotencyKey,
	)
}

// SignTencentContractSigningLinkCreate signs one fresh link request for an existing flow.
func (s *Signer) SignTencentContractSigningLinkCreate(request *http.Request, body []byte) error {
	return s.signTencentContractRead(
		request, body, processauth.TencentContractSigningLinksPath,
		processauth.TencentContractSigningLinkCreateScope,
	)
}

// SignTencentContractFlowRecheck signs one authoritative read-only flow query.
func (s *Signer) SignTencentContractFlowRecheck(request *http.Request, body []byte) error {
	return s.signTencentContractRead(
		request, body, processauth.TencentContractFlowRechecksPath,
		processauth.TencentContractFlowRecheckScope,
	)
}

// SignTencentContractDeliveryPage signs one bounded pending-delivery read.
func (s *Signer) SignTencentContractDeliveryPage(request *http.Request) error {
	if request == nil || request.URL == nil || request.Method != http.MethodGet ||
		request.URL.Path != processauth.TencentContractDeliveryPagesPath || request.URL.RawQuery == "" {
		return fmt.Errorf("tencent contract delivery page signer requires the exact local route")
	}
	businessKey, err := processauth.GenerateBusinessIdempotencyKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate Tencent contract delivery page correlation key: %w", err)
	}
	return s.Sign(
		request, processauth.TencentContractDeliveryReadScope,
		processauth.PayloadModeEmpty, nil, businessKey,
	)
}

// SignTencentContractDeliveryAcknowledgement signs one idempotent acknowledgement batch.
func (s *Signer) SignTencentContractDeliveryAcknowledgement(
	request *http.Request,
	body []byte,
	businessIdempotencyKey string,
) error {
	if request == nil || request.URL == nil || request.Method != http.MethodPost || len(body) == 0 ||
		request.URL.RequestURI() != processauth.TencentContractDeliveryAcknowledgementsPath {
		return fmt.Errorf("tencent contract delivery acknowledgement signer requires the exact local route")
	}
	return s.Sign(
		request, processauth.TencentContractDeliveryAcknowledgeScope,
		processauth.PayloadModeJSON, body, businessIdempotencyKey,
	)
}

// SignTencentContractFinalPDFRead signs one completed-flow PDF archive read.
func (s *Signer) SignTencentContractFinalPDFRead(request *http.Request, body []byte) error {
	return s.signTencentContractRead(
		request, body, processauth.TencentContractFinalPDFsPath,
		processauth.TencentContractFinalPDFReadScope,
	)
}

// SignTencentContractSignReport signs one idempotent paid-report ensure request.
func (s *Signer) SignTencentContractSignReport(
	request *http.Request,
	body []byte,
	businessIdempotencyKey string,
) error {
	if request == nil || request.URL == nil || request.Method != http.MethodPost || len(body) == 0 ||
		request.URL.RequestURI() != processauth.TencentContractSignReportsPath {
		return fmt.Errorf("tencent contract sign-report signer requires the exact local route")
	}
	return s.Sign(
		request, processauth.TencentContractSignReportScope,
		processauth.PayloadModeJSON, body, businessIdempotencyKey,
	)
}

func (s *Signer) signTencentContractRead(
	request *http.Request,
	body []byte,
	path string,
	scope string,
) error {
	if request == nil || request.URL == nil || request.Method != http.MethodPost || len(body) == 0 ||
		request.URL.RequestURI() != path {
		return fmt.Errorf("tencent contract read signer requires the exact local route")
	}
	businessKey, err := processauth.GenerateBusinessIdempotencyKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("generate Tencent contract read correlation key: %w", err)
	}
	return s.Sign(request, scope, processauth.PayloadModeJSON, body, businessKey)
}

// SignTencentOnboardingLink signs one official-link creation request.
func (s *Signer) SignTencentOnboardingLink(
	request *http.Request,
	body []byte,
	businessIdempotencyKey string,
) error {
	if request == nil || request.URL == nil || request.Method != http.MethodPost || len(body) == 0 ||
		request.URL.RequestURI() != processauth.TencentOnboardingLinksPath {
		return fmt.Errorf("tencent onboarding link signer requires the exact local route")
	}
	return s.Sign(
		request, processauth.TencentOnboardingLinkScope,
		processauth.PayloadModeJSON, body, businessIdempotencyKey,
	)
}

// SignTencentOnboardingRecheck signs one explicit authoritative recheck.
func (s *Signer) SignTencentOnboardingRecheck(
	request *http.Request,
	body []byte,
	businessIdempotencyKey string,
) error {
	if request == nil || request.URL == nil || request.Method != http.MethodPost || len(body) == 0 ||
		request.URL.RequestURI() != processauth.TencentOnboardingRechecksPath {
		return fmt.Errorf("tencent onboarding recheck signer requires the exact local route")
	}
	return s.Sign(
		request, processauth.TencentOnboardingRecheckScope,
		processauth.PayloadModeJSON, body, businessIdempotencyKey,
	)
}

// SignSupplierSubjectWrite signs one exact internal supplier-subject route.
func (s *Signer) SignSupplierSubjectWrite(
	request *http.Request,
	body []byte,
	businessIdempotencyKey string,
) error {
	if request == nil || request.URL == nil || request.Method != http.MethodPost || len(body) == 0 {
		return fmt.Errorf("supplier-subject signer requires a JSON POST request")
	}
	var scope string
	switch request.URL.RequestURI() {
	case processauth.SupplierSubjectBindingsPath:
		scope = processauth.SupplierSubjectBindScope
	case processauth.SupplierSubjectRevocationsPath:
		scope = processauth.SupplierSubjectRevokeScope
	case processauth.SupplierSubjectReopeningsPath:
		scope = processauth.SupplierSubjectReopenScope
	default:
		return fmt.Errorf("supplier-subject signer requires an exact local route")
	}
	return s.Sign(request, scope, processauth.PayloadModeJSON, body, businessIdempotencyKey)
}

// Close clears the signer's copied secret bytes. A closed signer rejects use.
func (s *Signer) Close() {
	if s == nil || s.ring == nil {
		return
	}
	s.ring.Clear()
	s.ring = nil
}

// clearBytes removes copied secret material from the supplied slice.
func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

// readPrivateFile reads one bounded regular file after checking its opened handle.
func readPrivateFile(path string, maximum int64) (content []byte, returnError error) {
	path = strings.TrimSpace(path)
	if path == "" || len(path) > maximumPathBytes || !filepath.IsAbs(path) ||
		strings.ContainsAny(path, "\r\n\x00") {
		return nil, fmt.Errorf("private configuration path is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open private configuration file: %w", err)
	}
	defer func() {
		if closeError := file.Close(); closeError != nil && returnError == nil {
			clearBytes(content)
			content = nil
			returnError = fmt.Errorf("close private configuration file: %w", closeError)
		}
	}()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("inspect private configuration file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("private configuration file grants group or other access")
	}
	content, err = io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil {
		clearBytes(content)
		return nil, fmt.Errorf("read private configuration file: %w", err)
	}
	if int64(len(content)) > maximum {
		clearBytes(content)
		return nil, fmt.Errorf("private configuration file is too large")
	}
	return content, nil
}
