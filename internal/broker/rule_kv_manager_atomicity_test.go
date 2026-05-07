package broker

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/rule"
)

// failingSubscriber wraps a real broker but injects a controllable failure for
// AddAndStartSubscription on specific subjects. RemoveSubscription is also
// recorded so tests can assert what was torn down.
type failingSubscriber struct {
	inner    *NATSBroker
	mu       sync.Mutex
	failOn   map[string]error
	removed  []string
}

func (f *failingSubscriber) AddAndStartSubscription(subject string) error {
	f.mu.Lock()
	err, fail := f.failOn[subject]
	f.mu.Unlock()
	if fail {
		return err
	}
	return f.inner.AddAndStartSubscription(subject)
}

func (f *failingSubscriber) RemoveSubscription(subject string) {
	f.mu.Lock()
	f.removed = append(f.removed, subject)
	f.mu.Unlock()
	f.inner.RemoveSubscription(subject)
}

func (f *failingSubscriber) wasRemoved(subject string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.removed, subject)
}

const ruleEventsFoo = `
- name: rule-foo
  trigger:
    nats:
      subject: events.foo
  action:
    nats:
      subject: events.processed
      payload: '{}'
`

const ruleEventsBar = `
- name: rule-bar
  trigger:
    nats:
      subject: events.bar
  action:
    nats:
      subject: events.processed
      payload: '{}'
`

// TestHandleRulePut_RollsBackWhenAddFails verifies that if AddAndStartSubscription
// fails for the new subject during a rule update, the previous subject's consumer
// is NOT torn down — i.e., the rule update is atomic and we keep the working
// state on transient broker errors.
func TestHandleRulePut_RollsBackWhenAddFails(t *testing.T) {
	nc, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "rules"})
	require.NoError(t, err)

	_, err = kv.Put(ctx, "rule-key", []byte(ruleEventsFoo))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newFullBroker(t, nc, js, processor)
	rulesLoader := rule.NewRulesLoader(log, nil)

	subscriber := &failingSubscriber{inner: natsBroker, failOn: map[string]error{}}

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	kvManager.subscriber = subscriber

	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	fooConsumer := natsBroker.GenerateConsumerName("events.foo")
	require.Eventually(t, func() bool {
		return consumerExists(t, js, "EVENTS", fooConsumer)
	}, 3*time.Second, 50*time.Millisecond, "consumer for events.foo must exist before update")

	subscriber.mu.Lock()
	subscriber.failOn["events.bar"] = errors.New("simulated transient broker failure")
	subscriber.mu.Unlock()

	_, err = kv.Put(ctx, "rule-key", []byte(ruleEventsBar))
	require.NoError(t, err)

	barConsumer := natsBroker.GenerateConsumerName("events.bar")

	assert.Never(t, func() bool {
		return subscriber.wasRemoved("events.foo")
	}, 1*time.Second, 50*time.Millisecond,
		"events.foo subscription must NOT be removed when the new subject's add fails")

	assert.True(t, consumerExists(t, js, "EVENTS", fooConsumer),
		"events.foo consumer must still exist after a failed rule update — rollback")
	assert.False(t, consumerExists(t, js, "EVENTS", barConsumer),
		"events.bar consumer must NOT exist when its add was forced to fail")
}

// failingOutbound implements OutboundSubscriber and can be told to fail
// AddOutboundSubscription for specific subjects.
type failingOutbound struct {
	mu      sync.Mutex
	failOn  map[string]error
	added   []string
	removed []string
}

func (f *failingOutbound) AddOutboundSubscription(_ context.Context, _, _, subject string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.failOn[subject]; ok {
		return err
	}
	f.added = append(f.added, subject)
	return nil
}

func (f *failingOutbound) RemoveOutboundSubscription(subject string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removed = append(f.removed, subject)
}

func (f *failingOutbound) wasAdded(subject string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.added, subject)
}

func (f *failingOutbound) wasRemoved(subject string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Contains(f.removed, subject)
}

const httpRuleEventsFoo = `
- name: rule-foo-http
  trigger:
    nats:
      subject: events.foo
  action:
    http:
      url: "https://example.com/foo"
      method: POST
`

const httpRuleEventsBar = `
- name: rule-bar-http
  trigger:
    nats:
      subject: events.bar
  action:
    http:
      url: "https://example.com/bar"
      method: POST
`

// TestHandleRulePut_RollsBackWhenOutboundAddFails verifies that when the
// outbound subscriber rejects the new subject, the existing outbound
// subscription is NOT torn down — same atomicity guarantee as the NATS path.
func TestHandleRulePut_RollsBackWhenOutboundAddFails(t *testing.T) {
	_, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "rules"})
	require.NoError(t, err)

	_, err = kv.Put(ctx, "rule-key", []byte(httpRuleEventsFoo))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newMinimalBroker(t, js)
	rulesLoader := rule.NewRulesLoader(log, nil)

	outbound := &failingOutbound{failOn: map[string]error{}}

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	kvManager.subscriber = noopSubscriber{}
	kvManager.SetOutboundSubscriber(outbound)
	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	require.Eventually(t, func() bool {
		return outbound.wasAdded("events.foo")
	}, 3*time.Second, 50*time.Millisecond, "events.foo outbound must be registered before update")

	outbound.mu.Lock()
	outbound.failOn["events.bar"] = errors.New("simulated outbound add failure")
	outbound.mu.Unlock()

	_, err = kv.Put(ctx, "rule-key", []byte(httpRuleEventsBar))
	require.NoError(t, err)

	assert.Never(t, func() bool {
		return outbound.wasRemoved("events.foo")
	}, 1*time.Second, 50*time.Millisecond,
		"events.foo outbound must NOT be torn down when the new subject's outbound add fails")

	assert.False(t, outbound.wasAdded("events.bar"),
		"events.bar must NOT appear as added when its add was forced to fail")
}

// TestHandleRulePut_OutboundAddFailure_CleansUpOrphanedConsumer verifies that
// when sub.AddOutboundSubscription rejects a subject, the JetStream consumer
// that doAddOutboundSubscription created moments earlier (via
// CreateOutboundConsumer) is cleaned up — otherwise it lingers in the stream
// as an orphan.
func TestHandleRulePut_OutboundAddFailure_CleansUpOrphanedConsumer(t *testing.T) {
	_, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "rules"})
	require.NoError(t, err)

	_, err = kv.Put(ctx, "rule-key", []byte(httpRuleEventsFoo))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newMinimalBroker(t, js)
	rulesLoader := rule.NewRulesLoader(log, nil)

	outbound := &failingOutbound{
		failOn: map[string]error{
			"events.foo": errors.New("simulated outbound add failure"),
		},
	}

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	kvManager.subscriber = noopSubscriber{}
	kvManager.SetOutboundSubscriber(outbound)
	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	consumerName := natsBroker.GenerateConsumerName("outbound-events.foo")

	assert.Eventually(t, func() bool {
		return !consumerExists(t, js, "EVENTS", consumerName)
	}, 2*time.Second, 50*time.Millisecond,
		"orphaned outbound consumer must be cleaned up after AddOutboundSubscription fails")
}
