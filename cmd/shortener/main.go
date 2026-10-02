package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/D0CCi/go-shortener-service/internal/httpapi"
	"github.com/D0CCi/go-shortener-service/internal/shortener"
	"github.com/D0CCi/go-shortener-service/internal/storage/memory"
)

const (
	readHeaderTimeout = 5 * time.Second  // slow clients cannot hold a connection (slowloris)
	readTimeout       = 10 * time.Second // whole request including body
	writeTimeout      = 10 * time.Second // whole response
	idleTimeout       = 60 * time.Second // keep-alive connection with no requests
	shutdownTimeout   = 5 * time.Second
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	baseURL := flag.String("base-url", "http://localhost:8080", "prefix for short links")
	storageType := flag.String("storage", "memory", "storage backend: memory or postgres")
	flag.Parse()

	var st shortener.Storage
	switch *storageType {
	case "memory":
		st = memory.New()
	default:
		log.Fatalf("unknown storage %q", *storageType)
	}

	svc := shortener.New(st)
	h := httpapi.New(svc, *baseURL)

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
			log.Fatal(err)
		}
	}()
	log.Printf("listening on %s", *addr)

	<-ctx.Done() // blocks until Ctrl+C or docker stop
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("stopped")
}
