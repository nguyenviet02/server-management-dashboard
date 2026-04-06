package auth

import (
	"math"
	"sync"
	"time"
)

// ============ Rate Limiter ============

// RateLimiter tracks login attempts per IP
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptInfo
	maxAttempts int
	windowSecs  int
}

type attemptInfo struct {
	count    int
	firstAt  time.Time
	lastFail time.Time
}

// NewRateLimiter creates a rate limiter (e.g. 5 attempts per 900 seconds)
func NewRateLimiter(maxAttempts, windowSecs int) *RateLimiter {
	rl := &RateLimiter{
		attempts:    make(map[string]*attemptInfo),
		maxAttempts: maxAttempts,
		windowSecs:  windowSecs,
	}
	// Cleanup goroutine
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			rl.cleanup()
		}
	}()
	return rl
}

// Check returns (allowed bool, waitSeconds int)
func (rl *RateLimiter) Check(ip string) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	info, exists := rl.attempts[ip]
	if !exists {
		return true, 0
	}

	// Window expired → reset
	if time.Since(info.firstAt) > time.Duration(rl.windowSecs)*time.Second {
		delete(rl.attempts, ip)
		return true, 0
	}

	if info.count >= rl.maxAttempts {
		remaining := time.Duration(rl.windowSecs)*time.Second - time.Since(info.firstAt)
		return false, int(remaining.Seconds())
	}

	// Exponential backoff: after each fail, wait 2^(n-1) seconds
	if info.count > 0 {
		backoff := time.Duration(math.Pow(2, float64(info.count-1))) * time.Second
		if time.Since(info.lastFail) < backoff {
			wait := backoff - time.Since(info.lastFail)
			return false, int(wait.Seconds()) + 1
		}
	}

	return true, 0
}

// RecordFail records a failed login attempt
func (rl *RateLimiter) RecordFail(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	info, exists := rl.attempts[ip]
	if !exists {
		rl.attempts[ip] = &attemptInfo{count: 1, firstAt: time.Now(), lastFail: time.Now()}
		return
	}
	info.count++
	info.lastFail = time.Now()
}

// RecordSuccess clears attempts for an IP
func (rl *RateLimiter) RecordSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	cutoff := time.Now().Add(-time.Duration(rl.windowSecs) * time.Second)
	for ip, info := range rl.attempts {
		if info.firstAt.Before(cutoff) {
			delete(rl.attempts, ip)
		}
	}
}
