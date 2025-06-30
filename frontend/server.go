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

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	layoutPath := filepath.Join("templates", "layout.html")
	contentPath := filepath.Join("templates", "content", fmt.Sprintf("%s.html", tmpl))

	t, err := template.ParseFiles(layoutPath, contentPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func renderContent(w http.ResponseWriter, tmpl string) {
	templatePath := filepath.Join("templates", "content", fmt.Sprintf("%s.html", tmpl))

	t, err := template.ParseFiles(templatePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := t.ExecuteTemplate(w, "Content", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		renderContent(w, "home")
		return
	}

	contentTemplatePath := filepath.Join("templates", "content", "home.html")
	t, err := template.ParseFiles(contentTemplatePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var contentBuffer bytes.Buffer
	if err := t.ExecuteTemplate(&contentBuffer, "Content", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Content": template.HTML(contentBuffer.String()),
	}
	renderTemplate(w, "home", data)
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		renderContent(w, "about")
	} else {
		renderTemplate(w, "about", nil)
	}
}

func workHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		renderContent(w, "work-history")
	} else {
		renderTemplate(w, "work-history", nil)
	}
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		renderContent(w, "projects")
	} else {
		renderTemplate(w, "projects", nil)
	}
}

func speakingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		renderContent(w, "speaking-engagements")
	} else {
		renderTemplate(w, "speaking-engagements", nil)
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		renderContent(w, "metrics")
	} else {
		renderTemplate(w, "metrics", nil)
	}
}

func contactHandler(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "true" {
		renderContent(w, "contact-me")
	} else {
		renderTemplate(w, "contact-me", nil)
	}
}
