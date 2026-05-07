package broker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/rule"
)

// TestRemoveSubscription_BlocksUntilWorkersDone verifies that
// SubscriptionManager.RemoveSubscription waits for any in-flight worker to
// finish before returning. This is the precondition that lets the caller
// (NATSBroker.RemoveSubscription) safely run DeleteConsumer afterwards
// without racing an in-flight Ack/Nak.
func TestRemoveSubscription_BlocksUntilWorkersDone(t *testing.T) {
	sm := &SubscriptionManager{
		subscriptions: make(map[string]*Subscription),
		logger:        logger.NewNopLogger(),
	}

	sub := &Subscription{Subject: "test.subject"}
	sub.workersWG.Add(1)
	sm.subscriptions[sub.Subject] = sub

	returned := make(chan struct{})
	var once sync.Once
	go func() {
		sm.RemoveSubscription(sub.Subject)
		once.Do(func() { close(returned) })
	}()

	select {
	case <-returned:
		t.Fatal("RemoveSubscription returned before in-flight worker signalled Done — drain not implemented")
	case <-time.After(100 * time.Millisecond):
	}

	sub.workersWG.Done()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("RemoveSubscription did not return after worker signalled Done")
	}

	assert.True(t, true, "drain semantic verified")
}

// TestAddAndStartSubscription_RollsBackOnStartFailure verifies that when
// startSubscription fails (here forced by an invalid PullExpiry < 1s), the
// SubscriptionManager removes the subject from sm.subscriptions. Otherwise the
// next call to AddAndStartSubscription hits the "already exists" fast-path
// (subscription.go:159) and the subject is permanently un-started.
func TestAddAndStartSubscription_RollsBackOnStartFailure(t *testing.T) {
	_, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()

	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	consumerName := "test-consumer-start-fail"
	_, err = js.CreateOrUpdateConsumer(ctx, "EVENTS", jetstream.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: "events.>",
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	require.NoError(t, err)

	// FetchTimeout < 1s makes PullExpiry option validation fail inside
	// jetstream.Consumer.Messages — startSubscription returns an error
	// without ever booting the worker pool.
	sm := &SubscriptionManager{
		jetStream:     js,
		logger:        logger.NewNopLogger(),
		subscriptions: make(map[string]*Subscription),
		consumerCfg: &config.ConsumerConfig{
			FetchBatchSize: 1,
			FetchTimeout:   100 * time.Millisecond,
			AckWaitTimeout: 30 * time.Second,
		},
	}

	err = sm.AddAndStartSubscription(ctx, "EVENTS", consumerName, "events.>", 1)
	require.Error(t, err, "AddAndStartSubscription must surface the PullExpiry validation error")

	sm.mu.RLock()
	_, leaked := sm.subscriptions["events.>"]
	sm.mu.RUnlock()
	assert.False(t, leaked,
		"sm.subscriptions must NOT retain the subject when startSubscription failed — otherwise retries skip via the already-exists fast path")
}

// TestNATSBrokerAddAndStart_RollsBackOnSMFailure verifies that when
// SubscriptionManager.AddAndStartSubscription fails, NATSBroker.AddAndStartSubscription
// untracks the subject in b.consumers AND deletes the durable consumer in
// JetStream — otherwise CreateConsumerForSubject's "exists" fast-path
// (nats.go:142) silently masks the failure on subsequent attempts.
func TestNATSBrokerAddAndStart_RollsBackOnSMFailure(t *testing.T) {
	nc, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()

	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	processor := rule.NewProcessor(logger.NewNopLogger(), nil, nil, nil)
	natsBroker := newFullBroker(t, nc, js, processor)

	natsBroker.subscriptionMgr.consumerCfg.FetchTimeout = 100 * time.Millisecond

	err = natsBroker.AddAndStartSubscription("events.foo")
	require.Error(t, err, "AddAndStartSubscription must surface the start failure")

	natsBroker.consumersMu.RLock()
	_, leaked := natsBroker.consumers["events.foo"]
	natsBroker.consumersMu.RUnlock()
	assert.False(t, leaked,
		"NATSBroker.consumers must NOT retain a subject whose subscription start failed")

	consumerName := natsBroker.GenerateConsumerName("events.foo")
	assert.False(t, consumerExists(t, js, "EVENTS", consumerName),
		"durable consumer must be deleted when start failure leaves it orphaned")
}
