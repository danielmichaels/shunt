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

const testHTTPRuleYAML = `
- name: test-http-rule
  trigger:
    http:
      path: /webhook/test
      method: POST
  action:
    nats:
      subject: events.test
      payload: '{"received": true}'
`

const testNATSRuleYAML = `
- name: test-nats-rule
  trigger:
    nats:
      subject: sensors.>
  action:
    nats:
      subject: events.sensors
      payload: '{"forwarded": true}'
`

// TestHTTPRuleFromKV_ProcessHTTPRoutes verifies that an HTTP-triggered rule pushed
// to KV is correctly routed by ProcessHTTP — this was the production 404 bug.
func TestHTTPRuleFromKV_ProcessHTTPRoutes(t *testing.T) {
	_, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "rules"})
	require.NoError(t, err)

	_, err = kv.Put(ctx, "http-rules", []byte(testHTTPRuleYAML))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newMinimalBroker(t, js)
	rulesLoader := rule.NewRulesLoader(log, nil)

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)

	err = kvManager.Watch(ctx)
	require.NoError(t, err)

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	// Key assertion: HTTP rule from KV must be routable via ProcessHTTP
	actions, err := processor.ProcessHTTP("/webhook/test", "POST", []byte(`{}`), nil)
	require.NoError(t, err)
	assert.NotEmpty(t, actions, "ProcessHTTP should match the KV-loaded HTTP rule; got 404 in production before fix")
	if len(actions) > 0 {
		assert.Equal(t, "test-http-rule", actions[0].RuleName)
	}
}

// TestHTTPRuleFromKV_UnknownPathReturnsNil verifies non-matching paths return nothing.
func TestHTTPRuleFromKV_UnknownPathReturnsNil(t *testing.T) {
	_, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "rules"})
	require.NoError(t, err)

	_, err = kv.Put(ctx, "http-rules", []byte(testHTTPRuleYAML))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newMinimalBroker(t, js)
	rulesLoader := rule.NewRulesLoader(log, nil)

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	actions, err := processor.ProcessHTTP("/not/registered", "POST", []byte(`{}`), nil)
	require.NoError(t, err)
	assert.Empty(t, actions, "unregistered path should return no actions")
}

// TestHTTPRuleFromKV_UpdatedOnKVChange verifies live rule updates propagate immediately.
func TestHTTPRuleFromKV_UpdatedOnKVChange(t *testing.T) {
	_, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "rules"})
	require.NoError(t, err)

	_, err = kv.Put(ctx, "http-rules", []byte(testHTTPRuleYAML))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newMinimalBroker(t, js)
	rulesLoader := rule.NewRulesLoader(log, nil)

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	// Verify initial rule works
	actions, err := processor.ProcessHTTP("/webhook/test", "POST", []byte(`{}`), nil)
	require.NoError(t, err)
	assert.NotEmpty(t, actions)

	// Push a different rule to KV
	const updatedRule = `
- name: test-http-rule-v2
  trigger:
    http:
      path: /webhook/updated
      method: POST
  action:
    nats:
      subject: events.updated
      payload: '{"updated": true}'
`
	_, err = kv.Put(ctx, "http-rules", []byte(updatedRule))
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		acts, err := processor.ProcessHTTP("/webhook/updated", "POST", []byte(`{}`), nil)
		return err == nil && len(acts) > 0
	}, 3*time.Second, 50*time.Millisecond, "updated rule should be routable after KV change")

	// Old path must no longer match
	actions, err = processor.ProcessHTTP("/webhook/test", "POST", []byte(`{}`), nil)
	require.NoError(t, err)
	assert.Empty(t, actions, "old path should not match after rule update")
}

// TestNATSRuleFromKV_StillRoutesAfterRefactor ensures NATS routing works when stream exists.
func TestNATSRuleFromKV_StillRoutesAfterRefactor(t *testing.T) {
	_, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "SENSORS",
		Subjects: []string{"sensors.>"},
	})
	require.NoError(t, err)

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "rules"})
	require.NoError(t, err)

	_, err = kv.Put(ctx, "nats-rules", []byte(testNATSRuleYAML))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newMinimalBroker(t, js)
	rulesLoader := rule.NewRulesLoader(log, nil)

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	actions, err := processor.ProcessForSubscription("sensors.>", "sensors.tank.001", []byte(`{"level":42}`), nil)
	require.NoError(t, err)
	assert.NotEmpty(t, actions, "NATS rule from KV should still match")
	if len(actions) > 0 {
		assert.Equal(t, "test-nats-rule", actions[0].RuleName)
	}
}

// TestNATSRuleFromKV_RejectedWhenNoStream verifies rules are rejected when
// trigger subject has no matching stream.
func TestNATSRuleFromKV_RejectedWhenNoStream(t *testing.T) {
	_, js, cleanup := setupNATS(t)
	defer cleanup()

	ctx := context.Background()
	log := logger.NewNopLogger()

	// Create a stream that does NOT cover sensors.>
	_, err := js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	require.NoError(t, err)

	kv, err := js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: "rules"})
	require.NoError(t, err)

	// Push a rule with trigger subject "sensors.>" — no stream covers it
	_, err = kv.Put(ctx, "nats-rules", []byte(testNATSRuleYAML))
	require.NoError(t, err)

	processor := rule.NewProcessor(log, nil, nil, nil)
	natsBroker := newMinimalBroker(t, js)
	rulesLoader := rule.NewRulesLoader(log, nil)

	kvManager := NewRuleKVManager("rules", false, processor, natsBroker, rulesLoader, log)
	require.NoError(t, kvManager.Watch(ctx))

	readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, kvManager.WaitReady(readyCtx))

	// Rule should NOT be loaded — stream validation should have rejected it
	actions, err := processor.ProcessForSubscription("sensors.>", "sensors.tank.001", []byte(`{"level":42}`), nil)
	require.NoError(t, err)
	assert.Empty(t, actions, "rule with no matching stream should be rejected and not loaded")
}
