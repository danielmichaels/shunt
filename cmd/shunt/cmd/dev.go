package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
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
	"demo.>",
}

const (
	// devDemoReceiverURLVar is used in local-capture.yaml rule file
	devDemoReceiverURLVar = "SHUNT_DEV_DEMO_RECEIVER_URL"
	devDemoReceiverPath   = "/shunt-demo"
)

// devDefaultRules are seeded on startup when --all-rules is not set.
var devDefaultRules = []string{
	"router/basic.yaml",
	"router/wildcard-examples.yaml",
	"http/webhooks.yaml",
}

// DevCmd starts an embedded NATS server alongside shunt for local development.
// No external NATS installation or Docker required.
type DevCmd struct {
	NATSPort         int    `help:"Embedded NATS server port" default:"14222"`
	HTTPAddr         string `help:"HTTP gateway listen address" default:":7080"`
	RulesDir         string `help:"Directory of rule YAML files to seed into KV on startup" default:"./rules"`
	LogLevel         string `help:"Log level" default:"debug" enum:"debug,info,warn,error"`
	AllRules         bool   `help:"Seed all rule files from RulesDir instead of only the defaults" default:"false"`
	DemoReceiver     bool   `help:"Start built-in local HTTP receiver for the NATS -> HTTP demo" default:"true" env:"SHUNT_DEV_DEMO_RECEIVER"`
	DemoReceiverAddr string `help:"Built-in demo receiver listen address" default:"127.0.0.1:18080" env:"SHUNT_DEV_DEMO_RECEIVER_ADDR"`
}

func (d *DevCmd) Run(globals *Globals) error {
	natsURL := fmt.Sprintf("nats://localhost:%d", d.NATSPort)
	demoReceiverURL := devDemoReceiverURL(d.DemoReceiverAddr)
	defaultRules := append([]string(nil), devDefaultRules...)
	if d.DemoReceiver {
		defaultRules = append(defaultRules, "http/local-capture.yaml", "http/demo-chain.yaml")
	}

	if err := os.Setenv(devDemoReceiverURLVar, demoReceiverURL); err != nil {
		return fmt.Errorf("set %s: %w", devDemoReceiverURLVar, err)
	}

	fmt.Fprintf(os.Stderr, "\n  shunt dev\n\n")
	fmt.Fprintf(os.Stderr, "  embedded NATS  %s\n", natsURL)
	fmt.Fprintf(os.Stderr, "  HTTP gateway   http://localhost%s\n", d.HTTPAddr)
	if d.DemoReceiver {
		fmt.Fprintf(os.Stderr, "  demo receiver  %s\n", demoReceiverURL)
	} else {
		fmt.Fprintf(os.Stderr, "  demo receiver  disabled\n")
	}
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

	if err := seedNATS(natsURL, d.RulesDir, d.AllRules, defaultRules, bootLog); err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	cfg := buildDevConfig(natsURL, d.HTTPAddr, d.LogLevel)

	appLogger, err := logger.NewLogger(&cfg.Logging)
	if err != nil {
		return err
	}

	if d.DemoReceiver {
		demoReceiver, err := startDevDemoReceiver(d.DemoReceiverAddr, appLogger)
		if err != nil {
			return fmt.Errorf("start demo receiver: %w (disable with --demo-receiver=false)", err)
		}
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := demoReceiver.Shutdown(shutdownCtx); err != nil {
				appLogger.Warn("failed to stop demo receiver", "error", err)
			}
		}()
	}

	bi := buildinfo.Get(globals.Version)
	appLogger.Info("starting shunt dev",
		"version", bi.Version,
		"nats", natsURL,
		"gateway", d.HTTPAddr,
		"demoReceiverEnabled", d.DemoReceiver,
		"demoReceiverURL", demoReceiverURL)

	printCheatsheet(d.HTTPAddr, d.NATSPort, d.DemoReceiver, demoReceiverURL)

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
func seedNATS(natsURL, rulesDir string, allRules bool, defaultRules []string, log *slog.Logger) error {
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
		if err := loadDefaultRules(ctx, kv, rulesDir, defaultRules, log); err != nil {
			log.Warn("default rule seeding incomplete", "error", err)
		}
	}

	return nil
}

func loadDefaultRules(ctx context.Context, kv jetstream.KeyValue, rulesDir string, defaultRules []string, log *slog.Logger) error {
	count := 0
	for _, rel := range defaultRules {
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

func devDemoReceiverURL(addr string) string {
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	return "http://" + addr + devDemoReceiverPath
}

func startDevDemoReceiver(addr string, log *slog.Logger) (*http.Server, error) {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc(devDemoReceiverPath, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024))
		if err != nil {
			log.Error("demo receiver failed to read body", "error", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		headers := make(map[string]string)
		for key, values := range r.Header {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}

		log.Info("demo receiver captured outbound HTTP request",
			"method", r.Method,
			"path", r.URL.Path,
			"remoteAddr", r.RemoteAddr,
			"headers", headers,
			"body", string(body))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	server := &http.Server{
		Handler:           mux,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("demo receiver stopped unexpectedly", "error", err)
		}
	}()

	log.Info("demo receiver started", "url", devDemoReceiverURL(addr))
	return server, nil
}

func printCheatsheet(httpAddr string, natsPort int, demoReceiverEnabled bool, demoReceiverURL string) {
	host := "localhost"
	if strings.HasPrefix(httpAddr, ":") {
		httpAddr = host + httpAddr
	}

	const rule = "  ──────────────────────────────────────────────────────────────────"
	w := os.Stderr
	section := func(title string) {
		fmt.Fprintf(w, "\n%s\n   %s\n%s\n\n", rule, title, rule)
	}

	fmt.Fprintf(w, "\n  ══════════════════════════════════════════════════════════════════\n")
	fmt.Fprintf(w, "   shunt dev cheatsheet\n")
	fmt.Fprintf(w, "  ══════════════════════════════════════════════════════════════════\n")

	section("1. Publish to NATS (trigger router rules)")
	fmt.Fprintf(w, "     nats -s nats://localhost:%d pub sensors.data \\\n", natsPort)
	fmt.Fprintf(w, "       '{\"sensor\":{\"id\":\"t1\",\"reading\":35,\"location\":\"bedroom\"}}'\n\n")
	fmt.Fprintf(w, "     nats -s nats://localhost:%d pub building.floor1.hvac.status \\\n", natsPort)
	fmt.Fprintf(w, "       '{\"status\":\"alert\",\"message\":\"overheat\",\"priority\":1}'\n\n")
	fmt.Fprintf(w, "     nats -s nats://localhost:%d pub events.deploy \\\n", natsPort)
	fmt.Fprintf(w, "       '{\"severity\":\"critical\",\"description\":\"deploy failed\",\"region\":\"us-east\"}'\n")

	section("2. POST HTTP webhooks (trigger http rules)")
	fmt.Fprintf(w, "     curl -s -X POST http://%s/webhooks/github/pr \\\n", httpAddr)
	fmt.Fprintf(w, "       -H 'Content-Type: application/json' \\\n")
	fmt.Fprintf(w, "       -H 'X-GitHub-Event: pull_request' \\\n")
	fmt.Fprintf(w, "       -d '{\"action\":\"opened\",\"number\":42,\"user\":{\"login\":\"alice\"},\n")
	fmt.Fprintf(w, "            \"pull_request\":{\"title\":\"feat\",\"html_url\":\"http://example.com\"},\n")
	fmt.Fprintf(w, "            \"repository\":{\"name\":\"shunt\"}}'\n\n")
	fmt.Fprintf(w, "     curl -s -X POST http://%s/webhooks/generic \\\n", httpAddr)
	fmt.Fprintf(w, "       -H 'Content-Type: application/json' \\\n")
	fmt.Fprintf(w, "       -d '{\"data\":{\"hello\":\"world\"}}'\n")

	if demoReceiverEnabled {
		section("3. Capture outbound HTTP (NATS → HTTP)")
		fmt.Fprintf(w, "     receiver listening at %s\n\n", demoReceiverURL)
		fmt.Fprintf(w, "     nats -s nats://localhost:%d pub notifications.demo \\\n", natsPort)
		fmt.Fprintf(w, "       '{\"message\":\"hello from shunt\",\"severity\":\"info\"}'\n\n")
		fmt.Fprintf(w, "     shunt POSTs to the receiver; request prints in this terminal.\n")

		section("4. Two-hop chain (HTTP → NATS → HTTP)")
		fmt.Fprintf(w, "     curl -s -X POST http://%s/webhooks/demo \\\n", httpAddr)
		fmt.Fprintf(w, "       -H 'Content-Type: application/json' \\\n")
		fmt.Fprintf(w, "       -d '{\"data\":{\"hello\":\"chain\"}}'\n\n")
		fmt.Fprintf(w, "     rule 1: POST /webhooks/demo → publish NATS demo.trigger\n")
		fmt.Fprintf(w, "     rule 2: NATS demo.trigger    → POST to receiver\n")
	}

	section("5. Subscribe to routed subjects")
	fmt.Fprintf(w, "     nats -s nats://localhost:%d sub \\\n", natsPort)
	fmt.Fprintf(w, "       'alerts.>'    'facilities.>' 'monitoring.>' 'escalation.>' \\\n")
	fmt.Fprintf(w, "       'ops.>'       'critical.>'   'github.>'     'webhooks.>'   \\\n")
	fmt.Fprintf(w, "       'payments.>'  'tenants.>'    'demo.>'\n\n")
	fmt.Fprintf(w, "     avoid 'nats sub \">\"' — picks up JetStream $JS.> chatter too.\n")

	fmt.Fprintf(w, "\n  ══════════════════════════════════════════════════════════════════\n\n")
}
