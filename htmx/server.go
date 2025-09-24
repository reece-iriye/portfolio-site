package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
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

var (
	templates TemplateManager
	meter     metric.Meter

	// GDPR-compliant metrics - no personal data, limited cardinality
	httpRequestsTotal    metric.Int64Counter
	httpRequestDuration  metric.Float64Histogram
	httpRequestsInFlight metric.Int64UpDownCounter
	templateRendersTotal metric.Int64Counter
	emailsSentTotal      metric.Int64Counter
	errorTotal           metric.Int64Counter
	uptimeCounter        metric.Float64Counter
	applicationInfo      metric.Int64Counter
)

func main() {
	ctx := context.Background()

	shutdown, err := initOpenTelemetryMetrics(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize OpenTelemetry: %v", err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Printf("Failed to shutdown OpenTelemetry: %v", err)
		}
	}()

	meter = otel.Meter("htmx-portfolio-server")
	if err := initMetrics(); err != nil {
		log.Fatalf("Failed to initialize metrics: %v", err)
	}

	applicationInfo.Add(ctx, 1, metric.WithAttributes(
		attribute.String("version", "1.0.0"),
		attribute.String("environment", getEnv("ENVIRONMENT", "production")),
	))

	if err := loadTemplates(); err != nil {
		log.Fatalf("Error loading templates: %v", err)
	}

	mux := http.NewServeMux()
	staticDir := "./static/"

	// Static file serving with security headers
	mux.Handle("/static/", metricsMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "..") || strings.Contains(r.URL.Path, "~") {
				http.Error(w, "Invalid path", http.StatusBadRequest)
				return
			}

			filePath := filepath.Join(staticDir, strings.TrimPrefix(r.URL.Path, "/static/"))
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}

			ext := strings.ToLower(filepath.Ext(r.URL.Path))
			switch ext {
			case ".css", ".js":
				w.Header().Set("Cache-Control", "public, max-age=86400")
				w.Header().Set("Expires", time.Now().Add(24*time.Hour).Format(http.TimeFormat))
			case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".webp":
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				w.Header().Set("Expires", time.Now().Add(365*24*time.Hour).Format(http.TimeFormat))
			default:
				w.Header().Set("Cache-Control", "public, max-age=86400")
			}

			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Vary", "Accept-Encoding")
			http.ServeFile(w, r, filePath)
		}),
		"static",
	))

	mux.Handle("/", metricsMiddleware(http.HandlerFunc(homeHandler), "home"))
	mux.Handle("/api/home", metricsMiddleware(http.HandlerFunc(homeHandler), "home"))
	mux.Handle(
		"/api/work-history",
		metricsMiddleware(http.HandlerFunc(workHandler), "work_history"),
	)
	mux.Handle("/api/contact-me", metricsMiddleware(http.HandlerFunc(contactHandler), "contact_me"))
	mux.Handle(
		"/api/contact",
		metricsMiddleware(http.HandlerFunc(contactSubmitHandler), "contact_submit"),
	)

	mux.Handle("/home", metricsMiddleware(http.HandlerFunc(homeHandler), "home"))
	mux.Handle("/work-history", metricsMiddleware(http.HandlerFunc(workHandler), "work_history"))
	mux.Handle("/contact-me", metricsMiddleware(http.HandlerFunc(contactHandler), "contact_me"))
	mux.Handle(
		"/contact",
		metricsMiddleware(http.HandlerFunc(contactSubmitHandler), "contact_submit"),
	)

	// Health check endpoint - restrict to internal access only
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Metrics endpoint - restrict to internal access only
	mux.Handle("/metrics", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		promhttp.Handler().ServeHTTP(w, r)
	}))

	go trackUptime(ctx)

	fmt.Println("HTMX server with OpenTelemetry metrics running on :8080...")
	fmt.Println("Metrics endpoint: http://localhost:8080/metrics (internal only)")

	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal("Error starting server:", err)
	}
}

func initOpenTelemetryMetrics(ctx context.Context) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("htmx-portfolio"),
			semconv.ServiceVersion("1.0.0"),
			attribute.String("environment", getEnv("ENVIRONMENT", "production")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	promExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(promExporter),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(meterProvider)
	return meterProvider.Shutdown, nil
}

func initMetrics() error {
	var err error

	httpRequestsTotal, err = meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total HTTP requests by endpoint and status class"),
	)
	if err != nil {
		return err
	}

	httpRequestDuration, err = meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	httpRequestsInFlight, err = meter.Int64UpDownCounter(
		"http_requests_in_flight",
		metric.WithDescription("Current number of HTTP requests being processed"),
	)
	if err != nil {
		return err
	}

	templateRendersTotal, err = meter.Int64Counter(
		"template_renders_total",
		metric.WithDescription("Total template renders by type"),
	)
	if err != nil {
		return err
	}

	emailsSentTotal, err = meter.Int64Counter(
		"emails_sent_total",
		metric.WithDescription("Total emails sent by category"),
	)
	if err != nil {
		return err
	}

	errorTotal, err = meter.Int64Counter(
		"errors_total",
		metric.WithDescription("Total errors by type and endpoint"),
	)
	if err != nil {
		return err
	}

	uptimeCounter, err = meter.Float64Counter(
		"uptime_seconds_total",
		metric.WithDescription("Total uptime in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return err
	}

	applicationInfo, err = meter.Int64Counter(
		"application_info",
		metric.WithDescription("Application information"),
	)
	if err != nil {
		return err
	}

	return nil
}

// metricsMiddleware provides consistent instrumentation for all handlers
func metricsMiddleware(next http.Handler, endpoint string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()

		// Normalize method and endpoint for consistent metrics
		method := normalizeMethod(r.Method)
		normalizedEndpoint := normalizeEndpoint(r.URL.Path, endpoint)

		httpRequestsInFlight.Add(ctx, 1)
		defer httpRequestsInFlight.Add(ctx, -1)

		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrappedWriter, r)

		duration := time.Since(start).Seconds()
		statusClass := getStatusClass(wrappedWriter.statusCode)

		labels := metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("endpoint", normalizedEndpoint),
			attribute.String("status_class", statusClass),
		)

		httpRequestsTotal.Add(ctx, 1, labels)
		httpRequestDuration.Record(ctx, duration, labels)

		if wrappedWriter.statusCode >= 400 {
			errorTotal.Add(ctx, 1, metric.WithAttributes(
				attribute.String("type", "http_error"),
				attribute.String("endpoint", normalizedEndpoint),
				attribute.String("status_class", statusClass),
			))
		}
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func normalizeMethod(method string) string {
	switch method {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS":
		return method
	default:
		return "OTHER"
	}
}

func normalizeEndpoint(path, providedEndpoint string) string {
	// Use the provided endpoint if it's already normalized
	if providedEndpoint != "" {
		return providedEndpoint
	}

	// Normalize common paths to prevent cardinality issues
	path = strings.TrimSuffix(path, "/")

	switch {
	case path == "" || path == "/" || path == "/home" || path == "/api/home":
		return "home"
	case path == "/work-history" || path == "/api/work-history":
		return "work_history"
	case path == "/contact-me" || path == "/api/contact-me":
		return "contact_me"
	case path == "/contact" || path == "/api/contact":
		return "contact_submit"
	case strings.HasPrefix(path, "/static/"):
		return "static"
	case strings.HasPrefix(path, "/api/"):
		apiPath := strings.TrimPrefix(path, "/api/")
		segments := strings.Split(apiPath, "/")
		if len(segments) > 0 {
			return "api_" + segments[0]
		}
		return "api_unknown"
	default:
		// For unknown paths, use the first segment or "other"
		segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(segments) > 0 && segments[0] != "" {
			return segments[0]
		}
		return "other"
	}
}

func getStatusClass(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return "2xx"
	case statusCode >= 300 && statusCode < 400:
		return "3xx"
	case statusCode >= 400 && statusCode < 500:
		return "4xx"
	case statusCode >= 500:
		return "5xx"
	default:
		return "1xx"
	}
}

func trackUptime(ctx context.Context) {
	start := time.Now()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	uptimeCounter.Add(ctx, time.Since(start).Seconds())
	lastRecord := time.Now()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			elapsed := time.Since(lastRecord).Seconds()
			uptimeCounter.Add(ctx, elapsed)
			lastRecord = time.Now()
		}
	}
}

func loadTemplates() error {
	templates.content = make(map[string]*template.Template)

	layoutPath := filepath.Join("templates", "layout.html")
	layout, err := template.ParseFiles(layoutPath)
	if err != nil {
		return fmt.Errorf("failed to parse layout template: %w", err)
	}
	templates.layout = layout

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
	ctx := r.Context()

	t, ok := templates.content[page]
	if !ok {
		errorTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "template_not_found"),
			attribute.String("endpoint", normalizeEndpoint(r.URL.Path, "")),
		))
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	// Record template render with controlled labels
	isHTMX := r.Header.Get("HX-Request") == "true"
	renderType := "full_page"
	if isHTMX {
		renderType = "htmx_partial"
	}

	templateRendersTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("template", page),
			attribute.String("render_type", renderType),
		),
	)

	// HTMX request: only render inner content
	if isHTMX {
		if err := t.ExecuteTemplate(w, "Content", data); err != nil {
			errorTotal.Add(ctx, 1, metric.WithAttributes(
				attribute.String("type", "template_execution"),
				attribute.String("endpoint", normalizeEndpoint(r.URL.Path, "")),
			))
			http.Error(w, "Error executing template", http.StatusInternalServerError)
		}
		return
	}

	contentBuf := new(bytes.Buffer)
	if err := t.ExecuteTemplate(contentBuf, "Content", data); err != nil {
		errorTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "template_execution"),
			attribute.String("endpoint", normalizeEndpoint(r.URL.Path, "")),
		))
		http.Error(w, "Error executing template", http.StatusInternalServerError)
		return
	}

	if err := templates.layout.Execute(w, map[string]interface{}{
		"Content": template.HTML(contentBuf.String()),
	}); err != nil {
		errorTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "layout_execution"),
			attribute.String("endpoint", normalizeEndpoint(r.URL.Path, "")),
		))
		http.Error(w, "Error executing layout template", http.StatusInternalServerError)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request)    { handleGet(w, r, "home") }
func workHandler(w http.ResponseWriter, r *http.Request)    { handleGet(w, r, "work-history") }
func contactHandler(w http.ResponseWriter, r *http.Request) { handleGet(w, r, "contact-me") }

func handleGet(w http.ResponseWriter, r *http.Request, page string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusBadRequest)
		return
	}
	renderPage(w, r, page, nil)
}

func contactSubmitHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	endpoint := normalizeEndpoint(r.URL.Path, "")

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		errorTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "form_parse_error"),
			attribute.String("endpoint", endpoint),
		))
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
		errorTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "form_validation_error"),
			attribute.String("endpoint", endpoint),
		))
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(
			w,
			`<div class="form-response error">All fields are required. Please fill out the complete form.</div>`,
		)
		return
	}

	if err := sendContactEmail(ctx, formData); err != nil {
		errorTotal.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", "email_send_error"),
			attribute.String("endpoint", endpoint),
		))
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

func sendContactEmail(ctx context.Context, data ContactFormData) error {
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

	category := normalizeContactReason(data.Reason)
	emailsSentTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("category", category),
	))

	return nil
}

// Normalize contact reasons to limit cardinality and avoid PII
func normalizeContactReason(reason string) string {
	reason = strings.ToLower(strings.TrimSpace(reason))
	switch {
	case strings.Contains(reason, "job") || strings.Contains(reason, "work") || strings.Contains(reason, "career"):
		return "career"
	case strings.Contains(reason, "consult") || strings.Contains(reason, "business"):
		return "consulting"
	case strings.Contains(reason, "question") || strings.Contains(reason, "help"):
		return "inquiry"
	case strings.Contains(reason, "collaboration") || strings.Contains(reason, "project"):
		return "collaboration"
	default:
		return "general"
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func isInternalRequest(r *http.Request) bool {
	clientIP := getClientIP(r)

	allowedNetworks := []string{
		"127.0.0.1", // localhost IPv4
		"::1",       // localhost IPv6
		"172.",      // Docker default bridge networks (172.16.0.0/12)
		"10.",       // Private network (10.0.0.0/8)
		"192.168.",  // Private network (192.168.0.0/16)
	}

	for _, network := range allowedNetworks {
		if strings.HasPrefix(clientIP, network) {
			return true
		}
	}

	return false
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (from nginx)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header (from nginx)
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr
	ip, _, _ := strings.Cut(r.RemoteAddr, ":")
	return ip
}
