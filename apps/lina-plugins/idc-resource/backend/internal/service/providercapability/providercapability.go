// Package providercapability collects actual enabled connector declarations into
// atomic, resource-owned snapshots consumed by provisioning and management.
package providercapability

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"time"
)

const (
	refreshInterval       = 5 * time.Minute
	failedRefreshInterval = 30 * time.Second
	discoveryTimeout      = 30 * time.Second
	snapshotComplete      = "complete"
	snapshotFailed        = "failed"
	unavailableAdapter    = "unavailable"
	channelEnabled        = "enabled"
)

// ErrDiscoveryFailed means one failed discovery was durably recorded. The
// bounded job may continue with other channels; no supplier payload is attached.
var ErrDiscoveryFailed = gerror.New("enabled provider capability discovery failed")

// Service advances one due, enabled channel per call without supplier writes.
type Service interface {
	// RunOne collects and atomically publishes a complete matrix, or stores a
	// failed attempt for bounded retry. Existing complete snapshots remain intact.
	// False means no channel is due. ErrDiscoveryFailed identifies a committed
	// failed attempt; other errors must stop the batch. Errors contain no secrets.
	RunOne(ctx context.Context) (bool, error)
}

type serviceImpl struct {
	database  gdb.DB
	construct func(context.Context, string) (resourcecap.Provider, func(), error)
}

var _ Service = (*serviceImpl)(nil)

// New binds the startup database and current enabled-provider session factory.
func New(database gdb.DB, construct func(context.Context, string) (resourcecap.Provider, func(), error)) (Service, error) {
	if database == nil || construct == nil {
		return nil, gerror.New("provider capability collector dependencies are required")
	}
	return &serviceImpl{database: database, construct: construct}, nil
}

type channelTarget struct {
	ChannelNo string `orm:"channel_no"`
	Version   int64  `orm:"version"`
}
