// Package cache provides a Redis-backed caching layer for AEGIS.
//
// It implements:
//   - Read-through caching for frequently accessed entities (items, exams, blueprints)
//   - Write-through invalidation on mutations
//   - Distributed locking for paper generation
//   - Session state caching for active exams
//   - Rate limit counters
//
// All cache keys are namespaced by organization ID for tenant isolation.
// TTLs are configurable per entity type.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
)

// ──────────────────────────────────────────────
//  Redis Client Interface
// ──────────────────────────────────────────────

// RedisClient abstracts Redis operations. This allows swapping between
// standalone Redis, Redis Cluster, and in-memory implementations.
type RedisClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)
	Incr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
}

// ──────────────────────────────────────────────
//  Cache Service
// ──────────────────────────────────────────────

// DefaultTTLs for different entity types.
var DefaultTTLs = map[string]time.Duration{
	"item":      15 * time.Minute,
	"blueprint": 30 * time.Minute,
	"exam":      10 * time.Minute,
	"session":   5 * time.Minute,
	"user":      30 * time.Minute,
}

// Service provides type-safe caching operations.
type Service struct {
	client RedisClient
	logger *zap.Logger
	prefix string // Key prefix for namespace isolation
}

// NewService creates a new cache service.
func NewService(client RedisClient, logger *zap.Logger) *Service {
	return &Service{
		client: client,
		logger: logger.With(zap.String("component", "cache")),
		prefix: "aegis:",
	}
}

// key constructs a namespaced cache key.
// Format: aegis:{orgID}:{entityType}:{entityID}
func (s *Service) key(orgID, entityType, entityID string) string {
	return fmt.Sprintf("%s%s:%s:%s", s.prefix, orgID, entityType, entityID)
}

// ──────────────────────────────────────────────
//  Generic Cache Operations
// ──────────────────────────────────────────────

// Get retrieves a cached entity, unmarshaling it into the destination.
// Returns false if the key does not exist (cache miss).
func (s *Service) Get(ctx context.Context, orgID, entityType, entityID string, dest interface{}) (bool, error) {
	cacheKey := s.key(orgID, entityType, entityID)

	data, err := s.client.Get(ctx, cacheKey)
	if err != nil {
		// Cache miss — not an error
		return false, nil
	}
	if data == "" {
		return false, nil
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		s.logger.Warn("cache unmarshal error, treating as miss",
			zap.String("key", cacheKey),
			zap.Error(err),
		)
		// Delete the corrupted entry
		s.client.Del(ctx, cacheKey)
		return false, nil
	}

	return true, nil
}

// Set stores an entity in the cache with the default TTL for its type.
func (s *Service) Set(ctx context.Context, orgID, entityType, entityID string, value interface{}) error {
	cacheKey := s.key(orgID, entityType, entityID)
	ttl := DefaultTTLs[entityType]
	if ttl == 0 {
		ttl = 15 * time.Minute
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal error: %w", err)
	}

	return s.client.Set(ctx, cacheKey, data, ttl)
}

// Invalidate removes an entity from the cache.
func (s *Service) Invalidate(ctx context.Context, orgID, entityType, entityID string) error {
	cacheKey := s.key(orgID, entityType, entityID)
	return s.client.Del(ctx, cacheKey)
}

// InvalidateMany removes multiple entities from the cache.
func (s *Service) InvalidateMany(ctx context.Context, orgID, entityType string, entityIDs []string) error {
	keys := make([]string, len(entityIDs))
	for i, id := range entityIDs {
		keys[i] = s.key(orgID, entityType, id)
	}
	return s.client.Del(ctx, keys...)
}

// ──────────────────────────────────────────────
//  Read-Through Cache Pattern
// ──────────────────────────────────────────────

// GetOrLoad implements the read-through cache pattern.
// It checks the cache first; on miss, calls the loader function,
// caches the result, and returns it.
func GetOrLoad[T any](ctx context.Context, svc *Service, orgID, entityType, entityID string, loader func() (*T, error)) (*T, error) {
	var cached T
	hit, err := svc.Get(ctx, orgID, entityType, entityID, &cached)
	if err != nil {
		svc.logger.Warn("cache get error, falling through to loader", zap.Error(err))
	}
	if hit {
		return &cached, nil
	}

	// Cache miss — load from source
	result, err := loader()
	if err != nil {
		return nil, err
	}

	// Store in cache (fire-and-forget, don't block on cache write)
	go func() {
		if setErr := svc.Set(context.Background(), orgID, entityType, entityID, result); setErr != nil {
			svc.logger.Warn("cache set error", zap.Error(setErr))
		}
	}()

	return result, nil
}

// ──────────────────────────────────────────────
//  Distributed Locking
// ──────────────────────────────────────────────

// AcquireLock attempts to acquire a distributed lock using Redis SET NX.
// Returns true if the lock was acquired, false if it was already held.
func (s *Service) AcquireLock(ctx context.Context, lockName string, ttl time.Duration) (bool, error) {
	lockKey := fmt.Sprintf("%slock:%s", s.prefix, lockName)
	return s.client.SetNX(ctx, lockKey, "locked", ttl)
}

// ReleaseLock releases a distributed lock.
func (s *Service) ReleaseLock(ctx context.Context, lockName string) error {
	lockKey := fmt.Sprintf("%slock:%s", s.prefix, lockName)
	return s.client.Del(ctx, lockKey)
}

// ──────────────────────────────────────────────
//  Rate Limiting
// ──────────────────────────────────────────────

// RateLimitResult holds the result of a rate limit check.
type RateLimitResult struct {
	Allowed   bool
	Remaining int64
	ResetAt   time.Time
}

// CheckRateLimit implements a sliding window rate limiter using Redis INCR + EXPIRE.
// Returns whether the request is allowed and how many requests remain.
func (s *Service) CheckRateLimit(ctx context.Context, key string, maxRequests int64, window time.Duration) (*RateLimitResult, error) {
	rateLimitKey := fmt.Sprintf("%sratelimit:%s", s.prefix, key)

	// Atomic increment
	count, err := s.client.Incr(ctx, rateLimitKey)
	if err != nil {
		// On Redis error, allow the request (fail open)
		s.logger.Warn("rate limit check failed, allowing request", zap.Error(err))
		return &RateLimitResult{Allowed: true, Remaining: maxRequests}, nil
	}

	// Set expiry on first request in window
	if count == 1 {
		s.client.Expire(ctx, rateLimitKey, window)
	}

	allowed := count <= maxRequests
	remaining := maxRequests - count
	if remaining < 0 {
		remaining = 0
	}

	return &RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetAt:   time.Now().Add(window),
	}, nil
}

// ──────────────────────────────────────────────
//  In-Memory Cache (Development/Testing)
// ──────────────────────────────────────────────

// InMemoryClient implements RedisClient using an in-memory map.
// FOR DEVELOPMENT AND TESTING ONLY.
type InMemoryClient struct {
	data     map[string]string
	expires  map[string]time.Time
	counters map[string]int64
}

// NewInMemoryClient creates a new in-memory Redis client.
func NewInMemoryClient() *InMemoryClient {
	return &InMemoryClient{
		data:     make(map[string]string),
		expires:  make(map[string]time.Time),
		counters: make(map[string]int64),
	}
}

func (c *InMemoryClient) Get(ctx context.Context, key string) (string, error) {
	if exp, ok := c.expires[key]; ok && time.Now().After(exp) {
		delete(c.data, key)
		delete(c.expires, key)
		return "", fmt.Errorf("key not found")
	}
	val, ok := c.data[key]
	if !ok {
		return "", fmt.Errorf("key not found")
	}
	return val, nil
}

func (c *InMemoryClient) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.data[key] = fmt.Sprintf("%v", value)
	if ttl > 0 {
		c.expires[key] = time.Now().Add(ttl)
	}
	return nil
}

func (c *InMemoryClient) Del(ctx context.Context, keys ...string) error {
	for _, k := range keys {
		delete(c.data, k)
		delete(c.expires, k)
	}
	return nil
}

func (c *InMemoryClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	var count int64
	for _, k := range keys {
		if _, ok := c.data[k]; ok {
			count++
		}
	}
	return count, nil
}

func (c *InMemoryClient) Incr(ctx context.Context, key string) (int64, error) {
	c.counters[key]++
	return c.counters[key], nil
}

func (c *InMemoryClient) Expire(ctx context.Context, key string, ttl time.Duration) error {
	c.expires[key] = time.Now().Add(ttl)
	return nil
}

func (c *InMemoryClient) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	if _, ok := c.data[key]; ok {
		return false, nil
	}
	c.data[key] = fmt.Sprintf("%v", value)
	c.expires[key] = time.Now().Add(ttl)
	return true, nil
}

func (c *InMemoryClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return nil, fmt.Errorf("Eval not supported in in-memory client")
}
