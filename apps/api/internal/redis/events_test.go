package redis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestEventsReachOnlyTheirOwner(t *testing.T) {
	rdb := newTestCooldownStore(t).rdb
	p := NewEventPublisher(rdb)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")
	alice, bob := "alice-"+suffix, "bob-"+suffix

	sub := p.Subscribe(ctx, alice)
	t.Cleanup(func() { _ = sub.Close() })
	if _, err := sub.Receive(ctx); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	ch := sub.Channel()

	if err := p.Publish(ctx, bob, "bob_event", map[string]string{"secret": "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := p.Publish(ctx, alice, "alice_event", nil); err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-ch:
		var evt Event
		if err := json.Unmarshal([]byte(msg.Payload), &evt); err != nil {
			t.Fatal(err)
		}
		if evt.Type != "alice_event" {
			t.Fatalf("alice received %q", evt.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("alice's event was not delivered")
	}

	select {
	case msg := <-ch:
		t.Fatalf("alice received another user's event: %s", msg.Payload)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestPublishWithoutOwnerIsDropped(t *testing.T) {
	p := NewEventPublisher(nil)
	if err := p.Publish(context.Background(), "", "orphan", nil); !errors.Is(err, ErrNoEventOwner) {
		t.Fatalf("got %v, want ErrNoEventOwner", err)
	}
}
