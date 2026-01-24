// Package geminicli provides Gemini CLI credential and rate limiting utilities.
package geminicli

import (
	"sync"
	"time"
)

// RateLimitConfig defines rate limiting parameters for Gemini API requests.
type RateLimitConfig struct {
	// MinRequestInterval is the minimum time between requests to the same account
	MinRequestInterval time.Duration
	// QuotaCooldown is how long to wait after a 429 before retrying the same account
	QuotaCooldown time.Duration
	// MaxConcurrentPerAccount limits concurrent requests per account
	MaxConcurrent int
}

// DefaultRateLimitConfig returns the default rate limiting configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		MinRequestInterval: 500 * time.Millisecond, // 2 requests per second max
		QuotaCooldown:      10 * time.Second,       // 10s cooldown after 429
		MaxConcurrent:      1,                      // Sequential requests per account
	}
}

// AccountState tracks rate limiting state for a single account.
type AccountState struct {
	mu               sync.Mutex
	lastRequest      time.Time
	quotaExceeded    bool
	quotaExceededAt  time.Time
	requestQueue     chan struct{}
	activeRequests   int
}

// newAccountState creates a new account state with the given config.
func newAccountState(cfg RateLimitConfig) *AccountState {
	return &AccountState{
		requestQueue: make(chan struct{}, cfg.MaxConcurrent),
	}
}

// TryAcquire attempts to acquire a request slot. Returns true if acquired, false if rate limited.
func (s *AccountState) TryAcquire(cfg RateLimitConfig) (bool, time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Check if account is in quota cooldown
	if s.quotaExceeded {
		if now.Sub(s.quotaExceededAt) < cfg.QuotaCooldown {
			return false, cfg.QuotaCooldown - now.Sub(s.quotaExceededAt)
		}
		s.quotaExceeded = false
	}

	// Check min interval between requests
	if !s.lastRequest.IsZero() {
		interval := now.Sub(s.lastRequest)
		if interval < cfg.MinRequestInterval {
			return false, cfg.MinRequestInterval - interval
		}
	}

	// Check concurrent limit
	if s.activeRequests >= cfg.MaxConcurrent {
		return false, cfg.MinRequestInterval
	}

	s.activeRequests++
	s.lastRequest = now
	return true, 0
}

// Release frees up a request slot.
func (s *AccountState) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeRequests > 0 {
		s.activeRequests--
	}
}

// markQuotaExceeded marks the account as rate limited.
func (s *AccountState) markQuotaExceeded() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quotaExceeded = true
	s.quotaExceededAt = time.Now()
}

// RateLimiter manages rate limiting across multiple Gemini accounts.
type RateLimiter struct {
	mu            sync.RWMutex
	accounts      map[string]*AccountState
	config        RateLimitConfig
	globalQueue   chan struct{}
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(cfg RateLimitConfig) *RateLimiter {
	if cfg.MaxConcurrent <= 0 {
		cfg = DefaultRateLimitConfig()
	}
	return &RateLimiter{
		accounts:    make(map[string]*AccountState),
		config:      cfg,
		globalQueue: make(chan struct{}, 100), // Global queue capacity
	}
}

// accountKey generates a unique key for an account.
func (r *RateLimiter) accountKey(authID, email string) string {
	if email != "" {
		return email
	}
	return authID
}

// GetAccountState returns or creates the account state for the given account.
func (r *RateLimiter) GetAccountState(authID, email string) *AccountState {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := r.accountKey(authID, email)
	if state, ok := r.accounts[key]; ok {
		return state
	}
	state := newAccountState(r.config)
	r.accounts[key] = state
	return state
}

// MarkQuotaExceeded marks an account as quota exceeded.
func (r *RateLimiter) MarkQuotaExceeded(authID, email string) {
	state := r.GetAccountState(authID, email)
	state.markQuotaExceeded()
}

// GetConfig returns the current rate limit configuration.
func (r *RateLimiter) GetConfig() RateLimitConfig {
	return r.config
}

// SetConfig updates the rate limit configuration.
func (r *RateLimiter) SetConfig(cfg RateLimitConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 1
	}
	r.config = cfg
}

// AccountStats returns statistics for an account.
func (r *RateLimiter) AccountStats(authID, email string) (quotaExceeded bool, lastRequest time.Time) {
	state := r.GetAccountState(authID, email)
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.quotaExceeded, state.lastRequest
}
