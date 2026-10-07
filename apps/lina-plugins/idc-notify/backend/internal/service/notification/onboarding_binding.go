package notification

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
	"lina-plugin-shared-services/backend/cap/challengecap"
)

const onboardingChallengePurpose = "customer.onboarding"

type onboardingBinding struct {
	destination       []byte
	destinationKind   string
	locale            string
	username          string
	customerType      string
	returnPath        string
	destinationDigest string
	browserDigest     string
	usernameSHA       string
	contactSHA        string
	browserSHA        string
	challengeBinding  string
}

func (s *serviceImpl) prepareOnboardingBinding(ctx context.Context, input OnboardingBindingInput) (*onboardingBinding, error) {
	username := strings.ToLower(strings.TrimSpace(input.Username))
	returnPath := strings.TrimSpace(input.ReturnPath)
	if returnPath == "" {
		returnPath = "/account"
	}
	if !browserNoncePattern.Match(input.BrowserNonce) || !validLocale(input.Locale) ||
		!utf8.ValidString(username) || len(username) < 2 || len(username) > 64 || strings.ContainsAny(username, "\r\n\t") ||
		input.CustomerType != "individual" ||
		len(returnPath) > 512 || !strings.HasPrefix(returnPath, "/account") || strings.HasPrefix(returnPath, "//") {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	destination, err := normalizePresalesDestination(input.DestinationKind, input.Destination)
	if err != nil {
		return nil, err
	}
	digests, err := s.hmacDigests(ctx, map[string][]byte{"destination": destination, "browser": input.BrowserNonce, "username": []byte(username)})
	if err != nil {
		clear(destination)
		return nil, err
	}
	challengeBinding := strings.Join([]string{digests["destination"], digests["browser"], digests["username"], input.CustomerType, returnPath}, ":")
	return &onboardingBinding{destination: destination, destinationKind: input.DestinationKind, locale: input.Locale,
		username: username, customerType: input.CustomerType, returnPath: returnPath,
		destinationDigest: digests["destination"], browserDigest: digests["browser"],
		usernameSHA: shaText(username), contactSHA: shaText(string(destination)), browserSHA: shaText(string(input.BrowserNonce)),
		challengeBinding: challengeBinding}, nil
}

func (s *serviceImpl) CreateOnboardingHumanChallenge(ctx context.Context, input OnboardingHumanChallengeInput) (*PresalesHumanChallenge, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	binding, err := s.prepareOnboardingBinding(ctx, input.OnboardingBindingInput)
	if err != nil {
		return nil, err
	}
	defer clear(binding.destination)
	kind := challengecap.PresentationKind(input.PresentationKind)
	if kind != challengecap.PresentationAccessible && kind != challengecap.PresentationImage {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	presentation, err := s.challenges.Create(ctx, challengecap.CreateRequest{Purpose: onboardingChallengePurpose, Binding: binding.challengeBinding, Kind: kind})
	if err != nil || presentation == nil {
		return nil, errorOrCode(err, CodeHumanVerificationUnavailable)
	}
	item := &PresalesHumanChallenge{ChallengeNo: presentation.ChallengeNo, ProviderCode: presentation.ProviderCode, ProviderAction: presentation.ProviderAction,
		Kind: string(presentation.Kind), AccessiblePrompt: presentation.AccessiblePrompt, SiteKey: presentation.SiteKey, ExpiresAt: presentation.ExpiresAt.UTC()}
	if len(presentation.ImageBytes) > 0 {
		if presentation.ImageMediaType != "image/png" {
			return nil, bizerr.NewCode(CodeHumanVerificationUnavailable)
		}
		item.ImageDataURL = "data:" + presentation.ImageMediaType + ";base64," + base64.StdEncoding.EncodeToString(presentation.ImageBytes)
	}
	return item, nil
}

func (s *serviceImpl) CompleteOnboardingHumanChallenge(ctx context.Context, input OnboardingHumanCompletionInput) (*PresalesHumanProof, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.Answer)
	defer clear(input.ProviderToken)
	binding, err := s.prepareOnboardingBinding(ctx, input.OnboardingBindingInput)
	if err != nil || input.ChallengeNo == "" || len(input.ChallengeNo) > 96 || len(input.Answer) > 256 || len(input.ProviderToken) > 4096 {
		return nil, errorOrCode(err, CodeInvalid)
	}
	defer clear(binding.destination)
	proof, err := s.challenges.Complete(ctx, challengecap.CompleteRequest{ChallengeNo: input.ChallengeNo, Purpose: onboardingChallengePurpose, Binding: binding.challengeBinding, Answer: string(input.Answer), ProviderToken: string(input.ProviderToken)})
	if err != nil || proof == nil {
		return nil, errorOrCode(err, CodeVerificationInvalid)
	}
	return &PresalesHumanProof{ProofToken: proof.Token, ExpiresAt: proof.ExpiresAt.UTC()}, nil
}

func shaText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
