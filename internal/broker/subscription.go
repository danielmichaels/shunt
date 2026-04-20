// file: internal/broker/subscription.go

package broker

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"log/slog"

	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/metrics"
	"github.com/danielmichaels/shunt/internal/natsutil"
	"github.com/danielmichaels/shunt/internal/rule"
	"github.com/danielmichaels/shunt/internal/trace"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Timeout and retry constants for subscription operations
const (
	// subscriptionOperationTimeout is the maximum time for JetStream operations like getting streams/consumers
	subscriptionOperationTimeout = 30 * time.Second

	// minHeartbeatInterval is the minimum heartbeat duration for consumer health checks
	minHeartbeatInterval = 1 * time.Second

	// errorBackoffDelay is the delay after encountering errors to avoid tight retry loops
	errorBackoffDelay = 100 * time.Millisecond

)

// Terminal error types - these errors indicate the message is permanently invalid
// and should not be retried.
var (
	// ErrMalformedJSON indicates the message payload contains invalid JSON that cannot be parsed.
	ErrMalformedJSON = errors.New("malformed JSON payload")

	// ErrInvalidPayload indicates the message payload is structurally invalid
	// (e.g., missing required fields, wrong types).
	ErrInvalidPayload = errors.New("invalid message payload")
)

// SubscriptionManager manages JetStream pull subscriptions for rule subjects
// using the JetStream Messages() iterator pattern for optimal performance.
//
// ARCHITECTURE: This manager uses the "Messages() Iterator" pattern recommended by NATS.
// For each rule subject, it creates:
//   - ONE Messages() iterator with internal pre-buffering and optimization
//   - A pool of "worker" goroutines that call iter.Next() in a blocking loop
//
// This leverages JetStream's built-in optimizations while maintaining a work queue pattern.
type SubscriptionManager struct {
	natsConn      *nats.Conn
	jetStream     jetstream.JetStream
	logger        *slog.Logger
	metrics       *metrics.Metrics
	processor     *rule.Processor
	consumerCfg   *config.ConsumerConfig
	publishCfg    *config.PublishConfig
	publisher     *natsutil.Publisher
	subscriptions map[string]*Subscription
	wg            sync.WaitGroup
	mu            sync.RWMutex
}

// Subscription represents a single JetStream pull consumer with Messages() iterator.
type Subscription struct {
	Subject      string
	ConsumerName string
	StreamName   string
	Consumer     jetstream.Consumer
	Workers      int                       // Number of concurrent workers
	iterator     jetstream.MessagesContext // Messages() iterator
	cancel       context.CancelFunc
	logger       *slog.Logger
	consumerCfg  *config.ConsumerConfig
}

// NewSubscriptionManager creates a new subscription manager.
func NewSubscriptionManager(
	natsConn *nats.Conn,
	js jetstream.JetStream,
	processor *rule.Processor,
	logger *slog.Logger,
	metrics *metrics.Metrics,
	consumerConfig *config.ConsumerConfig,
	publishConfig *config.PublishConfig,
) *SubscriptionManager {
	return &SubscriptionManager{
		natsConn:      natsConn,
		jetStream:     js,
		logger:        logger,
		metrics:       metrics,
		processor:     processor,
		consumerCfg:   consumerConfig,
		publishCfg:    publishConfig,
		publisher:     natsutil.NewPublisher(natsConn, js, publishConfig, metrics, logger),
		subscriptions: make(map[string]*Subscription),
	}
}

// AddSubscription creates a consumer handle for a subject.
// It accepts a context for cancellation and timeout control.
func (sm *SubscriptionManager) AddSubscription(ctx context.Context, streamName, consumerName, subject string, workers int) error {
	// Use a timeout context for JetStream operations to prevent indefinite blocking
	opCtx, cancel := context.WithTimeout(ctx, subscriptionOperationTimeout)
	defer cancel()

	// Perform network calls OUTSIDE the lock to avoid blocking other goroutines
	stream, err := sm.jetStream.Stream(opCtx, streamName)
	if err != nil {
		return fmt.Errorf("failed to get stream '%s': %w", streamName, err)
	}

	consumer, err := stream.Consumer(opCtx, consumerName)
	if err != nil {
		return fmt.Errorf("failed to get consumer '%s': %w", consumerName, err)
	}

	sub := &Subscription{
		Subject:      subject,
		ConsumerName: consumerName,
		StreamName:   streamName,
		Consumer:     consumer,
		Workers:      workers,
		logger:       sm.logger,
		consumerCfg:  sm.consumerCfg,
	}

	sm.mu.Lock()
	sm.subscriptions[subject] = sub
	sm.mu.Unlock()

	sm.logger.Debug("subscription added",
		"stream", streamName,
		"consumer", consumerName,
		"subject", subject,
		"workers", workers)

	return nil
}

// AddAndStartSubscription creates a consumer handle and immediately starts
// the iterator and worker pool. Used by RuleKVManager to add subscriptions
// at runtime without a separate Start() call.
func (sm *SubscriptionManager) AddAndStartSubscription(ctx context.Context, streamName, consumerName, subject string, workers int) error {
	sm.mu.RLock()
	_, exists := sm.subscriptions[subject]
	sm.mu.RUnlock()
	if exists {
		sm.logger.Debug("subscription already exists, skipping", "subject", subject)
		return nil
	}

	if err := sm.AddSubscription(ctx, streamName, consumerName, subject, workers); err != nil {
		return err
	}

	sm.mu.RLock()
	sub := sm.subscriptions[subject]
	sm.mu.RUnlock()

	return sm.startSubscription(ctx, sub)
}

// RemoveSubscription stops and removes a subscription by subject.
// Stops the iterator and cancels the context so workers drain naturally.
func (sm *SubscriptionManager) RemoveSubscription(subject string) {
	sm.mu.Lock()
	sub, exists := sm.subscriptions[subject]
	if !exists {
		sm.mu.Unlock()
		return
	}
	delete(sm.subscriptions, subject)
	sm.mu.Unlock()

	if sub.iterator != nil {
		sub.iterator.Stop()
	}
	if sub.cancel != nil {
		sub.cancel()
	}

	sm.logger.Debug("subscription removed", "subject", subject)
}

// Start begins consuming messages from all subscriptions using Messages() iterator.
func (sm *SubscriptionManager) Start(ctx context.Context) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.subscriptions) == 0 {
		return fmt.Errorf("no subscriptions configured")
	}

	sm.logger.Debug("starting subscription manager",
		"subscriptions", len(sm.subscriptions),
		"fetchBatchSize", sm.consumerCfg.FetchBatchSize,
		"fetchTimeout", sm.consumerCfg.FetchTimeout)

	for _, sub := range sm.subscriptions {
		if err := sm.startSubscription(ctx, sub); err != nil {
			return fmt.Errorf("failed to start subscription for '%s': %w", sub.Subject, err)
		}
	}

	sm.logger.Info("all subscriptions started successfully")
	return nil
}

// startSubscription initializes a Messages() iterator and worker pool for a single subscription.
func (sm *SubscriptionManager) startSubscription(ctx context.Context, sub *Subscription) error {
	subCtx, cancel := context.WithCancel(ctx)
	sub.cancel = cancel

	// Calculate heartbeat duration: half of fetch timeout, minimum 1 second
	// This ensures we detect stalled connections before the fetch timeout expires
	heartbeatDuration := sm.consumerCfg.FetchTimeout / 2
	if heartbeatDuration < minHeartbeatInterval {
		heartbeatDuration = minHeartbeatInterval
	}

	iter, err := sub.Consumer.Messages(
		jetstream.PullMaxMessages(sm.consumerCfg.FetchBatchSize),
		jetstream.PullExpiry(sm.consumerCfg.FetchTimeout),
		jetstream.PullHeartbeat(heartbeatDuration),
	)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to create Messages() iterator: %w", err)
	}

	sub.iterator = iter

	sm.logger.Debug("consumer iterator created",
		"subject", sub.Subject,
		"stream", sub.StreamName,
		"consumer", sub.ConsumerName,
		"pullMaxMessages", sm.consumerCfg.FetchBatchSize,
		"pullExpiry", sm.consumerCfg.FetchTimeout,
		"heartbeat", heartbeatDuration)

	for i := 0; i < sub.Workers; i++ {
		sm.wg.Add(1)
		go sm.messageWorker(subCtx, sub, i)
	}

	sm.logger.Debug("subscription started with worker pool",
		"subject", sub.Subject,
		"workers", sub.Workers)

	return nil
}

// messageWorker continuously pulls messages from the iterator and processes them.
// This replaces both the fetcher and processingWorker pattern with a simpler approach
// that leverages JetStream's internal optimizations.
func (sm *SubscriptionManager) messageWorker(ctx context.Context, sub *Subscription, workerID int) {
	defer sm.wg.Done()

	sm.logger.Debug("message worker started",
		"subject", sub.Subject,
		"workerID", workerID)

	for {
		// Check for context cancellation before blocking on Next()
		select {
		case <-ctx.Done():
			sm.logger.Debug("message worker context cancelled, shutting down",
				"subject", sub.Subject,
				"workerID", workerID)
			return
		default:
		}

		// Block until message available (key difference from channel-based approach)
		// JetStream handles all buffering, pre-fetching, and optimization internally
		msg, err := sub.iterator.Next()

		if err != nil {
			// Check for normal shutdown conditions first
			if ctx.Err() != nil {
				sm.logger.Debug("message worker detected context cancellation",
					"subject", sub.Subject,
					"workerID", workerID)
				return
			}

			// Check for iterator closed/stopped (normal shutdown)
			if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				sm.logger.Info("message iterator closed",
					"subject", sub.Subject,
					"workerID", workerID)
				return
			}

			// Log other errors but continue - JetStream will reconnect automatically
			sm.logger.Error("failed to get next message from iterator",
				"subject", sub.Subject,
				"workerID", workerID,
				"error", err,
				"errorType", fmt.Sprintf("%T", err))

			// Brief sleep on persistent errors to avoid tight loop
			time.Sleep(errorBackoffDelay)
			continue
		}

		// Process message with full error handling and panic recovery
		sm.processMessageWithRecovery(ctx, msg, sub.Subject, workerID)
	}
}

// processMessageWithRecovery wraps message processing with panic recovery and error handling.
// This ensures a single malformed message cannot crash the entire worker.
func (sm *SubscriptionManager) processMessageWithRecovery(ctx context.Context, msg jetstream.Msg, subject string, workerID int) {
	traceID := trace.FromNATSHeaders(msg.Headers())

	defer func() {
		if r := recover(); r != nil {
			sm.logger.Error("panic recovered in message worker",
				trace.LogKey, traceID,
				"panic", r,
				"subject", subject,
				"workerID", workerID,
				"stack", string(debug.Stack()))

			// Terminate poison message to prevent redelivery loop
			if termErr := msg.Term(); termErr != nil {
				sm.logger.Error("failed to terminate message after panic",
					trace.LogKey, traceID,
					"subject", subject,
					"error", termErr)
			}

			if sm.metrics != nil {
				sm.metrics.IncMessagesTotal("error")
			}
		}
	}()

	// Process the message through the rule engine
	if err := sm.processMessage(ctx, msg, subject, traceID); err != nil {
		sm.logger.Error("failed to process message",
			"subject", subject,
			"workerID", workerID,
			"error", err)

		// Differentiate between terminal and transient errors
		if isTerminalError(err) {
			// Terminal error: malformed message that will never succeed
			sm.logger.Warn("terminating malformed message to prevent redelivery loop",
				"subject", subject)
			if termErr := msg.Term(); termErr != nil {
				sm.logger.Error("failed to terminate message",
					"subject", subject,
					"error", termErr)
			}
		} else {
			// Transient error: might succeed on retry
			if nakErr := msg.Nak(); nakErr != nil {
				sm.logger.Error("failed to NAK message",
					"subject", subject,
					"error", nakErr)
			}
		}

		if sm.metrics != nil {
			sm.metrics.IncMessagesTotal("error")
		}
	} else {
		// Message processed successfully, acknowledge it
		if ackErr := msg.Ack(); ackErr != nil {
			sm.logger.Error("failed to ACK message",
				"subject", subject,
				"error", ackErr)
		}
		if sm.metrics != nil {
			sm.metrics.IncMessagesTotal("processed")
		}
	}
}

// processMessage handles a single message through the rule engine.
// triggerSubject is the trigger pattern this consumer is bound to (e.g., "sensors.tank.>"),
// used for O(1) KV rule lookup via ProcessForSubscription.
func (sm *SubscriptionManager) processMessage(ctx context.Context, msg jetstream.Msg, triggerSubject string, traceID string) error {
	start := time.Now()
	log := sm.logger.With(trace.LogKey, traceID)

	if sm.metrics != nil {
		sm.metrics.IncMessagesTotal("received")
	}

	log.Debug("processing message", "subject", msg.Subject(), "size", len(msg.Data()))

	// Extract headers
	headers := make(map[string]string)
	if msg.Headers() != nil {
		for key, values := range msg.Headers() {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
	}

	// Process through rule engine (uses ProcessNATS internally via backward compatibility layer)
	actions, err := sm.processor.ProcessForSubscription(triggerSubject, msg.Subject(), msg.Data(), headers)
	if err != nil {
		return fmt.Errorf("rule processing failed: %w", err)
	}

	// Publish all matched actions
	for _, action := range actions {
		action.TraceID = traceID

		if action.NATS != nil {
			if err := sm.publishActionWithRetry(ctx, action.NATS, action.RuleName, traceID); err != nil {
				log.Error("failed to publish NATS action after retries",
					"actionSubject", action.NATS.Subject,
					"rule_name", action.RuleName,
					"error", err)
				if sm.metrics != nil {
					sm.metrics.IncActionsTotal("error", action.RuleName)
				}
				return fmt.Errorf("failed to publish NATS action: %w", err)
			}
			if sm.metrics != nil {
				sm.metrics.IncActionsTotal("success", action.RuleName)
			}
		} else if action.HTTP != nil {
			log.Warn("HTTP action detected in shunt - HTTP actions not supported in this application",
				"actionURL", action.HTTP.URL,
				"hint", "Use http-gateway for HTTP actions")
		} else {
			log.Error("action has neither NATS nor HTTP configuration - this should never happen",
				"subject", msg.Subject())
		}
	}

	duration := time.Since(start)
	log.Debug("message processed", "subject", msg.Subject(), "duration", duration, "actionsPublished", len(actions))
	return nil
}

func (sm *SubscriptionManager) publishActionWithRetry(ctx context.Context, action *rule.NATSAction, ruleName string, traceID string) error {
	return sm.publisher.PublishAction(ctx, action, ruleName, traceID)
}

// Stop gracefully shuts down all subscriptions.
func (sm *SubscriptionManager) Stop() error {
	sm.mu.Lock()
	sm.logger.Info("stopping all subscriptions", "count", len(sm.subscriptions))

	for _, sub := range sm.subscriptions {
		if sub.iterator != nil {
			sm.logger.Debug("stopping consumer iterator", "subject", sub.Subject)
			sub.iterator.Stop()
			sm.logger.Debug("consumer iterator stopped", "subject", sub.Subject)
		}
	}

	for _, sub := range sm.subscriptions {
		if sub.cancel != nil {
			sub.cancel()
		}
	}
	sm.mu.Unlock()

	sm.logger.Debug("waiting for all workers to finish")
	sm.wg.Wait()

	sm.logger.Info("all subscriptions stopped successfully")
	return nil
}

// GetSubscriptionCount returns the number of active subscriptions.
func (sm *SubscriptionManager) GetSubscriptionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.subscriptions)
}

// isTerminalError checks if an error is permanent and should not be retried.
// This is used to decide whether to Terminate or Nak a message.
//
// Terminal errors are those where retrying the message would never succeed,
// such as malformed payloads.
func isTerminalError(err error) bool {
	return errors.Is(err, ErrMalformedJSON) ||
		errors.Is(err, ErrInvalidPayload) ||
		errors.Is(err, rule.ErrMalformedPayload)
}
