package main

import (
	"bytes"
	"embed"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/stonebraker/lap/apps/client-server/internal/httpx"

	"github.com/go-chi/chi/v5"
)

//go:embed templates/*.html templates/partials/*.html
var templateFS embed.FS

// VerificationResult represents the result from the verifier service
type VerificationResult struct {
	Verified             bool                   `json:"verified"`
	ResourcePresence     string                 `json:"resource_presence"`
	ResourceIntegrity    string                 `json:"resource_integrity"`
	PublisherAssociation string                 `json:"publisher_association"`
	Failure              *FailureDetails        `json:"failure"`
	Context              *VerificationContext   `json:"context"`
	Error                string                 `json:"error,omitempty"`
}

type FailureDetails struct {
	Check   string                 `json:"check"`
	Reason  string                 `json:"reason"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details"`
}

type VerificationContext struct {
	ResourceAttestationURL  string `json:"resource_attestation_url"`
	NamespaceAttestationURL string `json:"namespace_attestation_url"`
	VerifiedAt             int64  `json:"verified_at"`
}

// ProcessedFragment holds the fragment data with decoded canonical content
type ProcessedFragment struct {
	RawHTML          string
	CanonicalContent template.HTML
	CanonicalRaw     string
	PreviewContent   template.HTML
	PreviewRaw       string
	DecodeError      string
}

func main() {
	addr := flag.String("addr", ":8081", "address to listen on")
	dir := flag.String("dir", "apps/client-server/static", "directory to serve")
	flag.Parse()

	// Serve .htmx files as HTML
	_ = mime.AddExtensionType(".htmx", "text/html; charset=utf-8")

	mux := chi.NewRouter()
	
	// Add server-side fetch route
	mux.Get("/server-side-fetch/", serverSideFetchHandler)
	mux.Get("/server-side-fetch/{postID}", serverSideFetchHandler)
	
	// Mount static file server for everything else
	mux.Mount("/", httpx.NewStaticRouter(*dir))

	log.Printf("client-server serving %s on %s", *dir, *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(fmt.Errorf("server error: %w", err))
	}
}

// serverSideFetchHandler fetches Alice's post fragment and displays it
func serverSideFetchHandler(w http.ResponseWriter, r *http.Request) {
	// Get post ID from URL parameter, default to "1"
	postID := chi.URLParam(r, "postID")
	if postID == "" {
		postID = "1"
	}
	
	// Fetch the fragment from the publisher server
	fragmentURL := fmt.Sprintf("http://localhost:8080/people/alice/posts/%s/", postID)
	
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get(fragmentURL)
	if err != nil {
		renderError(w, "Failed to fetch fragment", err)
		return
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		renderError(w, "Fragment fetch failed", fmt.Errorf("HTTP %d", resp.StatusCode))
		return
	}
	
	fragmentHTML, err := io.ReadAll(resp.Body)
	if err != nil {
		renderError(w, "Failed to read fragment", err)
		return
	}
	
	// Send the fragment to the verifier service
	verificationResult := verifyFragment(string(fragmentHTML))
	
	// Process the fragment for safe rendering
	processedFragment := processFragment(string(fragmentHTML), verificationResult)
	
	// Fetch resource attestation if available
	var resourceAttestation string
	var resourceAttestationURL string
	if verificationResult.Context != nil && verificationResult.Context.ResourceAttestationURL != "" {
		resourceAttestationURL = verificationResult.Context.ResourceAttestationURL
		resourceAttestation = fetchResourceAttestation(resourceAttestationURL)
	}
	
	// Fetch namespace attestation if available
	var namespaceAttestation string
	var namespaceAttestationURL string
	if verificationResult.Context != nil && verificationResult.Context.NamespaceAttestationURL != "" {
		namespaceAttestationURL = verificationResult.Context.NamespaceAttestationURL
		namespaceAttestation = fetchNamespaceAttestation(namespaceAttestationURL)
	}
	
	// Render the page with the processed fragment and verification result
	renderFragmentPageWithVerificationAndNamespaceAttestation(w, processedFragment, fragmentURL, verificationResult, resourceAttestation, resourceAttestationURL, namespaceAttestation, namespaceAttestationURL)
}

// verifyFragment sends the fragment to the verifier service and returns the result
func verifyFragment(fragmentHTML string) *VerificationResult {
	verifierURL := "http://localhost:8082/verify"
	
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	
	// Send the fragment HTML as the request body
	resp, err := client.Post(verifierURL, "text/html", bytes.NewBufferString(fragmentHTML))
	if err != nil {
		return &VerificationResult{
			Verified: false,
			Error:    fmt.Sprintf("Failed to call verifier service: %v", err),
		}
	}
	defer resp.Body.Close()
	
	// Read the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &VerificationResult{
			Verified: false,
			Error:    fmt.Sprintf("Failed to read verifier response: %v", err),
		}
	}
	
	// Parse the JSON response
	var result VerificationResult
	if err := json.Unmarshal(body, &result); err != nil {
		return &VerificationResult{
			Verified: false,
			Error:    fmt.Sprintf("Failed to parse verifier response: %v", err),
		}
	}
	
	return &result
}

// processFragment extracts and processes the canonical content from the fragment
func processFragment(fragmentHTML string, verification *VerificationResult) *ProcessedFragment {
	processed := &ProcessedFragment{
		RawHTML: fragmentHTML,
	}
	
	// Extract preview content (the content inside la-preview section)
	// Use DOTALL flag to match across newlines
	previewRegex := regexp.MustCompile(`(?s)<section class="la-preview">(.*?)</section>`)
	if matches := previewRegex.FindStringSubmatch(fragmentHTML); len(matches) > 1 {
		previewRaw := strings.TrimSpace(matches[1])
		processed.PreviewRaw = previewRaw
		// Sanitize and render the preview content
		processed.PreviewContent = template.HTML(sanitizeHTML(previewRaw))
	}
	
	// If verification failed, don't decode canonical content
	if verification.Error != "" || !verification.Verified {
		processed.CanonicalContent = processed.PreviewContent
		return processed
	}
	
	// Extract base64 canonical content from href attribute
	hrefRegex := regexp.MustCompile(`href="data:text/html;base64,([^"]+)"`)
	matches := hrefRegex.FindStringSubmatch(fragmentHTML)
	if len(matches) < 2 {
		processed.DecodeError = "Could not find base64 canonical content in href attribute"
		processed.CanonicalContent = processed.PreviewContent
		return processed
	}
	
	// Decode the base64 content
	base64Content := matches[1]
	canonicalBytes, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		processed.DecodeError = fmt.Sprintf("Failed to decode base64 content: %v", err)
		processed.CanonicalContent = processed.PreviewContent
		return processed
	}
	
	// Store the raw canonical HTML and sanitize for rendering
	canonicalHTML := string(canonicalBytes)
	processed.CanonicalRaw = canonicalHTML
	sanitizedHTML := sanitizeHTML(canonicalHTML)
	processed.CanonicalContent = template.HTML(sanitizedHTML)
	
	return processed
}

// sanitizeHTML performs basic HTML sanitization to prevent XSS
func sanitizeHTML(htmlContent string) string {
	// For this demo, we'll do basic sanitization
	// In production, you'd want a proper HTML sanitizer library
	
	// Remove any script tags completely
	scriptRegex := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	htmlContent = scriptRegex.ReplaceAllString(htmlContent, "")
	
	// Remove any on* event attributes
	eventRegex := regexp.MustCompile(`(?i)\s+on\w+\s*=\s*["'][^"']*["']`)
	htmlContent = eventRegex.ReplaceAllString(htmlContent, "")
	
	// For this demo, we'll trust the content since it's from our own test data
	// In production, you'd want more comprehensive sanitization
	return htmlContent
}

// fetchResourceAttestation fetches the resource attestation JSON from the given URL
func fetchResourceAttestation(attestationURL string) string {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get(attestationURL)
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to fetch resource attestation: %v"}`, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Sprintf(`{"error": "Resource attestation fetch failed: HTTP %d"}`, resp.StatusCode)
	}
	
	attestationJSON, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to read resource attestation: %v"}`, err)
	}
	
	return string(attestationJSON)
}

// fetchNamespaceAttestation fetches the namespace attestation JSON from the given URL
func fetchNamespaceAttestation(attestationURL string) string {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get(attestationURL)
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to fetch namespace attestation: %v"}`, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return fmt.Sprintf(`{"error": "Namespace attestation fetch failed: HTTP %d"}`, resp.StatusCode)
	}
	
	attestationJSON, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf(`{"error": "Failed to read namespace attestation: %v"}`, err)
	}
	
	return string(attestationJSON)
}

// renderFragmentPageWithVerificationAndNamespaceAttestation renders the server-side fetch page with the fragment, verification, and attestations
func renderFragmentPageWithVerificationAndNamespaceAttestation(w http.ResponseWriter, fragment *ProcessedFragment, fragmentURL string, verification *VerificationResult, resourceAttestation string, resourceAttestationURL string, namespaceAttestation string, namespaceAttestationURL string) {
	tmpl, err := template.ParseFS(templateFS, 
		"templates/server-side-fetch.html",
		"templates/partials/*.html",
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Fragment                 *ProcessedFragment
		FragmentURL              string
		Verification             *VerificationResult
		ResourceAttestation      string
		ResourceAttestationURL   string
		NamespaceAttestation     string
		NamespaceAttestationURL  string
	}{
		Fragment:                 fragment,
		FragmentURL:              fragmentURL,
		Verification:             verification,
		ResourceAttestation:      resourceAttestation,
		ResourceAttestationURL:   resourceAttestationURL,
		NamespaceAttestation:     namespaceAttestation,
		NamespaceAttestationURL:  namespaceAttestationURL,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderError renders an error page
func renderError(w http.ResponseWriter, message string, err error) {
	tmpl, parseErr := template.ParseFS(templateFS, "templates/error.html")
	if parseErr != nil {
		http.Error(w, parseErr.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Message string
		Error   string
	}{
		Message: message,
		Error:   err.Error(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	if execErr := tmpl.Execute(w, data); execErr != nil {
		log.Printf("Error executing error template: %v", execErr)
	}
}