package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Events go to a channel per user, and the dashboard stream subscribes only
// to its own user's channel, so one tenant never receives another tenant's
// request logs, credentials or routing decisions.
const userEventsChannelPrefix = "gateway:events:user:"

// ErrNoEventOwner is returned for an event published without a user. Such an
// event has no one it may be shown to, so it is dropped.
var ErrNoEventOwner = errors.New("event has no owner")

// UserEventsChannel is the Redis channel carrying the given user's events.
func UserEventsChannel(userID string) string {
	return userEventsChannelPrefix + userID
}

type Event struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

type EventPublisher struct {
	rdb *goredis.Client
}

func NewEventPublisher(rdb *goredis.Client) *EventPublisher {
	return &EventPublisher{rdb: rdb}
}

// Publish sends an event to the dashboard streams of userID only.
func (p *EventPublisher) Publish(ctx context.Context, userID, eventType string, data interface{}) error {
	if userID == "" {
		return ErrNoEventOwner
	}
	event := Event{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now().UTC(),
	}
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.rdb.Publish(ctx, UserEventsChannel(userID), payload).Err()
}

// Subscribe listens to the events of userID.
func (p *EventPublisher) Subscribe(ctx context.Context, userID string) *goredis.PubSub {
	return p.rdb.Subscribe(ctx, UserEventsChannel(userID))
}
