// This file normalizes trusted proxy chains and stores only keyed request digests.

package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/netip"
	"strings"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
)

const (
	maximumForwardedForBytes = 4096
	maximumForwardedHops     = 32
)

type normalizedRequestEvidence struct {
	requestID       string
	traceID         string
	sourceIP        string
	userAgent       string
	ipDigest        string
	userAgentDigest string
}

// normalizeRequestEvidence validates metadata and retains canonical source
// values only in request memory until a specific replay-key version is chosen.
func (s *serviceImpl) normalizeRequestEvidence(input RequestEvidence) (normalizedRequestEvidence, error) {
	input.RemoteAddress = strings.TrimSpace(input.RemoteAddress)
	input.ForwardedFor = strings.TrimSpace(input.ForwardedFor)
	input.UserAgent = strings.TrimSpace(input.UserAgent)
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.TraceID = strings.TrimSpace(input.TraceID)
	if !validReferenceText(input.RequestID, maximumIdentifierRunes, false) ||
		!validReferenceText(input.TraceID, maximumIdentifierRunes, false) ||
		len(input.RemoteAddress) > 256 ||
		len(input.ForwardedFor) > maximumForwardedForBytes ||
		len(input.UserAgent) > maximumUserAgentBytes ||
		!utf8.ValidString(input.UserAgent) {
		return normalizedRequestEvidence{}, bizerr.NewCode(CodeInvalid)
	}
	result := normalizedRequestEvidence{requestID: input.RequestID, traceID: input.TraceID}
	if input.RemoteAddress == "" {
		if input.ForwardedFor != "" {
			return normalizedRequestEvidence{}, bizerr.NewCode(CodeInvalid)
		}
	} else {
		remote, err := parseRemoteAddress(input.RemoteAddress)
		if err != nil {
			return normalizedRequestEvidence{}, bizerr.NewCode(CodeInvalid)
		}
		client, err := s.resolveClientAddress(remote, input.ForwardedFor)
		if err != nil {
			return normalizedRequestEvidence{}, err
		}
		result.sourceIP = client.String()
	}
	if input.UserAgent != "" {
		normalizedUserAgent := strings.Join(strings.Fields(input.UserAgent), " ")
		if normalizedUserAgent == "" {
			return normalizedRequestEvidence{}, bizerr.NewCode(CodeInvalid)
		}
		result.userAgent = normalizedUserAgent
	}
	return result, nil
}

// requestEvidenceForKey derives persisted request digests for the event key
// version while retaining canonical raw values only in request memory.
func (s *serviceImpl) requestEvidenceForKey(
	input normalizedRequestEvidence,
	key []byte,
) (normalizedRequestEvidence, error) {
	result := input
	var err error
	if result.sourceIP != "" {
		result.ipDigest, err = keyedDigestWithKey(key, "source-ip", result.sourceIP)
		if err != nil {
			return normalizedRequestEvidence{}, err
		}
	} else {
		result.ipDigest = ""
	}
	if result.userAgent != "" {
		result.userAgentDigest, err = keyedDigestWithKey(key, "user-agent", result.userAgent)
		if err != nil {
			return normalizedRequestEvidence{}, err
		}
	} else {
		result.userAgentDigest = ""
	}
	return result, nil
}

// resolveClientAddress trusts forwarded hops only when the immediate peer is trusted.
func (s *serviceImpl) resolveClientAddress(remote netip.Addr, forwardedFor string) (netip.Addr, error) {
	current := remote.Unmap()
	if forwardedFor == "" || !s.isTrustedProxy(current) {
		return current, nil
	}
	parts := strings.Split(forwardedFor, ",")
	if len(parts) > maximumForwardedHops {
		return netip.Addr{}, bizerr.NewCode(CodeInvalid)
	}
	chain := make([]netip.Addr, 0, len(parts))
	for _, raw := range parts {
		value := strings.TrimSpace(raw)
		address, err := netip.ParseAddr(value)
		if err != nil {
			return netip.Addr{}, bizerr.NewCode(CodeInvalid)
		}
		chain = append(chain, address.Unmap())
	}
	for index := len(chain) - 1; index >= 0; index-- {
		if !s.isTrustedProxy(current) {
			break
		}
		current = chain[index]
	}
	return current, nil
}

// isTrustedProxy reports whether one canonical peer belongs to configured trust roots.
func (s *serviceImpl) isTrustedProxy(address netip.Addr) bool {
	for _, prefix := range s.trustedProxies {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

// parseRemoteAddress accepts canonical IP or host:port transport forms.
func parseRemoteAddress(value string) (netip.Addr, error) {
	if address, err := netip.ParseAddr(value); err == nil {
		return address.Unmap(), nil
	}
	if addressPort, err := netip.ParseAddrPort(value); err == nil {
		return addressPort.Addr().Unmap(), nil
	}
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		return netip.Addr{}, err
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, err
	}
	return address.Unmap(), nil
}

// keyedDigestWithKey returns a domain-separated HMAC-SHA256 digest for one
// explicit versioned key.
func keyedDigestWithKey(key []byte, domain string, value string) (string, error) {
	digest := hmac.New(sha256.New, key)
	for _, content := range [][]byte{[]byte(domain), {0}, []byte(value)} {
		if _, err := digest.Write(content); err != nil {
			return "", bizerr.WrapCode(err, CodeUnavailable)
		}
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
