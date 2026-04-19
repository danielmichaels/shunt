package natsutil

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"log/slog"

	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/metrics"
	"github.com/danielmichaels/shunt/internal/rule"
	"github.com/danielmichaels/shunt/internal/trace"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const publishRetryJitter = 25 * time.Millisecond

// Publisher publishes rule NATS actions with retry, per-rule mode override,
// exponential backoff, and optional metrics instrumentation.
type Publisher struct {
	nc         *nats.Conn
	js         jetstream.JetStream
	cfg        *config.PublishConfig
	metrics    *metrics.Metrics
	logger     *slog.Logger
}

func NewPublisher(
	nc *nats.Conn,
	js jetstream.JetStream,
	cfg *config.PublishConfig,
	m *metrics.Metrics,
	log *slog.Logger,
) *Publisher {
	return &Publisher{nc: nc, js: js, cfg: cfg, metrics: m, logger: log}
}

// PublishAction publishes action to NATS with retry and per-rule mode override.
func (p *Publisher) PublishAction(ctx context.Context, action *rule.NATSAction, ruleName, traceID string) error {
	maxRetries := p.cfg.MaxRetries
	baseDelay := p.cfg.RetryBaseDelay
	publishMode := p.cfg.Mode
	if action.Mode != "" {
		p.logger.Debug("per-rule mode override active",
			"subject", action.Subject,
			"ruleMode", action.Mode,
			"globalMode", p.cfg.Mode)
		publishMode = action.Mode
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if ctx.Err() != nil {
			return fmt.Errorf("context cancelled: %w", ctx.Err())
		}

		var err error
		switch publishMode {
		case "core":
			err = p.publishCore(action, traceID)
		case "jetstream":
			err = p.publishJetStream(ctx, action, traceID)
		default:
			p.logger.Warn("unknown publish mode, falling back to jetstream", "mode", publishMode)
			err = p.publishJetStream(ctx, action, traceID)
		}

		if err == nil {
			return nil
		}

		lastErr = err
		p.logger.Warn("action publish failed, will retry",
			"attempt", attempt+1, "maxRetries", maxRetries, "subject", action.Subject, "error", err)
		if p.metrics != nil {
			p.metrics.IncActionPublishFailures(ruleName)
		}

		if attempt == maxRetries-1 {
			break
		}

		delay := baseDelay * time.Duration(1<<attempt)
		jitter := time.Duration(rand.Intn(int(publishRetryJitter.Milliseconds()))) * time.Millisecond

		select {
		case <-ctx.Done():
			return fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
		case <-time.After(delay + jitter):
		}
	}

	return fmt.Errorf("failed to publish after %d attempts (mode: %s): %w", maxRetries, publishMode, lastErr)
}

func (p *Publisher) publishJetStream(ctx context.Context, action *rule.NATSAction, traceID string) error {
	msg := buildMsg(action, traceID)

	ackF, err := p.js.PublishMsgAsync(msg)
	if err != nil {
		return fmt.Errorf("jetstream async publish failed on send: %w", err)
	}

	pubCtx, cancel := context.WithTimeout(ctx, p.cfg.AckTimeout)
	defer cancel()

	select {
	case <-ackF.Ok():
		return nil
	case err := <-ackF.Err():
		if errors.Is(err, nats.ErrNoResponders) {
			return fmt.Errorf("jetstream publish failed: no stream is configured for action subject '%s'", action.Subject)
		}
		return fmt.Errorf("jetstream async publish failed on ack: %w", err)
	case <-pubCtx.Done():
		return fmt.Errorf("timeout waiting for publish acknowledgement: %w", pubCtx.Err())
	}
}

func (p *Publisher) publishCore(action *rule.NATSAction, traceID string) error {
	var payloadBytes []byte
	if action.Passthrough {
		payloadBytes = action.RawPayload
	} else {
		payloadBytes = []byte(action.Payload)
	}

	// Fast path: no headers and no trace ID
	if len(action.Headers) == 0 && traceID == "" {
		if err := p.nc.Publish(action.Subject, payloadBytes); err != nil {
			return fmt.Errorf("core nats publish failed: %w", err)
		}
		return nil
	}

	msg := nats.NewMsg(action.Subject)
	msg.Data = payloadBytes
	msg.Header = make(nats.Header)
	for key, value := range action.Headers {
		msg.Header.Set(key, value)
	}
	if traceID != "" {
		msg.Header.Set(trace.NATSHeader, traceID)
	}

	if err := p.nc.PublishMsg(msg); err != nil {
		return fmt.Errorf("core nats publish with headers failed: %w", err)
	}
	return nil
}

func buildMsg(action *rule.NATSAction, traceID string) *nats.Msg {
	var payloadBytes []byte
	if action.Passthrough {
		payloadBytes = action.RawPayload
	} else {
		payloadBytes = []byte(action.Payload)
	}

	msg := nats.NewMsg(action.Subject)
	msg.Data = payloadBytes

	if len(action.Headers) > 0 {
		msg.Header = make(nats.Header)
		for key, value := range action.Headers {
			msg.Header.Set(key, value)
		}
	}

	if traceID != "" {
		if msg.Header == nil {
			msg.Header = make(nats.Header)
		}
		msg.Header.Set(trace.NATSHeader, traceID)
	}

	return msg
}
