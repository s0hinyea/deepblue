// internal/handlers/reports.go — POST /api/reports
package handlers

import (
	"context"
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
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `<span class="toast-error">File too large (max 10 MB).</span>`)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `<span class="toast-error">Image field is required.</span>`)
		return
	}
	defer file.Close()

	siteID := r.FormValue("site_id")
	if siteID == "" {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `<span class="toast-error">Please select a station.</span>`)
		return
	}

	fileData, err := io.ReadAll(file)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `<span class="toast-error">Could not read file.</span>`)
		return
	}

	imageURL, err := services.UploadImage(r.Context(), fileData, header.Filename)
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `<span class="toast-error">S3 upload failed: %s</span>`, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err = db.ReportsCollection.InsertOne(ctx, bson.M{
		"site_id":    siteID,
		"image_url":  imageURL,
		"created_at": time.Now().UTC(),
	}); err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `<span class="toast-error">Database insert failed.</span>`)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<span class="toast-success">Report received — AI analysis starting...</span>`)
}
