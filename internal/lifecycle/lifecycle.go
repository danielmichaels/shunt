// file: internal/lifecycle/lifecycle.go

package lifecycle

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"
)

// Run runs an application with SIGTERM/SIGINT handling only (no SIGHUP reload).
// Used by shunt in KV mode where rule changes are picked up live via KV Watch.
func Run(
	createApp func() (Application, error),
	log *slog.Logger,
) error {
	shutdownSig := make(chan os.Signal, 1)
	signal.Notify(shutdownSig, os.Interrupt, syscall.SIGTERM)

	application, err := createApp()
	if err != nil {
		signal.Stop(shutdownSig)
		close(shutdownSig)
		return fmt.Errorf("failed to create application: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- application.Run(ctx)
	}()

	var runErr error
	select {
	case sig := <-shutdownSig:
		log.Info("shutdown signal received", "signal", sig)
	case runErr = <-errCh:
		if runErr != nil {
			log.Error("application stopped with error", "error", runErr)
		}
	}

	cancel()
	signal.Stop(shutdownSig)
	close(shutdownSig)

	log.Info("closing application")
	closeStart := time.Now()
	if closeErr := application.Close(); closeErr != nil {
		log.Error("error during application close",
			"error", closeErr, "duration", time.Since(closeStart))
	} else {
		log.Info("application closed successfully",
			"duration", time.Since(closeStart))
	}

	log.Info("shutdown complete")
	return runErr
}
