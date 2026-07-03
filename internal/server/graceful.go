package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/CURSED-ME/loopers-oss/internal/logging"
)

// ListenAndServeWithGracefulShutdown starts the HTTP server and listens for SIGTERM/SIGINT.
func ListenAndServeWithGracefulShutdown(srv *http.Server, adminSrv *http.Server, redisClient interface{ Close() error }, certFile, keyFile string, insecureDev bool, otelShutdown func(context.Context) error, extraShutdowns ...func()) {
	// Channel to listen for signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if insecureDev {
			logging.Logger.Warn().Msg("SERVER_INSECURE_DEV is true. Starting proxy in plaintext mode without TLS. DO NOT USE IN PRODUCTION.")
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logging.Logger.Fatal().Err(err).Msg("Server failed to start")
			}
			return
		}

		if certFile != "" && keyFile != "" {
			if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
				logging.Logger.Fatal().Err(err).Msg("Server failed to start (TLS)")
			}
		} else {
			logging.Logger.Fatal().Msg("TLS is required in production. Please configure server.tls_cert_file and server.tls_key_file")
		}
	}()

	if adminSrv != nil {
		go func() {
			logging.Logger.Info().Msgf("Admin server started listening on %s", adminSrv.Addr)
			if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logging.Logger.Fatal().Err(err).Msg("Admin server failed to start")
			}
		}()
	}

	logging.Logger.Info().Msgf("Server started listening on %s", srv.Addr)

	// Block until a signal is received
	sig := <-quit
	logging.Logger.Info().Msgf("Received signal: %v. Initiating graceful shutdown...", sig)

	// Log pre-shutdown goroutine count for troubleshooting leak issues
	logging.Logger.Info().Int("goroutines_before_shutdown", runtime.NumGoroutine()).Msg("Goroutines count audit")

	// Create shutdown context with a 30s timeout limit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logging.Logger.Error().Err(err).Msg("Server forced to shutdown prematurely")
	} else {
		logging.Logger.Info().Msg("HTTP server closed gracefully")
	}

	for _, shutdownFn := range extraShutdowns {
		if shutdownFn != nil {
			shutdownFn()
		}
	}

	if adminSrv != nil {
		if err := adminSrv.Shutdown(ctx); err != nil {
			logging.Logger.Error().Err(err).Msg("Admin server forced to shutdown prematurely")
		} else {
			logging.Logger.Info().Msg("Admin server closed gracefully")
		}
	}

	// Shutdown OpenTelemetry
	if otelShutdown != nil {
		if err := otelShutdown(ctx); err != nil {
			logging.Logger.Error().Err(err).Msg("Error closing OTel provider")
		} else {
			logging.Logger.Info().Msg("OpenTelemetry shut down gracefully")
		}
	}

	// Close Redis connection pool
	if redisClient != nil {
		if err := redisClient.Close(); err != nil {
			logging.Logger.Error().Err(err).Msg("Error closing Redis connection pool")
		} else {
			logging.Logger.Info().Msg("Redis connection closed gracefully")
		}
	}

	// Log post-shutdown goroutine count to ensure cleanup of all background tasks
	logging.Logger.Info().Int("goroutines_after_shutdown", runtime.NumGoroutine()).Msg("Goroutines count audit")
}
