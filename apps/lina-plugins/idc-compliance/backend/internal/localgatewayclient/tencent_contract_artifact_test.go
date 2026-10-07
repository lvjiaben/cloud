package localgatewayclient

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"lina-platform/idc-compliance-gateway/pkg/processauth"
	"lina-plugin-idc-compliance/backend/internal/localgatewayauth"
)

func TestTencentContractArtifactClientVerifiesPDFAndPendingState(t *testing.T) {
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := localgatewayauth.New(file, clientTestInstallationID, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	reportKey, err := processauth.GenerateBusinessIdempotencyKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.7\nverified archive")
	digest := sha256.Sum256(pdf)
	observedUnix := int64(1800000000)
	calls := 0
	httpClient := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.Method != http.MethodPost || request.Header.Get(processauth.HeaderSignature) == "" {
			t.Fatalf("artifact request was not signed: %+v", request)
		}
		if request.URL.RequestURI() == processauth.TencentContractFinalPDFsPath {
			header := make(http.Header)
			header.Set("Content-Type", "application/pdf")
			header.Set(processauth.TencentArtifactStateHeader, processauth.TencentArtifactStateReady)
			header.Set(processauth.TencentArtifactKindHeader, processauth.TencentArtifactKindFinalPDF)
			header.Set(processauth.TencentArtifactSHA256Header, hex.EncodeToString(digest[:]))
			header.Set(processauth.TencentArtifactObservedUnixHeader, "1800000000")
			return &http.Response{
				StatusCode: http.StatusOK, Header: header, ContentLength: int64(len(pdf)),
				Body: io.NopCloser(strings.NewReader(string(pdf))),
			}, nil
		}
		if request.URL.RequestURI() != processauth.TencentContractSignReportsPath ||
			request.Header.Get(processauth.HeaderBusinessIdempotency) != reportKey {
			t.Fatalf("unexpected report request path=%s businessKey=%s",
				request.URL.RequestURI(), request.Header.Get(processauth.HeaderBusinessIdempotency))
		}
		header := make(http.Header)
		header.Set("Content-Type", "application/json; charset=utf-8")
		return &http.Response{
			StatusCode: http.StatusAccepted, Header: header,
			Body: io.NopCloser(strings.NewReader(
				`{"state":"pending","code":"report_executing","nextCheckUnix":1800003600}`,
			)),
		}, nil
	})}
	client, err := New(defaultBaseURL, signer, httpClient)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	artifact, err := client.FetchTencentContractFinalPDF(
		t.Context(), "lwr_0123456789abcdef0123456789abcdef",
	)
	if err != nil || artifact == nil || string(artifact.Body) != string(pdf) ||
		artifact.SHA256 != hex.EncodeToString(digest[:]) ||
		artifact.SupplierObservedAt != time.Unix(observedUnix, 0).UTC() {
		t.Fatalf("unexpected final PDF artifact=%+v err=%v", artifact, err)
	}
	report, state, err := client.EnsureTencentContractSignReport(
		t.Context(), "lwr_0123456789abcdef0123456789abcdef", reportKey,
	)
	if err != nil || report != nil || state.State != "pending" ||
		state.Code != "report_executing" || state.NextCheckAt.Unix() != 1800003600 || calls != 2 {
		t.Fatalf("unexpected report=%+v state=%+v calls=%d err=%v", report, state, calls, err)
	}
}

func TestTencentContractArtifactClientRejectsChecksumMismatch(t *testing.T) {
	file, err := processauth.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := localgatewayauth.New(file, clientTestInstallationID, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.7\nwrong digest")
	httpClient := &http.Client{Timeout: time.Second, Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		header := make(http.Header)
		header.Set("Content-Type", "application/pdf")
		header.Set(processauth.TencentArtifactStateHeader, processauth.TencentArtifactStateReady)
		header.Set(processauth.TencentArtifactKindHeader, processauth.TencentArtifactKindFinalPDF)
		header.Set(processauth.TencentArtifactSHA256Header, strings.Repeat("0", 64))
		header.Set(processauth.TencentArtifactObservedUnixHeader, "1800000000")
		return &http.Response{
			StatusCode: http.StatusOK, Header: header, ContentLength: int64(len(pdf)),
			Body: io.NopCloser(strings.NewReader(string(pdf))),
		}, nil
	})}
	client, err := New(defaultBaseURL, signer, httpClient)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	artifact, err := client.FetchTencentContractFinalPDF(
		t.Context(), "lwr_0123456789abcdef0123456789abcdef",
	)
	if err == nil || artifact != nil {
		t.Fatalf("checksum mismatch was accepted: artifact=%+v err=%v", artifact, err)
	}
}
