package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/services"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestFetchAndSyncWaterData calls the real USGS endpoint and verifies that all
// three tracked sites land in MongoDB with non-zero metric values.
func TestFetchAndSyncWaterData(t *testing.T) {
	_ = godotenv.Load("../../.env")
	db.Connect()

	services.FetchAndSyncWaterData()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Each site is keyed by site_id — check all three are present with real data.
	sites := []struct {
		id   string
		name string
	}{
		{"01306460", "East Rockaway Inlet"},
		{"01372058", "Hudson River"},
		{"04085427", "Manitowoc River"},
	}

	for _, s := range sites {
		t.Run(s.name, func(t *testing.T) {
			var doc bson.M
			err := db.EntitiesCollection.FindOne(ctx, bson.M{"site_id": s.id}).Decode(&doc)
			if err != nil {
				t.Fatalf("site %s not found in water_entities: %v", s.id, err)
			}

			metrics, ok := doc["official_metrics"].(bson.M)
			if !ok {
				t.Fatalf("site %s missing official_metrics field", s.id)
			}

			ph, _ := metrics["ph"].(float64)
			tempC, _ := metrics["temp_c"].(float64)
			turb, _ := metrics["turbidity_ntu"].(float64)

			if ph == 0 && tempC == 0 && turb == 0 {
				t.Errorf("site %s: all metrics are zero — sync likely failed", s.id)
			}

			t.Logf("site %-12s | pH=%.2f  temp=%.1f°C  turb=%.2f NTU", s.id, ph, tempC, turb)
		})
	}
}
