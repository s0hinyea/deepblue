// internal/handlers/reports.go — POST /api/reports + GET /api/reports/{id}
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/s0hinyea/deepblue/internal/db"
	"github.com/s0hinyea/deepblue/internal/services"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func ReportsHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		jsonError(w, "File too large (max 10 MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		jsonError(w, "Image field is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	siteID := r.FormValue("site_id")
	if siteID == "" {
		jsonError(w, "Please select a station", http.StatusBadRequest)
		return
	}

	fileData, err := io.ReadAll(file)
	if err != nil {
		jsonError(w, "Could not read file", http.StatusInternalServerError)
		return
	}

	imageURL, err := services.UploadImage(r.Context(), fileData, header.Filename)
	if err != nil {
		jsonError(w, fmt.Sprintf("S3 upload failed: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := db.ReportsCollection.InsertOne(ctx, bson.M{
		"site_id":    siteID,
		"image_url":  imageURL,
		"created_at": time.Now().UTC(),
	})
	if err != nil {
		jsonError(w, "Database insert failed", http.StatusInternalServerError)
		return
	}

	// Return JSON with the report ID so the frontend can poll for AI tags
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"report_id": res.InsertedID.(bson.ObjectID).Hex(),
		"status":    "processing",
	})
}

// ReportStatusHandler returns the AI analysis results for a given report.
// GET /api/reports/{id}
func ReportStatusHandler(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("id")

	objID, err := bson.ObjectIDFromHex(reportID)
	if err != nil {
		jsonError(w, "Invalid report ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var doc bson.M
	if err := db.ReportsCollection.FindOne(ctx, bson.M{"_id": objID}).Decode(&doc); err != nil {
		jsonError(w, "Report not found", http.StatusNotFound)
		return
	}

	// Check if AI tags have been saved yet
	tags, hasTags := doc["ai_tags"]
	status := "processing"
	if hasTags && tags != nil {
		status = "complete"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"report_id": reportID,
		"site_id":   doc["site_id"],
		"status":    status,
		"ai_tags":   tags,
	})
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
