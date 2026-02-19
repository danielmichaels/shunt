package cmd

import (
	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/app"
	"github.com/danielmichaels/shunt/internal/broker"
	"github.com/danielmichaels/shunt/internal/lifecycle"
	"github.com/danielmichaels/shunt/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the shunt message routing server",
	Long: `Start the shunt server which connects to NATS JetStream, loads rules from
a KV bucket, and routes messages based on configured rules.

Optional subsystems (gateway, auth manager) can be enabled via configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")

		v := viper.New()
		v.BindPFlag("nats.urls", cmd.Flags().Lookup("nats-urls"))
		v.BindPFlag("metrics.enabled", cmd.Flags().Lookup("metrics-enabled"))
		v.BindPFlag("metrics.address", cmd.Flags().Lookup("metrics-addr"))
		v.BindPFlag("metrics.path", cmd.Flags().Lookup("metrics-path"))
		v.BindPFlag("logging.level", cmd.Flags().Lookup("log-level"))

		cfg, err := config.Load(configPath, v)
		if err != nil {
			return err
		}

		appLogger, err := logger.NewLogger(&cfg.Logging)
		if err != nil {
			return err
		}
		defer appLogger.Sync()

		createApp := func() (lifecycle.Application, error) {
			baseApp, err := app.NewAppBuilder(cfg).
				WithLogger().
				WithMetrics().
				WithNATSBroker().
				WithKVRuleProcessor().
				Build()
			if err != nil {
				return nil, err
			}

			routerApp := app.NewKVRouterApp(baseApp, cfg)

			kvManager := broker.NewRuleKVManager(
				cfg.Rules.KVBucket,
				cfg.KV.AutoProvision,
				baseApp.Processor,
				baseApp.Broker,
				baseApp.RulesLoader,
				baseApp.Logger,
			)
			routerApp.SetRuleKVManager(kvManager)

			return routerApp, nil
		}

		return lifecycle.Run(createApp, appLogger)
	},
}

func init() {
	serveCmd.Flags().String("config", "config/shunt.yaml", "path to config file (YAML or JSON, optional — env vars work without it)")
	serveCmd.Flags().StringSlice("nats-urls", nil, "NATS server URLs to override config (repeatable or comma-separated)")
	serveCmd.Flags().Bool("metrics-enabled", true, "override enabling of metrics server")
	serveCmd.Flags().String("metrics-addr", "", "override metrics server address")
	serveCmd.Flags().String("metrics-path", "", "override metrics endpoint path")
	serveCmd.Flags().String("log-level", "", "override log level (debug, info, warn, error)")
}
