package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/joho/godotenv"
	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/handlers"
)

func main() {
	// 1. Load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found (this is fine if vars are set in system)")
	}

	// 2. Connect to MongoDB
	db.Connect()

	// 3. Register routes
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handlers.HomeHandler)

	// 4. Start the HTTP server
	addr := ":8080"
	fmt.Printf("DeepBlue server running on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
