// Package tenant carries the workspace identity through requests, persistence,
// caches and background jobs. It never stores a process-wide current tenant.
package tenant

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
)

var ErrMissing = errors.New("tenant context is required")
var ErrMismatch = errors.New("tenant identity cannot be changed")

type contextKey struct{}

type Identity struct {
	ID   int64
	Slug string
}

func WithContext(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, identity)
}

func FromContext(ctx context.Context) (Identity, error) {
	if ctx == nil {
		return Identity{}, ErrMissing
	}
	identity, ok := ctx.Value(contextKey{}).(Identity)
	if !ok || identity.ID <= 0 {
		return Identity{}, ErrMissing
	}
	return identity, nil
}

// Key refuses to create an unscoped cache, file or job identifier.
func Key(ctx context.Context, key string) (string, error) {
	identity, err := FromContext(ctx)
	if err != nil {
		return "", err
	}
	return "tenant:" + strconv.FormatInt(identity.ID, 10) + ":" + key, nil
}

// MustKey is for code reached after tenant resolution. Missing context is a
// programming error and must never fall back to a shared namespace.
func MustKey(ctx context.Context, key string) string {
	value, err := Key(ctx, key)
	if err != nil {
		panic(err)
	}
	return value
}

// Registry partitions long-lived in-memory state by explicit tenant identity.
// The initializer must return an independent value, including maps and slices.
type Registry[T any] struct {
	values sync.Map
}

func (r *Registry[T]) Get(ctx context.Context, initialize func() T) (T, error) {
	identity, err := FromContext(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	if value, ok := r.values.Load(identity.ID); ok {
		return value.(T), nil
	}
	value, _ := r.values.LoadOrStore(identity.ID, initialize())
	return value.(T), nil
}

func (r *Registry[T]) Delete(id int64) {
	r.values.Delete(id)
}

// Settings publishes immutable configuration snapshots. Maps and slices in a
// snapshot must be replaced, never mutated. Separately synchronized caches may
// be referenced by pointer, but mutexes must not be copied into a snapshot.
type Settings[T any] struct {
	mu      sync.Mutex
	current atomic.Pointer[T]
}

func NewSettings[T any](initial *T) *Settings[T] {
	s := &Settings[T]{}
	s.current.Store(initial)
	return s
}

func (s *Settings[T]) Load() *T { return s.current.Load() }

func (s *Settings[T]) Update(update func(*T)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := *s.current.Load()
	update(&next)
	s.current.Store(&next)
}

// Path returns a server-owned workspace path, never a caller-supplied redirect.
// Hostnames isolate workspaces, so the path is not prefixed.
func Path(ctx context.Context, path string) string {
	if _, err := FromContext(ctx); err != nil {
		panic(ErrMissing)
	}
	return GatewayPath(path)
}
