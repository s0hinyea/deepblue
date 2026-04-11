// internal/handlers/advisory.go
//
// Phase 5 — Spec 5.3: GET /api/entity/{id}/advisory
package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/models"
	"github.com/s0hinyea/deepblue/internal/services"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// AdvisoryHandler handles GET /api/entity/{id}/advisory.
//
// {id} can be either a MongoDB ObjectID hex string or the USGS site_id
// (e.g. "04085427"). It runs the full RAG + Claude pipeline and returns a
// 2-sentence plain-English safety advisory as JSON.
func AdvisoryHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"missing entity id"}`, http.StatusBadRequest)
		return
	}

	// Accept either ObjectID hex or the human-readable site_id string.
	filter := bson.M{"site_id": id}
	if oid, err := bson.ObjectIDFromHex(id); err == nil {
		filter = bson.M{"_id": oid}
	}

	var entity models.WaterEntity
	if err := db.EntitiesCollection.FindOne(r.Context(), filter).Decode(&entity); err != nil {
		http.Error(w, `{"error":"entity not found"}`, http.StatusNotFound)
		return
	}

	paragraphs, err := services.GetRelevantGuidelines(r.Context(), entity.OfficialMetrics)
	if err != nil {
		log.Printf("[ADVISORY] RAG search failed for %s: %v", id, err)
		http.Error(w, `{"error":"RAG search failed"}`, http.StatusInternalServerError)
		return
	}

	advisory, err := services.GenerateAdvisory(r.Context(), entity, paragraphs)
	if err != nil {
		log.Printf("[ADVISORY] GenerateAdvisory failed for %s: %v", id, err)
		http.Error(w, `{"error":"advisory generation failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"entity":   entity.Name,
		"advisory": advisory,
	})
}
