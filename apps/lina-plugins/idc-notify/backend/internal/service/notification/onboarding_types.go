package notification

type OnboardingBindingInput struct {
	BrowserNonce    []byte
	Destination     []byte
	DestinationKind string
	Locale          string
	Username        string
	CustomerType    string
	ReturnPath      string
}

type OnboardingHumanChallengeInput struct {
	OnboardingBindingInput
	PresentationKind string
}

type OnboardingHumanCompletionInput struct {
	OnboardingBindingInput
	ChallengeNo   string
	Answer        []byte
	ProviderToken []byte
}

type OnboardingVerificationInput struct {
	OnboardingBindingInput
	ProofToken     string
	IdempotencyKey string
	ClientIP       []byte
}

type OnboardingIntentInput struct {
	OnboardingBindingInput
	VerificationNo string
	Code           []byte
	IdempotencyKey string
}
