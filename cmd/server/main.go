package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/govwallet/redemption/internal/handler"
	"github.com/govwallet/redemption/internal/redemption"
	"github.com/govwallet/redemption/internal/staffmapping"
)

func main() {
	var (
		addr            = flag.String("addr", envOrDefault("ADDR", ":8080"), "HTTP listen address")
		staffMappingCSV = flag.String("staff-mapping", envOrDefault("STAFF_MAPPING_CSV", "data/staff_mapping.csv"), "Path to staff mapping CSV")
		redemptionCSV   = flag.String("redemption-data", envOrDefault("REDEMPTION_CSV", "data/redemptions.csv"), "Path to redemption data CSV (created if absent)")
	)
	flag.Parse()

	// Load staff mapping.
	store := staffmapping.NewStore()
	if err := store.LoadFromFile(*staffMappingCSV); err != nil {
		log.Fatalf("failed to load staff mapping from %q: %v", *staffMappingCSV, err)
	}
	log.Printf("loaded staff mapping from %q", *staffMappingCSV)

	// Initialise redemption service (loads existing redemption data).
	svc, err := redemption.NewService(*redemptionCSV)
	if err != nil {
		log.Fatalf("failed to initialise redemption service: %v", err)
	}
	log.Printf("redemption data backed by %q", *redemptionCSV)

	// Register HTTP routes.
	mux := http.NewServeMux()
	handler.New(mux, store, svc)

	log.Printf("server listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
