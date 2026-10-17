package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// SlidingWindowLimiter counts requests per key over a sliding window, kept in
// Redis as a sorted set of request timestamps. A request is added only when
// it is allowed, so a client that keeps retrying while blocked does not
// extend its own block.
type SlidingWindowLimiter struct {
	rdb    *goredis.Client
	window time.Duration
}

func NewSlidingWindowLimiter(rdb *goredis.Client, window time.Duration) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{rdb: rdb, window: window}
}

// Decision is the outcome of one Allow call.
type Decision struct {
	Allowed bool
	// Count is the number of requests in the window, including this one when
	// it was allowed.
	Count int64
	// RetryAfter, when the request was refused, is the time until the oldest
	// request leaves the window, rounded up to whole seconds.
	RetryAfter time.Duration
}

// allowScript drops the timestamps that left the window, then adds the
// request only while the count is under the limit. The check and the add run
// in one script, so concurrent requests cannot both pass on the same slot.
// KEYS[1] the window; ARGV[1] now in ms; ARGV[2] window in ms; ARGV[3] limit;
// ARGV[4] a member unique to this request.
var allowScript = goredis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, ARGV[1] - ARGV[2])
local count = redis.call('ZCARD', KEYS[1])
if count < tonumber(ARGV[3]) then
  redis.call('ZADD', KEYS[1], ARGV[1], ARGV[4])
  redis.call('PEXPIRE', KEYS[1], ARGV[2] * 2)
  return {1, count + 1, '0'}
end
local oldest = redis.call('ZRANGE', KEYS[1], 0, 0, 'WITHSCORES')
return {0, count, oldest[2] or '0'}`)

// Allow records the request under key when fewer than limit requests were
// made in the window, and reports the decision.
func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string, limit int) (Decision, error) {
	nowMs := time.Now().UnixMilli()
	member, err := uniqueMember(nowMs)
	if err != nil {
		return Decision{}, err
	}
	res, err := allowScript.Run(ctx, l.rdb, []string{key}, nowMs, l.window.Milliseconds(), limit, member).Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("sliding window: %w", err)
	}
	if len(res) != 3 {
		return Decision{}, fmt.Errorf("sliding window: unexpected reply %v", res)
	}
	allowed, _ := res[0].(int64)
	count, _ := res[1].(int64)
	d := Decision{Allowed: allowed == 1, Count: count}
	if d.Allowed {
		return d, nil
	}
	d.RetryAfter = l.window
	if s, ok := res[2].(string); ok {
		if oldest, err := strconv.ParseFloat(s, 64); err == nil && oldest > 0 {
			d.RetryAfter = time.Duration(int64(oldest)-nowMs)*time.Millisecond + l.window
		}
	}
	d.RetryAfter = roundUpToSecond(d.RetryAfter)
	return d, nil
}

func uniqueMember(nowMs int64) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("sliding window member: %w", err)
	}
	return strconv.FormatInt(nowMs, 10) + ":" + hex.EncodeToString(b[:]), nil
}

func roundUpToSecond(d time.Duration) time.Duration {
	if d <= 0 {
		return time.Second
	}
	if r := d % time.Second; r != 0 {
		d += time.Second - r
	}
	return d
}
