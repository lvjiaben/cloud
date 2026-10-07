package notification

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/mail"
	"net/netip"
	"regexp"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-shared-services/backend/cap/challengecap"
)

const presalesChallengePurpose = "support.presales"

var (
	e164Pattern           = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
	browserNoncePattern   = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9_.:-]{15,127}$`)
	presalesTicketPattern = regexp.MustCompile(`^TKT_[A-Za-z0-9_-]{1,36}$`)
)

type presalesBinding struct {
	destination       []byte
	destinationKind   string
	locale            string
	operation         string
	ticketNo          string
	destinationDigest string
	browserDigest     string
	ticketDigest      string
	challengeBinding  string
}

func (s *serviceImpl) CreatePresalesHumanChallenge(ctx context.Context, input PresalesHumanChallengeInput) (*PresalesHumanChallenge, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	binding, err := s.preparePresalesBinding(ctx, input.PresalesBindingInput)
	if err != nil {
		return nil, err
	}
	kind := challengecap.PresentationKind(input.PresentationKind)
	if kind != challengecap.PresentationAccessible && kind != challengecap.PresentationImage {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	presentation, err := s.challenges.Create(ctx, challengecap.CreateRequest{
		Purpose: presalesChallengePurpose, Binding: binding.challengeBinding, Kind: kind,
	})
	if err != nil || presentation == nil {
		return nil, errorOrCode(err, CodeHumanVerificationUnavailable)
	}
	item := &PresalesHumanChallenge{ChallengeNo: presentation.ChallengeNo, ProviderCode: presentation.ProviderCode,
		ProviderAction: presentation.ProviderAction, Kind: string(presentation.Kind), AccessiblePrompt: presentation.AccessiblePrompt,
		SiteKey: presentation.SiteKey, ExpiresAt: presentation.ExpiresAt.UTC()}
	if len(presentation.ImageBytes) > 0 {
		if presentation.ImageMediaType != "image/png" {
			return nil, bizerr.NewCode(CodeHumanVerificationUnavailable)
		}
		item.ImageDataURL = "data:" + presentation.ImageMediaType + ";base64," + base64.StdEncoding.EncodeToString(presentation.ImageBytes)
	}
	return item, nil
}

func (s *serviceImpl) CompletePresalesHumanChallenge(ctx context.Context, input PresalesHumanCompletionInput) (*PresalesHumanProof, error) {
	defer clear(input.BrowserNonce)
	defer clear(input.Destination)
	defer clear(input.Answer)
	defer clear(input.ProviderToken)
	binding, err := s.preparePresalesBinding(ctx, input.PresalesBindingInput)
	if err != nil || input.ChallengeNo == "" || len(input.ChallengeNo) > 96 || len(input.Answer) > 256 || len(input.ProviderToken) > 4096 {
		return nil, errorOrCode(err, CodeInvalid)
	}
	proof, err := s.challenges.Complete(ctx, challengecap.CompleteRequest{
		ChallengeNo: input.ChallengeNo, Purpose: presalesChallengePurpose, Binding: binding.challengeBinding,
		Answer: string(input.Answer), ProviderToken: string(input.ProviderToken),
	})
	if err != nil || proof == nil {
		return nil, errorOrCode(err, CodeVerificationInvalid)
	}
	return &PresalesHumanProof{ProofToken: proof.Token, ExpiresAt: proof.ExpiresAt.UTC()}, nil
}

func (s *serviceImpl) preparePresalesBinding(ctx context.Context, input PresalesBindingInput) (*presalesBinding, error) {
	if !browserNoncePattern.Match(input.BrowserNonce) || !validLocale(input.Locale) ||
		(input.Operation != "create" && input.Operation != "renew") ||
		(input.Operation == "create" && input.TicketNo != "") ||
		(input.Operation == "renew" && !presalesTicketPattern.MatchString(input.TicketNo)) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	destination, err := normalizePresalesDestination(input.DestinationKind, input.Destination)
	if err != nil {
		return nil, err
	}
	digests, err := s.hmacDigests(ctx, map[string][]byte{
		"destination": destination,
		"browser":     input.BrowserNonce,
		"ticket":      []byte(input.TicketNo),
	})
	if err != nil {
		clear(destination)
		return nil, err
	}
	challengeBinding := strings.Join([]string{input.Operation, digests["destination"], digests["browser"], digests["ticket"]}, ":")
	return &presalesBinding{destination: destination, destinationKind: input.DestinationKind, locale: input.Locale,
		operation: input.Operation, ticketNo: input.TicketNo, destinationDigest: digests["destination"],
		browserDigest: digests["browser"], ticketDigest: digests["ticket"], challengeBinding: challengeBinding}, nil
}

func normalizePresalesDestination(kind string, raw []byte) ([]byte, error) {
	value := strings.TrimSpace(string(raw))
	switch kind {
	case "email":
		parsed, err := mail.ParseAddress(value)
		if err != nil || parsed.Address != value || len(value) > 320 || strings.ContainsAny(value, "\r\n") {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		at := strings.LastIndexByte(value, '@')
		if at <= 0 || at == len(value)-1 {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		value = value[:at] + "@" + strings.ToLower(value[at+1:])
	case "sms":
		if !e164Pattern.MatchString(value) {
			return nil, bizerr.NewCode(CodeInvalid)
		}
	default:
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return []byte(value), nil
}

func normalizeClientIP(raw []byte) ([]byte, error) {
	address, err := netip.ParseAddr(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return []byte(address.Unmap().String()), nil
}

func (s *serviceImpl) hmacDigests(ctx context.Context, values map[string][]byte) (map[string]string, error) {
	purpose, err := secretcap.NewPurpose("notify.verify")
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	request, err := secretcap.NewAccessRequest(s.verificationHandle, purpose)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	result := make(map[string]string, len(values))
	err = s.verificationAccessor.Access(ctx, request, func(plaintext []byte) error {
		for label, value := range values {
			mac := hmac.New(sha256.New, plaintext)
			_, _ = mac.Write([]byte("idc-notify-presales-v1\x00" + label + "\x00"))
			_, _ = mac.Write(value)
			result[label] = hex.EncodeToString(mac.Sum(nil))
		}
		return nil
	})
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return result, nil
}
