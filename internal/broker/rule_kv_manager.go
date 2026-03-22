package broker

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/danielmichaels/shunt/internal/rule"
	"github.com/nats-io/nats.go/jetstream"
	"log/slog"
)

type OutboundSubscriber interface {
	AddOutboundSubscription(ctx context.Context, streamName, consumerName, subject string) error
	RemoveOutboundSubscription(subject string)
}

type RuleKVManager struct {
	kvBucket            string
	autoProvision       bool
	processor           *rule.Processor
	broker              *NATSBroker
	rulesLoader         *rule.RulesLoader
	logger              *slog.Logger
	currentRules        map[string][]rule.Rule
	outboundSubscriber  OutboundSubscriber
	outboundSet         bool
	mu                  sync.Mutex
	wg                  sync.WaitGroup
	ready               chan struct{}
	readyOnce           sync.Once
	watcher             jetstream.KeyWatcher
	watchOnce           sync.Once
	watchErr            error
}

func NewRuleKVManager(
	kvBucket string,
	autoProvision bool,
	processor *rule.Processor,
	broker *NATSBroker,
	rulesLoader *rule.RulesLoader,
	log *slog.Logger,
) *RuleKVManager {
	return &RuleKVManager{
		kvBucket:      kvBucket,
		autoProvision: autoProvision,
		processor:     processor,
		broker:        broker,
		rulesLoader:   rulesLoader,
		logger:        log,
		currentRules:  make(map[string][]rule.Rule),
		ready:         make(chan struct{}),
	}
}

func (m *RuleKVManager) Watch(ctx context.Context) error {
	m.watchOnce.Do(func() {
		js := m.broker.GetJetStream()

		store, err := js.KeyValue(ctx, m.kvBucket)
		if err != nil {
			if errors.Is(err, jetstream.ErrBucketNotFound) && m.autoProvision {
				store, err = js.CreateKeyValue(ctx, jetstream.KeyValueConfig{Bucket: m.kvBucket})
				if err != nil {
					m.watchErr = fmt.Errorf("failed to auto-create rules KV bucket %q: %w", m.kvBucket, err)
					return
				}
				m.logger.Info("auto-provisioned rules KV bucket", "bucket", m.kvBucket)
			} else {
				m.watchErr = fmt.Errorf("failed to open KV bucket %q: %w", m.kvBucket, err)
				return
			}
		}

		watcher, err := store.WatchAll(ctx)
		if err != nil {
			m.watchErr = fmt.Errorf("failed to create watcher for bucket %q: %w", m.kvBucket, err)
			return
		}

		m.watcher = watcher
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.processWatchUpdates(ctx, watcher)
		}()

		m.logger.Info("rule KV watcher started", "bucket", m.kvBucket)
	})
	return m.watchErr
}

func (m *RuleKVManager) Stop() {
	if m.watcher != nil {
		if err := m.watcher.Stop(); err != nil {
			m.logger.Error("failed to stop rule KV watcher", "error", err)
		}
	}
	m.wg.Wait()
}

func (m *RuleKVManager) processWatchUpdates(ctx context.Context, watcher jetstream.KeyWatcher) {
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-watcher.Updates():
			if entry == nil {
				m.readyOnce.Do(func() { close(m.ready) })
				continue
			}

			switch entry.Operation() {
			case jetstream.KeyValuePut:
				m.handleRulePut(entry.Key(), entry.Value(), entry.Revision())
			case jetstream.KeyValueDelete, jetstream.KeyValuePurge:
				m.handleRuleDelete(entry.Key())
			}
		}
	}
}

func (m *RuleKVManager) handleRulePut(key string, value []byte, revision uint64) {
	rules, err := rule.ParseYAML(value)
	if err != nil {
		m.logger.Error("failed to parse rules from KV",
			"key", key, "revision", revision, "error", err)
		return
	}

	for i := range rules {
		m.rulesLoader.ExpandEnvironmentVariables(&rules[i])
		if err := m.rulesLoader.ValidateRule(&rules[i]); err != nil {
			m.logger.Error("invalid rule from KV, keeping previous rules for this key",
				"key", key, "ruleIndex", i, "revision", revision, "error", err)
			return
		}
	}

	if err := m.broker.RefreshStreams(); err != nil {
		m.logger.Warn("failed to refresh stream list before validating rules",
			"key", key, "error", err)
	}

	resolver := m.broker.GetStreamResolver()
	if streamErrs := resolver.ValidateRulesHaveStreams(rules); len(streamErrs) > 0 {
		for _, e := range streamErrs {
			m.logger.Error("stream validation failed, rejecting all rules for this key",
				"key", key, "revision", revision, "error", e)
		}
		return
	}

	m.mu.Lock()

	previousNATSSubjects := m.collectNATSSubjects(m.currentRules[key])
	previousHTTPSubjects := m.collectHTTPActionSubjects(m.currentRules[key])
	m.currentRules[key] = rules
	m.pushRulesToProcessor()

	newNATSSubjects := m.collectNATSSubjects(rules)
	newHTTPSubjects := m.collectHTTPActionSubjects(rules)
	outbound := m.outboundSubscriber

	m.mu.Unlock()

	for subject := range newNATSSubjects {
		if !previousNATSSubjects[subject] {
			if err := m.broker.AddAndStartSubscription(subject); err != nil {
				if errors.Is(err, ErrNoStreamFound) {
					m.logger.Warn("rule references subject with no JetStream stream, skipping subscription",
						"key", key, "subject", subject,
						"hint", "create a stream covering this subject or remove the rule")
				} else {
					m.logger.Error("failed to start subscription for new subject",
						"key", key, "subject", subject, "error", err)
				}
			}
		}
	}

	for subject := range newHTTPSubjects {
		if !previousHTTPSubjects[subject] {
			m.doAddOutboundSubscription(outbound, subject)
		}
	}

	m.logger.Debug("KV rules updated",
		"key", key, "ruleCount", len(rules), "revision", revision)
}

func (m *RuleKVManager) handleRuleDelete(key string) {
	m.mu.Lock()

	oldRules, existed := m.currentRules[key]
	if !existed {
		m.mu.Unlock()
		return
	}

	oldNATSSubjects := m.collectNATSSubjects(oldRules)
	oldHTTPSubjects := m.collectHTTPActionSubjects(oldRules)
	delete(m.currentRules, key)
	m.pushRulesToProcessor()

	stillNeededNATS := make(map[string]bool)
	stillNeededHTTP := make(map[string]bool)
	for _, rules := range m.currentRules {
		for subject := range m.collectNATSSubjects(rules) {
			stillNeededNATS[subject] = true
		}
		for subject := range m.collectHTTPActionSubjects(rules) {
			stillNeededHTTP[subject] = true
		}
	}
	outbound := m.outboundSubscriber

	m.mu.Unlock()

	for subject := range oldNATSSubjects {
		if !stillNeededNATS[subject] {
			m.broker.RemoveSubscription(subject)
		}
	}

	for subject := range oldHTTPSubjects {
		if !stillNeededHTTP[subject] {
			m.doRemoveOutboundSubscription(outbound, subject)
		}
	}

	m.logger.Info("KV rules deleted", "key", key)
}

func (m *RuleKVManager) pushRulesToProcessor() {
	natsRules := make(map[string][]*rule.Rule)
	httpRules := make(map[string][]*rule.Rule)
	for _, rules := range m.currentRules {
		for i := range rules {
			r := &rules[i]
			if r.Trigger.NATS != nil {
				natsRules[r.Trigger.NATS.Subject] = append(natsRules[r.Trigger.NATS.Subject], r)
			}
			if r.Trigger.HTTP != nil {
				httpRules[r.Trigger.HTTP.Path] = append(httpRules[r.Trigger.HTTP.Path], r)
			}
		}
	}
	m.processor.ReplaceRules(natsRules)
	m.processor.ReplaceHTTPRules(httpRules)
}

func (m *RuleKVManager) WaitReady(ctx context.Context) error {
	select {
	case <-m.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SetOutboundSubscriber retroactively registers already-loaded HTTP action rules.
func (m *RuleKVManager) SetOutboundSubscriber(sub OutboundSubscriber) {
	m.mu.Lock()
	m.outboundSubscriber = sub
	m.outboundSet = true

	var httpSubjects []string
	for _, rules := range m.currentRules {
		for subject := range m.collectHTTPActionSubjects(rules) {
			httpSubjects = append(httpSubjects, subject)
		}
	}
	m.mu.Unlock()

	for _, subject := range httpSubjects {
		m.doAddOutboundSubscription(sub, subject)
	}
}

func (m *RuleKVManager) doAddOutboundSubscription(sub OutboundSubscriber, subject string) {
	if sub == nil {
		m.mu.Lock()
		wasSet := m.outboundSet
		m.mu.Unlock()
		if wasSet {
			m.logger.Warn("HTTP action rule has no outbound subscriber, skipping",
				"subject", subject)
		} else {
			m.logger.Debug("HTTP action rule deferred until outbound subscriber set",
				"subject", subject)
		}
		return
	}

	streamName, consumerName, err := m.broker.CreateOutboundConsumer(subject)
	if err != nil {
		m.logger.Error("failed to create outbound consumer",
			"subject", subject, "error", err)
		return
	}

	if err := sub.AddOutboundSubscription(m.broker.ctx, streamName, consumerName, subject); err != nil {
		m.logger.Error("failed to add outbound subscription",
			"subject", subject, "error", err)
	}
}

func (m *RuleKVManager) doRemoveOutboundSubscription(sub OutboundSubscriber, subject string) {
	if sub == nil {
		return
	}
	sub.RemoveOutboundSubscription(subject)
}

func (m *RuleKVManager) collectNATSSubjects(rules []rule.Rule) map[string]bool {
	subjects := make(map[string]bool)
	for _, r := range rules {
		if r.Trigger.NATS != nil && r.Action.NATS != nil {
			subjects[r.Trigger.NATS.Subject] = true
		}
	}
	return subjects
}

func (m *RuleKVManager) collectHTTPActionSubjects(rules []rule.Rule) map[string]bool {
	subjects := make(map[string]bool)
	for _, r := range rules {
		if r.Trigger.NATS != nil && r.Action.HTTP != nil {
			subjects[r.Trigger.NATS.Subject] = true
		}
	}
	return subjects
}
