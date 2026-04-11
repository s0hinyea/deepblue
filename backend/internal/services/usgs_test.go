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

// TestFetchAndSyncWaterData calls the real USGS NY endpoint, syncs to MongoDB,
// and asserts that a meaningful number of stations landed with real data.
func TestFetchAndSyncWaterData(t *testing.T) {
	_ = godotenv.Load("../../.env")
	db.Connect()

	services.FetchAndSyncWaterData()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	count, err := db.EntitiesCollection.CountDocuments(ctx, bson.M{
		"official_metrics.last_updated": bson.M{"$exists": true},
	})
	if err != nil {
		t.Fatalf("CountDocuments failed: %v", err)
	}

	t.Logf("NY stations synced: %d", count)

	if count < 10 {
		t.Errorf("expected at least 10 NY stations, got %d", count)
	}

	// Spot-check one document has non-zero coords and at least one metric.
	var doc bson.M
	err = db.EntitiesCollection.FindOne(ctx, bson.M{
		"location": bson.M{"$exists": true},
	}).Decode(&doc)
	if err != nil {
		t.Fatalf("no entity with location found: %v", err)
	}

	metrics, _ := doc["official_metrics"].(bson.M)
	ph, _   := metrics["ph"].(float64)
	temp, _ := metrics["temp_c"].(float64)
	turb, _ := metrics["turbidity_ntu"].(float64)

	if ph == 0 && temp == 0 && turb == 0 {
		t.Errorf("spot-check entity has all-zero metrics")
	}
	t.Logf("spot-check site=%v name=%v ph=%.2f temp=%.1f turb=%.2f",
		doc["site_id"], doc["name"], ph, temp, turb)
}
