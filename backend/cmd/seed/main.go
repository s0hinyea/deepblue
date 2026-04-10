// cmd/seed/main.go
//
// Standalone pre-seed script.  Run once before the demo:
//
//	go run ./cmd/seed
//
// What it does:
//  1. Drops the existing water_entities collection (idempotent re-runs).
//  2. Creates a 2dsphere index on the `location` field.
//  3. Inserts 25 real NYC/NJ water bodies with realistic sensor metrics.
//  4. Computes and stores the Safety Score for each entity.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found")
	}

	db.Connect()
	ctx := context.Background()

	col := db.EntitiesCollection

	// ── 1. Drop existing data so the script is safe to re-run ────────────────
	if err := col.Drop(ctx); err != nil {
		log.Fatalf("Failed to drop collection: %v", err)
	}
	fmt.Println("Dropped water_entities collection.")

	// ── 2. Create 2dsphere index on `location` ────────────────────────────────
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "location", Value: "2dsphere"}},
		Options: nil,
	}
	if _, err := col.Indexes().CreateOne(ctx, indexModel); err != nil {
		log.Fatalf("Failed to create 2dsphere index: %v", err)
	}
	fmt.Println("Created 2dsphere index on location.")

	// ── 3. Build seed data ────────────────────────────────────────────────────
	now := time.Now()

	type seedEntry struct {
		name        string
		lon, lat    float64
		ph          float64
		tempC       float64
		turbidity   float64
	}

	seeds := []seedEntry{
		// Brooklyn
		{"Prospect Park Lake", -73.9692, 40.6601, 7.1, 19.5, 5.2},
		{"Marine Park Salt Marsh", -73.9161, 40.5988, 7.6, 21.0, 12.4},
		{"Ridgewood Reservoir", -73.8951, 40.6800, 6.9, 17.8, 3.1},

		// Manhattan
		{"Central Park Reservoir", -73.9584, 40.7851, 7.3, 18.2, 4.7},
		{"Central Park Lake", -73.9696, 40.7755, 7.0, 19.1, 6.3},
		{"Harlem Meer", -73.9521, 40.7966, 7.8, 20.4, 9.8},
		{"Turtle Pond", -73.9693, 40.7796, 6.8, 18.9, 3.5},
		{"Hudson River (Chelsea)", -74.0082, 40.7488, 7.4, 22.3, 18.7},
		{"East River (Roosevelt Island)", -73.9519, 40.7629, 7.5, 21.8, 22.1},

		// Queens
		{"Jamaica Bay (Broad Channel)", -73.8370, 40.6143, 7.9, 23.1, 14.6},
		{"Flushing Meadows Lake", -73.8453, 40.7441, 8.1, 22.7, 19.3},
		{"Rockaway Beach", -73.8076, 40.5734, 7.2, 20.6, 8.1},
		{"Jacob Riis Park Shoreline", -73.8698, 40.5608, 7.1, 19.9, 6.4},
		{"Little Neck Bay", -73.7481, 40.7766, 7.7, 21.3, 11.2},

		// Bronx
		{"Pelham Bay (Hunter Island)", -73.7990, 40.8696, 7.3, 20.8, 7.9},
		{"Van Cortlandt Lake", -73.8951, 40.8981, 6.7, 16.4, 4.2},
		{"Orchard Beach", -73.7870, 40.8693, 7.6, 21.5, 13.8},
		{"Bronx River (Starlight Park)", -73.8797, 40.8321, 8.3, 23.7, 24.5},

		// Staten Island
		{"Silver Lake Reservoir", -74.0923, 40.6272, 6.9, 17.1, 3.8},
		{"Clove Lakes Park Pond", -74.1181, 40.6272, 7.2, 18.6, 5.5},
		{"Wolfe's Pond", -74.1927, 40.5196, 7.0, 17.9, 4.0},
		{"Great Kills Harbor", -74.1508, 40.5514, 7.8, 22.0, 16.3},

		// New Jersey
		{"Liberty State Park Cove", -74.0544, 40.7028, 7.5, 22.4, 17.9},
		{"Hackensack River (Laurel Hill)", -74.0438, 40.8866, 8.4, 24.1, 28.6},
		{"Sandy Hook Bay", -74.0021, 40.4617, 7.1, 20.2, 7.3},
	}

	docs := make([]interface{}, 0, len(seeds))
	for _, s := range seeds {
		score := models.ComputeSafetyScore(s.ph, s.turbidity, 0.0)
		entity := models.WaterEntity{
			Name: s.name,
			Location: models.GeoPoint{
				Type:        "Point",
				Coordinates: [2]float64{s.lon, s.lat},
			},
			OfficialMetrics: models.OfficialMetrics{
				PH:           s.ph,
				TempC:        s.tempC,
				TurbidityNTU: s.turbidity,
				LastUpdated:  now,
			},
			SafetyScore:  score,
			SafetyLabel:  models.SafetyLabel(score),
			ReportCount:  0,
			LastReportAt: now,
		}
		docs = append(docs, entity)
	}

	// ── 4. Insert all documents ───────────────────────────────────────────────
	res, err := col.InsertMany(ctx, docs)
	if err != nil {
		log.Fatalf("InsertMany failed: %v", err)
	}

	fmt.Printf("Inserted %d water entities.\n", len(res.InsertedIDs))
	fmt.Println("Pre-seed complete. MongoDB water_entities is ready.")
}
