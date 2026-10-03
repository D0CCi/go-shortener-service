package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/D0CCi/go-shortener-service/internal/httpapi"
	"github.com/D0CCi/go-shortener-service/internal/shortener"
	"github.com/D0CCi/go-shortener-service/internal/storage/memory"
	"github.com/D0CCi/go-shortener-service/internal/storage/postgres"
)

const (
	readHeaderTimeout = 5 * time.Second  // slow clients cannot hold a connection (slowloris)
	readTimeout       = 10 * time.Second // whole request including body
	writeTimeout      = 10 * time.Second // whole response
	idleTimeout       = 60 * time.Second // keep-alive connection with no requests
	shutdownTimeout   = 5 * time.Second  // must be below docker stop's 10 s grace period
	connectTimeout    = 5 * time.Second  // fail fast if the database is unreachable at startup
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	baseURL := flag.String("base-url", "http://localhost:8080", "prefix for short links")
	storageType := flag.String("storage", "memory", "storage backend: memory or postgres")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	var st shortener.Storage
	switch *storageType {
	case "memory":
		st = memory.New()
	case "postgres":
		// The DSN contains a password, so it comes from the environment, not a flag visible in ps.
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			logger.Error("DATABASE_URL is not set")
			os.Exit(1)
		}
		ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
		pg, err := postgres.New(ctx, dsn)
		cancel()
		if err != nil {
			logger.Error("connect to postgres", "err", err)
			os.Exit(1)
		}
		defer pg.Close() // runs when main returns, after srv.Shutdown
		st = pg
	default:
		logger.Error("unknown storage", "storage", *storageType)
		os.Exit(1)
	}

	svc := shortener.New(st)
	h := httpapi.New(svc, *baseURL, logger)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "err", err)
			os.Exit(1)
		}
	}()
	logger.Info("listening", "addr", *addr)

	<-ctx.Done() // blocks until Ctrl+C or docker stop
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown failed", "err", err)
	}
	logger.Info("stopped")
}
