package broker

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/danielmichaels/shunt/internal/rule"
	"github.com/nats-io/nats.go/jetstream"
)

type RuleKVManager struct {
	kvBucket      string
	autoProvision bool
	processor     *rule.Processor
	broker        *NATSBroker
	rulesLoader   *rule.RulesLoader
	logger        *logger.Logger
	currentRules  map[string][]rule.Rule
	mu            sync.Mutex
	ready         chan struct{}
	readyOnce     sync.Once
	watcher       jetstream.KeyWatcher
	watchOnce     sync.Once
	watchErr      error
}

func NewRuleKVManager(
	kvBucket string,
	autoProvision bool,
	processor *rule.Processor,
	broker *NATSBroker,
	rulesLoader *rule.RulesLoader,
	log *logger.Logger,
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
		go m.processWatchUpdates(ctx, watcher)

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

	m.mu.Lock()

	previousSubjects := m.collectNATSSubjects(m.currentRules[key])
	m.currentRules[key] = rules
	m.pushRulesToProcessor()

	newSubjects := m.collectNATSSubjects(rules)

	m.mu.Unlock()

	for subject := range newSubjects {
		if !previousSubjects[subject] {
			if err := m.broker.AddAndStartSubscription(subject); err != nil {
				m.logger.Error("failed to start subscription for new subject",
					"key", key, "subject", subject, "error", err)
			}
		}
	}

	m.logger.Info("KV rules updated",
		"key", key, "ruleCount", len(rules), "revision", revision)
}

func (m *RuleKVManager) handleRuleDelete(key string) {
	m.mu.Lock()

	oldRules, existed := m.currentRules[key]
	if !existed {
		m.mu.Unlock()
		return
	}

	oldSubjects := m.collectNATSSubjects(oldRules)
	delete(m.currentRules, key)
	m.pushRulesToProcessor()

	stillNeeded := make(map[string]bool)
	for _, rules := range m.currentRules {
		for subject := range m.collectNATSSubjects(rules) {
			stillNeeded[subject] = true
		}
	}

	m.mu.Unlock()

	for subject := range oldSubjects {
		if !stillNeeded[subject] {
			m.broker.RemoveSubscription(subject)
		}
	}

	m.logger.Info("KV rules deleted", "key", key)
}

func (m *RuleKVManager) pushRulesToProcessor() {
	merged := make(map[string][]*rule.Rule)
	for _, rules := range m.currentRules {
		for i := range rules {
			r := &rules[i]
			if r.Trigger.NATS != nil {
				merged[r.Trigger.NATS.Subject] = append(merged[r.Trigger.NATS.Subject], r)
			}
		}
	}
	m.processor.ReplaceRules(merged)
}

func (m *RuleKVManager) WaitReady(ctx context.Context) error {
	select {
	case <-m.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *RuleKVManager) collectNATSSubjects(rules []rule.Rule) map[string]bool {
	subjects := make(map[string]bool)
	for _, r := range rules {
		if r.Trigger.NATS != nil {
			subjects[r.Trigger.NATS.Subject] = true
		}
	}
	return subjects
}