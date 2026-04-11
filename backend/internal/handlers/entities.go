// internal/handlers/entities.go — GET /api/entities
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/s0hinyea/deepblue/internal/db"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func EntitiesHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cursor, err := db.EntitiesCollection.Find(ctx, bson.M{})
	if err != nil {
		http.Error(w, `{"error":"db query failed"}`, http.StatusInternalServerError)
		return
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		http.Error(w, `{"error":"decode failed"}`, http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []bson.M{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
