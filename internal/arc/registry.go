package arc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Basekick-Labs/memtrace/internal/metadata"
	"github.com/rs/zerolog"
)

// ErrNoArcInstance is returned by Registry.Get when the requested org has no
// configured Arc instance. Surface as HTTP 503 at the request edge.
var ErrNoArcInstance = errors.New("no arc instance configured for org")

// Defaults are the timing/batch knobs shared by every Arc client. Per-org
// values (URL, API key, database, measurement) live in the metadata DB.
type Defaults struct {
	ConnectTimeout       int
	QueryTimeout         int
	WriteBatchSize       int
	WriteFlushIntervalMS int
}

// Registry resolves *Client instances by org_id. Built once at startup; per-org
// adds/removes go through Reload to keep the cache coherent.
type Registry struct {
	defaults Defaults
	store    *metadata.ArcInstanceStore
	logger   zerolog.Logger

	mu      sync.RWMutex
	clients map[string]*Client // org_id -> client
}

// NewRegistry eagerly builds clients for every row in arc_instances. Returns a
// non-nil registry even when the table is empty (Get will then return
// ErrNoArcInstance for every org until Reload is called).
func NewRegistry(ctx context.Context, store *metadata.ArcInstanceStore, defaults Defaults, logger zerolog.Logger) (*Registry, error) {
	r := &Registry{
		defaults: defaults,
		store:    store,
		logger:   logger.With().Str("component", "arc-registry").Logger(),
		clients:  make(map[string]*Client),
	}

	instances, err := store.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load arc instances: %w", err)
	}

	for _, inst := range instances {
		client := r.buildClient(inst)
		if err := r.warmup(ctx, client, inst); err != nil {
			r.logger.Warn().Err(err).
				Str("org_id", inst.OrgID).
				Str("url", inst.URL).
				Msg("Arc instance unreachable at startup; will retry in background")
		}
		r.clients[inst.OrgID] = client
	}

	r.logger.Info().Int("instances", len(instances)).Msg("Arc registry initialized")
	return r, nil
}

func (r *Registry) buildClient(inst *metadata.ArcInstance) *Client {
	return NewClient(
		inst.URL,
		inst.APIKey,
		inst.Database,
		inst.Measurement,
		r.defaults.WriteBatchSize,
		r.defaults.WriteFlushIntervalMS,
		r.defaults.ConnectTimeout,
		r.defaults.QueryTimeout,
		r.logger.With().Str("org_id", inst.OrgID).Logger(),
	)
}

func (r *Registry) warmup(ctx context.Context, client *Client, inst *metadata.ArcInstance) error {
	pingCtx, cancel := context.WithTimeout(ctx, time.Duration(r.defaults.ConnectTimeout)*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx); err != nil {
		return err
	}
	client.MarkConnected()
	r.logger.Info().Str("org_id", inst.OrgID).Str("url", inst.URL).Msg("Arc instance verified")
	return nil
}

// Get returns the *Client for an org, or ErrNoArcInstance if none is configured.
func (r *Registry) Get(orgID string) (*Client, error) {
	r.mu.RLock()
	client, ok := r.clients[orgID]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoArcInstance, orgID)
	}
	return client, nil
}

// Reload re-reads the row for orgID from the store and replaces the cached
// client. Pass an empty orgID to reload every instance. Closes the previous
// client(s) after the swap so in-flight requests on the old client finish.
func (r *Registry) Reload(ctx context.Context, orgID string) error {
	if orgID == "" {
		return r.reloadAll(ctx)
	}

	inst, err := r.store.GetByOrg(ctx, orgID)
	if err != nil {
		if errors.Is(err, metadata.ErrArcInstanceNotFound) {
			return r.remove(orgID)
		}
		return err
	}

	client := r.buildClient(inst)
	if err := r.warmup(ctx, client, inst); err != nil {
		r.logger.Warn().Err(err).Str("org_id", orgID).Msg("Reloaded Arc instance failed warmup")
	}

	r.mu.Lock()
	old := r.clients[orgID]
	r.clients[orgID] = client
	r.mu.Unlock()

	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (r *Registry) reloadAll(ctx context.Context) error {
	instances, err := r.store.List(ctx)
	if err != nil {
		return err
	}
	want := make(map[string]*metadata.ArcInstance, len(instances))
	for _, inst := range instances {
		want[inst.OrgID] = inst
	}

	r.mu.Lock()
	old := r.clients
	r.clients = make(map[string]*Client, len(instances))
	for orgID, inst := range want {
		r.clients[orgID] = r.buildClient(inst)
	}
	r.mu.Unlock()

	for orgID, c := range old {
		if _, kept := want[orgID]; !kept {
			_ = c.Close()
		} else {
			_ = c.Close() // replaced — close the old one
		}
	}

	for orgID, inst := range want {
		client, _ := r.Get(orgID)
		if client != nil {
			_ = r.warmup(ctx, client, inst)
		}
	}
	return nil
}

// Remove evicts and closes the client for orgID. Used when an org's arc
// instance is deleted via the admin CLI.
func (r *Registry) Remove(orgID string) error {
	return r.remove(orgID)
}

func (r *Registry) remove(orgID string) error {
	r.mu.Lock()
	client, ok := r.clients[orgID]
	delete(r.clients, orgID)
	r.mu.Unlock()
	if ok && client != nil {
		return client.Close()
	}
	return nil
}

// Health returns the per-org health snapshot from each client's background
// health check.
func (r *Registry) Health() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]bool, len(r.clients))
	for orgID, c := range r.clients {
		out[orgID] = c.IsConnected()
	}
	return out
}

// Orgs returns the org IDs with configured Arc instances.
func (r *Registry) Orgs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.clients))
	for orgID := range r.clients {
		out = append(out, orgID)
	}
	return out
}

// Close closes every cached client.
func (r *Registry) Close() error {
	r.mu.Lock()
	clients := r.clients
	r.clients = make(map[string]*Client)
	r.mu.Unlock()

	var firstErr error
	for _, c := range clients {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
