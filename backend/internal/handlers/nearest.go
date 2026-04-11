package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func GetNearestSafeSite(w http.ResponseWriter, r *http.Request) {
	latStr := r.URL.Query().Get("lat")
	lngStr := r.URL.Query().Get("lng")

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid lat"}`, http.StatusBadRequest)
		return
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid lng"}`, http.StatusBadRequest)
		return
	}

	filter := bson.M{
		"safety_label": "safe",
		"location": bson.M{
			"$near": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					"coordinates": []float64{lng, lat},
				},
			},
		},
	}

	var entity models.WaterEntity
	if err := db.EntitiesCollection.FindOne(r.Context(), filter).Decode(&entity); err != nil {
		http.Error(w, `{"error":"no safe station found nearby"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entity)
}
