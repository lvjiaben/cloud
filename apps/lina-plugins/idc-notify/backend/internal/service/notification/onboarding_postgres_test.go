package notification

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/cap/onboardingcap"
)

func TestCustomerOnboardingVerificationPostgreSQL(t *testing.T) {
	database, cleanup := openPresalesPostgreSQL(t)
	defer cleanup()
	_, file, _, _ := runtime.Caller(0)
	content, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "manifest", "sql", "008-customer-onboarding-verification.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err = database.Exec(t.Context(), string(content)); err != nil {
			t.Fatal(err)
		}
	}
	service, challenge, adapter, _ := presalesServiceFixture(t, database)
	onboarding := &onboardingStub{}
	service.onboarding = onboarding
	binding := OnboardingBindingInput{BrowserNonce: []byte("browser-nonce-onboarding-0001"), Destination: []byte("Buyer@Example.com"), DestinationKind: "email", Locale: "zh-CN", Username: "Buyer01", CustomerType: "individual", ReturnPath: "/account"}
	presentation, err := service.CreateOnboardingHumanChallenge(t.Context(), OnboardingHumanChallengeInput{OnboardingBindingInput: cloneOnboardingBinding(binding), PresentationKind: "accessible"})
	if err != nil || presentation.AccessiblePrompt == "" {
		t.Fatalf("presentation=%+v err=%v", presentation, err)
	}
	proof, err := service.CompleteOnboardingHumanChallenge(t.Context(), OnboardingHumanCompletionInput{OnboardingBindingInput: cloneOnboardingBinding(binding), ChallengeNo: presentation.ChallengeNo, Answer: []byte("7")})
	if err != nil || proof.ProofToken == "" {
		t.Fatalf("proof=%+v err=%v", proof, err)
	}
	verification, err := service.SendOnboardingVerificationCode(t.Context(), OnboardingVerificationInput{OnboardingBindingInput: cloneOnboardingBinding(binding), ProofToken: proof.ProofToken, IdempotencyKey: "onboarding-code-0001", ClientIP: []byte("127.0.0.1")})
	if err != nil || verification.VerificationNo == "" || challenge.consumeCount() != 1 || adapter.deliveryCount() != 1 {
		t.Fatalf("verification=%+v consumed=%d deliveries=%d err=%v", verification, challenge.consumeCount(), adapter.deliveryCount(), err)
	}
	intentInput := OnboardingIntentInput{OnboardingBindingInput: cloneOnboardingBinding(binding), VerificationNo: verification.VerificationNo, Code: []byte(adapter.lastCode()), IdempotencyKey: "onboarding-intent-0001"}
	intent, err := service.CreateVerifiedOnboardingIntent(t.Context(), intentInput)
	if err != nil || intent.Token == "" || onboarding.callCount() != 1 {
		t.Fatalf("intent=%+v calls=%d err=%v", intent, onboarding.callCount(), err)
	}
	replayInput := intentInput
	replayInput.OnboardingBindingInput = cloneOnboardingBinding(binding)
	replayInput.Code = []byte(adapter.lastCode())
	replay, err := service.CreateVerifiedOnboardingIntent(t.Context(), replayInput)
	if err != nil || !replay.Reused || replay.Token != intent.Token || onboarding.callCount() != 2 {
		t.Fatalf("replay=%+v calls=%d err=%v", replay, onboarding.callCount(), err)
	}
	request := onboarding.lastRequest()
	if request.UsernameDigest != shaText("buyer01") || request.ContactDigest != shaText("Buyer@example.com") || request.BrowserDigest != shaText("browser-nonce-onboarding-0001") || request.ContactChannel != "email" {
		t.Fatalf("unexpected source request: %+v", request)
	}
}

func cloneOnboardingBinding(input OnboardingBindingInput) OnboardingBindingInput {
	input.BrowserNonce = append([]byte(nil), input.BrowserNonce...)
	input.Destination = append([]byte(nil), input.Destination...)
	return input
}

type onboardingStub struct {
	mu      sync.Mutex
	calls   int
	request onboardingcap.CreateVerifiedIntentRequest
}

func (s *onboardingStub) CreateVerifiedIntent(_ context.Context, request onboardingcap.CreateVerifiedIntentRequest) (*onboardingcap.VerifiedIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.request = request
	return &onboardingcap.VerifiedIntent{IntentNo: "ONB_" + strings.Repeat("a", 32), Token: strings.Repeat("B", 43), ExpiresAt: time.Now().Add(30 * time.Minute), Reused: s.calls > 1}, nil
}
func (s *onboardingStub) callCount() int { s.mu.Lock(); defer s.mu.Unlock(); return s.calls }
func (s *onboardingStub) lastRequest() onboardingcap.CreateVerifiedIntentRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.request
}
