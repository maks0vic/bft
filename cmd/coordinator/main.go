package main

import (
	"flag"
	"log"
	"net/http"

	"bft/internal/config"
	"bft/internal/coordinator"
)

func main() {
	configDir := flag.String("config-dir", "configs", "directory with node configs")
	addr := flag.String("addr", "localhost:9000", "coordinator listen address")
	flag.Parse()

	configs, err := config.LoadDir(*configDir)
	if err != nil {
		log.Fatalf("load configs: %v", err)
	}

	service := coordinator.New(configs)

	mux := http.NewServeMux()
	mux.HandleFunc("/state", service.HandleState)
	mux.HandleFunc("/events", service.HandleEvents)
	mux.HandleFunc("/start", service.HandleStart)
	mux.HandleFunc("/reset", service.HandleReset)

	log.Printf("[coordinator] listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
