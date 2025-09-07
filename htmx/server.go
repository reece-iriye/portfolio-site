package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/gomail.v2"
)

type ContactFormData struct {
	Name    string
	Reason  string
	Subject string
	Body    string
}

type TemplateManager struct {
	layout  *template.Template
	content map[string]*template.Template
}

var templates TemplateManager

func main() {
	if err := loadTemplates(); err != nil {
		fmt.Printf("Error loading templates: %v\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/api/home", homeHandler)
	mux.HandleFunc("/api/work-history", workHandler)
	// mux.HandleFunc("/projects", projectsHandler)
	// mux.HandleFunc("/speaking-engagements", speakingHandler)
	mux.HandleFunc("/api/metrics", metricsHandler)
	mux.HandleFunc("/api/contact-me", contactHandler)
	mux.HandleFunc("/api/contact", contactSubmitHandler)

	fmt.Println("HTMX server running on :8080...")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		fmt.Println("Error starting server:", err)
		os.Exit(1)
	}
}

// loadTemplates pre-parses all templates into memory
func loadTemplates() error {
	templates.content = make(map[string]*template.Template)

	// Load layout
	layoutPath := filepath.Join("templates", "layout.html")
	layout, err := template.ParseFiles(layoutPath)
	if err != nil {
		return fmt.Errorf("failed to parse layout template: %w", err)
	}
	templates.layout = layout

	// Load all content templates
	contentDir := filepath.Join("templates", "content")
	files, err := os.ReadDir(contentDir)
	if err != nil {
		return fmt.Errorf("failed to read content directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := strings.TrimSuffix(file.Name(), filepath.Ext(file.Name()))
		path := filepath.Join(contentDir, file.Name())
		t, err := template.ParseFiles(path)
		if err != nil {
			return fmt.Errorf("failed to parse content template %s: %w", file.Name(), err)
		}
		templates.content[name] = t
	}

	return nil
}

func renderPage(w http.ResponseWriter, r *http.Request, page string, data interface{}) {
	t, ok := templates.content[page]
	if !ok {
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// HTMX request: only render inner content
	if r.Header.Get("HX-Request") == "true" {
		if err := t.ExecuteTemplate(w, "Content", data); err != nil {
			http.Error(
				w,
				fmt.Sprintf("Error executing template: %v", err),
				http.StatusInternalServerError,
			)
		}
		return
	}

	contentBuf := new(bytes.Buffer)
	if err := t.ExecuteTemplate(contentBuf, "Content", data); err != nil {
		http.Error(
			w,
			fmt.Sprintf("Error executing template: %v", err),
			http.StatusInternalServerError,
		)
		return
	}

	if err := templates.layout.Execute(w, map[string]interface{}{
		"Content": template.HTML(contentBuf.String()),
	}); err != nil {
		http.Error(
			w,
			fmt.Sprintf("Error executing layout template: %v", err),
			http.StatusInternalServerError,
		)
	}
}

// ---- Handlers ----
func homeHandler(w http.ResponseWriter, r *http.Request)     { handleGet(w, r, "home") }
func workHandler(w http.ResponseWriter, r *http.Request)     { handleGet(w, r, "work-history") }
func projectsHandler(w http.ResponseWriter, r *http.Request) { handleGet(w, r, "projects") }
func speakingHandler(w http.ResponseWriter, r *http.Request) {
	handleGet(w, r, "speaking-engagements")
}
func metricsHandler(w http.ResponseWriter, r *http.Request) { handleGet(w, r, "metrics") }
func contactHandler(w http.ResponseWriter, r *http.Request) { handleGet(w, r, "contact-me") }

func handleGet(w http.ResponseWriter, r *http.Request, page string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusBadRequest)
		return
	}
	renderPage(w, r, page, nil)
}

// ---- Contact Form ----
func contactSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(
			w,
			`<div class="form-response error">Error parsing form data. Please try again.</div>`,
		)
		return
	}

	formData := ContactFormData{
		Name:    strings.TrimSpace(r.FormValue("name")),
		Reason:  strings.TrimSpace(r.FormValue("reason")),
		Subject: strings.TrimSpace(r.FormValue("subject")),
		Body:    strings.TrimSpace(r.FormValue("body")),
	}

	if formData.Name == "" || formData.Reason == "" || formData.Subject == "" ||
		formData.Body == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(
			w,
			`<div class="form-response error">All fields are required. Please fill out the complete form.</div>`,
		)
		return
	}

	if err := sendContactEmail(formData); err != nil {
		fmt.Printf("Error sending email: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(
			w,
			`<div class="form-response error">There was an error sending your message. Please try again or contact me directly via LinkedIn.</div>`,
		)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(
		w,
		`<div class="form-response success">Thank you for your message! I'll get back to you soon.</div>`,
	)
}

func sendContactEmail(data ContactFormData) error {
	emailSubject := fmt.Sprintf(
		"Portfolio Contact Form for %s from %s: %s",
		data.Reason,
		data.Name,
		data.Subject,
	)
	emailBody := fmt.Sprintf(`Contact Form Submission

From: %s
Reason: %s
Subject: %s

Message:
%s
`, data.Name, data.Reason, data.Subject, data.Body)

	godotenv.Load(".env")

	key := os.Getenv("SMTP_KEY")
	login := os.Getenv("SMTP_EMAIL")
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("invalid SMTP port: %v", err)
	}
	from := os.Getenv("FROM_EMAIL")
	to := os.Getenv("TO_EMAIL")

	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", emailSubject)
	msg.SetBody("text/plain", emailBody)

	dialer := gomail.NewDialer(host, port, login, key)
	if err := dialer.DialAndSend(msg); err != nil {
		return fmt.Errorf("error sending email: %v", err)
	}

	return nil
}
