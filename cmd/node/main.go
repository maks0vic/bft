package main

import (
	"flag"
	"log"
	"net/http"

	"bft/internal/config"
	"bft/internal/node"
)

func main() {
	configPath := flag.String("config", "", "path to node config")
	flag.Parse()

	if *configPath == "" {
		log.Fatal("missing -config")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	n := node.NewNode(cfg)

	mux := http.NewServeMux()
	n.RegisterRoutes(mux)

	log.Printf("[%s] starting on %s byzantine=%v behavior=%s leader=%v",
		cfg.ID, cfg.Address, cfg.Byzantine, cfg.Behavior, cfg.Leader)

	log.Fatal(http.ListenAndServe(cfg.Address, mux))
}
