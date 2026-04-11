package handlers

import (
	"html/template"
	"net/http"
)

type HomePageData struct{}

type HomeHandler struct {
	templates *template.Template
}

func NewHomeHandler(templates *template.Template) *HomeHandler {
	return &HomeHandler{templates: templates}
}

func (h *HomeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.templates.ExecuteTemplate(w, "base", HomePageData{}); err != nil {
		http.Error(w, "failed to render page", http.StatusInternalServerError)
	}
}
