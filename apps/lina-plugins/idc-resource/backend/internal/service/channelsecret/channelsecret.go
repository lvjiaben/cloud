// Package channelsecret owns governed Provider-channel secret bindings.
package channelsecret

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/managementcap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

const (
	PermissionManage = "idc:admin:resource:channel-secret:manage"

	OperationCreate  Operation = "create"
	OperationReplace Operation = "replace"
	OperationRevoke  Operation = "revoke"
	OperationRotate  Operation = "rotate"

	maximumSecretBytes = secretcap.MaximumManagedPlaintextBytes
)

var (
	bindingNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]{0,127}$`)
	idempotencyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{7,127}$`)
	lowerHex64Pattern  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Operation string

type AccessRequest struct {
	Operation         Operation
	ChannelNo         string
	BindingName       string
	SecretDigest      string
	ExpectedUpdatedAt time.Time
	RotateLimit       int
	IdempotencyKey    string
}

type AccessContext struct {
	Token     string
	ExpiresAt time.Time
}

type MutationInput struct {
	ChannelNo         string
	BindingName       string
	Secret            []byte
	ExpectedUpdatedAt time.Time
	RotateLimit       int
	IdempotencyKey    string
	AccessContext     string
}

type Status struct {
	ChannelNo   string
	BindingName string
	Configured  bool
	MaskedTail  string
	UpdatedAt   time.Time
}

type ManagerFactory func(channelNo string) (secretcap.Manager, error)

type Service interface {
	IssueAccess(context.Context, AccessRequest) (*AccessContext, error)
	Create(context.Context, MutationInput) (*Status, error)
	Replace(context.Context, MutationInput) (*Status, error)
	Revoke(context.Context, MutationInput) (*Status, error)
	Rotate(context.Context, MutationInput) (secretcap.RotationResult, error)
}

type serviceImpl struct {
	database gdb.DB
	runtime  managementcap.Runtime
	manager  ManagerFactory
}

var _ Service = (*serviceImpl)(nil)

func New(database gdb.DB, runtime managementcap.Runtime, manager ManagerFactory) (Service, error) {
	if database == nil || managementcap.ValidateRuntime(runtime) != nil ||
		nilDependency(runtime.SensitiveAccess) || nilDependency(manager) {
		return nil, gerror.New("channel secret management dependencies are invalid")
	}
	return &serviceImpl{database: database, runtime: runtime, manager: manager}, nil
}

func (s *serviceImpl) IssueAccess(ctx context.Context, request AccessRequest) (*AccessContext, error) {
	request, err := normalizeAccessRequest(request)
	if err != nil {
		return nil, err
	}
	if _, err = s.authorize(ctx, request.ChannelNo); err != nil {
		return nil, err
	}
	if err = s.requireChannel(ctx, request.ChannelNo, false); err != nil {
		return nil, err
	}
	issued, err := s.runtime.SensitiveAccess.IssueSensitiveAccess(ctx, accessBinding(request))
	if err != nil || issued == nil || strings.TrimSpace(issued.Token) == "" || issued.ExpiresAt.IsZero() {
		return nil, bizerr.NewCode(CodeRecentAuthenticationRequired)
	}
	return &AccessContext{Token: issued.Token, ExpiresAt: issued.ExpiresAt.UTC()}, nil
}

func (s *serviceImpl) authorize(ctx context.Context, channelNo string) (int, error) {
	actor, err := s.runtime.Authorizer.Authorize(ctx, managementcap.AuthorizationRequest{
		Permission: PermissionManage, TargetNo: channelNo,
	})
	if err != nil || actor <= 0 {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return actor, nil
}

func normalizeAccessRequest(request AccessRequest) (AccessRequest, error) {
	request.ChannelNo = strings.TrimSpace(request.ChannelNo)
	request.BindingName = strings.TrimSpace(request.BindingName)
	request.SecretDigest = strings.TrimSpace(request.SecretDigest)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if resourcecap.ValidateChannelRef(request.ChannelNo) != nil ||
		!idempotencyPattern.MatchString(request.IdempotencyKey) {
		return AccessRequest{}, bizerr.NewCode(CodeInvalid)
	}
	switch request.Operation {
	case OperationCreate, OperationReplace:
		if !bindingNamePattern.MatchString(request.BindingName) ||
			!lowerHex64Pattern.MatchString(request.SecretDigest) || request.RotateLimit != 0 {
			return AccessRequest{}, bizerr.NewCode(CodeInvalid)
		}
		if request.Operation == OperationReplace && request.ExpectedUpdatedAt.IsZero() {
			return AccessRequest{}, bizerr.NewCode(CodeInvalid)
		}
	case OperationRevoke:
		if !bindingNamePattern.MatchString(request.BindingName) || request.SecretDigest != "" ||
			request.ExpectedUpdatedAt.IsZero() || request.RotateLimit != 0 {
			return AccessRequest{}, bizerr.NewCode(CodeInvalid)
		}
	case OperationRotate:
		if request.BindingName != "" || request.SecretDigest != "" ||
			!request.ExpectedUpdatedAt.IsZero() || request.RotateLimit < 1 || request.RotateLimit > 100 {
			return AccessRequest{}, bizerr.NewCode(CodeInvalid)
		}
	default:
		return AccessRequest{}, bizerr.NewCode(CodeInvalid)
	}
	request.ExpectedUpdatedAt = request.ExpectedUpdatedAt.UTC()
	return request, nil
}

func accessBinding(request AccessRequest) managementcap.SensitiveAccessBinding {
	canonical := strings.Join([]string{
		"idc-resource-channel-secret-v1", string(request.Operation), request.ChannelNo,
		request.BindingName, request.SecretDigest, request.ExpectedUpdatedAt.Format(time.RFC3339Nano),
		strconv.Itoa(request.RotateLimit), request.IdempotencyKey,
	}, "\x00")
	return managementcap.SensitiveAccessBinding{
		TargetDigest: managementcap.DigestSensitiveAccessTarget([]byte(canonical)),
	}
}

func secretDigest(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func bindingNumber(channelNo, bindingName, idempotencyKey string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{
		"idc-resource-channel-secret-binding-v1", channelNo, bindingName, idempotencyKey,
	}, "\x00")))
	return "BND_" + hex.EncodeToString(digest[:16])
}

func (s *serviceImpl) consume(
	ctx context.Context,
	operation Operation,
	input MutationInput,
) (AccessRequest, int, error) {
	request := AccessRequest{
		Operation: operation, ChannelNo: input.ChannelNo, BindingName: input.BindingName,
		ExpectedUpdatedAt: input.ExpectedUpdatedAt, RotateLimit: input.RotateLimit,
		IdempotencyKey: input.IdempotencyKey,
	}
	if operation == OperationCreate || operation == OperationReplace {
		if len(input.Secret) == 0 || len(input.Secret) > maximumSecretBytes {
			return AccessRequest{}, 0, bizerr.NewCode(CodeInvalid)
		}
		request.SecretDigest = secretDigest(input.Secret)
	}
	request, err := normalizeAccessRequest(request)
	if err != nil || strings.TrimSpace(input.AccessContext) == "" {
		return AccessRequest{}, 0, bizerr.NewCode(CodeInvalid)
	}
	actor, err := s.authorize(ctx, request.ChannelNo)
	if err != nil {
		return AccessRequest{}, 0, err
	}
	proof, err := s.runtime.SensitiveAccess.ConsumeSensitiveAccess(
		ctx, input.AccessContext, accessBinding(request),
	)
	if err != nil || proof == nil || proof.ActorUserID != actor {
		return AccessRequest{}, 0, bizerr.NewCode(CodeRecentAuthenticationRequired)
	}
	return request, actor, nil
}

func nilDependency(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
