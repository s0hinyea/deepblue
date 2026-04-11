package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/joho/godotenv"
	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/handlers"
	"github.com/s0hinyea/deepblue/internal/services"
)

func main() {
	// 1. Load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found (this is fine if vars are set in system)")
	}

	// 2. Connect to MongoDB
	db.Connect()

	// 3. Phase 3 — Spec 3.2: Background metronome goroutine.
	//    Fires immediately on startup, then every 15 minutes.
	go func() {
		services.FetchAndSyncWaterData()
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			services.FetchAndSyncWaterData()
		}
	}()

	// 4. Phase 4/5 — Spec 4.2 + 5.1/5.2: Change stream watcher with AI pipeline.
	go services.WatchReports(context.Background())

	// 5. Register routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handlers.HomeHandler)
	mux.HandleFunc("GET /api/entities", handlers.EntitiesHandler)
	mux.HandleFunc("POST /api/reports", handlers.ReportsHandler)
	mux.HandleFunc("GET /api/reports/{id}", handlers.ReportStatusHandler)
	mux.HandleFunc("GET /api/entity/{id}/advisory", handlers.AdvisoryHandler)
	mux.HandleFunc("GET /api/nearest-safe", handlers.GetNearestSafeSite)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// 6. Start the HTTP server
	addr := ":8080"
	fmt.Printf("DeepBlue server running on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
