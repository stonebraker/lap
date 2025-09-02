package main

import (
	"bytes"
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
	
	// Mount static file server for everything else
	mux.Mount("/", httpx.NewStaticRouter(*dir))

	log.Printf("client-server serving %s on %s", *dir, *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(fmt.Errorf("server error: %w", err))
	}
}

// serverSideFetchHandler fetches Alice's post 1 fragment and displays it
func serverSideFetchHandler(w http.ResponseWriter, r *http.Request) {
	// Fetch the fragment from the publisher server
	fragmentURL := "http://localhost:8080/people/alice/posts/1/"
	
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
	
	// Render the page with the processed fragment and verification result
	renderFragmentPageWithVerification(w, processedFragment, fragmentURL, verificationResult)
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

// renderFragmentPageWithVerification renders the server-side fetch page with the fragment and verification
func renderFragmentPageWithVerification(w http.ResponseWriter, fragment *ProcessedFragment, fragmentURL string, verification *VerificationResult) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Server-Side Fetch - LAP Demo</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        .tab-content {
            display: block;
        }
        .tab-content.hidden {
            display: none;
        }
    </style>
    <script>
        function showTab(tabName) {
            // Hide all tab contents
            const tabContents = document.querySelectorAll('.tab-content');
            tabContents.forEach(content => content.classList.add('hidden'));
            
            // Remove active styles from all tab buttons
            const tabButtons = document.querySelectorAll('.tab-button');
            tabButtons.forEach(button => {
                button.classList.remove('border-blue-500', 'text-blue-400', 'bg-gray-700/50');
                button.classList.add('border-transparent', 'text-gray-400');
            });
            
            // Show selected tab content
            document.getElementById(tabName + '-tab').classList.remove('hidden');
            
            // Add active styles to clicked tab button
            event.target.classList.remove('border-transparent', 'text-gray-400');
            event.target.classList.add('border-blue-500', 'text-blue-400', 'bg-gray-700/50');
        }
        
        function copyFragment() {
            const fragmentHTML = document.getElementById('fragment-data').value;
            const button = event.target.closest('button');
            const originalHTML = button.innerHTML;
            
            // Try modern clipboard API first
            if (navigator.clipboard && window.isSecureContext) {
                navigator.clipboard.writeText(fragmentHTML).then(function() {
                    showCopySuccess(button, originalHTML);
                }).catch(function(err) {
                    console.error('Clipboard API failed: ', err);
                    fallbackCopy(fragmentHTML, button, originalHTML);
                });
            } else {
                // Fallback for non-secure contexts or older browsers
                fallbackCopy(fragmentHTML, button, originalHTML);
            }
        }
        
        function fallbackCopy(text, button, originalHTML) {
            // Create a temporary textarea element
            const textarea = document.createElement('textarea');
            textarea.value = text;
            textarea.style.position = 'fixed';
            textarea.style.opacity = '0';
            document.body.appendChild(textarea);
            textarea.select();
            
            try {
                const successful = document.execCommand('copy');
                if (successful) {
                    showCopySuccess(button, originalHTML);
                } else {
                    showCopyError();
                }
            } catch (err) {
                console.error('Fallback copy failed: ', err);
                showCopyError();
            } finally {
                document.body.removeChild(textarea);
            }
        }
        
        function showCopySuccess(button, originalHTML) {
            button.innerHTML = '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>';
            button.classList.add('bg-green-600', 'text-white');
            button.classList.remove('bg-gray-800', 'bg-gray-200', 'text-gray-400', 'text-gray-600');
            
            setTimeout(function() {
                button.innerHTML = originalHTML;
                button.classList.remove('bg-green-600', 'text-white');
                // Restore original classes based on context
                if (button.closest('.border-green-500')) {
                    button.classList.add('bg-gray-800', 'text-gray-400');
                } else {
                    button.classList.add('bg-gray-200', 'text-gray-600');
                }
            }, 2000);
        }
        
        function showCopyError() {
            alert('Failed to copy fragment to clipboard. Please try selecting and copying the text manually.');
        }
    </script>
</head>
<body class="bg-gray-950 text-gray-100 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <div class="max-w-4xl mx-auto">
            <!-- Navigation -->
            <div class="mb-8">
                <a href="/" class="text-blue-400 hover:text-blue-300 flex items-center">
                    <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"></path>
                    </svg>
                    Back to Home
                </a>
            </div>

            <!-- Header -->
            <div class="text-center mb-12">
                <h1 class="text-3xl font-bold mb-4">Server-Side Fetch Demo</h1>
                <p class="text-gray-300 text-lg">
                    LAP fragment fetched from: <code class="bg-gray-800 px-2 py-1 rounded">{{.FragmentURL}}</code>
                </p>
            </div>

            <!-- Verification Results -->
            <div class="mb-8">
                {{if .Verification.Error}}
                    <div class="bg-red-900/20 border border-red-500 p-6 rounded-lg">
                        <h2 class="text-xl font-semibold mb-4 text-red-400">Verification Error</h2>
                        <p class="text-gray-300">{{.Verification.Error}}</p>
                        <p class="text-gray-400 text-sm mt-2">Make sure the Verifier Service is running on port 8082</p>
                    </div>
                {{else}}
                    <div class="{{if .Verification.Verified}}bg-green-900/20 border-green-500{{else}}bg-red-900/20 border-red-500{{end}} border p-6 rounded-lg">
                        <h2 class="text-xl font-semibold mb-4 {{if .Verification.Verified}}text-green-400{{else}}text-red-400{{end}}">
                            Verification Result: {{if .Verification.Verified}}✓ VERIFIED{{else}}✗ FAILED{{end}}
                        </h2>
                        
                        <!-- Verification Steps -->
                        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
                            <div class="bg-gray-800 p-4 rounded">
                                <h3 class="font-medium mb-2">Resource Presence</h3>
                                <span class="{{if eq .Verification.ResourcePresence "pass"}}text-green-400{{else if eq .Verification.ResourcePresence "fail"}}text-red-400{{else}}text-gray-400{{end}}">
                                    {{.Verification.ResourcePresence}}
                                </span>
                            </div>
                            <div class="bg-gray-800 p-4 rounded">
                                <h3 class="font-medium mb-2">Resource Integrity</h3>
                                <span class="{{if eq .Verification.ResourceIntegrity "pass"}}text-green-400{{else if eq .Verification.ResourceIntegrity "fail"}}text-red-400{{else}}text-gray-400{{end}}">
                                    {{.Verification.ResourceIntegrity}}
                                </span>
                            </div>
                            <div class="bg-gray-800 p-4 rounded">
                                <h3 class="font-medium mb-2">Publisher Association</h3>
                                <span class="{{if eq .Verification.PublisherAssociation "pass"}}text-green-400{{else if eq .Verification.PublisherAssociation "fail"}}text-red-400{{else}}text-gray-400{{end}}">
                                    {{.Verification.PublisherAssociation}}
                                </span>
                            </div>
                        </div>
                        
                        {{if .Verification.Failure}}
                        <div class="bg-gray-900 p-4 rounded border border-gray-600">
                            <h4 class="font-medium mb-2">Failure Details</h4>
                            <p class="text-sm text-gray-300 mb-1"><strong>Check:</strong> {{.Verification.Failure.Check}}</p>
                            <p class="text-sm text-gray-300 mb-1"><strong>Reason:</strong> {{.Verification.Failure.Reason}}</p>
                            <p class="text-sm text-gray-300"><strong>Message:</strong> {{.Verification.Failure.Message}}</p>
                        </div>
                        {{end}}
                    </div>
                {{end}}
            </div>

            <!-- Content Display -->
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
                <!-- Rendered Content -->
                <div class="bg-gray-800 p-8 rounded-lg border border-gray-700">
                    {{if .Verification.Verified}}
                        <h2 class="text-xl font-semibold mb-4 text-green-400">
                            Verified Canonical Content
                            {{if .Fragment.DecodeError}}
                                <span class="text-red-400 text-sm ml-2">(Decode Error)</span>
                            {{end}}
                        </h2>
                        <div class="relative bg-gray-900 text-gray-100 p-4 rounded border-2 border-green-500">
                            <button onclick="copyFragment()" class="absolute top-2 right-2 p-2 bg-gray-800 hover:bg-gray-700 text-gray-400 hover:text-gray-200 rounded transition-colors" title="Copy fragment to clipboard">
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                                </svg>
                            </button>
                            {{.Fragment.CanonicalContent}}
                        </div>
                        <div class="mt-3 flex items-center text-sm text-gray-400">
                            <svg class="w-4 h-4 mr-2 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
                            </svg>
                            This content was cryptographically verified and decoded from base64
                        </div>
                    {{else}}
                        <h2 class="text-xl font-semibold mb-4">
                            Preview Content
                            {{if .Fragment.DecodeError}}
                                <span class="text-red-400 text-sm ml-2">(Decode Error)</span>
                            {{end}}
                        </h2>
                        <div class="relative bg-white text-gray-900 p-4 rounded border">
                            <button onclick="copyFragment()" class="absolute top-2 right-2 p-2 bg-gray-200 hover:bg-gray-300 text-gray-600 hover:text-gray-800 rounded transition-colors" title="Copy fragment to clipboard">
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                                </svg>
                            </button>
                            <div class="text-red-600 mb-4 p-3 bg-red-100 rounded">
                                ⚠️ Content not verified - showing preview only
                            </div>
                            {{.Fragment.CanonicalContent}}
                        </div>
                    {{end}}
                    {{if .Fragment.DecodeError}}
                        <div class="mt-4 p-3 bg-red-900/20 border border-red-500 rounded text-red-400 text-sm">
                            <strong>Decode Error:</strong> {{.Fragment.DecodeError}}
                        </div>
                    {{end}}
                    
                    <!-- Content Explanation -->
                    {{if .Verification.Verified}}
                        <div class="mt-6 p-4 bg-green-900/20 border border-green-500 rounded">
                            <h3 class="font-medium mb-3 text-green-400">About This Content</h3>
                            <div class="text-sm text-gray-300">
                                <p class="mb-2">
                                    You are viewing the <strong class="text-green-300">verified canonical content</strong> - the authoritative version that has been cryptographically verified through the LAP protocol.
                                </p>
                                <p class="mb-2">
                                    This content was decoded from the base64-encoded data in the fragment's &lt;link&gt; element and has passed all three verification checks:
                                </p>
                                <ul class="list-disc list-inside ml-4 space-y-1 text-gray-400">
                                    <li>Resource Presence - confirms the resource exists at the expected location</li>
                                    <li>Resource Integrity - validates the content hasn't been tampered with</li>
                                    <li>Publisher Association - verifies the publisher's claim to this content</li>
                                </ul>
                                <div class="mt-3 p-2 bg-green-800/30 rounded text-xs text-green-200">
                                    <strong>Security:</strong> This content can be trusted as authentic and unmodified.
                                </div>
                            </div>
                        </div>
                    {{else}}
                        <div class="mt-6 p-4 bg-yellow-900/20 border border-yellow-500 rounded">
                            <h3 class="font-medium mb-3 text-yellow-400">About This Content</h3>
                            <div class="text-sm text-gray-300">
                                <p class="mb-2">
                                    You are viewing <strong class="text-yellow-300">preview content</strong> because the fragment could not be verified or verification failed.
                                </p>
                                <p class="mb-2">
                                    Preview content is extracted from the fragment's &lt;section class="la-preview"&gt; element and provides immediate readability, but it has important limitations:
                                </p>
                                <ul class="list-disc list-inside ml-4 space-y-1 text-gray-400">
                                    <li>It is <strong>not cryptographically verified</strong></li>
                                    <li>It can be modified by malicious clients while verification continues to pass</li>
                                    <li>It should not be trusted for security-critical applications</li>
                                </ul>
                                <div class="mt-3 p-2 bg-yellow-800/30 rounded text-xs text-yellow-200">
                                    <strong>Security Warning:</strong> This content cannot be trusted as authentic. Only verified canonical content should be considered authoritative.
                                </div>
                            </div>
                        </div>
                    {{end}}
                </div>

                <!-- Verification Result JSON -->
                <div class="bg-gray-800 p-8 rounded-lg border border-gray-700">
                    <h2 class="text-xl font-semibold mb-4">Verification Result JSON</h2>
                    <p class="text-gray-400 text-sm mb-4">
                        Raw JSON response from the verifier service
                    </p>
                    <div class="bg-gray-900 p-4 rounded border border-gray-600 text-sm overflow-x-auto">
                        <pre class="text-gray-100"><code>{
  "verified": {{if .Verification.Verified}}true{{else}}false{{end}},
  "resource_presence": "{{.Verification.ResourcePresence}}",
  "resource_integrity": "{{.Verification.ResourceIntegrity}}",
  "publisher_association": "{{.Verification.PublisherAssociation}}"{{if .Verification.Failure}},
  "failure": {
    "check": "{{.Verification.Failure.Check}}",
    "reason": "{{.Verification.Failure.Reason}}",
    "message": "{{.Verification.Failure.Message}}"
  }{{end}}{{if .Verification.Context}},
  "context": {
    "resource_attestation_url": "{{.Verification.Context.ResourceAttestationURL}}",
    "namespace_attestation_url": "{{.Verification.Context.NamespaceAttestationURL}}",
    "verified_at": {{.Verification.Context.VerifiedAt}}
  }{{end}}{{if .Verification.Error}},
  "error": "{{.Verification.Error}}"{{end}}
}</code></pre>
                    </div>
                </div>
            </div>

            <!-- LAP Protocol Roles -->
            <div class="mt-12 bg-gray-800 rounded-lg border border-gray-700">
                <h2 class="text-xl font-semibold mb-6 p-6 pb-0 text-gray-100">LAP Protocol Role Responsibilities</h2>
                
                <!-- Tab Navigation -->
                <div class="flex border-b border-gray-600 px-6">
                    <button class="tab-button px-4 py-2 text-sm font-medium border-b-2 border-blue-500 text-blue-400 bg-gray-700/50" onclick="showTab('server')">Server</button>
                    <button class="tab-button px-4 py-2 text-sm font-medium border-b-2 border-transparent text-gray-400 hover:text-gray-300" onclick="showTab('client')">Client</button>
                    <button class="tab-button px-4 py-2 text-sm font-medium border-b-2 border-transparent text-gray-400 hover:text-gray-300" onclick="showTab('verifier')">Verifier</button>
                </div>
                
                <!-- Tab Content -->
                <div class="p-6">
                    <!-- Server Tab -->
                    <div id="server-tab" class="tab-content">
                        <div class="text-sm text-gray-300 space-y-4">
                            <div>
                                <h3 class="font-medium mb-2 text-green-300">Resource Management (MUST)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Generate Resource Attestations for each fragment</li>
                                    <li>Ensure Resource Attestations are served from the same origin as the resource</li>
                                </ul>
                            </div>
                            
                            <div>
                                <h3 class="font-medium mb-2 text-green-300">Fragment Generation (MUST)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Embed canonical content bytes in fragment's &lt;link&gt; element using href="data:text/html;base64,..."</li>
                                    <li>Include SHA-256 hash of canonical content bytes in data-hash attribute</li>
                                    <li>Embed Resource Attestation data in &lt;link&gt; element's data attributes</li>
                                    <li>Include publisher's public key in data-la-publisher-claim attribute</li>
                                    <li>Specify Resource and Namespace Attestation URLs in respective data attributes</li>
                                </ul>
                            </div>
                            
                            <div>
                                <h3 class="font-medium mb-2 text-green-300">Attestation Management (MUST)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Create and sign Resource Attestations using the resource's private key</li>
                                    <li>Create and sign Namespace Attestations using the publisher's private key</li>
                                    <li>Serve attestations at specified URLs with proper namespace coverage</li>
                                </ul>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Client Tab -->
                    <div id="client-tab" class="tab-content hidden">
                        <div class="text-sm text-gray-300 space-y-4">
                            <div>
                                <h3 class="font-medium mb-2 text-purple-300">Fragment Handling (MUST)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Preserve all LAP protocol attributes when embedding fragments</li>
                                    <li>NOT modify the canonical content bytes in the &lt;link&gt; element</li>
                                    <li>NOT alter attestation URLs in data-la-resource-attestation-url and data-la-namespace-attestation-url attributes</li>
                                </ul>
                            </div>
                            
                            <div>
                                <h3 class="font-medium mb-2 text-purple-300">Verification State Handling (MUST)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Clearly indicate when a fragment has not been verified OR not render it</li>
                                    <li>Clearly indicate when a fragment has failed verification OR not render it</li>
                                    <li>Are NOT required to indicate successful verification</li>
                                </ul>
                            </div>
                            
                            <div>
                                <h3 class="font-medium mb-2 text-purple-300">Preview Content Handling (MUST)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>NOT modify preview content in ways that misrepresent the canonical bytes</li>
                                    <li>Replace only the contents of the preview section, not the entire &lt;section&gt; element</li>
                                    <li>Handle canonical content decode/parse failures gracefully by falling back to preview content</li>
                                    <li>Sanitize canonical content before DOM injection to prevent XSS attacks</li>
                                </ul>
                            </div>
                            
                            <div>
                                <h3 class="font-medium mb-2 text-purple-300">Recommended Practices (SHOULD)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Verify fragments before rendering (using a Verifier)</li>
                                    <li>Handle verification failures gracefully</li>
                                    <li>Respect fragment expiration times</li>
                                    <li>Provide means for users to manually verify at-rest fragments</li>
                                    <li>Replace preview section contents with decoded canonical bytes when rendering</li>
                                    <li>Validate that decoded canonical content is well-formed HTML before replacement</li>
                                </ul>
                            </div>
                            
                            <div class="mt-4 p-3 bg-purple-800/30 rounded text-xs">
                                <strong class="text-purple-300">Security Note:</strong> Preview content can be modified by malicious clients while verification continues to pass. 
                                Conforming clients should prioritize canonical content bytes over preview content for security-critical applications.
                            </div>
                        </div>
                    </div>
                    
                    <!-- Verifier Tab -->
                    <div id="verifier-tab" class="tab-content hidden">
                        <div class="text-sm text-gray-300 space-y-4">
                            <div>
                                <h3 class="font-medium mb-2 text-orange-300">Verification Process (MUST)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Perform all three verification checks: Resource Presence, Resource Integrity, and Publisher Association</li>
                                    <li>Validate Resource Attestation signatures against the resource's public key</li>
                                    <li>Validate Namespace Attestation signatures against the publisher's public key</li>
                                    <li>Confirm fragment's data-la-publisher-claim matches Namespace Attestation's key</li>
                                </ul>
                            </div>
                            
                            <div>
                                <h3 class="font-medium mb-2 text-orange-300">Network Operations (MUST)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Fetch Resource Attestations from URL specified in fragment's data-la-resource-attestation-url</li>
                                    <li>Fetch Namespace Attestations from URL specified in data-la-namespace-attestation-url</li>
                                    <li>Handle network failures gracefully</li>
                                </ul>
                            </div>
                            
                            <div>
                                <h3 class="font-medium mb-2 text-orange-300">Result Reporting (MUST)</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Return verification results conforming to the normative result object specification</li>
                                    <li>Report specific failure reasons and error codes</li>
                                    <li>Implement fail-fast behavior (stop at first failure, skip remaining checks)</li>
                                </ul>
                            </div>
                            
                            <div>
                                <h3 class="font-medium mb-2 text-orange-300">Implementation Options</h3>
                                <ul class="list-disc list-inside space-y-1 text-gray-400">
                                    <li>Verification libraries (vendored or imported by clients)</li>
                                    <li>Verification web services (called by clients over HTTP/API)</li>
                                    <li>Browser plugins (installed by users)</li>
                                    <li>Custom verification programs (not recommended for most use cases)</li>
                                </ul>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Raw HTML Display -->
            <div class="mt-12 bg-gray-800 p-8 rounded-lg border border-gray-700">
                <h2 class="text-xl font-semibold mb-4">Raw Fragment HTML</h2>
                <pre class="bg-gray-900 p-4 rounded border border-gray-600 text-sm overflow-x-auto"><code>{{.Fragment.RawHTML}}</code></pre>
            </div>
        </div>
    </div>
    
    <!-- Hidden element to store fragment HTML for copying -->
    <textarea id="fragment-data" style="display: none;">{{.Fragment.RawHTML}}</textarea>
</body>
</html>`

	t, err := template.New("server-side").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		Fragment     *ProcessedFragment
		FragmentURL  string
		Verification *VerificationResult
	}{
		Fragment:     fragment,
		FragmentURL:  fragmentURL,
		Verification: verification,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t.Execute(w, data)
}

// renderError renders an error page
func renderError(w http.ResponseWriter, message string, err error) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Error - LAP Demo</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-950 text-gray-100 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <div class="max-w-4xl mx-auto">
            <!-- Navigation -->
            <div class="mb-8">
                <a href="/" class="text-blue-400 hover:text-blue-300 flex items-center">
                    <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"></path>
                    </svg>
                    Back to Home
                </a>
            </div>

            <!-- Error Display -->
            <div class="bg-red-900/20 border border-red-500 p-8 rounded-lg text-center">
                <div class="w-16 h-16 bg-red-600 rounded-full flex items-center justify-center mx-auto mb-4">
                    <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                    </svg>
                </div>
                <h1 class="text-2xl font-bold mb-4">{{.Message}}</h1>
                <p class="text-gray-300 mb-4">{{.Error}}</p>
                <p class="text-gray-400 text-sm">
                    Make sure the Publisher Server is running on port 8080
                </p>
            </div>
        </div>
    </div>
</body>
</html>`

	t, err := template.New("error").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	t.Execute(w, data)
}