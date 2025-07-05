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
	http.Handle(
		"/static/",
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/home", homeHandler)
	http.HandleFunc("/work-history", workHandler)
	http.HandleFunc("/projects", projectsHandler)
	http.HandleFunc("/speaking-engagements", speakingHandler)
	http.HandleFunc("/metrics", metricsHandler)
	http.HandleFunc("/contact-me", contactHandler)

	fmt.Println("Server running on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
}

func renderPage(w http.ResponseWriter, r *http.Request, page string, data interface{}) {
	contentPath := filepath.Join("templates", "content", fmt.Sprintf("%s.html", page))

	// HTMX: render only the inner content template
	if r.Header.Get("HX-Request") == "true" {
		t := template.Must(template.ParseFiles(contentPath))
		t.ExecuteTemplate(w, "Content", data) // ✅ FIXED
		return
	}

	// Non-HTMX: render full layout with content injected
	contentBuf := new(bytes.Buffer)
	t := template.Must(template.ParseFiles(contentPath))
	t.ExecuteTemplate(contentBuf, "Content", data) // ✅ FIXED

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
