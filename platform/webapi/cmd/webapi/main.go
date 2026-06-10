// Command webapi is the read-only HTTP gateway that serves TALOS orchestration
// state (worktrees / merge-order / overlap / agents) to the web console. It
// reads the local repo with git + team-context/ownership.md only — no
// wt/mo/ov/ch binaries.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/John-Santa/talos/platform/webapi/adapter/gitfs"
	"github.com/John-Santa/talos/platform/webapi/adapter/httpapi"
	"github.com/John-Santa/talos/platform/webapi/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8100"
	}

	reader, err := gitfs.NewAutodetect(os.Getenv("TALOS_BASE_BRANCH"))
	if err != nil {
		logger.Error("could not locate the talos git repo", "err", err)
		os.Exit(1)
	}

	svc := service.NewGateway(reader)
	handler := httpapi.New(svc, os.Getenv("WEBAPI_CORS_ORIGIN"), logger)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("talos webapi listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutCtx); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
	}
}
