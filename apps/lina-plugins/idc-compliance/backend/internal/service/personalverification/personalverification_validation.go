// This file validates the supported PRC identity-card and minimum-age capability matrix.

package personalverification

import (
	"bytes"
	"encoding/hex"
	"io"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-compliance/backend/internal/provider/aliyunface"
)

var identityCardWeights = [...]int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
var identityCardChecks = [...]byte{'1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'}

type validatedIdentity struct {
	name         []byte
	number       []byte
	maskedName   string
	maskedNumber string
}

func validateStartInput(input StartInput, now time.Time) (validatedIdentity, error) {
	if input.CertType != aliyunface.CertTypeIdentityCard {
		return validatedIdentity{}, bizerr.NewCode(CodeUnsupportedDocument)
	}
	name := trimBytes(input.CertName)
	number := trimBytes(input.CertNo)
	if len(name) == 0 || len(name) > 128 || !validLegalName(name) ||
		utf8.RuneCount(name) > 64 || !validReturnURL(input.ReturnURL) {
		return validatedIdentity{}, bizerr.NewCode(CodeInvalid)
	}
	birth, ok := parseMainlandIdentityCard(number)
	if !ok {
		return validatedIdentity{}, bizerr.NewCode(CodeUnsupportedDocument)
	}
	if ageOn(now.UTC(), birth) < 14 {
		return validatedIdentity{}, bizerr.NewCode(CodeAgeUnsupported)
	}
	return validatedIdentity{
		name: append([]byte(nil), name...), number: append([]byte(nil), number...),
		maskedName: maskName(name), maskedNumber: maskIdentity(number),
	}, nil
}

func parseMainlandIdentityCard(number []byte) (time.Time, bool) {
	if len(number) != 18 {
		return time.Time{}, false
	}
	sum := 0
	for index := 0; index < 17; index++ {
		if number[index] < '0' || number[index] > '9' {
			return time.Time{}, false
		}
		sum += int(number[index]-'0') * identityCardWeights[index]
	}
	last := number[17]
	if last == 'x' {
		last = 'X'
	}
	if last != identityCardChecks[sum%11] {
		return time.Time{}, false
	}
	year := int(number[6]-'0')*1000 + int(number[7]-'0')*100 +
		int(number[8]-'0')*10 + int(number[9]-'0')
	month := int(number[10]-'0')*10 + int(number[11]-'0')
	day := int(number[12]-'0')*10 + int(number[13]-'0')
	birth := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if birth.Year() != year || int(birth.Month()) != month || birth.Day() != day {
		return time.Time{}, false
	}
	return birth, true
}

func ageOn(now, birth time.Time) int {
	age := now.Year() - birth.Year()
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		age--
	}
	return age
}

func maskName(name []byte) string {
	count := utf8.RuneCount(name)
	if count == 0 {
		return ""
	}
	if count == 1 {
		return "*"
	}
	first, _ := utf8.DecodeRune(name)
	return string(first) + strings.Repeat("*", count-1)
}

func validLegalName(name []byte) bool {
	if !utf8.Valid(name) {
		return false
	}
	for remaining := name; len(remaining) > 0; {
		character, size := utf8.DecodeRune(remaining)
		if size == 0 || unicode.IsControl(character) {
			return false
		}
		remaining = remaining[size:]
	}
	return true
}

func maskIdentity(number []byte) string {
	if len(number) < 8 {
		return ""
	}
	return string(number[:3]) + strings.Repeat("*", len(number)-7) + string(number[len(number)-4:])
}

func validReturnURL(value string) bool {
	if value == "" || value != strings.TrimSpace(value) || len(value) > 2048 {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil &&
		parsed.Fragment == ""
}

func randomHex(random io.Reader, bytes int) (string, error) {
	buffer := make([]byte, bytes)
	if _, err := io.ReadFull(random, buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func trimBytes(value []byte) []byte {
	return bytes.TrimSpace(value)
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
