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

func contactSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
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

	err = sendContactEmail(formData)
	if err != nil {
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
	emailBody := fmt.Sprintf(`
Contact Form Submission

From: %s
Reason: %s
Subject: %s

Message:
%s
	`, data.Name, data.Reason, data.Subject, data.Body)

	err := godotenv.Load(".env")
	if err != nil {
		return err
	}

	key := os.Getenv("SMTP_KEY")
	from := os.Getenv("SMTP_FROM")
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return fmt.Errorf("error converting port to string for SMTP host, invalid port: %v", err)
	}
	to := os.Getenv("TO_EMAIL")

	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", emailSubject)
	msg.SetBody("text/plain", emailBody)

	dialer := gomail.NewDialer(host, port, from, key)
	if err := dialer.DialAndSend(msg); err != nil {
		return fmt.Errorf(
			"error sending email to %s from %s on SMTP server %s: %v",
			to,
			from,
			host,
			err,
		)
	}

	return err
}
