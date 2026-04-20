package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/danielmichaels/shunt/config"
	"github.com/danielmichaels/shunt/internal/app"
	"github.com/danielmichaels/shunt/internal/broker"
	"github.com/danielmichaels/shunt/internal/buildinfo"
	"github.com/danielmichaels/shunt/internal/lifecycle"
	"github.com/danielmichaels/shunt/internal/logger"
)

// devStreamSubjects covers all trigger and action subjects from the bundled default rules.
// One stream named "DEV" is created with these wildcard patterns.
var devStreamSubjects = []string{
	"sensors.>",
	"building.>",
	"devices.>",
	"company.>",
	"monitoring.>",
	"events.>",
	"alerts.>",
	"facilities.>",
	"escalation.>",
	"ops.>",
	"critical.>",
	"github.>",
	"webhooks.>",
	"payments.>",
	"tenants.>",
	"notifications.>",
	"orders.>",
	"reports.>",
}

// devDefaultRules are seeded on startup when --all-rules is not set.
var devDefaultRules = []string{
	"router/basic.yaml",
	"router/wildcard-examples.yaml",
	"http/webhooks.yaml",
}

// DevCmd starts an embedded NATS server alongside shunt for local development.
// No external NATS installation or Docker required.
type DevCmd struct {
	NATSPort int    `help:"Embedded NATS server port" default:"14222"`
	HTTPAddr string `help:"HTTP gateway listen address" default:":7080"`
	RulesDir string `help:"Directory of rule YAML files to seed into KV on startup" default:"./rules"`
	LogLevel string `help:"Log level" default:"debug" enum:"debug,info,warn,error"`
	AllRules bool   `help:"Seed all rule files from RulesDir instead of only the defaults" default:"false"`
}

func (d *DevCmd) Run(globals *Globals) error {
	natsURL := fmt.Sprintf("nats://localhost:%d", d.NATSPort)

	fmt.Fprintf(os.Stderr, "\n  shunt dev\n\n")
	fmt.Fprintf(os.Stderr, "  embedded NATS  %s\n", natsURL)
	fmt.Fprintf(os.Stderr, "  HTTP gateway   http://localhost%s\n", d.HTTPAddr)
	fmt.Fprintf(os.Stderr, "  rules dir      %s\n\n", d.RulesDir)

	bootLog := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	ns, err := startEmbeddedNATS(d.NATSPort, bootLog)
	if err != nil {
		return fmt.Errorf("embedded NATS: %w", err)
	}
	defer func() {
		bootLog.Info("shutting down embedded NATS")
		ns.Shutdown()
		ns.WaitForShutdown()
	}()

	if err := seedNATS(natsURL, d.RulesDir, d.AllRules, bootLog); err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	cfg := buildDevConfig(natsURL, d.HTTPAddr, d.LogLevel)

	appLogger, err := logger.NewLogger(&cfg.Logging)
	if err != nil {
		return err
	}

	bi := buildinfo.Get(globals.Version)
	appLogger.Info("starting shunt dev",
		"version", bi.Version,
		"nats", natsURL,
		"gateway", d.HTTPAddr)

	printCheatsheet(d.HTTPAddr, d.NATSPort)

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
}

func startEmbeddedNATS(port int, log *slog.Logger) (*natsserver.Server, error) {
	storeDir, err := os.MkdirTemp("", "shunt-dev-nats-*")
	if err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}

	opts := &natsserver.Options{
		JetStream: true,
		Port:      port,
		StoreDir:  storeDir,
		NoSigs:    true,
		NoLog:     true,
	}

	ns, err := natsserver.NewServer(opts)
	if err != nil {
		os.RemoveAll(storeDir)
		return nil, fmt.Errorf("new server: %w", err)
	}

	go ns.Start()

	if !ns.ReadyForConnections(10 * time.Second) {
		ns.Shutdown()
		os.RemoveAll(storeDir)
		return nil, fmt.Errorf("NATS server did not become ready within 10s")
	}

	log.Info("embedded NATS started", "port", port)

	go func() {
		ns.WaitForShutdown()
		os.RemoveAll(storeDir)
	}()

	return ns, nil
}

// seedNATS creates the DEV JetStream stream and loads rules from dir into the rules KV bucket.
func seedNATS(natsURL, rulesDir string, allRules bool, log *slog.Logger) error {
	nc, err := nats.Connect(natsURL,
		nats.MaxReconnects(5),
		nats.ReconnectWait(200*time.Millisecond),
	)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "DEV",
		Subjects: devStreamSubjects,
		Storage:  jetstream.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("create DEV stream: %w", err)
	}
	log.Info("DEV stream ready", "subjects", len(devStreamSubjects))

	kv, err := js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket: "rules",
	})
	if err != nil {
		return fmt.Errorf("create rules kv: %w", err)
	}

	if allRules {
		if err := loadRulesFromDir(ctx, kv, rulesDir, log); err != nil {
			log.Warn("rule seeding incomplete", "dir", rulesDir, "error", err)
		}
	} else {
		if err := loadDefaultRules(ctx, kv, rulesDir, log); err != nil {
			log.Warn("default rule seeding incomplete", "error", err)
		}
	}

	return nil
}

func loadDefaultRules(ctx context.Context, kv jetstream.KeyValue, rulesDir string, log *slog.Logger) error {
	count := 0
	for _, rel := range devDefaultRules {
		path := filepath.Join(rulesDir, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				log.Info("default rule not found, skipping", "path", path)
				continue
			}
			log.Warn("skipping unreadable rule file", "path", path, "error", err)
			continue
		}

		key := ruleKey(rulesDir, path)
		if _, err := kv.Put(ctx, key, data); err != nil {
			log.Warn("failed to seed rule", "key", key, "error", err)
			continue
		}

		log.Debug("seeded rule", "key", key, "path", path)
		count++
	}

	log.Info("default rules seeded", "count", count)
	return nil
}

func loadRulesFromDir(ctx context.Context, kv jetstream.KeyValue, dir string, log *slog.Logger) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("rules dir not found, skipping seed", "dir", dir)
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}

	count := 0
	err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			log.Warn("skipping unreadable rule file", "path", path, "error", err)
			return nil
		}

		key := ruleKey(dir, path)
		if _, err := kv.Put(ctx, key, data); err != nil {
			log.Warn("failed to seed rule", "key", key, "error", err)
			return nil
		}

		log.Debug("seeded rule", "key", key, "path", path)
		count++
		return nil
	})
	if err != nil {
		return err
	}

	log.Info("rules seeded", "count", count, "dir", dir)
	return nil
}

// ruleKey derives a dot-separated KV key from the file path relative to the rules dir.
// e.g. rules/router/basic.yaml -> router.basic
func ruleKey(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		rel = filepath.Base(path)
	}
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))
	rel = strings.ReplaceAll(rel, string(filepath.Separator), ".")
	rel = strings.ReplaceAll(rel, " ", "-")
	return strings.ToLower(rel)
}

func buildDevConfig(natsURL, httpAddr, logLevel string) *config.Config {
	cfg := config.NewDefaults()

	cfg.NATS.URLs = []string{natsURL}
	cfg.NATS.Connection.MaxReconnects = -1

	cfg.KV.Enabled = true
	cfg.KV.AutoProvision = true
	cfg.KV.Buckets = []string{"rules"}

	cfg.Rules.KVBucket = "rules"

	cfg.Gateway.Enabled = true
	cfg.HTTP.Server.Address = httpAddr
	cfg.HTTP.Server.InboundWorkerCount = 10
	cfg.HTTP.Server.InboundQueueSize = 1000

	cfg.Logging.Level = logLevel
	cfg.Logging.Encoding = "console"

	cfg.Metrics.Enabled = true
	cfg.Metrics.Address = ":2112"

	cfg.Security.Verification.Enabled = false

	return cfg
}

func printCheatsheet(httpAddr string, natsPort int) {
	host := "localhost"
	if strings.HasPrefix(httpAddr, ":") {
		httpAddr = host + httpAddr
	}

	fmt.Fprintf(os.Stderr, "\n  ── dev cheatsheet ──────────────────────────────────────────\n\n")
	fmt.Fprintf(os.Stderr, "  NATS publish (trigger router rules):\n")
	fmt.Fprintf(os.Stderr, "    nats -s nats://localhost:%d pub sensors.data '{\"sensor\":{\"id\":\"t1\",\"reading\":35,\"location\":\"bedroom\"}}'\n", natsPort)
	fmt.Fprintf(os.Stderr, "    nats -s nats://localhost:%d pub building.floor1.hvac.status '{\"status\":\"alert\",\"message\":\"overheat\",\"priority\":1}'\n", natsPort)
	fmt.Fprintf(os.Stderr, "    nats -s nats://localhost:%d pub events.deploy '{\"severity\":\"critical\",\"description\":\"deploy failed\",\"affected_systems\":[\"api\"],\"region\":\"us-east\"}'\n", natsPort)
	fmt.Fprintf(os.Stderr, "\n  HTTP webhook (trigger http rules):\n")
	fmt.Fprintf(os.Stderr, "    curl -s -X POST http://%s/webhooks/github/pr \\\n", httpAddr)
	fmt.Fprintf(os.Stderr, "      -H 'Content-Type: application/json' \\\n")
	fmt.Fprintf(os.Stderr, "      -H 'X-GitHub-Event: pull_request' \\\n")
	fmt.Fprintf(os.Stderr, "      -d '{\"action\":\"opened\",\"number\":42,\"user\":{\"login\":\"alice\"},\"pull_request\":{\"title\":\"feat\",\"html_url\":\"http://example.com\"},\"repository\":{\"name\":\"shunt\"}}'\n")
	fmt.Fprintf(os.Stderr, "    curl -s -X POST http://%s/webhooks/generic -H 'Content-Type: application/json' -d '{\"data\":{\"hello\":\"world\"}}'\n", httpAddr)
	fmt.Fprintf(os.Stderr, "\n  Subscribe to routed messages:\n")
	fmt.Fprintf(os.Stderr, "    nats -s nats://localhost:%d sub 'alerts.>'\n", natsPort)
	fmt.Fprintf(os.Stderr, "    nats -s nats://localhost:%d sub '>'\n", natsPort)
	fmt.Fprintf(os.Stderr, "\n  ────────────────────────────────────────────────────────────\n\n")
}
