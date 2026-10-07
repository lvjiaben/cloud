package purchaseconfigurationfile

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/storagecap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const purchaseFileIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

type purchaseFileAudienceKey struct{}

type purchaseFilePrincipal struct{}

func (purchaseFilePrincipal) Current(ctx context.Context) (*principal.Audience, error) {
	audience, _ := ctx.Value(purchaseFileAudienceKey{}).(*principal.Audience)
	return audience, nil
}

type purchaseFileResolver struct{ target *Target }

func (r purchaseFileResolver) ResolvePurchaseFileTarget(context.Context, string, string, string) (*Target, error) {
	return r.target, nil
}

type purchaseFileScanner struct {
	ready   bool
	verdict string
}

func (s *purchaseFileScanner) Ready() bool { return s.ready }

func (s *purchaseFileScanner) Scan(context.Context, io.Reader, int64) (string, error) {
	return s.verdict, nil
}

type purchaseFileStorage struct {
	storagecap.Service
	mu      sync.Mutex
	objects map[string][]byte
}

func (s *purchaseFileStorage) Put(_ context.Context, input storagecap.PutInput) (*storagecap.PutOutput, error) {
	content, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.objects == nil {
		s.objects = make(map[string][]byte)
	}
	if _, exists := s.objects[input.Path]; exists && !input.Overwrite {
		return nil, errors.New("object already exists")
	}
	s.objects[input.Path] = append([]byte(nil), content...)
	return &storagecap.PutOutput{Object: &storagecap.Object{Path: input.Path, Size: int64(len(content)), ContentType: input.ContentType, Visibility: storagecap.VisibilityPrivate}}, nil
}

func (s *purchaseFileStorage) Delete(_ context.Context, input storagecap.DeleteInput) error {
	s.mu.Lock()
	delete(s.objects, input.Path)
	s.mu.Unlock()
	return nil
}

func TestPurchaseConfigurationFilePostgresOwnershipPolicyAndConcurrentUpload(t *testing.T) {
	ctx := context.Background()
	database := configurePurchaseFileIntegrationDB(t, ctx)
	pdf := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF")
	policy := &capmodel.ConfigurationUploadPolicyDeclaration{
		ID: "customer.identity.v1", MaximumFiles: 2,
		MaximumFileBytes: int64(len(pdf)), MaximumTotalBytes: int64(len(pdf) * 2),
		AllowedMIMETypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"},
	}
	target := &Target{ProductVersionNo: "PV_FILE_001", ConfigurationRevision: "cfg-rev-1", FieldKey: "identityDocument", Policy: policy}
	storage := &purchaseFileStorage{}
	scanner := &purchaseFileScanner{ready: true, verdict: "clean"}
	service, err := New(database, purchaseFilePrincipal{}, storage, purchaseFileResolver{target: target}, scanner)
	if err != nil {
		t.Fatal(err)
	}
	service.(*serviceImpl).random = bytes.NewReader(bytes.Repeat([]byte{0x31}, 128))
	owner := context.WithValue(ctx, purchaseFileAudienceKey{}, activePurchaseFileAudience(701, "CUS_FILES"))
	sameCustomerOtherLogin := context.WithValue(ctx, purchaseFileAudienceKey{}, activePurchaseFileAudience(702, "CUS_FILES"))

	first, err := service.Reserve(owner, ReserveInput{PublicSlug: "phone", ConfigurationRevision: "public-rev", FieldToken: "field-token", FileName: "identity.pdf", SizeBytes: int64(len(pdf))})
	if err != nil {
		t.Fatalf("reserve first: %v", err)
	}
	service.(*serviceImpl).random = bytes.NewReader(bytes.Repeat([]byte{0x32}, 128))
	second, err := service.Reserve(owner, ReserveInput{PublicSlug: "phone", ConfigurationRevision: "public-rev", FieldToken: "field-token", FileName: "identity-2.pdf", SizeBytes: int64(len(pdf))})
	if err != nil {
		t.Fatalf("reserve second: %v", err)
	}
	service.(*serviceImpl).random = bytes.NewReader(bytes.Repeat([]byte{0x33}, 128))
	if third, reserveErr := service.Reserve(owner, ReserveInput{PublicSlug: "phone", ConfigurationRevision: "public-rev", FieldToken: "field-token", FileName: "identity-3.pdf", SizeBytes: int64(len(pdf))}); third != nil || !bizerr.Is(reserveErr, CodeInvalid) {
		t.Fatalf("quota reserve=%+v err=%v", third, reserveErr)
	}

	item, err := service.Upload(owner, first.UploadToken, "application/pdf", bytes.NewReader(pdf))
	if err != nil || item.Status != StatusReady || len(storage.objects) != 1 {
		t.Fatalf("upload item=%+v objects=%d err=%v", item, len(storage.objects), err)
	}
	remaining := time.Until(item.ExpiresAt)
	if remaining < readyTokenLifetime-time.Minute || remaining > readyTokenLifetime+time.Minute {
		t.Fatalf("ready token lifetime=%s want seven days", remaining)
	}
	if foreign, foreignErr := service.Get(sameCustomerOtherLogin, first.UploadToken); foreign != nil || !bizerr.Is(foreignErr, CodeNotAccessible) {
		t.Fatalf("cross-login read=%+v err=%v", foreign, foreignErr)
	}
	verified, err := service.VerifyBatch(owner, VerifyInput{Target: target, FileTokens: []string{first.UploadToken}})
	if err != nil || len(verified) != 1 || verified[0].ObjectRef == "" || verified[0].SHA256Hex == "" {
		t.Fatalf("verify=%+v err=%v", verified, err)
	}
	if foreign, foreignErr := service.VerifyBatch(sameCustomerOtherLogin, VerifyInput{Target: target, FileTokens: []string{first.UploadToken}}); foreign != nil || !bizerr.Is(foreignErr, CodeNotAccessible) {
		t.Fatalf("cross-login verify=%+v err=%v", foreign, foreignErr)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, uploadErr := service.Upload(owner, second.UploadToken, "application/pdf", bytes.NewReader(pdf))
			results <- uploadErr
		}()
	}
	close(start)
	var successes int
	for range 2 {
		if uploadErr := <-results; uploadErr == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent uploads succeeded %d times, want exactly one", successes)
	}
	if len(storage.objects) != 2 {
		t.Fatalf("private object count=%d want=2", len(storage.objects))
	}
	consumed, err := service.ConsumeForOrder(owner, "ORD_FILES_001", []VerifyInput{{Target: target, FileTokens: []string{first.UploadToken}}})
	if err != nil || len(consumed) != 1 || len(consumed[0].Files) != 1 {
		t.Fatalf("consume=%+v err=%v", consumed, err)
	}
	if replayed, replayErr := service.ConsumeForOrder(owner, "ORD_FILES_001", []VerifyInput{{Target: target, FileTokens: []string{first.UploadToken}}}); replayErr != nil || len(replayed) != 1 {
		t.Fatalf("same-order consume replay=%+v err=%v", replayed, replayErr)
	}
	if reused, reuseErr := service.ConsumeForOrder(owner, "ORD_FILES_002", []VerifyInput{{Target: target, FileTokens: []string{first.UploadToken}}}); reused != nil || !bizerr.Is(reuseErr, CodeNotAccessible) {
		t.Fatalf("cross-order consume=%+v err=%v", reused, reuseErr)
	}
	orderOwner := OrderEvidenceOwner{OrderNo: "ORD_FILES_001", CustomerNo: "CUS_FILES", UploaderUserID: 701}
	if evidence, evidenceErr := service.VerifyOrderEvidence(ctx, orderOwner, []VerifyInput{{Target: target, FileTokens: []string{first.UploadToken}}}); evidenceErr != nil || len(evidence) != 1 {
		t.Fatalf("same-order permanent evidence=%+v err=%v", evidence, evidenceErr)
	}
	orderOwner.OrderNo = "ORD_FILES_002"
	if evidence, evidenceErr := service.VerifyOrderEvidence(ctx, orderOwner, []VerifyInput{{Target: target, FileTokens: []string{first.UploadToken}}}); evidence != nil || !bizerr.Is(evidenceErr, CodeNotAccessible) {
		t.Fatalf("cross-order permanent evidence=%+v err=%v", evidence, evidenceErr)
	}
	if revoked, revokeErr := service.Revoke(owner, first.UploadToken); revoked != nil || !bizerr.Is(revokeErr, CodeConflict) {
		t.Fatalf("consumed evidence was revocable: item=%+v err=%v", revoked, revokeErr)
	}
	if reusable, verifyErr := service.VerifyBatch(owner, VerifyInput{Target: target, FileTokens: []string{first.UploadToken}}); reusable != nil || !bizerr.Is(verifyErr, CodeNotAccessible) {
		t.Fatalf("consumed token remained reusable=%+v err=%v", reusable, verifyErr)
	}
	revoked, revokeErr := service.Revoke(owner, second.UploadToken)
	if revokeErr != nil || revoked == nil || revoked.Status != StatusRevoked {
		t.Fatalf("revoke ready token item=%+v err=%v", revoked, revokeErr)
	}
	if replay, replayErr := service.Revoke(owner, second.UploadToken); replayErr != nil || replay == nil || replay.Status != StatusRevoked {
		t.Fatalf("revoke replay item=%+v err=%v", replay, replayErr)
	}
	if foreign, foreignErr := service.Revoke(sameCustomerOtherLogin, second.UploadToken); foreign != nil || !bizerr.Is(foreignErr, CodeNotAccessible) {
		t.Fatalf("cross-login revoke item=%+v err=%v", foreign, foreignErr)
	}
	if reusable, verifyErr := service.VerifyBatch(owner, VerifyInput{Target: target, FileTokens: []string{second.UploadToken}}); reusable != nil || !bizerr.Is(verifyErr, CodeNotAccessible) {
		t.Fatalf("revoked token remained reusable=%+v err=%v", reusable, verifyErr)
	}
}

func TestPurchaseConfigurationFilePostgresScanFailClosedAndExactPolicy(t *testing.T) {
	ctx := context.Background()
	database := configurePurchaseFileIntegrationDB(t, ctx)
	pdf := []byte("%PDF-1.4\n%%EOF")
	policy := &capmodel.ConfigurationUploadPolicyDeclaration{
		ID: "customer.scan.v1", MaximumFiles: 1,
		MaximumFileBytes: int64(len(pdf)), MaximumTotalBytes: int64(len(pdf)),
		AllowedMIMETypes: []string{"application/pdf"}, AllowedExtensions: []string{".pdf"}, SafetyScanRequired: true,
	}
	target := &Target{ProductVersionNo: "PV_FILE_002", ConfigurationRevision: "cfg-rev-2", FieldKey: "certificate", Policy: policy}
	scanner := &purchaseFileScanner{ready: false, verdict: "clean"}
	service, err := New(database, purchaseFilePrincipal{}, &purchaseFileStorage{}, purchaseFileResolver{target: target}, scanner)
	if err != nil {
		t.Fatal(err)
	}
	service.(*serviceImpl).random = bytes.NewReader(bytes.Repeat([]byte{0x41}, 64))
	owner := context.WithValue(ctx, purchaseFileAudienceKey{}, activePurchaseFileAudience(801, "CUS_SCAN"))
	reservation, err := service.Reserve(owner, ReserveInput{PublicSlug: "ssl", ConfigurationRevision: "public-rev", FieldToken: "field-token", FileName: "certificate.pdf", SizeBytes: int64(len(pdf))})
	if err != nil {
		t.Fatal(err)
	}
	if item, uploadErr := service.Upload(owner, reservation.UploadToken, "application/pdf", bytes.NewReader(pdf)); item != nil || !bizerr.Is(uploadErr, CodeUnavailable) {
		t.Fatalf("scanner unavailable item=%+v err=%v", item, uploadErr)
	}
	status, err := service.Get(owner, reservation.UploadToken)
	if err != nil || status.Status != StatusReserved {
		t.Fatalf("transient scan failure did not restore reservation: item=%+v err=%v", status, err)
	}
	scanner.ready = true
	if _, err = service.Upload(owner, reservation.UploadToken, "application/pdf", bytes.NewReader(pdf)); err != nil {
		t.Fatalf("retry after scanner recovery: %v", err)
	}
	changedPolicy := *policy
	changedPolicy.MaximumTotalBytes++
	if result, verifyErr := service.VerifyBatch(owner, VerifyInput{Target: &Target{ProductVersionNo: target.ProductVersionNo, ConfigurationRevision: target.ConfigurationRevision, FieldKey: target.FieldKey, Policy: &changedPolicy}, FileTokens: []string{reservation.UploadToken}}); result != nil || !bizerr.Is(verifyErr, CodeConflict) {
		t.Fatalf("changed policy verification=%+v err=%v", result, verifyErr)
	}
}

func activePurchaseFileAudience(userID int, customerNo string) *principal.Audience {
	return &principal.Audience{UserID: userID, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: customerNo}
}

func configurePurchaseFileIntegrationDB(t *testing.T, ctx context.Context) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(purchaseFileIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL purchase-file integration tests", purchaseFileIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create PostgreSQL connection: %v", err)
	}
	var randomBytes [8]byte
	if _, err = rand.Read(randomBytes[:]); err != nil {
		t.Fatalf("create schema token: %v", err)
	}
	schema := "linapro_purchase_file_test_" + hex.EncodeToString(randomBytes[:])
	quotedSchema := `"` + strings.ReplaceAll(schema, `"`, `""`) + `"`
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	scopedDB, err := gdb.New(gdb.ConfigNode{Link: appendPurchaseFileDSNParameter(dsn, "search_path="+schema)})
	if err != nil {
		t.Fatalf("create scoped PostgreSQL connection: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := scopedDB.Close(ctx); closeErr != nil {
			t.Errorf("close scoped database: %v", closeErr)
		}
		if !strings.HasPrefix(schema, "linapro_purchase_file_test_") {
			t.Errorf("refusing to drop unexpected schema %q", schema)
		} else if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quotedSchema+" CASCADE"); dropErr != nil {
			t.Errorf("drop schema: %v", dropErr)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close admin database: %v", closeErr)
		}
	})
	migration, err := os.ReadFile(purchaseFileMigrationPath(t))
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	marker := []byte("-- Governed purchase-configuration file uploads belong to this same structured")
	section := bytes.Index(migration, marker)
	if section < 0 {
		t.Fatal("purchase file migration section is missing")
	}
	migration = migration[section:]
	if sidecar := bytes.Index(migration, []byte("-- Generic transaction evidence is a sidecar")); sidecar >= 0 {
		migration = migration[:sidecar]
	}
	if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("execute migration: %v", err)
	}
	if _, err = scopedDB.Exec(ctx, string(migration)); err != nil {
		t.Fatalf("repeat migration idempotently: %v", err)
	}
	return scopedDB
}

func purchaseFileMigrationPath(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", "075-structured-product-configuration.sql"))
}

func appendPurchaseFileDSNParameter(dsn, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}
