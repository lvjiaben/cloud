package commerce

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/glog"

	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	checkoutBenchmarkEnabledEnv      = "LINAPRO_IDC_CORE_CHECKOUT_BENCHMARK"
	checkoutBenchmarkMemoryGiBEnv    = "LINAPRO_IDC_CORE_BENCHMARK_MEMORY_GIB"
	checkoutBenchmarkStorageClassEnv = "LINAPRO_IDC_CORE_BENCHMARK_STORAGE_CLASS"
)

type checkoutBenchmarkManifest struct {
	SchemaVersion int                       `json:"schemaVersion"`
	BenchmarkID   string                    `json:"benchmarkId"`
	Hardware      checkoutBenchmarkHardware `json:"hardware"`
	Dataset       checkoutBenchmarkDataset  `json:"dataset"`
	Workload      checkoutBenchmarkWorkload `json:"workload"`
	Gates         checkoutBenchmarkGates    `json:"gates"`
}

type checkoutBenchmarkHardware struct {
	MinimumLogicalCPUs     int    `json:"minimumLogicalCpus"`
	MinimumMemoryGiB       int    `json:"minimumMemoryGiB"`
	StorageClass           string `json:"storageClass"`
	DatabaseNetwork        string `json:"databaseNetwork"`
	MinimumPostgreSQLMajor int    `json:"minimumPostgreSqlMajor"`
}

type checkoutBenchmarkDataset struct {
	TotalCustomerCount             int    `json:"totalCustomerCount"`
	BenchmarkCustomerCount         int    `json:"benchmarkCustomerCount"`
	CatalogProductCount            int    `json:"catalogProductCount"`
	InventorySellableUnits         int    `json:"inventorySellableUnits"`
	CartItemsPerOrder              int    `json:"cartItemsPerOrder"`
	ConfigurationSelectionsPerItem int    `json:"configurationSelectionsPerItem"`
	QuantityPerItem                int    `json:"quantityPerItem"`
	CurrencyCode                   string `json:"currencyCode"`
	DatasetScript                  string `json:"datasetScript"`
}

type checkoutBenchmarkWorkload struct {
	ColdSamples     int `json:"coldSamples"`
	WarmupSamples   int `json:"warmupSamples"`
	MeasuredSamples int `json:"measuredSamples"`
	Concurrency     int `json:"concurrency"`
}

type checkoutBenchmarkGates struct {
	P95Milliseconds                       int     `json:"p95Milliseconds"`
	P99Milliseconds                       int     `json:"p99Milliseconds"`
	MaximumQueriesPerSample               int     `json:"maximumQueriesPerSample"`
	MaximumQueryGrowthFromOneToEightItems int     `json:"maximumQueryGrowthFromOneToEightItems"`
	MaximumErrorRate                      float64 `json:"maximumErrorRate"`
	MaximumExternalNetworkCalls           int64   `json:"maximumExternalNetworkCalls"`
}

type checkoutBenchmarkCase struct {
	ctx            context.Context
	checkout       *serviceImpl
	payment        PaymentPreparationService
	quoteNo        string
	idempotencyKey string
}

type checkoutBenchmarkSample struct {
	Duration time.Duration
	Queries  int
	Err      error
}

type checkoutBenchmarkStage struct {
	Name         string  `json:"name"`
	Samples      int     `json:"samples"`
	Concurrency  int     `json:"concurrency"`
	ElapsedMs    int64   `json:"elapsedMs"`
	P50Ms        float64 `json:"p50Ms"`
	P95Ms        float64 `json:"p95Ms"`
	P99Ms        float64 `json:"p99Ms"`
	MaximumMs    float64 `json:"maximumMs"`
	MaximumQuery int     `json:"maximumQueries"`
	ErrorRate    float64 `json:"errorRate"`
}

type checkoutBenchmarkReport struct {
	BenchmarkID          string                      `json:"benchmarkId"`
	ManifestSHA256       string                      `json:"manifestSha256"`
	GeneratedAt          string                      `json:"generatedAt"`
	GoVersion            string                      `json:"goVersion"`
	GOOS                 string                      `json:"goos"`
	GOARCH               string                      `json:"goarch"`
	LogicalCPUs          int                         `json:"logicalCpus"`
	MemoryGiB            float64                     `json:"memoryGiB"`
	StorageClass         string                      `json:"storageClass"`
	DatabaseNetwork      string                      `json:"databaseNetwork"`
	PostgreSQLVersion    string                      `json:"postgreSqlVersion"`
	FixturePreparationMs int64                       `json:"fixturePreparationMs"`
	WarmupMs             int64                       `json:"warmupMs"`
	ExternalNetworkCalls int64                       `json:"externalNetworkCalls"`
	Dataset              checkoutBenchmarkDataset    `json:"dataset"`
	QueryShape           checkoutBenchmarkQueryShape `json:"queryShape"`
	Stages               []checkoutBenchmarkStage    `json:"stages"`
	Passed               bool                        `json:"passed"`
}

type checkoutBenchmarkEnvironment struct {
	memoryGiB       float64
	storageClass    string
	databaseNetwork string
	postgreSQL      string
}

type checkoutBenchmarkQueryShape struct {
	BaselineCartItems int `json:"baselineCartItems"`
	BaselineQueries   int `json:"baselineQueries"`
	TargetCartItems   int `json:"targetCartItems"`
	TargetQueries     int `json:"targetQueries"`
	QueryGrowth       int `json:"queryGrowth"`
}

type checkoutBenchmarkSampleKey struct{}

type checkoutBenchmarkQueryLogger struct {
	glog.ILogger
	mu     sync.Mutex
	counts map[int]int
}

func (l *checkoutBenchmarkQueryLogger) Debug(ctx context.Context, _ ...any) {
	l.record(ctx)
}

func (l *checkoutBenchmarkQueryLogger) Error(ctx context.Context, _ ...any) {
	l.record(ctx)
}

func (l *checkoutBenchmarkQueryLogger) record(ctx context.Context) {
	sampleID, ok := ctx.Value(checkoutBenchmarkSampleKey{}).(int)
	if !ok {
		return
	}
	l.mu.Lock()
	l.counts[sampleID]++
	l.mu.Unlock()
}

func (l *checkoutBenchmarkQueryLogger) count(sampleID int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.counts[sampleID]
}

type checkoutBenchmarkNetworkTripwire struct {
	calls atomic.Int64
}

func (t *checkoutBenchmarkNetworkTripwire) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return nil, errors.New("checkout benchmark blocked an external network call")
}

func TestCheckoutBenchmarkManifestPostgreSQL(t *testing.T) {
	if strings.TrimSpace(os.Getenv(checkoutBenchmarkEnabledEnv)) != "1" {
		t.Skipf("set %s=1 to run the versioned checkout benchmark", checkoutBenchmarkEnabledEnv)
	}
	manifest, manifestDigest, benchmarkDirectory := loadCheckoutBenchmarkManifest(t)
	validateCheckoutBenchmarkManifest(t, manifest)
	fixtureStarted := time.Now()
	ctx, database := newCommerceServiceTestDatabase(t)
	environment := validateCheckoutBenchmarkEnvironment(t, ctx, database, manifest)
	seedCommerceServiceCatalog(t, ctx, database)
	executeCheckoutBenchmarkDataset(t, ctx, database, benchmarkDirectory, manifest.Dataset.DatasetScript)
	assertCheckoutBenchmarkDataset(t, ctx, database, manifest.Dataset)
	cases := prepareCheckoutBenchmarkCases(t, ctx, database, manifest)
	queryShapeCase := prepareCheckoutBenchmarkQueryShapeCase(t, ctx, database, manifest.Dataset)
	fixtureElapsed := time.Since(fixtureStarted)

	queryLogger := &checkoutBenchmarkQueryLogger{ILogger: database.GetLogger(), counts: make(map[int]int)}
	previousLogger, previousDebug := database.GetLogger(), database.GetDebug()
	database.SetLogger(queryLogger)
	database.SetDebug(true)
	t.Cleanup(func() {
		database.SetDebug(previousDebug)
		database.SetLogger(previousLogger)
	})
	tripwire := &checkoutBenchmarkNetworkTripwire{}
	previousTransport := http.DefaultTransport
	http.DefaultTransport = tripwire
	t.Cleanup(func() { http.DefaultTransport = previousTransport })

	coldCases, warmupCases, measuredCases := splitCheckoutBenchmarkCases(cases, manifest.Workload)
	coldStarted := time.Now()
	coldSamples := runCheckoutBenchmarkCases(coldCases, 1, 0, queryLogger)
	coldElapsed := time.Since(coldStarted)
	warmupStarted := time.Now()
	warmupSamples := runCheckoutBenchmarkCases(warmupCases, 1, len(coldCases), queryLogger)
	warmupElapsed := time.Since(warmupStarted)
	assertCheckoutBenchmarkSamples(t, warmupSamples, manifest.Gates, false)
	measuredStarted := time.Now()
	measuredSamples := runCheckoutBenchmarkCases(
		measuredCases, manifest.Workload.Concurrency, len(coldCases)+len(warmupCases), queryLogger,
	)
	measuredElapsed := time.Since(measuredStarted)

	coldStage := projectCheckoutBenchmarkStage("application_cold", coldSamples, 1, coldElapsed)
	warmStage := projectCheckoutBenchmarkStage(
		"application_warm", measuredSamples, manifest.Workload.Concurrency, measuredElapsed,
	)
	assertCheckoutBenchmarkSamples(t, coldSamples, manifest.Gates, false)
	assertCheckoutBenchmarkSamples(t, measuredSamples, manifest.Gates, true)
	queryShapeSample := runCheckoutBenchmarkCase(queryShapeCase, len(cases), queryLogger)
	assertCheckoutBenchmarkSamples(t, []checkoutBenchmarkSample{queryShapeSample}, manifest.Gates, false)
	targetQueries := maximumCheckoutBenchmarkQueries(measuredSamples)
	queryGrowth := targetQueries - queryShapeSample.Queries
	if queryGrowth > manifest.Gates.MaximumQueryGrowthFromOneToEightItems {
		t.Fatalf(
			"checkout query growth one-item=%d eight-items=%d growth=%d maximum=%d",
			queryShapeSample.Queries, targetQueries, queryGrowth,
			manifest.Gates.MaximumQueryGrowthFromOneToEightItems,
		)
	}
	if tripwire.calls.Load() != manifest.Gates.MaximumExternalNetworkCalls {
		t.Fatalf("checkout external network calls=%d expected=%d", tripwire.calls.Load(), manifest.Gates.MaximumExternalNetworkCalls)
	}
	report := checkoutBenchmarkReport{
		BenchmarkID: manifest.BenchmarkID, ManifestSHA256: manifestDigest,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339), GoVersion: runtime.Version(),
		GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, LogicalCPUs: runtime.NumCPU(),
		MemoryGiB: environment.memoryGiB, StorageClass: environment.storageClass,
		DatabaseNetwork: environment.databaseNetwork, PostgreSQLVersion: environment.postgreSQL,
		FixturePreparationMs: fixtureElapsed.Milliseconds(), WarmupMs: warmupElapsed.Milliseconds(),
		ExternalNetworkCalls: tripwire.calls.Load(), Dataset: manifest.Dataset,
		QueryShape: checkoutBenchmarkQueryShape{
			BaselineCartItems: 1, BaselineQueries: queryShapeSample.Queries,
			TargetCartItems: manifest.Dataset.CartItemsPerOrder, TargetQueries: targetQueries,
			QueryGrowth: queryGrowth,
		},
		Stages: []checkoutBenchmarkStage{coldStage, warmStage}, Passed: true,
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("encode checkout benchmark report: %v", err)
	}
	t.Logf("CHECKOUT_BENCHMARK_REPORT %s", encoded)
}

func loadCheckoutBenchmarkManifest(t *testing.T) (
	checkoutBenchmarkManifest,
	string,
	string,
) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve checkout benchmark source path")
	}
	directory := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "hack", "benchmarks", "checkout", "v1")
	content, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		t.Fatalf("read checkout benchmark manifest: %v", err)
	}
	var manifest checkoutBenchmarkManifest
	if err = json.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("decode checkout benchmark manifest: %v", err)
	}
	digest := sha256.Sum256(content)
	return manifest, hex.EncodeToString(digest[:]), directory
}

func validateCheckoutBenchmarkManifest(t *testing.T, manifest checkoutBenchmarkManifest) {
	t.Helper()
	requiredCases := manifest.Workload.ColdSamples + manifest.Workload.WarmupSamples + manifest.Workload.MeasuredSamples
	if manifest.SchemaVersion != 1 || manifest.BenchmarkID == "" ||
		manifest.Hardware.MinimumLogicalCPUs < 1 || manifest.Hardware.MinimumMemoryGiB < 1 ||
		manifest.Hardware.StorageClass == "" || manifest.Hardware.DatabaseNetwork != "loopback" ||
		manifest.Hardware.MinimumPostgreSQLMajor < 14 ||
		manifest.Dataset.BenchmarkCustomerCount != requiredCases ||
		manifest.Dataset.TotalCustomerCount < manifest.Dataset.BenchmarkCustomerCount ||
		manifest.Dataset.CartItemsPerOrder != 8 ||
		manifest.Dataset.InventorySellableUnits < requiredCases*manifest.Dataset.CartItemsPerOrder ||
		manifest.Dataset.ConfigurationSelectionsPerItem != 1 || manifest.Dataset.QuantityPerItem < 1 ||
		manifest.Workload.ColdSamples != 1 || manifest.Workload.WarmupSamples < 1 ||
		manifest.Workload.MeasuredSamples < 20 || manifest.Workload.Concurrency < 1 ||
		manifest.Gates.P95Milliseconds < 1 || manifest.Gates.P99Milliseconds < manifest.Gates.P95Milliseconds ||
		manifest.Gates.MaximumQueriesPerSample < 1 ||
		manifest.Gates.MaximumQueryGrowthFromOneToEightItems < 0 ||
		manifest.Gates.MaximumExternalNetworkCalls != 0 {
		t.Fatalf("invalid checkout benchmark manifest: %+v", manifest)
	}
	if runtime.NumCPU() < manifest.Hardware.MinimumLogicalCPUs {
		t.Fatalf("checkout benchmark requires at least %d logical CPUs; found %d", manifest.Hardware.MinimumLogicalCPUs, runtime.NumCPU())
	}
}

func validateCheckoutBenchmarkEnvironment(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	manifest checkoutBenchmarkManifest,
) checkoutBenchmarkEnvironment {
	t.Helper()
	memoryGiB, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(checkoutBenchmarkMemoryGiBEnv)), 64)
	if err != nil || memoryGiB < float64(manifest.Hardware.MinimumMemoryGiB) {
		t.Fatalf(
			"set %s to measured memory GiB at or above %d",
			checkoutBenchmarkMemoryGiBEnv, manifest.Hardware.MinimumMemoryGiB,
		)
	}
	storageClass := strings.TrimSpace(os.Getenv(checkoutBenchmarkStorageClassEnv))
	if storageClass != manifest.Hardware.StorageClass {
		t.Fatalf("set %s=%s after verifying the benchmark storage", checkoutBenchmarkStorageClassEnv, manifest.Hardware.StorageClass)
	}
	if manifest.Hardware.DatabaseNetwork != "loopback" || !checkoutBenchmarkUsesLoopbackDatabase() {
		t.Fatal("checkout benchmark database must use a loopback endpoint")
	}
	postgreSQL := checkoutBenchmarkPostgreSQLVersion(t, ctx, database)
	majorText, _, _ := strings.Cut(postgreSQL, ".")
	major, err := strconv.Atoi(majorText)
	if err != nil || major < manifest.Hardware.MinimumPostgreSQLMajor {
		t.Fatalf(
			"checkout benchmark requires PostgreSQL major %d or newer; found %q",
			manifest.Hardware.MinimumPostgreSQLMajor, postgreSQL,
		)
	}
	return checkoutBenchmarkEnvironment{
		memoryGiB: memoryGiB, storageClass: storageClass,
		databaseNetwork: manifest.Hardware.DatabaseNetwork, postgreSQL: postgreSQL,
	}
}

func checkoutBenchmarkUsesLoopbackDatabase() bool {
	return checkoutBenchmarkDSNUsesLoopback(os.Getenv(commerceIntegrationDSNEnv))
}

func checkoutBenchmarkDSNUsesLoopback(dsn string) bool {
	normalized := strings.TrimSpace(dsn)
	marker := strings.LastIndex(strings.ToLower(normalized), "@tcp(")
	if marker < 0 {
		return false
	}
	endpoint := normalized[marker+len("@tcp("):]
	closing := strings.Index(endpoint, ")")
	if closing < 0 {
		return false
	}
	host, _, err := net.SplitHostPort(endpoint[:closing])
	if err != nil {
		return false
	}
	address := net.ParseIP(host)
	return strings.EqualFold(host, "localhost") || address != nil && address.IsLoopback()
}

func TestCheckoutBenchmarkDSNUsesLoopback(t *testing.T) {
	tests := map[string]struct {
		dsn      string
		expected bool
	}{
		"ipv4":             {dsn: "pgsql:user:secret@tcp(127.0.0.1:5432)/cloud", expected: true},
		"ipv6":             {dsn: "pgsql:user:secret@tcp([::1]:5432)/cloud", expected: true},
		"localhost":        {dsn: "pgsql:user:secret@tcp(localhost:5432)/cloud", expected: true},
		"remote":           {dsn: "pgsql:user:secret@tcp(192.0.2.1:5432)/cloud", expected: false},
		"password decoy":   {dsn: "pgsql:user:127.0.0.1@tcp(192.0.2.1:5432)/cloud", expected: false},
		"malformed":        {dsn: "pgsql:user:secret@tcp(127.0.0.1)/cloud", expected: false},
		"missing endpoint": {dsn: "postgres://127.0.0.1/cloud", expected: false},
	}
	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			if actual := checkoutBenchmarkDSNUsesLoopback(testCase.dsn); actual != testCase.expected {
				t.Fatalf("checkout benchmark loopback=%v expected=%v", actual, testCase.expected)
			}
		})
	}
}

func assertCheckoutBenchmarkDataset(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	dataset checkoutBenchmarkDataset,
) {
	t.Helper()
	assertCheckoutBenchmarkCount(t, ctx, database,
		"SELECT COUNT(*) FROM plugin_idc_core_customer_subject", int64(dataset.TotalCustomerCount))
	assertCheckoutBenchmarkCount(t, ctx, database, `
SELECT COUNT(*) FROM plugin_idc_core_customer_subject
WHERE customer_no LIKE 'benchmark-customer-%'`, int64(dataset.BenchmarkCustomerCount))
	assertCheckoutBenchmarkCount(t, ctx, database,
		"SELECT COUNT(*) FROM plugin_idc_core_catalog_product", int64(dataset.CatalogProductCount))
	assertCheckoutBenchmarkCount(t, ctx, database, `
SELECT sellable_quantity
FROM plugin_idc_core_catalog_inventory_snapshot
WHERE snapshot_no = 'inventory-commerce-benchmark-v1'`, int64(dataset.InventorySellableUnits))
}

func assertCheckoutBenchmarkCount(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	query string,
	expected int64,
) {
	t.Helper()
	value, err := database.GetValue(ctx, query)
	if err != nil || value.Int64() != expected {
		t.Fatalf("checkout benchmark dataset value=%d expected=%d err=%v", value.Int64(), expected, err)
	}
}

func executeCheckoutBenchmarkDataset(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	directory string,
	scriptName string,
) {
	t.Helper()
	if filepath.Base(scriptName) != scriptName || filepath.Ext(scriptName) != ".sql" {
		t.Fatalf("invalid checkout benchmark dataset script %q", scriptName)
	}
	content, err := os.ReadFile(filepath.Join(directory, scriptName))
	if err != nil {
		t.Fatalf("read checkout benchmark dataset: %v", err)
	}
	if _, err = database.Exec(ctx, string(content)); err != nil {
		t.Fatalf("execute checkout benchmark dataset: %v", err)
	}
}

func prepareCheckoutBenchmarkCases(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	manifest checkoutBenchmarkManifest,
) []checkoutBenchmarkCase {
	t.Helper()
	resolver, purchase := checkoutBenchmarkPurchase(t, ctx, database, manifest.Dataset.CurrencyCode)
	cases := make([]checkoutBenchmarkCase, 0, manifest.Dataset.BenchmarkCustomerCount)
	for sequence := 1; sequence <= manifest.Dataset.BenchmarkCustomerCount; sequence++ {
		cases = append(cases, prepareCheckoutBenchmarkCase(
			t, ctx, database, resolver, purchase, manifest.Dataset, sequence,
		))
	}
	return cases
}

func prepareCheckoutBenchmarkQueryShapeCase(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	dataset checkoutBenchmarkDataset,
) checkoutBenchmarkCase {
	t.Helper()
	resolver, purchase := checkoutBenchmarkPurchase(t, ctx, database, dataset.CurrencyCode)
	service, quote := newCheckoutCustomer(
		t, ctx, database, resolver, purchase, "customer-commerce-service-1", 9999, 1, false,
	)
	principalStub := commercePrincipalStub{audience: &principal.Audience{
		UserID: 9999, Code: principal.AudienceCustomer,
		Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1",
	}}
	payment, err := NewPaymentPreparationService(database, principalStub)
	if err != nil {
		t.Fatalf("create checkout benchmark query-shape payment service: %v", err)
	}
	return checkoutBenchmarkCase{
		ctx: ctx, checkout: service, payment: payment, quoteNo: quote.QuoteNo,
		idempotencyKey: "checkout-benchmark-v1-query-shape",
	}
}

func checkoutBenchmarkPurchase(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	currencyCode string,
) (catalog.PurchaseConfigurationResolver, catalog.PurchaseConfigurationInput) {
	t.Helper()
	publicReader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create checkout benchmark catalog reader: %v", err)
	}
	bindTestPublicBillingLabels(t, publicReader)
	resolver, err := catalog.PurchaseResolverFromPublicReader(publicReader)
	if err != nil {
		t.Fatalf("create checkout benchmark resolver: %v", err)
	}
	configuration, err := publicReader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil {
		t.Fatalf("load checkout benchmark configuration: %v", err)
	}
	quoted, err := publicReader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision},
		CurrencyCode:                     currencyCode,
	})
	if err != nil || quoted == nil || len(quoted.Terms) != 1 {
		t.Fatalf("load checkout benchmark quote result=%+v err=%v", quoted, err)
	}
	return resolver, catalog.PurchaseConfigurationInput{
		PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken: quoted.Terms[0].BillingTermToken, CurrencyCode: currencyCode,
		PurchaseCycles: 1,
	}
}

func prepareCheckoutBenchmarkCase(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	resolver catalog.PurchaseConfigurationResolver,
	purchase catalog.PurchaseConfigurationInput,
	dataset checkoutBenchmarkDataset,
	sequence int,
) checkoutBenchmarkCase {
	t.Helper()
	customerNo := fmt.Sprintf("benchmark-customer-%03d", sequence)
	principalStub := commercePrincipalStub{audience: &principal.Audience{
		UserID: 10_000 + sequence, Code: principal.AudienceCustomer,
		Status: principal.AudienceStatusActive, CustomerNo: customerNo,
	}}
	service, err := newService(
		database, resolver, principalStub, rand.Reader, commerceSettlementStub{},
	)
	if err != nil {
		t.Fatalf("create checkout benchmark service %d: %v", sequence, err)
	}
	payment, err := NewPaymentPreparationService(database, principalStub)
	if err != nil {
		t.Fatalf("create checkout benchmark payment service %d: %v", sequence, err)
	}
	cart, err := service.OpenAccountCart(ctx, dataset.CurrencyCode)
	if err != nil {
		t.Fatalf("open checkout benchmark cart %d: %v", sequence, err)
	}
	for range dataset.CartItemsPerOrder {
		cart, err = service.AddAccountItem(ctx, AddItemInput{
			ExpectedCartVersion: cart.Version,
			Quantity:            dataset.QuantityPerItem,
			Purchase:            purchase,
		})
		if err != nil {
			t.Fatalf("add checkout benchmark item %d: %v", sequence, err)
		}
	}
	quote, err := service.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: cart.Version})
	if err != nil {
		t.Fatalf("quote checkout benchmark cart %d: %v", sequence, err)
	}
	return checkoutBenchmarkCase{
		ctx: ctx, checkout: service, payment: payment, quoteNo: quote.QuoteNo,
		idempotencyKey: fmt.Sprintf("checkout-benchmark-v1-%03d", sequence),
	}
}

func splitCheckoutBenchmarkCases(
	cases []checkoutBenchmarkCase,
	workload checkoutBenchmarkWorkload,
) ([]checkoutBenchmarkCase, []checkoutBenchmarkCase, []checkoutBenchmarkCase) {
	coldEnd := workload.ColdSamples
	warmupEnd := coldEnd + workload.WarmupSamples
	return cases[:coldEnd], cases[coldEnd:warmupEnd], cases[warmupEnd:]
}

func runCheckoutBenchmarkCases(
	cases []checkoutBenchmarkCase,
	concurrency int,
	sampleOffset int,
	queryLogger *checkoutBenchmarkQueryLogger,
) []checkoutBenchmarkSample {
	results := make([]checkoutBenchmarkSample, len(cases))
	work := make(chan int)
	var wait sync.WaitGroup
	for range concurrency {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for index := range work {
				results[index] = runCheckoutBenchmarkCase(cases[index], sampleOffset+index, queryLogger)
			}
		}()
	}
	for index := range cases {
		work <- index
	}
	close(work)
	wait.Wait()
	return results
}

func runCheckoutBenchmarkCase(
	benchmarkCase checkoutBenchmarkCase,
	sampleID int,
	queryLogger *checkoutBenchmarkQueryLogger,
) checkoutBenchmarkSample {
	ctx := context.WithValue(benchmarkCase.ctx, checkoutBenchmarkSampleKey{}, sampleID)
	started := time.Now()
	checkout, err := benchmarkCase.checkout.CheckoutAccountQuote(ctx, CheckoutRequest{
		QuoteNo: benchmarkCase.quoteNo, IdempotencyKey: benchmarkCase.idempotencyKey,
	})
	if err == nil {
		var preparation *PaymentPreparation
		preparation, err = benchmarkCase.payment.PrepareAccountOrderPayment(ctx, checkout.OrderNo)
		if err == nil && (preparation == nil || !preparation.Ready) {
			err = fmt.Errorf("payment preparation is not ready: %+v", preparation)
		}
	}
	return checkoutBenchmarkSample{
		Duration: time.Since(started), Queries: queryLogger.count(sampleID), Err: err,
	}
}

func projectCheckoutBenchmarkStage(
	name string,
	samples []checkoutBenchmarkSample,
	concurrency int,
	elapsed time.Duration,
) checkoutBenchmarkStage {
	durations := make([]time.Duration, 0, len(samples))
	maximumQueries, errorsFound := 0, 0
	for _, sample := range samples {
		durations = append(durations, sample.Duration)
		if sample.Queries > maximumQueries {
			maximumQueries = sample.Queries
		}
		if sample.Err != nil {
			errorsFound++
		}
	}
	sort.Slice(durations, func(left, right int) bool { return durations[left] < durations[right] })
	errorRate := float64(errorsFound) / float64(len(samples))
	return checkoutBenchmarkStage{
		Name: name, Samples: len(samples), Concurrency: concurrency, ElapsedMs: elapsed.Milliseconds(),
		P50Ms:        checkoutBenchmarkMilliseconds(checkoutBenchmarkPercentile(durations, 0.50)),
		P95Ms:        checkoutBenchmarkMilliseconds(checkoutBenchmarkPercentile(durations, 0.95)),
		P99Ms:        checkoutBenchmarkMilliseconds(checkoutBenchmarkPercentile(durations, 0.99)),
		MaximumMs:    checkoutBenchmarkMilliseconds(durations[len(durations)-1]),
		MaximumQuery: maximumQueries, ErrorRate: errorRate,
	}
}

func maximumCheckoutBenchmarkQueries(samples []checkoutBenchmarkSample) int {
	maximum := 0
	for _, sample := range samples {
		if sample.Queries > maximum {
			maximum = sample.Queries
		}
	}
	return maximum
}

func assertCheckoutBenchmarkSamples(
	t *testing.T,
	samples []checkoutBenchmarkSample,
	gates checkoutBenchmarkGates,
	assertLatency bool,
) {
	t.Helper()
	stage := projectCheckoutBenchmarkStage("gate", samples, 1, 0)
	for index, sample := range samples {
		if sample.Err != nil {
			t.Fatalf("checkout benchmark sample %d failed: %v", index, sample.Err)
		}
		if sample.Queries > gates.MaximumQueriesPerSample {
			t.Fatalf("checkout benchmark sample %d queries=%d maximum=%d", index, sample.Queries, gates.MaximumQueriesPerSample)
		}
	}
	if stage.ErrorRate > gates.MaximumErrorRate {
		t.Fatalf("checkout benchmark error rate=%.4f maximum=%.4f", stage.ErrorRate, gates.MaximumErrorRate)
	}
	if assertLatency && (stage.P95Ms > float64(gates.P95Milliseconds) || stage.P99Ms > float64(gates.P99Milliseconds)) {
		t.Fatalf("checkout benchmark latency p95=%.3fms p99=%.3fms gates=%dms/%dms", stage.P95Ms, stage.P99Ms, gates.P95Milliseconds, gates.P99Milliseconds)
	}
}

func checkoutBenchmarkPercentile(sorted []time.Duration, percentile float64) time.Duration {
	index := int(float64(len(sorted))*percentile+0.999999999) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func checkoutBenchmarkMilliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}

func checkoutBenchmarkPostgreSQLVersion(t *testing.T, ctx context.Context, database gdb.DB) string {
	t.Helper()
	value, err := database.GetValue(ctx, "SHOW server_version")
	if err != nil {
		t.Fatalf("load checkout benchmark PostgreSQL version: %v", err)
	}
	return value.String()
}
