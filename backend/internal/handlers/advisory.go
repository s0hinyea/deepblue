// internal/handlers/advisory.go
//
// Phase 5 — Spec 5.3: GET /api/entity/{id}/advisory
package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/models"
	"github.com/s0hinyea/deepblue/internal/services"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func AdvisoryHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"missing entity id"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{"site_id": id}
	if oid, err := bson.ObjectIDFromHex(id); err == nil {
		filter = bson.M{"_id": oid}
	}

	var entity models.WaterEntity
	if err := db.EntitiesCollection.FindOne(r.Context(), filter).Decode(&entity); err != nil {
		http.Error(w, `{"error":"entity not found"}`, http.StatusNotFound)
		return
	}

	// ── Fetch recent community reports (last 30 days, AI-tagged only) ─────────
	siteID := entity.SiteID
	communityContext := "Community Visual Reports: None submitted in the last 30 days."
	if siteID != "" {
		since := time.Now().UTC().AddDate(0, 0, -30)
		rctx := r.Context()
		cursor, err := db.ReportsCollection.Find(rctx,
			bson.M{
				"site_id":    siteID,
				"ai_tags":    bson.M{"$exists": true, "$not": bson.M{"$size": 0}},
				"created_at": bson.M{"$gte": since},
			},
			options.Find().SetSort(bson.M{"created_at": -1}).SetLimit(5),
		)
		if err == nil {
			var reports []bson.M
			cursor.All(rctx, &reports)
			if len(reports) > 0 {
				lines := make([]string, 0, len(reports))
				for _, rep := range reports {
					tags := formatTags(rep["ai_tags"])
					date := ""
					if t, ok := rep["created_at"].(time.Time); ok {
						date = t.Format("2006-01-02")
					}
					lines = append(lines, fmt.Sprintf("- %s (reported %s)", tags, date))
				}
				communityContext = fmt.Sprintf(
					"Community Visual Reports (%d submission(s) in last 30 days):\n%s",
					len(reports), strings.Join(lines, "\n"),
				)
			}
		}
	}

	paragraphs, err := services.GetRelevantGuidelines(r.Context(), entity.OfficialMetrics)
	if err != nil {
		log.Printf("[ADVISORY] RAG search failed for %s: %v", id, err)
		http.Error(w, `{"error":"RAG search failed"}`, http.StatusInternalServerError)
		return
	}

	advisory, err := services.GenerateAdvisory(r.Context(), entity, paragraphs, communityContext)
	if err != nil {
		log.Printf("[ADVISORY] GenerateAdvisory failed for %s: %v", id, err)
		http.Error(w, `{"error":"advisory generation failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"entity":             entity.Name,
		"sensor_advisory":    advisory.SensorAdvisory,
		"community_advisory": advisory.CommunityAdvisory,
	})
}

// formatTags converts ai_tags ([]interface{} from bson.M) to a readable string.
func formatTags(raw interface{}) string {
	arr, ok := raw.(bson.A)
	if !ok {
		return "unknown"
	}
	tags := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok {
			tags = append(tags, s)
		}
	}
	if len(tags) == 0 {
		return "unknown"
	}
	return strings.Join(tags, ", ")
}
