// This file verifies buyer-owned Tencent eSign encrypted callbacks before they
// enter the provider-neutral installation-local convergence transaction.

package tencentess

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"regexp"
	"strings"
	"time"

	essbasic "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/essbasic/v20210526"

	"lina-platform/idc-compliance-gateway/internal/suppliercallback"
)

const (
	callbackMessageVersion = "ThirdPartyApp"
	callbackMessageType    = "FlowStatusChange"
	callbackFutureSkew     = 5 * time.Minute
	callbackEvidenceMagic  = "TCB1"
)

var callbackSignaturePattern = regexp.MustCompile(`^sha256=[0-9a-f]{64}$`)

type callbackReferenceResolver interface {
	ResolveExternalReference(context.Context, string, []byte) (suppliercallback.ResolvedReference, error)
}

// CallbackVerifier owns copied callback secrets for one buyer installation.
type CallbackVerifier struct {
	client      *Client
	references  callbackReferenceResolver
	verifyToken []byte
	callbackKey []byte
	now         func() time.Time
}

type encryptedCallback struct {
	Encrypt string `json:"encrypt"`
}

type callbackEnvelope struct {
	MessageID      string          `json:"MsgId"`
	MessageType    string          `json:"MsgType"`
	MessageVersion string          `json:"MsgVersion"`
	MessageData    json.RawMessage `json:"MsgData"`
}

type flowStatusCallback struct {
	ApplicationID           string          `json:"ApplicationId"`
	ProxyOrganizationOpenID string          `json:"ProxyOrganizationOpenId"`
	ProxyOperatorOpenID     string          `json:"ProxyOperatorOpenId"`
	CustomerData            string          `json:"CustomerData"`
	FlowID                  string          `json:"FlowId"`
	TemplateID              string          `json:"TemplateId"`
	FlowName                string          `json:"FlowName"`
	FlowStatus              string          `json:"FlowStatus"`
	FlowMessage             string          `json:"FlowMessage"`
	CreateOn                int64           `json:"CreateOn"`
	Deadline                int64           `json:"Deadline"`
	FlowApproverInfo        json.RawMessage `json:"FlowApproverInfo"`
	OccurTime               int64           `json:"OccurTime"`
	ReleaseFlowID           string          `json:"ReleaseFlowId"`
}

// NewCallbackVerifier copies the deployment-owned callback secrets. Both
// encryption and signature verification are mandatory when the route is enabled.
func NewCallbackVerifier(
	config Config,
	client *Client,
	references callbackReferenceResolver,
) (*CallbackVerifier, error) {
	if err := config.Validate(); err != nil || !config.HasCapability(CapabilityCallback) ||
		client == nil || !client.Configured() || references == nil {
		return nil, &Error{Code: ErrorNotConfigured, cause: err}
	}
	result := &CallbackVerifier{
		client: client, references: references,
		verifyToken: []byte(config.CallbackVerifyToken),
		callbackKey: []byte(config.CallbackURLKey),
		now:         time.Now,
	}
	if len(result.verifyToken) == 0 || len(result.callbackKey) != 32 {
		result.Close()
		return nil, &Error{Code: ErrorNotConfigured}
	}
	return result, nil
}

// Close clears copied callback secrets.
func (v *CallbackVerifier) Close() {
	if v == nil {
		return
	}
	clear(v.verifyToken)
	clear(v.callbackKey)
	v.verifyToken = nil
	v.callbackKey = nil
	v.references = nil
	v.client = nil
}

// Verify authenticates the exact raw request before decrypting or resolving a route.
func (v *CallbackVerifier) Verify(
	ctx context.Context,
	input suppliercallback.RawCallback,
) (suppliercallback.VerifiedEvent, error) {
	if v == nil || v.client == nil || v.references == nil || len(v.verifyToken) == 0 ||
		len(v.callbackKey) != 32 || ctx == nil || ctx.Err() != nil ||
		input.Provider != suppliercallback.ProviderTencentESSBasic ||
		!callbackSignaturePattern.Match(input.Signature) {
		return suppliercallback.VerifiedEvent{}, suppliercallback.ErrRejected
	}
	expectedMAC := hmac.New(sha256.New, v.verifyToken)
	_, _ = expectedMAC.Write(input.Body)
	expected := make([]byte, len("sha256=")+hex.EncodedLen(sha256.Size))
	copy(expected, "sha256=")
	hex.Encode(expected[len("sha256="):], expectedMAC.Sum(nil))
	validSignature := hmac.Equal(expected, input.Signature)
	clear(expected)
	if !validSignature {
		return suppliercallback.VerifiedEvent{}, suppliercallback.ErrRejected
	}
	plaintext, err := decryptCallback(input.Body, v.callbackKey)
	if err != nil {
		return suppliercallback.VerifiedEvent{}, suppliercallback.ErrRejected
	}
	defer clear(plaintext)
	envelope, data, observedAt, status, err := v.validateCallback(ctx, plaintext)
	if err != nil {
		return suppliercallback.VerifiedEvent{}, suppliercallback.ErrRejected
	}
	reference := []byte(data.FlowID)
	resolved, err := v.references.ResolveExternalReference(
		ctx, suppliercallback.ProviderTencentESSBasic, reference,
	)
	if err != nil || (resolved.OperationScope != OperationFlowCreate &&
		resolved.OperationScope != OperationFlowRelease) {
		return suppliercallback.VerifiedEvent{}, suppliercallback.ErrRejected
	}
	evidence, err := packCallbackEvidence(input.Body, input.Signature, plaintext)
	if err != nil {
		return suppliercallback.VerifiedEvent{}, suppliercallback.ErrRejected
	}
	return suppliercallback.VerifiedEvent{
		RouteNo: resolved.RouteNo, Provider: suppliercallback.ProviderTencentESSBasic,
		OperationScope: resolved.OperationScope, ExternalEventID: []byte(envelope.MessageID),
		Status: status, ProviderStatus: data.FlowStatus,
		EvidenceFrame: evidence, SupplierObservedAt: observedAt,
	}, nil
}

func (v *CallbackVerifier) validateCallback(
	ctx context.Context,
	plaintext []byte,
) (callbackEnvelope, flowStatusCallback, time.Time, suppliercallback.Status, error) {
	if rejectDuplicateJSON(plaintext) != nil {
		return callbackEnvelope{}, flowStatusCallback{}, time.Time{}, "", errors.New("duplicate callback field")
	}
	var envelope callbackEnvelope
	if decodeOneJSON(plaintext, &envelope, true) != nil ||
		!flowIDPattern.MatchString(envelope.MessageID) ||
		envelope.MessageType != callbackMessageType ||
		envelope.MessageVersion != callbackMessageVersion || len(envelope.MessageData) == 0 {
		return callbackEnvelope{}, flowStatusCallback{}, time.Time{}, "", errors.New("invalid callback envelope")
	}
	var data flowStatusCallback
	if decodeOneJSON(envelope.MessageData, &data, false) != nil ||
		!flowIDPattern.MatchString(data.FlowID) || data.ApplicationID != v.client.appID ||
		!validCallbackText(data.ProxyOrganizationOpenID, 128) ||
		(data.ProxyOperatorOpenID != "" && !validCallbackText(data.ProxyOperatorOpenID, 128)) ||
		(data.TemplateID != "" && !flowIDPattern.MatchString(data.TemplateID)) ||
		data.CreateOn <= 0 || data.Deadline < data.CreateOn || data.OccurTime <= 0 ||
		(data.ReleaseFlowID != "" && !flowIDPattern.MatchString(data.ReleaseFlowID)) {
		return callbackEnvelope{}, flowStatusCallback{}, time.Time{}, "", errors.New("invalid flow callback")
	}
	observedAt := time.Unix(data.OccurTime, 0).UTC()
	if observedAt.After(v.now().UTC().Add(callbackFutureSkew)) {
		return callbackEnvelope{}, flowStatusCallback{}, time.Time{}, "", errors.New("future flow callback")
	}
	status, ok := normalizeCallbackStatus(data.FlowStatus)
	if !ok || (data.FlowStatus == "RELIEVED" && data.ReleaseFlowID == "") {
		return callbackEnvelope{}, flowStatusCallback{}, time.Time{}, "", errors.New("unknown flow status")
	}
	subjectMatches := false
	err := v.client.withAgent(ctx, func(agent *essbasic.Agent) error {
		subjectMatches = agent != nil && stringValue(agent.AppId) == data.ApplicationID &&
			stringValue(agent.ProxyOrganizationOpenId) == data.ProxyOrganizationOpenID
		return nil
	})
	if err != nil || !subjectMatches {
		return callbackEnvelope{}, flowStatusCallback{}, time.Time{}, "", errors.New("callback subject mismatch")
	}
	return envelope, data, observedAt, status, nil
}

func decryptCallback(body []byte, key []byte) ([]byte, error) {
	if rejectDuplicateJSON(body) != nil {
		return nil, errors.New("invalid encrypted callback")
	}
	var wrapper encryptedCallback
	if decodeOneJSON(body, &wrapper, true) != nil || wrapper.Encrypt == "" {
		return nil, errors.New("invalid encrypted callback")
	}
	ciphertext := make([]byte, base64.StdEncoding.DecodedLen(len(wrapper.Encrypt)))
	decoded, err := base64.StdEncoding.Decode(ciphertext, []byte(wrapper.Encrypt))
	if err != nil || decoded == 0 || decoded%aes.BlockSize != 0 {
		clear(ciphertext)
		return nil, errors.New("invalid encrypted callback")
	}
	ciphertext = ciphertext[:decoded]
	defer clear(ciphertext)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, key[:aes.BlockSize]).CryptBlocks(plaintext, ciphertext)
	padding := int(plaintext[len(plaintext)-1])
	if padding < 1 || padding > aes.BlockSize || padding > len(plaintext) {
		clear(plaintext)
		return nil, errors.New("invalid callback padding")
	}
	for _, value := range plaintext[len(plaintext)-padding:] {
		if int(value) != padding {
			clear(plaintext)
			return nil, errors.New("invalid callback padding")
		}
	}
	return plaintext[:len(plaintext)-padding], nil
}

func normalizeCallbackStatus(value string) (suppliercallback.Status, bool) {
	switch value {
	case "INIT", "PART", "WILLEXPIRE":
		return suppliercallback.StatusRunning, true
	case "ALL", "RELIEVED":
		return suppliercallback.StatusSucceeded, true
	case "REJECT", "CANCEL", "DEADLINE", "INVALID", "EXCEPTION":
		return suppliercallback.StatusFailed, true
	default:
		return "", false
	}
}

func packCallbackEvidence(body, signature, plaintext []byte) ([]byte, error) {
	total := len(callbackEvidenceMagic) + 12 + len(body) + len(signature) + len(plaintext)
	if total > suppliercallback.MaximumEvidenceFrameBytes {
		return nil, errors.New("callback evidence is too large")
	}
	result := make([]byte, total)
	copy(result, callbackEvidenceMagic)
	offset := len(callbackEvidenceMagic)
	for _, value := range [][]byte{body, signature, plaintext} {
		binary.BigEndian.PutUint32(result[offset:offset+4], uint32(len(value)))
		offset += 4
		copy(result[offset:offset+len(value)], value)
		offset += len(value)
	}
	return result, nil
}

func decodeOneJSON(payload []byte, target any, disallowUnknown bool) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	if disallowUnknown {
		decoder.DisallowUnknownFields()
	}
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("callback JSON contains trailing data")
	}
	return nil
}

func rejectDuplicateJSON(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := walkJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errors.New("callback JSON contains trailing data")
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			nameToken, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := nameToken.(string)
			if !ok {
				return errors.New("callback JSON object key is invalid")
			}
			if _, exists := seen[name]; exists {
				return errors.New("callback JSON object key is duplicated")
			}
			seen[name] = struct{}{}
			if err = walkJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("callback JSON object is incomplete")
		}
	case '[':
		for decoder.More() {
			if err = walkJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("callback JSON array is incomplete")
		}
	default:
		return errors.New("callback JSON delimiter is invalid")
	}
	return nil
}

func validCallbackText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "\r\n\t\x00")
}
