// Package tencentess owns the installation-local Tencent eSign API boundary.
package tencentess

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
)

const (
	// APIVersion is the only Tencent eSign API version used by this adapter.
	APIVersion = "2021-05-26"
	// OfficialEndpoint is fixed so configuration cannot redirect secrets or traffic.
	OfficialEndpoint = "essbasic.tencentcloudapi.com"
	// MaximumConfigFileBytes bounds the private installation-owned account file.
	MaximumConfigFileBytes int64 = 16 << 10
)

// Capability identifies one independently gated Tencent eSign ability.
type Capability string

const (
	// CapabilityTemplateRead enables bounded template reads.
	CapabilityTemplateRead Capability = "template.read"
	// CapabilityFlowCreate enables guarded contract creation.
	CapabilityFlowCreate Capability = "flow.create"
	// CapabilityFlowRelease enables guarded release-agreement creation.
	CapabilityFlowRelease Capability = "flow.release"
	// CapabilitySignURL enables short-lived signing-link creation.
	CapabilitySignURL Capability = "sign_url.create"
	// CapabilityFlowQuery enables bounded active flow queries.
	CapabilityFlowQuery Capability = "flow.query"
	// CapabilityDownload enables authorized flow download links.
	CapabilityDownload Capability = "flow.download"
	// CapabilityEvidenceReport enables the buyer-purchased signing-report service.
	CapabilityEvidenceReport Capability = "evidence_report.create"
	// CapabilityCallback enables the buyer-owned encrypted callback endpoint.
	CapabilityCallback Capability = "callback.receive"
	// CapabilityAutoSign enables enterprise auto-sign authorization checks.
	CapabilityAutoSign Capability = "auto_sign.enterprise"
	// CapabilityUsageRead enables read-only bill-usage queries.
	CapabilityUsageRead Capability = "usage.read"
	// CapabilityAccountBill enables read-only account-bill queries.
	CapabilityAccountBill Capability = "account_bill.read"
)

var supportedCapabilities = map[Capability]struct{}{
	CapabilityTemplateRead:   {},
	CapabilityFlowCreate:     {},
	CapabilityFlowRelease:    {},
	CapabilitySignURL:        {},
	CapabilityFlowQuery:      {},
	CapabilityDownload:       {},
	CapabilityEvidenceReport: {},
	CapabilityCallback:       {},
	CapabilityAutoSign:       {},
	CapabilityUsageRead:      {},
	CapabilityAccountBill:    {},
}

// Config contains one buyer installation's own Tencent application identity.
// The file is deployment-owned and is never accepted from a browser request.
type Config struct {
	// APIVersion pins the official Tencent contract version.
	APIVersion string `json:"apiVersion"`
	// SecretID identifies this installation's buyer-owned Tencent application.
	SecretID string `json:"secretId"`
	// SecretKey authenticates this installation's buyer-owned Tencent application.
	SecretKey string `json:"secretKey"`
	// AppID identifies the buyer-owned Tencent application.
	AppID string `json:"appId"`
	// CallbackVerifyToken authenticates the exact raw Tencent callback body.
	CallbackVerifyToken string `json:"callbackVerifyToken,omitempty"`
	// CallbackURLKey decrypts the encrypted Tencent callback body.
	CallbackURLKey string `json:"callbackUrlKey,omitempty"`
	// Capabilities is the closed deployment-owned capability allowlist.
	Capabilities []Capability `json:"capabilities"`
}

// LoadConfigFile reads one bounded private Tencent account file.
func LoadConfigFile(path string) (result Config, returnError error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open Tencent eSign configuration: %w", err)
	}
	defer func() {
		if closeError := file.Close(); closeError != nil && returnError == nil {
			result.Clear()
			result = Config{}
			returnError = fmt.Errorf("close Tencent eSign configuration: %w", closeError)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return Config{}, fmt.Errorf("inspect Tencent eSign configuration: %w", err)
	}
	if !info.Mode().IsRegular() {
		return Config{}, errors.New("tencent eSign configuration is not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return Config{}, errors.New("tencent eSign configuration permissions are not private")
	}
	payload, err := io.ReadAll(io.LimitReader(file, MaximumConfigFileBytes+1))
	if err != nil {
		return Config{}, fmt.Errorf("read Tencent eSign configuration: %w", err)
	}
	defer clearBytes(payload)
	if int64(len(payload)) > MaximumConfigFileBytes {
		return Config{}, errors.New("tencent eSign configuration is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var config Config
	if err = decoder.Decode(&config); err != nil {
		return Config{}, errors.New("tencent eSign configuration JSON is invalid")
	}
	var trailing any
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		config.Clear()
		return Config{}, errors.New("tencent eSign configuration must contain one JSON object")
	}
	if err = config.Validate(); err != nil {
		config.Clear()
		return Config{}, err
	}
	return config, nil
}

// Validate enforces the fixed API version, bounded identifiers, and closed capability set.
func (c Config) Validate() error {
	if c.APIVersion != APIVersion ||
		!validOpaque(c.SecretID, 1, 128) || !validOpaque(c.SecretKey, 1, 256) ||
		!validOpaque(c.AppID, 1, 64) || len(c.Capabilities) == 0 ||
		len(c.Capabilities) > len(supportedCapabilities) {
		return errors.New("tencent eSign configuration is invalid")
	}
	seen := make(map[Capability]struct{}, len(c.Capabilities))
	for _, capability := range c.Capabilities {
		if _, supported := supportedCapabilities[capability]; !supported {
			return errors.New("tencent eSign configuration contains an unsupported capability")
		}
		if _, duplicate := seen[capability]; duplicate {
			return errors.New("tencent eSign configuration contains a duplicate capability")
		}
		seen[capability] = struct{}{}
	}
	_, callbackEnabled := seen[CapabilityCallback]
	callbackConfigured := c.CallbackVerifyToken != "" || c.CallbackURLKey != ""
	if callbackEnabled != callbackConfigured || (callbackEnabled &&
		(!validOpaque(c.CallbackVerifyToken, 1, 256) ||
			len([]byte(c.CallbackURLKey)) != 32 || !validOpaque(c.CallbackURLKey, 32, 32))) {
		return errors.New("tencent eSign callback configuration is invalid")
	}
	return nil
}

// HasCapability reports only deployment intent; runtime checks remain authoritative.
func (c Config) HasCapability(capability Capability) bool {
	for _, configured := range c.Capabilities {
		if configured == capability {
			return true
		}
	}
	return false
}

// SortedCapabilities returns a deterministic copy for status projection and tests.
func (c Config) SortedCapabilities() []Capability {
	result := append([]Capability(nil), c.Capabilities...)
	sort.Slice(result, func(left, right int) bool { return result[left] < result[right] })
	return result
}

// Clear releases direct references to installation-owned credentials after assembly.
func (c *Config) Clear() {
	if c == nil {
		return
	}
	c.SecretID = ""
	c.SecretKey = ""
	c.AppID = ""
	c.CallbackVerifyToken = ""
	c.CallbackURLKey = ""
	clear(c.Capabilities)
	c.Capabilities = nil
}

func validOpaque(value string, minimum, maximum int) bool {
	return len(value) >= minimum && len(value) <= maximum && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "\r\n\t\x00")
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
