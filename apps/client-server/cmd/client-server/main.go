package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime"
	"net/http"
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
	
	// Render the page with the fetched fragment and verification result
	renderFragmentPageWithVerification(w, string(fragmentHTML), fragmentURL, verificationResult)
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

// renderFragmentPageWithVerification renders the server-side fetch page with the fragment and verification
func renderFragmentPageWithVerification(w http.ResponseWriter, fragmentHTML, fragmentURL string, verification *VerificationResult) {
	tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Server-Side Fetch - LAP Demo</title>
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

            <!-- Fragment Display -->
            <div class="bg-gray-800 p-8 rounded-lg border border-gray-700 mb-8">
                <h2 class="text-xl font-semibold mb-4">Fetched Fragment</h2>
                <div class="bg-gray-900 p-4 rounded border border-gray-600">
                    {{.FragmentHTML}}
                </div>
            </div>

            <!-- Raw HTML Display -->
            <div class="bg-gray-800 p-8 rounded-lg border border-gray-700">
                <h2 class="text-xl font-semibold mb-4">Raw Fragment HTML</h2>
                <pre class="bg-gray-900 p-4 rounded border border-gray-600 text-sm overflow-x-auto"><code>{{.FragmentHTML}}</code></pre>
            </div>
        </div>
    </div>
</body>
</html>`

	t, err := template.New("server-side").Parse(tmpl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		FragmentHTML string
		FragmentURL  string
		Verification *VerificationResult
	}{
		FragmentHTML: fragmentHTML,
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