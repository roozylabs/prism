package database

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisTimeoutsFailFastUnlessConfigured(t *testing.T) {
	opts, err := redis.ParseURL("redis://localhost:6379")
	if err != nil {
		t.Fatal(err)
	}
	applyRedisTimeouts(opts)
	if opts.DialTimeout != redisDialTimeout || opts.ReadTimeout != redisIOTimeout || opts.WriteTimeout != redisIOTimeout || opts.MaxRetries != 1 {
		t.Fatalf("defaults not applied: %+v", opts)
	}

	opts, err = redis.ParseURL("redis://localhost:6379?read_timeout=3s&max_retries=4")
	if err != nil {
		t.Fatal(err)
	}
	applyRedisTimeouts(opts)
	if opts.ReadTimeout != 3*time.Second || opts.MaxRetries != 4 {
		t.Fatalf("URL settings overridden: read %v, retries %d", opts.ReadTimeout, opts.MaxRetries)
	}
}
