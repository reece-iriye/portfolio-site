package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	mux := http.NewServeMux()
	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	mux.HandleFunc("GET /", homeHandler)
	mux.HandleFunc("GET /home", homeHandler)
	mux.HandleFunc("GET /work-history", workHandler)
	mux.HandleFunc("GET /projects", projectsHandler)
	mux.HandleFunc("GET /speaking-engagements", speakingHandler)
	mux.HandleFunc("GET /metrics", metricsHandler)
	mux.HandleFunc("GET /contact-me", contactHandler)

	fmt.Println("Server running on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
}

func renderPage(w http.ResponseWriter, r *http.Request, page string, data interface{}) {
	contentPath := filepath.Join("templates", "content", fmt.Sprintf("%s.html", page))

	// HTMX: render only the inner content template
	if r.Header.Get("HX-Request") == "true" {
		t := template.Must(template.ParseFiles(contentPath))
		t.ExecuteTemplate(w, "Content", data)
		return
	}

	// Non-HTMX: render full layout with content injected
	contentBuf := new(bytes.Buffer)
	t := template.Must(template.ParseFiles(contentPath))
	t.ExecuteTemplate(contentBuf, "Content", data)

	fullLayout := template.Must(template.ParseFiles("templates/layout.html"))
	fullLayout.Execute(w, map[string]interface{}{
		"Content": template.HTML(contentBuf.String()),
	})
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, r, "home", nil)
}

func workHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, r, "work-history", nil)
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, r, "projects", nil)
}

func speakingHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, r, "speaking-engagements", nil)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, r, "metrics", nil)
}

func contactHandler(w http.ResponseWriter, r *http.Request) {
	renderPage(w, r, "contact-me", nil)
}
