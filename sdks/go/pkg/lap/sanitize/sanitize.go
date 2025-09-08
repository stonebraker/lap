package sanitize

import (
	"github.com/microcosm-cc/bluemonday"
)

// SanitizeCanonicalContent sanitizes decoded canonical bytes using a strict policy
// to prevent XSS attacks and JavaScript embedding. Returns the sanitized HTML string.
func SanitizeCanonicalContent(canonicalBytes []byte) (string, error) {
	// Create a UGC policy that allows safe HTML tags but removes dangerous elements
	policy := bluemonday.UGCPolicy()
	
	// Allow hidden attribute only on anchor tags
	policy.AllowAttrs("hidden").OnElements("a")
	
	// Convert bytes to string
	htmlContent := string(canonicalBytes)
	
	// Sanitize the HTML content
	sanitizedHTML := policy.Sanitize(htmlContent)
	
	return sanitizedHTML, nil
}
