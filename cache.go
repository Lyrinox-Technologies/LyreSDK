package lyresdk

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// Caller is the extension point for Forge-generated capability bindings,
// chains, instrumentation and application-owned wrappers.
type Caller interface {
	Call(context.Context, string, any, any) error
}
type CacheOptions struct {
	// ProviderID pins related cache operations to one provider. Cache state is
	// ephemeral and is not shared between providers or retained on restart.
	ProviderID string
	// ImplementationPrefix defaults to "cache." (for example cache.get).
	ImplementationPrefix string
}
type Cache struct {
	caller    Caller
	namespace string
	options   CacheOptions
}
type CacheEntry struct {
	Value     json.RawMessage `json:"value"`
	Revision  string          `json:"revision"`
	ExpiresAt time.Time       `json:"expires_at"`
}
type CachePage struct {
	Keys      []string `json:"keys"`
	NextAfter string   `json:"next_after"`
}
type CacheListOptions struct {
	Prefix, After string
	Limit         int
}

func NewCache(caller Caller, namespace string, options CacheOptions) *Cache {
	if options.ImplementationPrefix == "" {
		options.ImplementationPrefix = "cache."
	}
	return &Cache{caller: caller, namespace: namespace, options: options}
}
func (c *Client) Cache(namespace string, options CacheOptions) *Cache {
	return NewCache(c, namespace, options)
}
func (s *Service) Cache(namespace string, options CacheOptions) *Cache {
	return NewCache(s, namespace, options)
}
func (c *Cache) call(ctx context.Context, op string, input map[string]any, output any) error {
	if c.caller == nil {
		return errors.New("cache requires a Lyre caller")
	}
	input["namespace"] = c.namespace
	r := Reference{Capability: "cache." + op, ContractVersion: 1}
	if c.options.ProviderID != "" {
		r.ProviderID = c.options.ProviderID
		r.ProviderCapabilityID = c.options.ImplementationPrefix + op
	}
	return c.caller.Call(ctx, r.String(), input, output)
}
func cacheTTL(input map[string]any, ttl time.Duration) error {
	if ttl == 0 {
		return nil
	}
	if ttl < time.Second || ttl > 24*time.Hour || ttl%time.Second != 0 {
		return errors.New("cache TTL must be whole seconds from 1 second through 24 hours")
	}
	input["ttl_seconds"] = int64(ttl / time.Second)
	return nil
}
func (c *Cache) Put(ctx context.Context, key string, value any, ttl time.Duration) (CacheEntry, error) {
	return c.put(ctx, key, value, ttl, nil)
}
func (c *Cache) PutIfRevision(ctx context.Context, key string, value any, ttl time.Duration, revision string) (CacheEntry, error) {
	return c.put(ctx, key, value, ttl, &revision)
}
func (c *Cache) put(ctx context.Context, key string, value any, ttl time.Duration, revision *string) (CacheEntry, error) {
	in := map[string]any{"key": key, "value": value}
	var entry CacheEntry
	if e := cacheTTL(in, ttl); e != nil {
		return entry, e
	}
	if revision != nil {
		in["if_revision"] = *revision
	}
	e := c.call(ctx, "put", in, &entry)
	return entry, e
}
func (c *Cache) Get(ctx context.Context, key string, output any) (CacheEntry, bool, error) {
	// Missing entries carry an empty timestamp, so parse it only on a hit.
	var raw struct {
		Found     bool            `json:"found"`
		Value     json.RawMessage `json:"value"`
		Revision  string          `json:"revision"`
		ExpiresAt string          `json:"expires_at"`
	}
	var entry CacheEntry
	if e := c.call(ctx, "get", map[string]any{"key": key}, &raw); e != nil {
		return entry, false, e
	}
	if !raw.Found {
		return entry, false, nil
	}
	expires, e := time.Parse(time.RFC3339Nano, raw.ExpiresAt)
	if e != nil {
		return entry, false, e
	}
	entry = CacheEntry{Value: raw.Value, Revision: raw.Revision, ExpiresAt: expires}
	if output != nil {
		e = decodeJSON(raw.Value, output)
	}
	return entry, true, e
}
func (c *Cache) Delete(ctx context.Context, key string) (bool, error) { return c.delete(ctx, key, nil) }
func (c *Cache) DeleteIfRevision(ctx context.Context, key, revision string) (bool, error) {
	return c.delete(ctx, key, &revision)
}
func (c *Cache) delete(ctx context.Context, key string, revision *string) (bool, error) {
	var out struct {
		Deleted bool `json:"deleted"`
	}
	in := map[string]any{"key": key}
	if revision != nil {
		in["if_revision"] = *revision
	}
	e := c.call(ctx, "delete", in, &out)
	return out.Deleted, e
}
func (c *Cache) Clear(ctx context.Context) (int, error) {
	var out struct {
		Deleted int `json:"deleted"`
	}
	e := c.call(ctx, "clear", map[string]any{}, &out)
	return out.Deleted, e
}
func (c *Cache) List(ctx context.Context, options CacheListOptions) (CachePage, error) {
	in := map[string]any{"prefix": options.Prefix, "after": options.After}
	if options.Limit != 0 {
		in["limit"] = options.Limit
	}
	var out CachePage
	e := c.call(ctx, "list", in, &out)
	return out, e
}
func (c *Cache) Increment(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, CacheEntry, error) {
	in := map[string]any{"key": key, "delta": delta}
	var out CacheEntry
	if e := cacheTTL(in, ttl); e != nil {
		return 0, out, e
	}
	e := c.call(ctx, "increment", in, &out)
	if e != nil {
		return 0, out, e
	}
	var value int64
	e = json.Unmarshal(out.Value, &value)
	return value, out, e
}
