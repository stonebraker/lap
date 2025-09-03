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
	"github.com/stonebraker/lap/apps/demo-utils/artifacts"

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

// ProfileData holds the profile information extracted from the profile fragment
type ProfileData struct {
	Name        string
	DisplayName string
	Picture     string
	Website     string
	PublicKey   string
	Error       string
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
	
	// Add reset artifacts route
	mux.Post("/people/alice/reset-artifacts", resetArtifactsHandler)
	
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
	verificationResult := verifyFragment(string(fragmentHTML), fragmentURL)
	
	// Process the fragment for safe rendering
	processedFragment := processFragment(string(fragmentHTML), verificationResult)
	
	// Extract attestation URLs from the raw HTML fragment (always, regardless of verification status)
	extractedResourceURL, extractedNamespaceURL := extractAttestationURLsFromHTML(string(fragmentHTML))
	
	// Fetch resource attestation - try verification context first, then extracted URL
	var resourceAttestation string
	var resourceAttestationURL string
	if verificationResult.Context != nil && verificationResult.Context.ResourceAttestationURL != "" {
		resourceAttestationURL = verificationResult.Context.ResourceAttestationURL
		resourceAttestation = fetchResourceAttestation(resourceAttestationURL)
	} else if extractedResourceURL != "" {
		resourceAttestationURL = extractedResourceURL
		resourceAttestation = fetchResourceAttestation(resourceAttestationURL)
	}
	
	// Fetch namespace attestation - try verification context first, then extracted URL
	var namespaceAttestation string
	var namespaceAttestationURL string
	var profileData *ProfileData
	if verificationResult.Context != nil && verificationResult.Context.NamespaceAttestationURL != "" {
		namespaceAttestationURL = verificationResult.Context.NamespaceAttestationURL
		namespaceAttestation = fetchNamespaceAttestation(namespaceAttestationURL)
	} else if extractedNamespaceURL != "" {
		namespaceAttestationURL = extractedNamespaceURL
		namespaceAttestation = fetchNamespaceAttestation(namespaceAttestationURL)
	}
	
	// If verification passed and we have namespace attestation, try to fetch profile data
	if verificationResult.Verified && namespaceAttestation != "" {
		if namespaceURL, err := extractNamespaceURL(namespaceAttestation); err == nil {
			profileData = fetchProfileFragment(namespaceURL)
		}
	} else {
		// If verification failed, set profileData to nil to avoid template errors
		profileData = nil
	}
	
	// Render the page with the processed fragment and verification result
	renderFragmentPageWithVerificationAndNamespaceAttestation(w, processedFragment, fragmentURL, postID, verificationResult, resourceAttestation, resourceAttestationURL, namespaceAttestation, namespaceAttestationURL, profileData)
}

// verifyFragment sends the fragment to the verifier service and returns the result
func verifyFragment(fragmentHTML string, fragmentURL string) *VerificationResult {
	verifierURL := "http://localhost:8082/verify"
	
	client := &http.Client{
		Timeout: 15 * time.Second,
	}
	
	// Create request with the actual fetch URL in header
	req, err := http.NewRequest("POST", verifierURL, bytes.NewBufferString(fragmentHTML))
	if err != nil {
		return &VerificationResult{
			Verified: false,
			Error:    fmt.Sprintf("Failed to create request: %v", err),
		}
	}
	req.Header.Set("Content-Type", "text/html")
	req.Header.Set("X-Fetch-URL", fragmentURL)
	
	// Send the request
	resp, err := client.Do(req)
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

// extractNamespaceURL extracts the namespace URL from the namespace attestation JSON
func extractNamespaceURL(namespaceAttestationJSON string) (string, error) {
	var attestation struct {
		Payload struct {
			Namespace string `json:"namespace"`
		} `json:"payload"`
	}
	
	if err := json.Unmarshal([]byte(namespaceAttestationJSON), &attestation); err != nil {
		return "", fmt.Errorf("failed to parse namespace attestation: %v", err)
	}
	
	return attestation.Payload.Namespace, nil
}

// fetchProfileFragment fetches the profile fragment from the given namespace URL
func fetchProfileFragment(namespaceURL string) *ProfileData {
	profileURL := namespaceURL + "profile/index.htmx"
	
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Get(profileURL)
	if err != nil {
		return &ProfileData{
			Error: fmt.Sprintf("Failed to fetch profile: %v", err),
		}
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return &ProfileData{
			Error: fmt.Sprintf("Profile fetch failed: HTTP %d", resp.StatusCode),
		}
	}
	
	profileHTML, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ProfileData{
			Error: fmt.Sprintf("Failed to read profile: %v", err),
		}
	}
	
	return parseProfileFromHTML(string(profileHTML))
}

// extractAttestationURLsFromHTML extracts resource and namespace attestation URLs from raw HTML fragment
func extractAttestationURLsFromHTML(htmlContent string) (string, string) {
	var resourceURL, namespaceURL string
	
	// Extract resource attestation URL
	if idx := strings.Index(htmlContent, `data-la-resource-attestation-url="`); idx >= 0 {
		start := idx + len(`data-la-resource-attestation-url="`)
		end := strings.Index(htmlContent[start:], `"`)
		if end >= 0 {
			resourceURL = htmlContent[start : start+end]
		}
	}
	
	// Extract namespace attestation URL
	if idx := strings.Index(htmlContent, `data-la-namespace-attestation-url="`); idx >= 0 {
		start := idx + len(`data-la-namespace-attestation-url="`)
		end := strings.Index(htmlContent[start:], `"`)
		if end >= 0 {
			namespaceURL = htmlContent[start : start+end]
		}
	}
	
	return resourceURL, namespaceURL
}

// parseProfileFromHTML extracts profile information from the profile fragment HTML
func parseProfileFromHTML(profileHTML string) *ProfileData {
	profile := &ProfileData{}
	
	// Extract name from h1 tag
	nameRegex := regexp.MustCompile(`<h1[^>]*>([^<]+)</h1>`)
	if matches := nameRegex.FindStringSubmatch(profileHTML); len(matches) > 1 {
		profile.DisplayName = strings.TrimSpace(matches[1])
		profile.Name = profile.DisplayName // Use display name as name for now
	}
	
	// Extract profile picture from img src
	imgRegex := regexp.MustCompile(`<img[^>]*src="([^"]+)"[^>]*alt="[^"]*profile[^"]*"[^>]*>`)
	if matches := imgRegex.FindStringSubmatch(profileHTML); len(matches) > 1 {
		profile.Picture = matches[1]
	}
	
	// Extract website from href
	websiteRegex := regexp.MustCompile(`<a[^>]*href="([^"]+)"[^>]*>([^<]+)</a>`)
	if matches := websiteRegex.FindStringSubmatch(profileHTML); len(matches) > 2 {
		profile.Website = matches[1]
	}
	
	// Extract public key from data-public-key attribute or text content
	keyRegex := regexp.MustCompile(`data-public-key="([^"]+)"`)
	if matches := keyRegex.FindStringSubmatch(profileHTML); len(matches) > 1 {
		fullKey := matches[1]
		if len(fullKey) > 8 {
			profile.PublicKey = fullKey[:4] + "..." + fullKey[len(fullKey)-4:]
		} else {
			profile.PublicKey = fullKey
		}
	} else {
		// Fallback: look for key text pattern
		keyTextRegex := regexp.MustCompile(`Key:\s*([a-f0-9]{4}\.\.\.[a-f0-9]{4})`)
		if matches := keyTextRegex.FindStringSubmatch(profileHTML); len(matches) > 1 {
			profile.PublicKey = matches[1]
		}
	}
	
	return profile
}

// renderFragmentPageWithVerificationAndNamespaceAttestation renders the server-side fetch page with the fragment, verification, and attestations
func renderFragmentPageWithVerificationAndNamespaceAttestation(w http.ResponseWriter, fragment *ProcessedFragment, fragmentURL string, currentPostID string, verification *VerificationResult, resourceAttestation string, resourceAttestationURL string, namespaceAttestation string, namespaceAttestationURL string, profileData *ProfileData) {
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
		CurrentPostID            string
		Verification             *VerificationResult
		ResourceAttestation      string
		ResourceAttestationURL   string
		NamespaceAttestation     string
		NamespaceAttestationURL  string
		ProfileData              *ProfileData
	}{
		Fragment:                 fragment,
		FragmentURL:              fragmentURL,
		CurrentPostID:            currentPostID,
		Verification:             verification,
		ResourceAttestation:      resourceAttestation,
		ResourceAttestationURL:   resourceAttestationURL,
		NamespaceAttestation:     namespaceAttestation,
		NamespaceAttestationURL:  namespaceAttestationURL,
		ProfileData:              profileData,
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

// resetArtifactsHandler calls the lapctl reset-artifacts command
func resetArtifactsHandler(w http.ResponseWriter, r *http.Request) {
	// Call the extracted ResetArtifacts function directly
	base := "http://localhost:8080"
	root := "apps/server/static/publisherapi/people/alice"
	keysDir := "keys"
	
	// Capture stderr output by redirecting it to a buffer
	var stderr bytes.Buffer
	// Note: The artifacts.ResetArtifacts function writes to os.Stderr
	// We'll capture the error and include it in the response
	
	err := artifacts.ResetArtifacts(base, root, keysDir)
	
	// Prepare response
	response := map[string]interface{}{
		"success": err == nil,
		"stdout":  "", // No stdout from the function
		"stderr":  stderr.String(),
	}
	
	if err != nil {
		response["error"] = err.Error()
		log.Printf("reset-artifacts failed: %v", err)
	}
	
	// Set response headers
	w.Header().Set("Content-Type", "application/json")
	
	// Write JSON response
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}