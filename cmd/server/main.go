package main

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"deepblue/internal/handlers"
)

func main() {
	addr := envOrDefault("PORT", "8080")

	templates, err := parseTemplates("client/templates")
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", handlers.NewHomeHandler(templates))
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("client/static"))))

	log.Printf("deepblue client listening on http://localhost:%s", addr)

	if err := http.ListenAndServe(":"+addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func parseTemplates(root string) (*template.Template, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() || filepath.Ext(path) != ".html" {
			return nil
		}

		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return template.ParseFiles(files...)
}
