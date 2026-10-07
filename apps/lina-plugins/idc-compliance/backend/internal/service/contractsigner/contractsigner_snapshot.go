package contractsigner

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/secretcap"
)

const profileSnapshotVersion byte = 1

type privateProfile struct {
	name   []byte
	certNo []byte
	phone  []byte
}

func validatePrivateProfile(name, certNo, phone []byte) (*privateProfile, error) {
	name = bytes.TrimSpace(name)
	certNo = bytes.TrimSpace(certNo)
	phone = bytes.TrimSpace(phone)
	if len(name) == 0 || len(name) > 128 || utf8.RuneCount(name) > 64 || !utf8.Valid(name) ||
		!validIdentityCard(certNo) || !validPhone(phone) {
		return nil, gerror.New("contract signer private profile is invalid")
	}
	for remaining := name; len(remaining) > 0; {
		character, size := utf8.DecodeRune(remaining)
		if size == 0 || unicode.IsControl(character) {
			return nil, gerror.New("contract signer private profile is invalid")
		}
		remaining = remaining[size:]
	}
	return &privateProfile{name: append([]byte(nil), name...), certNo: append([]byte(nil), certNo...), phone: append([]byte(nil), phone...)}, nil
}

func (p *privateProfile) clear() {
	if p == nil {
		return
	}
	clear(p.name)
	clear(p.certNo)
	clear(p.phone)
}

func encodeProfile(challenge []byte, profile *privateProfile) []byte {
	if len(challenge) != 32 || profile == nil {
		return nil
	}
	values := [][]byte{challenge, profile.name, profile.certNo, profile.phone}
	total := 1
	for _, value := range values {
		total += 2 + len(value)
	}
	payload := make([]byte, total)
	payload[0] = profileSnapshotVersion
	offset := 1
	for _, value := range values {
		binary.BigEndian.PutUint16(payload[offset:offset+2], uint16(len(value)))
		offset += 2
		copy(payload[offset:offset+len(value)], value)
		offset += len(value)
	}
	return payload
}

func decodeProfile(payload []byte) ([]byte, *privateProfile, error) {
	if len(payload) < 1 || payload[0] != profileSnapshotVersion {
		return nil, nil, gerror.New("contract signer profile snapshot is invalid")
	}
	offset := 1
	values := make([][]byte, 0, 4)
	for index := 0; index < 4; index++ {
		if offset+2 > len(payload) {
			return nil, nil, gerror.New("contract signer profile snapshot is invalid")
		}
		size := int(binary.BigEndian.Uint16(payload[offset : offset+2]))
		offset += 2
		if size <= 0 || offset+size > len(payload) {
			return nil, nil, gerror.New("contract signer profile snapshot is invalid")
		}
		values = append(values, payload[offset:offset+size])
		offset += size
	}
	if offset != len(payload) || len(values[0]) != 32 {
		return nil, nil, gerror.New("contract signer profile snapshot is invalid")
	}
	profile, err := validatePrivateProfile(values[1], values[2], values[3])
	if err != nil {
		return nil, nil, err
	}
	return append([]byte(nil), values[0]...), profile, nil
}

func profileDigest(payload []byte) string {
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func profileProducer(payload []byte) secretcap.PlaintextProducer {
	return func(consume secretcap.Consumer) error {
		if consume == nil {
			return gerror.New("contract signer profile consumer is required")
		}
		return consume(payload)
	}
}

func (s *serviceImpl) accessProfile(ctx context.Context, row signerRow, consume func(*privateProfile) error) error {
	handle, err := secretcap.NewHandle(row.ProfileSnapshotRef)
	if err != nil {
		return gerror.Wrap(err, "open contract signer profile snapshot")
	}
	purpose, err := secretcap.NewPurpose("compliance.contract-signer-profile")
	if err != nil {
		return err
	}
	request, err := secretcap.NewAccessRequest(handle, purpose)
	if err != nil {
		return err
	}
	return s.profileAccessor.Access(ctx, request, func(payload []byte) error {
		if subtle.ConstantTimeCompare([]byte(profileDigest(payload)), []byte(row.ProfileSnapshotSHA256)) != 1 {
			return gerror.New("contract signer profile snapshot integrity check failed")
		}
		challenge, profile, decodeErr := decodeProfile(payload)
		clear(challenge)
		if decodeErr != nil {
			return decodeErr
		}
		defer profile.clear()
		return consume(profile)
	})
}

func validPhone(value []byte) bool {
	if len(value) < 9 || len(value) > 16 || value[0] != '+' || value[1] < '1' || value[1] > '9' {
		return false
	}
	for _, digit := range value[2:] {
		if digit < '0' || digit > '9' {
			return false
		}
	}
	return true
}

var identityWeights = [...]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
var identityChecks = [...]byte{'1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'}

func validIdentityCard(value []byte) bool {
	if len(value) != 18 {
		return false
	}
	sum := 0
	for index := 0; index < 17; index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
		sum += int(value[index]-'0') * identityWeights[index]
	}
	last := value[17]
	if last == 'x' {
		last = 'X'
	}
	return last == identityChecks[sum%11]
}

func maskName(value []byte) string {
	runes := []rune(string(value))
	if len(runes) == 0 {
		return ""
	}
	if len(runes) == 1 {
		return "*"
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-1)
}

func maskIdentity(value []byte) string {
	return string(value[:3]) + strings.Repeat("*", len(value)-7) + string(value[len(value)-4:])
}
func maskPhone(value []byte) string { return string(value[:3]) + "****" + string(value[len(value)-4:]) }
