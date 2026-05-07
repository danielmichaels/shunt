package broker

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/rule"
)

const ruleEventsWide = `
- name: rule-wide
  trigger:
    nats:
      subject: events.>
  action:
    nats:
      subject: events.processed
      payload: '{}'
`

const ruleEventsNarrow = `
- name: rule-narrow
  trigger:
    nats:
      subject: events.specific.*
  action:
    nats:
      subject: events.processed
      payload: '{}'
`

const sharedNATSRuleA = `
- name: rule-a
  trigger:
    nats:
      subject: events.>
  action:
    nats:
      subject: events.processed
      payload: '{}'
`

const sharedNATSRuleB = `
- name: rule-b
  trigger:
    nats:
      subject: events.>
  action:
    nats:
      subject: events.audit
      payload: '{}'
`

func TestHandleRulePut_RemovesObsoleteSubject(t *testing.T) {
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

	_, err = kv.Put(ctx, "rule-key", []byte(ruleEventsWide))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newFullBroker(t, nc, js, processor)
	rulesLoader := rule.NewRulesLoader(log, nil)

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	wideConsumer := natsBroker.GenerateConsumerName("events.>")
	require.Eventually(t, func() bool {
		return consumerExists(t, js, "EVENTS", wideConsumer)
	}, 3*time.Second, 50*time.Millisecond, "wide-subject consumer should be created on initial PUT")

	_, err = kv.Put(ctx, "rule-key", []byte(ruleEventsNarrow))
	require.NoError(t, err)

	narrowConsumer := natsBroker.GenerateConsumerName("events.specific.*")
	require.Eventually(t, func() bool {
		return consumerExists(t, js, "EVENTS", narrowConsumer)
	}, 3*time.Second, 50*time.Millisecond, "narrow-subject consumer should be created after subject change")

	assert.Eventually(t, func() bool {
		return !consumerExists(t, js, "EVENTS", wideConsumer)
	}, 3*time.Second, 50*time.Millisecond, "old wide-subject consumer must be deleted after subject change")
}

func TestHandleRulePut_KeepsSubjectStillUsedByOtherKey(t *testing.T) {
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

	_, err = kv.Put(ctx, "key-a", []byte(sharedNATSRuleA))
	require.NoError(t, err)
	_, err = kv.Put(ctx, "key-b", []byte(sharedNATSRuleB))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newFullBroker(t, nc, js, processor)
	rulesLoader := rule.NewRulesLoader(log, nil)

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	sharedConsumer := natsBroker.GenerateConsumerName("events.>")
	require.Eventually(t, func() bool {
		return consumerExists(t, js, "EVENTS", sharedConsumer)
	}, 3*time.Second, 50*time.Millisecond, "shared-subject consumer should exist after both keys are pushed")

	_, err = kv.Put(ctx, "key-a", []byte(ruleEventsNarrow))
	require.NoError(t, err)

	narrowConsumer := natsBroker.GenerateConsumerName("events.specific.*")
	require.Eventually(t, func() bool {
		return consumerExists(t, js, "EVENTS", narrowConsumer)
	}, 3*time.Second, 50*time.Millisecond, "narrow consumer for the updated key should be created")

	assert.Never(t, func() bool {
		return !consumerExists(t, js, "EVENTS", sharedConsumer)
	}, 500*time.Millisecond, 50*time.Millisecond,
		"consumer for shared subject must persist while another KV key still references it")
}

func TestRemoveSubscription_DeletesDurableConsumer(t *testing.T) {
	nc, js, cleanup := setupNATS(t)
	defer cleanup()

	log := logger.NewNopLogger()

	_, err := js.CreateStream(context.Background(), jetstream.StreamConfig{
		Name:     "FOO",
		Subjects: []string{"foo.>"},
	})
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newFullBroker(t, nc, js, processor)

	require.NoError(t, natsBroker.AddAndStartSubscription("foo.bar"))

	consumerName := natsBroker.GenerateConsumerName("foo.bar")
	require.True(t, consumerExists(t, js, "FOO", consumerName), "consumer should exist after AddAndStartSubscription")

	natsBroker.RemoveSubscription("foo.bar")

	assert.Eventually(t, func() bool {
		return !consumerExists(t, js, "FOO", consumerName)
	}, 3*time.Second, 50*time.Millisecond, "RemoveSubscription must delete the durable consumer")
}

func TestRemoveSubscription_IdempotentOnMissingConsumer(t *testing.T) {
	nc, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "FOO",
		Subjects: []string{"foo.>"},
	})
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newFullBroker(t, nc, js, processor)

	assert.NotPanics(t, func() {
		natsBroker.RemoveSubscription("never.subscribed")
	}, "removing an unknown subject must not panic")

	require.NoError(t, natsBroker.AddAndStartSubscription("foo.bar"))
	consumerName := natsBroker.GenerateConsumerName("foo.bar")
	require.True(t, consumerExists(t, js, "FOO", consumerName))

	delCtx, delCancel := context.WithTimeout(ctx, 2*time.Second)
	defer delCancel()
	require.NoError(t, js.DeleteConsumer(delCtx, "FOO", consumerName))

	assert.NotPanics(t, func() {
		natsBroker.RemoveSubscription("foo.bar")
	}, "RemoveSubscription must be idempotent when consumer was already deleted out-of-band")
}

func TestHandleRuleDelete_DeletesDurableConsumer(t *testing.T) {
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

	_, err = kv.Put(ctx, "rule-key", []byte(ruleEventsWide))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newFullBroker(t, nc, js, processor)
	rulesLoader := rule.NewRulesLoader(log, nil)

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	consumerName := natsBroker.GenerateConsumerName("events.>")
	require.Eventually(t, func() bool {
		return consumerExists(t, js, "EVENTS", consumerName)
	}, 3*time.Second, 50*time.Millisecond, "consumer should be created on initial PUT")

	require.NoError(t, kv.Delete(ctx, "rule-key"))

	assert.Eventually(t, func() bool {
		return !consumerExists(t, js, "EVENTS", consumerName)
	}, 3*time.Second, 50*time.Millisecond, "consumer must be deleted from JetStream after KV delete")
}

func TestRemoveOutboundSubscription_DeletesDurableConsumer(t *testing.T) {
	_, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "OUTBOUND",
		Subjects: []string{"outbound.>"},
	})
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newMinimalBroker(t, js)
	outbound := newTestOutboundClient(t, js, processor)

	streamName, consumerName, err := natsBroker.CreateOutboundConsumer("outbound.events")
	require.NoError(t, err)
	require.Equal(t, "OUTBOUND", streamName)

	require.NoError(t, outbound.AddOutboundSubscription(ctx, streamName, consumerName, "outbound.events"))

	require.True(t, consumerExists(t, js, streamName, consumerName), "outbound consumer should exist after AddOutboundSubscription")

	outbound.RemoveOutboundSubscription("outbound.events")

	assert.Eventually(t, func() bool {
		return !consumerExists(t, js, streamName, consumerName)
	}, 3*time.Second, 50*time.Millisecond, "RemoveOutboundSubscription must delete the durable consumer")
}
