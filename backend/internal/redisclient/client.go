// Package redisclient wraps go-redis construction and key namespacing for the
// backend's shared queue state (docs/redis-queue-horizontal-scaling.md).
//
// Redis is an optional dependency: it is only dialed when REDIS_ADDR is set,
// and it backs queuing/notification state only — session truth stays in the
// Kubernetes API.
package redisclient

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// DefaultKeyPrefix namespaces every backend key so a shared Redis instance
// stays legible.
const DefaultKeyPrefix = "ksbx:"

// Options configures the shared Redis connection. Whether an empty Addr means
// "Redis disabled" is the caller's decision, not this package's.
type Options struct {
	Addr        string
	Password    string
	DB          int
	KeyPrefix   string
	DialTimeout time.Duration
}

// Client is a thin wrapper over go-redis that adds the configured key prefix.
type Client struct {
	*redis.Client
	prefix string
}

// New builds a Client from Options, applying defaults for prefix and timeout.
func New(o Options) *Client {
	if o.KeyPrefix == "" {
		o.KeyPrefix = DefaultKeyPrefix
	}
	if o.DialTimeout <= 0 {
		o.DialTimeout = 5 * time.Second
	}
	return &Client{
		Client: redis.NewClient(&redis.Options{
			Addr:        o.Addr,
			Password:    o.Password,
			DB:          o.DB,
			DialTimeout: o.DialTimeout,
		}),
		prefix: o.KeyPrefix,
	}
}

// Key returns suffix namespaced under the configured prefix.
func (c *Client) Key(suffix string) string { return c.prefix + suffix }

// Healthy pings Redis with a short deadline; nil means reachable.
func (c *Client) Healthy(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return c.Ping(ctx).Err()
}
