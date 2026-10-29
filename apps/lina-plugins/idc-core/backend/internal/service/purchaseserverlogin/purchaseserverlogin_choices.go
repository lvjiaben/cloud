// This file persists only an opaque password reference and enforces exact
// customer ownership before a saved choice can enter a purchase snapshot.
package purchaseserverlogin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"regexp"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/secretcap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const resourceOwnerPluginID = "idc-resource"
const resourceChannelScopePrefix = "resource.channel.c"
const tokenPrefix = "PSL_"

var loginTokenPattern = regexp.MustCompile(`^PSL_[0-9a-f]{64}$`)
var hostnameLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func (s *serviceImpl) Create(ctx context.Context, input Input) (*Created, error) {
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	target, err := s.resolver.ResolvePurchaseServerLoginTarget(ctx, input.PublicSlug, input.ConfigurationRevision)
	if err != nil || !validTarget(target, input.ConfigurationRevision) || !validChoices(input, target) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var previous *Resolved
	if input.PreviousServerLoginToken != "" {
		previous, err = s.Resolve(ctx, input.PreviousServerLoginToken, audience.CustomerNo, target.ProductVersionNo)
		if err != nil {
			return nil, err
		}
		if previous == nil || previous.ChannelNo != target.ChannelNo {
			return nil, bizerr.NewCode(CodeNotAccessible)
		}
	}
	if input.Hostname == "" && input.Password == "" && (previous == nil || previous.PasswordSecretRef == "") {
		return &Created{}, nil
	}
	tokenBytes := make([]byte, 32)
	if _, err = io.ReadFull(s.random, tokenBytes); err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	token := tokenPrefix + hex.EncodeToString(tokenBytes)
	secretRef, manager, created, err := s.storePassword(ctx, token, target.ChannelNo, input.Password)
	if err != nil {
		return nil, err
	}
	if input.Password == "" && previous != nil {
		if previous.PasswordSecretRef != "" && !target.PasswordEnabled {
			return nil, bizerr.NewCode(CodeInvalid)
		}
		secretRef = previous.PasswordSecretRef
	}
	previousCartItemNo := ""
	if previous != nil {
		previousCartItemNo = previous.CartItemNo
	}
	_, err = s.database.Model(do.PurchaseServerLogin{}).Ctx(ctx).Data(do.PurchaseServerLogin{
		TenantId: idccap.DefaultTenantID, ServerLoginToken: token, CustomerNo: audience.CustomerNo, UserId: audience.UserID,
		ProductVersionNo: target.ProductVersionNo, ConfigurationRevision: target.ConfigurationRevision,
		Hostname: input.Hostname, PasswordSecretRef: secretRef, ChannelNo: target.ChannelNo, CartItemNo: previousCartItemNo,
	}).Insert()
	if err != nil {
		if manager != nil {
			if _, revokeErr := manager.Revoke(ctx, created.Handle, created.Metadata.UpdatedAt); revokeErr != nil {
				return nil, bizerr.NewCode(CodeUnavailable)
			}
		}
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &Created{ServerLoginToken: token, Hostname: input.Hostname, PasswordConfigured: secretRef != ""}, nil
}

func (s *serviceImpl) Resolve(ctx context.Context, token, customerNo, versionNo string) (*Resolved, error) {
	if token == "" {
		return nil, nil
	}
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return nil, err
	}
	if !loginTokenPattern.MatchString(token) || audience.CustomerNo != customerNo || versionNo == "" {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	var row *struct {
		Hostname          string
		PasswordSecretRef string `orm:"password_secret_ref"`
		ChannelNo         string `orm:"channel_no"`
		CartItemNo        string `orm:"cart_item_no"`
	}
	err = s.database.Model(do.PurchaseServerLogin{}).Ctx(ctx).Fields("hostname,password_secret_ref,channel_no,cart_item_no").Where(do.PurchaseServerLogin{
		TenantId: idccap.DefaultTenantID, ServerLoginToken: token, CustomerNo: customerNo, UserId: audience.UserID, ProductVersionNo: versionNo,
	}).Scan(&row)
	if err != nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return &Resolved{Hostname: row.Hostname, PasswordSecretRef: row.PasswordSecretRef, ChannelNo: row.ChannelNo, CartItemNo: row.CartItemNo}, nil
}

func (s *serviceImpl) currentCustomer(ctx context.Context) (*principal.Audience, error) {
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID <= 0 || audience.CustomerNo == "" || audience.Code != principal.AudienceCustomer || audience.Status != principal.AudienceStatusActive {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return audience, nil
}

func (s *serviceImpl) storePassword(ctx context.Context, token, channelNo, password string) (string, secretcap.Manager, secretcap.CreateResult, error) {
	if password == "" {
		return "", nil, secretcap.CreateResult{}, nil
	}
	digest := sha256.Sum256([]byte(channelNo))
	manager, err := s.secrets.Manager(secretcap.Scope{TenantID: idccap.DefaultTenantID, OwnerPluginID: resourceOwnerPluginID, CallerScope: resourceChannelScopePrefix + hex.EncodeToString(digest[:16])})
	if err != nil || manager == nil {
		return "", nil, secretcap.CreateResult{}, bizerr.NewCode(CodeUnavailable)
	}
	value := []byte(password)
	defer clear(value)
	created, err := manager.Create(ctx, token, func(consume secretcap.Consumer) error { return consume(value) })
	if err != nil || created.Handle.IsZero() || !created.Metadata.Configured {
		return "", nil, secretcap.CreateResult{}, bizerr.NewCode(CodeUnavailable)
	}
	return created.Handle.OpaqueRef(), manager, created, nil
}

func validTarget(target *Target, revision string) bool {
	return target != nil && target.ProductVersionNo != "" && target.ChannelNo != "" && target.ConfigurationRevision != "" && target.ConfigurationRevision == revision
}
func validChoices(input Input, target *Target) bool {
	if input.Hostname != "" && (!target.HostnameEnabled || !validHostname(input.Hostname)) {
		return false
	}
	if input.Password != "" && (!target.PasswordEnabled || !validPassword(input.Password, target.PasswordRules.Characters)) {
		return false
	}
	return true
}
func validPassword(value string, classes []string) bool {
	if len(value) < 6 || len(value) > 20 {
		return false
	}
	alphabet := ""
	if len(classes) == 0 {
		alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%*-_=+?"
	}
	for _, class := range classes {
		switch class {
		case "lowercase":
			alphabet += "abcdefghijklmnopqrstuvwxyz"
		case "uppercase":
			alphabet += "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		case "digits":
			alphabet += "0123456789"
		case "symbols":
			alphabet += "!@#$%*-_=+?"
		default:
			return false
		}
	}
	for _, character := range value {
		if !strings.ContainsRune(alphabet, character) {
			return false
		}
	}
	return true
}

func (s *serviceImpl) Bind(ctx context.Context, token, customerNo, versionNo, cartItemNo string) error {
	if token == "" {
		return nil
	}
	audience, err := s.currentCustomer(ctx)
	if err != nil {
		return err
	}
	if !loginTokenPattern.MatchString(token) || audience.CustomerNo != customerNo || versionNo == "" || cartItemNo == "" {
		return bizerr.NewCode(CodeNotAccessible)
	}
	result, err := s.database.Model(do.PurchaseServerLogin{}).Ctx(ctx).Where(do.PurchaseServerLogin{
		TenantId: idccap.DefaultTenantID, ServerLoginToken: token, CustomerNo: customerNo, UserId: audience.UserID, ProductVersionNo: versionNo,
	}).Where("(cart_item_no = ? OR cart_item_no = ?)", "", cartItemNo).Data(do.PurchaseServerLogin{CartItemNo: cartItemNo}).Update()
	if err != nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return bizerr.NewCode(CodeUnavailable)
	}
	if count != 1 {
		return bizerr.NewCode(CodeNotAccessible)
	}
	return nil
}

func validHostname(value string) bool {
	if len(value) == 0 || len(value) > 63 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if !hostnameLabelPattern.MatchString(label) {
			return false
		}
	}
	return true
}
