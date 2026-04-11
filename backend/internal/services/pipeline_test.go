// internal/services/pipeline_test.go
//
// Integration tests for the full AI pipeline.
// These hit real AWS Bedrock and MongoDB Atlas — requires .env to be present.
//
// Run with:
//
//	go test ./internal/services/... -v -run TestPipeline -timeout 120s
package services_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/models"
	"github.com/s0hinyea/deepblue/internal/services"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// publicAlgaeImageURL is a publicly readable JPEG of murky/green water.
// Unsplash CDN — no hotlink restrictions, always returns 200.
const publicAlgaeImageURL = "https://images.unsplash.com/photo-1611273426858-450d8e3c9fce?w=640&q=80"

func setup(t *testing.T) {
	t.Helper()
	_ = godotenv.Load("../../.env")
	db.Connect()
}

// TestPipeline_AnalyzeImageLabels verifies that Claude returns at least one
// recognised risk tag for a visibly algae-laden water photo.
func TestPipeline_AnalyzeImageLabels(t *testing.T) {
	setup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tags, err := services.AnalyzeImageLabels(ctx, publicAlgaeImageURL)
	if err != nil {
		t.Fatalf("AnalyzeImageLabels error: %v", err)
	}

	t.Logf("Claude returned tags: %v", tags)

	if len(tags) == 0 {
		t.Fatal("expected at least one tag, got empty slice")
	}

	known := map[string]bool{
		"algae": true, "debris": true, "pollution": true,
		"discoloration": true, "foam": true, "turbid": true, "clear": true,
	}
	for _, tag := range tags {
		if !known[tag] {
			t.Errorf("unexpected tag %q — not in allowed set", tag)
		}
	}
}

// TestPipeline_GetRelevantGuidelines verifies that the RAG pipeline returns
// at least one paragraph from the knowledge_chunks collection for a query
// about elevated turbidity.
func TestPipeline_GetRelevantGuidelines(t *testing.T) {
	setup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	metrics := models.OfficialMetrics{
		PH:           8.9,  // high — algae bloom indicator
		TurbidityNTU: 35.0, // above EPA 25 NTU recreational threshold
		TempC:        24.0,
	}

	paragraphs, err := services.GetRelevantGuidelines(ctx, metrics)
	if err != nil {
		t.Fatalf("GetRelevantGuidelines error: %v", err)
	}

	t.Logf("RAG returned %d paragraphs", len(paragraphs))
	for i, p := range paragraphs {
		t.Logf("  [%d] %.120s...", i, p)
	}

	if len(paragraphs) == 0 {
		t.Fatal("expected at least one guideline paragraph, got none — is knowledge_chunks seeded?")
	}
}

// TestPipeline_GenerateAdvisory verifies that Claude produces a non-empty
// advisory string given a mocked water entity and guideline paragraphs.
func TestPipeline_GenerateAdvisory(t *testing.T) {
	setup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	entity := models.WaterEntity{
		Name: "Test Station — Hudson River",
		OfficialMetrics: models.OfficialMetrics{
			PH:           8.8,
			TurbidityNTU: 30.0,
			TempC:        22.5,
		},
		SafetyScore: 0.75,
	}
	guidelines := []string{
		"WHO guidelines recommend avoiding recreational contact when turbidity exceeds 25 NTU.",
		"Elevated pH above 8.5 combined with warm water temperatures can indicate cyanobacteria bloom conditions.",
	}

	advisory, err := services.GenerateAdvisory(ctx, entity, guidelines, "Community Visual Reports: None submitted in the last 30 days.")
	if err != nil {
		t.Fatalf("GenerateAdvisory error: %v", err)
	}

	t.Logf("Sensor advisory: %s", advisory.SensorAdvisory)
	t.Logf("Community advisory: %s", advisory.CommunityAdvisory)

	if strings.TrimSpace(advisory.SensorAdvisory) == "" {
		t.Fatal("sensor_advisory is empty")
	}
	if len(advisory.SensorAdvisory) < 30 {
		t.Errorf("sensor_advisory suspiciously short (%d chars): %q", len(advisory.SensorAdvisory), advisory.SensorAdvisory)
	}
	t.Logf("Community advisory: %s", advisory.CommunityAdvisory)
}

// TestPipeline_EndToEnd inserts a synthetic community report, waits for the
// watcher to process it via the AI pipeline, then asserts that the parent
// water entity's safety_score was updated.
//
// Prerequisites: a water entity with site_id "test-e2e-site" must exist (the
// test creates a minimal one and cleans it up).
func TestPipeline_EndToEnd(t *testing.T) {
	setup(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const testSiteID = "test-e2e-site"

	// Ensure a clean entity exists for the site.
	_, _ = db.EntitiesCollection.DeleteOne(ctx, bson.M{"site_id": testSiteID})
	_, err := db.EntitiesCollection.InsertOne(ctx, bson.M{
		"site_id":      testSiteID,
		"name":         "E2E Test Station",
		"safety_score": 0.0,
		"safety_label": "UNKNOWN",
		"official_metrics": bson.M{
			"ph":            7.2,
			"temp_c":        18.0,
			"turbidity_ntu": 5.0,
		},
	})
	if err != nil {
		t.Fatalf("failed to insert test entity: %v", err)
	}
	t.Cleanup(func() {
		cleanCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
		defer c()
		_, _ = db.EntitiesCollection.DeleteOne(cleanCtx, bson.M{"site_id": testSiteID})
		_, _ = db.ReportsCollection.DeleteMany(cleanCtx, bson.M{"site_id": testSiteID, "_test": true})
	})

	// Start the watcher.
	watchCtx, watchCancel := context.WithCancel(ctx)
	defer watchCancel()
	go services.WatchReports(watchCtx)

	// Give the change stream handshake time to complete.
	time.Sleep(2 * time.Second)

	// Insert a report that references our test entity.
	_, err = db.ReportsCollection.InsertOne(ctx, bson.M{
		"site_id":    testSiteID,
		"image_url":  publicAlgaeImageURL,
		"created_at": time.Now().UTC(),
		"_test":      true,
	})
	if err != nil {
		t.Fatalf("failed to insert test report: %v", err)
	}
	t.Logf("Test report inserted — waiting for AI pipeline...")

	// Poll until safety_score is updated or timeout.
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		var entity bson.M
		if err := db.EntitiesCollection.FindOne(ctx, bson.M{"site_id": testSiteID}).Decode(&entity); err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		score, _ := entity["safety_score"].(float64)
		label, _ := entity["safety_label"].(string)

		if score > 0 || (label != "" && label != "UNKNOWN") {
			t.Logf("Pipeline complete — safety_score=%.3f label=%s", score, label)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Error("pipeline did not update entity safety_score within 45s timeout")
}
