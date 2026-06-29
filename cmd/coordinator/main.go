package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bft/internal/coordinator"
)

func main() {
	addr := flag.String("addr", "localhost:9000", "coordinator listen address")
	nodeBasePort := flag.Int("node-base-port", 8001, "starting port for generated node processes")
	flag.Parse()

	repoRoot, err := os.Getwd()
	if err != nil {
		log.Fatalf("get working directory: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	service := coordinator.New(repoRoot, *nodeBasePort, ctx)
	defer func() {
		if err := service.Close(); err != nil {
			log.Printf("[coordinator] shutdown cleanup error: %v", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/state", service.HandleState)
	mux.HandleFunc("/events", service.HandleEvents)
	mux.HandleFunc("/start", service.HandleStart)
	mux.HandleFunc("/reset", service.HandleReset)

	server := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("[coordinator] listening on %s", *addr)
	err = server.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
