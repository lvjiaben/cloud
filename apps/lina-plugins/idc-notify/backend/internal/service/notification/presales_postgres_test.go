package notification

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/cap/supportcap"
	"lina-plugin-idc-notify/backend/cap/channelcap"
	"lina-plugin-shared-services/backend/cap/challengecap"
)

func TestAnonymousPresalesVerificationAndTicketAreIdempotentPostgreSQL(t *testing.T) {
	database, cleanup := openPresalesPostgreSQL(t)
	defer cleanup()
	service, challenge, adapter, support := presalesServiceFixture(t, database)
	binding := PresalesBindingInput{BrowserNonce: []byte("browser-nonce-000000000001"), Destination: []byte("Buyer@Example.com"),
		DestinationKind: "email", Locale: "en-US", Operation: "create"}
	presentation, err := service.CreatePresalesHumanChallenge(t.Context(), PresalesHumanChallengeInput{
		PresalesBindingInput: clonePresalesBinding(binding), PresentationKind: "accessible",
	})
	if err != nil || presentation.AccessiblePrompt == "" {
		t.Fatalf("create challenge: %+v err=%v", presentation, err)
	}
	proof, err := service.CompletePresalesHumanChallenge(t.Context(), PresalesHumanCompletionInput{
		PresalesBindingInput: clonePresalesBinding(binding), ChallengeNo: presentation.ChallengeNo, Answer: []byte("7"),
	})
	if err != nil {
		t.Fatalf("complete challenge: %v", err)
	}
	input := PresalesVerificationInput{PresalesBindingInput: clonePresalesBinding(binding), ProofToken: proof.ProofToken,
		IdempotencyKey: "verification-idempotency-001", ClientIP: []byte("203.0.113.8")}
	const workers = 8
	results := make(chan *PresalesVerification, workers)
	errorsFound := make(chan error, workers)
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			copyInput := input
			copyInput.PresalesBindingInput = clonePresalesBinding(binding)
			copyInput.ClientIP = []byte("203.0.113.8")
			item, sendErr := service.SendPresalesVerificationCode(t.Context(), copyInput)
			results <- item
			errorsFound <- sendErr
		}()
	}
	wait.Wait()
	close(results)
	close(errorsFound)
	var verificationNo string
	for sendErr := range errorsFound {
		if sendErr != nil {
			t.Fatalf("concurrent idempotent send: %v", sendErr)
		}
	}
	for item := range results {
		if item == nil || item.DeliveryState != "delivered" && item.DeliveryState != "result_unknown" {
			t.Fatalf("bad verification result: %+v", item)
		}
		if verificationNo == "" {
			verificationNo = item.VerificationNo
		} else if verificationNo != item.VerificationNo {
			t.Fatalf("idempotency returned different numbers")
		}
	}
	if challenge.consumeCount() != 1 || adapter.deliveryCount() != 1 {
		t.Fatalf("duplicate boundary call: consume=%d deliveries=%d", challenge.consumeCount(), adapter.deliveryCount())
	}
	code := adapter.lastCode()
	if !regexp.MustCompile(`^[0-9]{6}$`).MatchString(code) {
		t.Fatalf("provider did not receive a six-digit code: %q", code)
	}
	noteValue := "Two private racks"
	urgentValue := true
	customFields := []supportcap.CustomFieldValue{
		{FieldNo: "SCF_service_area", OptionNos: []string{"SCO_network", "SCO_compute"}},
		{FieldNo: "SCF_notes", TextValue: &noteValue},
		{FieldNo: "SCF_urgent", BooleanValue: &urgentValue},
	}
	_, err = service.CreateAnonymousPresalesTicket(t.Context(), PresalesTicketInput{
		BrowserNonce: []byte("browser-nonce-000000000001"), Destination: []byte("Buyer@Example.com"), DestinationKind: "email", Locale: "en-US",
		OperationNo: "ticket-operation-invalid-0001", VerificationNo: verificationNo, Code: []byte(code),
		CategoryNo: "sales", Subject: "Dedicated server", Body: "Please prepare a quotation.",
		CustomFields: []supportcap.CustomFieldValue{{FieldNo: "SCF_ambiguous", TextValue: &noteValue, BooleanValue: &urgentValue}},
	})
	if err == nil || !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("invalid support request should fail before consuming verification: %v", err)
	}
	status, queryErr := database.GetValue(t.Context(), `SELECT status_code FROM plugin_idc_notify_contact_verification
WHERE tenant_id=0 AND verification_no=?`, verificationNo)
	if queryErr != nil || status.String() != "sent" {
		t.Fatalf("invalid support request consumed verification: status=%q err=%v", status.String(), queryErr)
	}
	access, err := service.CreateAnonymousPresalesTicket(t.Context(), PresalesTicketInput{
		BrowserNonce: []byte("browser-nonce-000000000001"), Destination: []byte("Buyer@Example.com"), DestinationKind: "email", Locale: "en-US",
		OperationNo: "ticket-operation-0001", VerificationNo: verificationNo, Code: []byte(code),
		CategoryNo: "sales", Subject: "Dedicated server", Body: "Please prepare a quotation.",
		CustomFields: customFields,
	})
	if err != nil || access.TicketNo == "" || access.AccessToken == "" || access.LinkDeliveryState != "delivered" {
		t.Fatalf("create anonymous ticket: %+v err=%v", access, err)
	}
	if got := support.lastCreate().CustomFields; !reflect.DeepEqual(got, customFields) {
		t.Fatalf("custom fields were not passed unchanged to the support owner: got=%+v want=%+v", got, customFields)
	}
	replayedFields := []supportcap.CustomFieldValue{
		{FieldNo: "SCF_urgent", BooleanValue: &urgentValue},
		{FieldNo: "SCF_notes", TextValue: &noteValue},
		{FieldNo: "SCF_service_area", OptionNos: []string{"SCO_compute", "SCO_network"}},
	}
	replayed, err := service.CreateAnonymousPresalesTicket(t.Context(), PresalesTicketInput{
		BrowserNonce: []byte("browser-nonce-000000000001"), Destination: []byte("Buyer@Example.com"), DestinationKind: "email", Locale: "en-US",
		OperationNo: "ticket-operation-0001", VerificationNo: verificationNo, Code: []byte(code),
		CategoryNo: "sales", Subject: "Dedicated server", Body: "Please prepare a quotation.",
		CustomFields: replayedFields,
	})
	if err != nil || replayed.AccessToken != access.AccessToken {
		t.Fatalf("replay ticket operation: %+v err=%v", replayed, err)
	}
	if adapter.deliveryCount() != 2 || support.createCount() != 2 {
		t.Fatalf("ticket replay duplicated link or failed core recovery: deliveries=%d support=%d", adapter.deliveryCount(), support.createCount())
	}
	if !strings.Contains(adapter.lastBody(), "/auth/presales/access/") || !strings.Contains(adapter.lastBody(), "?ticket="+access.TicketNo) {
		t.Fatalf("access link did not use the configured origin and frozen path: %q", adapter.lastBody())
	}
	changedValue := "Four private racks"
	_, err = service.CreateAnonymousPresalesTicket(t.Context(), PresalesTicketInput{
		BrowserNonce: []byte("browser-nonce-000000000001"), Destination: []byte("Buyer@Example.com"), DestinationKind: "email", Locale: "en-US",
		OperationNo: "ticket-operation-0001", VerificationNo: verificationNo, Code: []byte(code),
		CategoryNo: "sales", Subject: "Dedicated server", Body: "Please prepare a quotation.",
		CustomFields: []supportcap.CustomFieldValue{{FieldNo: "SCF_notes", TextValue: &changedValue}},
	})
	if err == nil || !bizerr.Is(err, CodeConflict) || support.createCount() != 2 {
		t.Fatalf("changed custom fields must conflict before a second owner call: err=%v calls=%d", err, support.createCount())
	}
}

func TestPresalesVerificationLocksAfterFiveWrongCodesPostgreSQL(t *testing.T) {
	database, cleanup := openPresalesPostgreSQL(t)
	defer cleanup()
	service, _, adapter, _ := presalesServiceFixture(t, database)
	input := PresalesVerificationInput{PresalesBindingInput: PresalesBindingInput{
		BrowserNonce: []byte("browser-nonce-lock-00000001"), Destination: []byte("locked@example.com"),
		DestinationKind: "email", Locale: "en-US", Operation: "create",
	}, ProofToken: "proof_lock", IdempotencyKey: "verification-lock-001", ClientIP: []byte("192.0.2.49")}
	verification, err := service.SendPresalesVerificationCode(t.Context(), input)
	if err != nil || verification == nil || adapter.deliveryCount() != 1 {
		t.Fatalf("send lock verification: %+v err=%v deliveries=%d", verification, err, adapter.deliveryCount())
	}
	correctCode := adapter.lastCode()
	wrongCode := "000000"
	if correctCode == wrongCode {
		wrongCode = "999999"
	}
	for attempt := 0; attempt < 5; attempt++ {
		_, err = service.CreateAnonymousPresalesTicket(t.Context(), PresalesTicketInput{
			BrowserNonce: []byte("browser-nonce-lock-00000001"), Destination: []byte("locked@example.com"),
			DestinationKind: "email", Locale: "en-US", OperationNo: "ticket-lock-attempt",
			VerificationNo: verification.VerificationNo, Code: []byte(wrongCode), CategoryNo: "sales",
			Subject: "Dedicated server", Body: "Please prepare a quotation.",
		})
		if err == nil || !bizerr.Is(err, CodeVerificationInvalid) {
			t.Fatalf("wrong code attempt %d: %v", attempt+1, err)
		}
	}
	var locked struct {
		StatusCode     string `json:"status_code"`
		FailedAttempts int    `json:"failed_attempts"`
	}
	if err = database.Model(contactVerificationTable).Ctx(t.Context()).
		Fields("status_code", "failed_attempts").
		Where("tenant_id=? AND verification_no=?", notificationTenantID, verification.VerificationNo).
		Scan(&locked); err != nil {
		t.Fatal(err)
	}
	if locked.StatusCode != "locked" || locked.FailedAttempts != 5 {
		t.Fatalf("verification was not locked: status=%q attempts=%d", locked.StatusCode, locked.FailedAttempts)
	}
	_, err = service.CreateAnonymousPresalesTicket(t.Context(), PresalesTicketInput{
		BrowserNonce: []byte("browser-nonce-lock-00000001"), Destination: []byte("locked@example.com"),
		DestinationKind: "email", Locale: "en-US", OperationNo: "ticket-lock-correct",
		VerificationNo: verification.VerificationNo, Code: []byte(correctCode), CategoryNo: "sales",
		Subject: "Dedicated server", Body: "Please prepare a quotation.",
	})
	if err == nil || !bizerr.Is(err, CodeVerificationInvalid) {
		t.Fatalf("locked verification accepted correct code: %v", err)
	}
}

func TestConcurrentChangedVerificationShapeConflictsWithoutSecondSendPostgreSQL(t *testing.T) {
	database, cleanup := openPresalesPostgreSQL(t)
	defer cleanup()
	service, _, adapter, _ := presalesServiceFixture(t, database)
	service.challenges = newConsumeBarrierChallenge(2)
	inputs := []PresalesVerificationInput{
		{PresalesBindingInput: PresalesBindingInput{BrowserNonce: []byte("browser-shape-a-000000001"), Destination: []byte("shape-a@example.com"),
			DestinationKind: "email", Locale: "en-US", Operation: "create"}, ProofToken: "proof_shape_a",
			IdempotencyKey: "verification-shape-race-001", ClientIP: []byte("192.0.2.61")},
		{PresalesBindingInput: PresalesBindingInput{BrowserNonce: []byte("browser-shape-b-000000001"), Destination: []byte("shape-b@example.com"),
			DestinationKind: "email", Locale: "en-US", Operation: "create"}, ProofToken: "proof_shape_b",
			IdempotencyKey: "verification-shape-race-001", ClientIP: []byte("192.0.2.62")},
	}
	errorsFound := make(chan error, len(inputs))
	var wait sync.WaitGroup
	for index := range inputs {
		wait.Add(1)
		go func(input PresalesVerificationInput) {
			defer wait.Done()
			_, sendErr := service.SendPresalesVerificationCode(t.Context(), input)
			errorsFound <- sendErr
		}(inputs[index])
	}
	wait.Wait()
	close(errorsFound)
	var succeeded, conflicted int
	for sendErr := range errorsFound {
		if sendErr == nil {
			succeeded++
		} else if bizerr.Is(sendErr, CodeConflict) {
			conflicted++
		} else {
			t.Fatalf("unexpected changed-shape result: %v", sendErr)
		}
	}
	if succeeded != 1 || conflicted != 1 || adapter.deliveryCount() != 1 {
		t.Fatalf("changed shape crossed provider twice: succeeded=%d conflicted=%d deliveries=%d",
			succeeded, conflicted, adapter.deliveryCount())
	}
}

func TestPresalesPreparedProviderFailureStaysUnknownAndNeverResendsPostgreSQL(t *testing.T) {
	database, cleanup := openPresalesPostgreSQL(t)
	defer cleanup()
	service, _, adapter, _ := presalesServiceFixture(t, database)
	adapter.fail = true
	input := PresalesVerificationInput{PresalesBindingInput: PresalesBindingInput{BrowserNonce: []byte("browser-nonce-unknown-000001"),
		Destination: []byte("unknown@example.com"), DestinationKind: "email", Locale: "zh-CN", Operation: "create"},
		ProofToken: "proof_unknown", IdempotencyKey: "verification-unknown-001", ClientIP: []byte("198.51.100.9")}
	first, err := service.SendPresalesVerificationCode(t.Context(), input)
	if err != nil || first.DeliveryState != "result_unknown" {
		t.Fatalf("unknown send: %+v err=%v", first, err)
	}
	replayInput := input
	replayInput.PresalesBindingInput.BrowserNonce = []byte("browser-nonce-unknown-000001")
	replayInput.PresalesBindingInput.Destination = []byte("unknown@example.com")
	replayInput.ClientIP = []byte("198.51.100.9")
	replayed, err := service.SendPresalesVerificationCode(t.Context(), replayInput)
	if err != nil || replayed.VerificationNo != first.VerificationNo || adapter.deliveryCount() != 1 {
		t.Fatalf("unknown result replayed provider: %+v err=%v deliveries=%d", replayed, err, adapter.deliveryCount())
	}
}

func TestPresalesPreparedCrashReplayNeverCallsProviderPostgreSQL(t *testing.T) {
	database, cleanup := openPresalesPostgreSQL(t)
	defer cleanup()
	service, _, adapter, _ := presalesServiceFixture(t, database)
	input := PresalesVerificationInput{PresalesBindingInput: PresalesBindingInput{BrowserNonce: []byte("browser-nonce-crash-0000001"),
		Destination: []byte("crash@example.com"), DestinationKind: "email", Locale: "en-US", Operation: "create"},
		ProofToken: "proof_crash", IdempotencyKey: "verification-crash-001", ClientIP: []byte("192.0.2.19")}
	binding, err := service.preparePresalesBinding(t.Context(), clonePresalesBinding(input.PresalesBindingInput))
	if err != nil {
		t.Fatal(err)
	}
	defer clear(binding.destination)
	digests, err := service.hmacDigests(t.Context(), map[string][]byte{"idempotency": []byte(input.IdempotencyKey), "code": []byte("nver_crash:123456")})
	if err != nil {
		t.Fatal(err)
	}
	shape := digestText(binding.operation, binding.destinationKind, binding.destinationDigest, binding.browserDigest, binding.ticketDigest, binding.locale)
	if _, err = database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_contact_verification
 (tenant_id,verification_no,purpose_code,operation_code,destination_kind,destination_digest,browser_digest,ticket_digest,
  locale_code,template_no,template_version,request_shape_digest,idempotency_digest,code_digest,masked_destination,
  status_code,delivery_state,retry_after_at,expires_at)
 VALUES (0,'nver_crash','support.presales','create','email',?,?,?,?,
  'ntpl_presales_verification_email_en',1,?,?,?,'c***@example.com','prepared','prepared',
  CURRENT_TIMESTAMP+INTERVAL '60 seconds',CURRENT_TIMESTAMP+INTERVAL '10 minutes')`,
		binding.destinationDigest, binding.browserDigest, binding.ticketDigest, binding.locale, shape, digests["idempotency"], digests["code"]); err != nil {
		t.Fatal(err)
	}
	replayed, err := service.SendPresalesVerificationCode(t.Context(), input)
	if err != nil || replayed.DeliveryState != "result_unknown" || adapter.deliveryCount() != 0 {
		t.Fatalf("prepared crash replay crossed provider boundary: %+v err=%v deliveries=%d", replayed, err, adapter.deliveryCount())
	}
}

func TestHumanProofConsumptionRollsBackWhenRateLimitRejectsPostgreSQL(t *testing.T) {
	database, cleanup := openPresalesPostgreSQL(t)
	defer cleanup()
	service, _, adapter, _ := presalesServiceFixture(t, database)
	if _, err := database.Exec(t.Context(), `CREATE TABLE challenge_consumption_probe (proof_token text PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	probe := &transactionalChallengeStub{database: database}
	service.challenges = probe
	destination := []byte("limited@example.com")
	digests, err := service.hmacDigests(t.Context(), map[string][]byte{"rate_destination": destination})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(t.Context(), `INSERT INTO plugin_idc_notify_rate_window
 (tenant_id,dimension_code,subject_digest,window_seconds,window_started_at,counter,expires_at)
 VALUES (0,'destination',?,3600,date_trunc('hour',CURRENT_TIMESTAMP),5,date_trunc('hour',CURRENT_TIMESTAMP)+INTERVAL '1 hour')`,
		digests["rate_destination"]); err != nil {
		t.Fatal(err)
	}
	_, err = service.SendPresalesVerificationCode(t.Context(), PresalesVerificationInput{
		PresalesBindingInput: PresalesBindingInput{BrowserNonce: []byte("browser-nonce-rate-00000001"), Destination: destination,
			DestinationKind: "email", Locale: "en-US", Operation: "create"}, ProofToken: "proof_rate_limited",
		IdempotencyKey: "verification-rate-001", ClientIP: []byte("203.0.113.29"),
	})
	if err == nil || !bizerr.Is(err, CodeVerificationRateLimited) {
		t.Fatalf("expected rate limit, got %v", err)
	}
	count, queryErr := database.GetValue(t.Context(), "SELECT count(*) FROM challenge_consumption_probe")
	if queryErr != nil || count.Int() != 0 || probe.calls != 1 || adapter.deliveryCount() != 0 {
		t.Fatalf("proof consumption did not roll back: count=%d calls=%d deliveries=%d err=%v", count.Int(), probe.calls, adapter.deliveryCount(), queryErr)
	}
}

func openPresalesPostgreSQL(t *testing.T) (gdb.DB, func()) {
	t.Helper()
	database, cleanup := openNotificationServicePostgreSQL(t)
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		cleanup()
		t.Fatal("resolve presales test path")
	}
	pluginRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	for _, name := range []string{"002-notification-event-policy-materialization.sql", "004-notification-contact-verification.sql"} {
		content, err := os.ReadFile(filepath.Join(pluginRoot, "manifest", "sql", name))
		if err != nil {
			cleanup()
			t.Fatalf("read %s: %v", name, err)
		}
		if _, err = database.Exec(t.Context(), string(content)); err != nil {
			cleanup()
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	if _, err := database.Exec(t.Context(), `UPDATE plugin_idc_notify_channel
SET provider_code='presales_email_stub',status_code='enabled',configuration_generation=1,configuration_version=1,
    non_secret_configuration='{}'::jsonb,credential_secret_ref='sec_presales_email',credential_updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND channel_code='email'`); err != nil {
		cleanup()
		t.Fatalf("configure email fixture: %v", err)
	}
	return database, cleanup
}

func presalesServiceFixture(t *testing.T, database gdb.DB) (*serviceImpl, *presalesChallengeStub, *presalesAdapterStub, *presalesSupportStub) {
	t.Helper()
	handle, err := secretcap.NewHandle("sec_verification_test")
	if err != nil {
		t.Fatal(err)
	}
	challenge := &presalesChallengeStub{}
	adapter := &presalesAdapterStub{}
	support := &presalesSupportStub{}
	return &serviceImpl{database: database, challenges: challenge, support: support,
		verificationAccessor: presalesSecretAccessor{key: []byte("01234567890123456789012345678901")}, verificationHandle: handle,
		runtimeConfig: RuntimeConfig{PublicCustomerOrigin: "https://customer.example.com", SMSVerificationDailyLimit: 1000},
		adapters:      map[string]adapterBinding{"presales_email_stub": {descriptor: channelcap.Descriptor{ProviderCode: "presales_email_stub", Kind: channelcap.KindEmail}, adapter: adapter}},
		now:           time.Now}, challenge, adapter, support
}

func clonePresalesBinding(input PresalesBindingInput) PresalesBindingInput {
	input.BrowserNonce = append([]byte(nil), input.BrowserNonce...)
	input.Destination = append([]byte(nil), input.Destination...)
	return input
}

type presalesSecretAccessor struct{ key []byte }

func (a presalesSecretAccessor) Access(_ context.Context, request secretcap.AccessRequest, consume secretcap.Consumer) error {
	if request.Purpose != secretcap.Purpose("notify.verify") {
		return errors.New("wrong secret purpose")
	}
	key := append([]byte(nil), a.key...)
	defer clear(key)
	return consume(key)
}

type presalesChallengeStub struct {
	mu       sync.Mutex
	consumed int
}

type transactionalChallengeStub struct {
	database gdb.DB
	calls    int
}

type consumeBarrierChallengeStub struct {
	mu        sync.Mutex
	remaining int
	ready     chan struct{}
}

func newConsumeBarrierChallenge(count int) *consumeBarrierChallengeStub {
	return &consumeBarrierChallengeStub{remaining: count, ready: make(chan struct{})}
}

func (s *consumeBarrierChallengeStub) Create(context.Context, challengecap.CreateRequest) (*challengecap.Presentation, error) {
	return nil, errors.New("unused")
}

func (s *consumeBarrierChallengeStub) Complete(context.Context, challengecap.CompleteRequest) (*challengecap.Proof, error) {
	return nil, errors.New("unused")
}

func (s *consumeBarrierChallengeStub) Consume(context.Context, challengecap.ConsumeRequest) (*challengecap.ConsumeResult, error) {
	s.mu.Lock()
	s.remaining--
	if s.remaining == 0 {
		close(s.ready)
	}
	s.mu.Unlock()
	select {
	case <-s.ready:
		return &challengecap.ConsumeResult{ChallengeNo: "hc_barrier", ConsumedAt: time.Now()}, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("timed out waiting for concurrent challenge consumption")
	}
}

func (s *transactionalChallengeStub) Create(context.Context, challengecap.CreateRequest) (*challengecap.Presentation, error) {
	return nil, errors.New("unused")
}
func (s *transactionalChallengeStub) Complete(context.Context, challengecap.CompleteRequest) (*challengecap.Proof, error) {
	return nil, errors.New("unused")
}
func (s *transactionalChallengeStub) Consume(ctx context.Context, request challengecap.ConsumeRequest) (*challengecap.ConsumeResult, error) {
	s.calls++
	if _, err := s.database.Exec(ctx, "INSERT INTO challenge_consumption_probe (proof_token) VALUES (?)", request.Token); err != nil {
		return nil, err
	}
	return &challengecap.ConsumeResult{ChallengeNo: "hc_probe", ConsumedAt: time.Now()}, nil
}

func (s *presalesChallengeStub) Create(_ context.Context, request challengecap.CreateRequest) (*challengecap.Presentation, error) {
	return &challengecap.Presentation{ChallengeNo: "hc_presales", ProviderCode: "builtin", Kind: request.Kind,
		AccessiblePrompt: "Enter 7", ExpiresAt: time.Now().Add(2 * time.Minute)}, nil
}
func (s *presalesChallengeStub) Complete(_ context.Context, _ challengecap.CompleteRequest) (*challengecap.Proof, error) {
	return &challengecap.Proof{Token: "proof_presales", ExpiresAt: time.Now().Add(2 * time.Minute)}, nil
}
func (s *presalesChallengeStub) Consume(_ context.Context, _ challengecap.ConsumeRequest) (*challengecap.ConsumeResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.consumed++
	return &challengecap.ConsumeResult{ChallengeNo: "hc_presales", ConsumedAt: time.Now()}, nil
}
func (s *presalesChallengeStub) consumeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.consumed
}

type presalesAdapterStub struct {
	mu         sync.Mutex
	deliveries []channelcap.Delivery
	fail       bool
}

func (s *presalesAdapterStub) ValidateConfiguration(context.Context, map[string]string, map[string][]byte) error {
	return nil
}
func (s *presalesAdapterStub) ValidateDestination([]byte) error { return nil }
func (s *presalesAdapterStub) Deliver(_ context.Context, delivery channelcap.Delivery) (channelcap.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delivery.Destination = append([]byte(nil), delivery.Destination...)
	s.deliveries = append(s.deliveries, delivery)
	if s.fail {
		return channelcap.Result{}, errors.New("provider connection lost")
	}
	return channelcap.Result{Class: channelcap.ResultDelivered, CrossedExternalBorder: true}, nil
}
func (s *presalesAdapterStub) Query(context.Context, channelcap.Query) (channelcap.Result, error) {
	return channelcap.Result{}, nil
}
func (s *presalesAdapterStub) deliveryCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.deliveries)
}
func (s *presalesAdapterStub) lastBody() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deliveries[len(s.deliveries)-1].Body
}
func (s *presalesAdapterStub) lastCode() string {
	body := s.lastBody()
	match := regexp.MustCompile(`[0-9]{6}`).FindString(body)
	return match
}

type presalesSupportStub struct {
	mu                sync.Mutex
	creates           int
	lastCreateRequest supportcap.CreateAnonymousPresalesRequest
}

func (s *presalesSupportStub) CreateAnonymousPresales(_ context.Context, request supportcap.CreateAnonymousPresalesRequest) (*supportcap.CreateAnonymousPresalesResult, error) {
	s.mu.Lock()
	s.creates++
	s.lastCreateRequest = request
	s.lastCreateRequest.CustomFields = append([]supportcap.CustomFieldValue(nil), request.CustomFields...)
	for index := range s.lastCreateRequest.CustomFields {
		s.lastCreateRequest.CustomFields[index].OptionNos = append([]string(nil), request.CustomFields[index].OptionNos...)
	}
	s.mu.Unlock()
	return &supportcap.CreateAnonymousPresalesResult{TicketNo: "TKT_presales_test", AccessToken: strings.Repeat("A", 43), ExpiresAt: time.Now().Add(2 * time.Hour)}, nil
}
func (s *presalesSupportStub) RenewAnonymousPresalesAccess(_ context.Context, _ supportcap.RenewAnonymousPresalesAccessRequest) (*supportcap.RenewAnonymousPresalesAccessResult, error) {
	return &supportcap.RenewAnonymousPresalesAccessResult{TicketNo: "TKT_presales_test", AccessToken: strings.Repeat("B", 43), ExpiresAt: time.Now().Add(2 * time.Hour)}, nil
}
func (s *presalesSupportStub) createCount() int { s.mu.Lock(); defer s.mu.Unlock(); return s.creates }
func (s *presalesSupportStub) lastCreate() supportcap.CreateAnonymousPresalesRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastCreateRequest
}
