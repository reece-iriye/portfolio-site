package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
)

// ContactFormData represents the form submission data
type ContactFormData struct {
	Name    string
	Reason  string
	Subject string
	Body    string
}

func main() {
	mux := http.NewServeMux()
	mux.Handle(
		"/static/",
		http.StripPrefix("/static/", http.FileServer(http.Dir("static"))),
	)

	mux.HandleFunc("/", homeHandler)
	mux.HandleFunc("/home", homeHandler)
	mux.HandleFunc("/work-history", workHandler)
	// mux.HandleFunc("/projects", projectsHandler)
	// mux.HandleFunc("/speaking-engagements", speakingHandler)
	mux.HandleFunc("/metrics", metricsHandler)
	mux.HandleFunc("/contact-me", contactHandler)
	mux.HandleFunc("/contact", contactSubmitHandler) // New handler for form submission

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
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusBadRequest)
		return
	}
	renderPage(w, r, "home", nil)
}

func workHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusBadRequest)
		return
	}
	renderPage(w, r, "work-history", nil)
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusBadRequest)
		return
	}
	renderPage(w, r, "projects", nil)
}

func speakingHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusBadRequest)
		return
	}
	renderPage(w, r, "speaking-engagements", nil)
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusBadRequest)
		return
	}
	renderPage(w, r, "metrics", nil)
}

func contactHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Invalid request method", http.StatusBadRequest)
		return
	}
	renderPage(w, r, "contact-me", nil)
}

// New handler for contact form submission
func contactSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Parse form data
	err := r.ParseForm()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(
			w,
			`<div class="form-response error">Error parsing form data. Please try again.</div>`,
		)
		return
	}

	// Extract and validate form fields
	formData := ContactFormData{
		Name:    strings.TrimSpace(r.FormValue("name")),
		Reason:  strings.TrimSpace(r.FormValue("reason")),
		Subject: strings.TrimSpace(r.FormValue("subject")),
		Body:    strings.TrimSpace(r.FormValue("body")),
	}

	// Validate required fields
	if formData.Name == "" || formData.Reason == "" || formData.Subject == "" ||
		formData.Body == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(
			w,
			`<div class="form-response error">All fields are required. Please fill out the complete form.</div>`,
		)
		return
	}

	// Send email (you'll need to configure your SMTP settings)
	err = sendContactEmail(formData)
	if err != nil {
		fmt.Printf("Error sending email: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(
			w,
			`<div class="form-response error">There was an error sending your message. Please try again or contact me directly.</div>`,
		)
		return
	}

	// Success response
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(
		w,
		`<div class="form-response success">Thank you for your message! I'll get back to you soon.</div>`,
	)
}

// Email sending function - configure with your SMTP settings
func sendContactEmail(data ContactFormData) error {
	// Email configuration - set these as environment variables
	smtpHost := os.Getenv("SMTP_HOST") // e.g., "smtp.gmail.com"
	smtpPort := os.Getenv("SMTP_PORT") // e.g., "587"
	smtpUser := os.Getenv("SMTP_USER") // your email
	smtpPass := os.Getenv("SMTP_PASS") // your email password or app password
	toEmail := os.Getenv("TO_EMAIL")   // where you want to receive contact emails

	// If email not configured, just log the message (for development)
	if smtpHost == "" || smtpPort == "" || smtpUser == "" || smtpPass == "" || toEmail == "" {
		fmt.Printf("Contact form submission (email not configured):\n")
		fmt.Printf("From: %s\n", data.Name)
		fmt.Printf("Reason: %s\n", data.Reason)
		fmt.Printf("Subject: %s\n", data.Subject)
		fmt.Printf("Body: %s\n", data.Body)
		return nil // Return nil to simulate success for development
	}

	// Create email message
	emailBody := fmt.Sprintf(`
Contact Form Submission

From: %s
Reason: %s
Subject: %s

Message:
%s
	`, data.Name, data.Reason, data.Subject, data.Body)

	msg := []byte(fmt.Sprintf("To: %s\r\n"+
		"Subject: Contact Form: %s\r\n"+
		"\r\n"+
		"%s\r\n", toEmail, data.Subject, emailBody))

	// Send email
	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpUser, []string{toEmail}, msg)

	return err
}
