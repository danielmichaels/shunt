// file: internal/app/router.go

package app

import (
	"context"
	"fmt"
	"time"

	"log/slog"

	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/authmgr"
	"github.com/danielmichaels/shunt/internal/authmgr/providers"
	"github.com/danielmichaels/shunt/internal/broker"
	"github.com/danielmichaels/shunt/internal/gateway"
	"github.com/danielmichaels/shunt/internal/lifecycle"
	"github.com/danielmichaels/shunt/internal/metrics"
	"github.com/danielmichaels/shunt/internal/rule"
)

// Timeout constants for RouterApp operations
const (
	// metricsShutdownTimeout is the maximum time to wait for the metrics server to shutdown
	metricsShutdownTimeout = 5 * time.Second

	// outboundSetupTimeout is the maximum time to wait for outbound subscriptions to be configured
	outboundSetupTimeout = 60 * time.Second
)

// Verify RouterApp implements lifecycle.Application interface at compile time
var _ lifecycle.Application = (*RouterApp)(nil)

// RouterApp represents the shunt application with all its components including KV support
type RouterApp struct {
	config           *config.Config
	logger           *slog.Logger
	metrics          *metrics.Metrics
	processor        *rule.Processor
	broker           *broker.NATSBroker
	base             *BaseApp
	metricsCollector *metrics.MetricsCollector
	ruleKVManager    *broker.RuleKVManager

	inboundServer  *gateway.InboundServer
	outboundClient *gateway.OutboundClient
	authManager    *authmgr.Manager
	authNATSClient *authmgr.NATSClient
}

// NewKVRouterApp creates a shunt application that uses KV Watch for rule management.
// KV Watch manages subscriptions at runtime.
func NewKVRouterApp(base *BaseApp, cfg *config.Config) *RouterApp {
	app := &RouterApp{
		config:           cfg,
		logger:           base.Logger,
		metrics:          base.Metrics,
		processor:        base.Processor,
		broker:           base.Broker,
		base:             base,
		metricsCollector: base.Collector,
	}

	app.broker.InitializeSubscriptionManager(app.processor)

	return app
}

// SetRuleKVManager assigns the KV manager that watches for rule changes at runtime.
func (app *RouterApp) SetRuleKVManager(mgr *broker.RuleKVManager) {
	app.ruleKVManager = mgr
}

// Run starts the application and waits for shutdown signal.
func (app *RouterApp) Run(ctx context.Context) error {
	app.logger.Info("starting shunt in KV mode",
		"kvBucket", app.config.Rules.KVBucket,
		"natsUrls", app.config.NATS.URLs)

	if err := app.ruleKVManager.Watch(ctx); err != nil {
		return fmt.Errorf("failed to start KV watcher: %w", err)
	}

	readyCtx, readyCancel := context.WithTimeout(ctx, 30*time.Second)
	defer readyCancel()

	if err := app.ruleKVManager.WaitReady(readyCtx); err != nil {
		return fmt.Errorf("KV initial sync timed out: %w", err)
	}

	app.logger.Info("KV initial sync complete, processing messages")

	if err := app.startGateway(ctx); err != nil {
		return fmt.Errorf("failed to start gateway subsystem: %w", err)
	}

	if err := app.startAuthManager(); err != nil {
		return fmt.Errorf("failed to start auth-manager subsystem: %w", err)
	}

	<-ctx.Done()
	app.logger.Info("shutting down gracefully...")

	app.stopGateway()

	return nil
}

func (app *RouterApp) startGateway(ctx context.Context) error {
	if !app.config.Gateway.Enabled {
		return nil
	}

	if app.config.HTTP.Server.Address == "" {
		return fmt.Errorf("http.server.address is required when gateway is enabled")
	}

	app.logger.Info("starting gateway subsystem")

	serverConfig := &gateway.ServerConfig{
		Address:             app.config.HTTP.Server.Address,
		ReadTimeout:         app.config.HTTP.Server.ReadTimeout,
		WriteTimeout:        app.config.HTTP.Server.WriteTimeout,
		IdleTimeout:         app.config.HTTP.Server.IdleTimeout,
		MaxHeaderBytes:      app.config.HTTP.Server.MaxHeaderBytes,
		ShutdownGracePeriod: app.config.HTTP.Server.ShutdownGracePeriod,
		InboundWorkerCount:  app.config.HTTP.Server.InboundWorkerCount,
		InboundQueueSize:    app.config.HTTP.Server.InboundQueueSize,
	}
	publishConfig := &gateway.PublishConfig{
		Mode:           app.config.NATS.Publish.Mode,
		AckTimeout:     app.config.NATS.Publish.AckTimeout,
		MaxRetries:     app.config.NATS.Publish.MaxRetries,
		RetryBaseDelay: app.config.NATS.Publish.RetryBaseDelay,
	}

	app.inboundServer = gateway.NewInboundServer(
		app.logger,
		app.metrics,
		app.processor,
		app.broker.GetJetStream(),
		app.broker.GetNATSConn(),
		serverConfig,
		publishConfig,
	)

	consumerConfig := &gateway.ConsumerConfig{
		WorkerCount:    app.config.NATS.Consumers.WorkerCount,
		FetchBatchSize: app.config.NATS.Consumers.FetchBatchSize,
		FetchTimeout:   app.config.NATS.Consumers.FetchTimeout,
		MaxAckPending:  app.config.NATS.Consumers.MaxAckPending,
		AckWaitTimeout: app.config.NATS.Consumers.AckWaitTimeout,
		MaxDeliver:     app.config.NATS.Consumers.MaxDeliver,
	}

	app.outboundClient = gateway.NewOutboundClient(
		app.logger,
		app.metrics,
		app.processor,
		app.broker.GetJetStream(),
		consumerConfig,
		&app.config.HTTP.Client,
	)

	allRules := app.processor.GetAllRules()
	outboundSubjects := make(map[string]bool)

	setupCtx, cancel := context.WithTimeout(context.Background(), outboundSetupTimeout)
	defer cancel()

	for _, r := range allRules {
		if r.Trigger.NATS != nil && r.Action.HTTP != nil {
			subject := r.Trigger.NATS.Subject
			if outboundSubjects[subject] {
				continue
			}
			outboundSubjects[subject] = true

			if err := app.broker.CreateConsumerForSubject(subject); err != nil {
				return fmt.Errorf("failed to create consumer for subject '%s': %w", subject, err)
			}

			streamName, err := app.broker.FindStreamForSubject(subject)
			if err != nil {
				return fmt.Errorf("failed to find stream for subject '%s': %w", subject, err)
			}
			consumerName := app.broker.GetConsumerName(subject)

			workers := app.config.NATS.Consumers.WorkerCount
			if err := app.outboundClient.AddSubscription(setupCtx, streamName, consumerName, subject, workers); err != nil {
				return fmt.Errorf("failed to add outbound subscription for '%s': %w", subject, err)
			}
		}
	}

	if err := app.inboundServer.Start(ctx); err != nil {
		return fmt.Errorf("failed to start inbound server: %w", err)
	}

	if err := app.outboundClient.Start(ctx); err != nil {
		return fmt.Errorf("failed to start outbound client: %w", err)
	}

	app.logger.Info("gateway subsystem started",
		"httpAddress", app.config.HTTP.Server.Address,
		"outboundSubscriptions", len(outboundSubjects))

	return nil
}

func (app *RouterApp) stopGateway() {
	if app.inboundServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), app.config.HTTP.Server.ShutdownGracePeriod)
		defer cancel()
		if err := app.inboundServer.Stop(shutdownCtx); err != nil {
			app.logger.Error("failed to stop inbound server", "error", err)
		}
	}
	if app.outboundClient != nil {
		if err := app.outboundClient.Stop(); err != nil {
			app.logger.Error("failed to stop outbound client", "error", err)
		}
	}
}

func (app *RouterApp) startAuthManager() error {
	if !app.config.AuthManager.Enabled {
		return nil
	}

	app.logger.Info("starting auth-manager subsystem")

	natsClient, err := authmgr.NewNATSClientFromConn(
		app.broker.GetNATSConn(),
		&authmgr.StorageConfig{
			Bucket:        app.config.AuthManager.Storage.Bucket,
			KeyPrefix:     app.config.AuthManager.Storage.KeyPrefix,
			AutoProvision: app.config.KV.AutoProvision,
		},
		app.logger,
	)
	if err != nil {
		return fmt.Errorf("failed to create auth-manager NATS client: %w", err)
	}
	app.authNATSClient = natsClient

	providerList, err := createProviders(app.config.AuthManager.Providers, app.logger)
	if err != nil {
		return fmt.Errorf("failed to create auth providers: %w", err)
	}

	app.authManager = authmgr.NewManager(natsClient, providerList, app.logger, nil)

	if err := app.authManager.Start(); err != nil {
		return fmt.Errorf("failed to start auth manager: %w", err)
	}

	app.logger.Info("auth-manager subsystem started",
		"providers", len(providerList),
		"kvBucket", app.config.AuthManager.Storage.Bucket)

	return nil
}

func createProviders(configs []config.AuthManagerProvider, log *slog.Logger) ([]providers.Provider, error) {
	var providerList []providers.Provider

	for _, cfg := range configs {
		kvKey := cfg.KVKey
		if kvKey == "" {
			kvKey = cfg.ID
		}

		var p providers.Provider

		switch cfg.Type {
		case "oauth2":
			refreshBefore, _ := time.ParseDuration(cfg.RefreshBefore)
			p = providers.NewOAuth2Provider(
				kvKey,
				cfg.TokenURL,
				cfg.ClientID,
				cfg.ClientSecret,
				cfg.Scopes,
				refreshBefore,
			)
		case "custom-http":
			refreshEvery, _ := time.ParseDuration(cfg.RefreshEvery)
			p = providers.NewCustomHTTPProvider(
				kvKey,
				cfg.AuthURL,
				cfg.Method,
				cfg.Headers,
				cfg.Body,
				cfg.TokenPath,
				refreshEvery,
			)
		default:
			return nil, fmt.Errorf("unknown provider type: %s", cfg.Type)
		}

		providerList = append(providerList, p)
		log.Info("auth provider configured", "id", cfg.ID, "type", cfg.Type, "kvKey", kvKey)
	}

	return providerList, nil
}

// Close gracefully shuts down all application components
func (app *RouterApp) Close() error {
	app.logger.Info("closing application components")

	var errs []error

	if app.authManager != nil {
		if err := app.authManager.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop auth manager: %w", err))
		}
	}

	if app.authNATSClient != nil {
		if err := app.authNATSClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close auth NATS client: %w", err))
		}
	}

	if app.metricsCollector != nil {
		app.metricsCollector.Stop()
	}

	if app.base != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), metricsShutdownTimeout)
		defer cancel()
		if err := app.base.ShutdownMetricsServer(shutdownCtx); err != nil {
			errs = append(errs, err)
		}
	}

	if app.ruleKVManager != nil {
		app.ruleKVManager.Stop()
	}

	if app.broker != nil {
		if err := app.broker.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close NATS broker: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}
