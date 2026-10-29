// Package customerlogin owns encrypted, customer-scoped contact login indexes.
// Passwords and host account state remain entirely host-owned.
package customerlogin

import (
	"context"
	"crypto/cipher"
	"crypto/tls"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/usercap"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"net"
	"net/http"
	"time"
)

const table = "plugin_idc_core_customer_login_contact"

type Service interface {
	customerlogincap.Directory
	// DefaultRoleID reads the enabled built-in self-only user role through the
	// current operator public role-query permission.
	DefaultRoleID(context.Context) (int, error)
	// RecordProvisioned records only the exact values just accepted by the host
	// account owner inside the caller's provisioning transaction.
	RecordProvisioned(context.Context, int, string, string, string) error
}
type serviceImpl struct {
	database gdb.DB
	bizctx   bizctxcap.Service
	users    usercap.Service
	indexKey []byte
	seal     cipher.AEAD
	client   *http.Client
}

var _ Service = (*serviceImpl)(nil)

// New creates one startup directory backed by the shared database and stable,
// purpose-separated installation keys. It keeps no account-state cache.
func New(database gdb.DB, bizctx bizctxcap.Service, users usercap.Service, indexKey []byte, seal cipher.AEAD) (Service, error) {
	if database == nil || bizctx == nil || users == nil || len(indexKey) != 32 || seal == nil {
		return nil, gerror.New("customer login directory dependencies are required")
	}
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, TLSHandshakeTimeout: 5 * time.Second, IdleConnTimeout: time.Minute, MaxIdleConns: 8}
	return &serviceImpl{database: database, bizctx: bizctx, users: users, indexKey: append([]byte(nil), indexKey...), seal: seal, client: &http.Client{Transport: transport, Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// Validate is limited to customer identities known to the owner. Public
// authentication callers cannot use it to inspect arbitrary host users.
func (s *serviceImpl) Validate(ctx context.Context, userID int) (string, error) {
	return s.customer(ctx, userID)
}
