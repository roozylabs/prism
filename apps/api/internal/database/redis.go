package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewRedis(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	applyRedisTimeouts(opts)
	client := redis.NewClient(opts)

	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	log.Println("Connected to Redis")
	return client, nil
}

// Redis sits on the request path of every gateway call (rate limits,
// idempotency, cooldowns, concurrency). With go-redis defaults (5s dial, 3s
// reads, 3 retries) an unreachable Redis adds seconds to each of those calls.
// Unless the URL sets them, operations fail fast instead; blocking commands
// such as BLMOVE get their own timeout from go-redis.
const (
	redisDialTimeout = 500 * time.Millisecond
	redisIOTimeout   = 500 * time.Millisecond
)

func applyRedisTimeouts(opts *redis.Options) {
	if opts.DialTimeout == 0 {
		opts.DialTimeout = redisDialTimeout
	}
	if opts.ReadTimeout == 0 {
		opts.ReadTimeout = redisIOTimeout
	}
	if opts.WriteTimeout == 0 {
		opts.WriteTimeout = redisIOTimeout
	}
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 1
	}
}
