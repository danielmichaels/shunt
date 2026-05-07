package gateway

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/testutil"
)

// TestRemoveOutboundSubscription_BlocksUntilWorkersDone verifies the drain
// invariant: RemoveOutboundSubscription does not return until every in-flight
// worker has signalled Done. This is the precondition that lets the
// subsequent DeleteConsumer call run without racing in-flight Ack/Nak — the
// race that produces duplicate HTTP requests when the consumer is recreated
// with the same name.
func TestRemoveOutboundSubscription_BlocksUntilWorkersDone(t *testing.T) {
	_, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	c := &OutboundClient{
		logger:    logger.NewNopLogger(),
		jetstream: js,
	}

	sub := &OutboundSubscription{
		Subject:      "test.subject",
		StreamName:   "no-such-stream",
		ConsumerName: "no-such-consumer",
	}
	sub.workersWG.Add(1)
	c.subscriptions = append(c.subscriptions, sub)

	returned := make(chan struct{})
	var once sync.Once
	go func() {
		c.RemoveOutboundSubscription(sub.Subject)
		once.Do(func() { close(returned) })
	}()

	select {
	case <-returned:
		t.Fatal("RemoveOutboundSubscription returned before in-flight worker signalled Done — drain not enforced")
	case <-time.After(100 * time.Millisecond):
	}

	sub.workersWG.Done()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("RemoveOutboundSubscription did not return after worker signalled Done")
	}
}

// TestAddOutboundSubscription_RollsBackOnStartFailure verifies that when
// startSubscription fails inside AddOutboundSubscription, the slice append
// performed by AddSubscription (client.go:153) is undone — otherwise
// c.subscriptions retains a stale entry pointing at a consumer the rule-update
// rollback may have already deleted.
func TestAddOutboundSubscription_RollsBackOnStartFailure(t *testing.T) {
	_, js, cleanup := testutil.SetupNATS(t)
	defer cleanup()

	ctx := context.Background()

	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	consumerName := "outbound-start-fail"
	_, err = js.CreateOrUpdateConsumer(ctx, "EVENTS", jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: "events.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	require.NoError(t, err)

	c := &OutboundClient{
		logger:    logger.NewNopLogger(),
		jetstream: js,
		consumerCfg: &ConsumerConfig{
			WorkerCount:    1,
			FetchBatchSize: 1,
			FetchTimeout:   100 * time.Millisecond,
			AckWaitTimeout: 30 * time.Second,
		},
	}

	err = c.AddOutboundSubscription(ctx, "EVENTS", consumerName, "events.>")
	require.Error(t, err, "AddOutboundSubscription must surface the start failure")

	c.mu.RLock()
	subs := append([]*OutboundSubscription(nil), c.subscriptions...)
	c.mu.RUnlock()
	for _, s := range subs {
		assert.NotEqual(t, "events.>", s.Subject,
			"c.subscriptions must NOT retain a subject whose subscription start failed")
	}
}
