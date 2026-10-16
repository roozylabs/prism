package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// AttemptLimiter counts attempts per key in a fixed window, for throttling
// sign-in and sign-up. The window starts at the first attempt and the key
// expires with it.
type AttemptLimiter struct {
	rdb    *goredis.Client
	prefix string
	limit  int64
	window time.Duration
}

func NewAttemptLimiter(rdb *goredis.Client, prefix string, limit int, window time.Duration) *AttemptLimiter {
	return &AttemptLimiter{rdb: rdb, prefix: prefix, limit: int64(limit), window: window}
}

// hitScript increments the counter and starts the window on the first hit.
var hitScript = goredis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return n`)

// Hit records an attempt. It reports whether the attempt is within the limit
// and, when it is not, how long until the window ends.
func (l *AttemptLimiter) Hit(ctx context.Context, key string) (bool, time.Duration, error) {
	n, err := hitScript.Run(ctx, l.rdb, []string{l.prefix + key}, l.window.Milliseconds()).Int64()
	if err != nil {
		return true, 0, err
	}
	if n <= l.limit {
		return true, 0, nil
	}
	return false, l.retryAfter(ctx, key), nil
}

// Blocked reports whether key has used up its attempts, without counting one.
func (l *AttemptLimiter) Blocked(ctx context.Context, key string) (bool, time.Duration, error) {
	n, err := l.rdb.Get(ctx, l.prefix+key).Int64()
	if errors.Is(err, goredis.Nil) {
		return false, 0, nil
	}
	if err != nil {
		return false, 0, err
	}
	if n < l.limit {
		return false, 0, nil
	}
	return true, l.retryAfter(ctx, key), nil
}

// Reset clears the attempts of key, e.g. after a successful sign-in.
func (l *AttemptLimiter) Reset(ctx context.Context, key string) error {
	return l.rdb.Del(ctx, l.prefix+key).Err()
}

func (l *AttemptLimiter) retryAfter(ctx context.Context, key string) time.Duration {
	ttl, err := l.rdb.PTTL(ctx, l.prefix+key).Result()
	if err != nil || ttl <= 0 {
		return l.window
	}
	return ttl
}
