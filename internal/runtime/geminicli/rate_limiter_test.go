package geminicli

import (
	"sync"
	"testing"
	"time"
)

func TestAccountState_TryAcquire(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	cfg.MinRequestInterval = 0 // No interval restriction for this test
	cfg.MaxConcurrent = 2

	state := newAccountState(cfg)

	// First acquire should succeed
	acquired, waitTime := state.TryAcquire(cfg)
	if !acquired {
		t.Error("Expected first acquire to succeed")
	}
	if waitTime != 0 {
		t.Errorf("Expected wait time 0, got %v", waitTime)
	}

	// Second acquire (within concurrency limit) should also succeed
	acquired, waitTime = state.TryAcquire(cfg)
	if !acquired {
		t.Error("Expected second acquire to succeed (within concurrency limit)")
	}
	if waitTime != 0 {
		t.Errorf("Expected wait time 0, got %v", waitTime)
	}

	// Third acquire should fail (exceeds concurrency limit)
	acquired, waitTime = state.TryAcquire(cfg)
	if acquired {
		t.Error("Expected third acquire to fail (exceeds concurrency limit)")
	}

	state.Release()
	state.Release()

	// After releasing, should be able to acquire again
	acquired, _ = state.TryAcquire(cfg)
	if !acquired {
		t.Error("Expected acquire to succeed after release")
	}
}

func TestAccountState_MarkQuotaExceeded(t *testing.T) {
	cfg := DefaultRateLimitConfig()
	cfg.QuotaCooldown = 100 * time.Millisecond

	state := newAccountState(cfg)

	// Mark as quota exceeded
	state.markQuotaExceeded()

	// Try to acquire immediately - should fail
	acquired, waitTime := state.TryAcquire(cfg)
	if acquired {
		t.Error("Expected acquire to fail when quota exceeded")
	}
	if waitTime < cfg.QuotaCooldown/2 {
		t.Errorf("Expected wait time >= %v, got %v", cfg.QuotaCooldown/2, waitTime)
	}

	// After cooldown, should be able to acquire again
	time.Sleep(cfg.QuotaCooldown + 50*time.Millisecond)
	acquired, _ = state.TryAcquire(cfg)
	if !acquired {
		t.Error("Expected acquire to succeed after quota cooldown")
	}
}

func TestRateLimiter_GetAccountState(t *testing.T) {
	limiter := NewRateLimiter(DefaultRateLimitConfig())

	state1 := limiter.GetAccountState("auth1", "user1@example.com")
	state2 := limiter.GetAccountState("auth1", "user1@example.com")
	state3 := limiter.GetAccountState("auth2", "user2@example.com")

	if state1 != state2 {
		t.Error("Expected same state for same account (email)")
	}
	if state1 == state3 {
		t.Error("Expected different state for different accounts")
	}
}

func TestRateLimiter_MarkQuotaExceeded(t *testing.T) {
	limiter := NewRateLimiter(DefaultRateLimitConfig())

	// Mark account as quota exceeded
	limiter.MarkQuotaExceeded("auth1", "user1@example.com")

	// Check account stats
	quotaExceeded, _ := limiter.AccountStats("auth1", "user1@example.com")
	if !quotaExceeded {
		t.Error("Expected quotaExceeded to be true")
	}

	// Different account should not be marked
	quotaExceeded, _ = limiter.AccountStats("auth2", "user2@example.com")
	if quotaExceeded {
		t.Error("Expected quotaExceeded to be false for different account")
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewRateLimiter(DefaultRateLimitConfig())
	cfg := RateLimitConfig{
		MinRequestInterval: 0,
		QuotaCooldown:      1 * time.Second,
		MaxConcurrent:      10,
	}

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// First wave: 10 concurrent requests (should all succeed)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state := limiter.GetAccountState("auth1", "user1@example.com")
			acquired, _ := state.TryAcquire(cfg)
			if acquired {
				mu.Lock()
				successCount++
				mu.Unlock()
				time.Sleep(10 * time.Millisecond)
				state.Release()
			}
		}()
	}

	wg.Wait()

	// First wave should have 10 successful acquires
	if successCount != 10 {
		t.Errorf("Expected 10 successful acquires in first wave, got %d", successCount)
	}

	// Second wave: 10 more requests (should all succeed now)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state := limiter.GetAccountState("auth1", "user1@example.com")
			acquired, _ := state.TryAcquire(cfg)
			if acquired {
				mu.Lock()
				successCount++
				mu.Unlock()
				state.Release()
			}
		}()
	}

	wg.Wait()

	// Total should be 20 successful acquires
	if successCount != 20 {
		t.Errorf("Expected 20 successful acquires total, got %d", successCount)
	}
}

func TestDefaultRateLimitConfig(t *testing.T) {
	cfg := DefaultRateLimitConfig()

	if cfg.MinRequestInterval != 500*time.Millisecond {
		t.Errorf("Expected MinRequestInterval 500ms, got %v", cfg.MinRequestInterval)
	}
	if cfg.QuotaCooldown != 10*time.Second {
		t.Errorf("Expected QuotaCooldown 10s, got %v", cfg.QuotaCooldown)
	}
	if cfg.MaxConcurrent != 1 {
		t.Errorf("Expected MaxConcurrent 1, got %d", cfg.MaxConcurrent)
	}
}
