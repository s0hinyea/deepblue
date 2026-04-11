// internal/services/watcher.go
//
// Phase 4 (structure) + Phase 5 (AI pipeline) — Spec 4.2 & 5.2
//
// Lives in `services` (not `db`) to avoid an import cycle:
//   services → db is fine; db → services would be circular.
package services

import (
	"context"
	"log"
	"time"

	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// WatchReports opens a change stream on community_reports and, for every new
// insertion, runs the full Phase 5 AI pipeline:
//
//  1. Calls Claude 3.5 Sonnet (multimodal) to label the image.
//  2. Persists the returned risk tags on the report document.
//  3. Fetches the parent water_entity by site_id.
//  4. Recomputes the safety score (sensor data + visual risk).
//  5. Persists the new score and label back to the entity.
func WatchReports(ctx context.Context) {
	stream, err := db.ReportsCollection.Watch(ctx, mongo.Pipeline{})
	if err != nil {
		log.Fatalf("[WATCHER] Failed to open change stream: %v", err)
	}
	defer stream.Close(ctx)

	log.Println("[WATCHER] Listening for new community reports...")

	for stream.Next(ctx) {
		var event bson.M
		if err := stream.Decode(&event); err != nil {
			log.Printf("[WATCHER] Decode error: %v", err)
			continue
		}

		if opType, _ := event["operationType"].(string); opType != "insert" {
			continue
		}

		fullDoc, ok := event["fullDocument"].(bson.M)
		if !ok {
			continue
		}

		imageURL, _ := fullDoc["image_url"].(string)
		siteID, _ := fullDoc["site_id"].(string)
		reportID, _ := fullDoc["_id"].(bson.ObjectID)

		log.Printf("[WATCHER] Detected new survey! Image at: %s", imageURL)

		// Offload to a goroutine so the change stream cursor keeps advancing.
		go processReport(ctx, reportID, imageURL, siteID)
	}

	if err := stream.Err(); err != nil {
		log.Printf("[WATCHER] Change stream closed with error: %v", err)
	}
}

// processReport executes the AI pipeline for one newly inserted report.
func processReport(ctx context.Context, reportID bson.ObjectID, imageURL, siteID string) {
	opCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ── Step 1: Image analysis ────────────────────────────────────────────────
	tags, err := AnalyzeImageLabels(opCtx, imageURL)
	if err != nil {
		log.Printf("[WATCHER] AnalyzeImageLabels failed (report %s): %v", reportID.Hex(), err)
		tags = []string{}
	}
	log.Printf("[WATCHER] AI risk tags for report %s: %v", reportID.Hex(), tags)

	// ── Step 2: Persist tags onto the report document ─────────────────────────
	if _, err := db.ReportsCollection.UpdateOne(opCtx,
		bson.M{"_id": reportID},
		bson.M{"$set": bson.M{"ai_tags": tags}},
	); err != nil {
		log.Printf("[WATCHER] Failed to save AI tags: %v", err)
	}

	if siteID == "" {
		log.Printf("[WATCHER] No site_id on report %s — skipping entity score update", reportID.Hex())
		return
	}

	// ── Step 3: Load the parent water entity ─────────────────────────────────
	var entity models.WaterEntity
	if err := db.EntitiesCollection.FindOne(opCtx, bson.M{"site_id": siteID}).Decode(&entity); err != nil {
		log.Printf("[WATCHER] Water entity not found for site_id=%s: %v", siteID, err)
		return
	}

	// ── Step 4 & 5: Recompute and persist safety score ────────────────────────
	vAvg := visualRiskScore(tags)
	score := models.ComputeSafetyScore(entity.OfficialMetrics.PH, entity.OfficialMetrics.TurbidityNTU, vAvg)
	label := models.SafetyLabel(score)

	if _, err := db.EntitiesCollection.UpdateOne(opCtx,
		bson.M{"site_id": siteID},
		bson.M{
			"$set": bson.M{
				"safety_score":   score,
				"safety_label":   label,
				"last_report_at": time.Now().UTC(),
			},
			"$inc": bson.M{"report_count": 1},
		},
	); err != nil {
		log.Printf("[WATCHER] Failed to update entity safety score: %v", err)
		return
	}

	log.Printf("[WATCHER] Entity %s updated — score=%.3f label=%s (vAvg=%.2f)", siteID, score, label, vAvg)
}

// visualRiskScore maps AI-detected tags to a 0–1 visual-risk penalty (vAvg).
//
//	algae              → 0.9  (bloom risk, per WHO cyanobacteria guidance)
//	pollution, foam    → 0.7  (chemical / runoff contamination)
//	debris, discoloration → 0.5  (moderate visible hazard)
//	turbid             → 0.3  (reduced clarity only)
//	clear / no tags    → 0.0
func visualRiskScore(tags []string) float64 {
	highest := 0.0
	for _, tag := range tags {
		var v float64
		switch tag {
		case "algae":
			v = 0.9
		case "pollution", "foam":
			v = 0.7
		case "debris", "discoloration":
			v = 0.5
		case "turbid":
			v = 0.3
		}
		if v > highest {
			highest = v
		}
	}
	return highest
}
